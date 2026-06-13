package domain

import (
	"testing"
)

func TestNewMessage_InitialStatus(t *testing.T) {
	m := NewMessage("id-1", "ws-1", "+33600000000", "Bonjour")
	if m.Status != StatusCreated {
		t.Errorf("status initial attendu %s, obtenu %s", StatusCreated, m.Status)
	}
	if m.ID != "id-1" {
		t.Errorf("id attendu id-1, obtenu %s", m.ID)
	}
}

func TestTransitionTo_ValidSequence(t *testing.T) {
	m := NewMessage("id-2", "ws-1", "+33600000001", "Hello")

	steps := []Status{StatusQueued, StatusSent, StatusDelivered}
	for _, next := range steps {
		if err := m.TransitionTo(next); err != nil {
			t.Errorf("transition vers %s depuis %s refusee: %v", next, m.Status, err)
		}
	}
	if m.Status != StatusDelivered {
		t.Errorf("statut final attendu %s, obtenu %s", StatusDelivered, m.Status)
	}
}

func TestTransitionTo_InvalidTransition(t *testing.T) {
	m := NewMessage("id-3", "ws-1", "+33600000002", "Test")
	// created -> delivered est interdit
	err := m.TransitionTo(StatusDelivered)
	if err == nil {
		t.Error("transition invalide aurait du echouer")
	}
}

func TestTransitionTo_TerminalStateFailed(t *testing.T) {
	m := NewMessage("id-4", "ws-1", "+33600000003", "Test")
	_ = m.TransitionTo(StatusFailed)
	// failed -> sent est interdit
	err := m.TransitionTo(StatusSent)
	if err == nil {
		t.Error("transition depuis un etat terminal aurait du echouer")
	}
}

func TestTransitionTo_CreatedToFailed(t *testing.T) {
	m := NewMessage("id-5", "ws-1", "+33600000004", "Test")
	if err := m.TransitionTo(StatusFailed); err != nil {
		t.Errorf("created -> failed devrait etre autorise: %v", err)
	}
}
