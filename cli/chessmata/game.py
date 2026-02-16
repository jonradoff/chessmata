"""Interactive game mode for terminal-based chess."""

import sys
import time
import threading
from typing import Optional, Callable

from .api import ChessmataAPI, APIError, Game, Move
from .board import render_board, format_move_history, parse_move_input, get_current_turn_from_fen, is_promotion_move
from .websocket import GameWebSocket
from .config import Config, Credentials


class InteractiveGame:
    """Interactive terminal-based chess game."""

    def __init__(
        self,
        api: ChessmataAPI,
        session_id: str,
        player_id: str,
        player_color: str,
    ):
        self.api = api
        self.session_id = session_id
        self.player_id = player_id
        self.player_color = player_color
        self.game: Optional[Game] = None
        self.moves: list = []
        self._ws: Optional[GameWebSocket] = None
        self._running = False
        self._needs_redraw = False
        self._last_move: Optional[tuple] = None
        self._message: Optional[str] = None
        self._error: Optional[str] = None
        self._opponent_turn_start: Optional[float] = None
        self._spinner_chars = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏']
        self._spinner_index = 0

    def start(self) -> None:
        """Start the interactive game session."""
        self._running = True

        # Fetch initial game state
        try:
            self.game = self.api.get_game(self.session_id)
            self.moves = self.api.get_moves(self.session_id)
            if self.moves:
                last = self.moves[-1]
                self._last_move = (last.from_square, last.to_square)
        except APIError as e:
            print(f"Error fetching game: {e}")
            return

        # Connect WebSocket for real-time updates
        self._connect_websocket()

        # Main game loop
        self._game_loop()

    def _connect_websocket(self) -> None:
        """Connect to WebSocket for real-time game updates."""
        try:
            self._ws = GameWebSocket(
                server_url=self.api.base_url,
                session_id=self.session_id,
                player_id=self.player_id,
                on_game_update=self._on_game_update,
                on_opponent_move=self._on_opponent_move,
                on_player_joined=self._on_player_joined,
                on_resignation=self._on_resignation,
                on_error=self._on_ws_error,
            )
            self._ws.connect()
        except Exception as e:
            print(f"Warning: WebSocket connection failed: {e}")
            print("Game will continue without real-time updates. Press Enter to refresh.")

    def _on_game_update(self, data: dict) -> None:
        """Handle game update from WebSocket."""
        if 'game' in data:
            self._update_game_from_dict(data['game'])
        self._needs_redraw = True

    def _on_opponent_move(self, data: dict) -> None:
        """Handle opponent move from WebSocket."""
        if 'game' in data:
            self._update_game_from_dict(data['game'])
        if 'move' in data:
            move_data = data['move']
            self._last_move = (move_data.get('from', ''), move_data.get('to', ''))
            self._message = f"Opponent played: {move_data.get('notation', '')}"
        self._needs_redraw = True

    def _on_player_joined(self, data: dict) -> None:
        """Handle player joined event."""
        if 'game' in data:
            self._update_game_from_dict(data['game'])
        self._message = "Opponent has joined the game!"
        self._needs_redraw = True

    def _on_resignation(self, data: dict) -> None:
        """Handle resignation event."""
        if 'game' in data:
            self._update_game_from_dict(data['game'])
        self._message = "Opponent has resigned!"
        self._needs_redraw = True

    def _on_ws_error(self, message: str) -> None:
        """Handle WebSocket error."""
        self._error = f"Connection error: {message}"
        self._needs_redraw = True

    def _update_game_from_dict(self, game_dict: dict) -> None:
        """Update game state from dictionary."""
        # Re-fetch full game to get proper parsing
        try:
            self.game = self.api.get_game(self.session_id)
            self.moves = self.api.get_moves(self.session_id)
        except APIError:
            pass

    def _game_loop(self) -> None:
        """Main game loop."""
        self._draw_screen()

        while self._running:
            # Check if game is over
            if self.game and self.game.status == 'complete':
                self._show_game_result()
                break

            # Check if it's our turn
            is_my_turn = self._is_my_turn()

            if self._needs_redraw:
                self._opponent_turn_start = None  # Reset timer on redraw
                self._draw_screen()
                self._needs_redraw = False

            if is_my_turn:
                self._opponent_turn_start = None  # Reset timer when it's our turn
                self._handle_player_turn()
            else:
                self._wait_for_opponent()

    def _is_my_turn(self) -> bool:
        """Check if it's the current player's turn."""
        if not self.game:
            return False
        return self.game.current_turn == self.player_color

    def _draw_screen(self) -> None:
        """Draw the game screen."""
        # Clear screen
        print("\033[2J\033[H", end="")

        if not self.game:
            print("Loading game...")
            return

        # Header
        print("=" * 50)
        print(f"  CHESSMATA - Game: {self.session_id[:8]}...")
        print(f"  You are playing as: {self.player_color.upper()}")
        print("=" * 50)
        print()

        # Show time control
        if self.game.time_control and self.game.time_control.mode != 'unlimited':
            tc = self.game.time_control
            print(f"  Time Control: {tc.mode.capitalize()}")
            print()

        # Show players with clocks
        for player in self.game.players:
            marker = " (you)" if player.id == self.player_id else ""
            name = player.display_name or player.agent_name or "Anonymous"
            elo = f" [{player.elo_rating}]" if player.elo_rating else ""

            # Add clock display
            clock_str = ""
            if self.game.player_times and self.game.time_control and self.game.time_control.mode != 'unlimited':
                if player.color == 'white':
                    time_ms = self.game.player_times.white_remaining_ms
                else:
                    time_ms = self.game.player_times.black_remaining_ms
                clock_str = f"  [{self._format_clock(time_ms)}]"
                # Add active indicator
                if self.game.status == 'active' and self.game.current_turn == player.color:
                    clock_str += " <-"

            print(f"  {player.color.capitalize()}: {name}{elo}{marker}{clock_str}")
        print()

        # Show game status
        if self.game.status == 'waiting':
            print("  Status: Waiting for opponent to join...")
            print(f"  Share this link: {self.api.base_url}/game/{self.session_id}")
            print()
        elif self.game.status == 'complete':
            if self.game.winner:
                winner_text = "You won!" if self.game.winner == self.player_color else "You lost."
                print(f"  Game Over - {winner_text}")
                if self.game.win_reason:
                    print(f"  Reason: {self.game.win_reason}")
            else:
                print("  Game Over - Draw")
            print()

        # Render board
        board_str = render_board(
            self.game.board_state,
            player_color=self.player_color,
            use_unicode=True,
            use_color=True,
            last_move=self._last_move,
            in_check=self._is_in_check(),
        )
        print(board_str)
        print()

        # Show turn indicator
        if self.game.status == 'active':
            if self._is_my_turn():
                print("  >>> YOUR TURN <<<")
            else:
                print("  Waiting for opponent...")
        print()

        # Show message or error
        if self._message:
            print(f"  {self._message}")
            self._message = None
        if self._error:
            print(f"  ERROR: {self._error}")
            self._error = None

        # Show move history (last 10 moves)
        if self.moves:
            print()
            print("  Recent moves:")
            history = format_move_history(self.moves[-20:])
            for line in history.split('\n')[-10:]:
                print(f"    {line}")

        print()
        print("-" * 50)

    def _is_in_check(self) -> bool:
        """Check if current player is in check."""
        if not self.moves:
            return False
        last_move = self.moves[-1]
        return last_move.check and not last_move.checkmate

    def _handle_player_turn(self) -> None:
        """Handle player's turn - get and execute move."""
        print("  Enter move (e.g., e2e4) or command:")
        print("  Commands: 'resign', 'refresh', 'quit', 'help'")
        print()

        try:
            user_input = input("  Your move: ").strip().lower()
        except (EOFError, KeyboardInterrupt):
            self._running = False
            return

        if not user_input:
            return

        # Handle commands
        if user_input == 'quit' or user_input == 'exit':
            self._running = False
            return
        elif user_input == 'resign':
            self._handle_resign()
            return
        elif user_input == 'refresh':
            self._refresh_game()
            return
        elif user_input == 'help':
            self._show_help()
            return

        # Parse and execute move
        parsed = parse_move_input(user_input)
        if not parsed:
            self._error = f"Invalid move format: {user_input}"
            self._needs_redraw = True
            return

        from_sq, to_sq, promotion = parsed

        # Auto-detect pawn promotion and prompt if no piece specified
        if not promotion and self.game and is_promotion_move(self.game.board_state, from_sq, to_sq):
            promotion = self._prompt_promotion()
            if not promotion:
                return  # Cancelled

        self._make_move(from_sq, to_sq, promotion)

    def _prompt_promotion(self) -> Optional[str]:
        """Prompt the user to choose a promotion piece.

        Returns:
            Single character for the promotion piece, or None if cancelled.
        """
        print()
        print("  Pawn promotion! Choose a piece:")
        print("    q - Queen")
        print("    r - Rook")
        print("    b - Bishop")
        print("    n - Knight")
        print()
        try:
            choice = input("  Promote to (q/r/b/n): ").strip().lower()
        except (EOFError, KeyboardInterrupt):
            return None

        if choice in ('q', 'r', 'b', 'n'):
            return choice

        if choice in ('queen', 'rook', 'bishop', 'knight'):
            return choice[0]

        # Default to queen
        print("  Defaulting to Queen.")
        return 'q'

    def _make_move(self, from_sq: str, to_sq: str, promotion: Optional[str] = None) -> None:
        """Make a move."""
        try:
            result = self.api.make_move(
                self.session_id,
                self.player_id,
                from_sq,
                to_sq,
                promotion,
            )

            if result.get('success'):
                self._last_move = (from_sq, to_sq)
                self._message = f"Move played: {from_sq}-{to_sq}"
                # Refresh game state
                self._refresh_game()
            else:
                self._error = result.get('error', 'Invalid move')

        except APIError as e:
            self._error = str(e)

        self._needs_redraw = True

    def _handle_resign(self) -> None:
        """Handle resignation."""
        print()
        confirm = input("  Are you sure you want to resign? (yes/no): ").strip().lower()
        if confirm in ('yes', 'y'):
            try:
                self.api.resign_game(self.session_id, self.player_id)
                self._message = "You have resigned."
                self._refresh_game()
            except APIError as e:
                self._error = f"Failed to resign: {e}"
        self._needs_redraw = True

    def _refresh_game(self) -> None:
        """Refresh game state from server."""
        try:
            self.game = self.api.get_game(self.session_id)
            self.moves = self.api.get_moves(self.session_id)
            if self.moves:
                last = self.moves[-1]
                self._last_move = (last.from_square, last.to_square)
        except APIError as e:
            self._error = f"Failed to refresh: {e}"
        self._needs_redraw = True

    def _format_wait_time(self, seconds: float) -> str:
        """Format wait time as mm:ss."""
        mins = int(seconds) // 60
        secs = int(seconds) % 60
        return f"{mins}:{secs:02d}"

    def _format_clock(self, ms: int) -> str:
        """Format milliseconds as a clock display.

        - Under 10 seconds: shows tenths (0:05.2)
        - Under 1 minute: shows seconds (0:45)
        - Under 1 hour: shows minutes:seconds (14:32)
        - Over 1 hour: shows hours:minutes:seconds (1:30:00)
        """
        if ms <= 0:
            return "0:00"

        total_seconds = ms // 1000
        hours = total_seconds // 3600
        minutes = (total_seconds % 3600) // 60
        seconds = total_seconds % 60

        # Under 10 seconds - show tenths
        if total_seconds < 10:
            tenths = (ms % 1000) // 100
            return f"0:0{seconds}.{tenths}"

        # Under 1 minute - just seconds
        if total_seconds < 60:
            return f"0:{seconds:02d}"

        # Under 1 hour - minutes:seconds
        if hours == 0:
            return f"{minutes}:{seconds:02d}"

        # Over 1 hour - hours:minutes:seconds
        return f"{hours}:{minutes:02d}:{seconds:02d}"

    def _wait_for_opponent(self) -> None:
        """Wait for opponent's turn with spinner animation."""
        # Initialize opponent turn timer if not set
        if self._opponent_turn_start is None:
            self._opponent_turn_start = time.time()

        try:
            import select

            # Show spinner with wait time and clock
            elapsed = time.time() - self._opponent_turn_start
            spinner = self._spinner_chars[self._spinner_index]
            self._spinner_index = (self._spinner_index + 1) % len(self._spinner_chars)
            wait_str = self._format_wait_time(elapsed)

            # Add opponent clock if time control is active
            clock_str = ""
            if self.game and self.game.player_times and self.game.time_control and self.game.time_control.mode != 'unlimited':
                opp_color = 'black' if self.player_color == 'white' else 'white'
                if opp_color == 'white':
                    opp_time = self.game.player_times.white_remaining_ms
                else:
                    opp_time = self.game.player_times.black_remaining_ms
                clock_str = f" [{self._format_clock(opp_time)}]"

            # Use carriage return to update in place
            sys.stdout.write(f"\r  {spinner} Waiting for opponent...{clock_str} ({wait_str})  ")
            sys.stdout.flush()

            if sys.platform != 'win32':
                # Unix - use select for non-blocking input
                rlist, _, _ = select.select([sys.stdin], [], [], 0.2)
                if rlist:
                    user_input = sys.stdin.readline().strip().lower()
                    print()  # New line after input
                    if user_input == 'quit' or user_input == 'exit':
                        self._running = False
                    elif user_input == 'resign':
                        self._handle_resign()
                    elif user_input:
                        self._refresh_game()
            else:
                # Windows - simple input with manual refresh
                import msvcrt
                time.sleep(0.2)
                if msvcrt.kbhit():
                    user_input = input().strip().lower()
                    if user_input == 'quit' or user_input == 'exit':
                        self._running = False
                    elif user_input == 'resign':
                        self._handle_resign()
                    elif user_input:
                        self._refresh_game()

            # Check for WebSocket updates
            if self._needs_redraw:
                print()  # Clear the spinner line
                self._opponent_turn_start = None  # Reset timer
                self._draw_screen()
                self._needs_redraw = False

        except (EOFError, KeyboardInterrupt):
            print()
            self._running = False

    def _show_game_result(self) -> None:
        """Show game result."""
        self._draw_screen()

        print()
        if self.game and self.game.winner:
            if self.game.winner == self.player_color:
                print("  *** VICTORY! ***")
            else:
                print("  *** DEFEAT ***")

            if self.game.elo_changes:
                ec = self.game.elo_changes
                if self.player_color == 'white':
                    change = ec.white_change
                    new_elo = ec.white_new_elo
                else:
                    change = ec.black_change
                    new_elo = ec.black_new_elo

                sign = "+" if change >= 0 else ""
                print(f"  Elo change: {sign}{change} (new rating: {new_elo})")
        else:
            print("  *** DRAW ***")

        print()
        input("  Press Enter to exit...")

    def _show_help(self) -> None:
        """Show help information."""
        print()
        print("  === HELP ===")
        print()
        print("  Move format: e2e4 or e2-e4 or e2 e4")
        print()
        print("  Special moves:")
        print("    Castling:    e1g1 (kingside) or e1c1 (queenside)")
        print("    En passant:  Use normal capture notation (e.g., e5d6)")
        print("    Promotion:   e7e8q or e7e8=q (q/r/b/n)")
        print("                 You will be prompted if you omit the piece")
        print()
        print("  Commands:")
        print("    resign  - Resign the game")
        print("    refresh - Refresh game state")
        print("    quit    - Exit the game")
        print("    help    - Show this help")
        print()
        input("  Press Enter to continue...")
        self._needs_redraw = True

    def stop(self) -> None:
        """Stop the game and cleanup."""
        self._running = False
        if self._ws:
            self._ws.disconnect()


def play_game(
    config: Config,
    credentials: Credentials,
    session_id: str,
    player_id: Optional[str] = None,
) -> None:
    """Start an interactive game session.

    Args:
        config: Application configuration
        credentials: User credentials
        session_id: Game session ID to join
        player_id: Optional player ID if rejoining
    """
    api = ChessmataAPI(config, credentials)

    # Join the game
    print(f"Joining game {session_id}...")

    try:
        result = api.join_game(session_id, player_id)
        player_id = result['playerId']
        player_color = result['color']
        print(f"Joined as {player_color}")
    except APIError as e:
        print(f"Error joining game: {e}")
        return

    # Start interactive game
    game = InteractiveGame(api, session_id, player_id, player_color)

    try:
        game.start()
    except KeyboardInterrupt:
        print("\nGame interrupted.")
    finally:
        game.stop()


def create_and_play_game(config: Config, credentials: Credentials) -> None:
    """Create a new game and wait for opponent.

    Args:
        config: Application configuration
        credentials: User credentials
    """
    api = ChessmataAPI(config, credentials)

    print("Creating new game...")

    try:
        session_id, player_id = api.create_game()
        print(f"Game created! Session ID: {session_id}")
        print(f"Share this link: {config.server_url}/game/{session_id}")
        print()
    except APIError as e:
        print(f"Error creating game: {e}")
        return

    # Start interactive game (will wait for opponent)
    game = InteractiveGame(api, session_id, player_id, 'white')

    try:
        game.start()
    except KeyboardInterrupt:
        print("\nGame interrupted.")
    finally:
        game.stop()
