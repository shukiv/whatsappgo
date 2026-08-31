package events

import "testing"

func TestSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe(1)
	defer cancel()
	b.Publish(Event{Name: "one"})
	b.Publish(Event{Name: "two"})
	if got := <-ch; got.Name != "one" {
		t.Fatalf("got %q", got.Name)
	}
}
