import { useState, useCallback, useEffect, useRef } from 'react'
import * as api from '../api/gameApi'

export type DrawOfferResult = 'accepted' | 'declined' | 'auto_declined' | null

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
  lastOpponentMove: { from: string; to: string } | null
  // Draw offer state
  drawOfferPending: boolean        // true while our draw offer is pending (waiting for response)
  drawOfferReceived: boolean       // true when opponent offered us a draw
  drawOfferResult: DrawOfferResult // result after response
  drawAutoDeclineMessage: string | null
}

const STORAGE_KEY = 'chess_game_session'
// In production, use relative URLs based on current protocol/host
const getWsBase = () => {
  if (import.meta.env.VITE_WS_URL) return import.meta.env.VITE_WS_URL
  if (window.location.hostname === 'localhost') return 'ws://localhost:9029/ws'
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${wsProtocol}//${window.location.host}/ws`
}
const WS_BASE = getWsBase()

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
    lastOpponentMove: null,
    drawOfferPending: false,
    drawOfferReceived: false,
    drawOfferResult: null,
    drawAutoDeclineMessage: null,
  })

  const hasInitialized = useRef(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const wsSessionRef = useRef<{ sessionId: string; playerId: string } | null>(null)
  const stateRef = useRef(state)
  stateRef.current = state
  const restoringStartTime = useRef<number | null>(null)
  const mountedRef = useRef(true)
  const reconnectAttemptRef = useRef(0)

  // WebSocket connection management
  const connectWebSocket = useCallback((sessionId: string, playerId: string) => {
    // Close existing connection and cancel pending reconnect
    if (wsRef.current) {
      wsRef.current.close()
    }
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }

    wsSessionRef.current = { sessionId, playerId }

    // Include auth token for authenticated player connections
    const token = localStorage.getItem('chessmata_auth_token')
    let wsUrl = `${WS_BASE}/games/${sessionId}?playerId=${playerId}`
    if (token) {
      wsUrl += `&token=${encodeURIComponent(token)}`
    }
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('WebSocket connected')
      reconnectAttemptRef.current = 0
      // On reconnect, refresh game state to catch anything we missed
      if (stateRef.current.game) {
        Promise.all([
          api.getGame(sessionId),
          api.getMoves(sessionId),
        ]).then(([game, movesResponse]) => {
          const moves = movesResponse.moves || []
          // Determine last opponent move for arrow
          let lastOpponentMove: { from: string; to: string } | null = null
          if (moves.length > 0) {
            const lastMove = moves[moves.length - 1]
            if (lastMove.playerId !== playerId && lastMove.from && lastMove.to) {
              lastOpponentMove = { from: lastMove.from, to: lastMove.to }
            }
          }
          setState(prev => {
            // Only update if fetched data is at least as current
            if (prev.moves.length > moves.length) return prev
            return {
              ...prev,
              game: { ...game, serverTime: Date.now() },
              moves,
              lastOpponentMove: lastOpponentMove || prev.lastOpponentMove,
            }
          })
        }).catch(() => {
          // Silently fail — we're reconnected, WS will deliver updates
        })
      }
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)

        // Include serverTime with game data for clock sync
        const gameWithServerTime = message.game ? {
          ...message.game,
          serverTime: message.serverTime || Date.now(),
        } : null

        if (message.type === 'game_update') {
          setState(prev => ({
            ...prev,
            game: gameWithServerTime,
          }))
        } else if (message.type === 'move') {
          const move = message.move
          setState(prev => ({
            ...prev,
            game: gameWithServerTime,
            moves: [...prev.moves, move],
            lastOpponentMove: move?.from && move?.to ? { from: move.from, to: move.to } : null,
          }))
        } else if (message.type === 'player_joined') {
          setState(prev => ({
            ...prev,
            game: gameWithServerTime,
          }))
        } else if (message.type === 'resignation') {
          setState(prev => ({
            ...prev,
            game: gameWithServerTime,
          }))
        } else if (message.type === 'draw_offered') {
          const drawFromColor = message.drawFromColor as string | undefined
          const currentPlayerColor = stateRef.current.playerColor
          const isFromOpponent = drawFromColor && drawFromColor !== currentPlayerColor
          setState(prev => ({
            ...prev,
            game: gameWithServerTime || prev.game,
            drawOfferReceived: isFromOpponent ? true : prev.drawOfferReceived,
          }))
        } else if (message.type === 'draw_declined') {
          const autoDeclined = message.autoDeclined as boolean | undefined
          setState(prev => ({
            ...prev,
            game: gameWithServerTime || prev.game,
            // If we were the one who offered, show the decline result
            drawOfferPending: false,
            drawOfferResult: prev.drawOfferPending
              ? (autoDeclined ? 'auto_declined' : 'declined')
              : prev.drawOfferResult,
            drawOfferReceived: false,
          }))
        } else if (message.type === 'game_over') {
          // Check if this was a draw by agreement while we had a pending offer
          const wasDrawAgreement = message.game?.winReason === 'agreement'
          setState(prev => ({
            ...prev,
            game: gameWithServerTime,
            drawOfferPending: false,
            drawOfferReceived: false,
            drawOfferResult: prev.drawOfferPending && wasDrawAgreement ? 'accepted' : prev.drawOfferResult,
          }))
        }
      } catch (err) {
        console.error('WebSocket message parse error:', err)
      }
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
      // Auto-reconnect if game is still active and component is mounted
      if (!mountedRef.current) return
      const currentState = stateRef.current
      const session = wsSessionRef.current
      if (session && currentState.sessionId && currentState.game?.status === 'active') {
        // Exponential backoff: 1s, 2s, 4s, 8s, capped at 15s
        const attempt = reconnectAttemptRef.current
        const delay = Math.min(1000 * Math.pow(2, attempt), 15000)
        reconnectAttemptRef.current = attempt + 1
        console.log(`Scheduling WebSocket reconnect in ${delay}ms (attempt ${attempt + 1})...`)
        reconnectTimerRef.current = setTimeout(() => {
          if (!mountedRef.current) return
          if (wsSessionRef.current?.sessionId === session.sessionId) {
            console.log('Reconnecting WebSocket...')
            connectWebSocket(session.sessionId, session.playerId)
          }
        }, delay)
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    wsRef.current = ws
  }, [])

  const disconnectWebSocket = useCallback(() => {
    wsSessionRef.current = null
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
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
        lastOpponentMove: null,
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

      // Include serverTime with game for clock sync
      const gameWithServerTime = {
        ...response.game,
        serverTime: response.serverTime || Date.now(),
      }

      // Set lastOpponentMove from move history so the arrow renders on reload
      const moves = movesResponse.moves || []
      let lastOpponentMove: { from: string; to: string } | null = null
      if (moves.length > 0) {
        const lastMove = moves[moves.length - 1]
        if (lastMove.playerId !== response.playerId && lastMove.from && lastMove.to) {
          lastOpponentMove = { from: lastMove.from, to: lastMove.to }
        }
      }

      setState(prev => ({
        ...prev,
        sessionId: response.sessionId,
        playerId: response.playerId,
        playerColor: response.color,
        game: gameWithServerTime,
        moves,
        shareLink: fullShareLink,
        isLoading: false,
        isRestoring: false,
        lastOpponentMove,
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

      setState(prev => {
        const fetchedMoves = movesResponse.moves || []
        // If WebSocket already delivered more recent state (e.g. opponent responded
        // very quickly), don't overwrite with our stale fetch results
        if (prev.moves.length > fetchedMoves.length) {
          return { ...prev, isMoving: false, error: null }
        }
        return { ...prev, game, moves: fetchedMoves, error: null, isMoving: false }
      })

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

    console.log('resignGame: Starting resignation', { sessionId, playerId })

    try {
      const response = await api.resignGame(sessionId, playerId)
      console.log('resignGame: API response received', response)
      if (response.success) {
        // Fetch the updated game state so we can show the game summary
        // Do NOT clear state here - let the UI show the defeat dialog first
        // The leaveGame function will be called when user dismisses the summary
        console.log('resignGame: Fetching updated game state...')
        const game = await api.getGame(sessionId)
        console.log('resignGame: Got updated game', { status: game.status, winner: game.winner })
        setState(prev => {
          console.log('resignGame: Updating state with game, keeping sessionId:', prev.sessionId)
          return {
            ...prev,
            game,
          }
        })
        console.log('resignGame: State updated, resignation complete')
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
  }, [])

  const offerDraw = useCallback(async () => {
    const { sessionId, playerId } = stateRef.current
    if (!sessionId || !playerId) return

    setState(prev => ({ ...prev, drawOfferPending: true, drawOfferResult: null, drawAutoDeclineMessage: null }))

    try {
      const response = await api.offerDraw(sessionId, playerId)
      if (!response.success) {
        setState(prev => ({ ...prev, drawOfferPending: false, error: response.error || 'Failed to offer draw' }))
        return
      }
      // If auto-declined, set result immediately (don't wait for WS)
      if (response.autoDeclined) {
        setState(prev => ({
          ...prev,
          drawOfferPending: false,
          drawOfferResult: 'auto_declined',
          drawAutoDeclineMessage: response.autoDeclineMessage || 'Your opponent has draw offers set to auto-decline.',
        }))
      }
    } catch (err) {
      setState(prev => ({
        ...prev,
        drawOfferPending: false,
        error: err instanceof Error ? err.message : 'Failed to offer draw',
      }))
    }
  }, [])

  const respondToDraw = useCallback(async (accept: boolean) => {
    const { sessionId, playerId } = stateRef.current
    if (!sessionId || !playerId) return

    setState(prev => ({ ...prev, drawOfferReceived: false }))

    try {
      await api.respondToDraw(sessionId, playerId, accept)
      // Game state update will come via WS
    } catch (err) {
      setState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : 'Failed to respond to draw',
      }))
    }
  }, [])

  const claimDraw = useCallback(async (reason: api.DrawClaimReason): Promise<boolean> => {
    const { sessionId, playerId } = stateRef.current
    if (!sessionId || !playerId) return false

    try {
      const response = await api.claimDraw(sessionId, playerId, reason)
      if (!response.success) {
        setState(prev => ({ ...prev, error: response.error || 'Draw claim rejected' }))
        return false
      }
      // Game state update will come via WS if successful
      return true
    } catch (err) {
      setState(prev => ({
        ...prev,
        error: err instanceof Error ? err.message : 'Failed to claim draw',
      }))
      return false
    }
  }, [])

  const clearDrawState = useCallback(() => {
    setState(prev => ({
      ...prev,
      drawOfferPending: false,
      drawOfferReceived: false,
      drawOfferResult: null,
      drawAutoDeclineMessage: null,
    }))
  }, [])

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
      lastOpponentMove: null,
      drawOfferPending: false,
      drawOfferReceived: false,
      drawOfferResult: null,
      drawAutoDeclineMessage: null,
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
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      wsSessionRef.current = null
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [])

  // Re-fetch game state when user returns to the tab (catches games that
  // ended while the tab was hidden and WebSocket messages were lost)
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        const { sessionId, playerId, game } = stateRef.current
        if (sessionId && playerId && game) {
          Promise.all([
            api.getGame(sessionId),
            api.getMoves(sessionId),
          ]).then(([freshGame, movesResponse]) => {
            const moves = movesResponse.moves || []
            let lastOpponentMove: { from: string; to: string } | null = null
            if (moves.length > 0) {
              const lastMove = moves[moves.length - 1]
              if (lastMove.playerId !== playerId && lastMove.from && lastMove.to) {
                lastOpponentMove = { from: lastMove.from, to: lastMove.to }
              }
            }
            setState(prev => {
              if (prev.moves.length > moves.length) return prev
              return {
                ...prev,
                game: { ...freshGame, serverTime: Date.now() },
                moves,
                lastOpponentMove: lastOpponentMove || prev.lastOpponentMove,
              }
            })
          }).catch(() => {})
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  return {
    ...state,
    createGame,
    joinGame,
    makeMove,
    refreshGame,
    leaveGame,
    resignGame,
    offerDraw,
    respondToDraw,
    claimDraw,
    clearDrawState,
    clearError,
    clearConnectionError,
  }
}
