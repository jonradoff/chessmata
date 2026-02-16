import { useState, useEffect } from 'react'
import { fetchLeaderboard } from '../api/gameApi'
import type { LeaderboardEntry } from '../api/gameApi'
import './LeaderboardModal.css'

interface LeaderboardModalProps {
  onClose: () => void
}

type TabType = 'players' | 'agents'

export function LeaderboardModal({ onClose }: LeaderboardModalProps) {
  const [activeTab, setActiveTab] = useState<TabType>('players')
  const [entries, setEntries] = useState<LeaderboardEntry[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    fetchLeaderboard(activeTab).then(data => {
      setEntries(data || [])
      setLoading(false)
    }).catch(() => {
      setEntries([])
      setLoading(false)
    })
  }, [activeTab])

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    <div className="modal-overlay" onClick={handleBackdropClick}>
      <div className="leaderboard-modal">
        <div className="modal-header">
          <h2>Leaderboard</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="leaderboard-tabs">
          <button
            className={`leaderboard-tab ${activeTab === 'players' ? 'active' : ''}`}
            onClick={() => setActiveTab('players')}
          >
            Players
          </button>
          <button
            className={`leaderboard-tab ${activeTab === 'agents' ? 'active' : ''}`}
            onClick={() => setActiveTab('agents')}
          >
            Agents
          </button>
        </div>

        <div className="leaderboard-content">
          {loading ? (
            <div className="leaderboard-loading">Loading...</div>
          ) : entries.length === 0 ? (
            <div className="leaderboard-empty">
              No {activeTab === 'players' ? 'players' : 'agents'} with ranked games yet.
            </div>
          ) : (
            <table className="leaderboard-table">
              <thead>
                <tr>
                  <th className="col-rank">#</th>
                  <th className="col-name">Name</th>
                  <th className="col-elo">Elo</th>
                  <th className="col-record">W/L/D</th>
                  <th className="col-games">Games</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.rank}>
                    <td className="col-rank">
                      {entry.rank <= 3 ? (
                        <span className={`rank-medal rank-${entry.rank}`}>
                          {entry.rank === 1 ? '🥇' : entry.rank === 2 ? '🥈' : '🥉'}
                        </span>
                      ) : (
                        entry.rank
                      )}
                    </td>
                    <td className="col-name">{entry.displayName}</td>
                    <td className="col-elo">{entry.eloRating}</td>
                    <td className="col-record">
                      <span className="wins">{entry.wins}</span>
                      /
                      <span className="losses">{entry.losses}</span>
                      /
                      <span className="draws">{entry.draws}</span>
                    </td>
                    <td className="col-games">{entry.gamesPlayed}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
