#!/usr/bin/env python3
"""Chessmata CLI - Terminal-based chess client.

A command-line interface for playing chess on the Chessmata server.
Supports authentication, interactive games, and matchmaking.
"""

import argparse
import getpass
import sys
from typing import Optional

from . import __version__
from .config import Config, Credentials, setup_interactive, get_config_dir
from .api import ChessmataAPI, APIError
from .game import play_game, create_and_play_game
from .matchmaking import start_matchmaking


def cmd_setup(args: argparse.Namespace) -> int:
    """Run interactive setup."""
    setup_interactive()
    return 0


def cmd_login(args: argparse.Namespace) -> int:
    """Login to Chessmata."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)

    # Get credentials
    if args.email:
        email = args.email
    elif config.email:
        email_input = input(f"Email [{config.email}]: ").strip()
        email = email_input if email_input else config.email
    else:
        email = input("Email: ").strip()

    if not email:
        print("Email is required.")
        return 1

    # Save email for next time
    if email != config.email:
        config.email = email
        config.save()

    password = getpass.getpass("Password: ")
    if not password:
        print("Password is required.")
        return 1

    # Attempt login
    print("Logging in...")
    try:
        creds = api.login(email, password)
        print(f"Successfully logged in as {creds.display_name or creds.email}")
        if creds.elo_rating:
            print(f"Elo Rating: {creds.elo_rating}")
        return 0
    except APIError as e:
        print(f"Login failed: {e}")
        return 1


def cmd_register(args: argparse.Namespace) -> int:
    """Register a new account."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)

    # Get registration info
    email = input("Email: ").strip()
    if not email:
        print("Email is required.")
        return 1

    display_name = input("Display Name: ").strip()
    if not display_name:
        print("Display name is required.")
        return 1

    password = getpass.getpass("Password: ")
    if not password:
        print("Password is required.")
        return 1

    password_confirm = getpass.getpass("Confirm Password: ")
    if password != password_confirm:
        print("Passwords do not match.")
        return 1

    # Attempt registration
    print("Creating account...")
    try:
        creds = api.register(email, password, display_name)
        print(f"Account created successfully!")
        print(f"Welcome, {creds.display_name}!")
        return 0
    except APIError as e:
        print(f"Registration failed: {e}")
        return 1


def cmd_logout(args: argparse.Namespace) -> int:
    """Logout from Chessmata."""
    config = Config.load()
    credentials = Credentials.load()

    if not credentials.is_logged_in():
        print("Not currently logged in.")
        return 0

    api = ChessmataAPI(config, credentials)

    print("Logging out...")
    try:
        api.logout()
        print("Successfully logged out.")
        return 0
    except APIError as e:
        # Still clear local credentials even if server logout fails
        credentials.clear()
        print("Logged out (local credentials cleared).")
        return 0


def cmd_status(args: argparse.Namespace) -> int:
    """Show current status."""
    config = Config.load()
    credentials = Credentials.load()

    print()
    print("=== Chessmata CLI Status ===")
    print()
    print(f"Config directory: {get_config_dir()}")
    print(f"Server: {config.server_url}")
    print()

    if credentials.is_logged_in():
        print("Logged in as:")
        print(f"  Email: {credentials.email}")
        print(f"  Display Name: {credentials.display_name}")
        if credentials.elo_rating:
            print(f"  Elo Rating: {credentials.elo_rating}")

        # Validate token
        api = ChessmataAPI(config, credentials)
        if api.validate_token():
            print("  Token: Valid")
        else:
            print("  Token: Expired (please login again)")
    else:
        print("Not logged in.")
        print("Use 'chessmata login' to authenticate.")

    print()
    return 0


def cmd_play(args: argparse.Namespace) -> int:
    """Play a game (create or join)."""
    config = Config.load()
    credentials = Credentials.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    if args.session:
        # Join existing game
        session_id = args.session

        # Extract session ID from URL if needed
        if '/' in session_id:
            # Assume it's a URL like https://example.com/game/abc123
            parts = session_id.rstrip('/').split('/')
            session_id = parts[-1]

        play_game(config, credentials, session_id, args.player_id)
    else:
        # Create new game
        create_and_play_game(config, credentials)

    return 0


def cmd_uci(args: argparse.Namespace) -> int:
    """Enter UCI engine mode (stdin/stdout protocol)."""
    config = Config.load()
    credentials = Credentials.load()

    if not credentials.is_logged_in():
        # Use stderr since stdout is reserved for UCI protocol
        print("Error: not logged in. Run 'chessmata login' first.", file=sys.stderr)
        return 1

    from .uci import run_uci
    run_uci(config, credentials)
    return 0


def cmd_leaderboard(args: argparse.Namespace) -> int:
    """View the leaderboard."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)

    leaderboard_type = args.type or 'players'
    print(f"\n=== Chessmata Leaderboard ({leaderboard_type.title()}) ===\n")

    try:
        entries = api.get_leaderboard(leaderboard_type)
    except APIError as e:
        print(f"Failed to fetch leaderboard: {e}")
        return 1

    if not entries:
        print("No entries found.")
        return 0

    # Header
    print(f"{'#':>3}  {'Name':<20}  {'Elo':>6}  {'W':>4}  {'L':>4}  {'D':>4}  {'Games':>5}")
    print('-' * 58)

    for entry in entries:
        rank = entry.get('rank', '-')
        name = entry.get('displayName', 'Unknown')
        elo = entry.get('eloRating', 0)
        wins = entry.get('wins', 0)
        losses = entry.get('losses', 0)
        draws = entry.get('draws', 0)
        games = entry.get('gamesPlayed', 0)
        print(f"{rank:>3}  {name:<20}  {elo:>6}  {wins:>4}  {losses:>4}  {draws:>4}  {games:>5}")

    print()
    return 0


def cmd_lookup(args: argparse.Namespace) -> int:
    """Look up a user by display name."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)

    try:
        data = api.lookup_user(args.name)
    except APIError as e:
        print(f"User not found: {e}")
        return 1

    print(f"\n  Display Name: {data.get('displayName', '?')}")
    print(f"  User ID:      {data.get('userId', '?')}")
    print(f"  Elo Rating:   {data.get('eloRating', '?')}")
    print()
    return 0


def cmd_history(args: argparse.Namespace) -> int:
    """View game history for a user."""
    config = Config.load()
    credentials = Credentials.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config, credentials)

    # Resolve user ID: use --user-id directly, or look up by display name, or use self
    user_id = args.user_id
    display_label = user_id or ''

    if not user_id and args.name:
        try:
            data = api.lookup_user(args.name)
            user_id = data.get('userId')
            display_label = args.name
        except APIError as e:
            print(f"User not found: {e}")
            return 1
    elif not user_id:
        if not credentials.is_logged_in():
            print("Not logged in. Specify --name or --user-id, or login first.")
            return 1
        user_id = credentials.user_id
        display_label = credentials.display_name or credentials.email or user_id

    # Build filters
    result_filter = args.result
    ranked_filter = None
    if args.ranked:
        ranked_filter = 'true'
    elif args.unranked:
        ranked_filter = 'false'

    page = args.page or 1
    limit = args.limit or 20

    try:
        data = api.get_user_game_history(
            user_id, page=page, limit=limit,
            result=result_filter, ranked=ranked_filter,
        )
    except APIError as e:
        print(f"Failed to fetch history: {e}")
        return 1

    games = data.get('games', [])
    total = data.get('total', 0)

    title_parts = ['Game History']
    if display_label:
        title_parts.append(f'for {display_label}')
    if result_filter:
        title_parts.append(f'({result_filter})')
    if ranked_filter == 'true':
        title_parts.append('[ranked]')
    elif ranked_filter == 'false':
        title_parts.append('[unranked]')

    print(f"\n=== {' '.join(title_parts)} ===")
    print(f"Page {page} | {total} total games\n")

    if not games:
        print("No games found.")
        return 0

    for g in games:
        white = g.get('whiteDisplayName') or g.get('whiteAgent') or '?'
        black = g.get('blackDisplayName') or g.get('blackAgent') or '?'
        winner = g.get('winner', '')
        reason = g.get('winReason', '')
        is_ranked = g.get('isRanked', False)
        elo_w = g.get('whiteEloChange', 0)
        elo_b = g.get('blackEloChange', 0)
        moves = g.get('totalMoves', 0)
        completed = g.get('completedAt', '')[:10]
        sid = g.get('sessionId', '')

        if winner == 'white':
            result_str = f"{white} won"
        elif winner == 'black':
            result_str = f"{black} won"
        else:
            result_str = "Draw"
        if reason:
            reason_label = reason.replace('_', ' ').title()
            result_str += f" ({reason_label})"

        ranked_tag = ' [R]' if is_ranked else ''
        elo_str = ''
        if is_ranked and (elo_w or elo_b):
            elo_str = f"  Elo: W{elo_w:+d} B{elo_b:+d}"

        print(f"  {completed}  {white} vs {black}{ranked_tag}")
        print(f"           {result_str}  |  {moves} moves{elo_str}  [{sid}]")

    if len(games) < total:
        remaining = total - (page * limit)
        if remaining > 0:
            print(f"\n  ({remaining} more — use --page {page + 1})")

    print()
    return 0


def cmd_games(args: argparse.Namespace) -> int:
    """List recent active or completed games."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)

    ranked_filter = None
    if args.ranked:
        ranked_filter = 'true'
    elif args.unranked:
        ranked_filter = 'false'

    limit = args.limit or 20

    if args.completed:
        # Show completed games
        try:
            games = api.list_completed_games(limit=limit, ranked=ranked_filter)
        except APIError as e:
            print(f"Failed to fetch games: {e}")
            return 1

        print(f"\n=== Recently Completed Games ===\n")

        if not games:
            print("No completed games found.")
            return 0

        for g in games:
            players = g.get('players', [])
            white_name = '?'
            black_name = '?'
            for p in players:
                if p.get('color') == 'white':
                    white_name = p.get('displayName') or p.get('agentName') or '?'
                elif p.get('color') == 'black':
                    black_name = p.get('displayName') or p.get('agentName') or '?'

            winner = g.get('winner', '')
            reason = g.get('winReason', '')
            is_ranked = g.get('isRanked', False)
            completed = (g.get('completedAt') or '')[:10]
            sid = g.get('sessionId', '')

            if winner == 'white':
                result_str = f"{white_name} won"
            elif winner == 'black':
                result_str = f"{black_name} won"
            else:
                result_str = "Draw"
            if reason:
                result_str += f" ({reason.replace('_', ' ').title()})"

            ranked_tag = ' [R]' if is_ranked else ''
            print(f"  {completed}  {white_name} vs {black_name}{ranked_tag}")
            print(f"           {result_str}  [{sid}]")
    else:
        # Show active games (default)
        inactive_mins = args.inactive_mins or 10
        try:
            games = api.list_active_games(limit=limit, inactive_mins=inactive_mins, ranked=ranked_filter)
        except APIError as e:
            print(f"Failed to fetch games: {e}")
            return 1

        print(f"\n=== Active Games ===\n")

        if not games:
            print("No active games right now.")
            return 0

        for g in games:
            players = g.get('players', [])
            white_name = '?'
            black_name = '?'
            for p in players:
                if p.get('color') == 'white':
                    white_name = p.get('displayName') or p.get('agentName') or '?'
                elif p.get('color') == 'black':
                    black_name = p.get('displayName') or p.get('agentName') or '?'

            turn = g.get('currentTurn', '?')
            is_ranked = g.get('isRanked', False)
            tc = g.get('timeControl', {})
            tc_mode = tc.get('mode', 'unlimited') if tc else 'unlimited'
            sid = g.get('sessionId', '')

            ranked_tag = ' [R]' if is_ranked else ''
            tc_tag = f' ({tc_mode})' if tc_mode != 'unlimited' else ''
            print(f"  {white_name} vs {black_name}{ranked_tag}{tc_tag}")
            print(f"           {turn}'s turn  [{sid}]")

    print()
    return 0


def _format_time(ms: int) -> str:
    """Format milliseconds as M:SS."""
    if ms <= 0:
        return "0:00"
    total_seconds = ms // 1000
    minutes = total_seconds // 60
    seconds = total_seconds % 60
    return f"{minutes}:{seconds:02d}"


def cmd_game(args: argparse.Namespace) -> int:
    """View details and moves for a game by session ID."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config)
    session_id = args.session_id

    # Extract session ID from URL if needed
    if '/' in session_id:
        parts = session_id.rstrip('/').split('/')
        session_id = parts[-1]

    # Fetch game state
    try:
        game = api.get_game(session_id)
    except APIError as e:
        print(f"Game not found: {e}")
        return 1

    # Display game info
    white = None
    black = None
    for p in game.players:
        if p.color == 'white':
            white = p
        elif p.color == 'black':
            black = p

    def _player_line(p):
        if p is None:
            return '(waiting for opponent)'
        if p.agent_name:
            name = p.agent_name
            label = f"{name} [agent]"
        elif p.user_id:
            name = p.display_name or '?'
            label = f"{name} [user: {p.user_id}]"
        else:
            name = p.display_name or '?'
            label = f"{name} [guest]"
        if p.elo_rating:
            label += f" (Elo {p.elo_rating})"
        return label

    white_name = (white.display_name or white.agent_name or '?') if white else '?'
    black_name = (black.display_name or black.agent_name or '?') if black else '?'

    print(f"\n=== Game {session_id[:16]} ===\n")
    print(f"  White: {_player_line(white)}")
    print(f"  Black: {_player_line(black)}")
    print(f"  Status: {game.status}")

    if game.is_ranked:
        print("  Ranked: Yes")

    if game.time_control and game.time_control.mode != 'unlimited':
        base = _format_time(game.time_control.base_time_ms)
        inc = game.time_control.increment_ms // 1000
        print(f"  Time Control: {game.time_control.mode} ({base}+{inc}s)")

    if game.status == 'active':
        print(f"  Turn: {game.current_turn}")
        if game.player_times:
            print(f"  White Time: {_format_time(game.player_times.white_remaining_ms)}")
            print(f"  Black Time: {_format_time(game.player_times.black_remaining_ms)}")
    elif game.status == 'complete':
        if game.winner:
            winner_name = white_name if game.winner == 'white' else black_name
            print(f"  Winner: {winner_name} ({game.winner})")
        else:
            print("  Result: Draw")
        if game.win_reason:
            print(f"  Reason: {game.win_reason.replace('_', ' ').title()}")
        if game.elo_changes:
            print(f"  Elo Change: White {game.elo_changes.white_change:+d}, Black {game.elo_changes.black_change:+d}")

    # Fetch and display moves
    if not args.no_moves:
        try:
            moves = api.get_moves(session_id)
        except APIError as e:
            print(f"\n  (Failed to fetch moves: {e})")
            return 0

        if moves:
            print(f"\n  Moves ({len(moves)}):")
            print(f"  {'#':>4}  {'White':<10}  {'Black':<10}")
            print(f"  {'-'*28}")

            # Pair moves into white/black rows
            i = 0
            move_num = 1
            while i < len(moves):
                white_notation = ''
                black_notation = ''

                if i < len(moves) and moves[i].from_square:
                    white_notation = moves[i].notation or f"{moves[i].from_square}-{moves[i].to_square}"
                    if moves[i].check:
                        white_notation += '+'
                    if moves[i].checkmate:
                        white_notation = white_notation.rstrip('+') + '#'
                    i += 1

                if i < len(moves) and moves[i].from_square:
                    black_notation = moves[i].notation or f"{moves[i].from_square}-{moves[i].to_square}"
                    if moves[i].check:
                        black_notation += '+'
                    if moves[i].checkmate:
                        black_notation = black_notation.rstrip('+') + '#'
                    i += 1

                if white_notation or black_notation:
                    print(f"  {move_num:>4}. {white_notation:<10}  {black_notation:<10}")
                    move_num += 1
                else:
                    # Skip non-move entries (e.g., resignation notation)
                    if i < len(moves):
                        note = moves[i].notation
                        if note:
                            print(f"        {note}")
                    i += 1
        else:
            print("\n  No moves yet.")

    print()
    return 0


def cmd_lobby(args: argparse.Namespace) -> int:
    """Show matchmaking lobby (players/agents waiting for a match)."""
    config = Config.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    api = ChessmataAPI(config.server_url)

    try:
        entries = api.get_lobby()
    except Exception as e:
        print(f"Error: {e}")
        return 1

    if not entries:
        print("\n  No one is waiting for a match right now.\n")
        return 0

    print(f"\n  Matchmaking Lobby ({len(entries)} waiting)")
    print("  " + "-" * 70)

    # Header
    print(f"  {'Name':<20} {'Type':<8} {'Elo':>5}  {'Ranked':<7} {'Time Controls':<20} {'Waiting'}")
    print("  " + "-" * 70)

    from datetime import datetime, timezone

    for entry in entries:
        name = entry.get('displayName', '?')
        agent = entry.get('agentName', '')
        if agent:
            name = f"{name} ({agent})"
        if len(name) > 19:
            name = name[:17] + '..'

        is_agent = 'Agent' if agent else 'Human'

        elo = entry.get('currentElo', 1200)
        ranked = 'Yes' if entry.get('isRanked') else 'No'

        tc_list = entry.get('timeControls', [])
        tc_str = ', '.join(tc_list) if tc_list else 'any'
        if len(tc_str) > 19:
            tc_str = tc_str[:17] + '..'

        # Calculate wait time
        wait_since = entry.get('waitingSince', '')
        wait_str = ''
        if wait_since:
            try:
                dt = datetime.fromisoformat(wait_since.replace('Z', '+00:00'))
                diff = datetime.now(timezone.utc) - dt
                secs = int(diff.total_seconds())
                if secs < 60:
                    wait_str = f"{secs}s"
                elif secs < 3600:
                    wait_str = f"{secs // 60}m"
                else:
                    wait_str = f"{secs // 3600}h {(secs % 3600) // 60}m"
            except (ValueError, TypeError):
                pass

        print(f"  {name:<20} {is_agent:<8} {elo:>5}  {ranked:<7} {tc_str:<20} {wait_str}")

    print()
    return 0


def cmd_match(args: argparse.Namespace) -> int:
    """Find opponent via matchmaking."""
    config = Config.load()
    credentials = Credentials.load()

    if not config.is_configured():
        print("Client not configured. Running setup first...")
        setup_interactive()
        config = Config.load()

    # Parse opponent type
    opponent_type = 'either'
    if args.humans_only:
        opponent_type = 'human'
    elif args.agents_only:
        opponent_type = 'ai'

    start_matchmaking(
        config=config,
        credentials=credentials,
        is_ranked=args.ranked,
        opponent_type=opponent_type,
        engine_name=getattr(args, 'engine_name', None),
    )

    return 0


def main() -> int:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        prog='chessmata',
        description='Chessmata CLI - Terminal-based chess client',
    )
    parser.add_argument(
        '-v', '--version',
        action='version',
        version=f'%(prog)s {__version__}',
    )

    subparsers = parser.add_subparsers(dest='command', help='Available commands')

    # Setup command
    setup_parser = subparsers.add_parser(
        'setup',
        help='Configure the CLI client',
    )
    setup_parser.set_defaults(func=cmd_setup)

    # Login command
    login_parser = subparsers.add_parser(
        'login',
        help='Login to your Chessmata account',
    )
    login_parser.add_argument(
        '-e', '--email',
        help='Email address',
    )
    login_parser.set_defaults(func=cmd_login)

    # Register command
    register_parser = subparsers.add_parser(
        'register',
        help='Create a new Chessmata account',
    )
    register_parser.set_defaults(func=cmd_register)

    # Logout command
    logout_parser = subparsers.add_parser(
        'logout',
        help='Logout from your account',
    )
    logout_parser.set_defaults(func=cmd_logout)

    # Status command
    status_parser = subparsers.add_parser(
        'status',
        help='Show current login status',
    )
    status_parser.set_defaults(func=cmd_status)

    # Play command
    play_parser = subparsers.add_parser(
        'play',
        help='Play a game (create new or join existing)',
    )
    play_parser.add_argument(
        'session',
        nargs='?',
        help='Session ID or game URL to join (omit to create new game)',
    )
    play_parser.add_argument(
        '-p', '--player-id',
        help='Player ID for rejoining a game',
    )
    play_parser.set_defaults(func=cmd_play)

    # Match command
    match_parser = subparsers.add_parser(
        'match',
        help='Find opponent via matchmaking',
    )
    match_parser.add_argument(
        '-r', '--ranked',
        action='store_true',
        help='Play a ranked game (requires login)',
    )
    match_parser.add_argument(
        '--humans-only',
        action='store_true',
        help='Only match with human players',
    )
    match_parser.add_argument(
        '--agents-only',
        action='store_true',
        help='Only match with AI agents',
    )
    match_parser.add_argument(
        '--engine-name',
        help='Engine name (prevents matching with agents using the same engine)',
    )
    match_parser.set_defaults(func=cmd_match)

    # Lobby command
    lobby_parser = subparsers.add_parser(
        'lobby',
        help='Show players/agents waiting for a match',
    )
    lobby_parser.set_defaults(func=cmd_lobby)

    # Leaderboard command
    leaderboard_parser = subparsers.add_parser(
        'leaderboard',
        help='View the leaderboard',
    )
    leaderboard_parser.add_argument(
        '-t', '--type',
        choices=['players', 'agents'],
        default='players',
        help='Leaderboard type: players (default) or agents',
    )
    leaderboard_parser.set_defaults(func=cmd_leaderboard)

    # Lookup command
    lookup_parser = subparsers.add_parser(
        'lookup',
        help='Look up a user by display name',
    )
    lookup_parser.add_argument(
        'name',
        help='Display name to look up',
    )
    lookup_parser.set_defaults(func=cmd_lookup)

    # History command
    history_parser = subparsers.add_parser(
        'history',
        help='View game history for a user',
    )
    history_group = history_parser.add_mutually_exclusive_group()
    history_group.add_argument(
        '-n', '--name',
        help='Look up user by display name',
    )
    history_group.add_argument(
        '--user-id',
        help='User ID directly',
    )
    history_parser.add_argument(
        '--result',
        choices=['wins', 'losses', 'draws'],
        help='Filter by result',
    )
    history_ranked = history_parser.add_mutually_exclusive_group()
    history_ranked.add_argument(
        '--ranked',
        action='store_true',
        help='Show only ranked games',
    )
    history_ranked.add_argument(
        '--unranked',
        action='store_true',
        help='Show only unranked games',
    )
    history_parser.add_argument(
        '--page',
        type=int,
        default=1,
        help='Page number (default: 1)',
    )
    history_parser.add_argument(
        '--limit',
        type=int,
        default=20,
        help='Results per page (default: 20)',
    )
    history_parser.set_defaults(func=cmd_history)

    # Games command
    games_parser = subparsers.add_parser(
        'games',
        help='List recent active or completed games',
    )
    games_parser.add_argument(
        '-c', '--completed',
        action='store_true',
        help='Show completed games instead of active',
    )
    games_ranked = games_parser.add_mutually_exclusive_group()
    games_ranked.add_argument(
        '--ranked',
        action='store_true',
        help='Show only ranked games',
    )
    games_ranked.add_argument(
        '--unranked',
        action='store_true',
        help='Show only unranked games',
    )
    games_parser.add_argument(
        '--limit',
        type=int,
        default=20,
        help='Max results (default: 20)',
    )
    games_parser.add_argument(
        '--inactive-mins',
        type=int,
        default=10,
        help='For active games: max minutes since last activity (default: 10)',
    )
    games_parser.set_defaults(func=cmd_games)

    # Game command (view single game)
    game_parser = subparsers.add_parser(
        'game',
        help='View details and moves for a game',
    )
    game_parser.add_argument(
        'session_id',
        help='Session ID or game URL',
    )
    game_parser.add_argument(
        '--no-moves',
        action='store_true',
        help='Skip showing moves',
    )
    game_parser.set_defaults(func=cmd_game)

    # UCI command
    uci_parser = subparsers.add_parser(
        'uci',
        help='Run as a UCI engine (for chess GUIs like Arena, CuteChess)',
    )
    uci_parser.set_defaults(func=cmd_uci)

    # Parse arguments
    args = parser.parse_args()

    # Show help if no command
    if not args.command:
        # Check if logged in and show status
        credentials = Credentials.load()
        if credentials.is_logged_in():
            print(f"Logged in as: {credentials.display_name or credentials.email}")
        else:
            print("Not logged in. Use 'chessmata login' to authenticate.")
        print()
        parser.print_help()
        return 0

    # Run command
    return args.func(args)


if __name__ == '__main__':
    sys.exit(main())
