package main

import (
	"bufio"
	"fmt"
	"os"

	"game/internal/adapters/memory"
	"game/internal/usecase"
	"game/internal/world"
)

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