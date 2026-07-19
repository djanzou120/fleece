package api

import (
	"errors"
	"testing"
	"time"
)

func TestNewCampaign(t *testing.T) {
	c := NewCampaign("c1", "ws1", "Black Friday", "sms")
	if c.Status != CampaignDraft {
		t.Errorf("status initial attendu %s, obtenu %s", CampaignDraft, c.Status)
	}
	if c.ID != "c1" {
		t.Errorf("ID attendu c1, obtenu %s", c.ID)
	}
	if c.ScheduledAt != nil {
		t.Error("ScheduledAt doit etre nil apres creation")
	}
}

func TestCampaign_Transition_valid(t *testing.T) {
	cases := []struct {
		name string
		from CampaignStatus
		to   CampaignStatus
	}{
		{"draft→cancelled", CampaignDraft, CampaignCancelled},
		{"scheduled→running", CampaignScheduled, CampaignRunning},
		{"scheduled→cancelled", CampaignScheduled, CampaignCancelled},
		{"running→completed", CampaignRunning, CampaignCompleted},
		{"running→failed", CampaignRunning, CampaignFailed},
		{"running→paused", CampaignRunning, CampaignPaused},
		{"paused→running", CampaignPaused, CampaignRunning},
		{"paused→cancelled", CampaignPaused, CampaignCancelled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Campaign{ID: "c", Status: tc.from}
			if err := c.Transition(tc.to); err != nil {
				t.Errorf("transition valide %s→%s a echoue : %v", tc.from, tc.to, err)
			}
			if c.Status != tc.to {
				t.Errorf("etat attendu %s, obtenu %s", tc.to, c.Status)
			}
		})
	}
}

func TestCampaign_Transition_invalid(t *testing.T) {
	cases := []struct {
		name string
		from CampaignStatus
		to   CampaignStatus
	}{
		{"draft→running", CampaignDraft, CampaignRunning},
		{"draft→completed", CampaignDraft, CampaignCompleted},
		{"draft→failed", CampaignDraft, CampaignFailed},
		{"draft→paused", CampaignDraft, CampaignPaused},
		{"scheduled→draft", CampaignScheduled, CampaignDraft},
		{"scheduled→completed", CampaignScheduled, CampaignCompleted},
		{"scheduled→failed", CampaignScheduled, CampaignFailed},
		{"scheduled→paused", CampaignScheduled, CampaignPaused},
		{"running→draft", CampaignRunning, CampaignDraft},
		{"running→scheduled", CampaignRunning, CampaignScheduled},
		{"running→cancelled", CampaignRunning, CampaignCancelled},
		{"paused→completed", CampaignPaused, CampaignCompleted},
		{"paused→failed", CampaignPaused, CampaignFailed},
		{"completed→running", CampaignCompleted, CampaignRunning},
		{"failed→running", CampaignFailed, CampaignRunning},
		{"cancelled→draft", CampaignCancelled, CampaignDraft},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Campaign{ID: "c", Status: tc.from}
			err := c.Transition(tc.to)
			if err == nil {
				t.Errorf("transition invalide %s→%s aurait du echouer", tc.from, tc.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
			}
			if c.Status != tc.from {
				t.Errorf("l'etat ne devrait pas avoir change : attendu %s, obtenu %s", tc.from, c.Status)
			}
		})
	}
}

func TestCampaign_Schedule_valid(t *testing.T) {
	c := NewCampaign("c1", "ws1", "Promo", "sms")
	c.MessageBody = "Bonjour, voici notre offre !"
	at := time.Now().UTC().Add(24 * time.Hour)

	if err := c.Schedule(at); err != nil {
		t.Fatalf("Schedule valide a echoue : %v", err)
	}
	if c.Status != CampaignScheduled {
		t.Errorf("etat attendu %s, obtenu %s", CampaignScheduled, c.Status)
	}
	if c.ScheduledAt == nil {
		t.Error("ScheduledAt ne doit pas etre nil apres Schedule")
	}
}

func TestCampaign_Schedule_missingMessageBody(t *testing.T) {
	c := NewCampaign("c1", "ws1", "Promo", "sms")
	// MessageBody vide (valeur zero)

	err := c.Schedule(time.Now().UTC().Add(time.Hour))
	if err == nil {
		t.Fatal("Schedule sans MessageBody devrait echouer")
	}
	if !errors.Is(err, ErrMissingMessageBody) {
		t.Errorf("erreur attendue ErrMissingMessageBody, obtenu %v", err)
	}
	// L'etat ne doit pas avoir change.
	if c.Status != CampaignDraft {
		t.Errorf("l'etat ne devrait pas avoir change : attendu %s, obtenu %s", CampaignDraft, c.Status)
	}
	// ScheduledAt doit rester nil.
	if c.ScheduledAt != nil {
		t.Error("ScheduledAt doit rester nil apres un Schedule echoue")
	}
}

func TestCampaign_Schedule_invalidState(t *testing.T) {
	c := &Campaign{ID: "c1", Status: CampaignRunning, MessageBody: "corps"}
	err := c.Schedule(time.Now().UTC().Add(time.Hour))
	if err == nil {
		t.Fatal("Schedule depuis un etat non-draft devrait echouer")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("erreur attendue ErrInvalidTransition, obtenu %v", err)
	}
}

func TestCampaign_Schedule_rollbackOnTransitionFailure(t *testing.T) {
	// Si la transition echoue (etat non-draft), ScheduledAt ne doit pas etre modifie.
	c := &Campaign{ID: "c1", Status: CampaignCompleted, MessageBody: "corps"}
	originalScheduledAt := c.ScheduledAt

	_ = c.Schedule(time.Now().UTC().Add(time.Hour))

	if c.ScheduledAt != originalScheduledAt {
		t.Error("ScheduledAt ne devrait pas avoir change apres un Schedule echoue")
	}
}

func TestCampaign_Transition_terminalStatesAreTerminal(t *testing.T) {
	terminals := []CampaignStatus{CampaignCompleted, CampaignFailed, CampaignCancelled}
	targets := []CampaignStatus{CampaignDraft, CampaignScheduled, CampaignRunning, CampaignPaused, CampaignCompleted, CampaignFailed, CampaignCancelled}

	for _, from := range terminals {
		for _, to := range targets {
			name := string(from) + "→" + string(to)
			t.Run(name, func(t *testing.T) {
				c := &Campaign{ID: "c", Status: from}
				err := c.Transition(to)
				if err == nil {
					t.Errorf("depuis l'etat terminal %s, toute transition devrait echouer (tentative vers %s)", from, to)
				}
			})
		}
	}
}
