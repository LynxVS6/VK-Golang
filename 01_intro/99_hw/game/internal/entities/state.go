package entities

type Game struct {
	Rooms      map[string]*Room
	Current    *Room
	Inventory  map[string]bool
	BackpackOn bool
	DoorOpened bool
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