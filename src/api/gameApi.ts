// In production, use relative URLs since the backend serves the frontend
const API_BASE = import.meta.env.VITE_API_URL || (window.location.hostname === 'localhost' ? 'http://localhost:9029/api' : '/api')

// Build headers with optional auth token from localStorage
function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = localStorage.getItem('chessmata_auth_token')
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  return headers
}

export interface Player {
  id: string
  color: 'white' | 'black'
  userId?: string
  displayName?: string
  agentName?: string
  eloRating?: number
  joinedAt: string
}

export interface EloChanges {
  whiteChange: number
  blackChange: number
  whiteNewElo: number
  blackNewElo: number
}

export type TimeControlMode = 'unlimited' | 'casual' | 'standard' | 'quick' | 'blitz' | 'tournament'

export interface TimeControl {
  mode: TimeControlMode
  baseTimeMs: number
  incrementMs: number
}

export interface PlayerTime {
  remainingMs: number
  lastMoveAt: number
}

export interface PlayerTimes {
  white: PlayerTime
  black: PlayerTime
}

export interface DrawOffers {
  whiteOffers: number
  blackOffers: number
  pendingFrom?: 'white' | 'black'
}

export interface Game {
  id: string
  sessionId: string
  players: Player[]
  status: 'waiting' | 'active' | 'complete'
  currentTurn: 'white' | 'black'
  boardState: string
  winner?: 'white' | 'black'
  winReason?: 'checkmate' | 'resignation' | 'timeout' | 'stalemate' | 'insufficient_material' | 'threefold_repetition' | 'fivefold_repetition' | 'fifty_moves' | 'seventy_five_moves' | 'agreement'
  isRanked?: boolean
  eloChanges?: EloChanges
  timeControl?: TimeControl
  playerTimes?: PlayerTimes
  drawOffers?: DrawOffers
  canClaimThreefold?: boolean
  canClaimFiftyMoves?: boolean
  startedAt?: string
  completedAt?: string
  createdAt: string
  updatedAt: string
  serverTime?: number // Server timestamp in ms for clock sync
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
  serverTime: number // Server timestamp in ms for clock sync
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
    headers: authHeaders(),
    body: JSON.stringify({
      playerId,
      displayName: 'Player',
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
  const headers = authHeaders()
  if (playerId) {
    headers['X-Player-ID'] = playerId
  }

  const body = {
    playerId: playerId || crypto.randomUUID(),
    displayName: 'Player',
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
    headers: authHeaders(),
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
    headers: authHeaders(),
    body: JSON.stringify({ playerId }),
  })
  return response.json()
}

// Matchmaking API

export interface JoinQueueRequest {
  connectionId: string
  displayName: string
  agentName?: string
  engineName?: string
  clientSoftware?: string
  isRanked: boolean
  preferredColor?: 'white' | 'black'
  opponentType: 'human' | 'ai' | 'either'
  timeControls?: TimeControlMode[]
}

export interface JoinQueueResponse {
  message: string
  queueId: string
}

export interface QueueStatusResponse {
  position: number
  estimatedWait: string
  status: 'waiting' | 'matched' | 'expired'
  matchedSessionId?: string
}

export async function joinMatchmakingQueue(
  request: JoinQueueRequest,
  token?: string
): Promise<JoinQueueResponse> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const response = await fetch(`${API_BASE}/matchmaking/join`, {
    method: 'POST',
    headers,
    body: JSON.stringify(request),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to join matchmaking queue')
  }

  return response.json()
}

export async function leaveMatchmakingQueue(connectionId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/matchmaking/leave?connectionId=${encodeURIComponent(connectionId)}`, {
    method: 'POST',
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to leave matchmaking queue')
  }
}

export async function getMatchmakingStatus(connectionId: string): Promise<QueueStatusResponse> {
  const response = await fetch(`${API_BASE}/matchmaking/status?connectionId=${encodeURIComponent(connectionId)}`)

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to get matchmaking status')
  }

  return response.json()
}

// Draw-related API functions

export interface DrawResponse {
  success: boolean
  offersRemaining?: number
  autoDeclined?: boolean
  autoDeclineMessage?: string
  error?: string
}

export async function offerDraw(sessionId: string, playerId: string): Promise<DrawResponse> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/offer-draw`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ playerId }),
  })
  return response.json()
}

export async function respondToDraw(
  sessionId: string,
  playerId: string,
  accept: boolean
): Promise<DrawResponse> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/respond-draw`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ playerId, accept }),
  })
  return response.json()
}

// Leaderboard
export interface LeaderboardEntry {
  rank: number
  displayName: string
  eloRating: number
  wins: number
  losses: number
  draws: number
  gamesPlayed: number
}

export async function fetchLeaderboard(type: 'players' | 'agents'): Promise<LeaderboardEntry[]> {
  const response = await fetch(`${API_BASE}/leaderboard?type=${type}`)
  if (!response.ok) return []
  return response.json()
}

// Game listing types
export interface GameListItem {
  sessionId: string
  players: Player[]
  status: string
  currentTurn?: string
  winner?: string
  winReason?: string
  isRanked?: boolean
  timeControl?: TimeControl
  boardState?: string
  startedAt?: string
  completedAt?: string
}

export async function fetchActiveGames(limit = 10, inactiveMins = 10): Promise<GameListItem[]> {
  const response = await fetch(`${API_BASE}/games/active?limit=${limit}&inactiveMins=${inactiveMins}`)
  if (!response.ok) return []
  return await response.json() || []
}

export async function fetchCompletedGames(limit = 10): Promise<GameListItem[]> {
  const response = await fetch(`${API_BASE}/games/completed?limit=${limit}`)
  if (!response.ok) return []
  return await response.json() || []
}

export interface LobbyEntry {
  displayName: string
  agentName?: string
  engineName?: string
  isRanked: boolean
  currentElo: number
  opponentType: string
  timeControls: TimeControlMode[]
  preferredColor?: string
  waitingSince: string
}

export async function fetchLobby(): Promise<LobbyEntry[]> {
  const response = await fetch(`${API_BASE}/matchmaking/lobby`)
  if (!response.ok) return []
  return await response.json() || []
}

// Match history types for profile game history
export interface MatchHistoryEntry {
  id: string
  sessionId: string
  isRanked: boolean
  whiteDisplayName: string
  blackDisplayName: string
  whiteAgent?: string
  blackAgent?: string
  whiteUserId?: string
  blackUserId?: string
  winner?: string
  winReason: string
  whiteEloChange: number
  blackEloChange: number
  totalMoves: number
  gameDuration: number
  completedAt: string
}

export interface PaginatedGameHistory {
  games: MatchHistoryEntry[]
  total: number
  page: number
  limit: number
}

export async function fetchUserGameHistory(
  userId: string,
  page = 1,
  limit = 20,
  result?: string,
  ranked?: string,
  token?: string
): Promise<PaginatedGameHistory> {
  let url = `${API_BASE}/users/${userId}/games?page=${page}&limit=${limit}`
  if (result) {
    url += `&result=${result}`
  }
  if (ranked) {
    url += `&ranked=${ranked}`
  }
  const headers: Record<string, string> = {}
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const response = await fetch(url, { headers })
  if (!response.ok) {
    return { games: [], total: 0, page, limit }
  }
  return response.json()
}

export async function lookupUserByDisplayName(
  displayName: string
): Promise<{ userId: string; displayName: string; eloRating: number } | null> {
  const response = await fetch(`${API_BASE}/users/lookup?displayName=${encodeURIComponent(displayName)}`)
  if (!response.ok) return null
  return response.json()
}

export type DrawClaimReason = 'threefold_repetition' | 'fifty_moves'

export async function claimDraw(
  sessionId: string,
  playerId: string,
  reason: DrawClaimReason
): Promise<DrawResponse> {
  const response = await fetch(`${API_BASE}/games/${sessionId}/claim-draw`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ playerId, reason }),
  })
  return response.json()
}
