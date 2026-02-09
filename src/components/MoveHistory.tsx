import type { Move } from '../api/gameApi'
import './MoveHistory.css'

interface MoveHistoryProps {
  moves: Move[]
  isMoving?: boolean
}

export function MoveHistory({ moves, isMoving = false }: MoveHistoryProps) {
  // Group moves into pairs (white, black)
  const movePairs: { number: number; white?: Move; black?: Move }[] = []

  for (let i = 0; i < moves.length; i++) {
    const move = moves[i]
    const pairIndex = Math.floor(i / 2)

    if (i % 2 === 0) {
      movePairs[pairIndex] = { number: pairIndex + 1, white: move }
    } else {
      movePairs[pairIndex].black = move
    }
  }

  // If we're moving and the last move is white (odd number of moves), show spinner for black's response
  const showOpponentSpinner = isMoving && moves.length > 0 && moves.length % 2 === 1

  return (
    <div className="move-history">
      <h3 className="move-history-title">
        Move History
        {isMoving && <span className="move-spinner">⏳</span>}
      </h3>
      <div className="move-history-list">
        {moves.length === 0 && !isMoving ? (
          <div className="no-moves">No moves yet</div>
        ) : (
          <>
            {movePairs.map((pair) => (
              <div key={pair.number} className="move-pair">
                <span className="move-number">{pair.number}.</span>
                <span className="move-notation white-move">
                  {pair.white?.notation || '...'}
                </span>
                <span className="move-notation black-move">
                  {pair.black ? pair.black.notation : (showOpponentSpinner && pair.number === movePairs.length ? <span className="inline-spinner"></span> : '')}
                </span>
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  )
}
