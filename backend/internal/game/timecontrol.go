package game

// TimeControlMode represents a chess time control setting
type TimeControlMode string

const (
	TimeUnlimited  TimeControlMode = "unlimited"
	TimeCasual     TimeControlMode = "casual"
	TimeStandard   TimeControlMode = "standard"
	TimeQuick      TimeControlMode = "quick"
	TimeBlitz      TimeControlMode = "blitz"
	TimeTournament TimeControlMode = "tournament"
)

// TimeControl defines a time control configuration
type TimeControl struct {
	Mode        TimeControlMode `json:"mode" bson:"mode"`
	BaseTimeMs  int64           `json:"baseTimeMs" bson:"baseTimeMs"`     // Base time in milliseconds
	IncrementMs int64           `json:"incrementMs" bson:"incrementMs"`   // Increment per move in milliseconds
}

// TimeControlConfigs maps time control modes to their configurations
var TimeControlConfigs = map[TimeControlMode]TimeControl{
	TimeUnlimited:  {Mode: TimeUnlimited, BaseTimeMs: 0, IncrementMs: 0},
	TimeCasual:     {Mode: TimeCasual, BaseTimeMs: 30 * 60 * 1000, IncrementMs: 0},             // 30 minutes
	TimeStandard:   {Mode: TimeStandard, BaseTimeMs: 15 * 60 * 1000, IncrementMs: 10 * 1000},   // 15 min + 10s
	TimeQuick:      {Mode: TimeQuick, BaseTimeMs: 5 * 60 * 1000, IncrementMs: 3 * 1000},        // 5 min + 3s
	TimeBlitz:      {Mode: TimeBlitz, BaseTimeMs: 3 * 60 * 1000, IncrementMs: 2 * 1000},        // 3 min + 2s
	TimeTournament: {Mode: TimeTournament, BaseTimeMs: 90 * 60 * 1000, IncrementMs: 30 * 1000}, // 90 min + 30s
}

// AllTimeControlModes returns all valid time control modes
var AllTimeControlModes = []TimeControlMode{
	TimeUnlimited,
	TimeCasual,
	TimeStandard,
	TimeQuick,
	TimeBlitz,
	TimeTournament,
}

// IsValidTimeControlMode checks if a mode string is valid
func IsValidTimeControlMode(mode string) bool {
	for _, m := range AllTimeControlModes {
		if string(m) == mode {
			return true
		}
	}
	return false
}

// GetTimeControl returns the TimeControl for a given mode
func GetTimeControl(mode TimeControlMode) TimeControl {
	if tc, ok := TimeControlConfigs[mode]; ok {
		return tc
	}
	// Default to unlimited if invalid mode
	return TimeControlConfigs[TimeUnlimited]
}

// IsUnlimited returns true if the time control has no time limit
func (tc TimeControl) IsUnlimited() bool {
	return tc.Mode == TimeUnlimited || tc.BaseTimeMs == 0
}

// GetDisplayName returns a human-readable name for the time control mode
func (mode TimeControlMode) GetDisplayName() string {
	switch mode {
	case TimeUnlimited:
		return "Unlimited"
	case TimeCasual:
		return "Casual (30 min)"
	case TimeStandard:
		return "Standard (15+10)"
	case TimeQuick:
		return "Quick (5+3)"
	case TimeBlitz:
		return "Blitz (3+2)"
	case TimeTournament:
		return "Tournament (90+30)"
	default:
		return string(mode)
	}
}

// PlayerTime tracks time remaining for a player
type PlayerTime struct {
	RemainingMs int64  `json:"remainingMs" bson:"remainingMs"` // Milliseconds remaining
	LastMoveAt  int64  `json:"lastMoveAt" bson:"lastMoveAt"`   // Unix timestamp in milliseconds of last move
}

// PlayerTimes tracks time for both players
type PlayerTimes struct {
	White PlayerTime `json:"white" bson:"white"`
	Black PlayerTime `json:"black" bson:"black"`
}

// DrawOffers tracks draw offer state
type DrawOffers struct {
	WhiteOffers int    `json:"whiteOffers" bson:"whiteOffers"` // Number of times white offered draw
	BlackOffers int    `json:"blackOffers" bson:"blackOffers"` // Number of times black offered draw
	PendingFrom string `json:"pendingFrom,omitempty" bson:"pendingFrom,omitempty"` // "white" or "black" if offer is pending
}

// MaxDrawOffers is the maximum number of draw offers a player can make
const MaxDrawOffers = 3
