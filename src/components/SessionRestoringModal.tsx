import './SessionRestoringModal.css'

interface SessionRestoringModalProps {
  error?: string | null
}

export function SessionRestoringModal({ error }: SessionRestoringModalProps) {
  if (error) {
    return (
      <div className="modal-overlay">
        <div className="session-restoring-modal error">
          <div className="modal-content">
            <div className="error-icon">⚠️</div>
            <h2>Failed to Restore Session</h2>
            <p className="error-message">{error}</p>
            <button
              className="modal-button primary"
              onClick={() => window.location.href = '/'}
            >
              Return Home
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="modal-overlay">
      <div className="session-restoring-modal">
        <div className="modal-content">
          <div className="spinner-large"></div>
          <h2>Restoring Game Session</h2>
          <p>Please wait while we reconnect you to your game...</p>
        </div>
      </div>
    </div>
  )
}
