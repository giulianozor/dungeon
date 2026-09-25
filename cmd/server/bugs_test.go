package main

import (
	"strings"
	"testing"
)

func TestSlideInvalidLineShiftsNoTokens(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	a, b := r.byPid["a"], r.byPid["b"]

	for _, row := range []int{0, 2, 4} { // even rows can never slide
		b.x, b.y = 2, row
		a.x, a.y = 0, 1
		if slid := r.slideActiveLocked(a, row, -1, true, false); slid {
			t.Fatalf("even row %d must not slide, got true", row)
		}
		if b.x != 2 || b.y != row {
			t.Fatalf("token shifted by invalid slide on row %d: b=(%d,%d)", row, b.x, b.y)
		}
	}
}

func TestSlideBlockedLineShiftsNoTokens(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	a, b := r.byPid["a"], r.byPid["b"]

	r.board.SetSlideLines([]int{1}, nil) // only row 1 is slideable this turn
	b.x, b.y = 2, 3
	a.x, a.y = 0, 1
	if slid := r.slideActiveLocked(a, 3, -1, true, false); slid {
		t.Fatal("blocked odd row must not slide, got true")
	}
	if b.x != 2 || b.y != 3 {
		t.Fatalf("token shifted by blocked slide: b=(%d,%d)", b.x, b.y)
	}
}

func TestSlideValidLineStillShiftsTokens(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	a, b := r.byPid["a"], r.byPid["b"]

	r.board.SetSlideLines([]int{1}, nil)
	b.x, b.y = 2, 1
	a.x, a.y = 0, 1
	if !r.slideActiveLocked(a, 1, -1, true, false) {
		t.Fatal("allowed slide must return true")
	}
	if b.y != 1 || b.x == 2 {
		t.Fatalf("token on allowed row must shift right, b=(%d,%d)", b.x, b.y)
	}
}

func TestSlideNoParamsDoesNothing(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A")
	m.join(r.id, "b", "B")
	a, b := r.byPid["a"], r.byPid["b"]
	b.x, b.y = 2, 1
	if r.slideActiveLocked(a, -1, -1, false, false) {
		t.Fatal("slide with no row/col must not happen")
	}
	if b.x != 2 || b.y != 1 {
		t.Fatalf("token moved without a slide: b=(%d,%d)", b.x, b.y)
	}
}

func TestSanitizeName_ClearsControlCharsAndCapsLength(t *testing.T) {
	m := newRoomManager()
	r := m.create("multi", 2, 2, "a", "A\r\nB\tC")
	m.join(r.id, "b", "   ")
	p0 := r.byPid["a"]
	for _, bad := range []rune{'\r', '\n', '\t'} {
		if strings.ContainsRune(p0.name, bad) {
			t.Fatalf("name contains control char: %q", p0.name)
		}
	}
	if p0.name != "ABC" {
		t.Fatalf("control chars not stripped, got %q", p0.name)
	}
	if r.byPid["b"].name != "Player 2" {
		t.Fatalf("blank name not defaulted, got %q", r.byPid["b"].name)
	}
	r.addPlayerLocked("c", strings.Repeat("x", 100))
	if got := len([]rune(r.byPid["c"].name)); got > 24 {
		t.Fatalf("name capped to 24 runes, got %d", got)
	}
}

func TestTruncateRunes_DoesNotSplitMultiByte(t *testing.T) {
	input := strings.Repeat("é", 300)
	got := truncateRunes(input, 200)
	if len([]rune(got)) != 200 {
		t.Fatalf("truncated runes = %d, want 200", len([]rune(got)))
	}
}
