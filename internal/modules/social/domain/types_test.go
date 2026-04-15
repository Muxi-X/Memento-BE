package domain

import "testing"

func TestParseReactionType(t *testing.T) {
	tp, ok := ParseReactionType("inspired")
	if !ok {
		t.Fatal("ParseReactionType(inspired) ok = false, want true")
	}
	if tp != ReactionTypeInspired {
		t.Fatalf("ParseReactionType(inspired) = %q, want %q", tp, ReactionTypeInspired)
	}

	if _, ok := ParseReactionType("invalid"); ok {
		t.Fatal("ParseReactionType(invalid) ok = true, want false")
	}
}

func TestParseNotificationType(t *testing.T) {
	tp, ok := ParseNotificationType("reaction_received")
	if !ok {
		t.Fatal("ParseNotificationType(reaction_received) ok = false, want true")
	}
	if tp != NotificationTypeReactionReceived {
		t.Fatalf("ParseNotificationType(reaction_received) = %q, want %q", tp, NotificationTypeReactionReceived)
	}

	if _, ok := ParseNotificationType("invalid"); ok {
		t.Fatal("ParseNotificationType(invalid) ok = true, want false")
	}
}
