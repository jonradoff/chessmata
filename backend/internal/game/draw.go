package game

import (
	"strings"
	"unicode"
)

// DrawReason represents the reason for a draw
type DrawReason string

const (
	DrawByAgreement           DrawReason = "agreement"
	DrawByStalemate           DrawReason = "stalemate"
	DrawByThreefoldRepetition DrawReason = "threefold_repetition"
	DrawByFivefoldRepetition  DrawReason = "fivefold_repetition"
	DrawByFiftyMoves          DrawReason = "fifty_moves"
	DrawBySeventyFiveMoves    DrawReason = "seventy_five_moves"
	DrawByInsufficientMaterial DrawReason = "insufficient_material"
)

// IsInsufficientMaterial checks if neither player can checkmate (FIDE rules)
// Returns true for:
// - King vs King
// - King + Bishop vs King
// - King + Knight vs King
// - King + Bishop vs King + Bishop (same color squares)
func IsInsufficientMaterial(board *Board) bool {
	var whitePieces, blackPieces []rune
	var whiteBishopSquares, blackBishopSquares []bool // true = light square, false = dark square

	for r := 0; r < 8; r++ {
		for f := 0; f < 8; f++ {
			piece := board.Squares[r][f]
			if piece == 0 {
				continue
			}

			pieceType := unicode.ToUpper(piece)
			isLightSquare := (r+f)%2 == 1 // Light squares have odd sum

			if IsWhitePiece(piece) {
				whitePieces = append(whitePieces, pieceType)
				if pieceType == Bishop {
					whiteBishopSquares = append(whiteBishopSquares, isLightSquare)
				}
			} else {
				blackPieces = append(blackPieces, pieceType)
				if pieceType == Bishop {
					blackBishopSquares = append(blackBishopSquares, isLightSquare)
				}
			}
		}
	}

	// Remove kings from consideration
	whitePieces = removePiece(whitePieces, King)
	blackPieces = removePiece(blackPieces, King)

	// King vs King
	if len(whitePieces) == 0 && len(blackPieces) == 0 {
		return true
	}

	// King + minor piece vs King
	if len(whitePieces) == 0 && len(blackPieces) == 1 {
		return blackPieces[0] == Bishop || blackPieces[0] == Knight
	}
	if len(blackPieces) == 0 && len(whitePieces) == 1 {
		return whitePieces[0] == Bishop || whitePieces[0] == Knight
	}

	// King + Bishop vs King + Bishop (same color squares)
	if len(whitePieces) == 1 && len(blackPieces) == 1 {
		if whitePieces[0] == Bishop && blackPieces[0] == Bishop {
			// Both bishops on same color squares
			if len(whiteBishopSquares) > 0 && len(blackBishopSquares) > 0 {
				return whiteBishopSquares[0] == blackBishopSquares[0]
			}
		}
	}

	return false
}

func removePiece(pieces []rune, toRemove rune) []rune {
	result := make([]rune, 0, len(pieces))
	for _, p := range pieces {
		if p != toRemove {
			result = append(result, p)
		}
	}
	return result
}

// GetPositionKey extracts the position-relevant parts of FEN for repetition detection
// This includes: piece placement, active color, castling rights, en passant square
// It excludes: halfmove clock and fullmove number
func GetPositionKey(fen string) string {
	parts := strings.Split(fen, " ")
	if len(parts) < 4 {
		return fen
	}
	// Include pieces, turn, castling, and en passant
	return parts[0] + " " + parts[1] + " " + parts[2] + " " + parts[3]
}

// CountPositionRepetitions counts how many times a position has occurred
func CountPositionRepetitions(positionHistory []string, currentFEN string) int {
	currentKey := GetPositionKey(currentFEN)
	count := 0
	for _, pos := range positionHistory {
		if GetPositionKey(pos) == currentKey {
			count++
		}
	}
	return count
}

// IsThreefoldRepetition checks if current position has occurred 3+ times
func IsThreefoldRepetition(positionHistory []string, currentFEN string) bool {
	return CountPositionRepetitions(positionHistory, currentFEN) >= 3
}

// IsFivefoldRepetition checks if current position has occurred 5+ times (auto-draw)
func IsFivefoldRepetition(positionHistory []string, currentFEN string) bool {
	return CountPositionRepetitions(positionHistory, currentFEN) >= 5
}

// IsFiftyMoveRule checks if 50 moves have been made without pawn move or capture
// In FEN, halfmove clock counts half-moves (plies), so 100 = 50 full moves
func IsFiftyMoveRule(halfMoveClock int) bool {
	return halfMoveClock >= 100
}

// IsSeventyFiveMoveRule checks for automatic draw at 75 moves
// FIDE rule: game is drawn if 75 moves without capture or pawn move
func IsSeventyFiveMoveRule(halfMoveClock int) bool {
	return halfMoveClock >= 150
}

// CanClaimDraw checks if a player can claim a draw (threefold or 50-move)
func CanClaimDraw(positionHistory []string, currentFEN string, halfMoveClock int) (bool, DrawReason) {
	if IsThreefoldRepetition(positionHistory, currentFEN) {
		return true, DrawByThreefoldRepetition
	}
	if IsFiftyMoveRule(halfMoveClock) {
		return true, DrawByFiftyMoves
	}
	return false, ""
}

// IsAutomaticDraw checks for conditions that automatically end the game as a draw
func IsAutomaticDraw(board *Board, positionHistory []string, currentFEN string) (bool, DrawReason) {
	// Fivefold repetition (automatic)
	if IsFivefoldRepetition(positionHistory, currentFEN) {
		return true, DrawByFivefoldRepetition
	}

	// 75-move rule (automatic)
	if IsSeventyFiveMoveRule(board.HalfMoveClock) {
		return true, DrawBySeventyFiveMoves
	}

	// Insufficient material
	if IsInsufficientMaterial(board) {
		return true, DrawByInsufficientMaterial
	}

	// Stalemate
	if board.IsStalemate() {
		return true, DrawByStalemate
	}

	return false, ""
}

// GetDrawReasonDisplay returns a human-readable description of the draw reason
func (r DrawReason) GetDisplayText() string {
	switch r {
	case DrawByAgreement:
		return "Draw by agreement"
	case DrawByStalemate:
		return "Draw by stalemate"
	case DrawByThreefoldRepetition:
		return "Draw by threefold repetition"
	case DrawByFivefoldRepetition:
		return "Draw by fivefold repetition (automatic)"
	case DrawByFiftyMoves:
		return "Draw by 50-move rule"
	case DrawBySeventyFiveMoves:
		return "Draw by 75-move rule (automatic)"
	case DrawByInsufficientMaterial:
		return "Draw by insufficient material"
	default:
		return "Draw"
	}
}
