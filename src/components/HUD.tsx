import { useState, useEffect } from 'react'
import { SettingsModal } from './SettingsModal'
import { MoveHistory } from './MoveHistory'
import { NewGameModal } from './NewGameModal'
import { EndGameModal } from './EndGameModal'
import { AboutModal } from './AboutModal'
import { LeaderboardModal } from './LeaderboardModal'
import { WatchGameModal } from './WatchGameModal'
import { AuthModal } from './AuthModal'
import { ProfileModal } from './ProfileModal'
import { GameSummaryModal } from './GameSummaryModal'
import { Clock, TimeControlBadge } from './Clock'
import { useAuth } from '../hooks/useAuth'
import type { Settings, SettingsContextType } from '../hooks/useSettings'
import type { Move, Game, DrawClaimReason } from '../api/gameApi'
import type { DrawOfferResult } from '../hooks/useOnlineGame'
import { DrawOfferModal } from './DrawOfferModal'
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
  onOfferDraw?: () => Promise<void>
  onClaimDraw?: (reason: DrawClaimReason) => Promise<boolean>
  onRespondToDraw?: (accept: boolean) => Promise<void>
  onClearDrawState?: () => void
  drawOfferPending?: boolean
  drawOfferReceived?: boolean
  drawOfferResult?: DrawOfferResult
  drawAutoDeclineMessage?: string | null
  onJoinGame?: (sessionId: string, existingPlayerId?: string) => Promise<unknown>
  sessionId?: string | null
  playerId?: string | null
  playerColor?: 'white' | 'black' | null
  game?: Game | null
  moves?: Move[]
  shareLink?: string | null
  isLoading?: boolean
  isMoving?: boolean
  error?: string | null
  isPlayerInCheck?: boolean
  // Watch mode props
  isWatchMode?: boolean
  watchGame?: Game | null
  watchMoves?: Move[]
  onStopWatching?: () => void
  onWatchGame?: (sessionId: string, isActive: boolean) => void
  onViewCompletedGame?: (sessionId: string) => void
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
  onOfferDraw,
  onClaimDraw,
  onRespondToDraw,
  onClearDrawState,
  drawOfferPending,
  drawOfferReceived,
  drawOfferResult,
  drawAutoDeclineMessage,
  onJoinGame,
  sessionId,
  playerId,
  playerColor,
  game,
  moves = [],
  shareLink,
  isLoading,
  isMoving,
  error,
  isPlayerInCheck,
  isWatchMode,
  watchGame: watchedGame,
  watchMoves: watchedMoves = [],
  onStopWatching,
  onWatchGame,
  onViewCompletedGame,
}: HUDProps) {
  const [showSettings, setShowSettings] = useState(false)
  const [showNewGame, setShowNewGame] = useState(false)
  const [showEndGame, setShowEndGame] = useState(false)
  const [showAbout, setShowAbout] = useState(false)
  const [showLeaderboard, setShowLeaderboard] = useState(false)
  const [showWatchGame, setShowWatchGame] = useState(false)
  const [showAuth, setShowAuth] = useState(false)
  const [showProfile, setShowProfile] = useState(false)
  const [showGameSummary, setShowGameSummary] = useState(false)
  const [isEndingGame, setIsEndingGame] = useState(false)
  const [copied, setCopied] = useState(false)
  const [prevGameStatus, setPrevGameStatus] = useState<string | null>(null)
  const [hasShownSummary, setHasShownSummary] = useState(false)

  const auth = useAuth()

  // Reset hasShownSummary when game changes (new session)
  useEffect(() => {
    if (!sessionId) {
      setHasShownSummary(false)
    }
  }, [sessionId])

  // Debug: Log whenever game or playerColor changes
  useEffect(() => {
    console.log('HUD props changed - game:', game?.status, 'playerColor:', playerColor, 'sessionId:', sessionId)
  }, [game, playerColor, sessionId])

  // Show game summary when game ends
  useEffect(() => {
    console.log('Game status effect:', { status: game?.status, prevGameStatus, hasShownSummary, showGameSummary })
    if (game?.status === 'complete' && (prevGameStatus === 'active' || prevGameStatus === null) && !hasShownSummary && !isWatchMode) {
      // Delay slightly to let state settle
      console.log('Triggering game summary modal')
      setHasShownSummary(true)
      setTimeout(() => setShowGameSummary(true), 500)
    }
    if (game?.status) {
      setPrevGameStatus(game.status)
    }
  }, [game?.status, prevGameStatus, hasShownSummary])

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

  const handleMatchFound = async (matchedSessionId: string, connectionId: string) => {
    if (onJoinGame) {
      try {
        // Pass connectionId as the playerId so the backend recognizes us
        await onJoinGame(matchedSessionId, connectionId)
        setShowNewGame(false)
      } catch (err) {
        console.error('Failed to join matched game:', err)
      }
    }
  }

  const handleEndGameClick = () => {
    setShowEndGame(true)
  }

  const handleResign = async () => {
    if (onResign) {
      setIsEndingGame(true)
      try {
        console.log('Before onResign - game:', game?.status, 'playerColor:', playerColor)
        await onResign()
        console.log('After onResign - game:', game?.status, 'playerColor:', playerColor)
        setShowEndGame(false)
        // Explicitly show game summary after resignation
        // The useEffect may not trigger reliably due to state batching
        setHasShownSummary(true)
        console.log('Setting showGameSummary to true in 300ms...')
        setTimeout(() => {
          console.log('Timeout fired - setting showGameSummary to true')
          setShowGameSummary(true)
        }, 300)
      } catch (err) {
        console.error('Failed to resign:', err)
      } finally {
        setIsEndingGame(false)
      }
    }
  }

  const handleOfferDraw = async () => {
    if (onOfferDraw) {
      try {
        await onOfferDraw()
      } catch (err) {
        console.error('Failed to offer draw:', err)
      }
    }
  }

  const handleClaimDraw = async (reason: DrawClaimReason): Promise<boolean> => {
    if (onClaimDraw) {
      setIsEndingGame(true)
      try {
        const success = await onClaimDraw(reason)
        if (success) {
          setShowEndGame(false)
          setHasShownSummary(true)
          setTimeout(() => setShowGameSummary(true), 300)
        }
        // If not successful, just close the loading state - error is shown via game error state
        return success
      } catch (err) {
        console.error('Failed to claim draw:', err)
        return false
      } finally {
        setIsEndingGame(false)
      }
    }
    return false
  }

  const handleEndGameCancel = () => {
    setShowEndGame(false)
  }

  const isMyTurn = game && playerColor === game.currentTurn
  const waitingForOpponent = game?.status === 'waiting'
  const isActiveGame = game?.status === 'active'
  const isGameOver = game?.status === 'complete'

  // Get opponent info
  const opponent = game?.players.find(p => p.color !== playerColor)
  const opponentName = opponent?.displayName || opponent?.agentName || 'Anonymous'

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
        {/* Stop Watching button - shown in watch mode */}
        {isWatchMode && (
          <button
            className="leave-game-button"
            onClick={onStopWatching}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                <polyline points="16 17 21 12 16 7" />
                <line x1="21" y1="12" x2="9" y2="12" />
              </svg>
            </span>
            <span className="toggle-text">Stop Watching</span>
          </button>
        )}

        {/* New Game button - shown when not in a session and not watching */}
        {!isWatchMode && !sessionId && (
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

        {/* End Game button - shown during active game */}
        {!isWatchMode && sessionId && isActiveGame && (
          <button
            className="end-game-button"
            onClick={handleEndGameClick}
          >
            <span className="toggle-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
            </span>
            <span className="toggle-text">End Game</span>
          </button>
        )}

        {/* Leave Game button - shown when waiting for opponent */}
        {!isWatchMode && sessionId && waitingForOpponent && (
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

        {/* New Game button - shown when game is over */}
        {!isWatchMode && sessionId && isGameOver && (
          <button
            className="new-game-button"
            onClick={() => {
              if (onLeaveGame) onLeaveGame()
              setShowNewGame(true)
            }}
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

        <button
          className="leaderboard-button"
          onClick={() => setShowLeaderboard(true)}
        >
          <span className="toggle-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M6 9H4.5a2.5 2.5 0 0 1 0-5C7.5 4 8 5.5 8 7" />
              <path d="M18 9h1.5a2.5 2.5 0 0 0 0-5C16.5 4 16 5.5 16 7" />
              <path d="M8 7v8a4 4 0 0 0 8 0V7" />
              <line x1="8" y1="21" x2="16" y2="21" />
              <line x1="12" y1="19" x2="12" y2="21" />
            </svg>
          </span>
          <span className="toggle-text">Leaderboard</span>
        </button>

        <button
          className="watch-button"
          onClick={() => setShowWatchGame(true)}
        >
          <span className="toggle-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          </span>
          <span className="toggle-text">Watch Game</span>
        </button>

        {auth.isAuthenticated ? (
          <button
            className="profile-button"
            onClick={() => setShowProfile(true)}
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

      {/* Watch mode status panel */}
      {isWatchMode && watchedGame && (
        <div className="game-status-panel">
          <div className="game-status-header">
            <div className="game-status-badges">
              <span className="watch-mode-badge">Watching</span>
              {watchedGame.isRanked && (
                <span className="ranked-badge">Ranked</span>
              )}
              {watchedGame.timeControl && watchedGame.timeControl.mode !== 'unlimited' && (
                <TimeControlBadge mode={watchedGame.timeControl.mode} />
              )}
            </div>
            <span className="watch-player-names">
              {watchedGame.players.find(p => p.color === 'white')?.displayName ||
               watchedGame.players.find(p => p.color === 'white')?.agentName || 'White'}
              {' vs '}
              {watchedGame.players.find(p => p.color === 'black')?.displayName ||
               watchedGame.players.find(p => p.color === 'black')?.agentName || 'Black'}
            </span>
          </div>
          {watchedGame.status === 'active' && (
            <div className="turn-indicator">
              <span className="opponent-turn">{watchedGame.currentTurn}'s turn</span>
            </div>
          )}
          {watchedGame.status === 'complete' && (
            <div className="turn-indicator">
              <span className="game-complete">
                {watchedGame.winner
                  ? `${watchedGame.players.find(p => p.color === watchedGame.winner)?.displayName ||
                       watchedGame.players.find(p => p.color === watchedGame.winner)?.agentName || watchedGame.winner} won`
                  : 'Draw'}
                {watchedGame.winReason ? ` by ${watchedGame.winReason.replace(/_/g, ' ')}` : ''}
              </span>
            </div>
          )}
        </div>
      )}

      {/* Game status panel */}
      {!isWatchMode && sessionId && (
        <div className="game-status-panel">
          <div className="game-status-header">
            <div className="game-status-badges">
              {game?.isRanked && (
                <span className="ranked-badge">Ranked</span>
              )}
              {game?.timeControl && game.timeControl.mode !== 'unlimited' && (
                <TimeControlBadge mode={game.timeControl.mode} />
              )}
            </div>
            <span className={`player-color ${playerColor}`}>
              Playing as {playerColor}
            </span>
            {!waitingForOpponent && (
              <span className="opponent-name">vs {opponentName}</span>
            )}
            {isGameOver && (
              <span className="game-complete">{getGameOverMessage()}</span>
            )}
          </div>

          {/* Clocks display - show when game has time control */}
          {game?.timeControl && game.timeControl.mode !== 'unlimited' && game?.playerTimes && !waitingForOpponent && (() => {
            // Calculate actual remaining time using serverTime
            // For the active player: actualRemaining = remainingMs - (serverTime - lastMoveAt)
            // For the inactive player: use remainingMs as-is
            const serverTime = game.serverTime || Date.now()
            const whiteTimes = game.playerTimes!.white
            const blackTimes = game.playerTimes!.black

            let whiteRemaining = whiteTimes.remainingMs
            let blackRemaining = blackTimes.remainingMs

            if (isActiveGame) {
              if (game.currentTurn === 'white' && whiteTimes.lastMoveAt > 0) {
                const elapsed = serverTime - whiteTimes.lastMoveAt
                whiteRemaining = Math.max(0, whiteTimes.remainingMs - elapsed)
              } else if (game.currentTurn === 'black' && blackTimes.lastMoveAt > 0) {
                const elapsed = serverTime - blackTimes.lastMoveAt
                blackRemaining = Math.max(0, blackTimes.remainingMs - elapsed)
              }
            }

            const opponentTime = playerColor === 'white' ? blackRemaining : whiteRemaining
            const myTime = playerColor === 'white' ? whiteRemaining : blackRemaining

            return (
              <div className="clocks-container">
                <Clock
                  timeMs={opponentTime}
                  isActive={isActiveGame && game.currentTurn !== playerColor}
                  label={opponentName}
                />
                <Clock
                  timeMs={myTime}
                  isActive={isActiveGame && game.currentTurn === playerColor}
                  label="You"
                  isPlayer
                />
              </div>
            )
          })()}

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
              {isPlayerInCheck && (
                <span className="check-warning">Your King is in Check!</span>
              )}
              {isMyTurn ? (
                <span className="your-turn">Your turn</span>
              ) : (
                <span className="opponent-turn">Waiting on opponent</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* Move history panel - show when in game or watching */}
      {isWatchMode && watchedGame && (
        <MoveHistory moves={watchedMoves} isMoving={false} />
      )}
      {!isWatchMode && sessionId && (
        <MoveHistory moves={moves} isMoving={isMoving} />
      )}

      {showSettings && (
        <SettingsModal
          settings={settings}
          updateSettings={updateSettings}
          onClose={() => setShowSettings(false)}
          isAuthenticated={auth.isAuthenticated}
          token={auth.token}
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
          onMatchFound={handleMatchFound}
          onClose={handleCloseNewGame}
          lastGameWasRanked={game?.isRanked}
        />
      )}

      {showEndGame && game && playerColor && (
        <EndGameModal
          game={game}
          playerColor={playerColor}
          onResign={handleResign}
          onOfferDraw={handleOfferDraw}
          onClaimDraw={handleClaimDraw}
          onCancel={handleEndGameCancel}
          isLoading={isEndingGame}
        />
      )}

      {/* Draw offer modal - shown when offering, receiving, or viewing result */}
      {(drawOfferPending || drawOfferReceived || drawOfferResult) && game && playerColor && (
        <DrawOfferModal
          drawOfferPending={drawOfferPending || false}
          drawOfferReceived={drawOfferReceived || false}
          drawOfferResult={drawOfferResult || null}
          drawAutoDeclineMessage={drawAutoDeclineMessage || null}
          game={game}
          playerColor={playerColor}
          onRespondToDraw={onRespondToDraw || (async () => {})}
          onDismiss={onClearDrawState || (() => {})}
        />
      )}

      {showAbout && (
        <AboutModal onClose={() => setShowAbout(false)} />
      )}

      {showLeaderboard && (
        <LeaderboardModal onClose={() => setShowLeaderboard(false)} />
      )}

      {showWatchGame && onWatchGame && (
        <WatchGameModal
          onClose={() => setShowWatchGame(false)}
          onWatchGame={onWatchGame}
        />
      )}

      {showAuth && (
        <AuthModal
          onClose={() => setShowAuth(false)}
          onLogin={auth.login}
          onRegister={auth.register}
          onGoogleLogin={auth.loginWithGoogle}
        />
      )}

      {showProfile && auth.user && auth.token && (
        <ProfileModal
          user={auth.user}
          token={auth.token}
          onClose={() => setShowProfile(false)}
          onLogout={() => {
            setShowProfile(false)
            auth.logout()
          }}
          onViewGame={onViewCompletedGame ? (sessionId) => {
            setShowProfile(false)
            onViewCompletedGame(sessionId)
          } : undefined}
        />
      )}

      {(() => {
        console.log('GameSummary render check:', { showGameSummary, hasGame: !!game, playerColor, gameStatus: game?.status })
        return null
      })()}
      {showGameSummary && game && playerColor && (
        <GameSummaryModal
          game={game}
          moves={moves}
          playerColor={playerColor}
          onClose={() => {
            setShowGameSummary(false)
            // Clean up game state when closing summary after game over
            if (onLeaveGame) onLeaveGame()
          }}
          onNewGame={() => {
            setShowGameSummary(false)
            if (onLeaveGame) onLeaveGame()
            setShowNewGame(true)
          }}
        />
      )}
    </div>
  )
}
