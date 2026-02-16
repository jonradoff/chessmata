"""UCI (Universal Chess Interface) mode for Chessmata CLI.

Implements the UCI protocol to allow Chessmata's server-side AI to be used
as a UCI engine in any UCI-compatible chess GUI (Arena, CuteChess, etc.).

The engine proxies moves between the GUI and the Chessmata server:
- Human moves from the GUI are submitted to the server as the logged-in user
- Server AI moves are returned as bestmove responses
"""

import sys
import time
import threading
import uuid
from typing import Optional, List

from .api import ChessmataAPI, APIError
from .config import Config, Credentials


class UCIEngine:
    """UCI protocol engine that proxies Chessmata server AI."""

    ENGINE_NAME = "Chessmata Proxy"
    ENGINE_AUTHOR = "Metavert"

    def __init__(self, api: ChessmataAPI, credentials: Credentials):
        self.api = api
        self.credentials = credentials

        # UCI options
        self.ranked = False
        self.opponent_type = 'ai'
        self.time_control = 'unlimited'
        self.debug = False

        # Game state
        self.session_id: Optional[str] = None
        self.player_id: Optional[str] = None
        self.engine_color: Optional[str] = None  # Color the "engine" (server AI) plays
        self.human_color: Optional[str] = None

        # Position tracking
        self.start_fen: Optional[str] = None  # None means startpos
        self.moves: List[str] = []  # Moves from position command
        self.synced_move_count = 0  # How many moves have been accounted for on server

        # Threading
        self._search_thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()
        self._running = True

    def run(self):
        """Main UCI loop - read commands from stdin."""
        while self._running:
            try:
                line = input()
            except EOFError:
                break

            line = line.strip()
            if not line:
                continue

            self._handle_command(line)

    def _handle_command(self, line: str):
        """Dispatch a UCI command."""
        parts = line.split()
        cmd = parts[0]

        if cmd == 'uci':
            self._cmd_uci()
        elif cmd == 'debug':
            self._cmd_debug(parts)
        elif cmd == 'isready':
            self._cmd_isready()
        elif cmd == 'setoption':
            self._cmd_setoption(parts)
        elif cmd == 'ucinewgame':
            self._cmd_ucinewgame()
        elif cmd == 'position':
            self._cmd_position(parts)
        elif cmd == 'go':
            self._cmd_go(parts)
        elif cmd == 'stop':
            self._cmd_stop()
        elif cmd == 'quit':
            self._cmd_quit()
        # Silently ignore unrecognized commands per UCI spec

    def _send(self, line: str):
        """Send a line to the GUI via stdout."""
        sys.stdout.write(line + '\n')
        sys.stdout.flush()

    def _debug_log(self, msg: str):
        """Send debug info string if debug mode is on."""
        if self.debug:
            self._send(f"info string {msg}")

    # --- UCI command handlers ---

    def _cmd_uci(self):
        self._send(f"id name {self.ENGINE_NAME}")
        self._send(f"id author {self.ENGINE_AUTHOR}")
        self._send("option name Ranked type check default false")
        self._send("option name OpponentType type combo default ai var ai var human var either")
        self._send("option name TimeControl type combo default unlimited var unlimited var casual var standard var quick var blitz var tournament")
        self._send("uciok")

    def _cmd_debug(self, parts: List[str]):
        if len(parts) > 1:
            self.debug = parts[1] == 'on'

    def _cmd_isready(self):
        self._send("readyok")

    def _cmd_setoption(self, parts: List[str]):
        # Parse: setoption name <name> [value <value>]
        try:
            name_idx = parts.index('name') + 1
        except ValueError:
            return

        try:
            value_idx = parts.index('value') + 1
            name = ' '.join(parts[name_idx:value_idx - 1])
            value = ' '.join(parts[value_idx:])
        except ValueError:
            name = ' '.join(parts[name_idx:])
            value = None

        name_lower = name.lower()

        if name_lower == 'ranked':
            self.ranked = value is not None and value.lower() == 'true'
        elif name_lower == 'opponenttype':
            if value and value.lower() in ('ai', 'human', 'either'):
                self.opponent_type = value.lower()
        elif name_lower == 'timecontrol':
            if value and value.lower() in ('unlimited', 'casual', 'standard', 'quick', 'blitz', 'tournament'):
                self.time_control = value.lower()

    def _cmd_ucinewgame(self):
        self.session_id = None
        self.player_id = None
        self.engine_color = None
        self.human_color = None
        self.start_fen = None
        self.moves = []
        self.synced_move_count = 0
        self._stop_event.clear()

    def _cmd_position(self, parts: List[str]):
        # position [fen <fenstring> | startpos] moves <move1> ...
        self.start_fen = None
        self.moves = []

        i = 1
        if i < len(parts) and parts[i] == 'startpos':
            i += 1
        elif i < len(parts) and parts[i] == 'fen':
            i += 1
            fen_parts = []
            while i < len(parts) and parts[i] != 'moves':
                fen_parts.append(parts[i])
                i += 1
            self.start_fen = ' '.join(fen_parts)

        if i < len(parts) and parts[i] == 'moves':
            i += 1
            self.moves = parts[i:]

    def _cmd_go(self, parts: List[str]):
        self._stop_event.clear()
        self._search_thread = threading.Thread(target=self._search, daemon=True)
        self._search_thread.start()

    def _cmd_stop(self):
        self._stop_event.set()

    def _cmd_quit(self):
        self._stop_event.set()
        self._running = False

    # --- Search / game logic ---

    def _get_side_to_move(self) -> str:
        """Determine which side is to move from position + moves."""
        if self.start_fen:
            parts = self.start_fen.split()
            base_color = 'white' if len(parts) < 2 or parts[1] == 'w' else 'black'
        else:
            base_color = 'white'

        # Each move flips the turn
        if len(self.moves) % 2 == 0:
            return base_color
        return 'black' if base_color == 'white' else 'white'

    def _search(self):
        """Execute search: sync with server and get AI move."""
        try:
            if self._stop_event.is_set():
                self._send("bestmove 0000")
                return

            side_to_move = self._get_side_to_move()

            if self.session_id is None:
                # First go — determine colors and start a game
                expected_engine_color = side_to_move
                self.engine_color = expected_engine_color
                self.human_color = 'black' if expected_engine_color == 'white' else 'white'

                self._debug_log(f"Engine color: {self.engine_color}")

                if self.start_fen:
                    self._send("info string Warning: custom FEN not supported; server uses standard start")

                if not self._start_game():
                    self._send("info string Failed to start game")
                    self._send("bestmove 0000")
                    return

                # Verify the server assigned the expected colors
                if self.engine_color != expected_engine_color:
                    self._send("info string Color mismatch - server assigned differently. Retry with ucinewgame.")
                    self._send("bestmove 0000")
                    self.session_id = None
                    return

            # Sync any new human moves to the server
            self._sync_human_moves()

            # Wait for the server AI to respond
            move = self._wait_for_ai_move()

            if move:
                self._send(f"bestmove {move}")
            else:
                self._send("bestmove 0000")

        except Exception as e:
            self._debug_log(f"Error: {e}")
            self._send("bestmove 0000")

    def _start_game(self) -> bool:
        """Start a new game via matchmaking. Returns True on success."""
        self._send("info string Starting matchmaking...")

        connection_id = str(uuid.uuid4())
        display_name = self.credentials.display_name or 'UCI Player'

        # Request our preferred color (the human side) so the AI gets the engine side
        try:
            self.api.join_matchmaking(
                connection_id=connection_id,
                display_name=display_name,
                is_ranked=self.ranked,
                opponent_type=self.opponent_type,
                time_controls=[self.time_control],
                preferred_color=self.human_color,
            )
        except APIError as e:
            self._debug_log(f"Matchmaking error: {e}")
            return False

        # Poll for match
        self._send("info string Waiting for opponent...")
        start_time = time.time()

        while not self._stop_event.is_set():
            elapsed = time.time() - start_time
            if elapsed > 120:
                self._debug_log("Matchmaking timeout (120s)")
                try:
                    self.api.leave_matchmaking(connection_id)
                except APIError:
                    pass
                return False

            try:
                status = self.api.get_matchmaking_status(connection_id)

                if status.get('status') == 'matched':
                    self.session_id = status.get('matchedSessionId')
                    self._send(f"info string Match found: {self.session_id}")
                    break

                if status.get('status') == 'expired':
                    self._debug_log("Queue expired")
                    return False

            except APIError as e:
                self._debug_log(f"Poll error: {e}")

            # Periodic status update
            elapsed_int = int(elapsed)
            if elapsed_int > 0 and elapsed_int % 5 == 0:
                self._send(f"info string Matchmaking... {elapsed_int}s")

            time.sleep(1)

        if self._stop_event.is_set():
            try:
                self.api.leave_matchmaking(connection_id)
            except APIError:
                pass
            return False

        if not self.session_id:
            return False

        # Small delay to let the game fully persist
        time.sleep(1)

        # Fetch game to find our player info
        try:
            game = self.api.get_game(self.session_id)

            # Find our player by user ID
            for player in game.players:
                if player.user_id and player.user_id == self.credentials.user_id:
                    self.player_id = player.id
                    self.human_color = player.color
                    self.engine_color = 'black' if player.color == 'white' else 'white'
                    break

            # Fallback: find by connection ID
            if not self.player_id:
                for player in game.players:
                    if player.id == connection_id:
                        self.player_id = player.id
                        self.human_color = player.color
                        self.engine_color = 'black' if player.color == 'white' else 'white'
                        break

            if not self.player_id:
                self._debug_log("Could not find player in game")
                return False

            self._debug_log(f"Human={self.human_color} Engine={self.engine_color} PlayerID={self.player_id}")

        except APIError as e:
            self._debug_log(f"Error fetching game: {e}")
            return False

        self.synced_move_count = 0
        return True

    def _sync_human_moves(self):
        """Submit any un-synced human moves to the server.

        Iterates through the position's move list starting from synced_move_count.
        Engine moves (already on the server) are skipped; human moves are submitted.
        """
        while self.synced_move_count < len(self.moves):
            if self._stop_event.is_set():
                return

            move_idx = self.synced_move_count
            move_str = self.moves[move_idx]

            # Determine if this is an engine (AI) move or human move
            if self.engine_color == 'white':
                is_engine_move = (move_idx % 2 == 0)
            else:
                is_engine_move = (move_idx % 2 == 1)

            if is_engine_move:
                # Server AI already played this move; skip it
                self.synced_move_count += 1
                continue

            # Human move — submit to server
            from_sq = move_str[:2]
            to_sq = move_str[2:4]
            promotion = move_str[4:] if len(move_str) > 4 else None

            self._debug_log(f"Submitting human move: {move_str}")

            try:
                result = self.api.make_move(
                    self.session_id,
                    self.player_id,
                    from_sq,
                    to_sq,
                    promotion or None,
                )
                if not result.get('success'):
                    error = result.get('error', 'unknown')
                    self._debug_log(f"Move rejected: {error}")
                    return
            except APIError as e:
                self._debug_log(f"Move error: {e}")
                return

            self.synced_move_count += 1
            time.sleep(0.1)

    def _wait_for_ai_move(self) -> Optional[str]:
        """Poll the server until the AI makes a move, then return it in UCI format."""
        self._debug_log("Waiting for AI move...")

        start = time.time()
        max_wait = 60

        while not self._stop_event.is_set():
            if time.time() - start > max_wait:
                self._debug_log("AI move timeout (60s)")
                return None

            try:
                game = self.api.get_game(self.session_id)

                # Game completed (checkmate, stalemate, resignation, etc.)
                if game.status == 'complete':
                    self._debug_log(f"Game over: winner={game.winner} reason={game.win_reason}")
                    server_moves = self.api.get_moves(self.session_id)
                    if server_moves and len(server_moves) > len(self.moves):
                        last = server_moves[-1]
                        return self._format_uci_move(last)
                    return None

                # Check if AI has moved (it's now the human's turn)
                if game.current_turn == self.human_color:
                    server_moves = self.api.get_moves(self.session_id)
                    if server_moves and len(server_moves) > len(self.moves):
                        last = server_moves[-1]
                        self._debug_log(f"AI move: {self._format_uci_move(last)}")
                        return self._format_uci_move(last)

            except APIError as e:
                self._debug_log(f"Poll error: {e}")

            time.sleep(0.5)

        return None

    @staticmethod
    def _format_uci_move(move) -> str:
        """Format a server Move object as a UCI move string (e.g. e2e4, e7e8q)."""
        result = move.from_square + move.to_square
        if move.promotion:
            result += move.promotion.lower()
        return result


def run_uci(config: Config, credentials: Credentials):
    """Entry point for UCI engine mode."""
    api = ChessmataAPI(config, credentials)
    engine = UCIEngine(api, credentials)
    engine.run()
