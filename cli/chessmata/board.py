"""Chess board rendering and FEN parsing for terminal display."""

from typing import Any, Dict, List, Optional, Tuple


# Unicode chess pieces
PIECES_UNICODE = {
    'K': '\u2654',  # White King
    'Q': '\u2655',  # White Queen
    'R': '\u2656',  # White Rook
    'B': '\u2657',  # White Bishop
    'N': '\u2658',  # White Knight
    'P': '\u2659',  # White Pawn
    'k': '\u265A',  # Black King
    'q': '\u265B',  # Black Queen
    'r': '\u265C',  # Black Rook
    'b': '\u265D',  # Black Bishop
    'n': '\u265E',  # Black Knight
    'p': '\u265F',  # Black Pawn
}

# ASCII chess pieces (fallback)
PIECES_ASCII = {
    'K': 'K', 'Q': 'Q', 'R': 'R', 'B': 'B', 'N': 'N', 'P': 'P',
    'k': 'k', 'q': 'q', 'r': 'r', 'b': 'b', 'n': 'n', 'p': 'p',
}

# ANSI color codes
RESET = '\033[0m'
BOLD = '\033[1m'
WHITE_PIECE = '\033[97m'  # Bright white
BLACK_PIECE = '\033[90m'  # Dark gray
LIGHT_SQUARE_BG = '\033[48;5;180m'  # Tan
DARK_SQUARE_BG = '\033[48;5;94m'   # Brown
HIGHLIGHT_BG = '\033[48;5;220m'    # Yellow highlight
CHECK_BG = '\033[48;5;196m'        # Red for check


def parse_fen(fen: str) -> Dict[str, str]:
    """Parse FEN string to get piece positions.

    Returns a dict mapping square names (e.g., 'e4') to piece chars (e.g., 'K', 'p').
    """
    board = {}

    # Split FEN to get just the board part
    parts = fen.split(' ')
    board_part = parts[0]

    ranks = board_part.split('/')

    for rank_idx, rank in enumerate(ranks):
        file_idx = 0
        actual_rank = 8 - rank_idx  # FEN starts from rank 8

        for char in rank:
            if char.isdigit():
                # Empty squares
                file_idx += int(char)
            else:
                # Piece
                file_letter = chr(ord('a') + file_idx)
                square = f"{file_letter}{actual_rank}"
                board[square] = char
                file_idx += 1

    return board


def get_current_turn_from_fen(fen: str) -> str:
    """Extract whose turn it is from FEN string."""
    parts = fen.split(' ')
    if len(parts) > 1:
        return 'white' if parts[1] == 'w' else 'black'
    return 'white'


def parse_fen_state(fen: str) -> Dict[str, Any]:
    """Parse full FEN string to extract game state info.

    Returns a dict with:
        castling: str - castling rights (e.g., 'KQkq' or '-')
        en_passant: str - en passant target square (e.g., 'e3' or '-')
        halfmove: int - halfmove clock
        fullmove: int - fullmove number
    """
    parts = fen.split(' ')
    return {
        'castling': parts[2] if len(parts) > 2 else '-',
        'en_passant': parts[3] if len(parts) > 3 else '-',
        'halfmove': int(parts[4]) if len(parts) > 4 else 0,
        'fullmove': int(parts[5]) if len(parts) > 5 else 1,
    }


def format_castling_rights(castling: str, player_color: str) -> str:
    """Format castling rights for display.

    Args:
        castling: FEN castling rights string (e.g., 'KQkq')
        player_color: 'white' or 'black'

    Returns:
        Human-readable castling rights string
    """
    if castling == '-':
        return 'None'

    rights = []
    if player_color == 'white':
        if 'K' in castling:
            rights.append('O-O')
        if 'Q' in castling:
            rights.append('O-O-O')
    else:
        if 'k' in castling:
            rights.append('O-O')
        if 'q' in castling:
            rights.append('O-O-O')

    return ', '.join(rights) if rights else 'None'


def render_board(
    fen: str,
    player_color: str = 'white',
    use_unicode: bool = True,
    use_color: bool = True,
    last_move: Optional[Tuple[str, str]] = None,
    highlight_squares: Optional[List[str]] = None,
    in_check: bool = False,
    show_state: bool = True,
) -> str:
    """Render a chess board as a string for terminal display.

    Args:
        fen: Board state in FEN notation
        player_color: 'white' or 'black' - determines board orientation
        use_unicode: Use Unicode chess symbols (requires compatible terminal)
        use_color: Use ANSI colors
        last_move: Tuple of (from_square, to_square) to highlight
        highlight_squares: List of squares to highlight
        in_check: Whether the current player's king is in check
        show_state: Show castling rights and en passant info

    Returns:
        String representation of the board
    """
    pieces = PIECES_UNICODE if use_unicode else PIECES_ASCII
    board = parse_fen(fen)

    lines = []

    # Determine board orientation
    if player_color == 'white':
        ranks = range(8, 0, -1)
        files = 'abcdefgh'
    else:
        ranks = range(1, 9)
        files = 'hgfedcba'

    # Top border with file labels
    file_labels = '    ' + '  '.join(files.upper()) + '  '
    lines.append(file_labels)
    lines.append('  +' + '-' * 25 + '+')

    for rank in ranks:
        row = f'{rank} |'

        for file in files:
            square = f'{file}{rank}'
            piece = board.get(square, '')

            # Determine square color
            file_idx = ord(file) - ord('a')
            is_light = (file_idx + rank) % 2 == 1

            # Check if this square should be highlighted
            is_highlighted = False
            is_check_square = False

            if last_move and square in last_move:
                is_highlighted = True
            if highlight_squares and square in highlight_squares:
                is_highlighted = True

            # Check if this is the king in check
            if in_check and piece:
                current_turn = get_current_turn_from_fen(fen)
                if (current_turn == 'white' and piece == 'K') or (current_turn == 'black' and piece == 'k'):
                    is_check_square = True

            if use_color:
                # Choose background color
                if is_check_square:
                    bg = CHECK_BG
                elif is_highlighted:
                    bg = HIGHLIGHT_BG
                elif is_light:
                    bg = LIGHT_SQUARE_BG
                else:
                    bg = DARK_SQUARE_BG

                if piece:
                    # Choose piece color
                    piece_color = WHITE_PIECE if piece.isupper() else BLACK_PIECE
                    piece_char = pieces.get(piece, piece)
                    row += f'{bg}{piece_color}{BOLD} {piece_char} {RESET}'
                else:
                    row += f'{bg}   {RESET}'
            else:
                if piece:
                    piece_char = pieces.get(piece, piece)
                    row += f' {piece_char} '
                else:
                    # Use dots for light squares, spaces for dark (or vice versa)
                    row += ' . ' if is_light else '   '

        row += f'| {rank}'
        lines.append(row)

    # Bottom border and file labels
    lines.append('  +' + '-' * 25 + '+')
    lines.append(file_labels)

    # Show game state info (castling rights, en passant)
    if show_state:
        state = parse_fen_state(fen)
        state_parts = []

        castling_str = format_castling_rights(state['castling'], player_color)
        if castling_str != 'None':
            state_parts.append(f'Castling: {castling_str}')

        if state['en_passant'] != '-':
            state_parts.append(f'En passant: {state["en_passant"]}')

        if state_parts:
            lines.append('  ' + '  |  '.join(state_parts))

    return '\n'.join(lines)


def format_move_history(moves: list, columns: int = 2) -> str:
    """Format move history for display.

    Args:
        moves: List of Move objects
        columns: Number of move pairs per row (for compact display)

    Returns:
        Formatted move history string
    """
    if not moves:
        return "No moves yet."

    lines = []
    move_pairs = []
    current_pair = []

    for move in moves:
        notation = move.notation

        # Add check/checkmate symbols if not already present
        if move.checkmate and '#' not in notation:
            notation += '#'
        elif move.check and '+' not in notation:
            notation += '+'

        current_pair.append(notation)

        if len(current_pair) == 2:
            move_pairs.append(current_pair)
            current_pair = []

    # Handle odd number of moves
    if current_pair:
        move_pairs.append(current_pair)

    # Format output
    for i, pair in enumerate(move_pairs):
        move_num = i + 1
        if len(pair) == 2:
            lines.append(f"{move_num:3}. {pair[0]:<8} {pair[1]}")
        else:
            lines.append(f"{move_num:3}. {pair[0]}")

    return '\n'.join(lines)


def square_to_coords(square: str) -> Tuple[int, int]:
    """Convert square notation (e.g., 'e4') to coordinates (file, rank)."""
    file = ord(square[0]) - ord('a')
    rank = int(square[1]) - 1
    return file, rank


def coords_to_square(file: int, rank: int) -> str:
    """Convert coordinates to square notation."""
    return f"{chr(ord('a') + file)}{rank + 1}"


def is_valid_square(square: str) -> bool:
    """Check if a square notation is valid."""
    if len(square) != 2:
        return False
    file, rank = square[0], square[1]
    return file in 'abcdefgh' and rank in '12345678'


def is_promotion_move(fen: str, from_square: str, to_square: str) -> bool:
    """Check if a move is a pawn promotion (pawn reaching rank 1 or 8).

    Args:
        fen: Current board state in FEN notation
        from_square: Source square (e.g., 'e7')
        to_square: Target square (e.g., 'e8')

    Returns:
        True if this is a pawn promotion move
    """
    board = parse_fen(fen)
    piece = board.get(from_square, '')
    if not piece:
        return False

    # Check if the piece is a pawn
    if piece not in ('P', 'p'):
        return False

    # Check if moving to rank 1 or 8
    to_rank = to_square[1]
    return to_rank in ('1', '8')


def parse_move_input(move_input: str) -> Optional[Tuple[str, str, Optional[str]]]:
    """Parse user move input into (from, to, promotion).

    Supports formats:
    - 'e2e4' or 'e2 e4' - coordinate notation
    - 'e2-e4' or 'e2 to e4' - coordinate with separator
    - 'e7e8q' or 'e7e8=q' - pawn promotion

    Returns:
        Tuple of (from_square, to_square, promotion) or None if invalid
    """
    move_input = move_input.strip().lower()

    # Remove common separators
    move_input = move_input.replace('-', '').replace(' to ', '').replace(' ', '')

    # Handle promotion notation
    promotion = None
    if len(move_input) >= 5:
        if move_input[-2] == '=':
            promotion = move_input[-1]
            move_input = move_input[:-2]
        elif move_input[-1] in 'qrbn':
            # Check if it's a promotion (to rank 8 or 1)
            if len(move_input) == 5:
                promotion = move_input[-1]
                move_input = move_input[:-1]

    if len(move_input) != 4:
        return None

    from_square = move_input[:2]
    to_square = move_input[2:4]

    if not is_valid_square(from_square) or not is_valid_square(to_square):
        return None

    return from_square, to_square, promotion
