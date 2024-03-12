package main

import (
	"fmt"
	"strings"
	"bufio"
	"os"
)

type RoomInfo struct {
	items      []Item
	directions []string
	doorIsOpen bool
}
type Person struct {
	bag         bool
	inventory   []string
	currentRoom string
}

type Item struct {
	name    string
	objects []string
}

var rooms, person = initGame()

const noCorrectCommand = "некорректная команда"

func printDirections(answer *string, directions []string) {
	for i := range directions {
		if i == len(directions) - 1 {
			*answer += directions[i]
		} else {
			*answer += fmt.Sprintf("%s, ", directions[i])
		}
	}
}

func printObjects(answer *string, objects []string, endFlag bool) {
	for i := range objects {
		if i == len(objects) - 1 && endFlag {
			*answer += fmt.Sprintf("%s. ", objects[i])
		} else {
			*answer += fmt.Sprintf("%s, ", objects[i])
		}
	}
}

func find(value string, array []string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
}

func (item *Item) deleteObject(i int) {
	item.objects = append(item.objects[:i], item.objects[i+1:]...)
}

func (room *RoomInfo) deleteItem(i int) {
	room.items = append(room.items[:i], room.items[i+1:]...)
}

func lookAround(answer *string) {
	if person.currentRoom == "кухня" {
		*answer += "ты находишься на кухне, "
		endFlag := false
		roomItems := rooms[person.currentRoom].items

		for i := range roomItems {
			*answer += fmt.Sprintf("на %sе: ", roomItems[i].name)
			printObjects(answer, roomItems[i].objects, endFlag)
		}

		if person.bag {
			*answer += "надо идти в универ. "
		} else {
			*answer += "надо собрать рюкзак и идти в универ. "
		}

	} else {
		endFlag := false
		roomItems := rooms[person.currentRoom].items

		if len(roomItems) == 0 {
			*answer += "пустая комната. "
		}

		for i := range roomItems {
			*answer += fmt.Sprintf("на %sе: ", roomItems[i].name)
		
			if i == len(roomItems) - 1 {
				endFlag = true
			}
			printObjects(answer, roomItems[i].objects, endFlag)
		}
	}

	*answer += "можно пройти - "
	printDirections(answer, rooms[person.currentRoom].directions)
}

func move(answer *string, room string) {
	switch room {
	case "коридор": 
		person.currentRoom = room
		*answer += "ничего интересного. можно пройти - "
		printDirections(answer, rooms[room].directions)

	case "комната":
		person.currentRoom = room
		*answer += "ты в своей комнате. можно пройти - "
		printDirections(answer, rooms[room].directions)

	case "кухня":
		person.currentRoom = room
		*answer += "кухня, ничего интересного. можно пройти - "
		printDirections(answer, rooms[room].directions)

	case "улица":
		if person.currentRoom == "коридор" && rooms["коридор"].doorIsOpen {
			person.currentRoom = room
			*answer += "на улице весна. "
			*answer += "можно пройти - "
			printDirections(answer, rooms[person.currentRoom].directions)
			
			rooms, person = initGame()
		} else {
			*answer += "дверь закрыта"
		}

	default:
		*answer = "введенной комнаты в игре нет"
	}
}

func putOn(answer *string, item string) {
	if item == "рюкзак" {
		enabledObject := false
		for i := range rooms[person.currentRoom].items {
			for j, object := range rooms[person.currentRoom].items[i].objects {
				if object == item {
					enabledObject = true
					rooms[person.currentRoom].items[i].deleteObject(j)
				}
			}

			if len(rooms[person.currentRoom].items[i].objects) == 0 {
				rooms[person.currentRoom].deleteItem(i)
			}
		}

		if enabledObject {
			person.bag = true
		}
	}

	*answer += fmt.Sprintf("вы надели: %s", item)
}

func take(answer *string, item string) {
	if !person.bag {
		*answer += "некуда класть"
	} else {
		enabledObject := false
		for i := range rooms[person.currentRoom].items {
			for j, object := range rooms[person.currentRoom].items[i].objects {
				if object == item {
					enabledObject = true
					rooms[person.currentRoom].items[i].deleteObject(j)
				}
			}

			if len(rooms[person.currentRoom].items[i].objects) == 0 {
				rooms[person.currentRoom].deleteItem(i)
			}
		}

		if enabledObject {
			person.inventory = append(person.inventory, item)
			*answer += fmt.Sprintf("предмет добавлен в инвентарь: %s", item)
		} else {
			*answer += "нет такого"
		}
	}
}

func apply(answer *string, key, object string) {
	if key == "ключи" && object == "дверь" {
		rooms[person.currentRoom].doorIsOpen = true
		*answer += "дверь открыта"
	} else {
		*answer += "не к чему применить"
	}
} 

func main() {
	sc := bufio.NewScanner(os.Stdin)
	fmt.Println("Введите команду:")

	for sc.Scan() {
		command := sc.Text()
		fmt.Printf("%s\n", handleCommand(command))
	}
}

func initGame() (map[string]*RoomInfo, Person) {
	newRooms := make(map[string]*RoomInfo)

	newRooms["кухня"] = &RoomInfo{
		items: []Item{
			{
				name:    "стол",
				objects: []string{"чай"},
			},
		},
		directions: []string{"коридор"},
		doorIsOpen: true,
	}

	newRooms["коридор"] = &RoomInfo{
		items:      []Item{},
		directions: []string{"кухня", "комната", "улица"},
		doorIsOpen: false,
	}

	newRooms["комната"] = &RoomInfo{
		items: []Item{
			{
				name:    "стол",
				objects: []string{"ключи", "конспекты"},
			},
			{
				name:    "стул",
				objects: []string{"рюкзак"},
			},
		},
		directions: []string{"коридор"},
		doorIsOpen: true,
	}

	newRooms["улица"] = &RoomInfo{
		items:      []Item{},
		directions: []string{"домой"},
	}

	newRooms["дом"] = &RoomInfo{
		items:      []Item{},
		directions: []string{"коридор"},
	}

	newPerson := Person{
		bag:         false,
		inventory:   []string{},
		currentRoom: "кухня",
	}

	return newRooms, newPerson
}

func handleCommand(command string) string {
	var answer string
	parameters := strings.Split(command, " ")
	cmd := parameters[0]

	switch cmd {
	case "осмотреться":
		if len(parameters) != 1 {
			return noCorrectCommand
		}
		lookAround(&answer)

	case "идти":
		if len(parameters) != 2 {
			return noCorrectCommand
		}
		if !find(parameters[1], rooms[person.currentRoom].directions) {
			return fmt.Sprintf("нет пути в %s", parameters[1])
		}
		move(&answer, parameters[1])

	case "надеть":
		if len(parameters) != 2 {
			return noCorrectCommand
		}
		putOn(&answer, parameters[1])

	case "взять":
		if len(parameters) != 2 {
			return noCorrectCommand
		}
		take(&answer, parameters[1])

	case "применить":
		if len(parameters) != 3 {
			return noCorrectCommand
		}
		if !find(parameters[1], person.inventory) {
			return fmt.Sprintf("нет предмета в инвентаре - %s", parameters[1])
		}
		apply(&answer, parameters[1], parameters[2])

	default: 
		answer = "неизвестная команда"
	}

	return answer
}