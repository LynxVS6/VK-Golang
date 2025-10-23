package main

import (
	"fmt"
	"sort"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	// замыкаем все звенья конвейера каналами и запускаем каждое звено в своей горутине
	var wg sync.WaitGroup

	// первый вход — закрытый канал
	in := make(chan interface{})
	close(in)

	for _, c := range cmds {
		out := make(chan interface{})
		wg.Add(1)

		go func(c cmd, in, out chan interface{}) {
			defer wg.Done()
			defer close(out)
			c(in, out)
		}(c, in, out)

		in = out
	}

	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	// in  - string (email)
	// out - User (уникальный по ID)
	resCh := make(chan User)
	var wg sync.WaitGroup

	// агрегатор уникальных пользователей
	done := make(chan struct{})
	go func() {
		defer close(done)
		seen := make(map[uint64]struct{})
		for u := range resCh {
			if _, ok := seen[u.ID]; ok {
				continue
			}
			seen[u.ID] = struct{}{}
			out <- u
		}
	}()

	// получаем пользователей параллельно
	for v := range in {
		email := v.(string)
		wg.Add(1)
		go func(e string) {
			defer wg.Done()
			u := GetUser(e)
			resCh <- u
		}(email)
	}

	wg.Wait()
	close(resCh)
	<-done
}

func SelectMessages(in, out chan interface{}) {
	// in  - User
	// out - MsgID (все письма пользователей)
	var wg sync.WaitGroup
	msgsCh := make(chan []MsgID)

	// передаём результаты дальше «по струе»
	aggDone := make(chan struct{})
	go func() {
		defer close(aggDone)
		for ids := range msgsCh {
			for _, id := range ids {
				out <- id
			}
		}
	}()

	batch := make([]User, 0, GetMessagesMaxUsersBatch)

	dispatch := func(b []User) {
		if len(b) == 0 {
			return
		}
		// копируем, чтобы не переиспользовать один и тот же слайс
		bcp := make([]User, len(b))
		copy(bcp, b)
		wg.Add(1)
		go func(users []User) {
			defer wg.Done()
			msgs, err := GetMessages(users...)
			if err != nil {
				return
			}
			msgsCh <- msgs
		}(bcp)
	}

	for v := range in {
		u := v.(User)
		batch = append(batch, u)
		if len(batch) == GetMessagesMaxUsersBatch {
			dispatch(batch)
			batch = batch[:0]
		}
	}
	// добиваем хвост
	if len(batch) > 0 {
		dispatch(batch)
	}

	wg.Wait()
	close(msgsCh)
	<-aggDone
}

func CheckSpam(in, out chan interface{}) {
	// in  - MsgID
	// out - MsgData (ID, HasSpam)
	var wg sync.WaitGroup
	sem := make(chan struct{}, HasSpamMaxAsyncRequests)

	for v := range in {
		id := v.(MsgID)
		wg.Add(1)
		sem <- struct{}{}
		go func(id MsgID) {
			defer wg.Done()
			defer func() { <-sem }()
			has, err := HasSpam(id)
			if err != nil {
				return
			}
			out <- MsgData{ID: id, HasSpam: has}
		}(id)
	}

	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	// in  - MsgData
	// out - string "<has_spam> <msg_id>"
	var all []MsgData
	for v := range in {
		all = append(all, v.(MsgData))
	}

	// сортировка: сначала со спамом (true), затем по ID по возрастанию
	sort.Slice(all, func(i, j int) bool {
		if all[i].HasSpam != all[j].HasSpam {
			return all[i].HasSpam && !all[j].HasSpam
		}
		return all[i].ID < all[j].ID
	})

	for _, m := range all {
		out <- fmt.Sprintf("%v %d", m.HasSpam, m.ID)
	}
}
