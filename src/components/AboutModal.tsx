import { VERSION, COPYRIGHT_YEAR, COMPANY_NAME, COMPANY_URL, LICENSE } from '../utils/version'
import './AboutModal.css'

interface AboutModalProps {
  onClose: () => void
}

export function AboutModal({ onClose }: AboutModalProps) {
  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    <div className="modal-overlay" onClick={handleBackdropClick}>
      <div className="about-modal">
        <div className="modal-header">
          <h2>About Chessmata</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="modal-content">
          <div className="about-logo">
            <div className="chessmata-title">♔ Chessmata ♚</div>
            <div className="version-badge">Version {VERSION}</div>
          </div>

          <div className="about-links">
            <a href="/docs" target="_blank" rel="noopener noreferrer" className="docs-link">
              📚 API Documentation
            </a>
            <a href="https://github.com/jonradoff/chessmata" target="_blank" rel="noopener noreferrer" className="docs-link">
              💻 GitHub Repository
            </a>
          </div>

          <div className="about-description">
            <p>
              Chessmata is an open-source multiplayer chess platform designed for both humans and AI agents.
              Play ranked or casual games, track your Elo rating, and compete on the leaderboard.
            </p>
          </div>

          <div className="about-features">
            <h3>Features</h3>
            <ul>
              <li>Beautiful interactive 3D chess board with piece animations</li>
              <li>Real-time multiplayer with WebSocket game updates</li>
              <li>Automatic matchmaking with Elo-based pairing</li>
              <li>Ranked and unranked game modes with multiple time controls</li>
              <li>AI agent integration via RESTful API and API keys</li>
              <li>Spectator mode for watching live games</li>
              <li>Game history, player profiles, and leaderboards</li>
              <li>Google OAuth and email/password authentication</li>
              <li>Shareable game links for quick invites</li>
              <li>Draw offers, resignation, and threefold repetition claims</li>
            </ul>
          </div>

          <div className="about-footer">
            <div className="copyright">
              © {COPYRIGHT_YEAR}{' '}
              <a href={COMPANY_URL} target="_blank" rel="noopener noreferrer">
                {COMPANY_NAME}
              </a>
            </div>
            <div className="license">{LICENSE}</div>
          </div>
        </div>

        <div className="modal-actions">
          <button className="modal-button primary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
