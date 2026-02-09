import { useState, useCallback, useEffect } from 'react'
import { Canvas } from '@react-three/fiber'
import { ChessScene } from './components/ChessScene'
import { HUD } from './components/HUD'
import { ConnectionErrorModal } from './components/ConnectionErrorModal'
import { InvalidMoveModal } from './components/InvalidMoveModal'
import { SessionRestoringModal } from './components/SessionRestoringModal'
import { useGameState } from './hooks/useGameState'
import { useSettingsState } from './hooks/useSettings'
import { useOnlineGame } from './hooks/useOnlineGame'
import { parseFEN } from './utils/fenParser'
import type { PieceType } from './utils/chessRules'
import './App.css'

function App() {
  const [is3D, setIs3D] = useState(true)
  const [isTransitioning, setIsTransitioning] = useState(false)
  const [invalidMove, setInvalidMove] = useState<{ reason: string; pieceType: PieceType } | null>(null)
  const gameState = useGameState()
  const { settings, updateSettings } = useSettingsState()
  const onlineGame = useOnlineGame()

  // Debug: Log isRestoring state
  useEffect(() => {
    console.log('App.tsx - isRestoring:', onlineGame.isRestoring, 'sessionId:', onlineGame.sessionId)
  }, [onlineGame.isRestoring, onlineGame.sessionId])

  // Sync board state from server whenever game updates
  useEffect(() => {
    if (onlineGame.game?.boardState) {
      try {
        const pieces = parseFEN(onlineGame.game.boardState)
        gameState.syncFromPieces(pieces)
      } catch (err) {
        console.error('Failed to parse FEN:', err)
      }
    }
  }, [onlineGame.game?.boardState, gameState.syncFromPieces])

  const handleToggle = useCallback(() => {
    if (isTransitioning) return
    setIsTransitioning(true)
    setIs3D(prev => !prev)
  }, [isTransitioning])

  const handleTransitionComplete = useCallback(() => {
    setIsTransitioning(false)
  }, [])

  // Online game context for the chess scene
  const onlineContext = {
    sessionId: onlineGame.sessionId,
    playerId: onlineGame.playerId,
    playerColor: onlineGame.playerColor,
    currentTurn: onlineGame.game?.currentTurn || null,
    gameStatus: onlineGame.game?.status || null,
    makeMove: onlineGame.makeMove,
    onInvalidMove: setInvalidMove,
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
        sessionId={onlineGame.sessionId}
        playerId={onlineGame.playerId}
        playerColor={onlineGame.playerColor}
        game={onlineGame.game}
        moves={onlineGame.moves}
        shareLink={onlineGame.shareLink}
        isLoading={onlineGame.isLoading}
        isMoving={onlineGame.isMoving}
        error={onlineGame.error}
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
      {(() => {
        console.log('Checking SessionRestoringModal condition - isRestoring:', onlineGame.isRestoring)
        return onlineGame.isRestoring && (
          <SessionRestoringModal error={onlineGame.connectionError} />
        )
      })()}
    </div>
  )
}

export default App
