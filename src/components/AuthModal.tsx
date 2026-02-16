import { useState, useEffect, useCallback, useRef } from 'react'
import { forgotPassword, suggestDisplayName, checkDisplayName } from '../api/authApi'
import './AuthModal.css'

interface AuthModalProps {
  onClose: () => void
  onLogin: (email: string, password: string) => Promise<void>
  onRegister: (email: string, password: string, displayName: string) => Promise<void>
  onGoogleLogin: () => void
}

type ViewType = 'login' | 'register' | 'forgot-password'

export function AuthModal({ onClose, onLogin, onRegister, onGoogleLogin }: AuthModalProps) {
  const [activeView, setActiveView] = useState<ViewType>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [displayNameError, setDisplayNameError] = useState<string | null>(null)
  const [displayNameAvailable, setDisplayNameAvailable] = useState<boolean | null>(null)
  const [isCheckingDisplayName, setIsCheckingDisplayName] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const checkTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Fetch suggested display name when switching to register view
  useEffect(() => {
    if (activeView === 'register' && !displayName) {
      suggestDisplayName()
        .then(result => {
          setDisplayName(result.displayName)
          setDisplayNameAvailable(true)
        })
        .catch(err => {
          console.error('Failed to get suggested display name:', err)
        })
    }
  }, [activeView])

  // Check display name availability with debouncing
  const checkDisplayNameAvailability = useCallback(async (name: string) => {
    if (!name || name.length < 3) {
      setDisplayNameAvailable(null)
      setDisplayNameError(name.length > 0 && name.length < 3 ? 'Display name must be at least 3 characters' : null)
      return
    }

    if (name.length > 20) {
      setDisplayNameAvailable(false)
      setDisplayNameError('Display name must be 20 characters or less')
      return
    }

    setIsCheckingDisplayName(true)
    try {
      const result = await checkDisplayName(name)
      setDisplayNameAvailable(result.available)
      setDisplayNameError(result.available ? null : result.reason || 'Display name is not available')
    } catch (err) {
      console.error('Failed to check display name:', err)
      setDisplayNameError('Failed to check availability')
    } finally {
      setIsCheckingDisplayName(false)
    }
  }, [])

  const handleDisplayNameChange = (value: string) => {
    setDisplayName(value)
    setDisplayNameAvailable(null)
    setDisplayNameError(null)

    // Clear any existing timeout
    if (checkTimeoutRef.current) {
      clearTimeout(checkTimeoutRef.current)
    }

    // Debounce the check
    checkTimeoutRef.current = setTimeout(() => {
      if (value.trim()) {
        checkDisplayNameAvailability(value.trim())
      }
    }, 500)
  }

  const handleDisplayNameBlur = () => {
    // Check immediately on blur if we haven't checked yet
    if (displayName.trim() && displayNameAvailable === null && !isCheckingDisplayName) {
      checkDisplayNameAvailability(displayName.trim())
    }
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  const handleModalMouseDown = (e: React.MouseEvent) => {
    e.stopPropagation()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccessMessage(null)
    setIsLoading(true)

    try {
      if (activeView === 'login') {
        if (!email || !password) {
          setError('Please enter email and password')
          setIsLoading(false)
          return
        }
        await onLogin(email, password)
        onClose()
      } else if (activeView === 'register') {
        if (!email || !password || !displayName) {
          setError('Please fill in all fields')
          setIsLoading(false)
          return
        }
        if (displayNameError) {
          setError(displayNameError)
          setIsLoading(false)
          return
        }
        if (displayNameAvailable === false) {
          setError('Please choose a different display name')
          setIsLoading(false)
          return
        }
        if (password.length < 8) {
          setError('Password must be at least 8 characters')
          setIsLoading(false)
          return
        }
        if (!/[A-Z]/.test(password) || !/[a-z]/.test(password) || !/[0-9]/.test(password)) {
          setError('Password must contain uppercase, lowercase, and a number')
          setIsLoading(false)
          return
        }
        await onRegister(email, password, displayName)
        onClose()
      } else if (activeView === 'forgot-password') {
        if (!email) {
          setError('Please enter your email')
          setIsLoading(false)
          return
        }
        await forgotPassword(email)
        setSuccessMessage('If an account with that email exists, a password reset link has been sent.')
        setEmail('')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setIsLoading(false)
    }
  }

  const handleGoogleLogin = () => {
    setError(null)
    onGoogleLogin()
  }

  const getTitle = () => {
    switch (activeView) {
      case 'login': return 'Login'
      case 'register': return 'Create Account'
      case 'forgot-password': return 'Reset Password'
    }
  }

  return (
    <div className="auth-modal-overlay" onMouseDown={handleBackdropClick}>
      <div className="auth-modal" onMouseDown={handleModalMouseDown}>
        <div className="modal-header">
          <h2>{getTitle()}</h2>
          <button type="button" className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        {activeView !== 'forgot-password' && (
          <div className="modal-tabs">
            <button
              type="button"
              className={`tab-button ${activeView === 'login' ? 'active' : ''}`}
              onClick={() => {
                setActiveView('login')
                setError(null)
                setSuccessMessage(null)
              }}
            >
              Login
            </button>
            <button
              type="button"
              className={`tab-button ${activeView === 'register' ? 'active' : ''}`}
              onClick={() => {
                setActiveView('register')
                setError(null)
                setSuccessMessage(null)
              }}
            >
              Register
            </button>
          </div>
        )}

        <div className="modal-content">
          {activeView === 'forgot-password' ? (
            <form onSubmit={handleSubmit}>
              <p className="forgot-password-text">
                Enter your email address and we'll send you a link to reset your password.
              </p>

              <div className="form-group">
                <label htmlFor="email">Email</label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="your.email@example.com"
                  disabled={isLoading}
                  autoComplete="email"
                />
              </div>

              {error && <div className="error-message">{error}</div>}
              {successMessage && <div className="success-message">{successMessage}</div>}

              <button
                type="submit"
                className="submit-button"
                disabled={isLoading}
              >
                {isLoading ? 'Sending...' : 'Send Reset Link'}
              </button>

              <button
                type="button"
                className="back-to-login-button"
                onClick={() => {
                  setActiveView('login')
                  setError(null)
                  setSuccessMessage(null)
                }}
              >
                Back to Login
              </button>
            </form>
          ) : (
            <>
              <form onSubmit={handleSubmit}>
                {activeView === 'register' && (
                  <div className="form-group">
                    <label htmlFor="displayName">Display Name</label>
                    <div className="input-with-status">
                      <input
                        id="displayName"
                        type="text"
                        value={displayName}
                        onChange={(e) => handleDisplayNameChange(e.target.value)}
                        onBlur={handleDisplayNameBlur}
                        placeholder="Choose a display name"
                        disabled={isLoading}
                        autoComplete="username"
                        className={displayNameError ? 'input-error' : displayNameAvailable ? 'input-success' : ''}
                      />
                      {isCheckingDisplayName && (
                        <span className="input-status checking">Checking...</span>
                      )}
                      {!isCheckingDisplayName && displayNameAvailable === true && (
                        <span className="input-status available">
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                            <polyline points="20 6 9 17 4 12" />
                          </svg>
                        </span>
                      )}
                      {!isCheckingDisplayName && displayNameAvailable === false && (
                        <span className="input-status unavailable">
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                            <line x1="18" y1="6" x2="6" y2="18" />
                            <line x1="6" y1="6" x2="18" y2="18" />
                          </svg>
                        </span>
                      )}
                    </div>
                    {displayNameError && <div className="field-error">{displayNameError}</div>}
                  </div>
                )}

                <div className="form-group">
                  <label htmlFor="email">Email</label>
                  <input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="your.email@example.com"
                    disabled={isLoading}
                    autoComplete="email"
                  />
                </div>

                <div className="form-group">
                  <label htmlFor="password">Password</label>
                  <input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder={activeView === 'register' ? '8+ chars, uppercase, lowercase, number' : 'Your password'}
                    disabled={isLoading}
                    autoComplete={activeView === 'login' ? 'current-password' : 'new-password'}
                  />
                </div>

                {activeView === 'login' && (
                  <button
                    type="button"
                    className="forgot-password-link"
                    onClick={() => {
                      setActiveView('forgot-password')
                      setError(null)
                      setSuccessMessage(null)
                    }}
                  >
                    Forgot password?
                  </button>
                )}

                {error && <div className="error-message">{error}</div>}

                <button
                  type="submit"
                  className="submit-button"
                  disabled={isLoading || (activeView === 'register' && displayNameAvailable === false)}
                >
                  {isLoading ? 'Please wait...' : (activeView === 'login' ? 'Login' : 'Create Account')}
                </button>
              </form>

              <div className="divider">
                <span>or</span>
              </div>

              <button
                type="button"
                className="google-button"
                onClick={handleGoogleLogin}
                disabled={isLoading}
              >
                <svg viewBox="0 0 24 24" width="20" height="20">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                </svg>
                Continue with Google
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
