import { useState } from 'react'
import type { Game, DrawClaimReason } from '../api/gameApi'
import './EndGameModal.css'

interface EndGameModalProps {
  game: Game
  playerColor: 'white' | 'black'
  onResign: () => Promise<void>
  onOfferDraw: () => Promise<void>
  onClaimDraw: (reason: DrawClaimReason) => Promise<boolean>
  onCancel: () => void
  isLoading?: boolean
}

type ModalView = 'main' | 'resign-confirm' | 'draw-options'

export function EndGameModal({
  game,
  playerColor,
  onResign,
  onOfferDraw,
  onClaimDraw,
  onCancel,
  isLoading,
}: EndGameModalProps) {
  const [view, setView] = useState<ModalView>('main')
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  // Calculate remaining draw offers
  const myDrawOffers = playerColor === 'white'
    ? game.drawOffers?.whiteOffers ?? 0
    : game.drawOffers?.blackOffers ?? 0
  const remainingOffers = 3 - myDrawOffers
  const hasPendingOffer = game.drawOffers?.pendingFrom != null
  const canOfferDraw = remainingOffers > 0 && !hasPendingOffer

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget && !isLoading && !actionLoading) {
      onCancel()
    }
  }

  const handleResign = async () => {
    setActionLoading('resign')
    try {
      await onResign()
    } finally {
      setActionLoading(null)
    }
  }

  const handleOfferDraw = async () => {
    setActionLoading('offer')
    try {
      await onOfferDraw()
      onCancel() // Close modal after offering
    } finally {
      setActionLoading(null)
    }
  }

  const handleClaimDraw = async (reason: DrawClaimReason) => {
    setActionLoading(reason)
    try {
      await onClaimDraw(reason)
    } finally {
      setActionLoading(null)
    }
  }

  const renderMainView = () => (
    <>
      <div className="end-game-header">
        <h2>End Game Options</h2>
      </div>
      <div className="end-game-content">
        <button
          className="end-game-option end-game-option--resign"
          onClick={() => setView('resign-confirm')}
          disabled={!!actionLoading}
        >
          <span className="end-game-option-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
              <line x1="4" y1="22" x2="4" y2="15" />
            </svg>
          </span>
          <div className="end-game-option-text">
            <span className="end-game-option-title">Resign</span>
            <span className="end-game-option-desc">Forfeit the game</span>
          </div>
        </button>

        <button
          className="end-game-option end-game-option--draw"
          onClick={() => setView('draw-options')}
          disabled={!!actionLoading || !canOfferDraw}
        >
          <span className="end-game-option-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M8 12h8" />
            </svg>
          </span>
          <div className="end-game-option-text">
            <span className="end-game-option-title">Offer Draw</span>
            <span className="end-game-option-desc">
              {hasPendingOffer
                ? 'Draw offer pending...'
                : `${remainingOffers} offer${remainingOffers !== 1 ? 's' : ''} remaining`}
            </span>
          </div>
        </button>
      </div>
      <div className="end-game-actions">
        <button
          className="end-game-cancel-btn"
          onClick={onCancel}
          disabled={!!actionLoading}
        >
          Cancel
        </button>
      </div>
    </>
  )

  const renderResignConfirm = () => (
    <>
      <div className="end-game-header">
        <h2>Resign Game?</h2>
      </div>
      <div className="end-game-content end-game-content--centered">
        <p className="end-game-warning">
          Are you sure you want to resign? Your opponent will be declared the winner.
        </p>
      </div>
      <div className="end-game-actions">
        <button
          className="end-game-back-btn"
          onClick={() => setView('main')}
          disabled={actionLoading === 'resign'}
        >
          Back
        </button>
        <button
          className="end-game-confirm-btn end-game-confirm-btn--danger"
          onClick={handleResign}
          disabled={actionLoading === 'resign'}
        >
          {actionLoading === 'resign' ? 'Resigning...' : 'Yes, Resign'}
        </button>
      </div>
    </>
  )

  const renderDrawOptions = () => (
    <>
      <div className="end-game-header">
        <h2>Draw Options</h2>
      </div>
      <div className="end-game-content">
        <button
          className="end-game-option end-game-option--offer"
          onClick={handleOfferDraw}
          disabled={!!actionLoading || !canOfferDraw}
        >
          <span className="end-game-option-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
              <circle cx="9" cy="7" r="4" />
              <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
              <path d="M16 3.13a4 4 0 0 1 0 7.75" />
            </svg>
          </span>
          <div className="end-game-option-text">
            <span className="end-game-option-title">Offer to Opponent</span>
            <span className="end-game-option-desc">
              {actionLoading === 'offer' ? 'Sending...' : 'Propose a draw'}
            </span>
          </div>
        </button>

        <div className="end-game-divider">
          <span>Or claim a draw</span>
        </div>

        <button
          className={`end-game-option end-game-option--claim${game.canClaimThreefold ? ' end-game-option--claim-active' : ''}`}
          onClick={() => handleClaimDraw('threefold_repetition')}
          disabled={!!actionLoading || !game.canClaimThreefold}
        >
          <div className="end-game-option-text">
            <span className="end-game-option-title">Threefold Repetition</span>
            <span className="end-game-option-desc">
              {actionLoading === 'threefold_repetition'
                ? 'Claiming...'
                : game.canClaimThreefold
                  ? 'Available — same position occurred 3+ times'
                  : 'Not available — same position must occur 3+ times'}
            </span>
          </div>
        </button>

        <button
          className={`end-game-option end-game-option--claim${game.canClaimFiftyMoves ? ' end-game-option--claim-active' : ''}`}
          onClick={() => handleClaimDraw('fifty_moves')}
          disabled={!!actionLoading || !game.canClaimFiftyMoves}
        >
          <div className="end-game-option-text">
            <span className="end-game-option-title">Fifty-Move Rule</span>
            <span className="end-game-option-desc">
              {actionLoading === 'fifty_moves'
                ? 'Claiming...'
                : game.canClaimFiftyMoves
                  ? 'Available — 50+ moves without capture or pawn move'
                  : 'Not available — 50 moves without capture or pawn move needed'}
            </span>
          </div>
        </button>
      </div>
      <div className="end-game-actions">
        <button
          className="end-game-back-btn"
          onClick={() => setView('main')}
          disabled={!!actionLoading}
        >
          Back
        </button>
      </div>
    </>
  )

  return (
    <div className="end-game-backdrop" onClick={handleBackdropClick}>
      <div className="end-game-modal">
        {view === 'main' && renderMainView()}
        {view === 'resign-confirm' && renderResignConfirm()}
        {view === 'draw-options' && renderDrawOptions()}
      </div>
    </div>
  )
}
