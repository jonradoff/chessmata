import type { PieceState, PieceType } from '../hooks/useGameState'

/**
 * Parses a FEN string and returns an array of pieces
 * FEN format: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
 */
export function parseFEN(fen: string): PieceState[] {
  const pieces: PieceState[] = []

  // Split FEN into parts (we only need the board layout part)
  const parts = fen.split(' ')
  const boardLayout = parts[0]

  // Split by ranks (rows), starting from rank 8 (top) to rank 1 (bottom)
  const ranks = boardLayout.split('/')

  let pieceId = 0

  ranks.forEach((rankStr, rankIndex) => {
    // Rank 0 in FEN is rank 7 in our coordinate system (top of board)
    // Rank 7 in FEN is rank 0 in our coordinate system (bottom of board)
    const rank = 7 - rankIndex

    let file = 0
    for (const char of rankStr) {
      if (char >= '1' && char <= '8') {
        // Number indicates empty squares
        file += parseInt(char)
      } else {
        // Letter indicates a piece
        const isWhite = char === char.toUpperCase()
        const pieceChar = char.toLowerCase()

        let type: PieceType
        switch (pieceChar) {
          case 'p':
            type = 'pawn'
            break
          case 'r':
            type = 'rook'
            break
          case 'n':
            type = 'knight'
            break
          case 'b':
            type = 'bishop'
            break
          case 'q':
            type = 'queen'
            break
          case 'k':
            type = 'king'
            break
          default:
            continue
        }

        pieces.push({
          id: `piece-${pieceId++}`,
          type,
          isWhite,
          file,
          rank,
        })

        file++
      }
    }
  })

  return pieces
}

/**
 * Converts pieces array back to FEN notation (board layout only)
 */
export function piecesToFEN(pieces: PieceState[]): string {
  const board: (string | null)[][] = Array(8).fill(null).map(() => Array(8).fill(null))

  // Place pieces on board
  pieces.forEach(piece => {
    const char = piece.type[0]
    const pieceChar = piece.isWhite ? char.toUpperCase() : char.toLowerCase()
    board[7 - piece.rank][piece.file] = pieceChar
  })

  // Convert to FEN string
  const ranks: string[] = []
  for (let rank = 0; rank < 8; rank++) {
    let rankStr = ''
    let emptyCount = 0

    for (let file = 0; file < 8; file++) {
      const square = board[rank][file]
      if (square === null) {
        emptyCount++
      } else {
        if (emptyCount > 0) {
          rankStr += emptyCount
          emptyCount = 0
        }
        rankStr += square
      }
    }

    if (emptyCount > 0) {
      rankStr += emptyCount
    }

    ranks.push(rankStr)
  }

  return ranks.join('/')
}
