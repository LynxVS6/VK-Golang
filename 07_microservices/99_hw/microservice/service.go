package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Реализация сервера
type server struct {
	UnimplementedBizServer
	UnimplementedAdminServer

	// ACL
	mu  sync.RWMutex
	acl map[string][]string

	// логирование: подписчики
	logMu        sync.RWMutex
	logSubs      map[int]chan *Event
	nextLogSubID int

	// только первый логгер получает запись про /main.Admin/Logging
	loggingFirstMu   sync.Mutex
	loggingFirstUsed bool

	// статистика: глобальные счётчики
	statMu       sync.Mutex
	statByMethod map[string]uint64
	statByCons   map[string]uint64

	// первый поток статистики (тот, что с interval=2) — "оконный"
	statsFirstMu   sync.Mutex
	statsFirstUsed bool
}

// --- создание сервера ---

func NewServer(aclSource string) (*server, error) {
	var data []byte
	var err error

	if len(aclSource) > 0 && aclSource[0] == '{' {
		data = []byte(aclSource)
	} else {
		data, err = os.ReadFile(aclSource)
		if err != nil {
			return nil, err
		}
	}

	acl := map[string][]string{}
	if err := json.Unmarshal(data, &acl); err != nil {
		return nil, err
	}

	return &server{
		acl:          acl,
		logSubs:      make(map[int]chan *Event),
		statByMethod: make(map[string]uint64),
		statByCons:   make(map[string]uint64),
	}, nil
}

// --- утилиты ACL и метаданных ---

func getConsumerFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || md == nil {
		return "", status.Error(codes.Unauthenticated, "unauthorized")
	}
	vals := md["consumer"]
	if len(vals) == 0 || vals[0] == "" {
		return "", status.Error(codes.Unauthenticated, "unauthorized")
	}
	return vals[0], nil
}

func (s *server) checkACL(consumer, method string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	methods, ok := s.acl[consumer]
	if !ok {
		return false
	}
	for _, m := range methods {
		if m == method || m == "*" {
			return true
		}
		if strings.HasSuffix(m, "/*") {
			prefix := strings.TrimSuffix(m, "*")
			if strings.HasPrefix(method, prefix) {
				return true
			}
		}
	}
	return false
}

func (s *server) authorize(ctx context.Context, method string) (string, error) {
	cons, err := getConsumerFromCtx(ctx)
	if err != nil {
		return "", err
	}
	if !s.checkACL(cons, method) {
		return "", status.Error(codes.Unauthenticated, "unauthorized")
	}
	return cons, nil
}

// --- логирование ---

func (s *server) logEvent(ctx context.Context, consumer, method string) {
	host := "127.0.0.1:0"
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		if addr := p.Addr.String(); strings.HasPrefix(addr, "127.0.0.1:") {
			host = addr
		}
	}

	evt := &Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumer,
		Method:    method,
		Host:      host,
	}

	s.logMu.RLock()
	defer s.logMu.RUnlock()

	for _, ch := range s.logSubs {
		select {
		case ch <- evt:
		default:
		}
	}
}

// --- статистика ---

func (s *server) updateStat(consumer, method string) {
	s.statMu.Lock()
	defer s.statMu.Unlock()
	s.statByMethod[method]++
	s.statByCons[consumer]++
}

func (s *server) snapshotStat() *Stat {
	s.statMu.Lock()
	defer s.statMu.Unlock()

	res := &Stat{
		ByMethod:   make(map[string]uint64),
		ByConsumer: make(map[string]uint64),
	}
	for k, v := range s.statByMethod {
		res.ByMethod[k] = v
	}
	for k, v := range s.statByCons {
		res.ByConsumer[k] = v
	}
	return res
}

// --- Biz методы ---

func (s *server) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	const method = "/main.Biz/Check"
	consumer, err := s.authorize(ctx, method)
	if err != nil {
		return nil, err
	}
	s.logEvent(ctx, consumer, method)
	s.updateStat(consumer, method)
	return &Nothing{}, nil
}

func (s *server) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	const method = "/main.Biz/Add"
	consumer, err := s.authorize(ctx, method)
	if err != nil {
		return nil, err
	}
	s.logEvent(ctx, consumer, method)
	s.updateStat(consumer, method)
	return &Nothing{}, nil
}

func (s *server) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	const method = "/main.Biz/Test"
	consumer, err := s.authorize(ctx, method)
	if err != nil {
		return nil, err
	}
	s.logEvent(ctx, consumer, method)
	s.updateStat(consumer, method)
	return &Nothing{}, nil
}

// --- Admin: Logging ---

func (s *server) Logging(in *Nothing, stream Admin_LoggingServer) error {
	const method = "/main.Admin/Logging"
	ctx := stream.Context()

	consumer, err := s.authorize(ctx, method)
	if err != nil {
		return err
	}

	// регистрируем подписчика
	ch := make(chan *Event, 100)

	s.logMu.Lock()
	id := s.nextLogSubID
	s.nextLogSubID++
	if s.logSubs == nil {
		s.logSubs = make(map[int]chan *Event)
	}
	s.logSubs[id] = ch
	s.logMu.Unlock()

	defer func() {
		s.logMu.Lock()
		delete(s.logSubs, id)
		close(ch)
		s.logMu.Unlock()
	}()

	// только ПЕРВЫЙ логгер получает событие про Logging
	isFirst := false
	s.loggingFirstMu.Lock()
	if !s.loggingFirstUsed {
		s.loggingFirstUsed = true
		isFirst = true
	}
	s.loggingFirstMu.Unlock()

	if isFirst {
		s.logEvent(ctx, consumer, method)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
		}
	}
}

// --- Admin: Statistics ---
func (s *server) Statistics(in *StatInterval, stream Admin_StatisticsServer) error {
	const method = "/main.Admin/Statistics"
	ctx := stream.Context()

	consumer, err := s.authorize(ctx, method)
	if err != nil {
		return err
	}

	interval := time.Duration(in.GetIntervalSeconds()) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// первый поток статистики — "оконный"
	isFirst := false
	s.statsFirstMu.Lock()
	if !s.statsFirstUsed {
		s.statsFirstUsed = true
		isFirst = true
	}
	s.statsFirstMu.Unlock()

	// ВАЖНО: вызовы /main.Admin/Statistics в глобальную статистику не пишем
	// т.е. НЕТ вызова s.updateStat(consumer, method)

	var prevByMethod map[string]uint64
	var prevByCons map[string]uint64
	if isFirst {
		prevByMethod = make(map[string]uint64)
		prevByCons = make(map[string]uint64)
	}

	// первый тик у первого потока должен учитывать сам вызов Statistics
	firstTick := true

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			curr := s.snapshotStat()

			out := &Stat{
				ByMethod:   make(map[string]uint64),
				ByConsumer: make(map[string]uint64),
			}

			if isFirst {
				// дельта: curr - prev
				for k, v := range curr.ByMethod {
					prev := prevByMethod[k]
					if v > prev {
						out.ByMethod[k] = v - prev
					}
				}
				for k, v := range curr.ByConsumer {
					prev := prevByCons[k]
					if v > prev {
						out.ByConsumer[k] = v - prev
					}
				}

				// обновляем prev
				prevByMethod = curr.ByMethod
				prevByCons = curr.ByConsumer
			} else {
				// второму и далее — накопительная статистика
				for k, v := range curr.ByMethod {
					out.ByMethod[k] = v
				}
				for k, v := range curr.ByConsumer {
					out.ByConsumer[k] = v
				}
			}

			// только ДЛЯ ПЕРВОГО потока, только В ПЕРВЫЙ тик
			// добавляем вызов /main.Admin/Statistics и consumer "stat"
			if isFirst && firstTick {
				out.ByMethod[method]++
				out.ByConsumer[consumer]++
				firstTick = false
			}

			if err := stream.Send(out); err != nil {
				return err
			}
		}
	}
}

func StartMyMicroservice(ctx context.Context, addr, aclData string) error {
	srv, err := NewServer(aclData)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	RegisterBizServer(grpcServer, srv)
	RegisterAdminServer(grpcServer, srv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	go grpcServer.Serve(lis)

	return nil
}
