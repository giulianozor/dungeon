package main

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/giulianozor/dungeon/internal/game"
)

type player struct {
	pid      string
	name     string
	symbol   string
	seat     int
	x, y     int
	score    int
	lastSeen time.Time
}

// playerSymbols is the pool of distinct markers shown on the shared board.
var playerSymbols = []string{"@", "#", "%", "&", "*", "+", "$", "?"}

type chatMsg struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
}

const chatLimit = 100

type room struct {
	mu           sync.Mutex
	id           string
	mode         string
	maxPlayers   int
	goalLimit    int
	players      []*player
	byPid        map[string]*player
	started      bool
	finished     bool
	curTurn      int
	totalCatches int
	messages     []chatMsg
	nextMsgID    int
	board        *game.Game
}

type roomManager struct {
	mu    sync.Mutex
	rooms map[string]*room
}

func newRoomManager() *roomManager {
	return &roomManager{rooms: make(map[string]*room)}
}

// idAlphabet is alphanumeric (uppercase letters + digits) with easily
// confused characters (O, 0, I, 1, l) removed so IDs are unambiguous.
const idAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = idAlphabet[rand.Intn(len(idAlphabet))]
	}
	return string(b)
}

func (m *roomManager) genID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < 100; i++ {
		id := randomID(4)
		if _, exists := m.rooms[id]; !exists {
			return id
		}
	}
	return randomID(6)
}

func (m *roomManager) create(mode string, maxPlayers, goalLimit int, pid, name string) *room {
	id := m.genID()
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &room{
		id:         id,
		mode:       mode,
		maxPlayers: maxPlayers,
		goalLimit:  goalLimit,
		players:    []*player{},
		byPid:      make(map[string]*player),
		board:      game.New(9, 6),
	}
	m.rooms[id] = r
	r.addPlayerLocked(pid, name)
	if r.mode == "single" || r.maxPlayers <= 1 {
		r.started = true
	}
	return r
}

func (m *roomManager) join(id, pid, name string) (*room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.rooms[id]
	if r == nil {
		return nil, errors.New("game not found")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byPid[pid] != nil {
		return r, nil
	}
	if r.started {
		return nil, errors.New("game already started")
	}
	if len(r.players) >= r.maxPlayers {
		return nil, errors.New("game is full")
	}
	r.addPlayerLocked(pid, name)
	r.addChat("", fmt.Sprintf("%s joined", r.players[len(r.players)-1].name))
	if len(r.players) == r.maxPlayers {
		r.started = true
	}
	return r, nil
}

func (m *roomManager) get(id string) (*room, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rooms[id]
	return r, ok
}

// sweep removes multiplayer players whose connection went stale (no request
// within the timeout). Explicit leaves use removePlayerLocked via /api/leave.
func (m *roomManager) sweep(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-timeout)
	for id, r := range m.rooms {
		if r.mode == "single" {
			continue
		}
		r.mu.Lock()
		var stale []string
		for _, p := range r.players {
			if p.lastSeen.Before(cutoff) {
				stale = append(stale, p.pid)
			}
		}
		for _, pid := range stale {
			if p := r.byPid[pid]; p != nil {
				r.removePlayerLocked(pid)
				r.addChat("", fmt.Sprintf("%s disconnected", p.name))
			}
		}
		empty := len(r.players) == 0
		r.mu.Unlock()
		if empty {
			delete(m.rooms, id)
		}
	}
}

// assignTokenLocked picks a free spot for a new token on the shared board:
// first an unoccupied corner, then any free interior cell away from the goal.
func (r *room) assignTokenLocked() (int, int) {
	s := r.board.State()
	w, h := len(s.Map[0]), len(s.Map)
	used := func(x, y int) bool {
		for _, p := range r.players {
			if p.x == x && p.y == y {
				return true
			}
		}
		return false
	}
	corners := [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}}
	for _, c := range corners {
		if !used(c[0], c[1]) {
			return c[0], c[1]
		}
	}
	for i := 0; i < 200; i++ {
		x, y := rand.Intn(w), rand.Intn(h)
		if (x == s.GoalX && y == s.GoalY) || used(x, y) {
			continue
		}
		return x, y
	}
	return 0, 0
}

func (r *room) nextSymbolLocked() string {
	used := map[string]bool{}
	for _, p := range r.players {
		used[p.symbol] = true
	}
	for _, s := range playerSymbols {
		if !used[s] {
			return s
		}
	}
	return "?"
}

// sanitizeName strips control characters from a player name and caps its
// length so names cannot break the UI or chat layout.
func sanitizeName(name string) string {
	var b strings.Builder
	for _, rn := range strings.TrimSpace(name) {
		if rn < 0x20 {
			continue
		}
		b.WriteRune(rn)
		if b.Len() >= 24 {
			break
		}
	}
	return b.String()
}

func (r *room) addPlayerLocked(pid, name string) *player {
	if existing := r.byPid[pid]; existing != nil {
		return existing
	}
	name = sanitizeName(name)
	if name == "" {
		name = fmt.Sprintf("Player %d", len(r.players)+1)
	}
	x, y := r.assignTokenLocked()
	p := &player{
		pid:      pid,
		name:     name,
		symbol:   r.nextSymbolLocked(),
		seat:     len(r.players),
		x:        x,
		y:        y,
		lastSeen: time.Now(),
	}
	r.players = append(r.players, p)
	r.byPid[pid] = p
	return p
}

// installToken puts the given player's token onto the shared board so the
// board's movement, slide and reachability primitives act on it.
func (r *room) installToken(p *player) {
	r.board.SetToken(p.x, p.y)
}

// readToken copies the board token back into the player.
func (r *room) readToken(p *player) {
	s := r.board.State()
	p.x, p.y = s.PlayerX, s.PlayerY
}

// shiftOtherTokensLocked moves every token except keep that sits on a slid
// row/column, mirroring the slide applied to the board. row/col are -1 when
// that axis was not slid.
func (r *room) shiftOtherTokensLocked(keep *player, row, col int, right, down bool) {
	s := r.board.State()
	w, h := len(s.Map[0]), len(s.Map)
	for _, p := range r.players {
		if p == keep {
			continue
		}
		if row >= 0 && p.y == row {
			if right {
				p.x = (p.x + 1) % w
			} else {
				p.x = (p.x - 1 + w) % w
			}
		} else if col >= 0 && p.x == col {
			if down {
				p.y = (p.y + 1) % h
			} else {
				p.y = (p.y - 1 + h) % h
			}
		}
	}
}

// slideActiveLocked applies the acting player's row/column slide to the
// shared board, moving other tokens that sit on the slid line. It returns
// true only when a slide actually happened; invalid or blocked lines are
// ignored so other tokens never shift without a real slide.
func (r *room) slideActiveLocked(p *player, row, col int, right, down bool) bool {
	r.installToken(p)
	if row >= 0 {
		r.board.SlideRow(row, right)
	} else if col >= 0 {
		r.board.SlideCol(col, down)
	}
	if !r.board.State().HasSlid {
		return false
	}
	r.shiftOtherTokensLocked(p, row, col, right, down)
	r.readToken(p)
	r.afterAction()
	return true
}

// removePlayerLocked removes a player from the room, renumbering seats and
// keeping the goal progress (totalCatches and remaining scores) untouched.
// If the removed player held the turn, it passes to the next player in
// join order.
func (r *room) removePlayerLocked(pid string) bool {
	for i, p := range r.players {
		if p.pid != pid {
			continue
		}
		r.players = append(r.players[:i], r.players[i+1:]...)
		delete(r.byPid, pid)
		if len(r.players) == 0 {
			r.curTurn = 0
			r.finished = true
			return true
		}
		for j := range r.players {
			r.players[j].seat = j
		}
		if i < r.curTurn {
			r.curTurn--
		}
		r.curTurn %= len(r.players)
		return true
	}
	return false
}

func (r *room) leave(pid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.byPid[pid]
	if p == nil {
		return false
	}
	r.removePlayerLocked(pid)
	r.addChat("", fmt.Sprintf("%s left", p.name))
	return true
}

func (r *room) addChat(name, text string) {
	r.messages = append(r.messages, chatMsg{ID: r.nextMsgID, Name: name, Text: text})
	r.nextMsgID++
	if len(r.messages) > chatLimit {
		r.messages = r.messages[len(r.messages)-chatLimit:]
	}
}

func (r *room) isActor(p *player) bool {
	return r.started && !r.finished && r.curTurn < len(r.players) && r.players[r.curTurn] == p
}

func (r *room) advanceTurn() {
	if len(r.players) > 0 {
		r.curTurn = (r.curTurn + 1) % len(r.players)
	}
}

// afterAction must be called after a player acts (move/slide). It detects
// a goal catch, bumps the player's score, and ends the game if the goal
// limit has been reached. On a catch the goal moves to a new random spot
// while the catcher keeps their position. A goal limit of 0 means the game
// continues forever.
func (r *room) afterAction() {
	p := r.players[r.curTurn]
	if r.board.State().Running {
		return
	}
	p.score++
	r.totalCatches++
	r.addChat("", fmt.Sprintf("%s caught the goal!", p.name))
	if r.goalLimit > 0 && r.totalCatches >= r.goalLimit {
		r.messages[len(r.messages)-1].Text += " Game over."
		r.finished = true
		return
	}
	r.board.NewRound()
	r.readToken(p)
	r.advanceTurn()
}

func (r *room) endTurn() {
	r.board.EndTurn()
	r.advanceTurn()
}

type playerInfo struct {
	Seat   int    `json:"seat"`
	Name   string `json:"name"`
	Score  int    `json:"score"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Symbol string `json:"symbol"`
}

type roomInfo struct {
	ID            string       `json:"id"`
	Mode          string       `json:"mode"`
	Started       bool         `json:"started"`
	Finished      bool         `json:"finished"`
	MaxPlayers    int          `json:"maxPlayers"`
	GoalLimit     int          `json:"goalLimit"`
	TotalCatches  int          `json:"totalCatches"`
	CurrentPlayer int          `json:"currentPlayer"`
	YourSeat      int          `json:"yourSeat"`
	YourTurn      bool         `json:"yourTurn"`
	Players       []playerInfo `json:"players"`
	Chat          []chatMsg    `json:"chat"`
}

func (r *room) info(pid string) roomInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.infoLocked(pid)
}

func (r *room) infoLocked(pid string) roomInfo {
	info := roomInfo{
		ID:            r.id,
		Mode:          r.mode,
		Started:       r.started,
		Finished:      r.finished,
		MaxPlayers:    r.maxPlayers,
		GoalLimit:     r.goalLimit,
		TotalCatches:  r.totalCatches,
		CurrentPlayer: -1,
		YourSeat:      -1,
		Chat:          append([]chatMsg(nil), r.messages...),
	}
	if r.started && len(r.players) > 0 {
		info.CurrentPlayer = r.players[r.curTurn].seat
	}
	for _, p := range r.players {
		info.Players = append(info.Players, playerInfo{
			Seat:   p.seat,
			Name:   p.name,
			Score:  p.score,
			X:      p.x,
			Y:      p.y,
			Symbol: p.symbol,
		})
		if p.pid == pid {
			info.YourSeat = p.seat
		}
	}
	info.YourTurn = r.started && !r.finished && info.CurrentPlayer == info.YourSeat
	return info
}

type leaderboardEntry struct {
	Rank  int    `json:"rank"`
	Seat  int    `json:"seat"`
	Name  string `json:"name"`
	Score int    `json:"score"`
}

type leaderboardResponse struct {
	ID         string             `json:"id"`
	Mode       string             `json:"mode"`
	GoalLimit  int                `json:"goalLimit"`
	TotalCatch int                `json:"totalCatches"`
	Winner     string             `json:"winner"`
	Entries    []leaderboardEntry `json:"entries"`
}

func (r *room) leaderboard() leaderboardResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	resp := leaderboardResponse{
		ID:         r.id,
		Mode:       r.mode,
		GoalLimit:  r.goalLimit,
		TotalCatch: r.totalCatches,
	}
	idx := make([]int, len(r.players))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := r.players[idx[a]], r.players[idx[b]]
		if pa.score != pb.score {
			return pa.score > pb.score
		}
		return pa.seat < pb.seat
	})
	for rank, i := range idx {
		p := r.players[i]
		resp.Entries = append(resp.Entries, leaderboardEntry{
			Rank:  rank + 1,
			Seat:  p.seat,
			Name:  p.name,
			Score: p.score,
		})
	}
	if len(idx) > 0 {
		resp.Winner = resp.Entries[0].Name
	}
	return resp
}
