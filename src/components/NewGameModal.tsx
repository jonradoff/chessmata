import { useState, useEffect, useCallback, useRef } from 'react'
import { useMatchmaking } from '../hooks/useMatchmaking'
import { useAuth } from '../hooks/useAuth'
import { TIME_CONTROL_DISPLAY_NAMES } from './Clock'
import { updatePreferences } from '../api/authApi'
import { fetchLobby, type LobbyEntry, type TimeControlMode } from '../api/gameApi'
import { EmailVerificationRequiredModal } from './EmailVerificationRequiredModal'
import './NewGameModal.css'

const RANKED_PREF_KEY = 'chessmata_ranked_preference'
const TIME_CONTROLS_PREF_KEY = 'chessmata_time_controls_preference'

const ALL_TIME_CONTROLS: TimeControlMode[] = ['unlimited', 'casual', 'standard', 'quick', 'blitz', 'tournament']
const DEFAULT_TIME_CONTROLS: TimeControlMode[] = ['casual']

function formatElapsed(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins > 0) {
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }
  return `${secs}s`
}

function SearchingView({
  isSearching,
  queueStatus,
  opponentType,
  isRanked,
  onCancel,
}: {
  isSearching: boolean
  queueStatus: { estimatedWait?: string } | null
  opponentType: 'human' | 'ai' | 'either'
  isRanked: boolean
  onCancel: () => void
}) {
  const [elapsedSeconds, setElapsedSeconds] = useState(0)

  useEffect(() => {
    if (!isSearching) {
      setElapsedSeconds(0)
      return
    }
    const start = Date.now()
    const interval = setInterval(() => {
      setElapsedSeconds(Math.floor((Date.now() - start) / 1000))
    }, 1000)
    return () => clearInterval(interval)
  }, [isSearching])

  return (
    <div className="searching-view">
      <div className="searching-animation">
        <div className="spinner"></div>
        <p className="searching-text">
          {queueStatus?.estimatedWait || 'Finding opponent...'}
        </p>
        <p className="searching-elapsed">{formatElapsed(elapsedSeconds)}</p>
      </div>
      <p className="searching-hint">
        Matching you with {opponentType === 'human' ? 'a human player' : opponentType === 'ai' ? 'an AI' : 'an opponent'}
        {isRanked && ' for a ranked game'}
      </p>
      <button
        className="cancel-search-btn"
        onClick={onCancel}
      >
        Cancel Search
      </button>
    </div>
  )
}

interface NewGameModalProps {
  isLoading: boolean
  error: string | null
  sessionId: string | null
  playerId: string | null
  shareLink: string | null
  onCreateGame: () => Promise<void>
  onMatchFound?: (sessionId: string, connectionId: string) => void
  onClose: () => void
  lastGameWasRanked?: boolean
}

function formatWaitTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m`
  const diffHr = Math.floor(diffMin / 60)
  return `${diffHr}h ${diffMin % 60}m`
}

type TabType = 'find-opponent' | 'connection-string' | 'lobby'

export function NewGameModal({
  isLoading,
  error,
  sessionId,
  playerId,
  shareLink,
  onCreateGame,
  onMatchFound,
  onClose,
  lastGameWasRanked,
}: NewGameModalProps) {
  const [activeTab, setActiveTab] = useState<TabType>('find-opponent')
  const [copiedShare, setCopiedShare] = useState(false)
  const [copiedResume, setCopiedResume] = useState(false)

  // Matchmaking options - default to last game's ranked setting or localStorage preference
  const [isRanked, setIsRanked] = useState(() => {
    // First priority: last game was ranked
    if (lastGameWasRanked !== undefined) {
      return lastGameWasRanked
    }
    // Second priority: check localStorage
    const stored = localStorage.getItem(RANKED_PREF_KEY)
    return stored === 'true'
  })
  const [opponentType, setOpponentType] = useState<'human' | 'ai' | 'either'>('either')
  const [preferredColor, setPreferredColor] = useState<'random' | 'white' | 'black'>('random')

  const [lobbyEntries, setLobbyEntries] = useState<LobbyEntry[]>([])
  const [lobbyLoading, setLobbyLoading] = useState(false)
  const [, setLobbyTick] = useState(0)

  const [showVerifyModal, setShowVerifyModal] = useState(false)
  const auth = useAuth()
  const matchmaking = useMatchmaking()

  const loadLobby = useCallback(async () => {
    setLobbyLoading(true)
    try {
      const entries = await fetchLobby()
      setLobbyEntries(entries)
    } catch {
      setLobbyEntries([])
    } finally {
      setLobbyLoading(false)
    }
  }, [])

  // Connect to lobby WebSocket when tab is active, fall back to polling
  const lobbyWsRef = useRef<WebSocket | null>(null)
  useEffect(() => {
    if (activeTab !== 'lobby') return

    let pollingInterval: ReturnType<typeof setInterval> | null = null

    // Determine WebSocket URL
    const getWsUrl = () => {
      if (import.meta.env.VITE_WS_URL) return `${import.meta.env.VITE_WS_URL}/lobby`
      if (window.location.hostname === 'localhost') return 'ws://localhost:9029/ws/lobby'
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      return `${wsProtocol}//${window.location.host}/ws/lobby`
    }

    const ws = new WebSocket(getWsUrl())
    lobbyWsRef.current = ws

    ws.onopen = () => {
      // Connected; lobby updates arrive via messages
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'lobby_update' && Array.isArray(data.entries)) {
          setLobbyEntries(data.entries)
          setLobbyLoading(false)
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.onerror = () => {
      // Fall back to HTTP polling if WS fails
      if (!pollingInterval) {
        loadLobby()
        pollingInterval = setInterval(loadLobby, 10000)
      }
    }

    ws.onclose = () => {
      // Fall back to HTTP polling if WS disconnects
      if (!pollingInterval) {
        pollingInterval = setInterval(loadLobby, 10000)
      }
    }

    // Initial load via HTTP while WS connects
    loadLobby()

    return () => {
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
      lobbyWsRef.current = null
      if (pollingInterval) {
        clearInterval(pollingInterval)
      }
    }
  }, [activeTab, loadLobby])

  // Tick every second to update lobby wait times client-side
  useEffect(() => {
    if (activeTab !== 'lobby' || lobbyEntries.length === 0) return
    const timer = setInterval(() => setLobbyTick(t => t + 1), 1000)
    return () => clearInterval(timer)
  }, [activeTab, lobbyEntries.length])

  // Initialize time controls from user preferences or localStorage
  const [selectedTimeControls, setSelectedTimeControls] = useState<TimeControlMode[]>(() => {
    // First check user preferences from auth state
    if (auth.user?.preferences?.preferredTimeControls && auth.user.preferences.preferredTimeControls.length > 0) {
      return auth.user.preferences.preferredTimeControls
    }
    // Fall back to localStorage
    const stored = localStorage.getItem(TIME_CONTROLS_PREF_KEY)
    if (stored) {
      try {
        const parsed = JSON.parse(stored)
        if (Array.isArray(parsed) && parsed.length > 0) {
          return parsed as TimeControlMode[]
        }
      } catch {
        // ignore parse errors
      }
    }
    return DEFAULT_TIME_CONTROLS
  })

  // Update time controls when user preferences load/change
  useEffect(() => {
    if (auth.user?.preferences?.preferredTimeControls && auth.user.preferences.preferredTimeControls.length > 0) {
      setSelectedTimeControls(auth.user.preferences.preferredTimeControls)
      // Also sync to localStorage
      localStorage.setItem(TIME_CONTROLS_PREF_KEY, JSON.stringify(auth.user.preferences.preferredTimeControls))
    }
  }, [auth.user?.preferences?.preferredTimeControls])

  // Handle match found
  useEffect(() => {
    if (matchmaking.matchedSessionId && matchmaking.connectionId && onMatchFound) {
      onMatchFound(matchmaking.matchedSessionId, matchmaking.connectionId)
      matchmaking.clearMatch()
      onClose()
    }
  }, [matchmaking.matchedSessionId, matchmaking.connectionId, onMatchFound, matchmaking, onClose])

  const resumeLink = sessionId && playerId
    ? `${window.location.origin}/game/${sessionId}?player=${playerId}`
    : null

  const handleCopyShare = async () => {
    if (shareLink) {
      await navigator.clipboard.writeText(shareLink)
      setCopiedShare(true)
      setTimeout(() => setCopiedShare(false), 2000)
    }
  }

  const handleCopyResume = async () => {
    if (resumeLink) {
      await navigator.clipboard.writeText(resumeLink)
      setCopiedResume(true)
      setTimeout(() => setCopiedResume(false), 2000)
    }
  }

  const handleFindOpponent = async () => {
    // Block authenticated users with unverified emails
    if (auth.user && !auth.user.emailVerified) {
      setShowVerifyModal(true)
      return
    }
    const displayName = auth.user?.displayName || 'Player'
    // Save preferences for next time
    localStorage.setItem(RANKED_PREF_KEY, String(isRanked))
    localStorage.setItem(TIME_CONTROLS_PREF_KEY, JSON.stringify(selectedTimeControls))

    // If logged in, save to user profile
    if (auth.token && auth.user) {
      try {
        await updatePreferences(auth.token, {
          preferredTimeControls: selectedTimeControls,
        })
      } catch (err) {
        // Non-critical, just log and continue
        console.error('Failed to save time control preferences:', err)
      }
    }

    matchmaking.findOpponent({
      displayName,
      isRanked,
      preferredColor: preferredColor !== 'random' ? preferredColor : undefined,
      opponentType,
      timeControls: selectedTimeControls,
      token: auth.token || undefined,
    })
  }

  const handleTimeControlToggle = (mode: TimeControlMode) => {
    setSelectedTimeControls(prev => {
      if (prev.includes(mode)) {
        // Don't allow deselecting the last one
        if (prev.length === 1) return prev
        return prev.filter(tc => tc !== mode)
      } else {
        return [...prev, mode]
      }
    })
  }

  const handleCancelSearch = () => {
    matchmaking.cancelSearch()
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      if (matchmaking.isSearching) {
        matchmaking.cancelSearch()
      }
      onClose()
    }
  }

  // Check if ranked is allowed (requires authentication)
  const canPlayRanked = !!auth.user

  return (
    <div className="new-game-backdrop" onClick={handleBackdropClick}>
      <div className="new-game-modal">
        <div className="new-game-header">
          <h2>New Game</h2>
          <button className="close-button" onClick={() => {
            if (matchmaking.isSearching) {
              matchmaking.cancelSearch()
            }
            onClose()
          }}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {/* Tab navigation - only show when not in a session */}
        {!sessionId && (
          <div className="new-game-tabs">
            <button
              className={`tab-button ${activeTab === 'find-opponent' ? 'active' : ''}`}
              onClick={() => setActiveTab('find-opponent')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
              Find Opponent
            </button>
            <button
              className={`tab-button ${activeTab === 'connection-string' ? 'active' : ''}`}
              onClick={() => setActiveTab('connection-string')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
              </svg>
              Connection String
            </button>
            <button
              className={`tab-button ${activeTab === 'lobby' ? 'active' : ''}`}
              onClick={() => setActiveTab('lobby')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                <circle cx="9" cy="7" r="4" />
                <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
                <path d="M16 3.13a4 4 0 0 1 0 7.75" />
              </svg>
              Lobby
            </button>
          </div>
        )}

        <div className="new-game-content">
          {(error || matchmaking.error) && (
            <div className="error-message">
              {error || matchmaking.error}
            </div>
          )}

          {/* Session created view - only show when we have a session ID */}
          {sessionId ? (
            <div className="links-section">
              <div className="link-group">
                <label>Send this link to your opponent:</label>
                <div className="link-row">
                  <input
                    type="text"
                    readOnly
                    value={shareLink || ''}
                    className="link-input"
                  />
                  <button
                    className="copy-btn"
                    onClick={handleCopyShare}
                  >
                    {copiedShare ? 'Copied!' : 'Copy'}
                  </button>
                </div>
              </div>

              <div className="link-group">
                <label>Your session link (to resume later):</label>
                <div className="link-row">
                  <input
                    type="text"
                    readOnly
                    value={resumeLink || ''}
                    className="link-input"
                  />
                  <button
                    className="copy-btn"
                    onClick={handleCopyResume}
                  >
                    {copiedResume ? 'Copied!' : 'Copy'}
                  </button>
                </div>
                <p className="link-hint">
                  Save this link to resume your game from any device.
                </p>
              </div>

              <div className="status-section">
                <p className="waiting-text">
                  Waiting for opponent to join...
                </p>
              </div>

              <button className="start-playing-btn" onClick={onClose}>
                Start Playing
              </button>
            </div>
          ) : activeTab === 'find-opponent' ? (
            /* Find Opponent Tab */
            <div className="find-opponent-section">
              {matchmaking.isSearching ? (
                /* Searching view */
                <SearchingView
                  isSearching={matchmaking.isSearching}
                  queueStatus={matchmaking.queueStatus}
                  opponentType={opponentType}
                  isRanked={isRanked}
                  onCancel={handleCancelSearch}
                />
              ) : (
                /* Options view */
                <>
                  <p className="description">
                    Find an opponent automatically based on your preferences.
                  </p>

                  <div className="matchmaking-options">
                    {/* Ranked option */}
                    <div className="option-group">
                      <label className="checkbox-label">
                        <input
                          type="checkbox"
                          checked={isRanked}
                          onChange={(e) => setIsRanked(e.target.checked)}
                          disabled={!canPlayRanked}
                        />
                        <span className="checkbox-custom"></span>
                        <span className="checkbox-text">
                          Play Ranked Game
                          {!canPlayRanked && (
                            <span className="option-note">(Login required)</span>
                          )}
                        </span>
                      </label>
                      <p className="option-description">
                        Ranked games affect your Elo rating and appear on the leaderboard.
                      </p>
                    </div>

                    {/* Time controls */}
                    <div className="option-group">
                      <label className="option-label">Time Controls</label>
                      <p className="option-description">
                        Select all time formats you're willing to play. You'll be matched with opponents who share at least one selection.
                      </p>
                      <div className="time-controls-grid">
                        {ALL_TIME_CONTROLS.map(mode => (
                          <label key={mode} className="checkbox-label time-control-option">
                            <input
                              type="checkbox"
                              checked={selectedTimeControls.includes(mode)}
                              onChange={() => handleTimeControlToggle(mode)}
                            />
                            <span className="checkbox-custom"></span>
                            <span className="checkbox-text">{TIME_CONTROL_DISPLAY_NAMES[mode]}</span>
                          </label>
                        ))}
                      </div>
                    </div>

                    {/* Opponent type */}
                    <div className="option-group">
                      <label className="option-label">Opponent Type</label>
                      <div className="radio-group">
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="opponentType"
                            value="either"
                            checked={opponentType === 'either'}
                            onChange={() => setOpponentType('either')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">Anyone</span>
                        </label>
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="opponentType"
                            value="human"
                            checked={opponentType === 'human'}
                            onChange={() => setOpponentType('human')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">Humans Only</span>
                        </label>
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="opponentType"
                            value="ai"
                            checked={opponentType === 'ai'}
                            onChange={() => setOpponentType('ai')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">AIs Only</span>
                        </label>
                      </div>
                    </div>

                    {/* Preferred color */}
                    <div className="option-group">
                      <label className="option-label">Play As</label>
                      <div className="radio-group">
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="preferredColor"
                            value="random"
                            checked={preferredColor === 'random'}
                            onChange={() => setPreferredColor('random')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">Random</span>
                        </label>
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="preferredColor"
                            value="white"
                            checked={preferredColor === 'white'}
                            onChange={() => setPreferredColor('white')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">White</span>
                        </label>
                        <label className="radio-label">
                          <input
                            type="radio"
                            name="preferredColor"
                            value="black"
                            checked={preferredColor === 'black'}
                            onChange={() => setPreferredColor('black')}
                          />
                          <span className="radio-custom"></span>
                          <span className="radio-text">Black</span>
                        </label>
                      </div>
                    </div>
                  </div>

                  <button
                    className="find-opponent-btn"
                    onClick={handleFindOpponent}
                    disabled={isLoading}
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="11" cy="11" r="8" />
                      <line x1="21" y1="21" x2="16.65" y2="16.65" />
                    </svg>
                    Find Opponent
                  </button>
                </>
              )}
            </div>
          ) : activeTab === 'connection-string' ? (
            /* Connection String Tab */
            <div className="create-section">
              <p className="description">
                Create a game and share the link with your opponent.
              </p>
              <button
                className="create-game-btn"
                onClick={() => {
                  if (auth.user && !auth.user.emailVerified) {
                    setShowVerifyModal(true)
                    return
                  }
                  onCreateGame()
                }}
                disabled={isLoading}
              >
                {isLoading ? 'Creating Game...' : 'Create Game'}
              </button>
            </div>
          ) : (
            /* Lobby Tab */
            <div className="lobby-section">
              <p className="description">
                Players and agents currently waiting for a match.
              </p>

              {lobbyLoading && lobbyEntries.length === 0 && (
                <div className="lobby-loading">Loading lobby...</div>
              )}

              {!lobbyLoading && lobbyEntries.length === 0 && (
                <div className="lobby-empty">No one is waiting for a match right now.</div>
              )}

              {lobbyEntries.length > 0 && (
                <div className="lobby-list">
                  {lobbyEntries.map((entry, i) => (
                    <div key={i} className="lobby-entry">
                      <div className="lobby-entry-name">
                        <span className="lobby-display-name">{entry.displayName}</span>
                        {entry.agentName && (
                          <span className="lobby-agent-badge">{entry.agentName}</span>
                        )}
                        {entry.engineName && (
                          <span className="lobby-agent-badge" title="Engine name">{entry.engineName}</span>
                        )}
                      </div>
                      <div className="lobby-entry-details">
                        <span className="lobby-elo">{entry.currentElo} Elo</span>
                        {entry.isRanked && <span className="lobby-ranked-badge">Ranked</span>}
                        <span className="lobby-opponent-type">
                          {entry.opponentType === 'human' ? 'Humans' : entry.opponentType === 'ai' ? 'AIs' : 'Anyone'}
                        </span>
                        {entry.timeControls && entry.timeControls.length > 0 && (
                          <span className="lobby-time-controls">
                            {entry.timeControls.map(tc => TIME_CONTROL_DISPLAY_NAMES[tc] || tc).join(', ')}
                          </span>
                        )}
                        {entry.preferredColor && (
                          <span className={`lobby-color-pref ${entry.preferredColor}`}>
                            {entry.preferredColor}
                          </span>
                        )}
                        <span className="lobby-wait-time">
                          {formatWaitTime(entry.waitingSince)}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              <button
                className="lobby-refresh-btn"
                onClick={loadLobby}
                disabled={lobbyLoading}
              >
                {lobbyLoading ? 'Refreshing...' : 'Refresh'}
              </button>
            </div>
          )}
        </div>
      </div>
      {showVerifyModal && auth.user && (
        <EmailVerificationRequiredModal
          email={auth.user.email}
          onClose={() => setShowVerifyModal(false)}
        />
      )}
    </div>
  )
}
