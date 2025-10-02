package world

import "game/internal/entities"

func Build() *entities.Game {
	g := &entities.Game{
		Rooms:      map[string]*entities.Room{},
		Inventory:  map[string]bool{},
		BackpackOn: false,
		DoorOpened: false,
	}

	kitchen := &entities.Room{Name: "кухня"}
	corridor := &entities.Room{Name: "коридор"}
	room := &entities.Room{Name: "комната"}
	street := &entities.Room{Name: "улица"}

	// граф комнат
	kitchen.Neighbors = map[string]*entities.Room{"коридор": corridor}
	kitchen.NeighborsOrder = []string{"коридор"}

	corridor.Neighbors = map[string]*entities.Room{"кухня": kitchen, "комната": room, "улица": street}
	corridor.NeighborsOrder = []string{"кухня", "комната", "улица"}

	room.Neighbors = map[string]*entities.Room{"коридор": corridor}
	room.NeighborsOrder = []string{"коридор"}

	street.Neighbors = map[string]*entities.Room{"домой": corridor}
	street.NeighborsOrder = []string{"домой"}

	// предметы
	kitchen.Items = map[string]bool{"чай": true}
	room.Items = map[string]bool{"ключи": true, "конспекты": true, "рюкзак": true}
	corridor.Items = map[string]bool{}
	street.Items = map[string]bool{}

	// описания
	kitchen.EnterDescFn = func(gm *entities.Game) string {
		return "кухня, ничего интересного. " + kitchen.CanGoText()
	}
	kitchen.LookDescFn = func(gm *entities.Game) string {
		if !gm.BackpackOn {
			return "ты находишься на кухне, на столе: чай, надо собрать рюкзак и идти в универ. " + kitchen.CanGoText()
		}
		return "ты находишься на кухне, на столе: чай, надо идти в универ. " + kitchen.CanGoText()
	}

	corridor.EnterDescFn = func(gm *entities.Game) string {
		return "ничего интересного. " + corridor.CanGoText()
	}
	corridor.LookDescFn = corridor.EnterDescFn

	room.EnterDescFn = func(gm *entities.Game) string {
		return "ты в своей комнате. " + room.CanGoText()
	}
	room.LookDescFn = func(gm *entities.Game) string {
		hasB := room.Items["рюкзак"]
		hasK := room.Items["ключи"]
		hasN := room.Items["конспекты"]
		switch {
		case hasB && hasK && hasN:
			return "на столе: ключи, конспекты, на стуле: рюкзак. " + room.CanGoText()
		case !hasB && hasK && hasN:
			return "на столе: ключи, конспекты. " + room.CanGoText()
		case !hasB && !hasK && hasN:
			return "на столе: конспекты. " + room.CanGoText()
		case !hasB && !hasK && !hasN:
			return "пустая комната. " + room.CanGoText()
		default:
			return "пустая комната. " + room.CanGoText()
		}
	}

	street.EnterDescFn = func(gm *entities.Game) string {
		return "на улице весна. " + street.CanGoText()
	}
	street.LookDescFn = street.EnterDescFn

	corridor.SpecialApply = func(gm *entities.Game, item, target string) (bool, string) {
		if target == "дверь" {
			if !gm.Inventory["ключи"] {
				return true, "нет предмета в инвентаре - ключи"
			}
			gm.DoorOpened = true
			return true, "дверь открыта"
		}
		return true, "не к чему применить"
	}

	g.Rooms = map[string]*entities.Room{
		"кухня":   kitchen,
		"коридор": corridor,
		"комната": room,
		"улица":   street,
	}
	g.Current = kitchen
	return g
}
