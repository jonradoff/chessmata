import { useState, useCallback, useRef, useEffect } from 'react'
import {
  joinMatchmakingQueue,
  leaveMatchmakingQueue,
  getMatchmakingStatus,
  type JoinQueueRequest,
  type QueueStatusResponse,
  type TimeControlMode,
} from '../api/gameApi'
import { AGENT_NAME, AGENT_VERSION } from '../utils/version'

export interface MatchmakingOptions {
  displayName: string
  isRanked: boolean
  preferredColor?: 'white' | 'black'
  opponentType: 'human' | 'ai' | 'either'
  timeControls?: TimeControlMode[]
  token?: string
}

export interface MatchmakingState {
  isSearching: boolean
  connectionId: string | null
  queueStatus: QueueStatusResponse | null
  matchedSessionId: string | null
  error: string | null
}

function getWsBase() {
  if (import.meta.env.VITE_WS_URL) return import.meta.env.VITE_WS_URL
  if (window.location.hostname === 'localhost') return 'ws://localhost:9029/ws'
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${wsProtocol}//${window.location.host}/ws`
}

export function useMatchmaking() {
  const [state, setState] = useState<MatchmakingState>({
    isSearching: false,
    connectionId: null,
    queueStatus: null,
    matchedSessionId: null,
    error: null,
  })

  const wsRef = useRef<WebSocket | null>(null)
  const connectionIdRef = useRef<string | null>(null)
  // Polling fallback interval (used only if WS fails to connect)
  const pollingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const cleanup = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    if (pollingIntervalRef.current) {
      clearInterval(pollingIntervalRef.current)
      pollingIntervalRef.current = null
    }
  }, [])

  // Clean up on unmount
  useEffect(() => {
    return cleanup
  }, [cleanup])

  const pollStatus = useCallback(async () => {
    if (!connectionIdRef.current) return

    try {
      const status = await getMatchmakingStatus(connectionIdRef.current)
      setState(prev => ({ ...prev, queueStatus: status }))

      if (status.status === 'matched' && status.matchedSessionId) {
        if (pollingIntervalRef.current) {
          clearInterval(pollingIntervalRef.current)
          pollingIntervalRef.current = null
        }
        setState(prev => ({
          ...prev,
          isSearching: false,
          matchedSessionId: status.matchedSessionId || null,
        }))
      }

      if (status.status === 'expired') {
        if (pollingIntervalRef.current) {
          clearInterval(pollingIntervalRef.current)
          pollingIntervalRef.current = null
        }
        setState(prev => ({
          ...prev,
          isSearching: false,
          error: 'Queue expired. Please try again.',
        }))
      }
    } catch (err) {
      console.error('Failed to poll queue status:', err)
    }
  }, [])

  const connectMatchmakingWs = useCallback((connectionId: string) => {
    const wsBase = getWsBase()
    const ws = new WebSocket(`${wsBase}/matchmaking/${connectionId}`)

    ws.onopen = () => {
      console.log('Matchmaking WebSocket connected')
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'match_found' && data.sessionId) {
          cleanup()
          setState(prev => ({
            ...prev,
            isSearching: false,
            matchedSessionId: data.sessionId,
          }))
        }
      } catch (err) {
        console.error('Matchmaking WS message parse error:', err)
      }
    }

    ws.onerror = () => {
      console.warn('Matchmaking WebSocket error')
    }

    ws.onclose = () => {
      console.log('Matchmaking WebSocket closed')
    }

    wsRef.current = ws

    // Always poll as backup — in multi-machine deployments the WS notification
    // may be sent to a different machine than the one the client is connected to.
    if (!pollingIntervalRef.current && connectionIdRef.current) {
      pollingIntervalRef.current = setInterval(pollStatus, 3000)
    }
  }, [cleanup, pollStatus])

  const findOpponent = useCallback(async (options: MatchmakingOptions) => {
    const connectionId = crypto.randomUUID()
    connectionIdRef.current = connectionId

    setState({
      isSearching: true,
      connectionId,
      queueStatus: null,
      matchedSessionId: null,
      error: null,
    })

    try {
      const request: JoinQueueRequest = {
        connectionId,
        displayName: options.displayName,
        clientSoftware: `${AGENT_NAME} v${AGENT_VERSION}`,
        isRanked: options.isRanked,
        preferredColor: options.preferredColor,
        opponentType: options.opponentType,
        timeControls: options.timeControls,
      }

      await joinMatchmakingQueue(request, options.token)

      // Primary: WebSocket push notifications (instant, no polling overhead)
      connectMatchmakingWs(connectionId)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to join matchmaking queue'
      setState(prev => ({
        ...prev,
        isSearching: false,
        error: message,
      }))
    }
  }, [connectMatchmakingWs])

  const cancelSearch = useCallback(async () => {
    cleanup()

    if (connectionIdRef.current) {
      try {
        await leaveMatchmakingQueue(connectionIdRef.current)
      } catch (err) {
        console.error('Failed to leave queue:', err)
      }
      connectionIdRef.current = null
    }

    setState({
      isSearching: false,
      connectionId: null,
      queueStatus: null,
      matchedSessionId: null,
      error: null,
    })
  }, [cleanup])

  const clearMatch = useCallback(() => {
    setState(prev => ({
      ...prev,
      matchedSessionId: null,
    }))
  }, [])

  return {
    ...state,
    findOpponent,
    cancelSearch,
    clearMatch,
  }
}
