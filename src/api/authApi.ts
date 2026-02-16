import type { TimeControlMode } from './gameApi'

// In production, use relative URLs since the backend serves the frontend
const API_BASE = import.meta.env.VITE_API_URL || (window.location.hostname === 'localhost' ? 'http://localhost:9029/api' : '/api')

export interface UserPreferences {
  autoDeclineDraws: boolean
  preferredTimeControls?: TimeControlMode[]
}

export interface User {
  id: string
  email: string
  displayName: string
  eloRating: number
  authMethods: string[]
  emailVerified: boolean
  rankedGamesPlayed: number
  rankedWins: number
  rankedLosses: number
  rankedDraws: number
  createdAt: string
  lastDisplayNameChange?: string
  displayNameChanges: number
  preferences?: UserPreferences
}

export interface AuthResponse {
  token: string
  user: User
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const response = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || error.message || 'Login failed')
  }

  const data = await response.json()
  return {
    token: data.accessToken,
    user: data.user,
  }
}

export async function register(
  email: string,
  password: string,
  displayName: string
): Promise<AuthResponse> {
  const response = await fetch(`${API_BASE}/auth/register`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password, displayName }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || error.message || 'Registration failed')
  }

  const data = await response.json()
  return {
    token: data.accessToken,
    user: data.user,
  }
}

export async function logout(token: string): Promise<void> {
  await fetch(`${API_BASE}/auth/logout`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  })
}

export async function getCurrentUser(token: string): Promise<User> {
  const response = await fetch(`${API_BASE}/auth/me`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    throw new Error('Failed to get user info')
  }

  return response.json()
}

export function initiateGoogleLogin(): void {
  window.location.href = `${API_BASE}/auth/google`
}

export async function forgotPassword(email: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/forgot-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to send reset email')
  }

  return response.json()
}

export async function resetPassword(token: string, newPassword: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/reset-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ token, newPassword }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to reset password')
  }

  return response.json()
}

export async function verifyEmail(token: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/verify-email`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ token }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to verify email')
  }

  return response.json()
}

export async function resendVerification(email: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/resend-verification`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to resend verification email')
  }

  return response.json()
}

export async function changePassword(token: string, newPassword: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/change-password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ newPassword }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to change password')
  }

  return response.json()
}

export async function suggestDisplayName(): Promise<{ displayName: string }> {
  const response = await fetch(`${API_BASE}/auth/suggest-display-name`)

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to get suggested display name')
  }

  return response.json()
}

export async function checkDisplayName(displayName: string): Promise<{ available: boolean; reason?: string }> {
  const response = await fetch(`${API_BASE}/auth/check-display-name`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ displayName }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to check display name')
  }

  return response.json()
}

export async function changeDisplayName(token: string, displayName: string): Promise<{ message: string; user: User }> {
  const response = await fetch(`${API_BASE}/auth/change-display-name`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ displayName }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to change display name')
  }

  return response.json()
}

export async function updatePreferences(
  token: string,
  preferences: Partial<UserPreferences>
): Promise<{ message: string; user: User }> {
  const response = await fetch(`${API_BASE}/auth/preferences`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify(preferences),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to update preferences')
  }

  return response.json()
}
