import { AGENT_NAME, AGENT_VERSION } from '../utils/version'

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:9029/api'

export interface Player {
  id: string
  color: 'white' | 'black'
  joinedAt: string
}

export interface Game {
  id: string
  sessionId: string
  players: Player[]
  status: 'waiting' | 'active' | 'complete'
  currentTurn: 'white' | 'black'
  boardState: string
  winner?: 'white' | 'black'
  winReason?: 'checkmate' | 'resignation' | 'timeout'
  createdAt: string
  updatedAt: string
}

export interface Move {
  id: string
  gameId: string
  sessionId: string
  playerId: string
  moveNumber: number
  from: string
  to: string
  piece: string
  notation: string
  capture: boolean
  check: boolean
  checkmate: boolean
  promotion?: string
  createdAt: string
}

export interface CreateGameResponse {
  sessionId: string
  playerId: string
  shareLink: string
}

export interface JoinGameResponse {
  sessionId: string
  playerId: string
  color: 'white' | 'black'
  game: Game
}

export interface MakeMoveResponse {
  success: boolean
  move?: Move
  boardState: string
  check: boolean
  checkmate: boolean
  stalemate: boolean
  error?: string
}

export async function createGame(): Promise<CreateGameResponse> {
  // Generate a unique player ID
  const playerId = crypto.randomUUID()

  const response = await fetch(`${API_BASE}/games`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      playerId,
      displayName: 'Player',
      agentName: AGENT_NAME,
      agentVersion: AGENT_VERSION,
    }),
  })
  if (!response.ok) {
    throw new Error('Failed to create game')
  }
  return response.json()
}

export async function getGame(sessionId: string): Promise<Game> {
  const response = await fetch(`${API_BASE}/games/${sessionId}`)
  if (!response.ok) {
    throw new Error('Game not found')
  }
  return response.json()
}

export async function joinGame(sessionId: string, playerId?: string): Promise<JoinGameResponse> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (playerId) {
    headers['X-Player-ID'] = playerId
  }

  const body = {
    playerId: playerId || crypto.randomUUID(),
    displayName: 'Player',
    agentName: AGENT_NAME,
    agentVersion: AGENT_VERSION,
  }

  const response = await fetch(`${API_BASE}/games/${sessionId}/join`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || 'Failed to join game')
  }
  return response.json()
}

export async function makeMove(
  sessionId: string,
  playerId: string,
  from: string,
  to: string,
  promotion?: string
): Promise<MakeMoveResponse> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/move`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ playerId, from, to, promotion }),
  })
  return response.json()
}

export async function getMoves(sessionId: string): Promise<{ moves: Move[] }> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/moves`)
  if (!response.ok) {
    throw new Error('Failed to fetch moves')
  }
  return response.json()
}

export interface ResignResponse {
  success: boolean
  winner?: string
  error?: string
}

export async function resignGame(sessionId: string, playerId: string): Promise<ResignResponse> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/resign`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ playerId }),
  })
  return response.json()
}
