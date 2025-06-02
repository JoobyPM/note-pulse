package notes

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// BenchmarkHub_Subscribe measures the performance of subscribing users
func BenchmarkHub_Subscribe(b *testing.B) {
	hub := NewHub(256)
	userIDs := make([]bson.ObjectID, b.N)
	for i := 0; i < b.N; i++ {
		userIDs[i] = bson.NewObjectID()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
			userID := userIDs[i%len(userIDs)]
			_, cancel := hub.Subscribe(connULID, userID)
			cancel() // Clean up immediately
			i++
		}
	})
}

// M3: BenchmarkHub_Broadcast-16                            	 2770507	       453.7 ns/op	       0 B/op	       0 allocs/op
// BenchmarkHub_Broadcast measures the performance of broadcasting events
func BenchmarkHub_Broadcast(b *testing.B) {
	hub := NewHub(256)

	// Set up multiple users with subscribers
	numUsers := 100
	numConnPerUser := 5
	users := make([]bson.ObjectID, numUsers)
	cancels := make([]func(), 0, numUsers*numConnPerUser)

	for i := range numUsers {
		users[i] = bson.NewObjectID()
		for range numConnPerUser {
			connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
			_, cancel := hub.Subscribe(connULID, users[i])
			cancels = append(cancels, cancel)
		}
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// Create test notes for broadcasting
	notes := make([]*Note, numUsers)
	for i := range numUsers {
		notes[i] = &Note{
			ID:     bson.NewObjectID(),
			UserID: users[i],
			Title:  "Benchmark Note",
			Body:   "Benchmark Body",
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			event := NoteEvent{
				Type: "created",
				Note: notes[i%len(notes)],
			}
			hub.Broadcast(context.Background(), event)
			i++
		}
	})
}

// M3: BenchmarkHub_ConcurrentSubscribeUnsubscribe-16       	  598104	      1793 ns/op	    7616 B/op	      10 allocs/op
// BenchmarkHub_ConcurrentSubscribeUnsubscribe measures mixed workload performance
func BenchmarkHub_ConcurrentSubscribeUnsubscribe(b *testing.B) {
	hub := NewHub(256)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			userID := bson.NewObjectID()
			connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

			// Subscribe
			_, cancel := hub.Subscribe(connULID, userID)

			// Broadcast to this user
			note := &Note{
				ID:     bson.NewObjectID(),
				UserID: userID,
				Title:  "Mixed Workload",
				Body:   "Test",
			}
			event := NoteEvent{
				Type: "created",
				Note: note,
			}
			hub.Broadcast(context.Background(), event)

			// Unsubscribe
			cancel()
			i++
		}
	})
}

// M3: BenchmarkHub_ScalingUsers/users_0k-16                	 5939218	       206.5 ns/op	       0 B/op	       0 allocs/op
// M3: BenchmarkHub_ScalingUsers/users_0k#01-16             	 5969818	       201.8 ns/op	       0 B/op	       0 allocs/op
// M3: BenchmarkHub_ScalingUsers/users_1k/users_1k-16       	 6453801	       196.8 ns/op	       0 B/op	       0 allocs/op
// M3: BenchmarkHub_ScalingUsers/users_5k/users_5k-16       	 8820848	       191.5 ns/op	       0 B/op	       0 allocs/op
// BenchmarkHub_ScalingUsers measures how performance scales with number of users
func BenchmarkHub_ScalingUsers(b *testing.B) {
	userCounts := []int{10, 100, 1000, 5000}

	for _, userCount := range userCounts {
		b.Run("users_"+string(rune('0'+userCount/1000))+"k", func(b *testing.B) {
			if userCount >= 1000 {
				b.Run("users_"+string(rune('0'+userCount/1000))+"k", func(b *testing.B) {
					benchmarkWithUserCount(b, userCount)
				})
			} else {
				benchmarkWithUserCount(b, userCount)
			}
		})
	}
}

func benchmarkWithUserCount(b *testing.B, userCount int) {
	hub := NewHub(256)

	// Set up users with subscribers
	users := make([]bson.ObjectID, userCount)
	cancels := make([]func(), userCount)

	for i := 0; i < userCount; i++ {
		users[i] = bson.NewObjectID()
		connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
		_, cancel := hub.Subscribe(connULID, users[i])
		cancels[i] = cancel
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// Create test notes
	notes := make([]*Note, userCount)
	for i := 0; i < userCount; i++ {
		notes[i] = &Note{
			ID:     bson.NewObjectID(),
			UserID: users[i],
			Title:  "Scaling Test",
			Body:   "Test",
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			event := NoteEvent{
				Type: "updated",
				Note: notes[i%len(notes)],
			}
			hub.Broadcast(context.Background(), event)
			i++
		}
	})
}

// M3: BenchmarkHub_ConcurrentBroadcastDifferentUsers-16    	 4108659	       267.2 ns/op	      76 B/op	       2 allocs/op
// BenchmarkHub_ConcurrentBroadcastDifferentUsers verifies concurrent broadcasts scale linearly
func BenchmarkHub_ConcurrentBroadcastDifferentUsers(b *testing.B) {
	hub := NewHub(256)

	// Set up multiple users, each with one subscriber
	numUsers := 1000
	users := make([]bson.ObjectID, numUsers)
	cancels := make([]func(), numUsers)

	for i := 0; i < numUsers; i++ {
		users[i] = bson.NewObjectID()
		connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
		_, cancel := hub.Subscribe(connULID, users[i])
		cancels[i] = cancel
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	// Create test notes for different users
	notes := make([]*Note, numUsers)
	for i := 0; i < numUsers; i++ {
		notes[i] = &Note{
			ID:     bson.NewObjectID(),
			UserID: users[i],
			Title:  "Concurrent Test",
			Body:   "Test",
		}
	}

	b.ResetTimer()

	// Run with different levels of concurrency
	var wg sync.WaitGroup

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Each goroutine broadcasts to different users to test lock contention
			wg.Add(1)
			go func(noteIndex int) {
				defer wg.Done()
				event := NoteEvent{
					Type: "created",
					Note: notes[noteIndex%len(notes)],
				}
				hub.Broadcast(context.Background(), event)
			}(i)
			i++
		}
	})

	wg.Wait()
}

// M3: BenchmarkHub_Memory/subscribe_unsubscribe_cycle-16   	  723106	      1826 ns/op	    7488 B/op	       9 allocs/op
// M3: BenchmarkHub_Memory/user_bucket_reuse-16             	  679950	      1847 ns/op	    7488 B/op	       9 allocs/op
// BenchmarkHub_Memory measures memory usage patterns
func BenchmarkHub_Memory(b *testing.B) {
	b.Run("subscribe_unsubscribe_cycle", func(b *testing.B) {
		hub := NewHub(256)
		userID := bson.NewObjectID()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
			_, cancel := hub.Subscribe(connULID, userID)
			cancel()
		}
	})

	b.Run("user_bucket_reuse", func(b *testing.B) {
		hub := NewHub(256)
		userIDs := make([]bson.ObjectID, 10) // Limited set to test bucket reuse
		for i := range userIDs {
			userIDs[i] = bson.NewObjectID()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			userID := userIDs[i%len(userIDs)]
			connULID := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
			_, cancel := hub.Subscribe(connULID, userID)
			cancel()
		}
	})
}
