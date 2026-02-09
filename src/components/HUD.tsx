import { useState } from 'react'
import { SettingsModal } from './SettingsModal'
import { MoveHistory } from './MoveHistory'
import { NewGameModal } from './NewGameModal'
import { ResignConfirmModal } from './ResignConfirmModal'
import { AboutModal } from './AboutModal'
import { AuthModal } from './AuthModal'
import { useAuth } from '../hooks/useAuth'
import type { Settings, SettingsContextType } from '../hooks/useSettings'
import type { Move, Game } from '../api/gameApi'
import './HUD.css'

interface HUDProps {
  is3D: boolean
  onToggle: () => void
  isTransitioning: boolean
  settings: Settings
  updateSettings: SettingsContextType['updateSettings']
  // Online game props
  onNewGame?: () => Promise<unknown>
  onLeaveGame?: () => void
  onResign?: () => Promise<void>
  sessionId?: string | null
  playerId?: string | null
  playerColor?: 'white' | 'black' | null
  game?: Game | null
  moves?: Move[]
  shareLink?: string | null
  isLoading?: boolean
  isMoving?: boolean
  error?: string | null
}

export function HUD({
  is3D,
  onToggle,
  isTransitioning,
  settings,
  updateSettings,
  onNewGame,
  onLeaveGame,
  onResign,
  sessionId,
  playerId,
  playerColor,
  game,
  moves = [],
  shareLink,
  isLoading,
  isMoving,
  error,
}: HUDProps) {
  const [showSettings, setShowSettings] = useState(false)
  const [showNewGame, setShowNewGame] = useState(false)
  const [showResignConfirm, setShowResignConfirm] = useState(false)
  const [showAbout, setShowAbout] = useState(false)
  const [showAuth, setShowAuth] = useState(false)
  const [isResigning, setIsResigning] = useState(false)
  const [copied, setCopied] = useState(false)

  const auth = useAuth()

  const handleCopyLink = async () => {
    if (shareLink) {
      await navigator.clipboard.writeText(shareLink)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const handleNewGameClick = () => {
    console.log('New Game button clicked, opening modal')
    setShowNewGame(true)
  }

  const handleCreateGame = async () => {
    console.log('Create Game button clicked')
    if (onNewGame) {
      try {
        await onNewGame()
        console.log('Game created successfully')
      } catch (err) {
        console.error('Failed to create game:', err)
      }
    }
  }

  const handleCloseNewGame = () => {
    setShowNewGame(false)
  }

  const handleResignClick = () => {
    setShowResignConfirm(true)
  }

  const handleResignConfirm = async () => {
    if (onResign) {
      setIsResigning(true)
      try {
        await onResign()
        setShowResignConfirm(false)
      } catch (err) {
        console.error('Failed to resign:', err)
      } finally {
        setIsResigning(false)
      }
    }
  }

  const handleResignCancel = () => {
    setShowResignConfirm(false)
  }

  const isMyTurn = game && playerColor === game.currentTurn
  const waitingForOpponent = game?.status === 'waiting'
  const isActiveGame = game?.status === 'active'
  const isGameOver = game?.status === 'complete'

  // Determine game over message
  const getGameOverMessage = () => {
    if (!isGameOver || !game) return null
    if (game.winReason === 'resignation') {
      const winner = game.winner === playerColor ? 'You' : 'Opponent'
      return `${winner} won by resignation`
    }
    if (game.winReason === 'checkmate') {
      const winner = game.winner === playerColor ? 'You' : 'Opponent'
      return `${winner} won by checkmate`
    }
    return 'Game Over'
  }

  return (
    <div className="hud">
      <div className="hud-menu">
        {/* New Game button - shown when not in a session */}
        {!sessionId && (
          <button
            className="new-game-button"
            onClick={handleNewGameClick}
            disabled={isLoading}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
            </span>
            <span className="toggle-text">New Game</span>
          </button>
        )}

        {/* Resign button - shown during active game */}
        {sessionId && isActiveGame && (
          <button
            className="resign-button"
            onClick={handleResignClick}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
                <line x1="4" y1="22" x2="4" y2="15" />
              </svg>
            </span>
            <span className="toggle-text">Resign</span>
          </button>
        )}

        {/* Leave Game button - shown when in waiting or complete status */}
        {sessionId && (waitingForOpponent || isGameOver) && (
          <button
            className="leave-game-button"
            onClick={onLeaveGame}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                <polyline points="16 17 21 12 16 7" />
                <line x1="21" y1="12" x2="9" y2="12" />
              </svg>
            </span>
            <span className="toggle-text">Leave Game</span>
          </button>
        )}

        <button
          className={`toggle-button ${isTransitioning ? 'transitioning' : ''}`}
          onClick={onToggle}
          disabled={isTransitioning}
        >
          <span className="toggle-icon">
            {is3D ? (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="3" y="3" width="18" height="18" rx="2" />
                <line x1="3" y1="12" x2="21" y2="12" />
                <line x1="12" y1="3" x2="12" y2="21" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M12 2L2 7l10 5 10-5-10-5z" />
                <path d="M2 17l10 5 10-5" />
                <path d="M2 12l10 5 10-5" />
              </svg>
            )}
          </span>
          <span className="toggle-text">
            {isTransitioning ? 'Transitioning...' : (is3D ? 'Switch to 2D' : 'Switch to 3D')}
          </span>
        </button>

        <button
          className="settings-button"
          onClick={() => setShowSettings(true)}
        >
          <span className="toggle-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
          </span>
          <span className="toggle-text">Settings</span>
        </button>

        {auth.isAuthenticated ? (
          <button
            className="profile-button"
            onClick={() => console.log('TODO: Show profile modal')}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
            </span>
            <span className="toggle-text">{auth.user?.displayName || 'Profile'}</span>
          </button>
        ) : (
          <button
            className="login-button"
            onClick={() => setShowAuth(true)}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
                <polyline points="10 17 15 12 10 7" />
                <line x1="15" y1="12" x2="3" y2="12" />
              </svg>
            </span>
            <span className="toggle-text">Login / Sign Up</span>
          </button>
        )}

        <button
          className="about-button"
          onClick={() => setShowAbout(true)}
        >
          <span className="toggle-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="16" x2="12" y2="12" />
              <line x1="12" y1="8" x2="12.01" y2="8" />
            </svg>
          </span>
          <span className="toggle-text">About</span>
        </button>
      </div>

      {/* Game status panel */}
      {sessionId && (
        <div className="game-status-panel">
          <div className="game-status-header">
            <span className={`player-color ${playerColor}`}>
              Playing as {playerColor}
            </span>
            {isGameOver && (
              <span className="game-complete">{getGameOverMessage()}</span>
            )}
          </div>

          {waitingForOpponent ? (
            <div className="share-section">
              <p className="share-text">Waiting for opponent to join...</p>
              {shareLink && (
                <button className="copy-link-button" onClick={handleCopyLink}>
                  {copied ? 'Copied!' : 'Copy Invite Link'}
                </button>
              )}
            </div>
          ) : isActiveGame && (
            <div className="turn-indicator">
              {isMyTurn ? (
                <span className="your-turn">Your turn</span>
              ) : (
                <span className="opponent-turn">Waiting on opponent</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* Move history panel - always show when in game */}
      {sessionId && (
        <MoveHistory moves={moves} isMoving={isMoving} />
      )}

      {showSettings && (
        <SettingsModal
          settings={settings}
          updateSettings={updateSettings}
          onClose={() => setShowSettings(false)}
        />
      )}

      {showNewGame && (
        <NewGameModal
          isLoading={isLoading || false}
          error={error || null}
          sessionId={sessionId || null}
          playerId={playerId || null}
          shareLink={shareLink || null}
          onCreateGame={handleCreateGame}
          onClose={handleCloseNewGame}
        />
      )}

      {showResignConfirm && (
        <ResignConfirmModal
          onConfirm={handleResignConfirm}
          onCancel={handleResignCancel}
          isLoading={isResigning}
        />
      )}

      {showAbout && (
        <AboutModal onClose={() => setShowAbout(false)} />
      )}

      {showAuth && (
        <AuthModal
          onClose={() => setShowAuth(false)}
          onLogin={auth.login}
          onRegister={auth.register}
          onGoogleLogin={auth.loginWithGoogle}
        />
      )}
    </div>
  )
}
