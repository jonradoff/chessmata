# Chessmata CLI

A terminal-based chess client for the Chessmata server. Play chess from your command line with full support for authentication, real-time updates, and matchmaking.

## Features

- **No external dependencies** - Uses only Python standard library
- **Interactive gameplay** - Full chess board rendering in terminal with Unicode pieces
- **Real-time updates** - WebSocket support for instant opponent moves
- **Authentication** - Login/register to track your Elo rating
- **Matchmaking** - Find opponents automatically with ranking preferences
- **Cross-platform** - Works on macOS, Linux, and Windows

## Installation

### From source

```bash
cd cli
pip install -e .
```

### Direct execution

```bash
cd cli
python -m chessmata
```

## Quick Start

1. **Initial setup** (optional - configures server URL):
   ```bash
   chessmata setup
   ```

2. **Create an account** (optional - for ranked games):
   ```bash
   chessmata register
   ```

3. **Login** (optional - for ranked games):
   ```bash
   chessmata login
   ```

4. **Create a new game**:
   ```bash
   chessmata play
   ```
   This creates a game and gives you a link to share with your opponent.

5. **Join an existing game**:
   ```bash
   chessmata play <session-id>
   # Or with full URL:
   chessmata play https://chessmata.metavert.io/game/abc123
   ```

6. **Find opponent via matchmaking**:
   ```bash
   chessmata match              # Casual game
   chessmata match --ranked     # Ranked game (requires login)
   chessmata match --humans-only   # Only human opponents
   ```

## Commands

| Command | Description |
|---------|-------------|
| `chessmata setup` | Configure the CLI (server URL, username) |
| `chessmata register` | Create a new Chessmata account |
| `chessmata login` | Login to your account |
| `chessmata logout` | Logout from your account |
| `chessmata status` | Show current login status and config |
| `chessmata play` | Create a new game |
| `chessmata play <session>` | Join an existing game |
| `chessmata match` | Find opponent via matchmaking |

## Gameplay

### Move Input

Enter moves in coordinate notation:
- `e2e4` or `e2-e4` or `e2 e4` - Move piece from e2 to e4
- `e7e8q` or `e7e8=q` - Pawn promotion (to queen)

### In-Game Commands

- `resign` - Resign the game
- `refresh` - Refresh game state
- `help` - Show help
- `quit` - Exit the game

### Board Display

The board is displayed with:
- Unicode chess pieces (with ASCII fallback)
- Colored squares (light tan / dark brown)
- Highlighted last move (yellow)
- Check indicator (red background on king)
- Board oriented from your perspective

## Configuration

Configuration is stored in `~/.config/chessmata/` (or `%APPDATA%\chessmata\` on Windows):

- `config.json` - Server URL and username
- `credentials.json` - Authentication tokens (permissions: 600)

## Requirements

- Python 3.8 or higher
- Terminal with ANSI color support (most modern terminals)
- Optional: Unicode support for piece symbols

## License

MIT License
