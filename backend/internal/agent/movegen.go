package agent

import (
	"chess-game/internal/game"
	"unicode"
)

// Move represents a chess move with from/to positions and optional promotion.
type Move struct {
	From      game.Position
	To        game.Position
	Promotion rune // 0 for no promotion
}

// GenerateLegalMoves returns all legal moves for the side to move.
func GenerateLegalMoves(board *game.Board) []Move {
	var moves []Move
	isWhite := board.WhiteToMove

	for rank := 0; rank < 8; rank++ {
		for file := 0; file < 8; file++ {
			piece := board.Squares[rank][file]
			if piece == 0 {
				continue
			}
			if game.IsWhitePiece(piece) != isWhite {
				continue
			}

			from := game.Position{File: file, Rank: rank}
			pieceType := unicode.ToUpper(piece)

			switch pieceType {
			case game.Pawn:
				moves = append(moves, generatePawnMoves(board, from, isWhite)...)
			case game.Knight:
				moves = append(moves, generateKnightMoves(board, from, isWhite)...)
			case game.Bishop:
				moves = append(moves, generateSlidingMoves(board, from, isWhite, bishopDirs)...)
			case game.Rook:
				moves = append(moves, generateSlidingMoves(board, from, isWhite, rookDirs)...)
			case game.Queen:
				moves = append(moves, generateSlidingMoves(board, from, isWhite, queenDirs)...)
			case game.King:
				moves = append(moves, generateKingMoves(board, from, isWhite)...)
			}
		}
	}

	return moves
}

var bishopDirs = [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
var rookDirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
var queenDirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
var knightOffsets = [][2]int{{2, 1}, {2, -1}, {-2, 1}, {-2, -1}, {1, 2}, {1, -2}, {-1, 2}, {-1, -2}}

func inBounds(file, rank int) bool {
	return file >= 0 && file < 8 && rank >= 0 && rank < 8
}

func isLegal(board *game.Board, from, to game.Position, promotion rune) bool {
	return board.ValidateMove(from, to) == nil
}

func generatePawnMoves(board *game.Board, from game.Position, isWhite bool) []Move {
	var moves []Move
	dir := 1
	startRank := 1
	promoRank := 7
	if !isWhite {
		dir = -1
		startRank = 6
		promoRank = 0
	}

	// Forward one
	to := game.Position{File: from.File, Rank: from.Rank + dir}
	if inBounds(to.File, to.Rank) && board.GetPiece(to) == 0 {
		if to.Rank == promoRank {
			for _, p := range []rune{game.Queen, game.Rook, game.Bishop, game.Knight} {
				if isLegal(board, from, to, p) {
					moves = append(moves, Move{From: from, To: to, Promotion: p})
				}
			}
		} else if isLegal(board, from, to, 0) {
			moves = append(moves, Move{From: from, To: to})
		}
	}

	// Forward two
	if from.Rank == startRank {
		to = game.Position{File: from.File, Rank: from.Rank + 2*dir}
		mid := game.Position{File: from.File, Rank: from.Rank + dir}
		if inBounds(to.File, to.Rank) && board.GetPiece(mid) == 0 && board.GetPiece(to) == 0 {
			if isLegal(board, from, to, 0) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}

	// Captures (including en passant)
	for _, df := range []int{-1, 1} {
		to = game.Position{File: from.File + df, Rank: from.Rank + dir}
		if !inBounds(to.File, to.Rank) {
			continue
		}
		dest := board.GetPiece(to)
		isCapture := dest != 0 && game.IsWhitePiece(dest) != isWhite
		isEP := to.String() == board.EnPassantSquare

		if isCapture || isEP {
			if to.Rank == promoRank {
				for _, p := range []rune{game.Queen, game.Rook, game.Bishop, game.Knight} {
					if isLegal(board, from, to, p) {
						moves = append(moves, Move{From: from, To: to, Promotion: p})
					}
				}
			} else if isLegal(board, from, to, 0) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}

	return moves
}

func generateKnightMoves(board *game.Board, from game.Position, isWhite bool) []Move {
	var moves []Move
	for _, off := range knightOffsets {
		to := game.Position{File: from.File + off[0], Rank: from.Rank + off[1]}
		if !inBounds(to.File, to.Rank) {
			continue
		}
		dest := board.GetPiece(to)
		if dest != 0 && game.IsWhitePiece(dest) == isWhite {
			continue
		}
		if isLegal(board, from, to, 0) {
			moves = append(moves, Move{From: from, To: to})
		}
	}
	return moves
}

func generateSlidingMoves(board *game.Board, from game.Position, isWhite bool, dirs [][2]int) []Move {
	var moves []Move
	for _, d := range dirs {
		for dist := 1; dist < 8; dist++ {
			to := game.Position{File: from.File + d[0]*dist, Rank: from.Rank + d[1]*dist}
			if !inBounds(to.File, to.Rank) {
				break
			}
			dest := board.GetPiece(to)
			if dest != 0 && game.IsWhitePiece(dest) == isWhite {
				break // own piece
			}
			if isLegal(board, from, to, 0) {
				moves = append(moves, Move{From: from, To: to})
			}
			if dest != 0 {
				break // captured enemy piece, stop sliding
			}
		}
	}
	return moves
}

func generateKingMoves(board *game.Board, from game.Position, isWhite bool) []Move {
	var moves []Move

	// Normal king moves
	for dr := -1; dr <= 1; dr++ {
		for df := -1; df <= 1; df++ {
			if dr == 0 && df == 0 {
				continue
			}
			to := game.Position{File: from.File + df, Rank: from.Rank + dr}
			if !inBounds(to.File, to.Rank) {
				continue
			}
			dest := board.GetPiece(to)
			if dest != 0 && game.IsWhitePiece(dest) == isWhite {
				continue
			}
			if isLegal(board, from, to, 0) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}

	// Castling moves (king moves 2 squares)
	for _, toFile := range []int{2, 6} {
		to := game.Position{File: toFile, Rank: from.Rank}
		if isLegal(board, from, to, 0) {
			moves = append(moves, Move{From: from, To: to})
		}
	}

	return moves
}
