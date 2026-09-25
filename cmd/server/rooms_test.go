package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/giulianozor/dungeon/internal/game"
)

// bfsPath returns the shortest movement path from (sx, sy) to (tx, ty),
// mirroring the in-game movement rules (wrapping included).
func bfsPath(g *game.Game, sx, sy, tx, ty int) [][2]int {
	s := g.State()
	h, w := len(s.Map), len(s.Map[0])
	visited := make([][]bool, h)
	prev := make([][][2]int, h)
	for y := range visited {
		visited[y] = make([]bool, w)
		prev[y] = make([][2]int, w)
	}
	q := [][2]int{{sy, sx}}
	visited[sy][sx] = true
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		cy, cx := cur[0], cur[1]
		if cy == ty && cx == tx {
			var path [][2]int
			for cy != sy || cx != sx {
				path = append([][2]int{{cx, cy}}, path...)
				p := prev[cy][cx]
				cy, cx = p[0], p[1]
			}
			return append([][2]int{{sx, sy}}, path...)
		}
		for _, d := range dirs {
			nx := ((cx+d[0])%w + w) % w
			ny := ((cy+d[1])%h + h) % h
			if visited[ny][nx] || (nx == cx && ny == cy) {
				continue
			}
			a, b := s.Map[cy][cx], s.Map[ny][nx]
			var ok bool
			switch {
			case d[0] == 0 && d[1] == -1:
				ok = a.OpenN && b.OpenS
			case d[0] == 0 && d[1] == 1:
				ok = a.OpenS && b.OpenN
			case d[0] == 1 && d[1] == 0:
				ok = a.OpenE && b.OpenW
			case d[0] == -1 && d[1] == 0:
				ok = a.OpenW && b.OpenE
			}
			if !ok {
				continue
			}
			visited[ny][nx] = true
			prev[ny][nx] = [2]int{cy, cx}
			q = append(q, [2]int{ny, nx})
		}
	}
	return nil
}

// driveToGoal moves the player's token step by step along the shortest path
// to the goal on the shared board, re-rolling the dice whenever the movement
// runs out, until caught.
func driveToGoal(t *testing.T, r *room, p *player) {
	t.Helper()
	for i := 0; i < 500; i++ {
		s := r.board.State()
		if !s.Running {
			return
		}
		r.installToken(p)
		path := bfsPath(r.board, p.x, p.y, s.GoalX, s.GoalY)
		if len(path) < 2 {
			r.board.EndTurn()
			continue
		}
		steps := s.Remaining
		if steps > len(path)-1 {
			steps = len(path) - 1
		}
		for k := 1; k <= steps; k++ {
			prev, nxt := path[k-1], path[k]
			r.board.Move(nxt[0]-prev[0], nxt[1]-prev[1])
			if !r.board.State().Running {
				r.readToken(p)
				return
			}
		}
		r.readToken(p)
		r.board.EndTurn()
	}
	t.Fatalf("driveToGoal failed to catch the goal in 500 turns")
}

func TestCreateSingleStarts(t *testing.T) {
	m := newRoomManager()
	r := m.create("single", 1, 1, "p1", "Solo")
	if !r.started {
		t.Fatal("single-player game should start immediately")
	}
	if len(r.players) != 1 {
		t.Fatalf("want 1 player, got %d", len(r.players))
	}
}

func TestCreateMultiLobbyAutostartOnFull(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 3, "host", "Host")
	if r.started {
		t.Fatal("multi-player game should only start when full")
	}
	if _, err := m.join(r.id, "host", "Host"); err != nil {
		t.Fatalf("rejoin same pid should be idempotent: %v", err)
	}
	if _, err := m.join(r.id, "guest", "Guest"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if !r.started {
		t.Fatal("expected autostart once all players are connected")
	}
	if len(r.players) != 2 {
		t.Fatalf("want 2 players, got %d", len(r.players))
	}
	if _, err := m.join(r.id, "third", "Third"); err == nil {
		t.Fatal("expected error joining a full game")
	}
	if _, err := m.join("NOPE", "x", "X"); err == nil {
		t.Fatal("expected error joining an unknown game")
	}
	if r.players[0].name != "Host" || r.players[1].name != "Guest" {
		t.Fatalf("names not stored: %+v", r.players)
	}
	if len(r.messages) != 1 || r.messages[0].Text != "Guest joined" {
		t.Fatalf("want a single join chat message, got %+v", r.messages)
	}
}

func TestJoinSetIsJoinOrder(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 3, 2, "a", "A")
	m.join(r.id, "b", "B")
	m.join(r.id, "c", "C")
	if len(r.players) != 3 {
		t.Fatalf("want 3 players, got %d", len(r.players))
	}
	for i, want := range []string{"a", "b", "c"} {
		if r.players[i].pid != want {
			t.Fatalf("seat %d = %q, want %q (join order)", i, r.players[i].pid, want)
		}
	}
}

func TestTurnAdvancesInJoinOrder(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 3, 2, "a", "A")
	m.join(r.id, "b", "B")
	m.join(r.id, "c", "C")
	order := []string{}
	for i := 0; i < 6; i++ {
		order = append(order, r.players[r.curTurn].pid)
		r.endTurn()
	}
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("turn %d = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestOnlyCurrentPlayerCanAct(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	b := r.byPid["b"]
	if r.isActor(b) {
		t.Fatal("player b must not be the actor first")
	}
	r.endTurn()
	if !r.isActor(b) {
		t.Fatal("player b should be the actor after a ends their turn")
	}
}

func TestPlayUntilGoalLimitReached(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")

	for i := 0; i < 1000 && !r.finished; i++ {
		p := r.players[r.curTurn]
		before := r.totalCatches
		driveToGoal(t, r, p)
		r.afterAction()
		if r.totalCatches == before {
			// No catch: the player ended their turn normally.
			r.endTurn()
		}
	}
	if !r.finished {
		t.Fatal("expected the game to finish after the goal limit")
	}
	if r.totalCatches != r.goalLimit {
		t.Fatalf("totalCatches = %d, want %d", r.totalCatches, r.goalLimit)
	}
	lb := r.leaderboard()
	if len(lb.Entries) != 2 {
		t.Fatalf("want 2 leaderboard entries, got %d", len(lb.Entries))
	}
	got := 0
	for _, e := range lb.Entries {
		got += e.Score
	}
	if got != r.goalLimit {
		t.Fatalf("leaderboard scores sum %d, want %d", got, r.goalLimit)
	}
}

func TestSingleCatchFinishes(t *testing.T) {
	m := newRoomManager()
	r := m.create("single", 1, 1, "solo", "Solo")
	driveToGoal(t, r, r.players[0])
	r.afterAction()
	if !r.finished {
		t.Fatal("expected single-player game to finish on catch")
	}
	if len(r.leaderboard().Entries) != 1 {
		t.Fatal("expected 1 leaderboard entry")
	}
}

func TestSingleEndsAfterChosenGoalLimit(t *testing.T) {
	m := newRoomManager()
	r := m.create("single", 1, 2, "solo", "Solo")
	for i := 1; i <= 2; i++ {
		driveToGoal(t, r, r.players[0])
		r.afterAction()
		if r.totalCatches != i {
			t.Fatalf("totalCatches = %d after catch %d", r.totalCatches, i)
		}
		if i < 2 && r.finished {
			t.Fatalf("game finished early after catch %d", i)
		}
	}
	if !r.finished {
		t.Fatal("expected single-player game to finish at the goal limit")
	}
	if got := r.leaderboard().Entries[0].Score; got != 2 {
		t.Fatalf("score = %d, want 2", got)
	}
}

func TestSingleEndlessGoals(t *testing.T) {
	m := newRoomManager()
	r := m.create("single", 1, 0, "solo", "Solo")
	for i := 1; i <= 5; i++ {
		driveToGoal(t, r, r.players[0])
		r.afterAction()
		if r.finished {
			t.Fatalf("endless game finished after catch %d", i)
		}
		if r.totalCatches != i {
			t.Fatalf("totalCatches = %d after catch %d", r.totalCatches, i)
		}
		if !r.board.State().Running {
			t.Fatalf("board must start a new round after catch %d", i)
		}
	}
}

func TestNameDefaultsAndTrims(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "   ")
	if r.players[0].name != "Player 1" {
		t.Fatalf("want default name, got %q", r.players[0].name)
	}
	m.join(r.id, "b", "  Bob  ")
	if r.players[1].name != "Bob" {
		t.Fatalf("want trimmed name, got %q", r.players[1].name)
	}
}

func TestRemovePlayerKeepsProgressAndRenumbers(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 3, 5, "a", "A")
	m.join(r.id, "b", "B")
	m.join(r.id, "c", "C")

	driveToGoal(t, r, r.players[r.curTurn])
	r.afterAction()
	if r.totalCatches != 1 {
		t.Fatalf("want 1 catch, got %d", r.totalCatches)
	}

	if !r.removePlayerLocked("b") {
		t.Fatal("expected b to be removed")
	}
	if r.byPid["b"] != nil {
		t.Fatal("b still present after removal")
	}
	if len(r.players) != 2 {
		t.Fatalf("want 2 players, got %d", len(r.players))
	}
	if r.totalCatches != 1 {
		t.Fatalf("catches must be kept, got %d", r.totalCatches)
	}
	for i, p := range r.players {
		if p.seat != i {
			t.Fatalf("seat %q = %d, want %d", p.pid, p.seat, i)
		}
	}
	lb := r.leaderboard()
	if len(lb.Entries) != 2 {
		t.Fatalf("want 2 leaderboard entries, got %d", len(lb.Entries))
	}
	for _, e := range lb.Entries {
		if e.Name == "B" {
			t.Fatal("removed player must not appear in the leaderboard")
		}
	}
}

func TestLeaveAddsChatAndRemovesPlayer(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")

	r.addChat("B", "hello")
	last := r.messages[len(r.messages)-1]
	if last.Name != "B" || last.Text != "hello" {
		t.Fatalf("bad chat message: %+v", last)
	}

	if !r.leave("b") {
		t.Fatal("leave failed")
	}
	if r.byPid["b"] != nil {
		t.Fatal("b still present after leave")
	}
	last = r.messages[len(r.messages)-1]
	if last.Name != "" || !strings.Contains(last.Text, "left") {
		t.Fatalf("expected a leave system message, got %+v", last)
	}
	if r.finished {
		t.Fatal("room must not finish while a player remains")
	}
	if !r.leave("a") {
		t.Fatal("leave a failed")
	}
	if !r.finished {
		t.Fatal("expected room to finish when the last player leaves")
	}
	if len(r.players) != 0 {
		t.Fatalf("want 0 players, got %d", len(r.players))
	}
}

func TestSweepRemovesStalePlayers(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")

	r.mu.Lock()
	r.byPid["b"].lastSeen = time.Now().Add(-time.Minute)
	r.mu.Unlock()

	m.sweep(30 * time.Second)

	if r.byPid["b"] != nil {
		t.Fatal("stale player not swept")
	}
	if len(r.players) != 1 || r.players[0].pid != "a" {
		t.Fatalf("active player must remain, got %+v", r.players)
	}
	if r.totalCatches != 0 {
		t.Fatal("sweep must not change the catch total")
	}
	last := r.messages[len(r.messages)-1]
	if !strings.Contains(last.Text, "disconnected") {
		t.Fatalf("expected a disconnect message, got %+v", last)
	}

	if _, still := m.rooms[r.id]; r.finished || !still {
		t.Fatal("room must stay alive while a player remains")
	}

	r.mu.Lock()
	r.byPid["a"].lastSeen = time.Now().Add(-time.Minute)
	r.mu.Unlock()
	for i := 0; i < 2 && len(r.players) > 0; i++ {
		m.sweep(30 * time.Second)
	}
	if _, still := m.rooms[r.id]; still {
		t.Fatal("empty room should be removed from the manager")
	}
}

func TestGameIDsAreUnambiguousAlphanumeric(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := randomID(4)
		for _, c := range id {
			if !(c >= 'A' && c <= 'Z') && !(c >= '2' && c <= '9') {
				t.Fatalf("id %q contains a non-alphanumeric or ambiguous char %q", id, c)
			}
			if strings.ContainsRune("OI01", c) {
				t.Fatalf("id %q contains ambiguous char %q", id, c)
			}
		}
	}
}

func TestGenIDProducesUniqueRooms(t *testing.T) {
	m := newRoomManager()
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		r := m.create("single", 1, 1, fmt.Sprintf("p%d", i), "P")
		if seen[r.id] {
			t.Fatalf("duplicate room id %q", r.id)
		}
		if len(r.id) != 4 {
			t.Fatalf("want 4-char id, got %q", r.id)
		}
		seen[r.id] = true
	}
}

func TestAllPlayersSeeActivePlayersBoard(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	gs := &gameServer{rm: m}

	if r.curTurn != 0 {
		t.Fatalf("expected a to act first, curTurn=%d", r.curTurn)
	}
	sa := gs.stateRes(r, r.byPid["a"])
	sb := gs.stateRes(r, r.byPid["b"])
	if sb.PlayerX != sa.PlayerX || sb.PlayerY != sa.PlayerY ||
		sb.GoalX != sa.GoalX || sb.GoalY != sa.GoalY {
		t.Fatalf("spectator must see the actor's board: a=(%d,%d,%d,%d) b=(%d,%d,%d,%d)",
			sa.PlayerX, sa.PlayerY, sa.GoalX, sa.GoalY,
			sb.PlayerX, sb.PlayerY, sb.GoalX, sb.GoalY)
	}
	if sb.Map[0][0] != sa.Map[0][0] {
		t.Fatal("spectator must see the same maze as the actor")
	}
	if !sa.Room.YourTurn || sb.Room.YourTurn {
		t.Fatalf("turn flags wrong: a=%v b=%v", sa.Room.YourTurn, sb.Room.YourTurn)
	}

	// Both screens must show the same shared board and BOTH tokens.
	byName := func(res stateResponse, name string) *playerInfo {
		for i := range res.Room.Players {
			if res.Room.Players[i].Name == name {
				return &res.Room.Players[i]
			}
		}
		t.Fatalf("player %s missing from state", name)
		return nil
	}
	pa, pb := byName(sa, "A"), byName(sa, "B")
	if pa.Symbol == pb.Symbol {
		t.Fatalf("players must get distinct symbols: %q vs %q", pa.Symbol, pb.Symbol)
	}
	ob := byName(sb, "B")
	if ob.X != pb.X || ob.Y != pb.Y {
		t.Fatalf("b's token differs between screens: %+v vs %+v", ob, pb)
	}

	r.endTurn()
	sa2 := gs.stateRes(r, r.byPid["a"])
	sb2 := gs.stateRes(r, r.byPid["b"])
	rb := r.byPid["b"]
	if sa2.PlayerX != rb.x || sa2.PlayerY != rb.y {
		t.Fatalf("after the turn passes, everyone must see b's board, got (%d,%d) want (%d,%d)",
			sa2.PlayerX, sa2.PlayerY, rb.x, rb.y)
	}
	if !sb2.Room.YourTurn || sa2.Room.YourTurn {
		t.Fatalf("turn flags wrong after switch: a=%v b=%v", sa2.Room.YourTurn, sb2.Room.YourTurn)
	}
}

func TestSlideShiftsOtherTokens(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 3, 3, "a", "A")
	m.join(r.id, "b", "B")
	m.join(r.id, "c", "C")
	a, b, c := r.players[0], r.players[1], r.players[2]
	b.x, b.y = 0, 3
	c.x, c.y = 2, 0

	if r.curTurn != 0 {
		t.Fatalf("expected a to act first")
	}
	r.installToken(a)
	r.board.SetSlideLines([]int{3}, nil)
	wantGoalX, wantGoalY := r.board.State().GoalX, r.board.State().GoalY
	r.board.SlideRow(3, true)
	if gs := r.board.State(); gs.GoalX == wantGoalX && gs.GoalY == wantGoalY {
		t.Fatalf("goal should move on the slid row, goal=(%d,%d)", gs.GoalX, gs.GoalY)
	}
	r.shiftOtherTokensLocked(a, 3, -1, true, false)
	if b.x != 1 || b.y != 3 {
		t.Fatalf("token on slid row must shift right, b=(%d,%d)", b.x, b.y)
	}
	if c.x != 2 || c.y != 0 {
		t.Fatalf("token off the slid row must not move, c=(%d,%d)", c.x, c.y)
	}
}

func TestNewRoundKeepsCatchingPlayersToken(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 3, 3, "a", "A")
	m.join(r.id, "b", "B")
	a := r.players[0]

	driveToGoal(t, r, a)
	if r.board.State().Running {
		t.Fatal("driveToGoal ended the turn without catching the goal")
	}
	caughtX, caughtY := a.x, a.y
	r.afterAction()
	if r.totalCatches != 1 {
		t.Fatalf("totalCatches = %d, want 1", r.totalCatches)
	}
	if a.x != caughtX || a.y != caughtY {
		t.Fatalf("catcher moved after a catch, now (%d,%d) want (%d,%d)", a.x, a.y, caughtX, caughtY)
	}
	if s := r.board.State(); s.GoalX == caughtX && s.GoalY == caughtY {
		t.Fatal("goal must move after a catch")
	}
}

func TestCatchSendsChatMessage(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	driveToGoal(t, r, r.players[0])
	r.afterAction()
	got := ""
	for _, msg := range r.messages {
		if strings.Contains(msg.Text, "A caught the goal") {
			got = msg.Text
		}
	}
	if got == "" {
		t.Fatalf("want a chat message on goal capture, got %+v", r.messages)
	}
	if strings.Contains(got, "Game over.") {
		t.Fatalf("continuing catch must not say game over: %q", got)
	}
}

func TestFinalCatchChatMessage(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 1, "a", "A")
	m.join(r.id, "b", "B")
	driveToGoal(t, r, r.players[0])
	r.afterAction()
	if !r.finished {
		t.Fatal("game should finish at the goal limit")
	}
	last := r.messages[len(r.messages)-1]
	if !strings.Contains(last.Text, "A caught the goal") || !strings.Contains(last.Text, "Game over.") {
		t.Fatalf("want a game-over chat message, got %+v", last)
	}
}

func TestMultiplayerBoardSize(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 4, 4, "a", "A")
	s := r.board.State()
	w, h := len(s.Map[0]), len(s.Map)
	if w != 9 || h != 6 {
		t.Fatalf("board size = %dx%d, want 9x6 so the UI fits", w, h)
	}
	if n := len(s.SlideRows) + len(s.SlideCols); n != 3 {
		t.Fatalf("slideable line total = %d, want 3 (half of 5 odd cols + 3 odd rows)", n)
	}
}

func TestSinglePlayerBoardSize(t *testing.T) {
	m := newRoomManager()
	r := m.create("single", 1, 1, "a", "A")
	s := r.board.State()
	if w, h := len(s.Map[0]), len(s.Map); w != 9 || h != 6 {
		t.Fatalf("single player board size = %dx%d, want 9x6", w, h)
	}
}
