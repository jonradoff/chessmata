"""Chessmata MCP Server - Expose the Chessmata chess API to AI agents via MCP.

Run with: chessmata-mcp (stdio transport)
Or: python -m chessmata.mcp_server
"""

import json
import os
from typing import Optional

from mcp.server.fastmcp import FastMCP

from .api import ChessmataAPI, APIError
from .config import Config, Credentials

mcp = FastMCP("chessmata")


def _get_api() -> ChessmataAPI:
    """Create API client from config/credentials, supporting env var overrides."""
    config = Config.load()
    server_url = os.environ.get("CHESSMATA_SERVER_URL")
    if server_url:
        config.server_url = server_url

    credentials = Credentials.load()
    api_key = os.environ.get("CHESSMATA_API_KEY")
    if api_key and not credentials.is_logged_in():
        credentials.access_token = api_key

    return ChessmataAPI(config, credentials)


def _error(e: APIError) -> str:
    return json.dumps({"error": str(e), "status_code": e.status_code})


def _game_to_dict(game) -> dict:
    """Convert a Game dataclass to a JSON-serializable dict."""
    players = []
    for p in game.players:
        players.append({
            "id": p.id,
            "color": p.color,
            "userId": p.user_id,
            "displayName": p.display_name,
            "agentName": p.agent_name,
            "eloRating": p.elo_rating,
        })

    result = {
        "id": game.id,
        "sessionId": game.session_id,
        "players": players,
        "status": game.status,
        "currentTurn": game.current_turn,
        "boardState": game.board_state,
        "winner": game.winner,
        "winReason": game.win_reason,
        "isRanked": game.is_ranked,
        "createdAt": game.created_at,
        "updatedAt": game.updated_at,
    }

    if game.elo_changes:
        result["eloChanges"] = {
            "whiteChange": game.elo_changes.white_change,
            "blackChange": game.elo_changes.black_change,
            "whiteNewElo": game.elo_changes.white_new_elo,
            "blackNewElo": game.elo_changes.black_new_elo,
        }

    if game.time_control:
        result["timeControl"] = {
            "mode": game.time_control.mode,
            "baseTimeMs": game.time_control.base_time_ms,
            "incrementMs": game.time_control.increment_ms,
        }

    if game.player_times:
        result["playerTimes"] = {
            "whiteRemainingMs": game.player_times.white_remaining_ms,
            "blackRemainingMs": game.player_times.black_remaining_ms,
        }

    return result


def _move_to_dict(move) -> dict:
    """Convert a Move dataclass to a JSON-serializable dict."""
    return {
        "moveNumber": move.move_number,
        "from": move.from_square,
        "to": move.to_square,
        "piece": move.piece,
        "notation": move.notation,
        "capture": move.capture,
        "check": move.check,
        "checkmate": move.checkmate,
        "promotion": move.promotion,
    }


# ── Authentication ──────────────────────────────────────────────────────────


@mcp.tool()
def login(email: str, password: str) -> str:
    """Login to Chessmata with email and password. Stores credentials for subsequent tool calls.

    Args:
        email: Account email address
        password: Account password
    """
    try:
        api = _get_api()
        creds = api.login(email, password)
        return json.dumps({
            "success": True,
            "displayName": creds.display_name,
            "email": creds.email,
            "userId": creds.user_id,
            "eloRating": creds.elo_rating,
        })
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_current_user() -> str:
    """Get the currently authenticated user's profile including Elo rating and game stats."""
    try:
        api = _get_api()
        data = api.get_current_user()
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def logout() -> str:
    """Logout from Chessmata and clear stored credentials."""
    try:
        api = _get_api()
        api.logout()
        return json.dumps({"success": True})
    except APIError as e:
        return _error(e)


# ── Game Discovery ──────────────────────────────────────────────────────────


@mcp.tool()
def list_active_games(
    limit: int = 20,
    inactive_mins: int = 10,
    ranked: Optional[str] = None,
) -> str:
    """List currently active games that can be watched or joined.

    Args:
        limit: Maximum number of games to return (1-50, default 20)
        inactive_mins: Exclude games inactive longer than this many minutes (default 10)
        ranked: Filter by "true" (ranked only), "false" (unranked only), or omit for all
    """
    try:
        api = _get_api()
        games = api.list_active_games(limit=limit, inactive_mins=inactive_mins, ranked=ranked)
        return json.dumps(games, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def list_completed_games(
    limit: int = 20,
    ranked: Optional[str] = None,
) -> str:
    """List recently completed games.

    Args:
        limit: Maximum number of games to return (1-50, default 20)
        ranked: Filter by "true" (ranked only), "false" (unranked only), or omit for all
    """
    try:
        api = _get_api()
        games = api.list_completed_games(limit=limit, ranked=ranked)
        return json.dumps(games, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_leaderboard(leaderboard_type: str = "players") -> str:
    """Get the leaderboard sorted by Elo rating.

    Args:
        leaderboard_type: "players" for human leaderboard, "agents" for AI agent leaderboard
    """
    try:
        api = _get_api()
        entries = api.get_leaderboard(leaderboard_type)
        return json.dumps(entries, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def lookup_user(display_name: str) -> str:
    """Look up a user by their display name. Returns userId, displayName, and eloRating.

    Args:
        display_name: The display name to search for (exact match)
    """
    try:
        api = _get_api()
        data = api.lookup_user(display_name)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_user_game_history(
    user_id: str,
    page: int = 1,
    limit: int = 20,
    result: Optional[str] = None,
    ranked: Optional[str] = None,
) -> str:
    """Get paginated game history for a user.

    Args:
        user_id: The user's MongoDB ObjectID
        page: Page number (default 1)
        limit: Results per page (default 20, max 50)
        result: Filter by "wins", "losses", or "draws"
        ranked: Filter by "true" (ranked only) or "false" (unranked only)
    """
    try:
        api = _get_api()
        data = api.get_user_game_history(
            user_id, page=page, limit=limit, result=result, ranked=ranked,
        )
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


# ── Game Management ─────────────────────────────────────────────────────────


@mcp.tool()
def create_game() -> str:
    """Create a new chess game. Returns sessionId and playerId. Share the sessionId for an opponent to join."""
    try:
        api = _get_api()
        session_id, player_id = api.create_game()
        return json.dumps({
            "sessionId": session_id,
            "playerId": player_id,
            "shareLink": f"{api.base_url}/game/{session_id}",
        })
    except APIError as e:
        return _error(e)


@mcp.tool()
def join_game(session_id: str, player_id: Optional[str] = None) -> str:
    """Join an existing game as the second player.

    Args:
        session_id: The game session ID to join
        player_id: Optional player ID for rejoining a game you were already in
    """
    try:
        api = _get_api()
        data = api.join_game(session_id, player_id)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_game(session_id: str) -> str:
    """Get full game state including board position (FEN), players, clocks, and draw offers.

    Args:
        session_id: The game session ID
    """
    try:
        api = _get_api()
        game = api.get_game(session_id)
        return json.dumps(_game_to_dict(game), indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_moves(session_id: str) -> str:
    """Get all moves for a game in order.

    Args:
        session_id: The game session ID
    """
    try:
        api = _get_api()
        moves = api.get_moves(session_id)
        return json.dumps([_move_to_dict(m) for m in moves], indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def make_move(
    session_id: str,
    player_id: str,
    from_square: str,
    to_square: str,
    promotion: Optional[str] = None,
) -> str:
    """Make a chess move in a game.

    Args:
        session_id: The game session ID
        player_id: Your player ID (received when creating or joining the game)
        from_square: Starting square in algebraic notation (e.g. "e2")
        to_square: Destination square in algebraic notation (e.g. "e4")
        promotion: Piece to promote pawn to: "q", "r", "b", or "n" (only for pawn promotion moves)
    """
    try:
        api = _get_api()
        data = api.make_move(session_id, player_id, from_square, to_square, promotion)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def resign_game(session_id: str, player_id: str) -> str:
    """Resign from a game, forfeiting to the opponent.

    Args:
        session_id: The game session ID
        player_id: Your player ID
    """
    try:
        api = _get_api()
        data = api.resign_game(session_id, player_id)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


# ── Draw Handling ───────────────────────────────────────────────────────────


@mcp.tool()
def offer_draw(session_id: str, player_id: str) -> str:
    """Offer a draw to the opponent. Each player can offer up to 3 draws per game.

    Args:
        session_id: The game session ID
        player_id: Your player ID
    """
    try:
        api = _get_api()
        data = api._post(f'/games/{session_id}/offer-draw', {'playerId': player_id})
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def respond_to_draw(session_id: str, player_id: str, accept: bool) -> str:
    """Accept or decline a draw offer from the opponent.

    Args:
        session_id: The game session ID
        player_id: Your player ID
        accept: True to accept the draw, False to decline
    """
    try:
        api = _get_api()
        data = api._post(f'/games/{session_id}/respond-draw', {
            'playerId': player_id,
            'accept': accept,
        })
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def claim_draw(session_id: str, player_id: str, reason: str) -> str:
    """Claim a draw by threefold repetition or fifty-move rule.

    Args:
        session_id: The game session ID
        player_id: Your player ID
        reason: "threefold_repetition" or "fifty_moves"
    """
    try:
        api = _get_api()
        data = api._post(f'/games/{session_id}/claim-draw', {
            'playerId': player_id,
            'reason': reason,
        })
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


# ── Matchmaking ─────────────────────────────────────────────────────────────


@mcp.tool()
def join_matchmaking(
    connection_id: str,
    display_name: str,
    is_ranked: bool = False,
    opponent_type: str = "either",
    preferred_color: Optional[str] = None,
) -> str:
    """Join the matchmaking queue to find an opponent.

    Args:
        connection_id: A unique ID for this queue entry (use a UUID)
        display_name: Your display name
        is_ranked: Whether to play a ranked game (requires authentication)
        opponent_type: "human", "ai", or "either"
        preferred_color: "white", "black", or omit for no preference
    """
    try:
        api = _get_api()
        data = api.join_matchmaking(
            connection_id=connection_id,
            display_name=display_name,
            is_ranked=is_ranked,
            opponent_type=opponent_type,
            preferred_color=preferred_color,
        )
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def get_matchmaking_status(connection_id: str) -> str:
    """Check the status of a matchmaking queue entry.

    Args:
        connection_id: The connection ID used when joining the queue
    """
    try:
        api = _get_api()
        data = api.get_matchmaking_status(connection_id)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


@mcp.tool()
def leave_matchmaking(connection_id: str) -> str:
    """Leave the matchmaking queue.

    Args:
        connection_id: The connection ID used when joining the queue
    """
    try:
        api = _get_api()
        data = api.leave_matchmaking(connection_id)
        return json.dumps(data, indent=2)
    except APIError as e:
        return _error(e)


def main():
    """Run the Chessmata MCP server (stdio transport)."""
    mcp.run(transport="stdio")


if __name__ == "__main__":
    main()
