"""API client for Chessmata server."""

import json
import urllib.request
import urllib.error
import urllib.parse
from typing import Optional, Dict, Any, Tuple
from dataclasses import dataclass

from .config import Config, Credentials


class APIError(Exception):
    """API error with status code and message."""

    def __init__(self, message: str, status_code: int = 0):
        super().__init__(message)
        self.status_code = status_code


@dataclass
class Player:
    """Player information."""
    id: str
    color: str
    user_id: Optional[str] = None
    display_name: Optional[str] = None
    agent_name: Optional[str] = None
    elo_rating: Optional[int] = None
    joined_at: Optional[str] = None


@dataclass
class EloChanges:
    """Elo rating changes after a game."""
    white_change: int
    black_change: int
    white_new_elo: int
    black_new_elo: int


@dataclass
class TimeControl:
    """Time control settings."""
    mode: str
    base_time_ms: int
    increment_ms: int


@dataclass
class PlayerTimes:
    """Player time state."""
    white_remaining_ms: int
    black_remaining_ms: int
    white_last_move_at: Optional[int] = None
    black_last_move_at: Optional[int] = None


@dataclass
class Game:
    """Game state."""
    id: str
    session_id: str
    players: list
    status: str  # 'waiting', 'active', 'complete'
    current_turn: str  # 'white' or 'black'
    board_state: str  # FEN notation
    winner: Optional[str] = None
    win_reason: Optional[str] = None
    is_ranked: bool = False
    elo_changes: Optional[EloChanges] = None
    time_control: Optional[TimeControl] = None
    player_times: Optional[PlayerTimes] = None
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


@dataclass
class Move:
    """A chess move."""
    id: str
    game_id: str
    session_id: str
    player_id: str
    move_number: int
    from_square: str
    to_square: str
    piece: str
    notation: str
    capture: bool = False
    check: bool = False
    checkmate: bool = False
    promotion: Optional[str] = None
    created_at: Optional[str] = None


class ChessmataAPI:
    """Client for the Chessmata API."""

    AGENT_NAME = "ChessmataCLI"
    AGENT_VERSION = "1.0.0"

    def __init__(self, config: Config, credentials: Optional[Credentials] = None):
        self.config = config
        self.credentials = credentials or Credentials.load()
        self.base_url = config.server_url.rstrip('/')

    def _get_headers(self, include_auth: bool = True) -> Dict[str, str]:
        """Get headers for API requests."""
        headers = {
            'Content-Type': 'application/json',
            'User-Agent': f'{self.AGENT_NAME}/{self.AGENT_VERSION}',
            'X-Agent-Name': self.AGENT_NAME,
            'X-Agent-Version': self.AGENT_VERSION,
        }

        if include_auth and self.credentials.is_logged_in():
            headers['Authorization'] = f'Bearer {self.credentials.access_token}'

        return headers

    def _request(
        self,
        method: str,
        endpoint: str,
        data: Optional[Dict[str, Any]] = None,
        include_auth: bool = True,
    ) -> Dict[str, Any]:
        """Make an HTTP request to the API."""
        url = f"{self.base_url}/api{endpoint}"
        headers = self._get_headers(include_auth)

        body = None
        if data is not None:
            body = json.dumps(data).encode('utf-8')

        req = urllib.request.Request(url, data=body, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=30) as response:
                response_data = response.read().decode('utf-8')
                if response_data:
                    return json.loads(response_data)
                return {}
        except urllib.error.HTTPError as e:
            error_body = e.read().decode('utf-8')
            try:
                error_data = json.loads(error_body)
                message = error_data.get('error') or error_data.get('message') or str(e)
            except json.JSONDecodeError:
                message = error_body or str(e)
            raise APIError(message, e.code)
        except urllib.error.URLError as e:
            raise APIError(f"Connection error: {e.reason}")

    def _get(self, endpoint: str, include_auth: bool = True) -> Dict[str, Any]:
        """Make a GET request."""
        return self._request('GET', endpoint, include_auth=include_auth)

    def _post(
        self,
        endpoint: str,
        data: Optional[Dict[str, Any]] = None,
        include_auth: bool = True,
    ) -> Dict[str, Any]:
        """Make a POST request."""
        return self._request('POST', endpoint, data=data, include_auth=include_auth)

    # Authentication endpoints

    def login(self, email: str, password: str) -> Credentials:
        """Login with email and password."""
        data = self._post('/auth/login', {'email': email, 'password': password}, include_auth=False)

        user = data.get('user', {})
        creds = Credentials(
            access_token=data.get('accessToken'),
            user_id=user.get('id'),
            email=user.get('email'),
            display_name=user.get('displayName'),
            elo_rating=user.get('eloRating'),
        )
        creds.save()
        self.credentials = creds
        return creds

    def register(self, email: str, password: str, display_name: str) -> Credentials:
        """Register a new account."""
        data = self._post('/auth/register', {
            'email': email,
            'password': password,
            'displayName': display_name,
        }, include_auth=False)

        user = data.get('user', {})
        creds = Credentials(
            access_token=data.get('accessToken'),
            user_id=user.get('id'),
            email=user.get('email'),
            display_name=user.get('displayName'),
            elo_rating=user.get('eloRating'),
        )
        creds.save()
        self.credentials = creds
        return creds

    def get_current_user(self) -> Dict[str, Any]:
        """Get current user info."""
        return self._get('/auth/me')

    def logout(self) -> None:
        """Logout and clear credentials."""
        try:
            self._post('/auth/logout')
        except APIError:
            pass  # Ignore errors during logout
        finally:
            self.credentials.clear()

    def validate_token(self) -> bool:
        """Validate the current access token."""
        if not self.credentials.is_logged_in():
            return False
        try:
            self.get_current_user()
            return True
        except APIError:
            return False

    # Game endpoints

    def create_game(self) -> Tuple[str, str]:
        """Create a new game. Returns (session_id, player_id)."""
        data = self._post('/games', {
            'clientSoftware': f'{self.AGENT_NAME}/{self.AGENT_VERSION}',
        })
        return data['sessionId'], data['playerId']

    def join_game(self, session_id: str, player_id: Optional[str] = None) -> Dict[str, Any]:
        """Join an existing game."""
        payload = {
            'clientSoftware': f'{self.AGENT_NAME}/{self.AGENT_VERSION}',
        }
        if player_id:
            payload['playerId'] = player_id

        return self._post(f'/games/{session_id}/join', payload)

    def get_game(self, session_id: str) -> Game:
        """Get game state."""
        data = self._get(f'/games/{session_id}')
        return self._parse_game(data)

    def get_moves(self, session_id: str) -> list:
        """Get all moves for a game."""
        data = self._get(f'/games/{session_id}/moves')
        if not data:
            return []
        moves = []
        for m in data.get('moves', []) or []:
            moves.append(Move(
                id=m['id'],
                game_id=m['gameId'],
                session_id=m['sessionId'],
                player_id=m['playerId'],
                move_number=m['moveNumber'],
                from_square=m.get('from', ''),
                to_square=m.get('to', ''),
                piece=m.get('piece', ''),
                notation=m.get('notation', ''),
                capture=m.get('capture', False),
                check=m.get('check', False),
                checkmate=m.get('checkmate', False),
                promotion=m.get('promotion'),
                created_at=m.get('createdAt'),
            ))
        return moves

    def make_move(
        self,
        session_id: str,
        player_id: str,
        from_square: str,
        to_square: str,
        promotion: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Make a move."""
        payload = {
            'playerId': player_id,
            'from': from_square,
            'to': to_square,
        }
        if promotion:
            payload['promotion'] = promotion

        return self._post(f'/games/{session_id}/move', payload)

    def resign_game(self, session_id: str, player_id: str) -> Dict[str, Any]:
        """Resign from a game."""
        return self._post(f'/games/{session_id}/resign', {'playerId': player_id})

    # Matchmaking endpoints

    def join_matchmaking(
        self,
        connection_id: str,
        display_name: str,
        is_ranked: bool = False,
        opponent_type: str = 'either',
        time_controls: Optional[list] = None,
        preferred_color: Optional[str] = None,
        engine_name: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Join the matchmaking queue."""
        payload = {
            'connectionId': connection_id,
            'displayName': display_name,
            'isRanked': is_ranked,
            'opponentType': opponent_type,
            'clientSoftware': f'{self.AGENT_NAME}/{self.AGENT_VERSION}',
        }
        if time_controls:
            payload['timeControls'] = time_controls
        if preferred_color:
            payload['preferredColor'] = preferred_color
        if engine_name:
            payload['engineName'] = engine_name
        return self._post('/matchmaking/join', payload)

    def leave_matchmaking(self, connection_id: str) -> Dict[str, Any]:
        """Leave the matchmaking queue."""
        return self._post(f'/matchmaking/leave?connectionId={urllib.parse.quote(connection_id)}')

    def get_matchmaking_status(self, connection_id: str) -> Dict[str, Any]:
        """Get matchmaking queue status."""
        return self._get(f'/matchmaking/status?connectionId={urllib.parse.quote(connection_id)}')

    def get_lobby(self) -> list:
        """Get matchmaking lobby (players/agents waiting for a match)."""
        data = self._get('/matchmaking/lobby', include_auth=False)
        if isinstance(data, list):
            return data
        return []

    # Leaderboard endpoint

    def get_leaderboard(self, leaderboard_type: str = 'players') -> list:
        """Get leaderboard entries.

        Args:
            leaderboard_type: 'players' or 'agents'

        Returns:
            List of leaderboard entries (dicts).
        """
        data = self._get(f'/leaderboard?type={urllib.parse.quote(leaderboard_type)}', include_auth=False)
        if isinstance(data, list):
            return data
        return []

    # User lookup & game listing endpoints

    def lookup_user(self, display_name: str) -> Dict[str, Any]:
        """Look up a user by display name. Returns userId, displayName, eloRating."""
        return self._get(
            f'/users/lookup?displayName={urllib.parse.quote(display_name)}',
            include_auth=False,
        )

    def get_user_game_history(
        self,
        user_id: str,
        page: int = 1,
        limit: int = 20,
        result: Optional[str] = None,
        ranked: Optional[str] = None,
    ) -> Dict[str, Any]:
        """Get paginated game history for a user.

        Args:
            user_id: User's MongoDB ObjectID.
            page: Page number (default 1).
            limit: Results per page (default 20, max 50).
            result: Filter by "wins", "losses", or "draws".
            ranked: Filter by "true" or "false".

        Returns:
            Dict with 'games', 'total', 'page', 'limit'.
        """
        params = f'page={page}&limit={limit}'
        if result:
            params += f'&result={urllib.parse.quote(result)}'
        if ranked:
            params += f'&ranked={urllib.parse.quote(ranked)}'
        return self._get(f'/users/{urllib.parse.quote(user_id)}/games?{params}')

    def list_active_games(
        self,
        limit: int = 20,
        inactive_mins: int = 10,
        ranked: Optional[str] = None,
    ) -> list:
        """List active games.

        Args:
            limit: Max results (default 20, max 50).
            inactive_mins: Exclude games not updated within N minutes.
            ranked: "true", "false", or None for all.

        Returns:
            List of game dicts.
        """
        params = f'limit={limit}&inactiveMins={inactive_mins}'
        if ranked:
            params += f'&ranked={urllib.parse.quote(ranked)}'
        data = self._get(f'/games/active?{params}', include_auth=False)
        if isinstance(data, list):
            return data
        return []

    def list_completed_games(
        self,
        limit: int = 20,
        ranked: Optional[str] = None,
    ) -> list:
        """List recently completed games.

        Args:
            limit: Max results (default 20, max 50).
            ranked: "true", "false", or None for all.

        Returns:
            List of game dicts.
        """
        params = f'limit={limit}'
        if ranked:
            params += f'&ranked={urllib.parse.quote(ranked)}'
        data = self._get(f'/games/completed?{params}', include_auth=False)
        if isinstance(data, list):
            return data
        return []

    def _parse_game(self, data: Dict[str, Any]) -> Game:
        """Parse game data into a Game object."""
        players = []
        for p in data.get('players', []):
            players.append(Player(
                id=p['id'],
                color=p['color'],
                user_id=p.get('userId'),
                display_name=p.get('displayName'),
                agent_name=p.get('agentName'),
                elo_rating=p.get('eloRating'),
                joined_at=p.get('joinedAt'),
            ))

        elo_changes = None
        if data.get('eloChanges'):
            ec = data['eloChanges']
            elo_changes = EloChanges(
                white_change=ec.get('whiteChange', 0),
                black_change=ec.get('blackChange', 0),
                white_new_elo=ec.get('whiteNewElo', 0),
                black_new_elo=ec.get('blackNewElo', 0),
            )

        time_control = None
        if data.get('timeControl'):
            tc = data['timeControl']
            time_control = TimeControl(
                mode=tc.get('mode', 'unlimited'),
                base_time_ms=tc.get('baseTimeMs', 0),
                increment_ms=tc.get('incrementMs', 0),
            )

        player_times = None
        if data.get('playerTimes'):
            pt = data['playerTimes']
            white_pt = pt.get('white', {})
            black_pt = pt.get('black', {})
            player_times = PlayerTimes(
                white_remaining_ms=white_pt.get('remainingMs', 0),
                black_remaining_ms=black_pt.get('remainingMs', 0),
                white_last_move_at=white_pt.get('lastMoveAt'),
                black_last_move_at=black_pt.get('lastMoveAt'),
            )

        return Game(
            id=data['id'],
            session_id=data['sessionId'],
            players=players,
            status=data['status'],
            current_turn=data['currentTurn'],
            board_state=data.get('boardState', ''),
            winner=data.get('winner'),
            win_reason=data.get('winReason'),
            is_ranked=data.get('isRanked', False),
            elo_changes=elo_changes,
            time_control=time_control,
            player_times=player_times,
            created_at=data.get('createdAt'),
            updated_at=data.get('updatedAt'),
        )
