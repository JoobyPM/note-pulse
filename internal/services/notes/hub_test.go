package notes

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/logger"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestHubChannelClosedAfterUnsubscribe(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()
	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)

	// Subscribe
	sub, cancel := hub.Subscribe(context.Background(), connULID, userID)
	require.NotNil(t, sub)
	require.NotNil(t, cancel)

	// Unsubscribe
	hub.Unsubscribe(context.Background(), connULID)

	// Verify that sending on the channel panics (channel closed)
	assert.Panics(t, func() {
		sub.Ch <- NoteEvent{Type: "test"}
	}, "should panic when sending to closed channel")

	// Verify Done channel is also closed
	select {
	case <-sub.Done:
		// Expected - channel should be closed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed")
	}
}

func TestHubCancelFunctionWorks(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()
	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)

	// Subscribe
	sub, cancel := hub.Subscribe(context.Background(), connULID, userID)
	require.NotNil(t, sub)
	require.NotNil(t, cancel)

	// Use cancel function
	cancel()

	// Verify that sending on the channel panics (channel closed)
	assert.Panics(t, func() {
		sub.Ch <- NoteEvent{Type: "test"}
	}, "should panic when sending to closed channel after cancel()")

	// Verify Done channel is also closed
	select {
	case <-sub.Done:
		// Expected - channel should be closed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after cancel()")
	}
}

func TestHubConcurrentBroadcastsForDifferentUsers(t *testing.T) {
	// Skip this test in short mode as it's resource-intensive
	if testing.Short() {
		t.Skip("skipping resource-intensive test in short mode")
	}

	setupTestLogger(t)

	const (
		numUsers       = 10 // subscribers to create
		broadcastCount = 50 // events per user
	)

	hub := NewHub(256)
	testData := setupConcurrentBroadcastTest(t, hub, numUsers)
	defer testData.cleanup()

	received := runConcurrentBroadcastTest(testData, broadcastCount)
	verifyConcurrentBroadcastResults(t, received, numUsers)
}

// setupTestLogger initializes a quiet logger for testing
func setupTestLogger(t *testing.T) {
	cfg := config.Config{LogLevel: "error", LogFormat: "text"}
	_, err := logger.Init(cfg)
	require.NoError(t, err)
}

// concurrentBroadcastTestData holds test setup data
type concurrentBroadcastTestData struct {
	users   []bson.ObjectID
	subs    []*Subscriber
	cancels []func()
	notes   []*Note
	hub     *Hub
}

// cleanup closes all subscribers
func (td *concurrentBroadcastTestData) cleanup() {
	for _, c := range td.cancels {
		c()
	}
}

// setupConcurrentBroadcastTest creates users, subscribers and notes for testing
func setupConcurrentBroadcastTest(t *testing.T, hub *Hub, numUsers int) *concurrentBroadcastTestData {
	users := make([]bson.ObjectID, numUsers)
	subs := make([]*Subscriber, numUsers)
	cancels := make([]func(), numUsers)
	notes := make([]*Note, numUsers)

	for i := range numUsers {
		users[i] = bson.NewObjectID()
		connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
		subs[i], cancels[i] = hub.Subscribe(context.Background(), connULID, users[i])

		notes[i] = &Note{
			ID:     bson.NewObjectID(),
			UserID: users[i],
			Title:  "test",
			Body:   "body",
		}
	}

	return &concurrentBroadcastTestData{
		users:   users,
		subs:    subs,
		cancels: cancels,
		notes:   notes,
		hub:     hub,
	}
}

// runConcurrentBroadcastTest executes the concurrent broadcast test
func runConcurrentBroadcastTest(testData *concurrentBroadcastTestData, broadcastCount int) []int {
	var (
		rcvMu    sync.Mutex
		received = make([]int, len(testData.users))
		wgRecv   sync.WaitGroup
	)

	startReceiverGoroutines(testData, &wgRecv, &rcvMu, received)
	runBroadcasterGoroutines(testData, broadcastCount)
	finalizeConcurrentTest(testData, &wgRecv)

	return received
}

// startReceiverGoroutines starts goroutines to receive events
func startReceiverGoroutines(testData *concurrentBroadcastTestData, wgRecv *sync.WaitGroup, rcvMu *sync.Mutex, received []int) {
	for i := range len(testData.users) {
		wgRecv.Add(1)
		go func(idx int) {
			defer wgRecv.Done()
			for {
				select {
				case ev, ok := <-testData.subs[idx].Ch:
					if !ok {
						return
					}
					if ev.Note != nil && ev.Note.UserID == testData.users[idx] {
						rcvMu.Lock()
						received[idx]++
						rcvMu.Unlock()
					}
				case <-testData.subs[idx].Done:
					return
				}
			}
		}(i)
	}
}

// runBroadcasterGoroutines starts goroutines to broadcast events
func runBroadcasterGoroutines(testData *concurrentBroadcastTestData, broadcastCount int) {
	var wgSend sync.WaitGroup
	for range broadcastCount {
		for u := range len(testData.users) {
			wgSend.Add(1)
			go func(idx int) {
				defer wgSend.Done()
				testData.hub.Broadcast(context.Background(), NoteEvent{
					Type: "created",
					Note: testData.notes[idx],
				})
			}(u)
		}
	}
	wgSend.Wait()
}

// finalizeConcurrentTest handles cleanup and synchronization
func finalizeConcurrentTest(testData *concurrentBroadcastTestData, wgRecv *sync.WaitGroup) {
	time.Sleep(200 * time.Millisecond) // small grace period
	testData.cleanup()                 // close all subscribers
	wgRecv.Wait()                      // receivers finished
}

// verifyConcurrentBroadcastResults verifies that all users received events
func verifyConcurrentBroadcastResults(t *testing.T, received []int, numUsers int) {
	for i := range numUsers {
		assert.Greater(t, received[i], 0, "user %d should have received events", i)
		t.Logf("user %d received %d events", i, received[i])
	}
}

func TestHubRaceConditionDetection(t *testing.T) {
	// This test is designed to be run with -race flag
	// Skip this test in short mode as it's resource intensive
	if testing.Short() {
		t.Skip("Skipping resource-intensive test in short mode")
	}

	// Initialize logger for testing
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewHub(256)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent subscribe/unsubscribe operations
	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			userID := bson.NewObjectID()
			connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)

			sub, cancel := hub.Subscribe(context.Background(), connULID, userID)

			// Broadcast some events
			note := &Note{
				ID:     bson.NewObjectID(),
				UserID: userID,
				Title:  "Test",
				Body:   "Test",
			}
			event := NoteEvent{
				Type: "created",
				Note: note,
			}

			hub.Broadcast(context.Background(), event)

			// Unsubscribe
			cancel()

			// Try to receive (should not block)
			select {
			case <-sub.Done:
				// Expected
			case <-time.After(10 * time.Millisecond):
				// Also fine - may not have received the close signal yet
			}
		}(i)
	}

	// Concurrent broadcasts
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			userID := bson.NewObjectID()
			note := &Note{
				ID:     bson.NewObjectID(),
				UserID: userID,
				Title:  "Broadcast Test",
				Body:   "Broadcast Test",
			}
			event := NoteEvent{
				Type: "updated",
				Note: note,
			}

			hub.Broadcast(context.Background(), event)
		}()
	}

	wg.Wait()
}

func TestHubUserBucketCleanup(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()

	// Subscribe and unsubscribe
	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
	_, cancel := hub.Subscribe(context.Background(), connULID, userID)

	// Verify user bucket exists
	hub.mu.RLock()
	_, exists := hub.subscribers[userID]
	hub.mu.RUnlock()
	assert.True(t, exists, "User bucket should exist after subscription")

	// Unsubscribe
	cancel()

	// Verify user bucket is cleaned up
	hub.mu.RLock()
	_, exists = hub.subscribers[userID]
	hub.mu.RUnlock()
	assert.False(t, exists, "User bucket should be cleaned up after last unsubscribe")
}

func TestHubMultipleConnectionsPerUser(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()

	// Subscribe multiple connections for the same user
	numConnections := 5
	subscribers := make([]*Subscriber, numConnections)
	cancels := make([]func(), numConnections)

	for i := range numConnections {
		connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
		sub, cancel := hub.Subscribe(context.Background(), connULID, userID)
		subscribers[i] = sub
		cancels[i] = cancel
	}

	// Verify subscriber count
	assert.Equal(t, numConnections, hub.GetSubscriberCount())

	// Broadcast an event
	note := &Note{
		ID:     bson.NewObjectID(),
		UserID: userID,
		Title:  "Multi-connection test",
		Body:   "Test",
	}
	event := NoteEvent{
		Type: "created",
		Note: note,
	}

	hub.Broadcast(context.Background(), event)

	// Verify all connections receive the event
	for i := range numConnections {
		select {
		case receivedEvent := <-subscribers[i].Ch:
			assert.Equal(t, event.Type, receivedEvent.Type)
			assert.Equal(t, event.Note.ID, receivedEvent.Note.ID)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Connection %d did not receive event", i)
		}
	}

	// Clean up
	for _, cancel := range cancels {
		cancel()
	}

	// Verify subscriber count is zero
	assert.Equal(t, 0, hub.GetSubscriberCount())
}

func TestHubBroadcastToNonexistentUser(t *testing.T) {
	hub := NewHub(256)

	// Broadcast to a user with no subscribers
	nonexistentUserID := bson.NewObjectID()
	note := &Note{
		ID:     bson.NewObjectID(),
		UserID: nonexistentUserID,
		Title:  "No subscribers",
		Body:   "Test",
	}
	event := NoteEvent{
		Type: "created",
		Note: note,
	}

	assert.NotPanics(t, func() {
		hub.Broadcast(context.Background(), event)
	}, "should not panic or cause issues")
}

// TestHub_NoLeakAfterWSDisconnect tests that all subscribers are cleaned up after disconnect
func TestHubNoLeakAfterWSDisconnect(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()
	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)

	// Subscribe
	sub, cancel := hub.Subscribe(context.Background(), connULID, userID)
	require.NotNil(t, sub)
	require.Equal(t, 1, hub.GetSubscriberCount())

	cancel()

	// Assert no leaks
	assert.Equal(t, 0, hub.GetSubscriberCount(), "hub should have no subscribers after disconnect (should not be any leaks)")

	// Verify channels are closed
	select {
	case <-sub.Done:
		// Expected - channel should be closed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed")
	}

	// Verify we can't send on the channel (should panic)
	assert.Panics(t, func() {
		sub.Ch <- NoteEvent{Type: "test"}
	}, "should panic when sending to closed channel")
}

// TestHub_BroadcastAfterUnsubscribe_NoPanic tests that broadcasting after unsubscribe doesn't panic
func TestHubBroadcastAfterUnsubscribeNoPanic(t *testing.T) {
	hub := NewHub(256)
	userID := bson.NewObjectID()

	// Create test note
	note := &Note{
		ID:     bson.NewObjectID(),
		UserID: userID,
		Title:  "Test Note",
		Body:   "Test Body",
	}
	event := NoteEvent{
		Type: "created",
		Note: note,
	}

	// Run multiple parallel goroutines to amplify race conditions
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Subscribe
			connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
			_, cancel := hub.Subscribe(context.Background(), connULID, userID)

			// Unsubscribe immediately
			cancel()

			// Broadcast after unsubscribe - should not panic
			assert.NotPanics(t, func() {
				hub.Broadcast(context.Background(), event)
			}, "Broadcasting after unsubscribe should not panic")
		}(i)
	}

	wg.Wait()
}
