import { useState, useEffect, useRef } from 'react'
import './Clock.css'

interface ClockProps {
  /** Time remaining in milliseconds */
  timeMs: number
  /** Whether this clock is currently counting down */
  isActive: boolean
  /** Label for the clock (e.g., "White", "Black") */
  label?: string
  /** Whether this is the current player's clock */
  isPlayer?: boolean
}

/**
 * Format time in milliseconds to a display string.
 * - Under 10 seconds: shows tenths (0:05.2)
 * - Under 1 minute: shows seconds (0:45)
 * - Under 1 hour: shows minutes:seconds (14:32)
 * - Over 1 hour: shows hours:minutes:seconds (1:30:00)
 */
function formatTime(ms: number): string {
  if (ms <= 0) return '0:00'

  const totalSeconds = Math.floor(ms / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  // Under 10 seconds - show tenths
  if (totalSeconds < 10) {
    const tenths = Math.floor((ms % 1000) / 100)
    return `0:0${seconds}.${tenths}`
  }

  // Under 1 minute - just seconds
  if (totalSeconds < 60) {
    return `0:${seconds.toString().padStart(2, '0')}`
  }

  // Under 1 hour - minutes:seconds
  if (hours === 0) {
    return `${minutes}:${seconds.toString().padStart(2, '0')}`
  }

  // Over 1 hour - hours:minutes:seconds
  return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`
}

export function Clock({ timeMs, isActive, label, isPlayer }: ClockProps) {
  const [displayTime, setDisplayTime] = useState(timeMs)
  const lastUpdateRef = useRef(Date.now())
  const animationFrameRef = useRef<number | null>(null)

  // Update display time when prop changes
  useEffect(() => {
    setDisplayTime(timeMs)
    lastUpdateRef.current = Date.now()
  }, [timeMs])

  // Local countdown when active
  useEffect(() => {
    if (!isActive) {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
      }
      return
    }

    const tick = () => {
      const now = Date.now()
      const elapsed = now - lastUpdateRef.current
      lastUpdateRef.current = now

      setDisplayTime(prev => Math.max(0, prev - elapsed))
      animationFrameRef.current = requestAnimationFrame(tick)
    }

    animationFrameRef.current = requestAnimationFrame(tick)

    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current)
      }
    }
  }, [isActive])

  const isLow = displayTime > 0 && displayTime < 60 * 1000 // Under 1 minute
  const isCritical = displayTime > 0 && displayTime < 10 * 1000 // Under 10 seconds
  const isExpired = displayTime <= 0

  const classNames = [
    'clock',
    isActive && 'clock--active',
    isLow && 'clock--low',
    isCritical && 'clock--critical',
    isExpired && 'clock--expired',
    isPlayer && 'clock--player',
  ].filter(Boolean).join(' ')

  return (
    <div className={classNames}>
      {label && <span className="clock__label">{label}</span>}
      <span className="clock__time">{formatTime(displayTime)}</span>
      {isActive && !isExpired && <span className="clock__indicator" />}
    </div>
  )
}

// Time control mode display names
export const TIME_CONTROL_DISPLAY_NAMES: Record<string, string> = {
  unlimited: 'Unlimited',
  casual: 'Casual (30 min)',
  standard: 'Standard (15+10)',
  quick: 'Quick (5+3)',
  blitz: 'Blitz (3+2)',
  tournament: 'Tournament (90+30)',
}

interface TimeControlBadgeProps {
  mode: string
  baseTimeMs?: number
  incrementMs?: number
}

export function TimeControlBadge({ mode }: TimeControlBadgeProps) {
  const displayName = TIME_CONTROL_DISPLAY_NAMES[mode] || mode

  if (mode === 'unlimited') {
    return (
      <span className="time-control-badge time-control-badge--unlimited">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="time-control-icon">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 6v6l4 2" />
        </svg>
        {displayName}
      </span>
    )
  }

  return (
    <span className="time-control-badge">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="time-control-icon">
        <circle cx="12" cy="12" r="10" />
        <path d="M12 6v6l4 2" />
      </svg>
      {displayName}
    </span>
  )
}
