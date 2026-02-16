import { API_BASE } from './config'

export interface ApiKey {
  id: string
  name: string
  keyPrefix: string
  createdAt: string
  lastUsedAt?: string
}

export async function createApiKey(token: string, name: string): Promise<{ apiKey: ApiKey; key: string }> {
  const response = await fetch(`${API_BASE}/auth/api-keys`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    body: JSON.stringify({ name }),
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to create API key')
  }

  return response.json()
}

export async function listApiKeys(token: string): Promise<{ apiKeys: ApiKey[] }> {
  const response = await fetch(`${API_BASE}/auth/api-keys`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to list API keys')
  }

  return response.json()
}

export async function deleteApiKey(token: string, keyId: string): Promise<{ message: string }> {
  const response = await fetch(`${API_BASE}/auth/api-keys/${keyId}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  })

  if (!response.ok) {
    const error = await response.json()
    throw new Error(error.error || 'Failed to delete API key')
  }

  return response.json()
}
