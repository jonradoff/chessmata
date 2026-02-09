// Chess rules validation for the frontend

export type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn'

export interface Position {
  file: number // 0-7 (a-h)
  rank: number // 0-7 (1-8)
}

export interface Piece {
  type: PieceType
  isWhite: boolean
  file: number
  rank: number
}

export interface Board {
  pieces: Piece[]
  whiteToMove: boolean
  castlingRights: {
    whiteKingside: boolean
    whiteQueenside: boolean
    blackKingside: boolean
    blackQueenside: boolean
  }
  enPassantSquare: Position | null
}

// Get piece at position
function getPieceAt(board: Board, file: number, rank: number): Piece | undefined {
  return board.pieces.find(p => p.file === file && p.rank === rank)
}

// Check if position is on the board
function isOnBoard(file: number, rank: number): boolean {
  return file >= 0 && file < 8 && rank >= 0 && rank < 8
}

// Check if path is clear (for sliding pieces)
function isPathClear(board: Board, from: Position, to: Position): boolean {
  const dx = Math.sign(to.file - from.file)
  const dy = Math.sign(to.rank - from.rank)

  let x = from.file + dx
  let y = from.rank + dy

  while (x !== to.file || y !== to.rank) {
    if (getPieceAt(board, x, y)) {
      return false
    }
    x += dx
    y += dy
  }

  return true
}

// Validate pawn move
function isValidPawnMove(board: Board, piece: Piece, to: Position): boolean {
  const direction = piece.isWhite ? 1 : -1
  const startRank = piece.isWhite ? 1 : 6
  const fileDiff = Math.abs(to.file - piece.file)
  const rankDiff = to.rank - piece.rank

  // Forward move
  if (fileDiff === 0) {
    // Single step
    if (rankDiff === direction) {
      return !getPieceAt(board, to.file, to.rank)
    }
    // Double step from starting position
    if (piece.rank === startRank && rankDiff === 2 * direction) {
      const midRank = piece.rank + direction
      return !getPieceAt(board, to.file, midRank) && !getPieceAt(board, to.file, to.rank)
    }
    return false
  }

  // Capture (diagonal)
  if (fileDiff === 1 && rankDiff === direction) {
    const targetPiece = getPieceAt(board, to.file, to.rank)
    // Regular capture
    if (targetPiece && targetPiece.isWhite !== piece.isWhite) {
      return true
    }
    // En passant
    if (board.enPassantSquare &&
        board.enPassantSquare.file === to.file &&
        board.enPassantSquare.rank === to.rank) {
      return true
    }
    return false
  }

  return false
}

// Validate knight move
function isValidKnightMove(piece: Piece, to: Position): boolean {
  const fileDiff = Math.abs(to.file - piece.file)
  const rankDiff = Math.abs(to.rank - piece.rank)
  return (fileDiff === 2 && rankDiff === 1) || (fileDiff === 1 && rankDiff === 2)
}

// Validate bishop move
function isValidBishopMove(board: Board, piece: Piece, to: Position): boolean {
  const fileDiff = Math.abs(to.file - piece.file)
  const rankDiff = Math.abs(to.rank - piece.rank)

  if (fileDiff !== rankDiff || fileDiff === 0) {
    return false
  }

  return isPathClear(board, piece, to)
}

// Validate rook move
function isValidRookMove(board: Board, piece: Piece, to: Position): boolean {
  const fileDiff = to.file - piece.file
  const rankDiff = to.rank - piece.rank

  if ((fileDiff !== 0 && rankDiff !== 0) || (fileDiff === 0 && rankDiff === 0)) {
    return false
  }

  return isPathClear(board, piece, to)
}

// Validate queen move
function isValidQueenMove(board: Board, piece: Piece, to: Position): boolean {
  return isValidBishopMove(board, piece, to) || isValidRookMove(board, piece, to)
}

// Validate king move (excluding castling check for now)
function isValidKingMove(piece: Piece, to: Position): boolean {
  const fileDiff = Math.abs(to.file - piece.file)
  const rankDiff = Math.abs(to.rank - piece.rank)
  return fileDiff <= 1 && rankDiff <= 1 && (fileDiff > 0 || rankDiff > 0)
}

// Find the king of the given color
function findKing(board: Board, isWhite: boolean): Piece | undefined {
  return board.pieces.find(p => p.type === 'king' && p.isWhite === isWhite)
}

// Check if a square is under attack by the given color
function isSquareUnderAttack(board: Board, file: number, rank: number, byWhite: boolean): boolean {
  // Check all pieces of the attacking color
  for (const piece of board.pieces) {
    if (piece.isWhite !== byWhite) continue

    // Create a temporary board state for the attacking color's turn
    const tempBoard = { ...board, whiteToMove: byWhite }

    // Special handling for pawns - they attack diagonally
    if (piece.type === 'pawn') {
      const direction = piece.isWhite ? 1 : -1
      const fileDiff = Math.abs(file - piece.file)
      const rankDiff = rank - piece.rank
      if (fileDiff === 1 && rankDiff === direction) {
        return true
      }
      continue
    }

    // For other pieces, check if they can move to this square
    const canAttack = (() => {
      switch (piece.type) {
        case 'knight':
          return isValidKnightMove(piece, { file, rank })
        case 'bishop':
          return isValidBishopMove(tempBoard, piece, { file, rank })
        case 'rook':
          return isValidRookMove(tempBoard, piece, { file, rank })
        case 'queen':
          return isValidQueenMove(tempBoard, piece, { file, rank })
        case 'king':
          return isValidKingMove(piece, { file, rank })
        default:
          return false
      }
    })()

    if (canAttack) return true
  }

  return false
}

// Check if the given color's king is in check
export function isInCheck(board: Board, isWhite: boolean): boolean {
  const king = findKing(board, isWhite)
  if (!king) return false

  return isSquareUnderAttack(board, king.file, king.rank, !isWhite)
}

// Make a move on the board and return the new board state
function makeMove(board: Board, from: Position, to: Position): Board {
  const newPieces = board.pieces
    .filter(p => !(p.file === to.file && p.rank === to.rank)) // Remove captured piece
    .map(p => {
      if (p.file === from.file && p.rank === from.rank) {
        return { ...p, file: to.file, rank: to.rank }
      }
      return p
    })

  return {
    ...board,
    pieces: newPieces,
    whiteToMove: !board.whiteToMove
  }
}

// Check if a move would leave the king in check
function wouldLeaveKingInCheck(board: Board, from: Position, to: Position): boolean {
  const piece = getPieceAt(board, from.file, from.rank)
  if (!piece) return false

  const newBoard = makeMove(board, from, to)
  return isInCheck(newBoard, piece.isWhite)
}

// Get all legal moves for a piece (excluding moves that would leave king in check)
function getLegalMovesForPiece(board: Board, piece: Piece): Position[] {
  const moves: Position[] = []

  for (let file = 0; file < 8; file++) {
    for (let rank = 0; rank < 8; rank++) {
      // First check basic move validity
      const basicCheck = (() => {
        switch (piece.type) {
          case 'pawn':
            return isValidPawnMove(board, piece, { file, rank })
          case 'knight':
            return isValidKnightMove(piece, { file, rank })
          case 'bishop':
            return isValidBishopMove(board, piece, { file, rank })
          case 'rook':
            return isValidRookMove(board, piece, { file, rank })
          case 'queen':
            return isValidQueenMove(board, piece, { file, rank })
          case 'king':
            return isValidKingMove(piece, { file, rank })
          default:
            return false
        }
      })()

      if (!basicCheck) continue

      // Check if destination has own piece
      const destPiece = getPieceAt(board, file, rank)
      if (destPiece && destPiece.isWhite === piece.isWhite) continue

      // Check if this move would leave king in check
      if (!wouldLeaveKingInCheck(board, piece, { file, rank })) {
        moves.push({ file, rank })
      }
    }
  }

  return moves
}

// Check if the current player has any legal moves
function hasLegalMoves(board: Board, isWhite: boolean): boolean {
  for (const piece of board.pieces) {
    if (piece.isWhite !== isWhite) continue
    const legalMoves = getLegalMovesForPiece(board, piece)
    if (legalMoves.length > 0) return true
  }
  return false
}

// Check if it's checkmate
export function isCheckmate(board: Board, isWhite: boolean): boolean {
  return isInCheck(board, isWhite) && !hasLegalMoves(board, isWhite)
}

// Check if it's stalemate
export function isStalemate(board: Board, isWhite: boolean): boolean {
  return !isInCheck(board, isWhite) && !hasLegalMoves(board, isWhite)
}

export interface MoveValidation {
  valid: boolean
  reason?: string
  pieceType?: PieceType
}

// Check if a move is valid based on piece type
export function isValidMove(board: Board, from: Position, to: Position): MoveValidation {
  const piece = getPieceAt(board, from.file, from.rank)

  if (!piece) {
    return { valid: false, reason: 'No piece at source position' }
  }

  // Check if it's this player's turn
  if (piece.isWhite !== board.whiteToMove) {
    return { valid: false, reason: 'Not your turn' }
  }

  // Check if destination is on board
  if (!isOnBoard(to.file, to.rank)) {
    return { valid: false, reason: 'Destination off board' }
  }

  // Check if destination has own piece
  const destPiece = getPieceAt(board, to.file, to.rank)
  if (destPiece && destPiece.isWhite === piece.isWhite) {
    return { valid: false, reason: 'Cannot capture own piece' }
  }

  // Validate based on piece type
  let isValid = false

  switch (piece.type) {
    case 'pawn':
      isValid = isValidPawnMove(board, piece, to)
      break
    case 'knight':
      isValid = isValidKnightMove(piece, to)
      break
    case 'bishop':
      isValid = isValidBishopMove(board, piece, to)
      // Check for blocking specifically for bishop
      if (!isValid) {
        const fileDiff = Math.abs(to.file - piece.file)
        const rankDiff = Math.abs(to.rank - piece.rank)
        if (fileDiff === rankDiff && fileDiff > 0 && !isPathClear(board, piece, to)) {
          return { valid: false, reason: 'Path is blocked', pieceType: piece.type }
        }
      }
      break
    case 'rook':
      isValid = isValidRookMove(board, piece, to)
      // Check for blocking specifically for rook
      if (!isValid) {
        const fileDiff = to.file - piece.file
        const rankDiff = to.rank - piece.rank
        if ((fileDiff === 0 || rankDiff === 0) && (fileDiff !== 0 || rankDiff !== 0) && !isPathClear(board, piece, to)) {
          return { valid: false, reason: 'Path is blocked', pieceType: piece.type }
        }
      }
      break
    case 'queen':
      isValid = isValidQueenMove(board, piece, to)
      // Check for blocking specifically for queen
      if (!isValid) {
        const fileDiff = to.file - piece.file
        const rankDiff = to.rank - piece.rank
        const fileDiffAbs = Math.abs(fileDiff)
        const rankDiffAbs = Math.abs(rankDiff)
        // Check if it's a valid queen direction but path is blocked
        if (((fileDiff === 0 || rankDiff === 0) || (fileDiffAbs === rankDiffAbs)) &&
            (fileDiffAbs > 0 || rankDiffAbs > 0) &&
            !isPathClear(board, piece, to)) {
          return { valid: false, reason: 'Path is blocked', pieceType: piece.type }
        }
      }
      break
    case 'king':
      isValid = isValidKingMove(piece, to)
      break
  }

  if (!isValid) {
    return { valid: false, reason: `Invalid ${piece.type} move`, pieceType: piece.type }
  }

  // Check if this move would leave the king in check
  if (wouldLeaveKingInCheck(board, from, to)) {
    if (isInCheck(board, piece.isWhite)) {
      return { valid: false, reason: 'King is in check', pieceType: piece.type }
    } else {
      return { valid: false, reason: 'Move would expose king to check', pieceType: piece.type }
    }
  }

  return { valid: true }
}

// Get all valid moves for a piece
export function getValidMoves(board: Board, piece: Piece): Position[] {
  const validMoves: Position[] = []

  for (let file = 0; file < 8; file++) {
    for (let rank = 0; rank < 8; rank++) {
      const result = isValidMove(board, piece, { file, rank })
      if (result.valid) {
        validMoves.push({ file, rank })
      }
    }
  }

  return validMoves
}

// Convert file/rank to algebraic notation
export function toAlgebraic(file: number, rank: number): string {
  return String.fromCharCode(97 + file) + (rank + 1)
}

// Convert algebraic notation to file/rank
export function fromAlgebraic(notation: string): Position {
  return {
    file: notation.charCodeAt(0) - 97,
    rank: parseInt(notation[1]) - 1
  }
}

// Check if the player can select this piece (is it their color and their turn?)
export function canSelectPiece(
  piece: Piece,
  playerColor: 'white' | 'black' | null,
  currentTurn: 'white' | 'black' | null
): boolean {
  if (!playerColor || !currentTurn) {
    return true // Allow selection in local game mode
  }

  const pieceColor = piece.isWhite ? 'white' : 'black'

  // Must be your piece
  if (pieceColor !== playerColor) {
    return false
  }

  // Must be your turn
  if (currentTurn !== playerColor) {
    return false
  }

  return true
}
