import { useState } from 'react'
import { resendVerification } from '../api/authApi'
import './EmailVerificationRequiredModal.css'

interface EmailVerificationRequiredModalProps {
  email: string
  onClose: () => void
}

export function EmailVerificationRequiredModal({ email, onClose }: EmailVerificationRequiredModalProps) {
  const [resending, setResending] = useState(false)
  const [resent, setResent] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleResend = async () => {
    setResending(true)
    setError(null)
    try {
      await resendVerification(email)
      setResent(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resend')
    } finally {
      setResending(false)
    }
  }

  return (
    <div className="modal-overlay">
      <div className="email-verify-modal">
        <div className="modal-header">
          <h2>Email Verification Required</h2>
        </div>
        <div className="modal-content">
          <div className="verify-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" width="64" height="64">
              <rect x="2" y="4" width="20" height="16" rx="2" />
              <path d="M22 4L12 13L2 4" />
            </svg>
          </div>
          <p className="verify-title">Please verify your email address</p>
          <p className="verify-message">
            You need to verify your email address before you can create or join games.
            Check your inbox for the verification email we sent to <strong>{email}</strong>.
          </p>
          {error && <p className="verify-error">{error}</p>}
          {resent && <p className="verify-success">Verification email sent! Check your inbox.</p>}
        </div>
        <div className="modal-actions">
          <button
            className="modal-button secondary"
            onClick={handleResend}
            disabled={resending || resent}
          >
            {resending ? 'Sending...' : resent ? 'Sent!' : 'Resend Verification Email'}
          </button>
          <button className="modal-button primary" onClick={onClose}>
            OK
          </button>
        </div>
      </div>
    </div>
  )
}
