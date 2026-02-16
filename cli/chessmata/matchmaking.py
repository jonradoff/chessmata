"""Matchmaking mode for finding opponents."""

import sys
import time
import uuid
from typing import Optional, List

from .api import ChessmataAPI, APIError
from .config import Config, Credentials
from .game import InteractiveGame


# Time control modes with display names
TIME_CONTROLS = {
    '1': ('unlimited', 'Unlimited'),
    '2': ('casual', 'Casual (30 min)'),
    '3': ('standard', 'Standard (15+10)'),
    '4': ('quick', 'Quick (5+3)'),
    '5': ('blitz', 'Blitz (3+2)'),
    '6': ('tournament', 'Tournament (90+30)'),
}

DEFAULT_TIME_CONTROLS = ['casual']


def select_time_controls() -> List[str]:
    """Prompt user to select time controls.

    Returns:
        List of time control mode names
    """
    print("  Select time controls (comma-separated, e.g., 1,3,4):")
    print()
    for key, (_, name) in TIME_CONTROLS.items():
        print(f"    {key}. {name}")
    print()
    print("  Leave blank for default (Casual)")
    print()

    try:
        selection = input("  Your choice: ").strip()
    except (EOFError, KeyboardInterrupt):
        return DEFAULT_TIME_CONTROLS

    if not selection:
        return DEFAULT_TIME_CONTROLS

    selected = []
    for char in selection.replace(' ', '').split(','):
        if char in TIME_CONTROLS:
            mode, _ = TIME_CONTROLS[char]
            if mode not in selected:
                selected.append(mode)

    if not selected:
        print("  Invalid selection, using defaults.")
        return DEFAULT_TIME_CONTROLS

    return selected


class MatchmakingSession:
    """Handle matchmaking queue and game start."""

    def __init__(
        self,
        api: ChessmataAPI,
        display_name: str,
        is_ranked: bool = False,
        opponent_type: str = 'either',
        time_controls: Optional[List[str]] = None,
        engine_name: Optional[str] = None,
    ):
        self.api = api
        self.display_name = display_name
        self.is_ranked = is_ranked
        self.opponent_type = opponent_type
        self.time_controls = time_controls or DEFAULT_TIME_CONTROLS
        self.engine_name = engine_name
        self.connection_id: Optional[str] = None
        self._cancelled = False

    def find_opponent(self) -> Optional[dict]:
        """Join matchmaking queue and wait for opponent.

        Returns:
            Match result with game info, or None if cancelled
        """
        print()
        print("=" * 50)
        print("  MATCHMAKING")
        print("=" * 50)
        print()
        print(f"  Display Name: {self.display_name}")
        print(f"  Ranked: {'Yes' if self.is_ranked else 'No'}")
        print(f"  Opponent Type: {self.opponent_type}")
        time_names = [TIME_CONTROLS.get(k, (tc, tc))[1] if k in TIME_CONTROLS else tc
                      for k, tc in [(k, v) for k, (v, _) in TIME_CONTROLS.items()] if tc in self.time_controls]
        if not time_names:
            time_names = self.time_controls
        print(f"  Time Controls: {', '.join(self.time_controls)}")
        print()

        # Generate connection ID
        self.connection_id = str(uuid.uuid4())

        # Join queue
        try:
            result = self.api.join_matchmaking(
                connection_id=self.connection_id,
                display_name=self.display_name,
                is_ranked=self.is_ranked,
                opponent_type=self.opponent_type,
                time_controls=self.time_controls,
                engine_name=self.engine_name,
            )

            if result.get('match'):
                # Immediately matched
                print("  Match found immediately!")
                return result['match']

            print(f"  Joined matchmaking queue (ID: {self.connection_id[:8]}...)")
            print()
            print("  Searching for opponent...")
            print("  Press Ctrl+C to cancel")
            print()

        except APIError as e:
            print(f"  Error joining matchmaking: {e}")
            return None

        # Poll for match
        return self._wait_for_match()

    def _wait_for_match(self) -> Optional[dict]:
        """Poll matchmaking status until match found or cancelled."""
        dots = 0
        start_time = time.time()

        while not self._cancelled:
            try:
                status = self.api.get_matchmaking_status(self.connection_id)

                if status.get('status') == 'matched':
                    print()
                    print("  Match found!")
                    return {
                        'sessionId': status.get('matchedSessionId'),
                        'connectionId': self.connection_id,
                    }

                if status.get('status') == 'expired':
                    print()
                    print("  Queue expired. Please try again.")
                    return None

                # Show waiting indicator
                elapsed = int(time.time() - start_time)
                queue_pos = status.get('queuePosition', '?')
                dots = (dots + 1) % 4
                sys.stdout.write(f"\r  Waiting{'.' * dots}{' ' * (3 - dots)} (Position: {queue_pos}, Time: {elapsed}s)")
                sys.stdout.flush()

                time.sleep(2)

            except APIError as e:
                print(f"\n  Error checking status: {e}")
                time.sleep(5)

            except KeyboardInterrupt:
                self._cancelled = True
                break

        # Cancelled
        if self.connection_id:
            print()
            print("  Cancelling...")
            try:
                self.api.leave_matchmaking(self.connection_id)
                print("  Left matchmaking queue.")
            except APIError:
                pass

        return None

    def cancel(self) -> None:
        """Cancel matchmaking."""
        self._cancelled = True


def start_matchmaking(
    config: Config,
    credentials: Credentials,
    is_ranked: bool = False,
    opponent_type: str = 'either',
    time_controls: Optional[List[str]] = None,
    engine_name: Optional[str] = None,
) -> None:
    """Start matchmaking to find an opponent.

    Args:
        config: Application configuration
        credentials: User credentials
        is_ranked: Whether to play a ranked game
        opponent_type: 'human', 'ai', or 'either'
        time_controls: List of time control modes
        engine_name: Engine name to prevent same-engine matching
    """
    api = ChessmataAPI(config, credentials)

    # Get display name - must be logged in to have one
    if credentials.display_name:
        display_name = credentials.display_name
    else:
        print("You must be logged in to use matchmaking.")
        print("Use 'chessmata login' to authenticate.")
        return

    # Check auth for ranked
    if is_ranked and not credentials.is_logged_in():
        print("You must be logged in to play ranked games.")
        print("Use 'chessmata login' to authenticate.")
        return

    # Prompt for time controls if not provided
    if time_controls is None:
        time_controls = select_time_controls()

    # Start matchmaking
    session = MatchmakingSession(
        api=api,
        display_name=display_name,
        is_ranked=is_ranked,
        opponent_type=opponent_type,
        time_controls=time_controls,
        engine_name=engine_name,
    )

    try:
        match = session.find_opponent()
    except KeyboardInterrupt:
        session.cancel()
        print("\nMatchmaking cancelled.")
        return

    if not match:
        return

    # Got a match - the game is already created with both players
    session_id = match.get('sessionId')
    connection_id = match.get('connectionId')

    if not session_id:
        print("Error: Invalid match data received.")
        return

    print(f"  Game ID: {session_id}")

    # Fetch the game to get our color (we're already in the game)
    try:
        game = api.get_game(session_id)
        # Find our player entry using connection_id as player_id
        player_color = None
        for player in game.players:
            if player.id == connection_id:
                player_color = player.color
                break

        if not player_color:
            print("Error: Could not find our player in the game.")
            return

        player_id = connection_id
    except APIError as e:
        print(f"  Error fetching game: {e}")
        return

    print(f"  You are playing as: {player_color}")
    print()
    input("  Press Enter to start the game...")

    # Start interactive game
    game_session = InteractiveGame(api, session_id, player_id, player_color)

    try:
        game_session.start()
    except KeyboardInterrupt:
        print("\nGame interrupted.")
    finally:
        game_session.stop()
