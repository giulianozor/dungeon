# Dungeon Game

A tile-based dungeon maze game with a web interface. Roll the dice, traverse
a grid of corridor tiles, and race to catch the goal before the other players.
Tiles are generated randomly at game start and rendered as cartoonish stone
brick and floor PNGs.

## Features

- **Single player** and **multiplayer** (2–4 players) modes.
- **Turn-based play**: roll two dice, then move up to the total, sneaking a
  row/column slide in before your turn ends.
- **Shared board**: every player in a room sits on the same 9×6 maze, shown
  with a distinct token so everyone can see who is where.
- **Goal catch**: reaching the goal scores a point and the goal teleports to a
  new random cell while the catcher keeps their position. Play until the goal
  limit is reached, then view the leaderboard.
- **Sliding rows/columns**: each turn a random subset of odd rows and columns
  can be shifted — the number of slideable lines stays fixed, but *which* ones
  change every turn.
- **Lobby/chat** so players can coordinate before and during the match.

## Architecture

```
cmd/server/
  main.go       - HTTP server, API routes, static page serving, tile cache
  rooms.go      - room/session management, multiplayer turns, tokens, chat
  tiles.go      - PNG tile image generator (brick walls, textured floor)
  static/
    index.html  - new game page (start single player or host/join multi)
    game.html   - play page (lobby, board, dice, slides, turn indicator)
    end.html    - game-over page with leaderboard
internal/
  game/game.go  - game state, map generation, movement, slides, reachability
  ui/
    renderer.go - Tile, GameState types shared by game and server
```

The server exposes JSON endpoints (`/api/create`, `/api/join`, `/api/room`,
`/api/state`, `/api/move`, `/api/roll`, `/api/slide`, `/api/endturn`,
`/api/chat`, `/api/leave`, `/api/leaderboard`) plus `/tile/N`, which serves
the generated PNG for a tile whose openings are encoded in the bitmask `N`
(N=1, S=2, E=4, W=8).

## Building & Running

```bash
make build   # build the server binary
make run     # build and start on http://localhost:8080
make test    # run tests
make clean   # remove build artifacts
make lint    # run staticcheck
make fmt     # format all Go source files
make tidy    # tidy go.mod
```

Open http://localhost:8080 in your browser to play.
