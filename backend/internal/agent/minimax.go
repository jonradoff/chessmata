package agent

import (
	"chess-game/internal/game"
	"crypto/rand"
	"math/big"
)

const (
	infinity  = 999999
	mateScore = 100000
)

// randomnessThreshold is the centipawn window within which moves are considered
// equally good. This adds variety to openings without sacrificing quality.
const randomnessThreshold = 30

// BestMove finds the best move using 2-ply minimax with alpha-beta pruning.
// Among moves within randomnessThreshold of the best score, one is chosen at random.
func BestMove(board *game.Board) *Move {
	moves := GenerateLegalMoves(board)
	if len(moves) == 0 {
		return nil
	}

	isWhite := board.WhiteToMove
	bestScore := -infinity

	type scoredMove struct {
		move  Move
		score int
	}
	scored := make([]scoredMove, 0, len(moves))

	alpha := -infinity
	beta := infinity

	for i := range moves {
		m := moves[i]
		newBoard := board.MakeMove(m.From, m.To, m.Promotion)

		// Minimax from opponent's perspective (depth 1 remaining)
		score := -alphaBeta(newBoard, 1, -beta, -alpha, !isWhite)

		scored = append(scored, scoredMove{move: m, score: score})

		if score > bestScore {
			bestScore = score
		}
		if score > alpha {
			alpha = score
		}
	}

	// Collect all moves within the randomness threshold of the best
	var candidates []Move
	for _, sm := range scored {
		if sm.score >= bestScore-randomnessThreshold {
			candidates = append(candidates, sm.move)
		}
	}

	if len(candidates) == 0 {
		result := scored[0].move
		return &result
	}

	// Pick a random candidate using crypto/rand
	idx := 0
	if len(candidates) > 1 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err == nil {
			idx = int(n.Int64())
		}
	}

	return &candidates[idx]
}

// alphaBeta performs alpha-beta search.
// maximizingWhite indicates we evaluate from white's perspective when true.
func alphaBeta(board *game.Board, depth int, alpha, beta int, maximizingWhite bool) int {
	// Terminal check
	if board.IsCheckmate() {
		return -mateScore - depth // Current side to move is mated (bad)
	}
	if board.IsStalemate() {
		return 0
	}

	if depth == 0 {
		eval := Evaluate(board)
		if !maximizingWhite {
			eval = -eval
		}
		return eval
	}

	moves := GenerateLegalMoves(board)
	if len(moves) == 0 {
		// No legal moves but not checkmate/stalemate should not happen,
		// but handle gracefully
		eval := Evaluate(board)
		if !maximizingWhite {
			eval = -eval
		}
		return eval
	}

	best := -infinity
	for _, m := range moves {
		newBoard := board.MakeMove(m.From, m.To, m.Promotion)
		score := -alphaBeta(newBoard, depth-1, -beta, -alpha, !maximizingWhite)

		if score > best {
			best = score
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break // beta cutoff
		}
	}

	return best
}
