import './ConnectionErrorModal.css'

interface ConnectionErrorModalProps {
  error: string
  onClose: () => void
}

export function ConnectionErrorModal({ error, onClose }: ConnectionErrorModalProps) {
  // Check if error is about missing credentials/authorization
  const isMissingSecret = error.toLowerCase().includes('unauthorized') ||
                          error.toLowerCase().includes('not authorized') ||
                          error.toLowerCase().includes('invalid player')

  return (
    <div className="modal-overlay">
      <div className="connection-error-modal">
        <div className="modal-header">
          <h2>Connection Failed</h2>
        </div>
        <div className="modal-content">
          {isMissingSecret ? (
            <>
              <div className="error-icon">🔒</div>
              <p className="error-title">Missing Session Secret</p>
              <p className="error-message">
                This game session requires a private authentication code that you don't have.
                Only players who have been properly invited can join this game.
              </p>
            </>
          ) : (
            <>
              <div className="error-icon">⚠️</div>
              <p className="error-title">Unable to Connect</p>
              <p className="error-message">{error}</p>
            </>
          )}
        </div>
        <div className="modal-actions">
          <button className="modal-button primary" onClick={onClose}>
            OK
          </button>
        </div>
      </div>
    </div>
  )
}
