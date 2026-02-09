import './ResignConfirmModal.css'

interface ResignConfirmModalProps {
  onConfirm: () => void
  onCancel: () => void
  isLoading?: boolean
}

export function ResignConfirmModal({ onConfirm, onCancel, isLoading }: ResignConfirmModalProps) {
  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget && !isLoading) {
      onCancel()
    }
  }

  return (
    <div className="resign-confirm-backdrop" onClick={handleBackdropClick}>
      <div className="resign-confirm-modal">
        <div className="resign-confirm-header">
          <h2>Resign Game?</h2>
        </div>
        <div className="resign-confirm-content">
          <p>Are you sure you want to resign? Your opponent will be declared the winner.</p>
        </div>
        <div className="resign-confirm-actions">
          <button
            className="resign-cancel-btn"
            onClick={onCancel}
            disabled={isLoading}
          >
            Cancel
          </button>
          <button
            className="resign-confirm-btn"
            onClick={onConfirm}
            disabled={isLoading}
          >
            {isLoading ? 'Resigning...' : 'Yes, Resign'}
          </button>
        </div>
      </div>
    </div>
  )
}
