package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/giulianozor/dungeon/internal/ui"
)

//go:embed static/*
var staticFiles embed.FS

type tileKey struct {
	N bool
	S bool
	E bool
	W bool
}

var tileCache struct {
	sync.RWMutex
	m map[tileKey][]byte
}

func init() {
	tileCache.m = make(map[tileKey][]byte)
}

func encodeTile(openN, openS, openE, openW bool) []byte {
	key := tileKey{openN, openS, openE, openW}

	tileCache.RLock()
	b, ok := tileCache.m[key]
	tileCache.RUnlock()
	if ok {
		return b
	}

	img := generateTile(openN, openS, openE, openW)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	b = buf.Bytes()

	tileCache.Lock()
	if cached, ok := tileCache.m[key]; ok {
		b = cached
	} else {
		tileCache.m[key] = b
	}
	tileCache.Unlock()
	return b
}

type gameServer struct {
	rm *roomManager
}

type stateResponse struct {
	Map       [][]ui.Tile `json:"map"`
	PlayerX   int         `json:"playerX"`
	PlayerY   int         `json:"playerY"`
	GoalX     int         `json:"goalX"`
	GoalY     int         `json:"goalY"`
	Message   string      `json:"message"`
	Running   bool        `json:"running"`
	Remaining int         `json:"remaining"`
	HasRolled bool        `json:"hasRolled"`
	HasSlid   bool        `json:"hasSlid"`
	SlideRows []int       `json:"slideRows"`
	SlideCols []int       `json:"slideCols"`
	Width     int         `json:"width"`
	Height    int         `json:"height"`
	Reachable [][]bool    `json:"reachable"`
	Room      roomInfo    `json:"room"`
}

// boardView reports which player's game every connected screen should render.
// During a live multiplayer round that is always the current actor, so the
// board (positions, goal, maze) is identical on every screen and updates on
// all of them after each move. Before the game starts, or in single-player
// mode, the requester sees their own game.
func (gs *gameServer) boardView(rm *room, requester *player) *player {
	if rm.mode == "multi" && rm.started && !rm.finished && len(rm.players) > 0 {
		return rm.players[rm.curTurn]
	}
	return requester
}

func (gs *gameServer) stateRes(rm *room, requester *player) stateResponse {
	actor := gs.boardView(rm, requester)
	rm.installToken(actor)
	s := rm.board.State()
	h := len(s.Map)
	w := len(s.Map[0])
	return stateResponse{
		Map:       s.Map,
		PlayerX:   s.PlayerX,
		PlayerY:   s.PlayerY,
		GoalX:     s.GoalX,
		GoalY:     s.GoalY,
		Message:   s.Message,
		Running:   s.Running,
		Remaining: s.Remaining,
		HasRolled: s.HasRolled,
		HasSlid:   s.HasSlid,
		SlideRows: s.SlideRows,
		SlideCols: s.SlideCols,
		Width:     w,
		Height:    h,
		Reachable: rm.board.Reachable(s.Remaining),
		Room:      rm.infoLocked(requester.pid),
	}
}

// resolve finds the room and the requesting player, locking the room.
// The caller must defer rm.mu.Unlock().
func (gs *gameServer) resolve(w http.ResponseWriter, r *http.Request) (*room, *player, bool) {
	roomID := r.URL.Query().Get("room")
	pid := r.URL.Query().Get("pid")
	if roomID == "" || pid == "" {
		http.Error(w, "room and pid required", http.StatusBadRequest)
		return nil, nil, false
	}
	rm, ok := gs.rm.get(roomID)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return nil, nil, false
	}
	rm.mu.Lock()
	p := rm.byPid[pid]
	if p == nil {
		rm.mu.Unlock()
		http.Error(w, "you are not in this game", http.StatusForbidden)
		return nil, nil, false
	}
	p.lastSeen = time.Now()
	return rm, p, true
}

func nameParam(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("name"))
}

// truncateRunes caps a string at n runes without splitting a multi-byte char.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n])
	}
	return s
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func main() {
	host := flag.String("host", "0.0.0.0", "interface to listen on")
	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	gs := &gameServer{rm: newRoomManager()}

	// Drop multiplayer players whose connection went stale so their board
	// is freed up; goal progress is kept.
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			gs.rm.sweep(60 * time.Second)
		}
	}()

	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("json encode error: %v", err)
		}
	}

	writeErr := func(w http.ResponseWriter, code int, msg string) {
		writeJSON(w, map[string]string{"error": msg})
	}

	mux.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		pid := r.URL.Query().Get("pid")
		if pid == "" {
			http.Error(w, "pid required", http.StatusBadRequest)
			return
		}
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "single"
		}
		name := nameParam(r)
		if mode == "multi" && name == "" {
			writeErr(w, http.StatusBadRequest, "please choose a name")
			return
		}
		if mode == "single" && name == "" {
			name = "Player 1"
		}
		maxPlayers := 1
		if mode == "multi" {
			maxPlayers = atoiDefault(r.URL.Query().Get("players"), 2)
			if maxPlayers < 2 {
				maxPlayers = 2
			}
			if maxPlayers > 4 {
				maxPlayers = 4
			}
		}
		goals := atoiDefault(r.URL.Query().Get("goals"), 3)
		if goals < 0 {
			goals = 0
		}
		rm := gs.rm.create(mode, maxPlayers, goals, pid, name)
		writeJSON(w, map[string]interface{}{
			"id":   rm.id,
			"room": rm.info(pid),
		})
	})

	mux.HandleFunc("/api/join", func(w http.ResponseWriter, r *http.Request) {
		pid := r.URL.Query().Get("pid")
		id := r.URL.Query().Get("room")
		if pid == "" || id == "" {
			http.Error(w, "room and pid required", http.StatusBadRequest)
			return
		}
		name := nameParam(r)
		if name == "" {
			writeErr(w, http.StatusBadRequest, "please choose a name")
			return
		}
		rm, err := gs.rm.join(id, pid, name)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]interface{}{
			"id":   rm.id,
			"room": rm.info(pid),
		})
	})

	mux.HandleFunc("/api/leave", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		rm.removePlayerLocked(p.pid)
		rm.addChat("", fmt.Sprintf("%s left", p.name))
		writeJSON(w, map[string]bool{"left": true})
	})

	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		text := strings.TrimSpace(r.URL.Query().Get("text"))
		if text != "" {
			text = truncateRunes(text, 200)
			rm.addChat(p.name, text)
		}
		writeJSON(w, rm.infoLocked(p.pid))
	})

	mux.HandleFunc("/api/room", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		writeJSON(w, rm.infoLocked(p.pid))
	})

	mux.HandleFunc("/api/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("room")
		if id == "" {
			http.Error(w, "room required", http.StatusBadRequest)
			return
		}
		rm, ok := gs.rm.get(id)
		if !ok {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}
		writeJSON(w, rm.leaderboard())
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		writeJSON(w, gs.stateRes(rm, p))
	})

	mux.HandleFunc("/api/move", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		if rm.isActor(p) {
			rm.installToken(p)
			if dir := r.URL.Query().Get("dir"); dir != "" {
				switch dir {
				case "up":
					rm.board.Move(0, -1)
				case "down":
					rm.board.Move(0, 1)
				case "left":
					rm.board.Move(-1, 0)
				case "right":
					rm.board.Move(1, 0)
				}
			} else if xStr := r.URL.Query().Get("x"); xStr != "" {
				if yStr := r.URL.Query().Get("y"); yStr != "" {
					x, err1 := strconv.Atoi(xStr)
					y, err2 := strconv.Atoi(yStr)
					if err1 == nil && err2 == nil {
						rm.board.MoveTo(x, y)
					}
				}
			}
			rm.readToken(p)
			rm.afterAction()
		}
		writeJSON(w, gs.stateRes(rm, p))
	})

	mux.HandleFunc("/api/roll", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		d1, d2 := 0, 0
		if rm.isActor(p) {
			d1, d2 = rm.board.RollDice()
		}
		res := gs.stateRes(rm, p)
		writeJSON(w, map[string]interface{}{
			"d1":    d1,
			"d2":    d2,
			"state": res,
		})
	})

	mux.HandleFunc("/api/slide", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		row, col := -1, -1
		right, down := false, false
		if rowStr := r.URL.Query().Get("row"); rowStr != "" {
			if v, err := strconv.Atoi(rowStr); err == nil {
				right = r.URL.Query().Get("dir") == "right"
				row = v
			}
		} else if colStr := r.URL.Query().Get("col"); colStr != "" {
			if v, err := strconv.Atoi(colStr); err == nil {
				down = r.URL.Query().Get("dir") == "down"
				col = v
			}
		}
		if rm.isActor(p) {
			rm.slideActiveLocked(p, row, col, right, down)
		}
		writeJSON(w, gs.stateRes(rm, p))
	})

	mux.HandleFunc("/api/endturn", func(w http.ResponseWriter, r *http.Request) {
		rm, p, ok := gs.resolve(w, r)
		if !ok {
			return
		}
		defer rm.mu.Unlock()
		if rm.isActor(p) {
			rm.endTurn()
		}
		writeJSON(w, gs.stateRes(rm, p))
	})

	mux.HandleFunc("/tile/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/tile/"):]
		if len(id) < 1 {
			http.NotFound(w, r)
			return
		}
		n, err := strconv.Atoi(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		openN := n&1 != 0
		openS := n&2 != 0
		openE := n&4 != 0
		openW := n&8 != 0

		w.Header().Set("Content-Type", "image/png")
		w.Write(encodeTile(openN, openS, openE, openW))
	})

	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("cannot resolve static FS: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))

	addr := *host + ":" + *port
	log.Printf("Game server starting on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
