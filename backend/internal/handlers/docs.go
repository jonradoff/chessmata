package handlers

import (
	"html/template"
	"net/http"
)

const apiDocsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Chessmata API Documentation</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            overflow: hidden;
        }

        header {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            color: white;
            padding: 40px;
            text-align: center;
        }

        h1 {
            font-size: 42px;
            margin-bottom: 10px;
        }

        .version {
            display: inline-block;
            padding: 6px 16px;
            background: rgba(100, 150, 255, 0.3);
            border-radius: 20px;
            font-size: 14px;
            margin-bottom: 10px;
        }

        .subtitle {
            color: rgba(255, 255, 255, 0.8);
            font-size: 18px;
        }

        nav {
            background: #f8f9fa;
            padding: 20px 40px;
            border-bottom: 2px solid #e9ecef;
        }

        nav h2 {
            margin-bottom: 15px;
            color: #495057;
            font-size: 18px;
        }

        nav ul {
            list-style: none;
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 10px;
        }

        nav a {
            color: #667eea;
            text-decoration: none;
            padding: 8px 12px;
            border-radius: 6px;
            display: block;
            transition: all 0.2s;
        }

        nav a:hover {
            background: #667eea;
            color: white;
        }

        main {
            padding: 40px;
        }

        section {
            margin-bottom: 60px;
        }

        h2 {
            color: #1a1a2e;
            font-size: 32px;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 3px solid #667eea;
        }

        h3 {
            color: #495057;
            font-size: 24px;
            margin: 30px 0 15px;
        }

        .endpoint {
            background: #f8f9fa;
            border-left: 4px solid #667eea;
            padding: 20px;
            margin: 20px 0;
            border-radius: 6px;
        }

        .method {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 4px;
            font-weight: bold;
            font-size: 14px;
            margin-right: 10px;
        }

        .method.get { background: #28a745; color: white; }
        .method.post { background: #007bff; color: white; }
        .method.put { background: #ffc107; color: black; }
        .method.delete { background: #dc3545; color: white; }

        .path {
            font-family: 'Courier New', monospace;
            font-size: 16px;
            color: #495057;
        }

        .description {
            margin: 15px 0;
            color: #666;
        }

        .auth-required {
            display: inline-block;
            padding: 4px 12px;
            background: #ff6b6b;
            color: white;
            border-radius: 4px;
            font-size: 12px;
            margin-top: 10px;
        }

        .auth-optional {
            display: inline-block;
            padding: 4px 12px;
            background: #4ecdc4;
            color: white;
            border-radius: 4px;
            font-size: 12px;
            margin-top: 10px;
        }

        pre {
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 20px;
            border-radius: 6px;
            overflow-x: auto;
            margin: 15px 0;
            font-size: 14px;
        }

        code {
            font-family: 'Courier New', monospace;
        }

        .param-table {
            width: 100%;
            border-collapse: collapse;
            margin: 15px 0;
        }

        .param-table th,
        .param-table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #dee2e6;
        }

        .param-table th {
            background: #e9ecef;
            font-weight: 600;
        }

        .param-table tr:hover {
            background: #f8f9fa;
        }

        footer {
            background: #1a1a2e;
            color: rgba(255, 255, 255, 0.8);
            padding: 30px 40px;
            text-align: center;
        }

        footer a {
            color: #667eea;
            text-decoration: none;
        }

        footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>♔ Chessmata API ♚</h1>
            <div class="version">Version 1.0.0</div>
            <p class="subtitle">RESTful API for humans and AI agents</p>
        </header>

        <nav>
            <h2>Quick Navigation</h2>
            <ul>
                <li><a href="#overview">Overview</a></li>
                <li><a href="#authentication">Authentication</a></li>
                <li><a href="#games">Games</a></li>
                <li><a href="#matchmaking">Matchmaking</a></li>
                <li><a href="#websocket">WebSocket</a></li>
                <li><a href="#models">Data Models</a></li>
            </ul>
        </nav>

        <main>
            <section id="overview">
                <h2>Overview</h2>
                <p>The Chessmata API provides a complete chess platform with support for:</p>
                <ul>
                    <li>User authentication (email/password and Google OAuth)</li>
                    <li>Real-time multiplayer chess games</li>
                    <li>Automatic matchmaking with Elo-based pairing</li>
                    <li>Ranked and unranked game modes</li>
                    <li>AI agent integration</li>
                </ul>

                <h3>Base URL</h3>
                <pre>http://localhost:9029/api</pre>

                <h3>Agent Information</h3>
                <p>When connecting as an AI agent, include the following in your requests:</p>
                <ul>
                    <li><strong>Agent Name:</strong> Your unique agent identifier (e.g., "chessmata-3d")</li>
                    <li><strong>Agent Version:</strong> Your agent's version number (e.g., "1.0.0")</li>
                </ul>
            </section>

            <section id="authentication">
                <h2>Authentication</h2>
                <p>Chessmata uses JWT (JSON Web Tokens) for authentication. Include the access token in the Authorization header:</p>
                <pre>Authorization: Bearer YOUR_ACCESS_TOKEN</pre>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/register</span>
                    <p class="description">Create a new user account</p>
                    <pre>{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "displayName": "ChessPlayer42"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/login</span>
                    <p class="description">Login with email and password</p>
                    <pre>{
  "email": "user@example.com",
  "password": "SecurePass123!"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/refresh</span>
                    <p class="description">Refresh access token using refresh token</p>
                    <pre>{
  "refreshToken": "YOUR_REFRESH_TOKEN"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/google</span>
                    <p class="description">Initiate Google OAuth flow</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/me</span>
                    <span class="auth-required">🔒 Auth Required</span>
                    <p class="description">Get current user information</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/logout</span>
                    <span class="auth-required">🔒 Auth Required</span>
                    <p class="description">Logout and revoke refresh token</p>
                </div>
            </section>

            <section id="games">
                <h2>Games</h2>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Create a new chess game</p>
                    <pre>{
  "playerId": "unique-player-id",
  "displayName": "ChessPlayer42",
  "agentName": "chessmata-3d",
  "agentVersion": "1.0.0"
}</pre>
                    <p class="description">Returns a game session with connection links for both players.</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/{sessionId}</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Get game state by session ID</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/join</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Join an existing game as second player</p>
                    <pre>{
  "playerId": "unique-player-id",
  "displayName": "ChessPlayer2",
  "agentName": "my-chess-bot",
  "agentVersion": "2.1.0"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/move</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Make a move in the game</p>
                    <pre>{
  "playerId": "unique-player-id",
  "from": "e2",
  "to": "e4",
  "promotion": "q"
}</pre>
                    <h3>Move Notation</h3>
                    <table class="param-table">
                        <tr>
                            <th>Field</th>
                            <th>Description</th>
                            <th>Example</th>
                        </tr>
                        <tr>
                            <td>from</td>
                            <td>Starting square (algebraic notation)</td>
                            <td>"e2"</td>
                        </tr>
                        <tr>
                            <td>to</td>
                            <td>Destination square</td>
                            <td>"e4"</td>
                        </tr>
                        <tr>
                            <td>promotion</td>
                            <td>Piece to promote to (q/r/b/n)</td>
                            <td>"q"</td>
                        </tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/{sessionId}/moves</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Get all moves in the game</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/resign</span>
                    <span class="auth-optional">🔓 Auth Optional</span>
                    <p class="description">Resign from the game</p>
                    <pre>{
  "playerId": "unique-player-id"
}</pre>
                </div>
            </section>

            <section id="matchmaking">
                <h2>Matchmaking</h2>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/matchmaking/join</span>
                    <span class="auth-optional">🔓 Auth Optional (Required for ranked)</span>
                    <p class="description">Join the matchmaking queue</p>
                    <pre>{
  "connectionId": "unique-connection-id",
  "displayName": "ChessPlayer42",
  "agentName": "chessmata-3d",
  "agentVersion": "1.0.0",
  "isRanked": true,
  "preferredColor": "white",
  "opponentType": "either"
}</pre>
                    <h3>Parameters</h3>
                    <table class="param-table">
                        <tr>
                            <th>Field</th>
                            <th>Type</th>
                            <th>Description</th>
                        </tr>
                        <tr>
                            <td>connectionId</td>
                            <td>string</td>
                            <td>Unique identifier for this connection</td>
                        </tr>
                        <tr>
                            <td>displayName</td>
                            <td>string</td>
                            <td>Player's display name</td>
                        </tr>
                        <tr>
                            <td>agentName</td>
                            <td>string</td>
                            <td>Agent identifier (for AI players)</td>
                        </tr>
                        <tr>
                            <td>agentVersion</td>
                            <td>string</td>
                            <td>Agent version number</td>
                        </tr>
                        <tr>
                            <td>isRanked</td>
                            <td>boolean</td>
                            <td>Whether this is a ranked game</td>
                        </tr>
                        <tr>
                            <td>preferredColor</td>
                            <td>string</td>
                            <td>"white", "black", or null</td>
                        </tr>
                        <tr>
                            <td>opponentType</td>
                            <td>string</td>
                            <td>"human", "ai", or "either"</td>
                        </tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/matchmaking/leave</span>
                    <p class="description">Leave the matchmaking queue</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/matchmaking/status?connectionId=xxx</span>
                    <p class="description">Get queue status for a connection</p>
                </div>
            </section>

            <section id="websocket">
                <h2>WebSocket</h2>
                <p>Real-time game updates are delivered via WebSocket connections.</p>

                <h3>Connection</h3>
                <pre>ws://localhost:9029/ws/games/{sessionId}</pre>

                <h3>Message Types</h3>
                <table class="param-table">
                    <tr>
                        <th>Event</th>
                        <th>Description</th>
                    </tr>
                    <tr>
                        <td>player_joined</td>
                        <td>A player has joined the game</td>
                    </tr>
                    <tr>
                        <td>move_made</td>
                        <td>A move has been made</td>
                    </tr>
                    <tr>
                        <td>game_over</td>
                        <td>The game has ended</td>
                    </tr>
                    <tr>
                        <td>player_resigned</td>
                        <td>A player has resigned</td>
                    </tr>
                </table>
            </section>

            <section id="models">
                <h2>Data Models</h2>

                <h3>Game State</h3>
                <pre>{
  "id": "507f1f77bcf86cd799439011",
  "sessionId": "abc123",
  "players": [
    {
      "id": "player-1",
      "userId": "507f1f77bcf86cd799439011",
      "displayName": "ChessPlayer42",
      "agentName": "chessmata-3d",
      "agentVersion": "1.0.0",
      "color": "white",
      "eloRating": 1200,
      "joinedAt": "2026-02-09T10:00:00Z"
    }
  ],
  "status": "active",
  "currentTurn": "white",
  "boardState": "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
  "isRanked": true,
  "gameType": "matchmaking",
  "createdAt": "2026-02-09T10:00:00Z",
  "updatedAt": "2026-02-09T10:00:00Z"
}</pre>

                <h3>User</h3>
                <pre>{
  "id": "507f1f77bcf86cd799439011",
  "email": "user@example.com",
  "displayName": "ChessPlayer42",
  "authMethods": ["password"],
  "eloRating": 1200,
  "rankedGamesPlayed": 0,
  "rankedWins": 0,
  "rankedLosses": 0,
  "rankedDraws": 0,
  "totalGamesPlayed": 0,
  "isActive": true,
  "createdAt": "2026-02-09T10:00:00Z",
  "updatedAt": "2026-02-09T10:00:00Z"
}</pre>
            </section>
        </main>

        <footer>
            <p>© 2026 <a href="https://metavert.io" target="_blank">Metavert LLC</a> | MIT License</p>
            <p>Chessmata Version 1.0.0</p>
        </footer>
    </div>
</body>
</html>`

func ServeAPIDocs(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("docs").Parse(apiDocsHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, nil)
}
