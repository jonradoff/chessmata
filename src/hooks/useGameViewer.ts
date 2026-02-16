import { useState, useCallback, useRef, useEffect } from 'react'
import { getGame, getMoves, type Game, type Move } from '../api/gameApi'
import { WS_BASE } from '../api/config'

export interface GameViewerState {
  sessionId: string | null
  game: Game | null
  moves: Move[]
  isWatching: boolean
  isViewing: boolean
  isLoading: boolean
  error: string | null
  lastMove: { from: string; to: string } | null
}

export function useGameViewer() {
  const [state, setState] = useState<GameViewerState>({
    sessionId: null,
    game: null,
    moves: [],
    isWatching: false,
    isViewing: false,
    isLoading: false,
    error: null,
    lastMove: null,
  })

  const wsRef = useRef<WebSocket | null>(null)

  const cleanup = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  useEffect(() => {
    return cleanup
  }, [cleanup])

  const watchGame = useCallback(async (sessionId: string) => {
    cleanup()
    setState({
      sessionId,
      game: null,
      moves: [],
      isWatching: true,
      isViewing: false,
      isLoading: true,
      error: null,
      lastMove: null,
    })

    try {
      const [game, movesData] = await Promise.all([
        getGame(sessionId),
        getMoves(sessionId),
      ])

      const moves = movesData.moves || []
      const lastMoveData = moves.length > 0 ? moves[moves.length - 1] : null

      setState(prev => ({
        ...prev,
        game,
        moves,
        lastMove: lastMoveData?.from && lastMoveData?.to ? { from: lastMoveData.from, to: lastMoveData.to } : null,
        isLoading: false,
      }))

      // Connect as spectator
      const ws = new WebSocket(`${WS_BASE}/games/${sessionId}?spectator=true`)

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          if (data.type === 'move' || data.type === 'game_update' || data.type === 'player_joined') {
            if (data.game) {
              setState(prev => ({ ...prev, game: data.game }))
            }
            if (data.move) {
              setState(prev => ({
                ...prev,
                moves: [...prev.moves, data.move],
                lastMove: data.move.from && data.move.to ? { from: data.move.from, to: data.move.to } : null,
              }))
            }
          } else if (data.type === 'game_over') {
            if (data.game) {
              setState(prev => ({
                ...prev,
                game: data.game,
                isWatching: false,
                isViewing: true,
              }))
            }
            ws.close()
          } else if (data.type === 'resignation') {
            if (data.game) {
              setState(prev => ({
                ...prev,
                game: data.game,
                isWatching: false,
                isViewing: true,
              }))
            }
            ws.close()
          } else if (data.type === 'draw_offered' || data.type === 'draw_declined') {
            if (data.game) {
              setState(prev => ({ ...prev, game: data.game }))
            }
          }
        } catch (err) {
          console.error('Spectator WS message parse error:', err)
        }
      }

      ws.onerror = () => {}

      ws.onclose = () => {}

      wsRef.current = ws
    } catch (err) {
      setState(prev => ({
        ...prev,
        isLoading: false,
        isWatching: false,
        error: err instanceof Error ? err.message : 'Failed to load game',
      }))
    }
  }, [cleanup])

  const viewCompletedGame = useCallback(async (sessionId: string) => {
    cleanup()
    setState({
      sessionId,
      game: null,
      moves: [],
      isWatching: false,
      isViewing: true,
      isLoading: true,
      error: null,
      lastMove: null,
    })

    try {
      const [game, movesData] = await Promise.all([
        getGame(sessionId),
        getMoves(sessionId),
      ])

      const moves = movesData.moves || []
      const lastMoveData = moves.length > 0 ? moves[moves.length - 1] : null

      setState(prev => ({
        ...prev,
        game,
        moves,
        lastMove: lastMoveData?.from && lastMoveData?.to ? { from: lastMoveData.from, to: lastMoveData.to } : null,
        isLoading: false,
      }))
    } catch (err) {
      setState(prev => ({
        ...prev,
        isLoading: false,
        isViewing: false,
        error: err instanceof Error ? err.message : 'Failed to load game',
      }))
    }
  }, [cleanup])

  const stopWatching = useCallback(() => {
    cleanup()
    setState({
      sessionId: null,
      game: null,
      moves: [],
      isWatching: false,
      isViewing: false,
      isLoading: false,
      error: null,
      lastMove: null,
    })
  }, [cleanup])

  return {
    ...state,
    watchGame,
    viewCompletedGame,
    stopWatching,
  }
}
