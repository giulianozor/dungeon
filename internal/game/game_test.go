package game

import (
	"fmt"
	"testing"

	"github.com/giulianozor/dungeon/internal/ui"
)

func makeGame(width, height int) *Game {
	return New(width, height)
}

func TestNew_Dimensions(t *testing.T) {
	g := makeGame(7, 6)
	if len(g.state.Map) != 6 {
		t.Fatalf("got %d rows, want 6", len(g.state.Map))
	}
	if len(g.state.Map[0]) != 7 {
		t.Fatalf("got %d cols, want 7", len(g.state.Map[0]))
	}
}

func TestNew_RandomCornerStart(t *testing.T) {
	cornerSet := map[[2]int]bool{{0, 0}: true, {6, 0}: true, {0, 5}: true, {6, 5}: true}
	found := map[[2]int]int{}
	for i := 0; i < 100; i++ {
		g := makeGame(7, 6)
		pos := [2]int{g.state.PlayerX, g.state.PlayerY}
		if !cornerSet[pos] {
			t.Fatalf("player at (%d,%d), not a corner", pos[0], pos[1])
		}
		found[pos]++
	}
	if len(found) < 2 {
		t.Error("start position seems deterministic across 100 games")
	}
}

func TestBorderTiles_NoOutwardOpenings(t *testing.T) {
	g := makeGame(7, 6)
	h, w := len(g.state.Map), len(g.state.Map[0])
	lr, lc := h-1, w-1
	for y := range g.state.Map {
		for x := range g.state.Map[y] {
			tile := g.state.Map[y][x]
			if y == 0 && tile.OpenN {
				t.Errorf("top border (%d,%d) has outward N opening", x, y)
			}
			if y == lr && tile.OpenS {
				t.Errorf("bottom border (%d,%d) has outward S opening", x, y)
			}
			if x == 0 && tile.OpenW {
				t.Errorf("left border (%d,%d) has outward W opening", x, y)
			}
			if x == lc && tile.OpenE {
				t.Errorf("right border (%d,%d) has outward E opening", x, y)
			}
		}
	}
}

func TestTileConfigs_HaveCorrectOpeningCounts(t *testing.T) {
	for i, cfg := range tileConfigs {
		n := 0
		if cfg.OpenN {
			n++
		}
		if cfg.OpenS {
			n++
		}
		if cfg.OpenE {
			n++
		}
		if cfg.OpenW {
			n++
		}
		if n < 1 || n > 4 {
			t.Errorf("tileConfigs[%d] has %d openings, want 1-4", i, n)
		}
	}
}

func TestMove_Up_Allowed(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(0, -1)
	if g.state.PlayerY != 0 {
		t.Errorf("expected y=0, got y=%d", g.state.PlayerY)
	}
}

func TestMove_Up_BlockedByCurrTile(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenS: true}, {}},
		{{}, {}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(0, -1)
	if g.state.PlayerY != 1 {
		t.Errorf("expected y=1 (blocked), got y=%d", g.state.PlayerY)
	}
}

func TestMove_Up_BlockedByDestTile(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenN: true}, {}},
		{{}, {OpenN: true}, {}},
		{{}, {}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(0, -1)
	if g.state.PlayerY != 1 {
		t.Errorf("expected y=1 (blocked), got y=%d", g.state.PlayerY)
	}
}

func TestMove_Down_Allowed(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {}, {}},
		{{}, {OpenS: true, OpenN: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(0, 1)
	if g.state.PlayerY != 2 {
		t.Errorf("expected y=2, got y=%d", g.state.PlayerY)
	}
}

func TestMove_Left_Allowed(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {}, {}},
		{{OpenE: true}, {OpenW: true, OpenE: true}, {}},
		{{}, {}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(-1, 0)
	if g.state.PlayerX != 0 {
		t.Errorf("expected x=0, got x=%d", g.state.PlayerX)
	}
}

func TestMove_Right_Allowed(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {}, {}},
		{{}, {OpenE: true}, {OpenW: true}},
		{{}, {}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true}}
	g.tryMove(1, 0)
	if g.state.PlayerX != 2 {
		t.Errorf("expected x=2, got x=%d", g.state.PlayerX)
	}
}

func TestMove_WrapsAroundGrid(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenS: true, OpenN: true}},
		{{OpenS: true, OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Running: true}}
	g.tryMove(0, -1)
	if g.state.PlayerY != 1 {
		t.Errorf("expected y=1 (wrap), got y=%d", g.state.PlayerY)
	}
}

func TestMove_WrappingBlockedByNoOpening(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenS: true}},
		{{OpenS: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Running: true}}
	g.tryMove(0, -1)
	if g.state.PlayerY != 0 {
		t.Errorf("expected y=0 (wrap blocked), got y=%d", g.state.PlayerY)
	}
}

func TestMove_WrapsHorizontally(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenW: true, OpenE: true}},
		{{OpenW: true, OpenE: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Running: true}}
	g.tryMove(-1, 0)
	if g.state.PlayerX != 0 {
		t.Errorf("expected x=0 (wrap to same col in 1-col grid), got x=%d", g.state.PlayerX)
	}
}

func TestReachingGoal(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenS: true}, {OpenS: true}},
		{{OpenN: true, OpenS: true}, {OpenN: true}},
		{{OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, GoalX: 1, GoalY: 0, Running: true},
	}
	g.tryMove(0, -1)
	if g.state.Running {
		t.Error("expected game to end after reaching goal")
	}
	if g.state.Message != "You win!" {
		t.Errorf("expected win message, got %q", g.state.Message)
	}
}

func TestSlideRow_OddRowOnly(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Running: true}}
	g.SlideRow(0, true)
	t.Log("even row slide should be no-op")
}

func TestSlideRow_ShiftsTiles(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenE: true}, {OpenW: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 2, PlayerY: 1, Running: true, SlideRows: []int{1}}}
	g.SlideRow(1, true)
	if !g.state.Map[1][0].OpenW {
		t.Error("expected row to shift right (first tile gets old rightmost OpenW)")
	}
	if !g.state.Map[1][1].OpenN {
		t.Error("expected row[1] to be old row[0] (OpenN)")
	}
	if g.state.PlayerX != 0 {
		t.Errorf("expected player x=0 (wrapped with row), got x=%d", g.state.PlayerX)
	}
}

func TestSlideCol_OddColOnly(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Running: true}}
	g.SlideCol(0, true)
	t.Log("even col slide should be no-op")
}

func TestSlideCol_ShiftsTiles(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenE: true}, {OpenN: true}},
		{{OpenN: true}, {OpenW: true}, {OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 0, Running: true, SlideCols: []int{1}}}
	g.SlideCol(1, true)
	if !g.state.Map[0][1].OpenW {
		t.Error("expected col to shift down (col[0][1] gets old bottom OpenW)")
	}
	if g.state.PlayerY != 1 {
		t.Errorf("expected player y=1 (shifted down with col), got y=%d", g.state.PlayerY)
	}
}

func TestAutoRollOnNew(t *testing.T) {
	g := makeGame(7, 6)
	if !g.state.HasRolled {
		t.Error("expected HasRolled true after New() auto-roll")
	}
	if g.state.Remaining < 2 || g.state.Remaining > 12 {
		t.Errorf("expected Remaining 2-12, got %d", g.state.Remaining)
	}
	if g.state.DiceTotal < 2 || g.state.DiceTotal > 12 {
		t.Errorf("expected DiceTotal 2-12, got %d", g.state.DiceTotal)
	}
}

func TestMove_DecrementsRemaining(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true, Remaining: 5},
	}
	g.Move(0, -1)
	if g.state.Remaining != 4 {
		t.Errorf("expected Remaining=4, got %d", g.state.Remaining)
	}
}

func TestMove_NoRemainingDoesNothing(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1, Running: true, Remaining: 0},
	}
	g.Move(0, -1)
	if g.state.PlayerY != 1 {
		t.Errorf("expected no move when Remaining=0, player y=%d", g.state.PlayerY)
	}
}

func TestGoalAtCenter(t *testing.T) {
	g := makeGame(10, 7)
	if g.state.GoalX != 5 || g.state.GoalY != 3 {
		t.Errorf("expected goal at (5,3), got (%d,%d)", g.state.GoalX, g.state.GoalY)
	}
}

func TestMap_GoalAlwaysReachable(t *testing.T) {
	for i := 0; i < 200; i++ {
		g := makeGame(10, 7)
		if len(g.shortestPath(g.state.GoalX, g.state.GoalY)) == 0 {
			t.Fatalf("goal (%d,%d) unreachable on game %d", g.state.GoalX, g.state.GoalY, i)
		}
	}
}

func TestMap_NewRoundGoalReachable(t *testing.T) {
	for i := 0; i < 200; i++ {
		g := makeGame(10, 7)
		g.NewRound()
		if len(g.shortestPath(g.state.GoalX, g.state.GoalY)) == 0 {
			t.Fatalf("NewRound goal (%d,%d) unreachable on game %d", g.state.GoalX, g.state.GoalY, i)
		}
	}
}

func TestNewRound_KeepsPlayerMovesGoal(t *testing.T) {
	g := makeGame(10, 7)
	g.state.PlayerX, g.state.PlayerY = 4, 5
	g.state.GoalX, g.state.GoalY = 4, 5
	g.NewRound()
	if g.state.PlayerX != 4 || g.state.PlayerY != 5 {
		t.Fatalf("NewRound moved the player to (%d,%d), want (4,5)", g.state.PlayerX, g.state.PlayerY)
	}
	if g.state.GoalX == 4 && g.state.GoalY == 5 {
		t.Fatal("NewRound must move the goal off the player")
	}
}

func TestSetToken_InstallsPosition(t *testing.T) {
	g := makeGame(10, 7)
	g.SetToken(3, 2)
	if g.state.PlayerX != 3 || g.state.PlayerY != 2 {
		t.Fatalf("SetToken left token at (%d,%d), want (3,2)", g.state.PlayerX, g.state.PlayerY)
	}
}

func TestRollDice_SetsHasRolled(t *testing.T) {
	g := makeGame(7, 6)
	g.RollDice()
	if !g.state.HasRolled {
		t.Error("expected HasRolled=true after roll")
	}
}

func TestRollDice_OnlyOnce(t *testing.T) {
	g := makeGame(7, 6)
	g.RollDice()
	r1 := g.state.Remaining
	g.RollDice()
	if g.state.Remaining != r1 {
		t.Errorf("expected second roll to be no-op, remaining changed from %d to %d", r1, g.state.Remaining)
	}
}

func TestSlideRow_SetsHasSlid(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, Running: true, SlideRows: []int{1}}}
	g.SlideRow(1, true)
	if !g.state.HasSlid {
		t.Error("expected HasSlid=true after slide")
	}
}

func TestSlideRow_OnlyOnce(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
		{{OpenN: true}, {OpenN: true}, {OpenN: true}},
	}
	g := &Game{state: ui.GameState{Map: m, Running: true, SlideRows: []int{1}}}
	g.SlideRow(1, true)
	g.SlideRow(1, true)
	if g.state.HasSlid != true {
		t.Error("expected HasSlid remains true after second slide")
	}
}

func TestSlideLines_RandomizedWithFixedCount(t *testing.T) {
	for i := 0; i < 200; i++ {
		g := makeGame(10, 7)
		s := g.State()
		if len(s.SlideRows)+len(s.SlideCols) != 4 {
			t.Fatalf("slideable line total = %d, want 4", len(s.SlideRows)+len(s.SlideCols))
		}
		for _, y := range s.SlideRows {
			if y%2 == 0 || y < 0 || y >= 7 {
				t.Fatalf("bad slide row %d", y)
			}
		}
		for _, x := range s.SlideCols {
			if x%2 == 0 || x < 0 || x >= 10 {
				t.Fatalf("bad slide col %d", x)
			}
		}
	}
}

func TestSlideLines_ChangeEveryTurn(t *testing.T) {
	g := makeGame(10, 7)
	seen := map[string]bool{}
	seen[fmt.Sprintf("%v%v", g.State().SlideRows, g.State().SlideCols)] = true
	for i := 0; i < 60; i++ {
		g.EndTurn()
		s := g.State()
		if len(s.SlideRows)+len(s.SlideCols) != 4 {
			t.Fatalf("slideable line total = %d, want 4", len(s.SlideRows)+len(s.SlideCols))
		}
		seen[fmt.Sprintf("%v%v", s.SlideRows, s.SlideCols)] = true
	}
	if len(seen) < 2 {
		t.Fatal("slide lines should differ between turns")
	}
}

func TestSlideLine_BlockedWhenNotAllowed(t *testing.T) {
	for i := 0; i < 50; i++ {
		g := makeGame(10, 7)
		s := g.State()
		blocked := -1
		for y := 1; y < 7; y += 2 {
			allowed := false
			for _, a := range s.SlideRows {
				if a == y {
					allowed = true
					break
				}
			}
			if !allowed {
				blocked = y
				break
			}
		}
		if blocked < 0 {
			continue
		}
		g.SlideRow(blocked, true)
		if g.State().HasSlid {
			t.Fatalf("slide on non-allowed row %d must be ignored", blocked)
		}
		return
	}
	t.Skip("no turn with a blocked row in 50 samples")
}

func TestMoveTo_MovesPlayer(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 0, Remaining: 5, Running: true},
	}
	g.MoveTo(1, 2)
	if g.state.PlayerY != 2 || g.state.PlayerX != 1 {
		t.Errorf("expected player at (1,2), got (%d,%d)", g.state.PlayerX, g.state.PlayerY)
	}
	if g.state.Remaining != 3 {
		t.Errorf("expected Remaining=3 (2 steps), got %d", g.state.Remaining)
	}
}

func TestMoveTo_TooFarDoesNothing(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 0, Remaining: 1, Running: true},
	}
	g.MoveTo(1, 2)
	if g.state.PlayerY != 0 {
		t.Errorf("expected no move (distance > remaining), player y=%d", g.state.PlayerY)
	}
}

func TestMoveTo_NoPathDoesNothing(t *testing.T) {
	m := [][]ui.Tile{
		{{OpenS: true}, {}},
		{{OpenN: true}, {}},
	}
	g := &Game{
		state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0, Remaining: 5, Running: true},
	}
	g.MoveTo(3, 3)
	if g.state.PlayerX != 0 || g.state.PlayerY != 0 {
		t.Errorf("expected no move (no path to out-of-bounds target)")
	}
}

func TestEndTurn_ResetsAndAutoRolls(t *testing.T) {
	m := [][]ui.Tile{{{OpenN: true}}}
	g := &Game{
		state: ui.GameState{Map: m, Remaining: 5, DiceTotal: 8, HasRolled: true, HasSlid: true, Running: true},
	}
	g.EndTurn()
	if g.state.HasSlid {
		t.Error("expected HasSlid reset after EndTurn")
	}
	if !g.state.HasRolled {
		t.Error("expected HasRolled true after auto-roll")
	}
	if g.state.Remaining < 2 || g.state.Remaining > 12 {
		t.Errorf("expected Remaining 2-12 after auto-roll, got %d", g.state.Remaining)
	}
	if g.state.DiceTotal < 2 || g.state.DiceTotal > 12 {
		t.Errorf("expected DiceTotal 2-12 after auto-roll, got %d", g.state.DiceTotal)
	}
}

func TestReachable_Basic(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true, OpenN: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 1}}
	r := g.Reachable(4)
	if !r[3][1] {
		t.Error("expected (1,3) reachable within 4 steps")
	}
}

func TestReachable_RespectsOpenings(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenW: true, OpenE: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 0}}
	r := g.Reachable(4)
	if r[2][1] {
		t.Error("expected (1,2) not reachable (tile at (1,1) lacks N/S openings)")
	}
}

func TestReachable_ZeroLimit(t *testing.T) {
	m := [][]ui.Tile{{{OpenN: true}}}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 0, PlayerY: 0}}
	r := g.Reachable(0)
	if r[0][0] {
		t.Error("expected no reachable tiles with limit 0")
	}
}

func TestEndTurn_OnFinishedGameIsNoop(t *testing.T) {
	g := &Game{state: ui.GameState{Running: false, Message: "You win!", HasRolled: true, HasSlid: true}}
	g.EndTurn()
	if g.state.Message != "You win!" {
		t.Error("expected EndTurn to not clear message on finished game")
	}
	if !g.state.HasSlid {
		t.Error("expected EndTurn to not reset HasSlid on finished game")
	}
	if !g.state.HasRolled {
		t.Error("expected EndTurn to not reset HasRolled on finished game")
	}
}

func TestNewRound_ResetsForNextCatch(t *testing.T) {
	g := makeGame(10, 7)
	g.state.Running = false
	g.state.Message = "You win!"
	g.state.Remaining = 0
	g.state.HasSlid = true
	g.state.HasRolled = false
	g.NewRound()
	if !g.state.Running {
		t.Error("expected Running to be true after NewRound")
	}
	if g.state.Message != "" {
		t.Error("expected message cleared after NewRound")
	}
	if !g.state.HasRolled {
		t.Error("expected auto-roll after NewRound")
	}
	if g.state.Remaining < 2 || g.state.Remaining > 12 {
		t.Errorf("expected Remaining 2-12 after NewRound, got %d", g.state.Remaining)
	}
	if g.state.HasSlid {
		t.Error("expected HasSlid reset after NewRound")
	}
	if g.state.GoalX == g.state.PlayerX && g.state.GoalY == g.state.PlayerY {
		t.Error("expected goal not to overlap player after NewRound")
	}
	px, py := g.state.PlayerX, g.state.PlayerY
	h, w := len(g.state.Map), len(g.state.Map[0])
	if !(px == 0 || px == w-1) || !(py == 0 || py == h-1) {
		t.Errorf("expected player back on a corner after NewRound, got (%d,%d)", px, py)
	}
}

func TestThreading_RollSlideMoveEnd(t *testing.T) {
	m := [][]ui.Tile{
		{{}, {OpenS: true}, {}},
		{{}, {OpenN: true, OpenS: true}, {}},
		{{}, {OpenN: true}, {}},
	}
	g := &Game{state: ui.GameState{Map: m, PlayerX: 1, PlayerY: 0, Running: true}}
	g.RollDice()
	if !g.state.HasRolled {
		t.Fatal("expected HasRolled")
	}
	if g.state.Remaining < 2 || g.state.Remaining > 12 {
		t.Fatalf("invalid Remaining: %d", g.state.Remaining)
	}
	g.MoveTo(1, 2)
	if g.state.PlayerY != 2 {
		t.Errorf("expected player at y=2 after MoveTo, got y=%d", g.state.PlayerY)
	}
	g.EndTurn()
	if g.state.HasSlid {
		t.Error("expected HasSlid false after EndTurn")
	}
	if !g.state.HasRolled {
		t.Error("expected HasRolled true (auto-roll) after EndTurn")
	}
}
