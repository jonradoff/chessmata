package game

import (
	"fmt"
	"strings"
	"unicode"
)

// Piece types
const (
	Pawn   = 'P'
	Knight = 'N'
	Bishop = 'B'
	Rook   = 'R'
	Queen  = 'Q'
	King   = 'K'
)

// Board represents a chess board state
type Board struct {
	Squares     [8][8]rune // Uppercase = white, lowercase = black, 0 = empty
	WhiteToMove bool
	CastleRights struct {
		WhiteKingSide  bool
		WhiteQueenSide bool
		BlackKingSide  bool
		BlackQueenSide bool
	}
	EnPassantSquare string
	HalfMoveClock   int
	FullMoveNumber  int
}

// Position represents a square on the board
type Position struct {
	File int // 0-7 (a-h)
	Rank int // 0-7 (1-8)
}

// ParsePosition converts algebraic notation (e.g., "e4") to Position
func ParsePosition(s string) (Position, error) {
	if len(s) != 2 {
		return Position{}, fmt.Errorf("invalid position: %s", s)
	}
	file := int(s[0] - 'a')
	rank := int(s[1] - '1')
	if file < 0 || file > 7 || rank < 0 || rank > 7 {
		return Position{}, fmt.Errorf("invalid position: %s", s)
	}
	return Position{File: file, Rank: rank}, nil
}

// String converts Position to algebraic notation
func (p Position) String() string {
	return fmt.Sprintf("%c%c", 'a'+p.File, '1'+p.Rank)
}

// ParseFEN parses a FEN string into a Board
func ParseFEN(fen string) (*Board, error) {
	parts := strings.Split(fen, " ")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid FEN: expected 6 parts, got %d", len(parts))
	}

	board := &Board{}

	// Parse piece placement
	ranks := strings.Split(parts[0], "/")
	if len(ranks) != 8 {
		return nil, fmt.Errorf("invalid FEN: expected 8 ranks")
	}

	for r := 7; r >= 0; r-- {
		file := 0
		for _, c := range ranks[7-r] {
			if unicode.IsDigit(c) {
				file += int(c - '0')
			} else {
				board.Squares[r][file] = c
				file++
			}
		}
	}

	// Parse active color
	board.WhiteToMove = parts[1] == "w"

	// Parse castling rights
	board.CastleRights.WhiteKingSide = strings.Contains(parts[2], "K")
	board.CastleRights.WhiteQueenSide = strings.Contains(parts[2], "Q")
	board.CastleRights.BlackKingSide = strings.Contains(parts[2], "k")
	board.CastleRights.BlackQueenSide = strings.Contains(parts[2], "q")

	// Parse en passant square
	if parts[3] != "-" {
		board.EnPassantSquare = parts[3]
	}

	// Parse half-move clock and full move number
	fmt.Sscanf(parts[4], "%d", &board.HalfMoveClock)
	fmt.Sscanf(parts[5], "%d", &board.FullMoveNumber)

	return board, nil
}

// ToFEN converts the Board to FEN notation
func (b *Board) ToFEN() string {
	var sb strings.Builder

	// Piece placement
	for r := 7; r >= 0; r-- {
		empty := 0
		for f := 0; f < 8; f++ {
			piece := b.Squares[r][f]
			if piece == 0 {
				empty++
			} else {
				if empty > 0 {
					sb.WriteRune(rune('0' + empty))
					empty = 0
				}
				sb.WriteRune(piece)
			}
		}
		if empty > 0 {
			sb.WriteRune(rune('0' + empty))
		}
		if r > 0 {
			sb.WriteRune('/')
		}
	}

	// Active color
	if b.WhiteToMove {
		sb.WriteString(" w ")
	} else {
		sb.WriteString(" b ")
	}

	// Castling rights
	castling := ""
	if b.CastleRights.WhiteKingSide {
		castling += "K"
	}
	if b.CastleRights.WhiteQueenSide {
		castling += "Q"
	}
	if b.CastleRights.BlackKingSide {
		castling += "k"
	}
	if b.CastleRights.BlackQueenSide {
		castling += "q"
	}
	if castling == "" {
		castling = "-"
	}
	sb.WriteString(castling)

	// En passant
	sb.WriteString(" ")
	if b.EnPassantSquare != "" {
		sb.WriteString(b.EnPassantSquare)
	} else {
		sb.WriteString("-")
	}

	// Half-move clock and full move number
	sb.WriteString(fmt.Sprintf(" %d %d", b.HalfMoveClock, b.FullMoveNumber))

	return sb.String()
}

// GetPiece returns the piece at the given position
func (b *Board) GetPiece(pos Position) rune {
	return b.Squares[pos.Rank][pos.File]
}

// IsWhitePiece returns true if the piece is white
func IsWhitePiece(piece rune) bool {
	return unicode.IsUpper(piece)
}

// IsBlackPiece returns true if the piece is black
func IsBlackPiece(piece rune) bool {
	return unicode.IsLower(piece) && piece != 0
}

// ValidateMove checks if a move is legal
func (b *Board) ValidateMove(from, to Position) error {
	piece := b.GetPiece(from)
	if piece == 0 {
		return fmt.Errorf("no piece at %s", from.String())
	}

	isWhite := IsWhitePiece(piece)
	if isWhite != b.WhiteToMove {
		return fmt.Errorf("not your turn")
	}

	// Check if destination has own piece
	destPiece := b.GetPiece(to)
	if destPiece != 0 {
		if IsWhitePiece(destPiece) == isWhite {
			return fmt.Errorf("cannot capture own piece")
		}
	}

	// Validate move based on piece type
	pieceType := unicode.ToUpper(piece)
	switch pieceType {
	case Pawn:
		if !b.isValidPawnMove(from, to, isWhite) {
			return fmt.Errorf("invalid pawn move")
		}
	case Knight:
		if !b.isValidKnightMove(from, to) {
			return fmt.Errorf("invalid knight move")
		}
	case Bishop:
		if !b.isValidBishopMove(from, to) {
			return fmt.Errorf("invalid bishop move")
		}
	case Rook:
		if !b.isValidRookMove(from, to) {
			return fmt.Errorf("invalid rook move")
		}
	case Queen:
		if !b.isValidQueenMove(from, to) {
			return fmt.Errorf("invalid queen move")
		}
	case King:
		if !b.isValidKingMove(from, to, isWhite) {
			return fmt.Errorf("invalid king move")
		}
	}

	// Make the move temporarily to check for check
	tempBoard := b.Copy()
	tempBoard.makeMove(from, to)
	if tempBoard.IsInCheck(isWhite) {
		return fmt.Errorf("move leaves king in check")
	}

	return nil
}

func (b *Board) isValidPawnMove(from, to Position, isWhite bool) bool {
	direction := 1
	startRank := 1
	if !isWhite {
		direction = -1
		startRank = 6
	}

	fileDiff := to.File - from.File
	rankDiff := to.Rank - from.Rank

	// Forward move
	if fileDiff == 0 {
		if rankDiff == direction && b.GetPiece(to) == 0 {
			return true
		}
		// Double move from starting position
		if from.Rank == startRank && rankDiff == 2*direction {
			midPos := Position{File: from.File, Rank: from.Rank + direction}
			if b.GetPiece(midPos) == 0 && b.GetPiece(to) == 0 {
				return true
			}
		}
	}

	// Capture
	if abs(fileDiff) == 1 && rankDiff == direction {
		destPiece := b.GetPiece(to)
		if destPiece != 0 && IsWhitePiece(destPiece) != isWhite {
			return true
		}
		// En passant
		if to.String() == b.EnPassantSquare {
			return true
		}
	}

	return false
}

func (b *Board) isValidKnightMove(from, to Position) bool {
	fileDiff := abs(to.File - from.File)
	rankDiff := abs(to.Rank - from.Rank)
	return (fileDiff == 2 && rankDiff == 1) || (fileDiff == 1 && rankDiff == 2)
}

func (b *Board) isValidBishopMove(from, to Position) bool {
	fileDiff := abs(to.File - from.File)
	rankDiff := abs(to.Rank - from.Rank)
	if fileDiff != rankDiff {
		return false
	}
	return b.isPathClear(from, to)
}

func (b *Board) isValidRookMove(from, to Position) bool {
	if from.File != to.File && from.Rank != to.Rank {
		return false
	}
	return b.isPathClear(from, to)
}

func (b *Board) isValidQueenMove(from, to Position) bool {
	return b.isValidBishopMove(from, to) || b.isValidRookMove(from, to)
}

func (b *Board) isValidKingMove(from, to Position, isWhite bool) bool {
	fileDiff := abs(to.File - from.File)
	rankDiff := abs(to.Rank - from.Rank)

	// Normal king move
	if fileDiff <= 1 && rankDiff <= 1 {
		return true
	}

	// Castling
	if rankDiff == 0 && fileDiff == 2 {
		if isWhite && from.Rank == 0 {
			if to.File == 6 && b.CastleRights.WhiteKingSide {
				return b.canCastle(from, to, isWhite)
			}
			if to.File == 2 && b.CastleRights.WhiteQueenSide {
				return b.canCastle(from, to, isWhite)
			}
		} else if !isWhite && from.Rank == 7 {
			if to.File == 6 && b.CastleRights.BlackKingSide {
				return b.canCastle(from, to, isWhite)
			}
			if to.File == 2 && b.CastleRights.BlackQueenSide {
				return b.canCastle(from, to, isWhite)
			}
		}
	}

	return false
}

func (b *Board) canCastle(from, to Position, isWhite bool) bool {
	// Check if king is in check
	if b.IsInCheck(isWhite) {
		return false
	}

	// Check if path is clear and not attacked
	direction := 1
	if to.File < from.File {
		direction = -1
	}

	for f := from.File + direction; f != to.File; f += direction {
		pos := Position{File: f, Rank: from.Rank}
		if b.GetPiece(pos) != 0 {
			return false
		}
		// Check if square is attacked
		tempBoard := b.Copy()
		tempBoard.Squares[from.Rank][from.File] = 0
		tempBoard.Squares[pos.Rank][pos.File] = b.GetPiece(from)
		if tempBoard.IsInCheck(isWhite) {
			return false
		}
	}

	// Check rook square
	rookFile := 7
	if direction == -1 {
		rookFile = 0
	}
	expectedRook := 'R'
	if !isWhite {
		expectedRook = 'r'
	}
	if b.Squares[from.Rank][rookFile] != expectedRook {
		return false
	}

	return true
}

func (b *Board) isPathClear(from, to Position) bool {
	fileDir := sign(to.File - from.File)
	rankDir := sign(to.Rank - from.Rank)

	f, r := from.File+fileDir, from.Rank+rankDir
	for f != to.File || r != to.Rank {
		if b.Squares[r][f] != 0 {
			return false
		}
		f += fileDir
		r += rankDir
	}
	return true
}

// IsInCheck returns true if the specified player's king is in check
func (b *Board) IsInCheck(isWhite bool) bool {
	// Find king position
	kingPiece := 'K'
	if !isWhite {
		kingPiece = 'k'
	}

	var kingPos Position
	found := false
	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			if b.Squares[r][f] == kingPiece {
				kingPos = Position{File: f, Rank: r}
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return false
	}

	// Check if any enemy piece can attack the king
	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			piece := b.Squares[r][f]
			if piece == 0 {
				continue
			}
			if IsWhitePiece(piece) == isWhite {
				continue
			}

			from := Position{File: f, Rank: r}
			if b.canAttack(from, kingPos, piece) {
				return true
			}
		}
	}

	return false
}

func (b *Board) canAttack(from, to Position, piece rune) bool {
	pieceType := unicode.ToUpper(piece)
	isWhite := IsWhitePiece(piece)

	switch pieceType {
	case Pawn:
		direction := 1
		if !isWhite {
			direction = -1
		}
		fileDiff := abs(to.File - from.File)
		rankDiff := to.Rank - from.Rank
		return fileDiff == 1 && rankDiff == direction
	case Knight:
		return b.isValidKnightMove(from, to)
	case Bishop:
		return b.isValidBishopMove(from, to)
	case Rook:
		return b.isValidRookMove(from, to)
	case Queen:
		return b.isValidQueenMove(from, to)
	case King:
		fileDiff := abs(to.File - from.File)
		rankDiff := abs(to.Rank - from.Rank)
		return fileDiff <= 1 && rankDiff <= 1
	}
	return false
}

func (b *Board) Copy() *Board {
	newBoard := &Board{
		WhiteToMove:     b.WhiteToMove,
		EnPassantSquare: b.EnPassantSquare,
		HalfMoveClock:   b.HalfMoveClock,
		FullMoveNumber:  b.FullMoveNumber,
	}
	newBoard.CastleRights = b.CastleRights
	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			newBoard.Squares[r][f] = b.Squares[r][f]
		}
	}
	return newBoard
}

func (b *Board) makeMove(from, to Position) {
	piece := b.Squares[from.Rank][from.File]
	b.Squares[to.Rank][to.File] = piece
	b.Squares[from.Rank][from.File] = 0
}

// MakeMove applies a move and returns the new board state
func (b *Board) MakeMove(from, to Position, promotion rune) *Board {
	newBoard := b.Copy()
	piece := newBoard.Squares[from.Rank][from.File]
	pieceType := unicode.ToUpper(piece)
	isWhite := IsWhitePiece(piece)

	// Handle en passant capture
	if pieceType == Pawn && to.String() == b.EnPassantSquare {
		captureRank := to.Rank
		if isWhite {
			captureRank--
		} else {
			captureRank++
		}
		newBoard.Squares[captureRank][to.File] = 0
	}

	// Handle castling
	if pieceType == King && abs(to.File-from.File) == 2 {
		if to.File == 6 { // King-side
			newBoard.Squares[from.Rank][5] = newBoard.Squares[from.Rank][7]
			newBoard.Squares[from.Rank][7] = 0
		} else { // Queen-side
			newBoard.Squares[from.Rank][3] = newBoard.Squares[from.Rank][0]
			newBoard.Squares[from.Rank][0] = 0
		}
	}

	// Make the move
	newBoard.Squares[to.Rank][to.File] = piece
	newBoard.Squares[from.Rank][from.File] = 0

	// Handle pawn promotion
	if pieceType == Pawn && (to.Rank == 7 || to.Rank == 0) {
		if promotion != 0 {
			if isWhite {
				newBoard.Squares[to.Rank][to.File] = unicode.ToUpper(promotion)
			} else {
				newBoard.Squares[to.Rank][to.File] = unicode.ToLower(promotion)
			}
		} else {
			// Default to queen
			if isWhite {
				newBoard.Squares[to.Rank][to.File] = 'Q'
			} else {
				newBoard.Squares[to.Rank][to.File] = 'q'
			}
		}
	}

	// Update en passant square
	newBoard.EnPassantSquare = ""
	if pieceType == Pawn && abs(to.Rank-from.Rank) == 2 {
		epRank := (from.Rank + to.Rank) / 2
		newBoard.EnPassantSquare = fmt.Sprintf("%c%c", 'a'+to.File, '1'+epRank)
	}

	// Update castling rights
	if pieceType == King {
		if isWhite {
			newBoard.CastleRights.WhiteKingSide = false
			newBoard.CastleRights.WhiteQueenSide = false
		} else {
			newBoard.CastleRights.BlackKingSide = false
			newBoard.CastleRights.BlackQueenSide = false
		}
	}
	if pieceType == Rook {
		if from.File == 0 && from.Rank == 0 {
			newBoard.CastleRights.WhiteQueenSide = false
		} else if from.File == 7 && from.Rank == 0 {
			newBoard.CastleRights.WhiteKingSide = false
		} else if from.File == 0 && from.Rank == 7 {
			newBoard.CastleRights.BlackQueenSide = false
		} else if from.File == 7 && from.Rank == 7 {
			newBoard.CastleRights.BlackKingSide = false
		}
	}

	// Update clocks
	if pieceType == Pawn || b.GetPiece(to) != 0 {
		newBoard.HalfMoveClock = 0
	} else {
		newBoard.HalfMoveClock++
	}
	if !isWhite {
		newBoard.FullMoveNumber++
	}

	newBoard.WhiteToMove = !b.WhiteToMove

	return newBoard
}

// IsCheckmate returns true if the current player is in checkmate
func (b *Board) IsCheckmate() bool {
	isWhite := b.WhiteToMove
	if !b.IsInCheck(isWhite) {
		return false
	}
	return !b.hasLegalMoves(isWhite)
}

// IsStalemate returns true if the current player is in stalemate
func (b *Board) IsStalemate() bool {
	isWhite := b.WhiteToMove
	if b.IsInCheck(isWhite) {
		return false
	}
	return !b.hasLegalMoves(isWhite)
}

func (b *Board) hasLegalMoves(isWhite bool) bool {
	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			piece := b.Squares[r][f]
			if piece == 0 {
				continue
			}
			if IsWhitePiece(piece) != isWhite {
				continue
			}

			from := Position{File: f, Rank: r}
			for tr := 0; tr < 8; tr++ {
				for tf := 0; tf < 8; tf++ {
					to := Position{File: tf, Rank: tr}
					if b.ValidateMove(from, to) == nil {
						return true
					}
				}
			}
		}
	}
	return false
}

// GenerateNotation generates standard algebraic notation for a move
func (b *Board) GenerateNotation(from, to Position, promotion rune) string {
	piece := b.GetPiece(from)
	pieceType := unicode.ToUpper(piece)
	isCapture := b.GetPiece(to) != 0

	// Handle en passant
	if pieceType == Pawn && to.String() == b.EnPassantSquare {
		isCapture = true
	}

	var notation strings.Builder

	// Castling
	if pieceType == King && abs(to.File-from.File) == 2 {
		if to.File == 6 {
			return "O-O"
		}
		return "O-O-O"
	}

	// Piece letter (not for pawns)
	if pieceType != Pawn {
		notation.WriteRune(pieceType)
	}

	// Disambiguation for pieces that could move to the same square
	if pieceType != Pawn && pieceType != King {
		needFile, needRank := b.needsDisambiguation(from, to, piece)
		if needFile {
			notation.WriteByte(byte('a' + from.File))
		}
		if needRank {
			notation.WriteByte(byte('1' + from.Rank))
		}
	}

	// Pawn captures include file
	if pieceType == Pawn && isCapture {
		notation.WriteByte(byte('a' + from.File))
	}

	// Capture symbol
	if isCapture {
		notation.WriteByte('x')
	}

	// Destination square
	notation.WriteString(to.String())

	// Promotion
	if pieceType == Pawn && (to.Rank == 7 || to.Rank == 0) {
		notation.WriteByte('=')
		if promotion != 0 {
			notation.WriteRune(unicode.ToUpper(promotion))
		} else {
			notation.WriteByte('Q')
		}
	}

	// Check/Checkmate
	newBoard := b.MakeMove(from, to, promotion)
	if newBoard.IsCheckmate() {
		notation.WriteByte('#')
	} else if newBoard.IsInCheck(!b.WhiteToMove) {
		notation.WriteByte('+')
	}

	return notation.String()
}

func (b *Board) needsDisambiguation(from, to Position, piece rune) (needFile, needRank bool) {
	pieceType := unicode.ToUpper(piece)
	isWhite := IsWhitePiece(piece)

	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			if f == from.File && r == from.Rank {
				continue
			}
			otherPiece := b.Squares[r][f]
			if unicode.ToUpper(otherPiece) != pieceType {
				continue
			}
			if IsWhitePiece(otherPiece) != isWhite {
				continue
			}

			otherFrom := Position{File: f, Rank: r}
			if b.ValidateMove(otherFrom, to) == nil {
				if f != from.File {
					needFile = true
				} else {
					needRank = true
				}
			}
		}
	}
	return
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	if x < 0 {
		return -1
	}
	if x > 0 {
		return 1
	}
	return 0
}
