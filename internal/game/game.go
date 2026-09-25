package game

import (
	"math/rand"
	"sort"

	"github.com/giulianozor/dungeon/internal/ui"
)

type Game struct {
	state ui.GameState
}

var tileConfigs = []ui.Tile{
	{OpenN: true},
	{OpenS: true},
	{OpenE: true},
	{OpenW: true},
	{OpenN: true, OpenS: true},
	{OpenE: true, OpenW: true},
	{OpenN: true, OpenE: true},
	{OpenN: true, OpenW: true},
	{OpenS: true, OpenE: true},
	{OpenS: true, OpenW: true},
	{OpenN: true, OpenS: true, OpenE: true},
	{OpenN: true, OpenS: true, OpenW: true},
	{OpenN: true, OpenE: true, OpenW: true},
	{OpenS: true, OpenE: true, OpenW: true},
	{OpenN: true, OpenS: true, OpenE: true, OpenW: true},
}

var tilesByCount = func() map[int][]ui.Tile {
	m := map[int][]ui.Tile{}
	for _, t := range tileConfigs {
		n := 0
		if t.OpenN {
			n++
		}
		if t.OpenS {
			n++
		}
		if t.OpenE {
			n++
		}
		if t.OpenW {
			n++
		}
		m[n] = append(m[n], t)
	}
	return m
}()

func pickCount() int {
	r := rand.Intn(109)
	switch {
	case r < 10:
		return 1
	case r < 59:
		return 2
	case r < 99:
		return 3
	default:
		return 4
	}
}

func pickTile(forbid func(ui.Tile) bool) ui.Tile {
	for attempt := 0; attempt < 20; attempt++ {
		cnt := pickCount()
		candidates := tilesByCount[cnt]
		if forbid == nil {
			return candidates[rand.Intn(len(candidates))]
		}
		var valid []ui.Tile
		for _, t := range candidates {
			if !forbid(t) {
				valid = append(valid, t)
			}
		}
		if len(valid) > 0 {
			return valid[rand.Intn(len(valid))]
		}
	}
	return ui.Tile{}
}

func New(width, height int) *Game {
	if width < 3 || height < 3 {
		width = 10
		height = 7
	}
	m := make([][]ui.Tile, height)
	for y := range m {
		m[y] = make([]ui.Tile, width)
		for x := range m[y] {
			switch {
			case x == 0 || x == width-1 || y == 0 || y == height-1:
				t := pickTile(func(t ui.Tile) bool {
					return (y == 0 && t.OpenN) || (y == height-1 && t.OpenS) ||
						(x == 0 && t.OpenW) || (x == width-1 && t.OpenE)
				})
				m[y][x] = t
			default:
				m[y][x] = pickTile(nil)
			}
		}
	}

	corners := [][2]int{
		{0, 0},
		{width - 1, 0},
		{0, height - 1},
		{width - 1, height - 1},
	}
	start := corners[rand.Intn(len(corners))]

	ensureConnected(m, width, height)

	g := &Game{
		state: ui.GameState{
			Map:     m,
			PlayerX: start[0], PlayerY: start[1],
			GoalX: width / 2, GoalY: height / 2,
			Running: true,
		},
	}
	g.RollDice()
	g.randomizeSlides()
	return g
}

// randomizeSlides picks a random subset of the slideable rows and columns for
// the current turn. The total number of slideable lines stays fixed.
func (g *Game) randomizeSlides() {
	w, h := len(g.state.Map[0]), len(g.state.Map)
	var rows, cols []int
	for y := 1; y < h; y += 2 {
		rows = append(rows, y)
	}
	for x := 1; x < w; x += 2 {
		cols = append(cols, x)
	}

	type line struct {
		kind string
		idx  int
	}
	pool := make([]line, 0, len(rows)+len(cols))
	for _, y := range rows {
		pool = append(pool, line{"row", y})
	}
	for _, x := range cols {
		pool = append(pool, line{"col", x})
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	n := len(pool) / 2
	g.state.SlideRows = g.state.SlideRows[:0]
	g.state.SlideCols = g.state.SlideCols[:0]
	for _, l := range pool[:n] {
		if l.kind == "row" {
			g.state.SlideRows = append(g.state.SlideRows, l.idx)
		} else {
			g.state.SlideCols = append(g.state.SlideCols, l.idx)
		}
	}
	sort.Ints(g.state.SlideRows)
	sort.Ints(g.state.SlideCols)
}

func (g *Game) slideAllowed(row, col int) bool {
	for _, y := range g.state.SlideRows {
		if y == row {
			return true
		}
	}
	for _, x := range g.state.SlideCols {
		if x == col {
			return true
		}
	}
	return false
}

func (g *Game) tryMove(dx, dy int) {
	nx, ny := g.state.PlayerX+dx, g.state.PlayerY+dy

	w, h := len(g.state.Map[0]), len(g.state.Map)
	ny = ((ny % h) + h) % h
	nx = ((nx % w) + w) % w

	if nx == g.state.PlayerX && ny == g.state.PlayerY {
		return
	}

	curr := g.state.Map[g.state.PlayerY][g.state.PlayerX]
	next := g.state.Map[ny][nx]

	if !canMoveBetween(curr, next, dx, dy) {
		return
	}

	g.state.PlayerX, g.state.PlayerY = nx, ny
	g.state.Remaining--

	if nx == g.state.GoalX && ny == g.state.GoalY {
		g.state.Message = "You win!"
		g.state.Running = false
	}
}

func canMoveBetween(curr, next ui.Tile, dx, dy int) bool {
	switch {
	case dx == 0 && dy == -1:
		return curr.OpenN && next.OpenS
	case dx == 0 && dy == 1:
		return curr.OpenS && next.OpenN
	case dx == 1 && dy == 0:
		return curr.OpenE && next.OpenW
	case dx == -1 && dy == 0:
		return curr.OpenW && next.OpenE
	}
	return false
}

// ensureConnected opens passages until every cell of the map is reachable
// from every other cell without using border wrap (border tiles never have
// outward openings, so wrap movement is effectively disabled). The map is
// undirected, so a single connected component guarantees that any corner
// start can reach any interior goal.
func ensureConnected(m [][]ui.Tile, w, h int) {
	dirs := [][2]int{{0, 1}, {0, -1}, {-1, 0}, {1, 0}}
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}

	queue := [][2]int{{0, 0}}
	visited[0][0] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		cy, cx := cur[0], cur[1]

		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h || visited[ny][nx] {
				continue
			}
			if !canMoveBetween(m[cy][cx], m[ny][nx], d[0], d[1]) {
				continue
			}
			visited[ny][nx] = true
			queue = append(queue, [2]int{ny, nx})
		}

		if len(queue) == 0 {
			// The visited set is a full component. Pick an unvisited cell
			// that touches it and open the passage between them, then resume
			// the flood from that cell.
			var linkC, linkN [2]int
			found := false
			for y := 0; y < h && !found; y++ {
				for x := 0; x < w && !found; x++ {
					if visited[y][x] {
						continue
					}
					for _, d := range dirs {
						nx, ny := x+d[0], y+d[1]
						if nx < 0 || ny < 0 || nx >= w || ny >= h || !visited[ny][nx] {
							continue
						}
						linkC = [2]int{y, x}
						linkN = [2]int{ny, nx}
						found = true
						break
					}
				}
			}
			if !found {
				return
			}
			forceCross(&m[linkC[0]][linkC[1]], &m[linkN[0]][linkN[1]], linkN[1]-linkC[1], linkN[0]-linkC[0])
			visited[linkC[0]][linkC[1]] = true
			queue = append(queue, linkC)
		}
	}
}

func forceCross(a, b *ui.Tile, dx, dy int) {
	switch {
	case dx == 0 && dy == -1:
		a.OpenN = true
		b.OpenS = true
	case dx == 0 && dy == 1:
		a.OpenS = true
		b.OpenN = true
	case dx == 1 && dy == 0:
		a.OpenE = true
		b.OpenW = true
	case dx == -1 && dy == 0:
		a.OpenW = true
		b.OpenE = true
	}
}

func (g *Game) teleportPlayer() {
	width := len(g.state.Map[0])
	height := len(g.state.Map)

	for i := 0; i < 100; i++ {
		x, y := rand.Intn(width-2)+1, rand.Intn(height-2)+1
		if (x != g.state.GoalX || y != g.state.GoalY) && (x != g.state.PlayerX || y != g.state.PlayerY) {
			g.state.PlayerX, g.state.PlayerY = x, y
			g.state.Message = "Poof! Teleported!"
			return
		}
	}
}

func (g *Game) SlideRow(row int, right bool) {
	if !g.state.Running || g.state.HasSlid || row < 0 || row >= len(g.state.Map) || row%2 == 0 || !g.slideAllowed(row, -1) {
		return
	}
	g.state.HasSlid = true
	w := len(g.state.Map[row])
	if right {
		last := g.state.Map[row][w-1]
		copy(g.state.Map[row][1:], g.state.Map[row][:w-1])
		g.state.Map[row][0] = last
		if g.state.PlayerY == row {
			g.state.PlayerX = (g.state.PlayerX + 1) % w
		}
		if g.state.GoalY == row {
			g.state.GoalX = (g.state.GoalX + 1) % w
		}
	} else {
		first := g.state.Map[row][0]
		copy(g.state.Map[row][:w-1], g.state.Map[row][1:])
		g.state.Map[row][w-1] = first
		if g.state.PlayerY == row {
			g.state.PlayerX = (g.state.PlayerX - 1 + w) % w
		}
		if g.state.GoalY == row {
			g.state.GoalX = (g.state.GoalX - 1 + w) % w
		}
	}
}

func (g *Game) SlideCol(col int, down bool) {
	if !g.state.Running || g.state.HasSlid || col < 0 || col >= len(g.state.Map[0]) || col%2 == 0 || !g.slideAllowed(-1, col) {
		return
	}
	g.state.HasSlid = true
	h := len(g.state.Map)
	colVals := make([]ui.Tile, h)
	for y := range colVals {
		colVals[y] = g.state.Map[y][col]
	}
	if down {
		last := colVals[h-1]
		copy(colVals[1:], colVals[:h-1])
		colVals[0] = last
	} else {
		first := colVals[0]
		copy(colVals[:h-1], colVals[1:])
		colVals[h-1] = first
	}
	for y := range colVals {
		g.state.Map[y][col] = colVals[y]
	}
	if g.state.PlayerX == col {
		if down {
			g.state.PlayerY = (g.state.PlayerY + 1) % h
		} else {
			g.state.PlayerY = (g.state.PlayerY - 1 + h) % h
		}
	}
	if g.state.GoalX == col {
		if down {
			g.state.GoalY = (g.state.GoalY + 1) % h
		} else {
			g.state.GoalY = (g.state.GoalY - 1 + h) % h
		}
	}
}

func (g *Game) RollDice() (int, int) {
	if !g.state.Running || g.state.HasRolled {
		return 0, 0
	}
	d1 := 1 + rand.Intn(6)
	d2 := 1 + rand.Intn(6)
	g.state.Remaining = d1 + d2
	g.state.DiceTotal = d1 + d2
	g.state.HasRolled = true
	return d1, d2
}

func (g *Game) Move(dx, dy int) {
	if !g.state.Running || g.state.Remaining <= 0 {
		return
	}
	g.tryMove(dx, dy)
}

func (g *Game) MoveTo(tx, ty int) {
	if !g.state.Running {
		return
	}
	path := g.shortestPath(tx, ty)
	if len(path) < 2 || len(path)-1 > g.state.Remaining {
		return
	}
	g.state.PlayerX, g.state.PlayerY = tx, ty
	g.state.Remaining -= len(path) - 1
	if tx == g.state.GoalX && ty == g.state.GoalY {
		g.state.Message = "You win!"
		g.state.Running = false
	}
}

func (g *Game) shortestPath(tx, ty int) [][2]int {
	h, w := len(g.state.Map), len(g.state.Map[0])
	visited := make([][]bool, h)
	prev := make([][][2]int, h)
	for y := range visited {
		visited[y] = make([]bool, w)
		prev[y] = make([][2]int, w)
	}

	queue := [][2]int{{g.state.PlayerY, g.state.PlayerX}}
	visited[g.state.PlayerY][g.state.PlayerX] = true

	for len(queue) > 0 {
		cy, cx := queue[0][0], queue[0][1]
		queue = queue[1:]

		if cy == ty && cx == tx {
			var path [][2]int
			for cy != g.state.PlayerY || cx != g.state.PlayerX {
				path = append([][2]int{{cx, cy}}, path...)
				p := prev[cy][cx]
				cy, cx = p[0], p[1]
			}
			path = append([][2]int{{g.state.PlayerX, g.state.PlayerY}}, path...)
			return path
		}

		dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
		for _, d := range dirs {
			nx := (cx + d[0] + w) % w
			ny := (cy + d[1] + h) % h
			if nx == cx && ny == cy {
				continue
			}
			if visited[ny][nx] {
				continue
			}

			if !canMoveBetween(g.state.Map[cy][cx], g.state.Map[ny][nx], d[0], d[1]) {
				continue
			}

			visited[ny][nx] = true
			prev[ny][nx] = [2]int{cy, cx}
			queue = append(queue, [2]int{ny, nx})
		}
	}
	return nil
}

func (g *Game) Reachable(limit int) [][]bool {
	h, w := len(g.state.Map), len(g.state.Map[0])
	reachable := make([][]bool, h)
	for y := range reachable {
		reachable[y] = make([]bool, w)
	}

	if limit <= 0 {
		return reachable
	}

	type node struct{ y, x, dist int }
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}

	queue := []node{{g.state.PlayerY, g.state.PlayerX, 0}}
	visited[g.state.PlayerY][g.state.PlayerX] = true
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.dist > 0 {
			reachable[cur.y][cur.x] = true
		}
		if cur.dist >= limit {
			continue
		}

		for _, d := range dirs {
			nx := ((cur.x + d[0]) + w) % w
			ny := ((cur.y + d[1]) + h) % h
			if nx == cur.x && ny == cur.y {
				continue
			}
			if visited[ny][nx] {
				continue
			}

			if !canMoveBetween(g.state.Map[cur.y][cur.x], g.state.Map[ny][nx], d[0], d[1]) {
				continue
			}

			visited[ny][nx] = true
			queue = append(queue, node{ny, nx, cur.dist + 1})
		}
	}
	return reachable
}

func (g *Game) EndTurn() {
	if !g.state.Running {
		return
	}
	g.state.Remaining = 0
	g.state.DiceTotal = 0
	g.state.HasRolled = false
	g.state.HasSlid = false
	g.state.Message = ""
	g.RollDice()
	g.randomizeSlides()
}

func (g *Game) NewRound() {
	w, h := len(g.state.Map[0]), len(g.state.Map)
	g.state.Running = true
	g.state.Message = ""
	g.state.Remaining = 0
	g.state.DiceTotal = 0
	g.state.HasRolled = false
	g.state.HasSlid = false

	// When the goal is caught the player keeps their position; only the
	// goal moves to a new random interior cell.
	for i := 0; i < 100; i++ {
		x := rand.Intn(w-2) + 1
		y := rand.Intn(h-2) + 1
		if x != g.state.PlayerX || y != g.state.PlayerY {
			g.state.GoalX, g.state.GoalY = x, y
			break
		}
	}
	g.RollDice()
	g.randomizeSlides()
}

// SetToken installs a token position on the shared board. The room keeps
// each player's token position and installs the active player's before any
// movement or reachability computation.
func (g *Game) SetToken(x, y int) {
	g.state.PlayerX, g.state.PlayerY = x, y
}

// SetSlideLines overrides the set of rows/columns slideable this turn.
func (g *Game) SetSlideLines(rows, cols []int) {
	g.state.SlideRows = rows
	g.state.SlideCols = cols
}

func (g *Game) Teleport()           { g.teleportPlayer() }
func (g *Game) State() ui.GameState { return g.state }
