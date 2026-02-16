import './PromotionModal.css'

interface PromotionModalProps {
  isWhite: boolean
  onSelect: (piece: 'queen' | 'rook' | 'bishop' | 'knight') => void
}

const pieces: { type: 'queen' | 'rook' | 'bishop' | 'knight'; label: string; symbol: string }[] = [
  { type: 'queen', label: 'Queen', symbol: '♛' },
  { type: 'rook', label: 'Rook', symbol: '♜' },
  { type: 'bishop', label: 'Bishop', symbol: '♝' },
  { type: 'knight', label: 'Knight', symbol: '♞' },
]

export function PromotionModal({ isWhite, onSelect }: PromotionModalProps) {
  return (
    <div className="promotion-backdrop">
      <div className="promotion-modal">
        <div className="promotion-title">Choose promotion piece</div>
        <div className="promotion-options">
          {pieces.map(p => (
            <button
              key={p.type}
              className="promotion-option"
              onClick={() => onSelect(p.type)}
              title={p.label}
            >
              <span className={`promotion-piece ${isWhite ? 'white-piece' : 'black-piece'}`}>
                {p.symbol}
              </span>
              <span className="promotion-label">{p.label}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
