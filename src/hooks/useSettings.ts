import { useState, useCallback } from 'react'

export type BoardMaterial = 'plain' | 'marble' | 'wood'

export interface Settings {
  boardMaterial: BoardMaterial
}

export interface SettingsContextType {
  settings: Settings
  updateSettings: (updates: Partial<Settings>) => void
}

const defaultSettings: Settings = {
  boardMaterial: 'plain'
}

export function useSettingsState() {
  const [settings, setSettings] = useState<Settings>(defaultSettings)

  const updateSettings = useCallback((updates: Partial<Settings>) => {
    setSettings(prev => ({ ...prev, ...updates }))
  }, [])

  return { settings, updateSettings }
}
