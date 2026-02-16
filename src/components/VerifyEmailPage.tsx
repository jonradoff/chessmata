import { useState, useEffect } from 'react'
import { verifyEmail } from '../api/authApi'
import './VerifyEmailPage.css'

interface VerifyEmailPageProps {
  token: string
  onSuccess: () => void
  onBackToHome: () => void
}

export function VerifyEmailPage({ token, onSuccess, onBackToHome }: VerifyEmailPageProps) {
  const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!token) {
      setStatus('error')
      setError('Invalid or missing verification token')
      return
    }

    const verify = async () => {
      try {
        await verifyEmail(token)
        setStatus('success')
        // Redirect to home after showing success
        setTimeout(() => {
          onSuccess()
        }, 3000)
      } catch (err) {
        setStatus('error')
        setError(err instanceof Error ? err.message : 'Failed to verify email')
      }
    }

    verify()
  }, [token, onSuccess])

  if (status === 'verifying') {
    return (
      <div className="verify-email-page">
        <div className="verify-email-container">
          <div className="verify-email-logo">
            <h1>Chessmata</h1>
          </div>
          <div className="verifying-container">
            <div className="spinner"></div>
            <h2>Verifying Your Email</h2>
            <p>Please wait while we verify your email address...</p>
          </div>
        </div>
      </div>
    )
  }

  if (status === 'success') {
    return (
      <div className="verify-email-page">
        <div className="verify-email-container">
          <div className="verify-email-logo">
            <h1>Chessmata</h1>
          </div>
          <div className="success-container">
            <div className="success-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                <polyline points="22 4 12 14.01 9 11.01" />
              </svg>
            </div>
            <h2>Email Verified!</h2>
            <p>Thank you for verifying your email address. Your account is now fully activated.</p>
            <p className="redirect-text">Redirecting to Chessmata...</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="verify-email-page">
      <div className="verify-email-container">
        <div className="verify-email-logo">
          <h1>Chessmata</h1>
          <p>Chess for Humans and AI Agents</p>
        </div>
        <div className="error-container">
          <div className="error-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="15" y1="9" x2="9" y2="15" />
              <line x1="9" y1="9" x2="15" y2="15" />
            </svg>
          </div>
          <h2>Verification Failed</h2>
          <p>{error}</p>
          <p className="help-text">
            This link may have expired or already been used. Please request a new verification email from your account settings.
          </p>
          <button className="home-button" onClick={onBackToHome}>
            Go to Chessmata
          </button>
        </div>
      </div>
    </div>
  )
}
