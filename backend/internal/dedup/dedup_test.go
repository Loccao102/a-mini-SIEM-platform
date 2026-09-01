package dedup

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDedup(t *testing.T) {
	// Setup: Create Redis client for testing
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use separate DB for testing
	})
	defer redisClient.FlushDB(context.Background())
	defer redisClient.Close()

	manager := NewManager(redisClient)
	ctx := context.Background()

	// Test 1: First event creates new group
	fp := "test-fingerprint-123"
	eventID1 := "event-1"
	now := time.Now()
	
	group1, err := manager.TrackEvent(ctx, fp, eventID1, now)
	if err != nil {
		t.Fatalf("TrackEvent failed: %v", err)
	}
	if group1.Count != 1 {
		t.Errorf("expected count=1, got %d", group1.Count)
	}
	if group1.RepresentativeID != eventID1 {
		t.Errorf("expected representative=%s, got %s", eventID1, group1.RepresentativeID)
	}

	// Test 2: Second event with same fingerprint increments count
	eventID2 := "event-2"
	time.Sleep(100 * time.Millisecond)
	group2, err := manager.TrackEvent(ctx, fp, eventID2, now.Add(100*time.Millisecond))
	if err != nil {
		t.Fatalf("TrackEvent failed: %v", err)
	}
	if group2.Count != 2 {
		t.Errorf("expected count=2, got %d", group2.Count)
	}
	if group2.RepresentativeID != eventID1 {
		t.Errorf("representative should stay same, got %s", group2.RepresentativeID)
	}

	// Test 3: GetGroup returns correct data
	group3, err := manager.GetGroup(ctx, fp)
	if err != nil {
		t.Fatalf("GetGroup failed: %v", err)
	}
	if group3.Count != 2 {
		t.Errorf("expected count=2, got %d", group3.Count)
	}

	// Test 4: Different fingerprint creates new group
	fp2 := "test-fingerprint-456"
	eventID3 := "event-3"
	group4, err := manager.TrackEvent(ctx, fp2, eventID3, now)
	if err != nil {
		t.Fatalf("TrackEvent failed: %v", err)
	}
	if group4.Count != 1 {
		t.Errorf("new fingerprint should have count=1, got %d", group4.Count)
	}

	// Test 5: DeleteGroup removes entry
	if err := manager.DeleteGroup(ctx, fp); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
	groupDeleted, err := manager.GetGroup(ctx, fp)
	if err != nil {
		t.Fatalf("GetGroup after delete failed: %v", err)
	}
	if groupDeleted != nil {
		t.Errorf("expected nil after delete, got %+v", groupDeleted)
	}

	t.Logf("✓ All dedup tests passed")
}
