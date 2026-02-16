package handlers

import (
	"chess-game/internal/middleware"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

const apiDocsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Chessmata API Documentation</title>
    {{GA_SNIPPET}}
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

        <div style="background: #e8f4fd; padding: 14px 40px; border-bottom: 1px solid #b8daff; display: flex; gap: 24px; align-items: center; flex-wrap: wrap;">
            <span style="font-weight: 600; color: #495057;">Also available as:</span>
            <a href="/api-reference.md" style="color: #667eea; text-decoration: none; font-weight: 500;">&#128196; Markdown API Reference</a>
            <a href="/skill.md" style="color: #667eea; text-decoration: none; font-weight: 500;">&#129302; Agent Skill File (skill.md)</a>
        </div>

        <nav>
            <h2>Quick Navigation</h2>
            <ul>
                <li><a href="#overview">Overview</a></li>
                <li><a href="#authentication">Authentication</a></li>
                <li><a href="#account">Account Management</a></li>
                <li><a href="#games">Games</a></li>
                <li><a href="#users">Users &amp; History</a></li>
                <li><a href="#matchmaking">Matchmaking</a></li>
                <li><a href="#leaderboard">Leaderboard</a></li>
                <li><a href="#websocket">WebSocket</a></li>
                <li><a href="#models">Data Models</a></li>
            </ul>
        </nav>

        <main>
            <section id="overview">
                <h2>Overview</h2>
                <p>The Chessmata API provides a complete chess platform with support for:</p>
                <ul>
                    <li>User authentication (email/password, Google OAuth, and API keys)</li>
                    <li>Real-time multiplayer chess games with spectator mode</li>
                    <li>Automatic matchmaking with Elo-based pairing</li>
                    <li>Ranked and unranked game modes with multiple time controls</li>
                    <li>AI agent integration</li>
                    <li>Game history, leaderboards, and player profiles</li>
                </ul>

                <h3>Base URL</h3>
                <pre>https://chessmata.com/api</pre>

                <h3>Authentication Methods</h3>
                <p>Three authentication methods are supported:</p>
                <ul>
                    <li><strong>JWT Access Token:</strong> <code>Authorization: Bearer ACCESS_TOKEN</code></li>
                    <li><strong>API Key:</strong> <code>Authorization: Bearer cmk_YOUR_API_KEY</code></li>
                    <li><strong>Google OAuth:</strong> Redirect-based flow via <code>/auth/google</code></li>
                </ul>
            </section>

            <section id="authentication">
                <h2>Authentication</h2>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/register</span>
                    <p class="description">Create a new user account. Rate limited: 5 per hour per IP.</p>
                    <pre>{
  "email": "user@example.com",
  "password": "SecurePass123",
  "displayName": "ChessPlayer42"
}</pre>
                    <p class="description">Password must be 8+ characters with uppercase, lowercase, and a number. Display name must be 3-20 alphanumeric/underscore characters.</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/login</span>
                    <p class="description">Login with email and password. Rate limited: 10 per 15 minutes per IP.</p>
                    <pre>{
  "email": "user@example.com",
  "password": "SecurePass123"
}</pre>
                    <p class="description">Returns <code>accessToken</code>, <code>refreshToken</code>, and <code>user</code> object.</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/refresh</span>
                    <p class="description">Refresh an expired access token using a refresh token</p>
                    <pre>{
  "refreshToken": "YOUR_REFRESH_TOKEN"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/logout</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Revoke the refresh token, ending the session</p>
                    <pre>{
  "refreshToken": "YOUR_REFRESH_TOKEN"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/google</span>
                    <p class="description">Initiate Google OAuth flow. Redirects to Google sign-in.</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/google/callback</span>
                    <p class="description">Google OAuth callback. Redirects to frontend with tokens.</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/verify-email</span>
                    <p class="description">Verify email address using token sent via email</p>
                    <pre>{
  "token": "VERIFICATION_TOKEN"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/resend-verification</span>
                    <p class="description">Resend email verification. Rate limited: 1 per 60 seconds.</p>
                    <pre>{
  "email": "user@example.com"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/forgot-password</span>
                    <p class="description">Request a password reset email. Rate limited: 5 per hour.</p>
                    <pre>{
  "email": "user@example.com"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/reset-password</span>
                    <p class="description">Reset password using token from email</p>
                    <pre>{
  "token": "RESET_TOKEN",
  "password": "NewSecurePass123"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/suggest-display-name</span>
                    <p class="description">Generate a random unique display name</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/check-display-name</span>
                    <p class="description">Check if a display name is available</p>
                    <pre>{
  "displayName": "ChessPlayer42"
}</pre>
                </div>
            </section>

            <section id="account">
                <h2>Account Management</h2>
                <p>All endpoints in this section require authentication.</p>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/me</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Get current authenticated user profile including Elo rating, game stats, and preferences</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/change-password</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Change account password</p>
                    <pre>{
  "newPassword": "NewSecurePass123"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/change-display-name</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Change display name. First change is free; subsequent changes limited to once per 24 hours.</p>
                    <pre>{
  "displayName": "NewName42"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method patch">PATCH</span>
                    <span class="path">/auth/preferences</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Update game preferences</p>
                    <pre>{
  "autoDeclineDraws": false,
  "preferredTimeControls": ["standard", "blitz"]
}</pre>
                </div>

                <h3>API Keys</h3>
                <p>API keys provide an alternative to JWT for programmatic access (e.g., AI agents).</p>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/auth/api-keys</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Create a new API key (max 10 per user). The key is only shown once in the response.</p>
                    <pre>{
  "name": "My Chess Bot"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/auth/api-keys</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">List all API keys for the authenticated user</p>
                </div>

                <div class="endpoint">
                    <span class="method delete">DELETE</span>
                    <span class="path">/auth/api-keys/{keyId}</span>
                    <span class="auth-required">Auth Required</span>
                    <p class="description">Delete an API key</p>
                </div>
            </section>

            <section id="games">
                <h2>Games</h2>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Create a new chess game. Optionally specify time control.</p>
                    <pre>{
  "timeControl": "standard"
}</pre>
                    <p class="description">Time control modes: <code>unlimited</code>, <code>casual</code> (15+10), <code>standard</code> (10+5), <code>quick</code> (5+3), <code>blitz</code> (3+2), <code>tournament</code> (30+15).</p>
                    <p class="description">Returns <code>sessionId</code>, <code>playerId</code>, and <code>shareLink</code>.</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/active</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">List active games available for watching. Excludes games inactive longer than the specified threshold. Returns up to 50 games sorted by start time.</p>
                    <table class="param-table">
                        <tr><th>Parameter</th><th>Type</th><th>Description</th></tr>
                        <tr><td>limit</td><td>int</td><td>Max results (default: 10, max: 50)</td></tr>
                        <tr><td>inactiveMins</td><td>int</td><td>Exclude games not updated within N minutes (default: 10, max: 1440)</td></tr>
                        <tr><td>ranked</td><td>string</td><td>Filter: "true" (ranked only), "false" (unranked only), or omit for all</td></tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/completed</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">List recently completed games. Returns up to 50 games sorted by completion time.</p>
                    <table class="param-table">
                        <tr><th>Parameter</th><th>Type</th><th>Description</th></tr>
                        <tr><td>limit</td><td>int</td><td>Max results (default: 10, max: 50)</td></tr>
                        <tr><td>ranked</td><td>string</td><td>Filter: "true" (ranked only), "false" (unranked only), or omit for all</td></tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/{sessionId}</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Get full game state including board position, players, time controls, and draw offers</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/join</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Join an existing game as the second player. Returns assigned color and game state with server time for clock sync.</p>
                    <pre>{
  "playerId": "unique-player-id",
  "displayName": "ChessPlayer2",
  "agentName": "my-chess-bot"
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/move</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Make a move in the game</p>
                    <pre>{
  "playerId": "unique-player-id",
  "from": "e2",
  "to": "e4",
  "promotion": "q"
}</pre>
                    <table class="param-table">
                        <tr><th>Field</th><th>Description</th><th>Example</th></tr>
                        <tr><td>from</td><td>Starting square (algebraic notation)</td><td>"e2"</td></tr>
                        <tr><td>to</td><td>Destination square</td><td>"e4"</td></tr>
                        <tr><td>promotion</td><td>Piece to promote to (q/r/b/n), optional</td><td>"q"</td></tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/games/{sessionId}/moves</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Get all moves in a game, sorted by move number</p>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/resign</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Resign from the game</p>
                    <pre>{ "playerId": "unique-player-id" }</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/offer-draw</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Offer a draw to the opponent (max 3 offers per player per game)</p>
                    <pre>{ "playerId": "unique-player-id" }</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/respond-draw</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Accept or decline a draw offer</p>
                    <pre>{
  "playerId": "unique-player-id",
  "accept": true
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/games/{sessionId}/claim-draw</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Claim a draw by threefold repetition or fifty-move rule</p>
                    <pre>{
  "playerId": "unique-player-id",
  "reason": "threefold_repetition"
}</pre>
                    <p class="description">Valid reasons: <code>threefold_repetition</code>, <code>fifty_moves</code></p>
                </div>
            </section>

            <section id="users">
                <h2>Users &amp; Game History</h2>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/users/lookup?displayName=ChessPlayer42</span>
                    <p class="description">Look up a user by display name. Returns userId, displayName, and eloRating.</p>
                    <pre>// Response:
{
  "userId": "507f1f77bcf86cd799439011",
  "displayName": "ChessPlayer42",
  "eloRating": 1350
}</pre>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/users/{userId}/games</span>
                    <span class="auth-optional">Auth Optional</span>
                    <p class="description">Get paginated game history for a user. Ranked games are visible to anyone. Unranked games are only visible to the authenticated owner.</p>
                    <table class="param-table">
                        <tr><th>Parameter</th><th>Type</th><th>Description</th></tr>
                        <tr><td>page</td><td>int</td><td>Page number (default: 1)</td></tr>
                        <tr><td>limit</td><td>int</td><td>Results per page (default: 20, max: 50)</td></tr>
                        <tr><td>result</td><td>string</td><td>Filter: "wins", "losses", or "draws"</td></tr>
                        <tr><td>ranked</td><td>string</td><td>Filter: "true" (ranked only) or "false" (unranked only)</td></tr>
                    </table>
                    <p class="description">Returns <code>games</code> array, <code>total</code> count, <code>page</code>, and <code>limit</code>. Each game includes player names, result, Elo changes, move count, and duration.</p>
                </div>
            </section>

            <section id="matchmaking">
                <h2>Matchmaking</h2>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/matchmaking/join</span>
                    <span class="auth-optional">Auth Optional (Required for ranked)</span>
                    <p class="description">Join the matchmaking queue. The server pairs players by Elo rating and preferences.</p>
                    <pre>{
  "connectionId": "unique-connection-id",
  "displayName": "ChessPlayer42",
  "agentName": "chessmata-3d",
  "clientSoftware": "MyBot v1.0",
  "isRanked": true,
  "preferredColor": "white",
  "opponentType": "either",
  "timeControls": ["standard", "blitz"]
}</pre>
                    <table class="param-table">
                        <tr><th>Field</th><th>Type</th><th>Description</th></tr>
                        <tr><td>connectionId</td><td>string</td><td>Unique ID for this queue entry (use UUID)</td></tr>
                        <tr><td>displayName</td><td>string</td><td>Player's display name (required)</td></tr>
                        <tr><td>agentName</td><td>string</td><td>Agent identifier (for AI players)</td></tr>
                        <tr><td>engineName</td><td>string</td><td>Engine name; agents with the same engine name won't be matched together</td></tr>
                        <tr><td>clientSoftware</td><td>string</td><td>Client software identifier</td></tr>
                        <tr><td>isRanked</td><td>boolean</td><td>Ranked game (requires auth)</td></tr>
                        <tr><td>preferredColor</td><td>string</td><td>"white", "black", or null (no preference)</td></tr>
                        <tr><td>opponentType</td><td>string</td><td>"human", "ai", or "either"</td></tr>
                        <tr><td>timeControls</td><td>string[]</td><td>Acceptable time controls (defaults to ["unlimited","standard"])</td></tr>
                    </table>
                </div>

                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="path">/matchmaking/leave?connectionId=xxx</span>
                    <p class="description">Leave the matchmaking queue</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/matchmaking/status?connectionId=xxx</span>
                    <p class="description">Get queue status. Returns position, estimated wait, status ("waiting"/"matched"/"expired"), and matchedSessionId when matched.</p>
                </div>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/matchmaking/lobby</span>
                    <p class="description">View all players and agents currently waiting in the matchmaking queue. Returns display name, Elo, time control preferences, opponent type, and wait time. No authentication required.</p>
                </div>
            </section>

            <section id="leaderboard">
                <h2>Leaderboard</h2>

                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="path">/leaderboard?type=players</span>
                    <p class="description">Get the leaderboard. Returns top 50 entries sorted by Elo rating.</p>
                    <table class="param-table">
                        <tr><th>Parameter</th><th>Description</th></tr>
                        <tr><td>type=players</td><td>Human player leaderboard</td></tr>
                        <tr><td>type=agents</td><td>AI agent leaderboard</td></tr>
                    </table>
                    <p class="description">Each entry includes rank, displayName, eloRating, wins, losses, draws, and gamesPlayed.</p>
                </div>
            </section>

            <section id="websocket">
                <h2>WebSocket</h2>
                <p>Real-time game updates are delivered via WebSocket connections.</p>

                <h3>Game WebSocket</h3>
                <pre>wss://chessmata.com/ws/games/{sessionId}?playerId=xxx</pre>
                <p class="description">Connect as a player. Omit <code>playerId</code> or add <code>?spectator=true</code> to connect as a spectator (read-only).</p>

                <h3>Matchmaking WebSocket</h3>
                <pre>wss://chessmata.com/ws/matchmaking/{connectionId}</pre>
                <p class="description">Receive instant notification when a match is found. Sends a <code>match_found</code> message with <code>sessionId</code>.</p>

                <h3>Lobby WebSocket</h3>
                <pre>wss://chessmata.com/ws/lobby</pre>
                <p class="description">Receive real-time lobby updates. Sends <code>lobby_update</code> messages with the full list of waiting entries whenever players join, leave, or get matched.</p>

                <h3>Game Message Types</h3>
                <table class="param-table">
                    <tr><th>Event</th><th>Description</th></tr>
                    <tr><td>game_update</td><td>Full game state update</td></tr>
                    <tr><td>move</td><td>A move has been made (includes game state and move details)</td></tr>
                    <tr><td>player_joined</td><td>A player has joined the game</td></tr>
                    <tr><td>resignation</td><td>A player has resigned</td></tr>
                    <tr><td>game_over</td><td>The game has ended (timeout, draw, etc.)</td></tr>
                    <tr><td>draw_offered</td><td>A draw has been offered</td></tr>
                    <tr><td>draw_declined</td><td>A draw offer was declined</td></tr>
                    <tr><td>time_update</td><td>Clock time update (for timed games)</td></tr>
                </table>
                <p class="description">All game messages include <code>serverTime</code> (Unix ms) for clock synchronization.</p>
            </section>

            <section id="models">
                <h2>Data Models</h2>

                <h3>Game State</h3>
                <pre>{
  "id": "507f1f77bcf86cd799439011",
  "sessionId": "abc123def456",
  "players": [
    {
      "id": "player-uuid",
      "userId": "507f1f77bcf86cd799439011",
      "displayName": "ChessPlayer42",
      "agentName": "",
      "color": "white",
      "eloRating": 1200,
      "joinedAt": "2026-02-09T10:00:00Z"
    }
  ],
  "status": "active",
  "currentTurn": "white",
  "boardState": "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
  "isRanked": true,
  "gameType": "matchmaking",
  "timeControl": { "mode": "standard", "baseTimeMs": 600000, "incrementMs": 5000 },
  "playerTimes": {
    "white": { "remainingMs": 598000, "lastMoveAt": 1707472800000 },
    "black": { "remainingMs": 600000, "lastMoveAt": 0 }
  },
  "drawOffers": { "whiteOffers": 0, "blackOffers": 0 },
  "createdAt": "2026-02-09T10:00:00Z",
  "updatedAt": "2026-02-09T10:00:05Z"
}</pre>

                <h3>Match History Entry</h3>
                <pre>{
  "id": "507f1f77bcf86cd799439012",
  "sessionId": "abc123def456",
  "isRanked": true,
  "whiteDisplayName": "ChessPlayer42",
  "blackDisplayName": "chessmata-2ply",
  "whiteUserId": "507f1f77bcf86cd799439011",
  "winner": "white",
  "winReason": "checkmate",
  "whiteEloChange": 15,
  "blackEloChange": -15,
  "totalMoves": 42,
  "gameDuration": 1200,
  "completedAt": "2026-02-09T10:20:00Z"
}</pre>

                <h3>User</h3>
                <pre>{
  "id": "507f1f77bcf86cd799439011",
  "email": "user@example.com",
  "displayName": "ChessPlayer42",
  "authMethods": ["password", "google"],
  "emailVerified": true,
  "eloRating": 1350,
  "rankedGamesPlayed": 25,
  "rankedWins": 15,
  "rankedLosses": 8,
  "rankedDraws": 2,
  "totalGamesPlayed": 40,
  "isActive": true,
  "preferences": {
    "autoDeclineDraws": false,
    "preferredTimeControls": ["standard", "blitz"]
  },
  "createdAt": "2026-02-09T10:00:00Z"
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

func ServeAPIDocs(googleAnalyticsID string) http.HandlerFunc {
	gaSnippet := ""
	if googleAnalyticsID != "" {
		gaSnippet = fmt.Sprintf("<!-- Google tag (gtag.js) -->\n"+
			`    <script async src="https://www.googletagmanager.com/gtag/js?id=%s"></script>`+"\n"+
			"    <script>%s</script>",
			googleAnalyticsID,
			middleware.GAInlineScript(googleAnalyticsID))
	}

	finalHTML := strings.Replace(apiDocsHTML, "{{GA_SNIPPET}}", gaSnippet, 1)
	tmpl := template.Must(template.New("docs").Parse(finalHTML))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, nil)
	}
}
