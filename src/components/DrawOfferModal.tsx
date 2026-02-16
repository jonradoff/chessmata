import type { Game } from '../api/gameApi'
import type { DrawOfferResult } from '../hooks/useOnlineGame'
import './DrawOfferModal.css'

interface DrawOfferModalProps {
  drawOfferPending: boolean
  drawOfferReceived: boolean
  drawOfferResult: DrawOfferResult
  drawAutoDeclineMessage: string | null
  game: Game
  playerColor: 'white' | 'black'
  onRespondToDraw: (accept: boolean) => Promise<void>
  onDismiss: () => void
}

export function DrawOfferModal({
  drawOfferPending,
  drawOfferReceived,
  drawOfferResult,
  drawAutoDeclineMessage,
  game,
  playerColor,
  onRespondToDraw,
  onDismiss,
}: DrawOfferModalProps) {
  // Calculate opponent's remaining offers
  const opponentColor = playerColor === 'white' ? 'black' : 'white'
  const opponentOffers = opponentColor === 'white'
    ? game.drawOffers?.whiteOffers ?? 0
    : game.drawOffers?.blackOffers ?? 0
  const opponentRemaining = 3 - opponentOffers

  // Show result modal (auto-declined, declined, accepted)
  if (drawOfferResult) {
    return (
      <div className="draw-modal-backdrop" onClick={onDismiss}>
        <div className="draw-modal" onClick={e => e.stopPropagation()}>
          <div className="draw-modal-icon">
            {drawOfferResult === 'accepted' ? (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                <polyline points="22 4 12 14.01 9 11.01" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
            )}
          </div>
          <h3 className="draw-modal-title">
            {drawOfferResult === 'accepted' && 'Draw Accepted'}
            {drawOfferResult === 'declined' && 'Draw Declined'}
            {drawOfferResult === 'auto_declined' && 'Draw Auto-Declined'}
          </h3>
          <p className="draw-modal-message">
            {drawOfferResult === 'accepted' && 'The game has been drawn by agreement.'}
            {drawOfferResult === 'declined' && 'Your opponent declined the draw offer.'}
            {drawOfferResult === 'auto_declined' && (
              drawAutoDeclineMessage || 'Your opponent has draw offers set to auto-decline.'
            )}
          </p>
          <button className="draw-modal-btn draw-modal-btn--primary" onClick={onDismiss}>
            OK
          </button>
        </div>
      </div>
    )
  }

  // Show "waiting for response" spinner (we offered a draw)
  if (drawOfferPending) {
    return (
      <div className="draw-modal-backdrop">
        <div className="draw-modal">
          <div className="draw-modal-spinner">
            <div className="spinner" />
          </div>
          <h3 className="draw-modal-title">Draw Offered</h3>
          <p className="draw-modal-message">Waiting for your opponent to respond...</p>
        </div>
      </div>
    )
  }

  // Show "opponent offered a draw" with accept/decline buttons
  if (drawOfferReceived) {
    return (
      <div className="draw-modal-backdrop">
        <div className="draw-modal">
          <div className="draw-modal-icon draw-modal-icon--offer">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M8 12h8" />
            </svg>
          </div>
          <h3 className="draw-modal-title">Draw Offered</h3>
          <p className="draw-modal-message">
            Your opponent is offering a draw.
          </p>
          <p className="draw-modal-detail">
            They have {opponentRemaining} draw offer{opponentRemaining !== 1 ? 's' : ''} remaining.
          </p>
          <div className="draw-modal-actions">
            <button
              className="draw-modal-btn draw-modal-btn--decline"
              onClick={() => onRespondToDraw(false)}
            >
              Decline
            </button>
            <button
              className="draw-modal-btn draw-modal-btn--accept"
              onClick={() => onRespondToDraw(true)}
            >
              Accept Draw
            </button>
          </div>
        </div>
      </div>
    )
  }

  return null
}
