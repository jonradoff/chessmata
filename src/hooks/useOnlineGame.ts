import { useState, useCallback, useEffect, useRef } from 'react'
import * as api from '../api/gameApi'

export interface OnlineGameState {
  sessionId: string | null
  playerId: string | null
  playerColor: 'white' | 'black' | null
  game: api.Game | null
  moves: api.Move[]
  isLoading: boolean
  isMoving: boolean
  isRestoring: boolean
  error: string | null
  connectionError: string | null
  shareLink: string | null
}

const STORAGE_KEY = 'chess_game_session'
const WS_BASE = import.meta.env.VITE_WS_URL || 'ws://localhost:9029/ws'

interface StoredSession {
  sessionId: string
  playerId: string
}

function getStoredSession(): StoredSession | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      return JSON.parse(stored)
    }
  } catch {
    // Ignore parse errors
  }
  return null
}

function storeSession(sessionId: string, playerId: string) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ sessionId, playerId }))
}

function clearStoredSession() {
  localStorage.removeItem(STORAGE_KEY)
}

export function useOnlineGame() {
  const [state, setState] = useState<OnlineGameState>({
    sessionId: null,
    playerId: null,
    playerColor: null,
    game: null,
    moves: [],
    isLoading: false,
    isMoving: false,
    isRestoring: false,
    error: null,
    connectionError: null,
    shareLink: null,
  })

  const hasInitialized = useRef(false)
  const wsRef = useRef<WebSocket | null>(null)
  const stateRef = useRef(state)
  stateRef.current = state
  const restoringStartTime = useRef<number | null>(null)

  // WebSocket connection management
  const connectWebSocket = useCallback((sessionId: string, playerId: string) => {
    // Close existing connection
    if (wsRef.current) {
      wsRef.current.close()
    }

    const ws = new WebSocket(`${WS_BASE}/games/${sessionId}?playerId=${playerId}`)

    ws.onopen = () => {
      console.log('WebSocket connected')
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)

        if (message.type === 'game_update') {
          setState(prev => ({
            ...prev,
            game: message.game,
          }))
        } else if (message.type === 'move') {
          setState(prev => ({
            ...prev,
            game: message.game,
            moves: [...prev.moves, message.move],
          }))
        } else if (message.type === 'player_joined') {
          setState(prev => ({
            ...prev,
            game: message.game,
          }))
        } else if (message.type === 'resignation') {
          // Opponent resigned - update game state
          setState(prev => ({
            ...prev,
            game: message.game,
          }))
        }
      } catch (err) {
        console.error('WebSocket message parse error:', err)
      }
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    wsRef.current = ws
  }, [])

  const disconnectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  const createGame = useCallback(async () => {
    console.log('Creating new game...')
    setState(prev => ({ ...prev, isLoading: true, error: null }))
    try {
      console.log('Calling API createGame...')
      const response = await api.createGame()
      console.log('Create game response:', response)
      const fullShareLink = `${window.location.origin}/game/${response.sessionId}`

      storeSession(response.sessionId, response.playerId)

      // Update URL without reload
      window.history.pushState({}, '', `/game/${response.sessionId}`)

      // Fetch the created game
      console.log('Fetching game details...')
      const game = await api.getGame(response.sessionId)
      console.log('Game details:', game)

      setState(prev => ({
        ...prev,
        sessionId: response.sessionId,
        playerId: response.playerId,
        playerColor: 'white',
        shareLink: fullShareLink,
        isLoading: false,
        game,
        moves: [],
      }))

      // Connect WebSocket
      console.log('Connecting WebSocket...')
      connectWebSocket(response.sessionId, response.playerId)

      return response
    } catch (err) {
      console.error('Create game error:', err)
      const errorMessage = err instanceof Error ? err.message : 'Failed to create game'
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: errorMessage,
      }))
      throw err
    }
  }, [connectWebSocket])

  const joinGame = useCallback(async (sessionId: string, existingPlayerId?: string) => {
    console.log('Attempting to join game:', { sessionId, existingPlayerId })
    setState(prev => ({ ...prev, isLoading: true, error: null, connectionError: null }))
    try {
      const response = await api.joinGame(sessionId, existingPlayerId)
      console.log('Join game response:', response)
      const fullShareLink = `${window.location.origin}/game/${response.sessionId}`

      storeSession(response.sessionId, response.playerId)

      // Update URL without reload
      window.history.pushState({}, '', `/game/${response.sessionId}`)

      // Fetch moves
      const movesResponse = await api.getMoves(sessionId)
      console.log('Fetched moves:', movesResponse)

      // If we started restoring, ensure modal shows for at least 800ms
      const minDisplayTime = 800
      if (restoringStartTime.current) {
        const elapsed = Date.now() - restoringStartTime.current
        console.log('Restoration took:', elapsed, 'ms. Minimum display time:', minDisplayTime, 'ms')
        if (elapsed < minDisplayTime) {
          const waitTime = minDisplayTime - elapsed
          console.log('Waiting additional', waitTime, 'ms to show modal')
          await new Promise(resolve => setTimeout(resolve, waitTime))
        }
        restoringStartTime.current = null
      }

      setState(prev => ({
        ...prev,
        sessionId: response.sessionId,
        playerId: response.playerId,
        playerColor: response.color,
        game: response.game,
        moves: movesResponse.moves || [],
        shareLink: fullShareLink,
        isLoading: false,
        isRestoring: false,
      }))

      // Connect WebSocket
      connectWebSocket(response.sessionId, response.playerId)

      return response
    } catch (err) {
      console.error('Failed to join game:', err)
      const errorMessage = err instanceof Error ? err.message : 'Failed to join game'

      // Ensure minimum display time for error state too
      const minDisplayTime = 800
      if (restoringStartTime.current) {
        const elapsed = Date.now() - restoringStartTime.current
        if (elapsed < minDisplayTime) {
          await new Promise(resolve => setTimeout(resolve, minDisplayTime - elapsed))
        }
        restoringStartTime.current = null
      }

      setState(prev => ({
        ...prev,
        isLoading: false,
        isRestoring: false,
        connectionError: errorMessage,
      }))
      throw err
    }
  }, [connectWebSocket])

  const makeMove = useCallback(async (from: string, to: string, promotion?: string) => {
    if (!state.sessionId || !state.playerId) {
      throw new Error('Not in a game')
    }

    // Create optimistic move to show immediately
    const optimisticMove: api.Move = {
      id: 'optimistic-' + Date.now(),
      gameId: state.game?.id || '',
      sessionId: state.sessionId,
      playerId: state.playerId,
      moveNumber: state.moves.length + 1,
      from,
      to,
      piece: '', // We don't know the exact piece type yet
      notation: `${from}-${to}`, // Simplified notation for now
      capture: false,
      check: false,
      checkmate: false,
      promotion,
      createdAt: new Date().toISOString(),
    }

    // Optimistically add the move to history
    setState(prev => ({
      ...prev,
      isMoving: true,
      moves: [...prev.moves, optimisticMove]
    }))

    try {
      const response = await api.makeMove(
        state.sessionId,
        state.playerId,
        from,
        to,
        promotion
      )

      if (!response.success) {
        // Remove optimistic move and show error
        setState(prev => ({
          ...prev,
          error: response.error || 'Invalid move',
          isMoving: false,
          moves: prev.moves.filter(m => m.id !== optimisticMove.id)
        }))
        return response
      }

      // Refresh game state and moves (this will replace the optimistic move with real data)
      const [game, movesResponse] = await Promise.all([
        api.getGame(state.sessionId),
        api.getMoves(state.sessionId),
      ])

      console.log('After move - game.currentTurn:', game.currentTurn, 'playerColor:', state.playerColor)

      setState(prev => ({
        ...prev,
        game,
        moves: movesResponse.moves,
        error: null,
        isMoving: false,
      }))

      return response
    } catch (err) {
      // Remove optimistic move on error
      setState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : 'Failed to make move',
        isMoving: false,
        moves: prev.moves.filter(m => m.id !== optimisticMove.id)
      }))
      throw err
    }
  }, [state.sessionId, state.playerId, state.game?.id, state.moves.length])

  const refreshGame = useCallback(async () => {
    if (!state.sessionId) return

    try {
      const [game, movesResponse] = await Promise.all([
        api.getGame(state.sessionId),
        api.getMoves(state.sessionId),
      ])

      setState(prev => ({
        ...prev,
        game,
        moves: movesResponse.moves,
      }))
    } catch (err) {
      // Silently fail refresh
    }
  }, [state.sessionId])

  const resignGame = useCallback(async () => {
    const { sessionId, playerId } = stateRef.current
    if (!sessionId || !playerId) {
      console.log('Cannot resign: missing sessionId or playerId', { sessionId, playerId })
      return
    }

    console.log('Resigning game:', { sessionId, playerId })

    try {
      const response = await api.resignGame(sessionId, playerId)
      console.log('Resign response:', response)
      if (response.success) {
        // Clear session and return to initial state
        disconnectWebSocket()
        clearStoredSession()
        window.history.pushState({}, '', '/')
        setState({
          sessionId: null,
          playerId: null,
          playerColor: null,
          game: null,
          moves: [],
          isLoading: false,
          isMoving: false,
          isRestoring: false,
          error: null,
          connectionError: null,
          shareLink: null,
        })
      } else {
        console.error('Resign failed:', response.error)
        setState(prev => ({ ...prev, error: response.error || 'Failed to resign' }))
      }
    } catch (err) {
      console.error('Resign error:', err)
      setState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : 'Failed to resign',
      }))
    }
  }, [disconnectWebSocket])

  const leaveGame = useCallback(() => {
    disconnectWebSocket()
    clearStoredSession()
    window.history.pushState({}, '', '/')
    setState({
      sessionId: null,
      playerId: null,
      playerColor: null,
      game: null,
      moves: [],
      isLoading: false,
      isMoving: false,
      isRestoring: false,
      error: null,
      connectionError: null,
      shareLink: null,
    })
  }, [disconnectWebSocket])

  const clearError = useCallback(() => {
    setState(prev => ({ ...prev, error: null }))
  }, [])

  const clearConnectionError = useCallback(() => {
    setState(prev => ({ ...prev, connectionError: null }))
    // If there's a connection error, navigate back to home
    window.history.pushState({}, '', '/')
  }, [])

  // Store joinGame in a ref so we can call it from effect without dependency
  const joinGameRef = useRef(joinGame)
  joinGameRef.current = joinGame

  // Check URL for session ID on mount
  useEffect(() => {
    if (hasInitialized.current) return
    hasInitialized.current = true

    console.log('=== useOnlineGame initialization ===')
    const path = window.location.pathname
    console.log('Current path:', path)
    const match = path.match(/\/game\/([a-f0-9]+)/)
    if (match) {
      const urlSessionId = match[1]
      console.log('Found session ID in URL:', urlSessionId)
      const stored = getStoredSession()
      console.log('Stored session:', stored)

      // Set restoring state before attempting to join
      restoringStartTime.current = Date.now()
      setState(prev => ({ ...prev, isRestoring: true }))

      if (stored && stored.sessionId === urlSessionId) {
        // Rejoin existing session
        console.log('Rejoining existing session')
        joinGameRef.current(urlSessionId, stored.playerId)
      } else {
        // Join as new player
        console.log('Joining as new player')
        joinGameRef.current(urlSessionId)
      }
    } else {
      // Check for stored session
      console.log('No session ID in URL, checking localStorage')
      const stored = getStoredSession()
      if (stored) {
        console.log('Found stored session, rejoining:', stored)
        restoringStartTime.current = Date.now()
        setState(prev => ({ ...prev, isRestoring: true }))
        joinGameRef.current(stored.sessionId, stored.playerId)
      } else {
        console.log('No stored session found')
      }
    }
  }, [])

  // Cleanup WebSocket on unmount
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [])

  return {
    ...state,
    createGame,
    joinGame,
    makeMove,
    refreshGame,
    leaveGame,
    resignGame,
    clearError,
    clearConnectionError,
  }
}
