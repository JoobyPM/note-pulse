package notes

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"note-pulse/internal/logger"

	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Subscriber represents a connection that can receive note events
type Subscriber struct {
	UserID bson.ObjectID
	Ch     chan NoteEvent
	Done   chan struct{}
}

// ConnInfo holds connection metadata
type ConnInfo struct {
	ID          ulid.ULID
	ConnectedAt time.Time
	Subscriber  *Subscriber
}

// userSubs holds subscribers for a specific user
type userSubs struct {
	mu sync.RWMutex
	m  map[ulid.ULID]ConnInfo
}

// Hub manages WebSocket connections and broadcasts events
type Hub struct {
	mu          sync.RWMutex
	subscribers map[bson.ObjectID]*userSubs
	connIndex   map[ulid.ULID]bson.ObjectID
	bufferSize  int
	dropped     uint64
}

// NewHub creates a new event hub with configurable buffer size
func NewHub(bufferSize int) *Hub {
	return &Hub{
		subscribers: make(map[bson.ObjectID]*userSubs),
		connIndex:   make(map[ulid.ULID]bson.ObjectID),
		bufferSize:  bufferSize,
	}
}

// Subscribe adds a new subscriber to the hub
func (h *Hub) Subscribe(connULID ulid.ULID, userID bson.ObjectID) (*Subscriber, func()) {
	log := logger.L()
	if log != nil && log.Enabled(context.Background(), slog.LevelDebug) {
		log.Debug("subscribing connection", "conn_id", connULID.String(), "user_id", userID.Hex())
	}

	h.mu.Lock()
	userBucket, exists := h.subscribers[userID]
	if !exists {
		userBucket = &userSubs{
			m: make(map[ulid.ULID]ConnInfo),
		}
		h.subscribers[userID] = userBucket
	}
	h.mu.Unlock()

	userBucket.mu.Lock()
	defer userBucket.mu.Unlock()

	sub := &Subscriber{
		UserID: userID,
		Ch:     make(chan NoteEvent, h.bufferSize),
		Done:   make(chan struct{}),
	}

	connInfo := ConnInfo{
		ID:          connULID,
		ConnectedAt: time.Now(),
		Subscriber:  sub,
	}

	userBucket.m[connULID] = connInfo

	h.mu.Lock()
	h.connIndex[connULID] = userID
	h.mu.Unlock()

	cancel := func() {
		h.Unsubscribe(connULID)
	}
	return sub, cancel
}

// Unsubscribe removes a subscriber from the hub
func (h *Hub) Unsubscribe(connULID ulid.ULID) {
	log := logger.L()
	if log != nil && log.Enabled(context.Background(), slog.LevelDebug) {
		log.Debug("unsubscribing connection", "conn_id", connULID.String())
	}

	h.mu.RLock()
	uid, ok := h.connIndex[connULID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	h.mu.RLock()
	bucket := h.subscribers[uid]
	h.mu.RUnlock()
	if bucket == nil {
		h.mu.Lock()
		delete(h.connIndex, connULID)
		h.mu.Unlock()
		return
	}

	bucket.mu.Lock()
	connInfo, exists := bucket.m[connULID]
	if exists {
		delete(bucket.m, connULID)
	}
	empty := len(bucket.m) == 0
	bucket.mu.Unlock()

	if exists {
		close(connInfo.Subscriber.Ch)
		close(connInfo.Subscriber.Done)
	}

	h.mu.Lock()
	delete(h.connIndex, connULID)
	if empty {
		delete(h.subscribers, uid)
	}
	h.mu.Unlock()
}

// Broadcast delivers ev to every subscriber of ev.Note.UserID
func (h *Hub) Broadcast(_ context.Context, ev NoteEvent) {
	if ev.Note == nil {
		return
	}

	log := logger.L()
	if log != nil && log.Enabled(context.Background(), slog.LevelDebug) {
		log.Debug("broadcasting event", "user_id", ev.Note.UserID.Hex(), "event_type", ev.Type)
	}

	bucket := h.bucket(ev.Note.UserID)
	if bucket == nil {
		return
	}

	bucket.mu.RLock()
	for _, connInfo := range bucket.m {
		sendOrDrop(connInfo.Subscriber.Ch, ev, func() {
			atomic.AddUint64(&h.dropped, 1)
			if log != nil {
				log.Warn("outbox full — dropping event", "conn_id", connInfo.ID.String(), "user_id", connInfo.Subscriber.UserID.Hex(), "event_type", ev.Type)
			}
		})
	}
	bucket.mu.RUnlock()
}

// GetSubscriberCount returns the current number of subscribers (for testing)
func (h *Hub) GetSubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalCount := 0
	for _, userBucket := range h.subscribers {
		userBucket.mu.RLock()
		totalCount += len(userBucket.m)
		userBucket.mu.RUnlock()
	}
	return totalCount
}

// sendOrDrop is the only place that can decide to drop an event.
func sendOrDrop(ch chan NoteEvent, ev NoteEvent, onDrop func()) {
	select {
	case ch <- ev: // hot path, no nesting
	default:
		onDrop()
	}
}

// Stats returns current pointers for observability / tests.
func (h *Hub) Stats() (subscribers int, dropped uint64) {
	return h.GetSubscriberCount(), atomic.LoadUint64(&h.dropped)
}

// helper: returns bucket or nil (tiny wrapper keeps Broadcast tidy)
func (h *Hub) bucket(uid bson.ObjectID) *userSubs {
	h.mu.RLock()
	b := h.subscribers[uid]
	h.mu.RUnlock()
	return b
}
