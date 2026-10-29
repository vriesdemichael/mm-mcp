package server

import (
	"context"
	"testing"
	"time"
)

// Presses on one draft take turns, and presses on others do not wait for
// them: the turns that keep a proposal pressed in two views from posting
// twice, where no request is involved.
func TestPressesOnOneDraftTakeTurns(t *testing.T) {
	t.Parallel()
	first, err := holdDraft(t.Context(), "turns-u1", "c1", "")
	if err != nil {
		t.Fatal(err)
	}
	// Another draft of the user's, a thread's, and another user's go at once.
	for _, other := range [][3]string{{"turns-u1", "c2", ""}, {"turns-u1", "c1", "r1"}, {"turns-u2", "c1", ""}} {
		release, err := holdDraft(t.Context(), other[0], other[1], other[2])
		if err != nil {
			t.Fatalf("%v waited for another draft: %v", other, err)
		}
		release()
	}

	// The same draft waits for the press before it.
	next := make(chan func())
	go func() {
		release, err := holdDraft(context.Background(), "turns-u1", "c1", "")
		if err != nil {
			t.Error(err)
		}
		next <- release
	}()
	select {
	case <-next:
		t.Fatal("a second press on the draft went while the first held it")
	case <-time.After(100 * time.Millisecond):
	}
	// A press whose call ends while it waits gives up its place.
	ended, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := holdDraft(ended, "turns-u1", "c1", ""); err == nil {
		t.Error("a press whose call ended went anyway")
	}
	first()
	select {
	case release := <-next:
		release()
	case <-time.After(5 * time.Second):
		t.Fatal("the second press never went")
	}

	presses.Lock()
	defer presses.Unlock()
	for key := range presses.held {
		if len(key) > 6 && key[:6] == "turns-" {
			t.Errorf("%s is still held once every press is done", key)
		}
	}
}
