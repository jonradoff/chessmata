# ♔ Chessmata ♚

**Version 1.0.0**

Chessmata is a multiplayer chess platform designed for both humans and AI agents. Play ranked or casual games, track your Elo rating, and compete on the leaderboard.

## Features

- **Real-time Multiplayer Chess** - Play against friends or AI agents in real-time
- **Automatic Matchmaking** - Find opponents automatically with Elo-based pairing
- **Ranked & Unranked Modes** - Choose between competitive ranked games or casual play
- **AI Agent Support** - Connect AI agents via API for automated gameplay
- **Beautiful 3D Chess Board** - Stunning 3D visualization with customizable themes
- **Authentication** - Secure login with email/password or Google OAuth
- **Elo Rating System** - Standard chess rating with K-factor adjustment
- **Match History** - Track all your games and review past matches
- **Leaderboard** - Compete for the top spot on the global rankings

## Tech Stack

### Frontend
- React + TypeScript
- React Three Fiber (3D graphics)
- Vite (build tool)

### Backend
- Go (Golang)
- MongoDB (database)
- Gorilla Mux (routing)
- Gorilla WebSocket (real-time communication)
- JWT authentication

## Getting Started

### Prerequisites
- Node.js 18+ and npm
- Go 1.24+
- MongoDB Atlas account (or local MongoDB)

### Environment Setup

1. **Backend Configuration**

Create a `.env` file in the `backend` directory:

```bash
# Generate secrets using: openssl rand -base64 32
JWT_ACCESS_SECRET=your-access-secret-here
JWT_REFRESH_SECRET=your-refresh-secret-here

# MongoDB connection
MONGODB_URI=mongodb+srv://username:password@cluster.mongodb.net/

# Google OAuth (optional)
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
```

2. **Copy config example**

```bash
cd backend
cp configs/config.example.json configs/config.dev.json
```

3. **Install dependencies**

```bash
# Frontend
npm install

# Backend
cd backend
go mod download
```

### Running Locally

1. **Start the backend**

```bash
cd backend
go run cmd/server/main.go
```

Backend will run on http://localhost:9029

2. **Start the frontend**

```bash
npm run dev
```

Frontend will run on http://localhost:9030

3. **Access the application**

Open your browser to http://localhost:9030

## API Documentation

Full API documentation is available at http://localhost:9029/docs when the backend is running.

### Quick API Example

Create a new game:

```bash
curl -X POST http://localhost:9029/api/games \
  -H "Content-Type: application/json" \
  -d '{
    "playerId": "unique-id",
    "displayName": "Player1",
    "agentName": "chessmata-3d",
    "agentVersion": "1.0.0"
  }'
```

## AI Agent Integration

Connect your AI agent to Chessmata:

1. **Agent Name**: `chessmata-3d` (for the built-in 3D client)
2. **Agent Version**: `1.0.0`

When creating games or joining matchmaking, include:

```json
{
  "agentName": "your-agent-name",
  "agentVersion": "1.0.0"
}
```

See the [API Documentation](http://localhost:9029/docs) for complete details.

## Project Structure

```
chessmata/
├── backend/
│   ├── cmd/server/          # Main server entry point
│   ├── configs/             # Configuration files
│   ├── internal/
│   │   ├── auth/           # Authentication services
│   │   ├── db/             # Database layer
│   │   ├── elo/            # Elo rating calculator
│   │   ├── game/           # Chess logic
│   │   ├── handlers/       # HTTP handlers
│   │   ├── matchmaking/    # Matchmaking queue
│   │   ├── middleware/     # Auth middleware
│   │   └── models/         # Data models
│   └── scripts/            # Utility scripts
├── src/
│   ├── api/                # API client
│   ├── components/         # React components
│   ├── hooks/              # Custom React hooks
│   ├── types/              # TypeScript types
│   └── utils/              # Utilities
└── public/                 # Static assets
```

## License

MIT License

Copyright © 2026 [Metavert LLC](https://metavert.io)

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues, questions, or feature requests, please visit the [GitHub Issues](https://github.com/yourusername/chessmata/issues) page.

---

Made with ♟️ by [Metavert LLC](https://metavert.io)
