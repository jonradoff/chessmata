import { useState, useEffect, useCallback } from 'react'
import { fetchActiveGames, fetchCompletedGames, type GameListItem } from '../api/gameApi'
import './WatchGameModal.css'

interface WatchGameModalProps {
  onClose: () => void
  onWatchGame: (sessionId: string, isActive: boolean) => void
}

type Tab = 'active' | 'completed'

function getPlayerName(game: GameListItem, color: 'white' | 'black'): string {
  const player = game.players.find(p => p.color === color)
  if (!player) return '?'
  return player.displayName || player.agentName || 'Anonymous'
}

function formatTimeAgo(dateStr?: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'just now'
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}

function formatWinReason(reason?: string): string {
  if (!reason) return ''
  const map: Record<string, string> = {
    checkmate: 'Checkmate',
    resignation: 'Resignation',
    timeout: 'Timeout',
    stalemate: 'Stalemate',
    insufficient_material: 'Insufficient Material',
    threefold_repetition: 'Threefold Repetition',
    fivefold_repetition: 'Fivefold Repetition',
    fifty_moves: 'Fifty Moves',
    seventy_five_moves: 'Seventy-Five Moves',
    agreement: 'Agreement',
  }
  return map[reason] || reason
}

export function WatchGameModal({ onClose, onWatchGame }: WatchGameModalProps) {
  const [tab, setTab] = useState<Tab>('active')
  const [activeGames, setActiveGames] = useState<GameListItem[]>([])
  const [completedGames, setCompletedGames] = useState<GameListItem[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadGames = useCallback(async (currentTab: Tab) => {
    setIsLoading(true)
    setError(null)
    try {
      if (currentTab === 'active') {
        const games = await fetchActiveGames(20)
        setActiveGames(games)
      } else {
        const games = await fetchCompletedGames(20)
        setCompletedGames(games)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load games')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadGames(tab)
  }, [tab, loadGames])

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) onClose()
  }

  const games = tab === 'active' ? activeGames : completedGames

  return (
    <div className="watch-modal-overlay" onMouseDown={handleOverlayClick}>
      <div className="watch-modal" onMouseDown={e => e.stopPropagation()}>
        <div className="watch-modal-header">
          <h2>Watch Games</h2>
          <button className="watch-close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="watch-tabs">
          <button
            className={`watch-tab ${tab === 'active' ? 'active' : ''}`}
            onClick={() => setTab('active')}
          >
            Active Games
          </button>
          <button
            className={`watch-tab ${tab === 'completed' ? 'active' : ''}`}
            onClick={() => setTab('completed')}
          >
            Completed Games
          </button>
        </div>

        <div className="watch-content">
          {isLoading && (
            <div className="watch-loading">Loading games...</div>
          )}

          {error && (
            <div className="watch-error">{error}</div>
          )}

          {!isLoading && !error && games.length === 0 && (
            <div className="watch-empty">
              {tab === 'active' ? 'No active games right now' : 'No completed games yet'}
            </div>
          )}

          {!isLoading && !error && games.length > 0 && (
            <div className="watch-game-list">
              {games.map(game => (
                <button
                  key={game.sessionId}
                  className="watch-game-entry"
                  onClick={() => {
                    onWatchGame(game.sessionId, tab === 'active')
                    onClose()
                  }}
                >
                  <div className="watch-game-players">
                    <span className="watch-white-name">{getPlayerName(game, 'white')}</span>
                    <span className="watch-vs">vs</span>
                    <span className="watch-black-name">{getPlayerName(game, 'black')}</span>
                  </div>
                  <div className="watch-game-meta">
                    {game.isRanked && <span className="watch-ranked-badge">Ranked</span>}
                    {game.timeControl && game.timeControl.mode !== 'unlimited' && (
                      <span className="watch-time-badge">{game.timeControl.mode}</span>
                    )}
                    {tab === 'active' && game.currentTurn && (
                      <span className="watch-turn">{game.currentTurn}'s turn</span>
                    )}
                    {tab === 'completed' && (
                      <>
                        {game.winner ? (
                          <span className="watch-result">
                            {getPlayerName(game, game.winner as 'white' | 'black')} won
                          </span>
                        ) : (
                          <span className="watch-result draw">Draw</span>
                        )}
                        {game.winReason && (
                          <span className="watch-reason">{formatWinReason(game.winReason)}</span>
                        )}
                      </>
                    )}
                    <span className="watch-time">
                      {tab === 'active'
                        ? formatTimeAgo(game.startedAt)
                        : formatTimeAgo(game.completedAt)}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}

          <button className="watch-refresh-button" onClick={() => loadGames(tab)} disabled={isLoading}>
            Refresh
          </button>
        </div>
      </div>
    </div>
  )
}
