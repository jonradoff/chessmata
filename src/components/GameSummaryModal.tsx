import type { Game, Move } from '../api/gameApi'
import './GameSummaryModal.css'

interface GameSummaryModalProps {
  game: Game
  moves: Move[]
  playerColor: 'white' | 'black'
  onClose: () => void
  onNewGame: () => void
}

export function GameSummaryModal({ game, moves, playerColor, onClose, onNewGame }: GameSummaryModalProps) {
  const isWinner = game.winner === playerColor
  const isDraw = !game.winner && game.winReason === 'stalemate'

  // Calculate game duration
  const getGameDuration = () => {
    if (!game.startedAt || !game.completedAt) {
      // Fallback: estimate from moves
      if (moves.length >= 2) {
        const firstMove = new Date(moves[0].createdAt)
        const lastMove = new Date(moves[moves.length - 1].createdAt)
        const durationMs = lastMove.getTime() - firstMove.getTime()
        return formatDuration(durationMs)
      }
      return 'N/A'
    }
    const start = new Date(game.startedAt)
    const end = new Date(game.completedAt)
    return formatDuration(end.getTime() - start.getTime())
  }

  const formatDuration = (ms: number) => {
    const totalSeconds = Math.floor(ms / 1000)
    const minutes = Math.floor(totalSeconds / 60)
    const seconds = totalSeconds % 60
    if (minutes === 0) {
      return `${seconds}s`
    }
    return `${minutes}m ${seconds}s`
  }

  const getResultMessage = () => {
    if (isDraw) {
      return 'Draw by Stalemate'
    }
    if (isWinner) {
      switch (game.winReason) {
        case 'checkmate':
          return 'Victory by Checkmate!'
        case 'resignation':
          return 'Victory - Opponent Resigned'
        default:
          return 'You Won!'
      }
    } else {
      switch (game.winReason) {
        case 'checkmate':
          return 'Defeat by Checkmate'
        case 'resignation':
          return 'Defeat - You Resigned'
        default:
          return 'You Lost'
      }
    }
  }

  const getResultClass = () => {
    if (isDraw) return 'draw'
    return isWinner ? 'victory' : 'defeat'
  }

  // Get player's Elo change
  const getEloChange = () => {
    if (!game.eloChanges) return null
    return playerColor === 'white' ? game.eloChanges.whiteChange : game.eloChanges.blackChange
  }

  const getNewElo = () => {
    if (!game.eloChanges) return null
    return playerColor === 'white' ? game.eloChanges.whiteNewElo : game.eloChanges.blackNewElo
  }

  const eloChange = getEloChange()
  const newElo = getNewElo()

  // Get player info
  const myPlayer = game.players.find(p => p.color === playerColor)
  const opponentPlayer = game.players.find(p => p.color !== playerColor)

  return (
    <div className="game-summary-overlay" onClick={onClose}>
      <div className="game-summary-modal" onClick={e => e.stopPropagation()}>
        <div className={`result-header ${getResultClass()}`}>
          <h2>{getResultMessage()}</h2>
        </div>

        <div className="summary-content">
          {/* Game Stats */}
          <div className="stats-section">
            <h3>Game Statistics</h3>
            <div className="stats-grid">
              <div className="stat-item">
                <span className="stat-label">Duration</span>
                <span className="stat-value">{getGameDuration()}</span>
              </div>
              <div className="stat-item">
                <span className="stat-label">Total Moves</span>
                <span className="stat-value">{moves.length}</span>
              </div>
              <div className="stat-item">
                <span className="stat-label">You Played</span>
                <span className="stat-value capitalize">{playerColor}</span>
              </div>
              <div className="stat-item">
                <span className="stat-label">Result</span>
                <span className="stat-value capitalize">{game.winReason || 'Complete'}</span>
              </div>
            </div>
          </div>

          {/* Player Info */}
          <div className="players-section">
            <div className="player-card you">
              <div className="player-header">
                <span className="player-label">You</span>
                <span className={`color-badge ${playerColor}`}>{playerColor}</span>
              </div>
              <span className="player-name">{myPlayer?.displayName || 'Anonymous'}</span>
              {myPlayer?.eloRating && (
                <span className="player-elo">{myPlayer.eloRating} Elo</span>
              )}
            </div>
            <div className="vs-divider">vs</div>
            <div className="player-card opponent">
              <div className="player-header">
                <span className="player-label">Opponent</span>
                <span className={`color-badge ${opponentPlayer?.color}`}>{opponentPlayer?.color}</span>
              </div>
              <span className="player-name">{opponentPlayer?.displayName || 'Anonymous'}</span>
              {opponentPlayer?.eloRating && (
                <span className="player-elo">{opponentPlayer.eloRating} Elo</span>
              )}
            </div>
          </div>

          {/* Elo Changes (for ranked games) */}
          {game.isRanked && eloChange !== null && newElo !== null && (
            <div className="elo-section">
              <h3>Rating Change</h3>
              <div className="elo-display">
                <div className={`elo-change ${eloChange >= 0 ? 'positive' : 'negative'}`}>
                  {eloChange >= 0 ? '+' : ''}{eloChange}
                </div>
                <div className="elo-arrow">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M5 12h14M12 5l7 7-7 7" />
                  </svg>
                </div>
                <div className="new-elo">
                  <span className="new-elo-value">{newElo}</span>
                  <span className="new-elo-label">New Rating</span>
                </div>
              </div>
            </div>
          )}

          {/* Unranked game note */}
          {!game.isRanked && (
            <div className="unranked-note">
              This was an unranked game. Play ranked games to track your Elo rating!
            </div>
          )}
        </div>

        <div className="summary-actions">
          <button className="action-btn secondary" onClick={onClose}>
            Close
          </button>
          <button className="action-btn primary" onClick={onNewGame}>
            New Game
          </button>
        </div>
      </div>
    </div>
  )
}
