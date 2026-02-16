import { useState, useCallback } from 'react'

export type BoardMaterial = 'plain' | 'marble' | 'wood' | 'resin' | 'monochrome' | 'neon'
export type PieceModel = 'basic' | 'standard' | 'detailed' | 'fantasy' | 'meshy' | 'cubist'
export type PieceMaterial = 'simple' | 'realistic' | 'crystal' | 'chrome' | 'wood'
export type LightingPreset = 'standard' | 'soft' | 'overhead' | 'front' | 'dramatic'

export interface Settings {
  boardMaterial: BoardMaterial
  pieceModel: PieceModel
  pieceMaterial: PieceMaterial
  lighting: LightingPreset
}

export interface SettingsContextType {
  settings: Settings
  updateSettings: (updates: Partial<Settings>) => void
}

const defaultSettings: Settings = {
  boardMaterial: 'marble',
  pieceModel: 'detailed',
  pieceMaterial: 'realistic',
  lighting: 'standard'
}

export function useSettingsState() {
  const [settings, setSettings] = useState<Settings>(defaultSettings)

  const updateSettings = useCallback((updates: Partial<Settings>) => {
    setSettings(prev => ({ ...prev, ...updates }))
  }, [])

  return { settings, updateSettings }
}
