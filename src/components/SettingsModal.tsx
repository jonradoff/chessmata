import { useState } from 'react'
import type { Settings, SettingsContextType, BoardMaterial } from '../hooks/useSettings'
import './SettingsModal.css'

type SettingsSection = 'graphics'

interface SettingsModalProps {
  settings: Settings
  updateSettings: SettingsContextType['updateSettings']
  onClose: () => void
}

export function SettingsModal({ settings, updateSettings, onClose }: SettingsModalProps) {
  const [activeSection, setActiveSection] = useState<SettingsSection>('graphics')

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  const handleMaterialChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSettings({ boardMaterial: e.target.value as BoardMaterial })
  }

  return (
    <div className="settings-backdrop" onClick={handleBackdropClick}>
      <div className="settings-modal">
        <div className="settings-header">
          <h2>Settings</h2>
          <button className="close-button" onClick={onClose}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <div className="settings-content">
          <nav className="settings-sidebar">
            <button
              className={`sidebar-item ${activeSection === 'graphics' ? 'active' : ''}`}
              onClick={() => setActiveSection('graphics')}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
                <line x1="8" y1="21" x2="16" y2="21" />
                <line x1="12" y1="17" x2="12" y2="21" />
              </svg>
              <span>Graphics</span>
            </button>
          </nav>

          <div className="settings-panel">
            {activeSection === 'graphics' && (
              <div className="settings-section">
                <h3>Graphics Settings</h3>

                <div className="setting-row">
                  <label htmlFor="board-material">Chessboard Material</label>
                  <select
                    id="board-material"
                    value={settings.boardMaterial}
                    onChange={handleMaterialChange}
                  >
                    <option value="plain">Plain</option>
                    <option value="marble">Marble</option>
                    <option value="wood">Wood</option>
                  </select>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
