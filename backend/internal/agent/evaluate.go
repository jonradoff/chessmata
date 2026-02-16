package agent

import (
	"chess-game/internal/game"
	"unicode"
)

// Material values in centipawns
const (
	pawnValue   = 100
	knightValue = 320
	bishopValue = 330
	rookValue   = 500
	queenValue  = 900
	kingValue   = 20000
)

// Piece-square tables (from white's perspective, index [rank][file])
// Values are bonuses/penalties in centipawns added to material value.

var pawnTable = [8][8]int{
	{0, 0, 0, 0, 0, 0, 0, 0},
	{5, 10, 10, -20, -20, 10, 10, 5},
	{5, -5, -10, 0, 0, -10, -5, 5},
	{0, 0, 0, 20, 20, 0, 0, 0},
	{5, 5, 10, 25, 25, 10, 5, 5},
	{10, 10, 20, 30, 30, 20, 10, 10},
	{50, 50, 50, 50, 50, 50, 50, 50},
	{0, 0, 0, 0, 0, 0, 0, 0},
}

var knightTable = [8][8]int{
	{-50, -40, -30, -30, -30, -30, -40, -50},
	{-40, -20, 0, 5, 5, 0, -20, -40},
	{-30, 5, 10, 15, 15, 10, 5, -30},
	{-30, 0, 15, 20, 20, 15, 0, -30},
	{-30, 5, 15, 20, 20, 15, 5, -30},
	{-30, 0, 10, 15, 15, 10, 0, -30},
	{-40, -20, 0, 0, 0, 0, -20, -40},
	{-50, -40, -30, -30, -30, -30, -40, -50},
}

var bishopTable = [8][8]int{
	{-20, -10, -10, -10, -10, -10, -10, -20},
	{-10, 5, 0, 0, 0, 0, 5, -10},
	{-10, 10, 10, 10, 10, 10, 10, -10},
	{-10, 0, 10, 10, 10, 10, 0, -10},
	{-10, 5, 5, 10, 10, 5, 5, -10},
	{-10, 0, 5, 10, 10, 5, 0, -10},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-20, -10, -10, -10, -10, -10, -10, -20},
}

var rookTable = [8][8]int{
	{0, 0, 0, 5, 5, 0, 0, 0},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{-5, 0, 0, 0, 0, 0, 0, -5},
	{5, 10, 10, 10, 10, 10, 10, 5},
	{0, 0, 0, 0, 0, 0, 0, 0},
}

var queenTable = [8][8]int{
	{-20, -10, -10, -5, -5, -10, -10, -20},
	{-10, 0, 5, 0, 0, 0, 0, -10},
	{-10, 5, 5, 5, 5, 5, 0, -10},
	{0, 0, 5, 5, 5, 5, 0, -5},
	{-5, 0, 5, 5, 5, 5, 0, -5},
	{-10, 0, 5, 5, 5, 5, 0, -10},
	{-10, 0, 0, 0, 0, 0, 0, -10},
	{-20, -10, -10, -5, -5, -10, -10, -20},
}

var kingMiddleTable = [8][8]int{
	{20, 30, 10, 0, 0, 10, 30, 20},
	{20, 20, 0, 0, 0, 0, 20, 20},
	{-10, -20, -20, -20, -20, -20, -20, -10},
	{-20, -30, -30, -40, -40, -30, -30, -20},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
	{-30, -40, -40, -50, -50, -40, -40, -30},
}

// Evaluate returns a score for the position in centipawns.
// Positive = white advantage, negative = black advantage.
func Evaluate(board *game.Board) int {
	score := 0

	for rank := 0; rank < 8; rank++ {
		for file := 0; file < 8; file++ {
			piece := board.Squares[rank][file]
			if piece == 0 {
				continue
			}

			isWhite := game.IsWhitePiece(piece)
			pieceType := unicode.ToUpper(piece)

			materialValue := 0
			positionalValue := 0

			switch pieceType {
			case game.Pawn:
				materialValue = pawnValue
				if isWhite {
					positionalValue = pawnTable[rank][file]
				} else {
					positionalValue = pawnTable[7-rank][file]
				}
			case game.Knight:
				materialValue = knightValue
				if isWhite {
					positionalValue = knightTable[rank][file]
				} else {
					positionalValue = knightTable[7-rank][file]
				}
			case game.Bishop:
				materialValue = bishopValue
				if isWhite {
					positionalValue = bishopTable[rank][file]
				} else {
					positionalValue = bishopTable[7-rank][file]
				}
			case game.Rook:
				materialValue = rookValue
				if isWhite {
					positionalValue = rookTable[rank][file]
				} else {
					positionalValue = rookTable[7-rank][file]
				}
			case game.Queen:
				materialValue = queenValue
				if isWhite {
					positionalValue = queenTable[rank][file]
				} else {
					positionalValue = queenTable[7-rank][file]
				}
			case game.King:
				materialValue = kingValue
				if isWhite {
					positionalValue = kingMiddleTable[rank][file]
				} else {
					positionalValue = kingMiddleTable[7-rank][file]
				}
			}

			total := materialValue + positionalValue
			if isWhite {
				score += total
			} else {
				score -= total
			}
		}
	}

	return score
}
