package testkit_test

import (
	"testing"
	"testing/quick"

	"github.com/dpopsuev/battery/event"
	"github.com/dpopsuev/battery/testkit"
)

func TestEventLog_Property_SinceLenConsistent(t *testing.T) {
	f := func(n uint8) bool {
		log := testkit.NewStubEventLog()
		count := int(n % 20)
		for range count {
			log.Emit(event.Event{Source: "test", Kind: "p"})
		}
		for i := range count {
			got := len(log.Since(i))
			want := count - i
			if got != want {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestEventLog_Property_SinceOrdered(t *testing.T) {
	f := func(n uint8) bool {
		log := testkit.NewStubEventLog()
		count := int(n%10) + 1
		for i := range count {
			log.Emit(event.Event{Source: "test", Kind: string(rune('a' + i))})
		}
		events := log.Since(0)
		for i := 1; i < len(events); i++ {
			if !events[i].Timestamp.After(events[i-1].Timestamp) &&
				!events[i].Timestamp.Equal(events[i-1].Timestamp) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatal(err)
	}
}

func TestEventLog_Property_NegativeSinceReturnsAll(t *testing.T) {
	log := testkit.NewStubEventLog()
	for range 5 {
		log.Emit(event.Event{Source: "test", Kind: "x"})
	}
	if len(log.Since(-999)) != 5 {
		t.Fatalf("Since(-999) = %d, want 5", len(log.Since(-999)))
	}
}

func TestEventLog_EmitZeroValue_NoPanic(t *testing.T) {
	log := testkit.NewStubEventLog()
	log.Emit(event.Event{}) // zero value — no panic
	if log.Len() != 1 {
		t.Fatalf("Len = %d", log.Len())
	}
}
