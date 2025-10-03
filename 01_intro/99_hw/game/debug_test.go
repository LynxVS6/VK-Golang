package main

import "testing"

func TestDebugSequence(t *testing.T) {
	initGame()
	cmds := []string{"идти коридор", "идти комната", "надеть рюкзак", "взять ключи", "применить ключи дверь"}
	for _, c := range cmds {
		res := handleCommand(c)
		t.Logf("cmd=%q -> %q", c, res)
	}
}
