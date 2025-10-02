package main

import (
	"game/internal/adapters/memory"
	"game/internal/usecase"
	"game/internal/world"
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

func main() {}