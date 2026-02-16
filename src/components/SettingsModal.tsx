import { useState, useEffect, useCallback } from 'react'
import type { Settings, SettingsContextType, BoardMaterial, PieceModel, PieceMaterial, LightingPreset } from '../hooks/useSettings'
import { createApiKey, listApiKeys, deleteApiKey } from '../api/apiKeysApi'
import type { ApiKey } from '../api/apiKeysApi'
import './SettingsModal.css'

type SettingsSection = 'graphics' | 'apikeys'

interface SettingsModalProps {
  settings: Settings
  updateSettings: SettingsContextType['updateSettings']
  onClose: () => void
  isAuthenticated?: boolean
  token?: string | null
}

export function SettingsModal({ settings, updateSettings, onClose, isAuthenticated, token }: SettingsModalProps) {
  const [activeSection, setActiveSection] = useState<SettingsSection>('graphics')

  // API Keys state
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [newKeyName, setNewKeyName] = useState('')
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<string | null>(null)
  const [apiKeyError, setApiKeyError] = useState<string | null>(null)
  const [apiKeyLoading, setApiKeyLoading] = useState(false)
  const [keyCopied, setKeyCopied] = useState(false)

  const loadApiKeys = useCallback(async () => {
    if (!token) return
    try {
      const data = await listApiKeys(token)
      setApiKeys(data.apiKeys)
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : 'Failed to load API keys')
    }
  }, [token])

  useEffect(() => {
    if (activeSection === 'apikeys' && token) {
      loadApiKeys()
    }
  }, [activeSection, token, loadApiKeys])

  const handleCreateApiKey = async () => {
    if (!token || !newKeyName.trim()) return
    setApiKeyError(null)
    setApiKeyLoading(true)
    try {
      const data = await createApiKey(token, newKeyName.trim())
      setNewlyCreatedKey(data.key)
      setNewKeyName('')
      setKeyCopied(false)
      await loadApiKeys()
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : 'Failed to create API key')
    } finally {
      setApiKeyLoading(false)
    }
  }

  const handleDeleteApiKey = async (keyId: string) => {
    if (!token) return
    setApiKeyError(null)
    try {
      await deleteApiKey(token, keyId)
      setApiKeys(prev => prev.filter(k => k.id !== keyId))
    } catch (err) {
      setApiKeyError(err instanceof Error ? err.message : 'Failed to delete API key')
    }
  }

  const handleCopyKey = async () => {
    if (!newlyCreatedKey) return
    try {
      await navigator.clipboard.writeText(newlyCreatedKey)
      setKeyCopied(true)
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement('textarea')
      textarea.value = newlyCreatedKey
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      setKeyCopied(true)
    }
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  const handleMaterialChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSettings({ boardMaterial: e.target.value as BoardMaterial })
  }

  const handlePieceModelChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSettings({ pieceModel: e.target.value as PieceModel })
  }

  const handlePieceMaterialChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSettings({ pieceMaterial: e.target.value as PieceMaterial })
  }

  const handleLightingChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSettings({ lighting: e.target.value as LightingPreset })
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
            {isAuthenticated && (
              <button
                className={`sidebar-item ${activeSection === 'apikeys' ? 'active' : ''}`}
                onClick={() => setActiveSection('apikeys')}
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
                </svg>
                <span>API Keys</span>
              </button>
            )}
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
                    <option value="resin">Resin</option>
                    <option value="monochrome">Monochrome</option>
                    <option value="neon">Dayglow Neon</option>
                  </select>
                </div>

                <div className="setting-row">
                  <label htmlFor="piece-model">Chess Piece Model</label>
                  <select
                    id="piece-model"
                    value={settings.pieceModel}
                    onChange={handlePieceModelChange}
                  >
                    <option value="basic">Basic</option>
                    <option value="standard">Standard</option>
                    <option value="detailed">Traditional Detailed</option>
                    <option value="fantasy">Fantasy</option>
                    <option value="meshy">Fantasy Meshy Generated</option>
                    <option value="cubist">Cubist</option>
                  </select>
                </div>

                <div className="setting-row">
                  <label htmlFor="piece-material">Piece Material</label>
                  <select
                    id="piece-material"
                    value={settings.pieceMaterial}
                    onChange={handlePieceMaterialChange}
                  >
                    <option value="simple">Simple</option>
                    <option value="realistic">Realistic</option>
                    <option value="wood">Wood</option>
                    <option value="crystal">Crystal</option>
                    <option value="chrome">Chrome</option>
                  </select>
                </div>

                <div className="setting-row">
                  <label htmlFor="lighting">Lighting</label>
                  <select
                    id="lighting"
                    value={settings.lighting}
                    onChange={handleLightingChange}
                  >
                    <option value="standard">Standard</option>
                    <option value="soft">Soft Diffuse</option>
                    <option value="overhead">Single Overhead</option>
                    <option value="front">Front Lit</option>
                    <option value="dramatic">Dramatic Side</option>
                  </select>
                </div>
              </div>
            )}

            {activeSection === 'apikeys' && (
              <div className="settings-section">
                <h3>API Keys</h3>
                <p className="api-keys-description">
                  Create API keys for programmatic access to the Chessmata API. Keys use Bearer authentication and have the same permissions as your account.
                </p>

                {apiKeyError && (
                  <div className="api-key-error">{apiKeyError}</div>
                )}

                {newlyCreatedKey && (
                  <div className="api-key-created-banner">
                    <div className="api-key-created-label">Your new API key (shown only once):</div>
                    <div className="api-key-created-value">
                      <code>{newlyCreatedKey}</code>
                      <button className="copy-key-button" onClick={handleCopyKey}>
                        {keyCopied ? 'Copied!' : 'Copy'}
                      </button>
                    </div>
                    <button className="api-key-dismiss" onClick={() => setNewlyCreatedKey(null)}>
                      Dismiss
                    </button>
                  </div>
                )}

                <div className="api-key-create-form">
                  <input
                    type="text"
                    className="api-key-name-input"
                    placeholder="Key name (e.g. My Bot)"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    maxLength={50}
                    onKeyDown={(e) => { if (e.key === 'Enter') handleCreateApiKey() }}
                  />
                  <button
                    className="api-key-create-button"
                    onClick={handleCreateApiKey}
                    disabled={apiKeyLoading || !newKeyName.trim()}
                  >
                    {apiKeyLoading ? 'Creating...' : 'Create Key'}
                  </button>
                </div>

                <div className="api-keys-list">
                  {apiKeys.length === 0 ? (
                    <div className="api-keys-empty">No API keys yet.</div>
                  ) : (
                    apiKeys.map(key => (
                      <div key={key.id} className="api-key-row">
                        <div className="api-key-info">
                          <div className="api-key-name">{key.name}</div>
                          <div className="api-key-meta">
                            <code>{key.keyPrefix}...</code>
                            <span>Created {new Date(key.createdAt).toLocaleDateString()}</span>
                            {key.lastUsedAt && (
                              <span>Last used {new Date(key.lastUsedAt).toLocaleDateString()}</span>
                            )}
                          </div>
                        </div>
                        <button
                          className="api-key-delete-button"
                          onClick={() => handleDeleteApiKey(key.id)}
                        >
                          Delete
                        </button>
                      </div>
                    ))
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
