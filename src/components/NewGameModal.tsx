import { useState } from 'react'
import './NewGameModal.css'

interface NewGameModalProps {
  isLoading: boolean
  error: string | null
  sessionId: string | null
  playerId: string | null
  shareLink: string | null
  onCreateGame: () => Promise<void>
  onClose: () => void
}

export function NewGameModal({
  isLoading,
  error,
  sessionId,
  playerId,
  shareLink,
  onCreateGame,
  onClose,
}: NewGameModalProps) {
  const [copiedShare, setCopiedShare] = useState(false)
  const [copiedResume, setCopiedResume] = useState(false)

  const resumeLink = sessionId && playerId
    ? `${window.location.origin}/game/${sessionId}?player=${playerId}`
    : null

  const handleCopyShare = async () => {
    if (shareLink) {
      await navigator.clipboard.writeText(shareLink)
      setCopiedShare(true)
      setTimeout(() => setCopiedShare(false), 2000)
    }
  }

  const handleCopyResume = async () => {
    if (resumeLink) {
      await navigator.clipboard.writeText(resumeLink)
      setCopiedResume(true)
      setTimeout(() => setCopiedResume(false), 2000)
    }
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    <div className="new-game-backdrop" onClick={handleBackdropClick}>
      <div className="new-game-modal">
        <div className="new-game-header">
          <h2>{sessionId ? 'Game Created!' : 'New Game'}</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="new-game-content">
          {error && (
            <div className="error-message">
              {error}
            </div>
          )}

          {!sessionId ? (
            <div className="create-section">
              <p className="description">
                Create a new game and invite an opponent to play.
              </p>
              <button
                className="create-game-btn"
                onClick={onCreateGame}
                disabled={isLoading}
              >
                {isLoading ? 'Creating Game...' : 'Create Game'}
              </button>
            </div>
          ) : (
            <div className="links-section">
              <div className="link-group">
                <label>Send this link to your opponent:</label>
                <div className="link-row">
                  <input
                    type="text"
                    readOnly
                    value={shareLink || ''}
                    className="link-input"
                  />
                  <button
                    className="copy-btn"
                    onClick={handleCopyShare}
                  >
                    {copiedShare ? 'Copied!' : 'Copy'}
                  </button>
                </div>
              </div>

              <div className="link-group">
                <label>Your session link (to resume later):</label>
                <div className="link-row">
                  <input
                    type="text"
                    readOnly
                    value={resumeLink || ''}
                    className="link-input"
                  />
                  <button
                    className="copy-btn"
                    onClick={handleCopyResume}
                  >
                    {copiedResume ? 'Copied!' : 'Copy'}
                  </button>
                </div>
                <p className="link-hint">
                  Save this link to resume your game from any device.
                </p>
              </div>

              <div className="status-section">
                <p className="waiting-text">
                  Waiting for opponent to join...
                </p>
              </div>

              <button className="start-playing-btn" onClick={onClose}>
                Start Playing
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
