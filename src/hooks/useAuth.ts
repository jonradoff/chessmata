import { useState, useEffect, useCallback } from 'react'
import * as authApi from '../api/authApi'

const TOKEN_KEY = 'chessmata_auth_token'

export interface AuthState {
  user: authApi.User | null
  token: string | null
  isLoading: boolean
  error: string | null
  isAuthenticated: boolean
}

export function useAuth() {
  const [state, setState] = useState<AuthState>({
    user: null,
    token: localStorage.getItem(TOKEN_KEY),
    isLoading: true,
    error: null,
    isAuthenticated: false,
  })

  // Load user on mount if token exists, or handle OAuth callback
  useEffect(() => {
    const loadUser = async () => {
      // Check for OAuth callback tokens in URL fragment (hash) first
      // Tokens are passed via fragment to avoid logging in server/referrer
      const urlParams = new URLSearchParams(window.location.hash.substring(1))
      const urlAccessToken = urlParams.get('access_token')

      if (urlAccessToken && window.location.pathname === '/auth/callback') {
        // Store the token from OAuth callback
        localStorage.setItem(TOKEN_KEY, urlAccessToken)

        // Clean the URL
        window.history.replaceState({}, '', '/')

        try {
          const user = await authApi.getCurrentUser(urlAccessToken)
          setState({
            user,
            token: urlAccessToken,
            isLoading: false,
            error: null,
            isAuthenticated: true,
          })
          return
        } catch (err) {
          console.error('Failed to load user from OAuth token:', err)
          localStorage.removeItem(TOKEN_KEY)
          setState({
            user: null,
            token: null,
            isLoading: false,
            error: 'Failed to complete login',
            isAuthenticated: false,
          })
          return
        }
      }

      // Otherwise check for existing token in localStorage
      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        setState(prev => ({ ...prev, isLoading: false }))
        return
      }

      try {
        const user = await authApi.getCurrentUser(token)
        setState({
          user,
          token,
          isLoading: false,
          error: null,
          isAuthenticated: true,
        })
      } catch (err) {
        // Token is invalid, clear it
        localStorage.removeItem(TOKEN_KEY)
        setState({
          user: null,
          token: null,
          isLoading: false,
          error: null,
          isAuthenticated: false,
        })
      }
    }

    loadUser()
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    setState(prev => ({ ...prev, isLoading: true, error: null }))
    try {
      const response = await authApi.login(email, password)
      localStorage.setItem(TOKEN_KEY, response.token)
      setState({
        user: response.user,
        token: response.token,
        isLoading: false,
        error: null,
        isAuthenticated: true,
      })
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Login failed'
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: errorMessage,
      }))
      throw err
    }
  }, [])

  const register = useCallback(
    async (email: string, password: string, displayName: string) => {
      setState(prev => ({ ...prev, isLoading: true, error: null }))
      try {
        const response = await authApi.register(email, password, displayName)
        localStorage.setItem(TOKEN_KEY, response.token)
        setState({
          user: response.user,
          token: response.token,
          isLoading: false,
          error: null,
          isAuthenticated: true,
        })
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Registration failed'
        setState(prev => ({
          ...prev,
          isLoading: false,
          error: errorMessage,
        }))
        throw err
      }
    },
    []
  )

  const logout = useCallback(async () => {
    const { token } = state
    if (token) {
      try {
        await authApi.logout(token)
      } catch (err) {
        console.error('Logout error:', err)
      }
    }
    localStorage.removeItem(TOKEN_KEY)
    setState({
      user: null,
      token: null,
      isLoading: false,
      error: null,
      isAuthenticated: false,
    })
  }, [state])

  const loginWithGoogle = useCallback(() => {
    authApi.initiateGoogleLogin()
  }, [])

  return {
    ...state,
    login,
    register,
    logout,
    loginWithGoogle,
  }
}
