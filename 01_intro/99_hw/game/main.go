package main

import (
	"sort"
	"strings"
)

// --- entities ---
type Room struct {
	Name           string
	EnterDescFn    func(*Game) string
	LookDescFn     func(*Game) string
	NeighborsOrder []string
	Neighbors      map[string]*Room
	Items          map[string]map[string]bool
	ItemsOrder     []string
	SpecialApply   func(*Game, string, string) (bool, string)
}

type Game struct {
	Rooms      map[string]*Room
	Current    *Room
	Inventory  map[string]bool
	BackpackOn bool
	DoorOpened bool
}

// --- in-memory repo ---
type Repo struct{ state *Game }

func NewRepo() *Repo          { return &Repo{} }
func (r *Repo) Load() *Game   { return r.state }
func (r *Repo) Save(g *Game)  { r.state = g }
func (r *Repo) Reset(g *Game) { r.state = g }

// --- parser ---
type Command struct{ Name, P1, P2, P3 string }

func Parse(raw string) Command {
	parts := strings.Fields(strings.TrimSpace(raw))
	get := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	return Command{get(0), get(1), get(2), get(3)}
}

// --- usecase/service (monolithic) ---
type GameService struct{ repo *Repo }

func NewGameService(r *Repo) *GameService { return &GameService{repo: r} }
func (s *GameService) Init(g *Game)       { s.repo.Reset(g) }
func (s *GameService) Handle(raw string) string {
	cmd := Parse(raw)
	g := s.repo.Load()
	switch cmd.Name {
	case "осмотреться":
		return g.Current.LookDescFn(g)
	case "идти":
		if g.Current.Name == "коридор" && cmd.P1 == "улица" && !g.DoorOpened {
			return "дверь закрыта"
		}
		next, ok := g.Current.Neighbors[cmd.P1]
		if !ok {
			return "нет пути в " + cmd.P1
		}
		g.Current = next
		s.repo.Save(g)
		return next.EnterDescFn(g)
	case "надеть":
		if cmd.P1 == "" {
			return "неизвестная команда"
		}
		if cmd.P1 != "рюкзак" {
			return "нет такого"
		}
		for _, items := range g.Current.Items {
			if items != nil && items["рюкзак"] {
				delete(items, "рюкзак")
				g.BackpackOn = true
				s.repo.Save(g)
				return "вы надели: рюкзак"
			}
		}
		return "нет такого"
	case "взять":
		if cmd.P1 == "" {
			return "неизвестная команда"
		}
		if !g.BackpackOn {
			return "некуда класть"
		}
		found := false
		for _, items := range g.Current.Items {
			if items != nil && items[cmd.P1] {
				delete(items, cmd.P1)
				found = true
				break
			}
		}
		if !found {
			return "нет такого"
		}
		g.Inventory[cmd.P1] = true
		s.repo.Save(g)
		return "предмет добавлен в инвентарь: " + cmd.P1
	case "применить":
		if cmd.P1 == "" || cmd.P2 == "" {
			return "неизвестная команда"
		}
		if !g.Inventory[cmd.P1] {
			return "нет предмета в инвентаре - " + cmd.P1
		}
		if g.Current.SpecialApply != nil {
			if handled, text := g.Current.SpecialApply(g, cmd.P1, cmd.P2); handled {
				s.repo.Save(g)
				return text
			}
		}
		return "не к чему применить"
	default:
		return "неизвестная команда"
	}
}

// --- world builder (monolithic) ---
func Build() *Game {
	g := &Game{Rooms: map[string]*Room{}, Inventory: map[string]bool{}, BackpackOn: false, DoorOpened: false}
	kitchen := &Room{Name: "кухня"}
	corridor := &Room{Name: "коридор"}
	room := &Room{Name: "комната"}
	street := &Room{Name: "улица"}

	kitchen.Neighbors = map[string]*Room{"коридор": corridor}
	kitchen.NeighborsOrder = []string{"коридор"}
	corridor.Neighbors = map[string]*Room{"кухня": kitchen, "комната": room, "улица": street}
	corridor.NeighborsOrder = []string{"кухня", "комната", "улица"}
	room.Neighbors = map[string]*Room{"коридор": corridor}
	room.NeighborsOrder = []string{"коридор"}
	street.Neighbors = map[string]*Room{"домой": corridor}
	street.NeighborsOrder = []string{"домой"}

	kitchen.Items = map[string]map[string]bool{"столе": {"чай": true}}
	kitchen.ItemsOrder = []string{"столе"}
	room.Items = map[string]map[string]bool{"столе": {"ключи": true, "конспекты": true}, "стуле": {"рюкзак": true}}
	room.ItemsOrder = []string{"столе", "стуле"}
	corridor.Items = map[string]map[string]bool{}
	corridor.ItemsOrder = nil
	street.Items = map[string]map[string]bool{}
	street.ItemsOrder = nil

	kitchen.EnterDescFn = func(gm *Game) string {
		return "кухня, ничего интересного. " + kitchen.CanGoText()
	}
	kitchen.LookDescFn = func(gm *Game) string {
		need := "надо собрать рюкзак и идти в универ."
		if gm.BackpackOn && gm.Inventory["ключи"] && gm.Inventory["конспекты"] {
			need = "надо идти в универ."
		}
		if hasAnyItems(kitchen) {
			return "ты находишься на кухне, " + contentsLine(kitchen) + ", " + need + " " + kitchen.CanGoText()
		}
		return "пустая комната, " + need + " " + kitchen.CanGoText()
	}
	corridor.EnterDescFn = func(gm *Game) string { return "ничего интересного. " + corridor.CanGoText() }
	corridor.LookDescFn = corridor.EnterDescFn
	room.EnterDescFn = func(gm *Game) string { return "ты в своей комнате. " + room.CanGoText() }
	room.LookDescFn = func(gm *Game) string {
		if !hasAnyItems(room) {
			return "пустая комната. " + room.CanGoText()
		}
		return contentsLine(room) + ". " + room.CanGoText()
	}
	street.EnterDescFn = func(gm *Game) string { return "на улице весна. " + street.CanGoText() }
	street.LookDescFn = street.EnterDescFn

	corridor.SpecialApply = func(gm *Game, item, target string) (bool, string) {
		if target == "дверь" && item == "ключи" {
			if gm.DoorOpened {
				return true, "дверь уже открыта"
			}
			gm.DoorOpened = true
			return true, "дверь открыта"
		}
		return false, "не к чему применить"
	}

	g.Rooms = map[string]*Room{"кухня": kitchen, "коридор": corridor, "комната": room, "улица": street}
	g.Current = kitchen
	return g
}

// --- helper functions ---
func hasAnyItems(r *Room) bool {
	for _, items := range r.Items {
		for _, ok := range items {
			if ok {
				return true
			}
		}
	}
	return false
}

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

func contentsLine(r *Room) string {
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

func (r *Room) CanGoText() string {
	return "можно пройти - " + joinWithComma(r.NeighborsOrder)
}

func joinWithComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// --- CLI service wiring for tests ---
var (
	_repo = NewRepo()
	_svc  = NewGameService(_repo)
)

func initGame()                       { _svc.Init(Build()) }
func handleCommand(cmd string) string { return _svc.Handle(cmd) }

func main() { initGame() }
