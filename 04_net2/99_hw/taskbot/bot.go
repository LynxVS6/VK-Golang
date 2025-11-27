package main

// сюда писать код

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/skinass/telegram-bot-api/v5"
)

var (
	// @BotFather в телеграме даст вам это
	BotToken = "XXX"

	// урл выдаст вам игрок или хероку
	WebhookURL = "https://525f2cb5.ngrok.io"
)

type Task struct {
	ID               int
	Title            string
	OwnerID          int64
	OwnerUsername    string
	AssigneeID       int64
	AssigneeUsername string
	HasAssignee      bool
	Resolved         bool
}

type Store struct {
	mu     sync.Mutex
	byID   map[int]*Task
	order  []int
	nextID int
}

func newStore() *Store {
	return &Store{byID: make(map[int]*Task), nextID: 1}
}

func (s *Store) create(title string, owner *tgbotapi.User) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &Task{
		ID:            s.nextID,
		Title:         title,
		OwnerID:       owner.ID,
		OwnerUsername: owner.UserName,
	}
	s.byID[t.ID] = t
	s.order = append(s.order, t.ID)
	s.nextID++
	return t
}

func (s *Store) assign(id int, u *tgbotapi.User) (t *Task, prevAssignee int64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok = s.byID[id]
	if !ok || t.Resolved {
		return nil, 0, false
	}
	prevAssignee = 0
	if t.HasAssignee {
		prevAssignee = t.AssigneeID
	}
	t.AssigneeID = u.ID
	t.AssigneeUsername = u.UserName
	t.HasAssignee = true
	return t, prevAssignee, true
}

func (s *Store) unassign(id int, who int64) (t *Task, ok bool, wasOnUser bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok = s.byID[id]
	if !ok || t.Resolved {
		return nil, false, false
	}
	if !t.HasAssignee || t.AssigneeID != who {
		return t, true, false
	}
	t.HasAssignee = false
	t.AssigneeID = 0
	t.AssigneeUsername = ""
	return t, true, true
}

func (s *Store) resolve(id int, who int64) (t *Task, ok bool, allowed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok = s.byID[id]
	if !ok || t.Resolved {
		return nil, false, false
	}
	if !t.HasAssignee || t.AssigneeID != who {
		return t, true, false
	}
	t.Resolved = true
	return t, true, true
}

func (s *Store) listActive() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res []*Task
	for _, id := range s.order {
		if t, ok := s.byID[id]; ok && !t.Resolved {
			res = append(res, t)
		}
	}
	return res
}

func (s *Store) listMy(uid int64) []*Task {
	all := s.listActive()
	var res []*Task
	for _, t := range all {
		if t.HasAssignee && t.AssigneeID == uid {
			res = append(res, t)
		}
	}
	return res
}

func (s *Store) listOwner(uid int64) []*Task {
	all := s.listActive()
	var res []*Task
	for _, t := range all {
		if t.OwnerID == uid {
			res = append(res, t)
		}
	}
	return res
}

var store = newStore()

func uname(u string) string {
	if u == "" {
		return ""
	}
	if strings.HasPrefix(u, "@") {
		return u
	}
	return "@" + u
}

func formatEntryTasks(t *Task, viewer int64) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "%d. %s by %s", t.ID, t.Title, uname(t.OwnerUsername))
	if !t.HasAssignee {
		fmt.Fprintf(b, "\n/assign_%d", t.ID)
		return b.String()
	}
	if t.AssigneeID == viewer {
		fmt.Fprintf(b, "\nassignee: я\n/unassign_%d /resolve_%d", t.ID, t.ID)
	} else {
		fmt.Fprintf(b, "\nassignee: %s", uname(t.AssigneeUsername))
	}
	return b.String()
}

func formatEntryMy(t *Task) string {
	return fmt.Sprintf("%d. %s by %s\n/unassign_%d /resolve_%d", t.ID, t.Title, uname(t.OwnerUsername), t.ID, t.ID)
}

func formatEntryOwner(t *Task) string {
	if !t.HasAssignee {
		return fmt.Sprintf("%d. %s by %s\n/assign_%d", t.ID, t.Title, uname(t.OwnerUsername), t.ID)
	}
	return fmt.Sprintf("%d. %s by %s", t.ID, t.Title, uname(t.OwnerUsername))
}

func formatList(ts []*Task, f func(*Task) string) string {
	if len(ts) == 0 {
		return "Нет задач"
	}
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, f(t))
	}
	return strings.Join(parts, "\n\n")
}

func startTaskBot(ctx context.Context) error {
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return err
	}

	if wh, err := tgbotapi.NewWebhook(WebhookURL); err == nil {
		_, _ = bot.Request(wh)
	}

	u, err := url.Parse(WebhookURL)
	if err != nil {
		return err
	}
	path := u.Path
	if path == "" {
		path = "/"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var upd tgbotapi.Update
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if upd.Message == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := upd.Message
		user := msg.From
		chatID := msg.Chat.ID
		text := strings.TrimSpace(msg.Text)

		send := func(id int64, txt string) {
			m := tgbotapi.NewMessage(id, txt)
			_, _ = bot.Send(m)
		}

		switch {
		case text == "/tasks":
			list := store.listActive()
			send(chatID, formatList(list, func(t *Task) string { return formatEntryTasks(t, user.ID) }))

		case text == "/my":
			list := store.listMy(user.ID)
			send(chatID, formatList(list, func(t *Task) string { return formatEntryMy(t) }))

		case text == "/owner":
			list := store.listOwner(user.ID)
			send(chatID, formatList(list, func(t *Task) string { return formatEntryOwner(t) }))

		case strings.HasPrefix(text, "/new"):
			name := strings.TrimSpace(strings.TrimPrefix(text, "/new"))
			if name == "" {
				return
			}
			t := store.create(name, user)
			send(chatID, fmt.Sprintf("Задача \"%s\" создана, id=%d", t.Title, t.ID))

		case strings.HasPrefix(text, "/assign_"):
			idStr := strings.TrimPrefix(text, "/assign_")
			id, _ := strconv.Atoi(idStr)
			t, prev, ok := store.assign(id, user)
			if !ok {
				return
			}
			send(chatID, fmt.Sprintf("Задача \"%s\" назначена на вас", t.Title))
			if prev != 0 && prev != user.ID {
				send(prev, fmt.Sprintf("Задача \"%s\" назначена на %s", t.Title, uname(t.AssigneeUsername)))
			} else if prev == 0 && t.OwnerID != user.ID {
				send(t.OwnerID, fmt.Sprintf("Задача \"%s\" назначена на %s", t.Title, uname(t.AssigneeUsername)))
			}

		case strings.HasPrefix(text, "/unassign_"):
			idStr := strings.TrimPrefix(text, "/unassign_")
			id, _ := strconv.Atoi(idStr)
			t, ok, wasOnUser := store.unassign(id, user.ID)
			if !ok {
				return
			}
			if !wasOnUser {
				send(chatID, "Задача не на вас")
				return
			}
			send(chatID, "Принято")
			send(t.OwnerID, fmt.Sprintf("Задача \"%s\" осталась без исполнителя", t.Title))

		case strings.HasPrefix(text, "/resolve_"):
			idStr := strings.TrimPrefix(text, "/resolve_")
			id, _ := strconv.Atoi(idStr)
			t, ok, allowed := store.resolve(id, user.ID)
			if !ok {
				return
			}
			if !allowed {
				return
			}
			send(chatID, fmt.Sprintf("Задача \"%s\" выполнена", t.Title))
			send(t.OwnerID, fmt.Sprintf("Задача \"%s\" выполнена %s", t.Title, uname(user.UserName)))
		}

		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: u.Host, Handler: mux}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	err = srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func main() {
	err := startTaskBot(context.Background())
	if err != nil {
		panic(err)
	}
}
