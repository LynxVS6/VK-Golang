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
	kitchen.Items = map[string]map[string]bool{"столе": {"чай": true}}
	kitchen.ItemsOrder = []string{"столе"}

	room.Items = map[string]map[string]bool{"столе": {"ключи": true, "конспекты": true}, "стуле": {"рюкзак": true}}
	room.ItemsOrder = []string{"столе", "стуле"}

	corridor.Items = map[string]map[string]bool{}
	corridor.ItemsOrder = nil

	street.Items = map[string]map[string]bool{}
	street.ItemsOrder = nil

	// описания
	kitchen.EnterDescFn = func(gm *entities.Game) string {
		return "кухня, ничего интересного. " + kitchen.CanGoText()
	}
	kitchen.LookDescFn = func(gm *entities.Game) string {
		need := "надо собрать рюкзак и идти в универ."
		if gm.BackpackOn && gm.Inventory["ключи"] && gm.Inventory["конспекты"] {
			need = "надо идти в универ."
		}
		if hasAnyItems(kitchen) {
			return "ты находишься на кухне, " + contentsLine(kitchen) + ", " + need + " " + kitchen.CanGoText()
		}
		return "пустая комната, " + need + " " + kitchen.CanGoText()
	}

	corridor.EnterDescFn = func(gm *entities.Game) string {
		return "ничего интересного. " + corridor.CanGoText()
	}
	corridor.LookDescFn = corridor.EnterDescFn

	room.EnterDescFn = func(gm *entities.Game) string {
		return "ты в своей комнате. " + room.CanGoText()
	}
	room.LookDescFn = func(gm *entities.Game) string {
		if !hasAnyItems(room) {
			return "пустая комната. " + room.CanGoText()
		}
		return contentsLine(room) + ". " + room.CanGoText()
	}
	street.EnterDescFn = func(gm *entities.Game) string {
		return "на улице весна. " + street.CanGoText()
	}
	street.LookDescFn = street.EnterDescFn

	corridor.SpecialApply = func(gm *entities.Game, item, target string) (bool, string) {
		if target == "дверь" && item == "ключи" {
			if gm.DoorOpened {
				return true, "дверь уже открыта"
			}
			gm.DoorOpened = true
			return true, "дверь открыта"
		}
		return false, "не к чему применить"
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
