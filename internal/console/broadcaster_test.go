package console

import (
	"fmt"
	"testing"
)

func TestBroadcasterPublishesAndStoresRecent(t *testing.T) {
	b := NewBroadcaster(2)
	sub := b.Subscribe(1)
	defer b.Unsubscribe(sub.ID)

	first := Event{ID: "1", Message: "first"}
	second := Event{ID: "2", Message: "second"}
	third := Event{ID: "3", Message: "third"}
	b.Publish(first)
	got := <-sub.Events
	if got.ID != first.ID {
		t.Fatalf("expected subscriber to receive %q, got %q", first.ID, got.ID)
	}

	b.Publish(second)
	b.Publish(third)
	recent := b.Recent(10, 0)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent events, got %d", len(recent))
	}
	if recent[0].ID != second.ID || recent[1].ID != third.ID {
		t.Fatalf("unexpected recent events: %#v", recent)
	}
}

func TestBroadcasterDoesNotBlockOnSlowSubscriber(t *testing.T) {
	b := NewBroadcaster(10)
	sub := b.Subscribe(1)
	defer b.Unsubscribe(sub.ID)

	b.Publish(Event{ID: "1"})
	b.Publish(Event{ID: "2"})

	recent := b.Recent(10, 0)
	if len(recent) != 2 {
		t.Fatalf("expected both events in recent ring, got %d", len(recent))
	}
}

func TestBroadcasterRecentWithOffset(t *testing.T) {
	b := NewBroadcaster(10)
	for i := 1; i <= 5; i++ {
		b.Publish(Event{ID: fmt.Sprintf("%d", i), Message: fmt.Sprintf("event-%d", i)})
	}
	page1 := b.Recent(2, 0)
	if len(page1) != 2 || page1[0].ID != "4" || page1[1].ID != "5" {
		t.Fatalf("page 1 unexpected: %#v", page1)
	}
	page2 := b.Recent(2, 2)
	if len(page2) != 2 || page2[0].ID != "2" || page2[1].ID != "3" {
		t.Fatalf("page 2 unexpected: %#v", page2)
	}
	page3 := b.Recent(2, 4)
	if len(page3) != 1 || page3[0].ID != "1" {
		t.Fatalf("page 3 unexpected: %#v", page3)
	}
	overflow := b.Recent(10, 10)
	if len(overflow) != 0 {
		t.Fatalf("expected empty for offset beyond total, got %d", len(overflow))
	}
}
