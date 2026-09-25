package ui

type Tile struct {
	OpenN bool
	OpenS bool
	OpenE bool
	OpenW bool
}

type GameState struct {
	Map       [][]Tile
	PlayerX   int
	PlayerY   int
	GoalX     int
	GoalY     int
	Message   string
	Running   bool
	Remaining int
	DiceTotal int
	HasRolled bool
	HasSlid   bool
	SlideRows []int
	SlideCols []int
}
