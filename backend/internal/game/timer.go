package game

import (
	"context"
	"log"
	"sync"
	"time"
)

// TimerBroadcaster is an interface for broadcasting time updates
type TimerBroadcaster interface {
	BroadcastTimeUpdate(sessionID string, whiteTimeMs, blackTimeMs int64, activeColor string)
	BroadcastGameOver(sessionID, winner, reason string)
}

// GameTimer tracks the time for a single game
type GameTimer struct {
	SessionID     string
	WhiteTimeMs   int64
	BlackTimeMs   int64
	ActiveColor   string // "white" or "black"
	LastTickAt    time.Time
	IsRunning     bool
	TimeControl   *TimeControl
	mu            sync.Mutex
	stopCh        chan struct{}
}

// TimerService manages timers for all active games
type TimerService struct {
	activeTimers map[string]*GameTimer
	broadcaster  TimerBroadcaster
	onTimeout    func(sessionID, winner, reason string)
	mu           sync.RWMutex
}

// NewTimerService creates a new timer service
func NewTimerService(broadcaster TimerBroadcaster, onTimeout func(sessionID, winner, reason string)) *TimerService {
	return &TimerService{
		activeTimers: make(map[string]*GameTimer),
		broadcaster:  broadcaster,
		onTimeout:    onTimeout,
	}
}

// StartTimer begins tracking time for a game
func (ts *TimerService) StartTimer(sessionID string, whiteTimeMs, blackTimeMs int64, activeColor string, timeControl *TimeControl) {
	if timeControl == nil || timeControl.IsUnlimited() {
		return // Don't track time for unlimited games
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Stop existing timer if any
	if existing, ok := ts.activeTimers[sessionID]; ok {
		close(existing.stopCh)
	}

	timer := &GameTimer{
		SessionID:   sessionID,
		WhiteTimeMs: whiteTimeMs,
		BlackTimeMs: blackTimeMs,
		ActiveColor: activeColor,
		LastTickAt:  time.Now(),
		IsRunning:   true,
		TimeControl: timeControl,
		stopCh:      make(chan struct{}),
	}

	ts.activeTimers[sessionID] = timer

	go ts.runTimer(timer)
}

// StopTimer stops tracking time for a game
func (ts *TimerService) StopTimer(sessionID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if timer, ok := ts.activeTimers[sessionID]; ok {
		timer.mu.Lock()
		timer.IsRunning = false
		timer.mu.Unlock()
		close(timer.stopCh)
		delete(ts.activeTimers, sessionID)
	}
}

// SwitchTurn switches the active player and adds increment
func (ts *TimerService) SwitchTurn(sessionID string, newActiveColor string, playerWhoMovedColor string) {
	ts.mu.RLock()
	timer, ok := ts.activeTimers[sessionID]
	ts.mu.RUnlock()

	if !ok {
		return
	}

	timer.mu.Lock()
	defer timer.mu.Unlock()

	// Deduct elapsed time from player who just moved
	elapsed := time.Since(timer.LastTickAt).Milliseconds()
	if playerWhoMovedColor == "white" {
		timer.WhiteTimeMs -= elapsed
		// Add increment for the player who just moved
		if timer.TimeControl != nil && timer.TimeControl.IncrementMs > 0 {
			timer.WhiteTimeMs += timer.TimeControl.IncrementMs
		}
	} else {
		timer.BlackTimeMs -= elapsed
		if timer.TimeControl != nil && timer.TimeControl.IncrementMs > 0 {
			timer.BlackTimeMs += timer.TimeControl.IncrementMs
		}
	}

	timer.ActiveColor = newActiveColor
	timer.LastTickAt = time.Now()
}

// UpdateTimes updates the time values (used when syncing with database)
func (ts *TimerService) UpdateTimes(sessionID string, whiteTimeMs, blackTimeMs int64) {
	ts.mu.RLock()
	timer, ok := ts.activeTimers[sessionID]
	ts.mu.RUnlock()

	if !ok {
		return
	}

	timer.mu.Lock()
	defer timer.mu.Unlock()

	timer.WhiteTimeMs = whiteTimeMs
	timer.BlackTimeMs = blackTimeMs
	timer.LastTickAt = time.Now()
}

// GetTimes returns current time values for a game
func (ts *TimerService) GetTimes(sessionID string) (whiteMs, blackMs int64, ok bool) {
	ts.mu.RLock()
	timer, exists := ts.activeTimers[sessionID]
	ts.mu.RUnlock()

	if !exists {
		return 0, 0, false
	}

	timer.mu.Lock()
	defer timer.mu.Unlock()

	// Calculate current times accounting for elapsed time
	whiteMs = timer.WhiteTimeMs
	blackMs = timer.BlackTimeMs

	elapsed := time.Since(timer.LastTickAt).Milliseconds()
	if timer.ActiveColor == "white" {
		whiteMs -= elapsed
	} else {
		blackMs -= elapsed
	}

	if whiteMs < 0 {
		whiteMs = 0
	}
	if blackMs < 0 {
		blackMs = 0
	}

	return whiteMs, blackMs, true
}

// runTimer runs the background timer loop for a game
func (ts *TimerService) runTimer(timer *GameTimer) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timer.stopCh:
			return
		case <-ticker.C:
			ts.tickTimer(timer)
		}
	}
}

// tickTimer processes a single tick for a game timer
func (ts *TimerService) tickTimer(timer *GameTimer) {
	timer.mu.Lock()
	defer timer.mu.Unlock()

	if !timer.IsRunning {
		return
	}

	// Calculate elapsed time since last tick
	now := time.Now()
	elapsed := now.Sub(timer.LastTickAt).Milliseconds()

	// Deduct time from active player
	if timer.ActiveColor == "white" {
		timer.WhiteTimeMs -= elapsed
	} else {
		timer.BlackTimeMs -= elapsed
	}

	timer.LastTickAt = now

	// Check for timeout
	if timer.WhiteTimeMs <= 0 {
		timer.WhiteTimeMs = 0
		timer.IsRunning = false
		// Black wins on time
		if ts.onTimeout != nil {
			go ts.onTimeout(timer.SessionID, "black", "timeout")
		}
		if ts.broadcaster != nil {
			ts.broadcaster.BroadcastGameOver(timer.SessionID, "black", "timeout")
		}
		return
	}

	if timer.BlackTimeMs <= 0 {
		timer.BlackTimeMs = 0
		timer.IsRunning = false
		// White wins on time
		if ts.onTimeout != nil {
			go ts.onTimeout(timer.SessionID, "white", "timeout")
		}
		if ts.broadcaster != nil {
			ts.broadcaster.BroadcastGameOver(timer.SessionID, "white", "timeout")
		}
		return
	}

	// Broadcast time update
	if ts.broadcaster != nil {
		ts.broadcaster.BroadcastTimeUpdate(
			timer.SessionID,
			timer.WhiteTimeMs,
			timer.BlackTimeMs,
			timer.ActiveColor,
		)
	}
}

// HasTimer returns whether a timer exists for the given session
func (ts *TimerService) HasTimer(sessionID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, ok := ts.activeTimers[sessionID]
	return ok
}

// ActiveGameCount returns the number of games being timed
func (ts *TimerService) ActiveGameCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.activeTimers)
}

// StartExistingGames loads and starts timers for games that were already in progress
func (ts *TimerService) StartExistingGames(ctx context.Context, loadGames func(ctx context.Context) ([]struct {
	SessionID   string
	WhiteTimeMs int64
	BlackTimeMs int64
	ActiveColor string
	TimeControl *TimeControl
}, error)) error {
	games, err := loadGames(ctx)
	if err != nil {
		return err
	}

	for _, g := range games {
		ts.StartTimer(g.SessionID, g.WhiteTimeMs, g.BlackTimeMs, g.ActiveColor, g.TimeControl)
		log.Printf("Resumed timer for game %s", g.SessionID)
	}

	return nil
}
