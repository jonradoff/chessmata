import { useState, useCallback } from 'react'
import { parseFENState } from '../utils/fenParser'

export type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn'

export interface PieceState {
  id: string
  type: PieceType
  isWhite: boolean
  file: number // 0-7 (a-h)
  rank: number // 0-7 (1-8)
}

export interface MoveArrow {
  from: { file: number; rank: number }
  to: { file: number; rank: number }
}

export interface GameState {
  pieces: PieceState[]
  selectedPieceId: string | null
  hoverSquare: { file: number; rank: number } | null
  lastMoveArrow: MoveArrow | null
}

function createInitialPieces(): PieceState[] {
  const pieces: PieceState[] = []
  let id = 0

  // White back row
  const backRowTypes: PieceType[] = ['rook', 'knight', 'bishop', 'queen', 'king', 'bishop', 'knight', 'rook']
  backRowTypes.forEach((type, file) => {
    pieces.push({ id: `piece-${id++}`, type, isWhite: true, file, rank: 0 })
  })

  // White pawns
  for (let file = 0; file < 8; file++) {
    pieces.push({ id: `piece-${id++}`, type: 'pawn', isWhite: true, file, rank: 1 })
  }

  // Black pawns
  for (let file = 0; file < 8; file++) {
    pieces.push({ id: `piece-${id++}`, type: 'pawn', isWhite: false, file, rank: 6 })
  }

  // Black back row
  backRowTypes.forEach((type, file) => {
    pieces.push({ id: `piece-${id++}`, type, isWhite: false, file, rank: 7 })
  })

  return pieces
}

const DEFAULT_FEN = 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1'

export function useGameState() {
  const [pieces, setPieces] = useState<PieceState[]>(createInitialPieces)
  const [selectedPieceId, setSelectedPieceId] = useState<string | null>(null)
  const [hoverSquare, setHoverSquare] = useState<{ file: number; rank: number } | null>(null)
  const [boardFEN, setBoardFEN] = useState<string>(DEFAULT_FEN)
  const [lastMoveArrow, setLastMoveArrowState] = useState<MoveArrow | null>(null)

  const selectPiece = useCallback((pieceId: string | null) => {
    setSelectedPieceId(pieceId)
    if (pieceId) {
      setLastMoveArrowState(null)
    }
  }, [])

  const movePiece = useCallback((pieceId: string, toFile: number, toRank: number) => {
    setPieces(prev => {
      // Remove any piece at the destination (capture)
      const filtered = prev.filter(p => !(p.file === toFile && p.rank === toRank))
      // Move the piece
      return filtered.map(p =>
        p.id === pieceId ? { ...p, file: toFile, rank: toRank } : p
      )
    })
    setSelectedPieceId(null)
  }, [])

  const updateHoverSquare = useCallback((square: { file: number; rank: number } | null) => {
    setHoverSquare(square)
  }, [])

  const getPieceAt = useCallback((file: number, rank: number): PieceState | undefined => {
    return pieces.find(p => p.file === file && p.rank === rank)
  }, [pieces])

  const syncFromPieces = useCallback((newPieces: PieceState[]) => {
    setPieces(newPieces)
    setSelectedPieceId(null)
    setHoverSquare(null)
  }, [])

  const syncFromFEN = useCallback((fen: string) => {
    const state = parseFENState(fen)
    setPieces(state.pieces)
    setBoardFEN(fen)
    setSelectedPieceId(null)
    setHoverSquare(null)
  }, [])

  const setLastMoveArrow = useCallback((arrow: MoveArrow | null) => {
    setLastMoveArrowState(arrow)
  }, [])

  return {
    pieces,
    selectedPieceId,
    hoverSquare,
    boardFEN,
    lastMoveArrow,
    selectPiece,
    movePiece,
    updateHoverSquare,
    getPieceAt,
    syncFromPieces,
    syncFromFEN,
    setLastMoveArrow,
  }
}

// Convert board coordinates to 3D world position
export function boardToWorld(file: number, rank: number): [number, number, number] {
  return [file - 3.5, 0.05, rank - 3.5]
}

// Convert 3D world position to board coordinates
export function worldToBoard(x: number, z: number): { file: number; rank: number } | null {
  const file = Math.round(x + 3.5)
  const rank = Math.round(z + 3.5)

  if (file >= 0 && file < 8 && rank >= 0 && rank < 8) {
    return { file, rank }
  }
  return null
}
