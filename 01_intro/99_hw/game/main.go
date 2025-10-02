package main

import (
	"game/internal/adapters/memory"
	"game/internal/usecase"
	"game/internal/world"
	"bufio"
	"fmt"
	"os"
)

var (
	repo = memory.NewRepo()
	svc  = usecase.NewGameService(repo)
)

func initGame() {
	svc.Init(world.Build())
}

func handleCommand(s string) string {
	return svc.Handle(s)
}

func main() {
	repo := memory.NewRepo()
	svc := usecase.NewGameService(repo)
	svc.Init(world.Build())

	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !in.Scan() {
			return
		}
		fmt.Println(svc.Handle(in.Text()))
	}
}