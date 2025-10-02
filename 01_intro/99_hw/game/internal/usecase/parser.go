package usecase

import "strings"

type Command struct {
	Name string
	P1   string
	P2   string
	P3   string
}

func Parse(raw string) Command {
	parts := strings.Fields(strings.TrimSpace(raw))
	get := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	return Command{
		Name: get(0),
		P1:   get(1),
		P2:   get(2),
		P3:   get(3),
	}
}
