package api

import (
	"errors"
	"testing"
)

func TestNewMessage(t *testing.T) {
	m := NewMessage("id1", "ws1", "+33600000001", "hello", "sms")
	if m.Status != MessageDraft {
		t.Errorf("status initial attendu %s, obtenu %s", MessageDraft, m.Status)
	}
	if m.ID != "id1" {
		t.Errorf("ID attendu id1, obtenu %s", m.ID)
	}
	if m.Channel != "sms" {
		t.Errorf("channel attendu sms, obtenu %s", m.Channel)
	}
}

func TestMessage_Transition_valid(t *testing.T) {
	cases := []struct {
		name string
		from MessageStatus
		to   MessageStatus
	}{
		{"draft→pending", MessageDraft, MessagePending},
		{"draft→failed", MessageDraft, MessageFailed},
		{"pending→sent", MessagePending, MessageSent},
		{"pending→failed", MessagePending, MessageFailed},
		{"pending→rejected", MessagePending, MessageRejected},
		{"sent→delivered", MessageSent, MessageDelivered},
		{"sent→failed", MessageSent, MessageFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Message{ID: "x", Status: tc.from}
			if err := m.Transition(tc.to); err != nil {
				t.Errorf("transition valide %s→%s a echoue : %v", tc.from, tc.to, err)
			}
			if m.Status != tc.to {
				t.Errorf("etat attendu %s, obtenu %s", tc.to, m.Status)
			}
		})
	}
}

func TestMessage_Transition_invalid(t *testing.T) {
	cases := []struct {
		name string
		from MessageStatus
		to   MessageStatus
	}{
		{"draft→sent", MessageDraft, MessageSent},
		{"draft→delivered", MessageDraft, MessageDelivered},
		{"draft→rejected", MessageDraft, MessageRejected},
		{"pending→draft", MessagePending, MessageDraft},
		{"pending→delivered", MessagePending, MessageDelivered},
		{"sent→pending", MessageSent, MessagePending},
		{"sent→draft", MessageSent, MessageDraft},
		{"sent→rejected", MessageSent, MessageRejected},
		{"delivered→pending", MessageDelivered, MessagePending},
		{"delivered→sent", MessageDelivered, MessageSent},
		{"delivered→failed", MessageDelivered, MessageFailed},
		{"failed→pending", MessageFailed, MessagePending},
		{"failed→sent", MessageFailed, MessageSent},
		{"rejected→pending", MessageRejected, MessagePending},
		{"rejected→draft", MessageRejected, MessageDraft},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Message{ID: "x", Status: tc.from}
			err := m.Transition(tc.to)
			if err == nil {
				t.Errorf("transition invalide %s→%s aurait du echouer", tc.from, tc.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
			}
			// L'etat ne doit pas avoir change.
			if m.Status != tc.from {
				t.Errorf("l'etat ne devrait pas avoir change : attendu %s, obtenu %s", tc.from, m.Status)
			}
		})
	}
}

func TestMessage_Transition_terminalStatesAreTerminal(t *testing.T) {
	terminals := []MessageStatus{MessageDelivered, MessageFailed, MessageRejected}
	targets := []MessageStatus{MessageDraft, MessagePending, MessageSent, MessageDelivered, MessageFailed, MessageRejected}

	for _, from := range terminals {
		for _, to := range targets {
			name := string(from) + "→" + string(to)
			t.Run(name, func(t *testing.T) {
				m := &Message{ID: "x", Status: from}
				err := m.Transition(to)
				if err == nil {
					t.Errorf("depuis l'etat terminal %s, toute transition devrait echouer (tentative vers %s)", from, to)
				}
			})
		}
	}
}
