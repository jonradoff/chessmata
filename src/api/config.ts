// Centralized API and WebSocket base URLs
// In production, use relative URLs since the backend serves the frontend
export const API_BASE = import.meta.env.VITE_API_URL || (window.location.hostname === 'localhost' ? 'http://localhost:9029/api' : '/api')

export const WS_BASE = (() => {
  if (import.meta.env.VITE_WS_URL) return import.meta.env.VITE_WS_URL
  if (window.location.hostname === 'localhost') return 'ws://localhost:9029/ws'
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${wsProtocol}//${window.location.host}/ws`
})()
