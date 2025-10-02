package memory

import "game/internal/entities"

type Repo struct {
	state *entities.Game
}

func NewRepo() *Repo { return &Repo{} }

func (r *Repo) Load() *entities.Game {
	return r.state
}

func (r *Repo) Save(g *entities.Game) {
	r.state = g
}

func (r *Repo) Reset(g *entities.Game) {
	r.state = g
}
