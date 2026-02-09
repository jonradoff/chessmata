import { useMemo } from 'react'
import type { PieceType } from '../utils/chessRules'
import './InvalidMoveModal.css'

interface InvalidMoveModalProps {
  reason: string
  pieceType: PieceType
  onClose: () => void
}

// Visual representation of how each piece moves
const pieceMovementInfo: Record<PieceType, {
  title: string
  description: string
  visual: string[]
}> = {
  pawn: {
    title: 'Pawn Movement',
    description: 'Pawns move forward one square (or two on first move) and capture diagonally.',
    visual: [
      '. . ↑ . .',
      '. C . C .',
      '. . ♟ . .',
    ]
  },
  knight: {
    title: 'Knight Movement',
    description: 'Knights move in an L-shape: 2 squares in one direction and 1 perpendicular.',
    visual: [
      '. > . > .',
      '> . . . >',
      '. . ♞ . .',
      '> . . . >',
      '. > . > .',
    ]
  },
  bishop: {
    title: 'Bishop Movement',
    description: 'Bishops move diagonally any number of squares.',
    visual: [
      '↖ . . . . . . ↗',
      '. ↖ . . . . ↗ .',
      '. . ↖ . . ↗ . .',
      '. . . ♝ . . . .',
      '. . ↙ . . ↘ . .',
      '. ↙ . . . . ↘ .',
      '↙ . . . . . . ↘',
    ]
  },
  rook: {
    title: 'Rook Movement',
    description: 'Rooks move horizontally or vertically any number of squares.',
    visual: [
      '. . . ↑ . . .',
      '. . . ↑ . . .',
      '. . . ↑ . . .',
      '← ← ← ♜ → → →',
      '. . . ↓ . . .',
      '. . . ↓ . . .',
      '. . . ↓ . . .',
    ]
  },
  queen: {
    title: 'Queen Movement',
    description: 'Queens combine rook and bishop movement.',
    visual: [
      '↖ . . ↑ . . ↗',
      '. ↖ . ↑ . ↗ .',
      '. . ↖ ↑ ↗ . .',
      '← ← ← ♛ → → →',
      '. . ↙ ↓ ↘ . .',
      '. ↙ . ↓ . ↘ .',
      '↙ . . ↓ . . ↘',
    ]
  },
  king: {
    title: 'King Movement',
    description: 'Kings move one square in any direction.',
    visual: [
      '↖ ↑ ↗',
      '← ♚ →',
      '↙ ↓ ↘',
    ]
  }
}

// Check-specific info
const checkInfo = {
  'King is in check': {
    title: 'King in Check!',
    description: 'Your king is under attack. You must move your king to safety, block the attack, or capture the attacking piece.',
    visual: [
      '. . . ♜ . . .',
      '. . . ↓ . . .',
      '. X X ♚ X X .',
      '. X X X X X .',
    ]
  },
  'Move would expose king to check': {
    title: 'Cannot Expose King',
    description: 'This move would put your king in check. You cannot make a move that leaves your king under attack.',
    visual: [
      '. ♜ . . . . .',
      '. → X . . . .',
      '. . . ♚ . . .',
    ]
  }
}

// Blocking-specific info for sliding pieces
const blockingInfo = {
  rook: {
    title: 'Path is Blocked',
    description: 'Rooks cannot jump over other pieces. The path must be clear to move horizontally or vertically.',
    visual: [
      '. . . ↑ . . .',
      '. . . X . . .',
      '. . . ♙ . . .',
      '← ← ← ♜ → → →',
    ]
  },
  bishop: {
    title: 'Path is Blocked',
    description: 'Bishops cannot jump over other pieces. The diagonal path must be clear to move.',
    visual: [
      '↖ . . . . . .',
      '. X . . . . .',
      '. . ♙ . . . .',
      '. . . ♝ . . .',
    ]
  },
  queen: {
    title: 'Path is Blocked',
    description: 'Queens cannot jump over other pieces. The path must be clear in all directions.',
    visual: [
      '↖ . . ↑ . . .',
      '. X . X . . .',
      '. . ♙ ♙ . . .',
      '← ← ← ♛ → → →',
    ]
  }
}

export function InvalidMoveModal({ reason, pieceType, onClose }: InvalidMoveModalProps) {
  const info = useMemo(() => {
    // Check if it's a check-related reason
    if (reason.includes('check')) {
      return checkInfo[reason as keyof typeof checkInfo] || checkInfo['King is in check']
    }

    // Check if it's a blocking-related reason for sliding pieces
    if (reason.includes('block') || reason.includes('path') || reason.includes('jump')) {
      if (pieceType === 'rook' || pieceType === 'bishop' || pieceType === 'queen') {
        return blockingInfo[pieceType as keyof typeof blockingInfo]
      }
    }

    // Otherwise show piece movement rules
    return pieceMovementInfo[pieceType]
  }, [reason, pieceType])

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    <div className="modal-overlay" onClick={handleBackdropClick}>
      <div className="invalid-move-modal">
        <div className="modal-header">
          <h2>Invalid Move</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="modal-content">
          <div className="error-reason">
            {reason}
          </div>

          {info && (
            <div className="movement-info">
              <h3>{info.title}</h3>
              <p className="description">{info.description}</p>

              <div className="visual-board">
                {info.visual.map((row, i) => (
                  <div key={i} className="visual-row">
                    {row.split(' ').map((cell, j) => {
                      const isHighlighted = cell === 'x'
                      const isBlocked = cell === 'X'
                      const isArrow = ['↓', '→', '←', '↑', '↗', '↖', '↘', '↙', '>'].includes(cell)
                      const isCapture = cell === 'C'
                      const isEmpty = cell === '.'
                      // const isPiece = !isHighlighted && !isBlocked && !isArrow && !isEmpty && !isCapture

                      return (
                        <div
                          key={`${i}-${j}`}
                          className={`visual-cell ${
                            isHighlighted ? 'highlighted' :
                            isBlocked ? 'blocked' :
                            isCapture ? 'capture' :
                            isArrow ? 'arrow' :
                            isEmpty ? 'empty' :
                            'piece'
                          }`}
                        >
                          {!isEmpty && !isHighlighted ? cell : ''}
                        </div>
                      )
                    })}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="modal-actions">
          <button className="modal-button primary" onClick={onClose}>
            Got it!
          </button>
        </div>
      </div>
    </div>
  )
}
