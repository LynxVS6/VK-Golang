package entities

type SpecialApplyFunc func(g *Game, item, target string) (handled bool, text string)

type Room struct {
	Name           string
	EnterDescFn    func(*Game) string
	LookDescFn     func(*Game) string
	NeighborsOrder []string
	Neighbors      map[string]*Room
	Items          map[string]bool
	SpecialApply   SpecialApplyFunc
}

func (r *Room) CanGoText() string {
	return "можно пройти - " + joinWithComma(r.NeighborsOrder)
}