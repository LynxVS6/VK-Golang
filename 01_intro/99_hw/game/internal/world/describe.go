package world

import (
	"sort"
	"strings"

	"game/internal/entities"
)

// есть ли хоть одна вещь в комнате
func hasAnyItems(r *entities.Room) bool {
	for _, items := range r.Items {
		for _, ok := range items {
			if ok {
				return true
			}
		}
	}
	return false
}

// собрать отсортированный список предметов на конкретной поверхности
func namesSlice(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, ok := range m {
		if ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// сформировать строку вида: "на столе: ключи, конспекты, на стуле: рюкзак"
func contentsLine(r *entities.Room) string {
	parts := []string{}
	for _, surface := range r.ItemsOrder {
		items := r.Items[surface]
		if items == nil {
			continue
		}
		names := namesSlice(items)
		if len(names) == 0 {
			continue
		}
		parts = append(parts, "на "+surface+": "+strings.Join(names, ", "))
	}
	return strings.Join(parts, ", ")
}
