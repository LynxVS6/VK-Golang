package main

import (
	"sort"
	"strings"
)

type Room struct {
	name        string
	shortDesc   string
	items       map[string]string
	exits       map[string]*Room
	lockedExits map[string]bool
	actions     map[string]func([]string) string
}

type Player struct {
	currentRoom *Room
	inventory   map[string]bool
	wearing     map[string]bool
}

var (
	player *Player
	rooms  map[string]*Room
)

func newRoom(name, shortDesc string) *Room {
	return &Room{
		name:        name,
		shortDesc:   shortDesc,
		items:       make(map[string]string),
		exits:       make(map[string]*Room),
		lockedExits: make(map[string]bool),
		actions:     make(map[string]func([]string) string),
	}
}

func initGame() {
	setupRooms()
	setupActions()
	createPlayer()
}

func setupRooms() {
	kitchen := newRoom("кухня", "кухня, ничего интересного")
	corridor := newRoom("коридор", "ничего интересного")
	room := newRoom("комната", "ты в своей комнате")
	street := newRoom("улица", "на улице весна")

	kitchen.exits["коридор"] = corridor
	corridor.exits["кухня"] = kitchen
	corridor.exits["комната"] = room
	corridor.exits["улица"] = street
	room.exits["коридор"] = corridor
	street.exits["домой"] = corridor

	corridor.lockedExits["улица"] = true

	kitchen.items["чай"] = "на столе"
	room.items["ключи"] = "на столе"
	room.items["конспекты"] = "на столе"
	room.items["рюкзак"] = "на стуле"

	rooms = map[string]*Room{
		"кухня":   kitchen,
		"коридор": corridor,
		"комната": room,
		"улица":   street,
	}
}

func (r *Room) getExits() string {
	var exits []string
	if r.name == "коридор" {
		if _, ok := r.exits["кухня"]; ok {
			exits = append(exits, "кухня")
		}
		if _, ok := r.exits["комната"]; ok {
			exits = append(exits, "комната")
		}
		if _, ok := r.exits["улица"]; ok {
			exits = append(exits, "улица")
		}
	} else {
		for exit := range r.exits {
			exits = append(exits, exit)
		}
	}
	return strings.Join(exits, ", ")
}

func (r *Room) goFunc() func([]string) string {
	return func(args []string) string {
		if len(args) < 1 {
			return "неизвестная команда"
		}
		dest := args[0]
		if next, ok := r.exits[dest]; ok {
			if r.lockedExits[dest] {
				return "дверь закрыта"
			}
			player.currentRoom = next
			return next.shortDesc + ". можно пройти - " + next.getExits()
		}
		return "нет пути в " + dest
	}
}

func (r *Room) takeFunc() func([]string) string {
	return func(args []string) string {
		if len(args) < 1 {
			return "неизвестная команда"
		}
		item := args[0]
		if !player.wearing["рюкзак"] {
			return "некуда класть"
		}
		if _, ok := r.items[item]; !ok {
			return "нет такого"
		}
		player.inventory[item] = true
		delete(r.items, item)
		return "предмет добавлен в инвентарь: " + item
	}
}

func (r *Room) setupKitchenActions() {
	r.actions["осмотреться"] = func(args []string) string {
		base := "ты находишься на кухне"
		itemsStr := ""
		if _, hasTea := r.items["чай"]; hasTea {
			itemsStr = ", на столе: чай"
		}
		special := ", надо собрать рюкзак и идти в универ"
		if player.wearing["рюкзак"] {
			special = ", надо идти в универ"
		}
		return base + itemsStr + special + ". можно пройти - " + r.getExits()
	}
	r.actions["идти"] = r.goFunc()
	r.actions["взять"] = r.takeFunc()
}

func (r *Room) setupCorridorActions() {
	r.actions["осмотреться"] = func(args []string) string {
		return r.shortDesc + ". можно пройти - " + r.getExits()
	}
	r.actions["идти"] = r.goFunc()
	r.actions["применить"] = func(args []string) string {
		if len(args) < 2 {
			return "неизвестная команда"
		}
		item, target := args[0], args[1]
		if !player.inventory[item] {
			return "нет предмета в инвентаре - " + item
		}
		if target == "дверь" && item == "ключи" {
			if r.lockedExits["улица"] {
				r.lockedExits["улица"] = false
				return "дверь открыта"
			}
			return "дверь уже открыта"
		}
		return "не к чему применить"
	}
}

func (r *Room) setupRoomActionsFunc() {
	r.actions["осмотреться"] = func(args []string) string {
		if len(r.items) == 0 {
			return "пустая комната. можно пройти - " + r.getExits()
		}
		tableItems := []string{}
		backpackStr := ""
		for item, loc := range r.items {
			if loc == "на столе" {
				tableItems = append(tableItems, item)
			} else if item == "рюкзак" && loc == "на стуле" {
				backpackStr = ", на стуле: рюкзак"
			}
		}
		sort.Strings(tableItems)
		itemsStr := ""
		if len(tableItems) > 0 {
			itemsStr = "на столе: " + strings.Join(tableItems, ", ")
		}
		itemsStr += backpackStr
		return itemsStr + ". можно пройти - " + r.getExits()
	}
	r.actions["идти"] = r.goFunc()
	r.actions["взять"] = r.takeFunc()
	r.actions["надеть"] = func(args []string) string {
		if len(args) < 1 {
			return "неизвестная команда"
		}
		item := args[0]
		if _, ok := r.items[item]; !ok {
			return "нет такого"
		}
		if item != "рюкзак" {
			return "нельзя надеть этот предмет"
		}
		player.wearing[item] = true
		delete(r.items, item)
		return "вы надели: " + item
	}
}

func (r *Room) setupStreetActions() {
	r.actions["осмотреться"] = func(args []string) string {
		return r.shortDesc + ". можно пройти - " + r.getExits()
	}
	r.actions["идти"] = r.goFunc()
}

func setupActions() {
	rooms["кухня"].setupKitchenActions()
	rooms["коридор"].setupCorridorActions()
	rooms["комната"].setupRoomActionsFunc()
	rooms["улица"].setupStreetActions()
}

func createPlayer() {
	player = &Player{
		currentRoom: rooms["кухня"],
		inventory:   make(map[string]bool),
		wearing:     make(map[string]bool),
	}
}

func handleCommand(command string) string {
	parts := strings.Split(command, " ")
	if len(parts) == 0 {
		return "неизвестная команда"
	}
	cmd := parts[0]
	args := parts[1:]
	if action, ok := player.currentRoom.actions[cmd]; ok {
		return action(args)
	}
	return "неизвестная команда"
}

func main() {
	initGame()
}
