package eventbus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	eventTypeBroadcast         = "broadcast"
	eventTypeMatchNotification = "match_notification"
)

// WSEvent is the document stored in the ws_events collection.
type WSEvent struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	OriginMachineID string             `bson:"originMachineId"`
	EventType       string             `bson:"eventType"`
	// For game broadcast events:
	SessionID       string `bson:"sessionId,omitempty"`
	Message         []byte `bson:"message,omitempty"`
	ExcludePlayerID string `bson:"excludePlayerId,omitempty"`
	// For matchmaking notifications:
	ConnectionID   string `bson:"connectionId,omitempty"`
	MatchSessionID string `bson:"matchSessionId,omitempty"`
	// Housekeeping:
	CreatedAt time.Time `bson:"createdAt"`
}

// BroadcastFunc delivers a message to local WebSocket clients.
type BroadcastFunc func(sessionId string, message []byte, excludePlayerId string)

// MatchNotifyFunc delivers a matchmaking notification to a local WS client.
type MatchNotifyFunc func(connectionId string, sessionId string)

// EventBus publishes WebSocket events to MongoDB and watches for
// events from other machines via Change Streams.
type EventBus struct {
	machineID        string
	collection       *mongo.Collection
	broadcastLocal   BroadcastFunc
	matchNotifyLocal MatchNotifyFunc
	cancelFunc       context.CancelFunc
	wg               sync.WaitGroup
	running          bool
	mu               sync.Mutex
}

func generateMachineID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// New creates an EventBus. If collection is nil, the EventBus runs in
// local-only mode (Publish is a no-op, no watcher runs).
func New(
	collection *mongo.Collection,
	broadcastLocal BroadcastFunc,
	matchNotifyLocal MatchNotifyFunc,
) *EventBus {
	return &EventBus{
		machineID:        generateMachineID(),
		collection:       collection,
		broadcastLocal:   broadcastLocal,
		matchNotifyLocal: matchNotifyLocal,
	}
}

// MachineID returns this instance's unique identifier.
func (eb *EventBus) MachineID() string {
	return eb.machineID
}

// EnsureIndexes creates the TTL index on ws_events.createdAt.
// Idempotent — safe to call on every startup.
func (eb *EventBus) EnsureIndexes(ctx context.Context) error {
	if eb.collection == nil {
		return nil
	}
	_, err := eb.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "createdAt", Value: 1}},
		Options: options.Index().
			SetExpireAfterSeconds(60).
			SetName("ttl_createdAt_60s"),
	})
	return err
}

// Start begins the Change Stream watcher in a background goroutine.
func (eb *EventBus) Start() {
	if eb.collection == nil {
		log.Println("[EventBus] No collection configured, running in local-only mode")
		return
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	eb.cancelFunc = cancel
	eb.running = true
	eb.wg.Add(1)

	go eb.watchLoop(ctx)
	log.Printf("[EventBus] Started (machineId=%s)", eb.machineID)
}

// Stop cancels the Change Stream watcher and waits for it to exit.
func (eb *EventBus) Stop() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if !eb.running {
		return
	}
	eb.running = false
	if eb.cancelFunc != nil {
		eb.cancelFunc()
	}
	eb.wg.Wait()
	log.Println("[EventBus] Stopped")
}

// PublishBroadcast inserts a game broadcast event into ws_events.
// Errors are logged, never returned (fire-and-forget).
func (eb *EventBus) PublishBroadcast(sessionId string, message []byte, excludePlayerId string) {
	if eb.collection == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	doc := WSEvent{
		OriginMachineID: eb.machineID,
		EventType:       eventTypeBroadcast,
		SessionID:       sessionId,
		Message:         message,
		ExcludePlayerID: excludePlayerId,
		CreatedAt:       time.Now(),
	}
	if _, err := eb.collection.InsertOne(ctx, doc); err != nil {
		log.Printf("[EventBus] Failed to publish broadcast: %v", err)
	}
}

// PublishMatchNotification inserts a matchmaking notification event.
func (eb *EventBus) PublishMatchNotification(connectionId string, matchSessionId string) {
	if eb.collection == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	doc := WSEvent{
		OriginMachineID: eb.machineID,
		EventType:       eventTypeMatchNotification,
		ConnectionID:    connectionId,
		MatchSessionID:  matchSessionId,
		CreatedAt:       time.Now(),
	}
	if _, err := eb.collection.InsertOne(ctx, doc); err != nil {
		log.Printf("[EventBus] Failed to publish match notification: %v", err)
	}
}

// watchLoop runs the Change Stream in a reconnecting loop.
func (eb *EventBus) watchLoop(ctx context.Context) {
	defer eb.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}
		err := eb.watch(ctx)
		if ctx.Err() != nil {
			return // normal shutdown
		}
		log.Printf("[EventBus] Change stream error (reconnecting in 2s): %v", err)
		time.Sleep(2 * time.Second)
	}
}

func (eb *EventBus) watch(ctx context.Context) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "operationType", Value: "insert"},
		}}},
	}
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	cs, err := eb.collection.Watch(ctx, pipeline, opts)
	if err != nil {
		return err
	}
	defer cs.Close(ctx)

	for cs.Next(ctx) {
		var changeDoc struct {
			FullDocument WSEvent `bson:"fullDocument"`
		}
		if err := cs.Decode(&changeDoc); err != nil {
			log.Printf("[EventBus] Failed to decode change event: %v", err)
			continue
		}

		event := changeDoc.FullDocument

		// Skip events from this machine (already delivered locally)
		if event.OriginMachineID == eb.machineID {
			continue
		}

		switch event.EventType {
		case eventTypeBroadcast:
			if eb.broadcastLocal != nil {
				eb.broadcastLocal(event.SessionID, event.Message, event.ExcludePlayerID)
			}
		case eventTypeMatchNotification:
			if eb.matchNotifyLocal != nil {
				eb.matchNotifyLocal(event.ConnectionID, event.MatchSessionID)
			}
		default:
			log.Printf("[EventBus] Unknown event type: %s", event.EventType)
		}
	}

	return cs.Err()
}
