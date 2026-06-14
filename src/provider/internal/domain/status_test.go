package domain

import "testing"

func TestDeliveryStatus_CanTransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		from DeliveryStatus
		to   DeliveryStatus
		want bool
	}{
		// Depuis Pending : autorise vers Sent, Failed, Rejected.
		{StatusPending, StatusSent, true},
		{StatusPending, StatusFailed, true},
		{StatusPending, StatusRejected, true},
		// Depuis Pending : interdit vers Delivered, Pending.
		{StatusPending, StatusDelivered, false},
		{StatusPending, StatusPending, false},
		// Depuis Sent : autorise vers Delivered, Failed, Rejected.
		{StatusSent, StatusDelivered, true},
		{StatusSent, StatusFailed, true},
		{StatusSent, StatusRejected, true},
		// Depuis Sent : interdit vers Sent, Pending.
		{StatusSent, StatusSent, false},
		{StatusSent, StatusPending, false},
		// Etats terminaux : aucune transition autorisee.
		{StatusDelivered, StatusFailed, false},
		{StatusDelivered, StatusSent, false},
		{StatusDelivered, StatusPending, false},
		{StatusFailed, StatusDelivered, false},
		{StatusFailed, StatusSent, false},
		{StatusRejected, StatusDelivered, false},
		{StatusRejected, StatusPending, false},
	}

	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.want {
			t.Errorf("CanTransitionTo(%s → %s) = %v, attendu %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestDeliveryStatus_TransitionTo_Success(t *testing.T) {
	next, err := StatusPending.TransitionTo(StatusSent)
	if err != nil {
		t.Fatalf("TransitionTo inattendu: %v", err)
	}
	if next != StatusSent {
		t.Errorf("attendu StatusSent, obtenu %s", next)
	}
}

func TestDeliveryStatus_TransitionTo_Invalid(t *testing.T) {
	_, err := StatusDelivered.TransitionTo(StatusFailed)
	if err == nil {
		t.Fatal("attendu une erreur pour transition invalide")
	}
}

func TestDeliveryStatus_IsTerminal(t *testing.T) {
	terminals := []DeliveryStatus{StatusDelivered, StatusFailed, StatusRejected}
	nonTerminals := []DeliveryStatus{StatusPending, StatusSent}

	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("IsTerminal(%s) = false, attendu true", s)
		}
	}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("IsTerminal(%s) = true, attendu false", s)
		}
	}
}
