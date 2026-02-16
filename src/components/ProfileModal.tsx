import { useState, useCallback, useRef, useEffect } from 'react'
import { changePassword, resendVerification, changeDisplayName, checkDisplayName } from '../api/authApi'
import { fetchUserGameHistory, type MatchHistoryEntry } from '../api/gameApi'
import type { User } from '../api/authApi'
import './ProfileModal.css'

interface ProfileModalProps {
  user: User
  token: string
  onClose: () => void
  onLogout: () => void
  onUserUpdate?: (user: User) => void
  onViewGame?: (sessionId: string) => void
}

type View = 'profile' | 'change-password' | 'change-display-name' | 'game-history'

export function ProfileModal({ user, token, onClose, onLogout, onUserUpdate, onViewGame }: ProfileModalProps) {
  const [view, setView] = useState<View>('profile')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [newDisplayName, setNewDisplayName] = useState(user.displayName)
  const [displayNameError, setDisplayNameError] = useState<string | null>(null)
  const [displayNameAvailable, setDisplayNameAvailable] = useState<boolean | null>(null)
  const [isCheckingDisplayName, setIsCheckingDisplayName] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [resendCooldown, setResendCooldown] = useState(0)
  const checkTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Game history state
  const [gameHistory, setGameHistory] = useState<MatchHistoryEntry[]>([])
  const [historyPage, setHistoryPage] = useState(1)
  const [historyTotal, setHistoryTotal] = useState(0)
  const [historyFilter, setHistoryFilter] = useState<string | undefined>(undefined)
  const [rankedFilter, setRankedFilter] = useState<string | undefined>(undefined)
  const [historyLoading, setHistoryLoading] = useState(false)

  const loadGameHistory = useCallback(async (page: number, filter?: string, ranked?: string, append = false) => {
    if (!user.id) return
    setHistoryLoading(true)
    try {
      const result = await fetchUserGameHistory(user.id, page, 20, filter, ranked, token)
      setGameHistory(prev => append ? [...prev, ...result.games] : result.games)
      setHistoryTotal(result.total)
      setHistoryPage(result.page)
    } catch (err) {
      console.error('Failed to load game history:', err)
    } finally {
      setHistoryLoading(false)
    }
  }, [user.id, token])

  useEffect(() => {
    if (view === 'game-history') {
      loadGameHistory(1, historyFilter, rankedFilter)
    }
  }, [view, historyFilter, rankedFilter, loadGameHistory])

  // Calculate time until next display name change
  const getDisplayNameChangeStatus = () => {
    if (user.displayNameChanges === 0) {
      return { canChange: true, message: null }
    }
    if (!user.lastDisplayNameChange) {
      return { canChange: true, message: null }
    }

    const lastChange = new Date(user.lastDisplayNameChange)
    const now = new Date()
    const hoursSinceChange = (now.getTime() - lastChange.getTime()) / (1000 * 60 * 60)

    if (hoursSinceChange >= 24) {
      return { canChange: true, message: null }
    }

    const hoursRemaining = Math.ceil(24 - hoursSinceChange)
    return {
      canChange: false,
      message: `You can change your display name again in ${hoursRemaining} hour${hoursRemaining === 1 ? '' : 's'}`
    }
  }

  const displayNameChangeStatus = getDisplayNameChangeStatus()

  // Check display name availability with debouncing
  const checkDisplayNameAvailability = useCallback(async (name: string) => {
    if (!name || name.length < 3) {
      setDisplayNameAvailable(null)
      setDisplayNameError(name.length > 0 && name.length < 3 ? 'Display name must be at least 3 characters' : null)
      return
    }

    if (name.length > 20) {
      setDisplayNameAvailable(false)
      setDisplayNameError('Display name must be 20 characters or less')
      return
    }

    // If it's the same as current, mark as available
    if (name === user.displayName) {
      setDisplayNameAvailable(true)
      setDisplayNameError(null)
      return
    }

    setIsCheckingDisplayName(true)
    try {
      const result = await checkDisplayName(name)
      setDisplayNameAvailable(result.available)
      setDisplayNameError(result.available ? null : result.reason || 'Display name is not available')
    } catch (err) {
      console.error('Failed to check display name:', err)
      setDisplayNameError('Failed to check availability')
    } finally {
      setIsCheckingDisplayName(false)
    }
  }, [user.displayName])

  const handleDisplayNameInputChange = (value: string) => {
    setNewDisplayName(value)
    setDisplayNameAvailable(null)
    setDisplayNameError(null)

    // Clear any existing timeout
    if (checkTimeoutRef.current) {
      clearTimeout(checkTimeoutRef.current)
    }

    // Debounce the check
    checkTimeoutRef.current = setTimeout(() => {
      if (value.trim()) {
        checkDisplayNameAvailability(value.trim())
      }
    }, 500)
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    if (!newPassword || !confirmPassword) {
      setError('Please fill in all fields')
      return
    }

    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    if (!/[A-Z]/.test(newPassword) || !/[a-z]/.test(newPassword) || !/[0-9]/.test(newPassword)) {
      setError('Password must contain uppercase, lowercase, and a number')
      return
    }

    if (newPassword !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    setIsLoading(true)

    try {
      await changePassword(token, newPassword)
      setSuccess('Password changed successfully!')
      setNewPassword('')
      setConfirmPassword('')
      setTimeout(() => {
        setView('profile')
        setSuccess(null)
      }, 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setIsLoading(false)
    }
  }

  const handleChangeDisplayName = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    if (!newDisplayName.trim()) {
      setError('Display name is required')
      return
    }

    if (newDisplayName === user.displayName) {
      setView('profile')
      return
    }

    if (displayNameError) {
      setError(displayNameError)
      return
    }

    if (displayNameAvailable === false) {
      setError('Please choose a different display name')
      return
    }

    setIsLoading(true)

    try {
      const result = await changeDisplayName(token, newDisplayName.trim())
      setSuccess('Display name changed successfully!')
      if (onUserUpdate) {
        onUserUpdate(result.user)
      }
      setTimeout(() => {
        setView('profile')
        setSuccess(null)
      }, 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to change display name')
    } finally {
      setIsLoading(false)
    }
  }

  const handleResendVerification = async () => {
    if (resendCooldown > 0) return

    setError(null)
    setSuccess(null)
    setIsLoading(true)

    try {
      await resendVerification(user.email)
      setSuccess('Verification email sent! Please check your inbox.')
      // Start cooldown timer
      setResendCooldown(60)
      const interval = setInterval(() => {
        setResendCooldown(prev => {
          if (prev <= 1) {
            clearInterval(interval)
            return 0
          }
          return prev - 1
        })
      }, 1000)
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to send verification email'
      // Check for rate limit error
      if (errorMessage.includes('Please wait')) {
        setError(errorMessage)
        // Extract seconds from error message and set cooldown
        const match = errorMessage.match(/(\d+) seconds/)
        if (match) {
          const seconds = parseInt(match[1])
          setResendCooldown(seconds)
          const interval = setInterval(() => {
            setResendCooldown(prev => {
              if (prev <= 1) {
                clearInterval(interval)
                return 0
              }
              return prev - 1
            })
          }, 1000)
        }
      } else {
        setError(errorMessage)
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  const handleModalMouseDown = (e: React.MouseEvent) => {
    e.stopPropagation()
  }

  if (view === 'game-history') {
    const hasMore = gameHistory.length < historyTotal

    const getResultForUser = (entry: MatchHistoryEntry) => {
      if (!entry.winner) return 'draw'
      // Determine if user was white or black
      const isWhite = entry.whiteUserId === user.id
      const isBlack = entry.blackUserId === user.id
      if (!isWhite && !isBlack) return 'unknown'
      if (isWhite && entry.winner === 'white') return 'win'
      if (isBlack && entry.winner === 'black') return 'win'
      return 'loss'
    }

    const getEloChange = (entry: MatchHistoryEntry) => {
      const isWhite = entry.whiteUserId === user.id
      return isWhite ? entry.whiteEloChange : entry.blackEloChange
    }

    const formatWinReason = (reason: string) => {
      const map: Record<string, string> = {
        checkmate: 'Checkmate', resignation: 'Resignation', timeout: 'Timeout',
        stalemate: 'Stalemate', insufficient_material: 'Insufficient Material',
        threefold_repetition: 'Threefold Repetition', fivefold_repetition: 'Fivefold Repetition',
        fifty_moves: 'Fifty Moves', seventy_five_moves: '75 Moves', agreement: 'Agreement',
      }
      return map[reason] || reason
    }

    const formatDate = (dateStr: string) => {
      const d = new Date(dateStr)
      return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
    }

    return (
      <div className="profile-modal-overlay" onMouseDown={handleOverlayClick}>
        <div className="profile-modal profile-modal-wide" onMouseDown={handleModalMouseDown}>
          <div className="modal-header">
            <h2>Game History</h2>
            <button className="close-button" onClick={onClose}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <div className="modal-content">
            <div className="game-history-filters">
              {([['all', undefined], ['ranked', 'true'], ['unranked', 'false']] as const).map(([label, value]) => (
                <button
                  type="button"
                  key={label}
                  className={`filter-btn ${(label === 'all' && !rankedFilter) || rankedFilter === value ? 'active' : ''}`}
                  onClick={() => {
                    setRankedFilter(value ?? undefined)
                    setGameHistory([])
                  }}
                >
                  {label.charAt(0).toUpperCase() + label.slice(1)}
                </button>
              ))}
            </div>
            <div className="game-history-filters">
              {(['all', 'wins', 'losses', 'draws'] as const).map(f => (
                <button
                  type="button"
                  key={f}
                  className={`filter-btn ${(f === 'all' && !historyFilter) || historyFilter === f ? 'active' : ''}`}
                  onClick={() => {
                    setHistoryFilter(f === 'all' ? undefined : f)
                    setGameHistory([])
                  }}
                >
                  {f.charAt(0).toUpperCase() + f.slice(1)}
                </button>
              ))}
            </div>

            {historyLoading && gameHistory.length === 0 && (
              <div className="history-loading">Loading games...</div>
            )}

            {!historyLoading && gameHistory.length === 0 && (
              <div className="history-empty">No games found</div>
            )}

            <div className="game-history-list">
              {gameHistory.map(entry => {
                const result = getResultForUser(entry)
                const eloChange = getEloChange(entry)
                return (
                  <button
                    key={entry.id}
                    className="game-entry"
                    onClick={() => {
                      if (onViewGame && entry.sessionId) {
                        onViewGame(entry.sessionId)
                      }
                    }}
                    disabled={!onViewGame || !entry.sessionId}
                  >
                    <div className="game-entry-top">
                      <span className="game-entry-players">
                        {entry.whiteDisplayName || entry.whiteAgent || '?'} vs {entry.blackDisplayName || entry.blackAgent || '?'}
                      </span>
                      <span className="game-entry-date">{formatDate(entry.completedAt)}</span>
                    </div>
                    <div className="game-entry-bottom">
                      <span className={`game-entry-result ${result}`}>
                        {result === 'win' ? 'Won' : result === 'loss' ? 'Lost' : result === 'draw' ? 'Draw' : '—'}
                      </span>
                      {entry.winReason && (
                        <span className="game-entry-reason">{formatWinReason(entry.winReason)}</span>
                      )}
                      {entry.isRanked && (
                        <span className="ranked-badge-sm">Ranked</span>
                      )}
                      {entry.isRanked && eloChange !== 0 && (
                        <span className={`game-entry-elo ${eloChange > 0 ? 'positive' : 'negative'}`}>
                          {eloChange > 0 ? '+' : ''}{eloChange}
                        </span>
                      )}
                      <span className="game-entry-moves">{entry.totalMoves} moves</span>
                    </div>
                  </button>
                )
              })}
            </div>

            {hasMore && (
              <button
                className="load-more-btn"
                onClick={() => loadGameHistory(historyPage + 1, historyFilter, rankedFilter, true)}
                disabled={historyLoading}
              >
                {historyLoading ? 'Loading...' : 'Load More'}
              </button>
            )}

            <button className="back-button" onClick={() => setView('profile')}>
              Back to Profile
            </button>
          </div>
        </div>
      </div>
    )
  }

  if (view === 'change-display-name') {
    return (
      <div className="profile-modal-overlay" onMouseDown={handleOverlayClick}>
        <div className="profile-modal" onMouseDown={handleModalMouseDown}>
          <div className="modal-header">
            <h2>Change Display Name</h2>
            <button className="close-button" onClick={onClose}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <div className="modal-content">
            <p className="change-info-text">
              Choose a new display name. Display names must be 3-20 characters and can only contain letters, numbers, and underscores.
            </p>

            <form onSubmit={handleChangeDisplayName}>
              <div className="form-group">
                <label htmlFor="newDisplayName">New Display Name</label>
                <div className="input-with-status">
                  <input
                    id="newDisplayName"
                    type="text"
                    value={newDisplayName}
                    onChange={(e) => handleDisplayNameInputChange(e.target.value)}
                    placeholder="Enter new display name"
                    disabled={isLoading}
                    className={displayNameError ? 'input-error' : displayNameAvailable ? 'input-success' : ''}
                  />
                  {isCheckingDisplayName && (
                    <span className="input-status checking">Checking...</span>
                  )}
                  {!isCheckingDisplayName && displayNameAvailable === true && (
                    <span className="input-status available">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                    </span>
                  )}
                  {!isCheckingDisplayName && displayNameAvailable === false && (
                    <span className="input-status unavailable">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                      </svg>
                    </span>
                  )}
                </div>
                {displayNameError && <div className="field-error">{displayNameError}</div>}
              </div>

              {error && <div className="error-message">{error}</div>}
              {success && <div className="success-message">{success}</div>}

              <button
                type="submit"
                className="submit-button"
                disabled={isLoading || displayNameAvailable === false || newDisplayName === user.displayName}
              >
                {isLoading ? 'Changing...' : 'Change Display Name'}
              </button>
            </form>

            <button className="back-button" onClick={() => {
              setView('profile')
              setNewDisplayName(user.displayName)
              setDisplayNameError(null)
              setDisplayNameAvailable(null)
              setError(null)
              setSuccess(null)
            }}>
              Back to Profile
            </button>
          </div>
        </div>
      </div>
    )
  }

  if (view === 'change-password') {
    return (
      <div className="profile-modal-overlay" onMouseDown={handleOverlayClick}>
        <div className="profile-modal" onMouseDown={handleModalMouseDown}>
          <div className="modal-header">
            <h2>Change Password</h2>
            <button className="close-button" onClick={onClose}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>

          <div className="modal-content">
            <p className="change-password-description">
              Enter your new password below. You will remain logged in after changing your password.
            </p>

            <form onSubmit={handleChangePassword}>
              <div className="form-group">
                <label htmlFor="newPassword">New Password</label>
                <input
                  id="newPassword"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="8+ chars, uppercase, lowercase, number"
                  disabled={isLoading}
                  autoComplete="new-password"
                />
              </div>

              <div className="form-group">
                <label htmlFor="confirmPassword">Confirm Password</label>
                <input
                  id="confirmPassword"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Confirm your new password"
                  disabled={isLoading}
                  autoComplete="new-password"
                />
              </div>

              {error && <div className="error-message">{error}</div>}
              {success && <div className="success-message">{success}</div>}

              <button
                type="submit"
                className="submit-button"
                disabled={isLoading}
              >
                {isLoading ? 'Changing Password...' : 'Change Password'}
              </button>
            </form>

            <button className="back-button" onClick={() => setView('profile')}>
              Back to Profile
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="profile-modal-overlay" onMouseDown={handleOverlayClick}>
      <div className="profile-modal" onMouseDown={handleModalMouseDown}>
        <div className="modal-header">
          <h2>Profile</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="modal-content">
          <div className="profile-info">
            <div className="profile-avatar">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
            </div>
            <div className="profile-details">
              <div className="display-name-row">
                <h3 className="display-name">{user.displayName}</h3>
                {displayNameChangeStatus.canChange ? (
                  <button
                    className="edit-display-name-button"
                    onClick={() => setView('change-display-name')}
                    title="Change display name"
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                    </svg>
                  </button>
                ) : (
                  <span className="change-cooldown-hint" title={displayNameChangeStatus.message || ''}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                      <circle cx="12" cy="12" r="10" />
                      <polyline points="12 6 12 12 16 14" />
                    </svg>
                  </span>
                )}
              </div>
              <p className="email">{user.email}</p>
              {!user.emailVerified && (
                <div className="email-not-verified">
                  <span className="warning-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                      <line x1="12" y1="9" x2="12" y2="13" />
                      <line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                  </span>
                  <span>Email not verified</span>
                  <button
                    className="resend-verification-button"
                    onClick={handleResendVerification}
                    disabled={isLoading || resendCooldown > 0}
                  >
                    {resendCooldown > 0 ? `Wait ${resendCooldown}s` : (isLoading ? 'Sending...' : 'Resend')}
                  </button>
                </div>
              )}
            </div>
          </div>

          {error && <div className="error-message">{error}</div>}
          {success && <div className="success-message">{success}</div>}

          <div className="profile-stats">
            <div className="stat-item">
              <span className="stat-label">Elo Rating</span>
              <span className="stat-value">{user.eloRating}</span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Ranked Games</span>
              <span className="stat-value">{user.rankedGamesPlayed}</span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Wins</span>
              <span className="stat-value wins">{user.rankedWins}</span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Losses</span>
              <span className="stat-value losses">{user.rankedLosses}</span>
            </div>
            <div className="stat-item">
              <span className="stat-label">Draws</span>
              <span className="stat-value">{user.rankedDraws}</span>
            </div>
          </div>

          <div className="profile-actions">
            <button
              className="view-games-button"
              onClick={() => setView('game-history')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
                <line x1="16" y1="13" x2="8" y2="13" />
                <line x1="16" y1="17" x2="8" y2="17" />
              </svg>
              View Games
            </button>

            <button
              className="change-password-button"
              onClick={() => setView('change-password')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
              Change Password
            </button>

            <button className="logout-button" onClick={onLogout}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                <polyline points="16 17 21 12 16 7" />
                <line x1="21" y1="12" x2="9" y2="12" />
              </svg>
              Logout
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
