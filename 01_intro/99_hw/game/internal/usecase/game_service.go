package usecase

import "game/internal/entities"

type Repo interface {
	Load() *entities.Game
	Save(*entities.Game)
	Reset(*entities.Game)
}

type GameService struct {
	repo Repo
}

func NewGameService(r Repo) *GameService { return &GameService{repo: r} }

func (s *GameService) Init(g *entities.Game) {
	s.repo.Reset(g)
}

func (s *GameService) Handle(raw string) string {
	cmd := Parse(raw)
	g := s.repo.Load()

	switch cmd.Name {
	case "осмотреться":
		return g.Current.LookDescFn(g)
	case "идти":
		if cmd.P1 == "" {
			return "неизвестная команда"
		}

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
		if cmd.P1 != "рюкзак" {
			return "нет такого"
		}
		if g.Current.Items != nil && g.Current.Items["рюкзак"] {
			delete(g.Current.Items, "рюкзак")
			g.BackpackOn = true
			s.repo.Save(g)
			return "вы надели: рюкзак"
		}
		return "нет такого"

	case "взять":
		if cmd.P1 == "" {
			return "неизвестная команда"
		}
		if !g.BackpackOn {
			return "некуда класть"
		}
		if !(g.Current.Items != nil && g.Current.Items[cmd.P1]) {
			return "нет такого"
		}
		delete(g.Current.Items, cmd.P1)
		if g.Inventory == nil {
			g.Inventory = map[string]bool{}
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
