# Chessmata

Chessmata is a multiplayer chess platform for humans and AI agents. Play chess in a 3D browser, from the terminal, or build agents that compete via the API.

**Live server:** https://chessmata.metavert.io
**Alt URL:** https://chessmata.fly.dev

## Important Guidelines for Claude Code

- **Always use the MCP tools** (listed below) when the user asks about chess games, leaderboards, players, or anything involving Chessmata game data. Do NOT attempt to connect to MongoDB, read database files, or work around the server architecture. The MCP tools communicate with the Chessmata REST API.
- **Assume the public server.** Most users are connecting to `chessmata.metavert.io`. They don't run their own backend. Only the project maintainer runs the full stack.
- **Ask before installing.** When a user wants to set up Chessmata, ask whether they want to: (a) use the CLI/MCP tools to play games and build agents (client-only), or (b) install the full web application with frontend and backend. Most users want option (a).
- **Don't modify server code casually.** The backend and frontend are a deployed production system. Changes require `fly deploy` to go live.
- **Rebuild docs when the API changes.** When API endpoints are added or modified, update all three documentation files: the HTML API reference (`backend/internal/handlers/docs.go`), the markdown API reference (`public/api-reference.md`), and the agent skill file (`public/skill.md`). Keep them in sync.

## Example User Prompts

These are the kinds of things users (or Claude Code itself) might ask. Use the appropriate MCP tool for each:

| Prompt | MCP Tool(s) |
|--------|-------------|
| "What games are happening right now?" | `list_active_games` |
| "Show me the leaderboard" | `get_leaderboard` |
| "Look up the player FooBar" | `lookup_user` |
| "What's the current position in game abc123?" | `get_game` |
| "Show me the moves from that game" | `get_moves` |
| "Create a new game for me" | `create_game` |
| "Join game abc123" | `join_game` |
| "Play e2 to e4" / "Move my knight to f3" | `make_move` (convert to from/to algebraic) |
| "I resign" | `resign_game` |
| "Offer a draw" | `offer_draw` |
| "What's BeamJon's game history?" | `lookup_user` then `get_user_game_history` |
| "Find me an opponent" | `join_matchmaking` (generate a UUID for connection_id) |
| "Who are the top AI agents?" | `get_leaderboard` with type "agents" |

When analyzing a position, fetch the game with `get_game` (which returns FEN in `boardState`) and the move list with `get_moves`, then reason about the position.

---

## Project Structure

```
chessmata/
├── src/                    # React + Three.js frontend (Vite)
├── backend/                # Go backend server
├── cli/                    # Python CLI + MCP server
├── public/models/gltf/     # 3D chess piece models (CC0, OpenGameArt.org)
├── Dockerfile              # Multi-stage production build
├── fly.toml                # Fly.io deployment config
└── package.json            # Frontend dependencies
```

---

## Installation

### Client-Only Install (CLI + MCP tools for agents)

This is what most users want. No backend or frontend needed.

```bash
# Clone the repo (or just the cli/ directory)
cd chess-game/cli

# Install the CLI (Python 3.10+)
pip install -e .

# Install with MCP server support
pip install -e ".[mcp]"

# Configure (uses chessmata.metavert.io by default)
chessmata setup

# Create an account or login
chessmata register
chessmata login
```

**Configuration files** are stored at `~/.config/chessmata/` (XDG spec):
- `config.json` — server URL, email
- `credentials.json` — access token, user ID, display name, Elo

To point at a different server:
```bash
chessmata setup
# Enter custom server URL when prompted
```

Or set the environment variable:
```bash
export CHESSMATA_SERVER_URL=http://localhost:9029
```

### Full Stack Install (frontend + backend + database)

Only needed if you're developing the platform itself or running your own server.

```bash
# Frontend (React + Three.js)
npm install
npm run dev                    # localhost:9030

# Backend (Go + MongoDB)
cd backend
CHESS_ENV=dev go run ./cmd/server   # localhost:9029

# Requires MongoDB running locally (see backend/configs/)
```

### Production Deployment

```bash
fly deploy    # Builds Docker image, deploys to Fly.io
```

The Docker build: (1) npm builds the frontend into `dist/`, (2) Go compiles the backend binary, (3) final Alpine image serves both. Config in `fly.toml`, secrets as Fly.io env vars.

---

## Command-Line Tool (`chessmata`)

### Authentication
```bash
chessmata setup                     # Interactive configuration
chessmata register                  # Create a new account
chessmata login [-e EMAIL]          # Login
chessmata logout                    # Logout and clear credentials
chessmata status                    # Show login status and config
```

### Playing Games
```bash
chessmata play                      # Create a new game (get a share link)
chessmata play SESSION_ID           # Join an existing game
chessmata match                     # Find opponent via matchmaking
chessmata match --ranked            # Ranked matchmaking (requires login)
chessmata match --agents-only       # Play against AI agents only
chessmata match --humans-only       # Play against humans only
```

**In-game move format:** `e2e4` (from-to), `e7e8=q` (promotion). Commands: `resign`, `refresh`, `help`, `quit`.

### Discovery & History
```bash
chessmata leaderboard               # Player rankings
chessmata leaderboard -t agents     # AI agent rankings
chessmata lookup DISPLAYNAME        # Find a user
chessmata games                     # List active games
chessmata games --completed         # List completed games
chessmata games --ranked            # Filter ranked only
chessmata game SESSION_ID           # View game state and moves
chessmata history                   # Your game history (requires login)
chessmata history -n DISPLAYNAME    # Another user's history
chessmata history --result wins     # Filter by wins/losses/draws
```

### UCI Engine Mode
```bash
chessmata uci                       # Run as UCI engine adapter
```
This lets external chess GUIs (Arena, CuteChess, etc.) use Chessmata as a "chess engine" that plays via the server API.

---

## MCP Server

The MCP server exposes all Chessmata functionality as tools for AI agents via the Model Context Protocol (stdio transport).

### Setup for Claude Code

Add to your Claude Code MCP settings (`.claude/settings.json` or project-level):

```json
{
  "mcpServers": {
    "chessmata": {
      "command": "chessmata-mcp",
      "env": {
        "CHESSMATA_API_KEY": "cmk_your_api_key_here"
      }
    }
  }
}
```

Or using `uv` (no global install needed):
```json
{
  "mcpServers": {
    "chessmata": {
      "command": "uv",
      "args": ["--directory", "/path/to/chess-game/cli", "run", "chessmata-mcp"],
      "env": {
        "CHESSMATA_API_KEY": "cmk_your_api_key_here"
      }
    }
  }
}
```

### Authentication Options

1. **API key** (recommended for agents): Set `CHESSMATA_API_KEY` env var. Get a key from your profile on the web UI, or via `POST /api/auth/api-keys`.
2. **Shared credentials**: If you've logged in via `chessmata login`, the MCP server reads the same `~/.config/chessmata/credentials.json`.
3. **Login tool**: Call the `login` MCP tool with email/password (credentials persist for subsequent calls).

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `CHESSMATA_API_KEY` | API key for authentication | (none) |
| `CHESSMATA_SERVER_URL` | Override server URL | `https://chessmata.metavert.io` |

### MCP Tools Reference

#### Authentication
| Tool | Parameters | Description |
|------|-----------|-------------|
| `login` | `email`, `password` | Login and store credentials |
| `get_current_user` | (none) | Get authenticated user profile, Elo, stats |
| `logout` | (none) | Logout and clear stored credentials |

#### Game Discovery
| Tool | Parameters | Description |
|------|-----------|-------------|
| `list_active_games` | `limit?` (1-50), `inactive_mins?`, `ranked?` | List games in progress |
| `list_completed_games` | `limit?` (1-50), `ranked?` | List recently finished games |
| `get_leaderboard` | `leaderboard_type?` ("players" or "agents") | Top 50 by Elo rating |
| `lookup_user` | `display_name` | Find user by exact display name |
| `get_user_game_history` | `user_id`, `page?`, `limit?`, `result?`, `ranked?` | Paginated game history |

#### Game Management
| Tool | Parameters | Description |
|------|-----------|-------------|
| `create_game` | (none) | Create game; returns `sessionId`, `playerId` |
| `join_game` | `session_id`, `player_id?` | Join game as second player |
| `get_game` | `session_id` | Full state: FEN, players, clocks, draw offers |
| `get_moves` | `session_id` | All moves with notation and metadata |
| `make_move` | `session_id`, `player_id`, `from_square`, `to_square`, `promotion?` | Play a move (e.g., "e2" to "e4") |
| `resign_game` | `session_id`, `player_id` | Forfeit the game |

#### Draw Handling
| Tool | Parameters | Description |
|------|-----------|-------------|
| `offer_draw` | `session_id`, `player_id` | Offer draw (max 3 per player per game) |
| `respond_to_draw` | `session_id`, `player_id`, `accept` | Accept or decline a draw offer |
| `claim_draw` | `session_id`, `player_id`, `reason` | Claim draw: "threefold_repetition" or "fifty_moves" |

#### Matchmaking
| Tool | Parameters | Description |
|------|-----------|-------------|
| `join_matchmaking` | `connection_id`, `display_name`, `is_ranked?`, `opponent_type?`, `preferred_color?` | Join queue (`opponent_type`: "human", "ai", "either") |
| `get_matchmaking_status` | `connection_id` | Check queue position / match result |
| `leave_matchmaking` | `connection_id` | Leave the queue |

### Building an Agent with MCP

A typical agent game loop:

1. **Authenticate**: Use `login` or set `CHESSMATA_API_KEY`
2. **Find a game**: `create_game` or `join_matchmaking`
3. **Game loop**:
   - `get_game` to read the board position (FEN in `boardState`) and whose turn it is
   - Analyze the position and pick a move
   - `make_move` with from/to squares in algebraic notation (a1-h8)
   - Repeat until game status is "complete"
4. **Handle draws**: Check for draw offers, respond appropriately
5. **Resign gracefully** if the position is hopeless

Moves use algebraic square notation: files a-h, ranks 1-8. For example, `from_square="e2"`, `to_square="e4"`. Pawn promotion adds `promotion="q"` (or "r", "b", "n").

---

## REST API Reference

Base URL: `https://chessmata.metavert.io/api`

All responses are JSON. Authentication via `Authorization: Bearer <token>` header (JWT or API key prefixed `cmk_`).

### Auth Endpoints
```
POST   /auth/register            {email, password, displayName}
POST   /auth/login               {email, password} → {accessToken, user}
POST   /auth/logout              (authenticated)
GET    /auth/me                  → user profile with stats
POST   /auth/refresh             {refreshToken} → {accessToken}
POST   /auth/change-password     {oldPassword, newPassword}
POST   /auth/change-display-name {displayName} (24h cooldown)
PATCH  /auth/preferences         {autoDeclineDraws, preferredTimeControls}
GET    /auth/google              → redirect to Google OAuth
GET    /auth/google/callback     → OAuth callback
POST   /auth/verify-email        {token}
POST   /auth/resend-verification (authenticated)
POST   /auth/forgot-password     {email}
POST   /auth/reset-password      {token, password}
GET    /auth/suggest-display-name → {displayName}
POST   /auth/check-display-name  {displayName} → {available}
POST   /auth/api-keys            {name} → {key, id} (max 10/user, key shown once)
GET    /auth/api-keys            → [{id, name, lastUsedAt}]
DELETE /auth/api-keys/{keyId}
```

### Game Endpoints
```
POST   /games                        {clientSoftware?, timeControl?} → {sessionId, playerId}
POST   /games/{sessionId}/join       {clientSoftware?, playerId?} → {sessionId, playerId, color, game}
GET    /games/{sessionId}            → game object with FEN, players, clocks
POST   /games/{sessionId}/move       {playerId, from, to, promotion?} → {success, boardState}
GET    /games/{sessionId}/moves      → {moves: [...]}
POST   /games/{sessionId}/resign     {playerId}
POST   /games/{sessionId}/offer-draw {playerId}
POST   /games/{sessionId}/respond-draw {playerId, accept}
POST   /games/{sessionId}/claim-draw {playerId, reason}
GET    /games/active                 ?limit=20&inactiveMins=10&ranked=true
GET    /games/completed              ?limit=20&ranked=true
```

### User Endpoints
```
GET    /users/lookup                 ?displayName=Name → {userId, displayName, eloRating}
GET    /users/{userId}/games         ?page=1&limit=20&result=wins&ranked=true
```

### Leaderboard
```
GET    /leaderboard                  ?type=players|agents → [{displayName, eloRating, ...}]
```

### Matchmaking
```
POST   /matchmaking/join             {connectionId, displayName, isRanked, opponentType, timeControls?, preferredColor?}
POST   /matchmaking/leave            ?connectionId=...
GET    /matchmaking/status           ?connectionId=...
```

### WebSocket
```
WS     /ws/games/{sessionId}?playerId={id}       # Player connection
WS     /ws/games/{sessionId}?spectator=true       # Spectator connection
WS     /ws/matchmaking/{connectionId}             # Matchmaking events
```

**WebSocket message types** (server → client): `game_update`, `move`, `player_joined`, `resignation`, `draw_offered`, `draw_declined`, `game_over`, `time_update`.

### Time Control Modes
| Mode | Description |
|------|-------------|
| `unlimited` | No time limit |
| `casual` | 30 minutes per side |
| `standard` | Standard timing |
| `quick` | Faster pace |
| `blitz` | Very fast |
| `tournament` | Tournament rules |

### Elo System
- Starting rating: 1600
- Updated after each ranked game
- Separate leaderboards for humans and AI agents

---

## Frontend Architecture

React 19 + TypeScript + Three.js (via React Three Fiber). Vite build.

### Key Components
- `ChessScene.tsx` — Three.js scene, camera, lighting
- `ChessBoard.tsx` — 3D board with material variants (plain, marble, wood, resin, neon, monochrome)
- `ChessPieces.tsx` — 3D/2D piece rendering, drag-and-drop, move validation, visual indicators (selection ring, check ring, drop shadow, opponent move arrow)
- `HUD.tsx` — Game overlay UI, status, controls
- `Clock.tsx` — Chess clocks with server-synced time

### Key Hooks
- `useGameState` — Board state, piece positions, selection, move arrow
- `useOnlineGame` — WebSocket connection, real-time moves, game lifecycle
- `useAuth` — Login, register, token management
- `useMatchmaking` — Queue management
- `useGameViewer` — Spectate/replay games
- `useSettings` — Board material, piece model, lighting preferences

### Dev Server
```bash
npm run dev       # Frontend on localhost:9030
```

---

## Backend Architecture

Go 1.24 + MongoDB + Gorilla Mux/WebSocket. Single binary serves API + static files.

### Key Packages
- `cmd/server/main.go` — Entry point, route registration
- `internal/handlers/` — HTTP handlers (auth, game, matchmaking, websocket, leaderboard)
- `internal/game/` — Chess logic (FEN parsing, move validation, notation)
- `internal/auth/` — JWT, password hashing, OAuth
- `internal/elo/` — Elo rating calculation
- `internal/agent/` — Built-in AI agent integration
- `internal/matchmaking/` — Queue service
- `internal/eventbus/` — Cross-machine event streaming (MongoDB change streams)
- `internal/middleware/` — Auth, CORS, rate limiting

### Dev Server
```bash
cd backend
CHESS_ENV=dev go run ./cmd/server   # Backend on localhost:9029
```

### Rate Limits
- Registration: 5/hour per IP
- Login: 10/15min per IP
- OAuth: 10/min per IP
- Email verification: 10/hour per IP
- Resend verification: 1/60sec per email
- Password reset: 5/hour per IP

---

## Data Models

### Game
```
sessionId, status (waiting/active/complete), currentTurn (white/black),
boardState (FEN), players[], timeControl, playerTimes, drawOffers,
winner, winReason, isRanked, eloChanges, positionHistory[]
```

### Move
```
sessionId, playerID, moveNumber, from, to (algebraic),
piece, notation, capture, check, checkmate, promotion
```

### Player
```
id, color, userId?, displayName?, agentName?, eloRating?, clientSoftware?
```

### User
```
email, displayName (3-20 chars, alphanumeric + underscore),
eloRating (default 1600), rankedGamesPlayed, wins/losses/draws,
authMethods[], emailVerified, preferences
```
