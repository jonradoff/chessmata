import { useState, useCallback, useEffect, useMemo } from 'react'
import { Canvas } from '@react-three/fiber'
import { ChessScene } from './components/ChessScene'
import { HUD } from './components/HUD'
import { ConnectionErrorModal } from './components/ConnectionErrorModal'
import { InvalidMoveModal } from './components/InvalidMoveModal'
import { PromotionModal } from './components/PromotionModal'
import type { PendingPromotion } from './components/ChessScene'
import { SessionRestoringModal } from './components/SessionRestoringModal'
import { VerifyEmailPage } from './components/VerifyEmailPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { useGameState } from './hooks/useGameState'
import { useSettingsState } from './hooks/useSettings'
import { useOnlineGame } from './hooks/useOnlineGame'
import { useGameViewer } from './hooks/useGameViewer'
import { isInCheck, fromAlgebraic } from './utils/chessRules'
import type { PieceType } from './utils/chessRules'
import { parseFENState } from './utils/fenParser'
import './App.css'

// Simple URL routing helper
function useUrlRoute() {
  const [route, setRoute] = useState(() => {
    const path = window.location.pathname
    const params = new URLSearchParams(window.location.search)
    return { path, params }
  })

  useEffect(() => {
    const handlePopState = () => {
      const path = window.location.pathname
      const params = new URLSearchParams(window.location.search)
      setRoute({ path, params })
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const navigate = useCallback((path: string) => {
    window.history.pushState({}, '', path)
    const params = new URLSearchParams(window.location.search)
    setRoute({ path, params })
  }, [])

  return { route, navigate }
}

function App() {
  const [is3D, setIs3D] = useState(true)
  const [isTransitioning, setIsTransitioning] = useState(false)
  const [invalidMove, setInvalidMove] = useState<{ reason: string; pieceType: PieceType } | null>(null)
  const [pendingPromotion, setPendingPromotion] = useState<PendingPromotion | null>(null)
  const [showVerificationSuccess, setShowVerificationSuccess] = useState(false)
  const gameState = useGameState()
  const { settings, updateSettings } = useSettingsState()
  const onlineGame = useOnlineGame()
  const gameViewer = useGameViewer()
  const { route, navigate } = useUrlRoute()

  // Determine which page to show based on URL
  const pageState = useMemo(() => {
    const path = route.path
    const token = route.params.get('token')
    // OAuth tokens come via URL fragment (hash), not query params
    const hashParams = new URLSearchParams(window.location.hash.substring(1))
    const accessToken = hashParams.get('access_token')
    const refreshToken = hashParams.get('refresh_token')

    if (path === '/verify-email' && token) {
      return { page: 'verify-email' as const, token, accessToken: null, refreshToken: null }
    }
    if (path === '/reset-password' && token) {
      return { page: 'reset-password' as const, token, accessToken: null, refreshToken: null }
    }
    if (path === '/auth/callback' && accessToken) {
      return { page: 'auth-callback' as const, token: null, accessToken, refreshToken }
    }
    return { page: 'main' as const, token: null, accessToken: null, refreshToken: null }
  }, [route])

  // Sync board state from server whenever game updates (online or watched)
  useEffect(() => {
    const boardState = gameViewer.game?.boardState || onlineGame.game?.boardState
    if (boardState) {
      try {
        gameState.syncFromFEN(boardState)
      } catch (err) {
        console.error('Failed to parse FEN:', err)
      }
    }
  }, [gameViewer.game?.boardState, onlineGame.game?.boardState, gameState.syncFromFEN])

  // Show arrow for last move (opponent moves in play mode, all moves in watch mode)
  useEffect(() => {
    const move = onlineGame.lastOpponentMove || gameViewer.lastMove
    if (move) {
      const from = fromAlgebraic(move.from)
      const to = fromAlgebraic(move.to)
      gameState.setLastMoveArrow({ from, to })
    } else {
      gameState.setLastMoveArrow(null)
    }
  }, [onlineGame.lastOpponentMove, gameViewer.lastMove, gameState.setLastMoveArrow])

  const handleToggle = useCallback(() => {
    if (isTransitioning) return
    setIsTransitioning(true)
    setIs3D(prev => !prev)
  }, [isTransitioning])

  const handleTransitionComplete = useCallback(() => {
    setIsTransitioning(false)
  }, [])

  // Compute if the current player's king is in check
  const isPlayerInCheck = useMemo(() => {
    const playerColor = onlineGame.playerColor
    if (!playerColor || !gameState.pieces.length) return false

    const fenState = parseFENState(gameState.boardFEN)
    const board = {
      pieces: fenState.pieces.map(p => ({
        type: p.type,
        isWhite: p.isWhite,
        file: p.file,
        rank: p.rank,
      })),
      whiteToMove: fenState.whiteToMove,
      castlingRights: fenState.castlingRights,
      enPassantSquare: fenState.enPassantSquare,
    }

    const isWhitePlayer = playerColor === 'white'
    return isInCheck(board, isWhitePlayer)
  }, [gameState.pieces, gameState.boardFEN, onlineGame.playerColor])

  // Handle promotion piece selection
  const handlePromotionSelect = async (piece: 'queen' | 'rook' | 'bishop' | 'knight') => {
    if (!pendingPromotion) return
    const { from, to, pieceId, toFile, toRank } = pendingPromotion
    setPendingPromotion(null)

    // Optimistic move
    gameState.movePiece(pieceId, toFile, toRank)

    const promotionMap: Record<string, string> = { queen: 'q', rook: 'r', bishop: 'b', knight: 'n' }
    const promotion = promotionMap[piece] || 'q'

    try {
      const response = await onlineGame.makeMove(from, to, promotion)
      if (!response.success) {
        console.error('Promotion move rejected by server:', response.error)
      }
    } catch (err) {
      console.error('Failed to make promotion move:', err)
    }
  }

  const isWatchMode = gameViewer.isWatching || gameViewer.isViewing

  // Online game context for the chess scene (memoized to prevent unnecessary re-renders)
  const onlineContext = useMemo(() => isWatchMode ? {
    sessionId: gameViewer.sessionId,
    playerId: null as string | null,
    playerColor: null as 'white' | 'black' | null,
    currentTurn: gameViewer.game?.currentTurn || null,
    gameStatus: gameViewer.game?.status || null,
    makeMove: undefined,
    onInvalidMove: setInvalidMove,
    onPendingPromotion: setPendingPromotion,
  } : {
    sessionId: onlineGame.sessionId,
    playerId: onlineGame.playerId,
    playerColor: onlineGame.playerColor,
    currentTurn: onlineGame.game?.currentTurn || null,
    gameStatus: onlineGame.game?.status || null,
    makeMove: onlineGame.makeMove,
    onInvalidMove: setInvalidMove,
    onPendingPromotion: setPendingPromotion,
  }, [
    isWatchMode,
    gameViewer.sessionId, gameViewer.game?.currentTurn, gameViewer.game?.status,
    onlineGame.sessionId, onlineGame.playerId, onlineGame.playerColor,
    onlineGame.game?.currentTurn, onlineGame.game?.status, onlineGame.makeMove,
  ])

  // Handle email verification page
  if (pageState.page === 'verify-email' && pageState.token) {
    return (
      <VerifyEmailPage
        token={pageState.token}
        onSuccess={() => {
          navigate('/')
          setShowVerificationSuccess(true)
          setTimeout(() => setShowVerificationSuccess(false), 5000)
        }}
        onBackToHome={() => navigate('/')}
      />
    )
  }

  // Handle password reset page
  if (pageState.page === 'reset-password' && pageState.token) {
    return (
      <ResetPasswordPage
        token={pageState.token}
        onSuccess={() => navigate('/')}
        onBackToHome={() => navigate('/')}
      />
    )
  }

  return (
    <div className="app">
      <Canvas
        shadows
        camera={{ position: [0, 8, 12], fov: 45 }}
      >
        <ChessScene
          is3D={is3D}
          isTransitioning={isTransitioning}
          onTransitionComplete={handleTransitionComplete}
          gameState={gameState}
          settings={settings}
          onlineContext={onlineContext}
        />
      </Canvas>
      <HUD
        is3D={is3D}
        onToggle={handleToggle}
        isTransitioning={isTransitioning}
        settings={settings}
        updateSettings={updateSettings}
        onNewGame={onlineGame.createGame}
        onLeaveGame={onlineGame.leaveGame}
        onResign={onlineGame.resignGame}
        onOfferDraw={onlineGame.offerDraw}
        onClaimDraw={onlineGame.claimDraw}
        onRespondToDraw={onlineGame.respondToDraw}
        onClearDrawState={onlineGame.clearDrawState}
        drawOfferPending={onlineGame.drawOfferPending}
        drawOfferReceived={onlineGame.drawOfferReceived}
        drawOfferResult={onlineGame.drawOfferResult}
        drawAutoDeclineMessage={onlineGame.drawAutoDeclineMessage}
        onJoinGame={onlineGame.joinGame}
        sessionId={onlineGame.sessionId}
        playerId={onlineGame.playerId}
        playerColor={onlineGame.playerColor}
        game={onlineGame.game}
        moves={onlineGame.moves}
        shareLink={onlineGame.shareLink}
        isLoading={onlineGame.isLoading}
        isMoving={onlineGame.isMoving}
        error={onlineGame.error}
        isPlayerInCheck={isPlayerInCheck}
        isWatchMode={isWatchMode}
        watchGame={gameViewer.game}
        watchMoves={gameViewer.moves}
        onStopWatching={gameViewer.stopWatching}
        onWatchGame={(sessionId, isActive) =>
          isActive ? gameViewer.watchGame(sessionId) : gameViewer.viewCompletedGame(sessionId)
        }
        onViewCompletedGame={gameViewer.viewCompletedGame}
        onTimeExpired={onlineGame.refreshGame}
      />
      {onlineGame.connectionError && (
        <ConnectionErrorModal
          error={onlineGame.connectionError}
          onClose={onlineGame.clearConnectionError}
        />
      )}
      {invalidMove && (
        <InvalidMoveModal
          reason={invalidMove.reason}
          pieceType={invalidMove.pieceType}
          onClose={() => setInvalidMove(null)}
        />
      )}
      {pendingPromotion && (
        <PromotionModal
          isWhite={pendingPromotion.isWhite}
          onSelect={handlePromotionSelect}
        />
      )}
      {onlineGame.isRestoring && (
        <SessionRestoringModal error={onlineGame.connectionError} />
      )}
      {showVerificationSuccess && (
        <div className="verification-success-toast">
          <div className="toast-content">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
            <span>Email verified successfully! Welcome to Chessmata.</span>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
