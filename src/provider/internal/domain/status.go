package domain

import "fmt"

// DeliveryStatus represente l'etat de livraison d'un message sortant.
type DeliveryStatus string

const (
	// StatusPending indique que le message est en attente d'envoi.
	StatusPending DeliveryStatus = "pending"
	// StatusSent indique que le message a ete transmis au fournisseur avec succes.
	StatusSent DeliveryStatus = "sent"
	// StatusDelivered indique que le message a ete livre au destinataire.
	StatusDelivered DeliveryStatus = "delivered"
	// StatusFailed indique que l'envoi du message a echoue definitivement.
	StatusFailed DeliveryStatus = "failed"
	// StatusRejected indique que le fournisseur a refuse le message.
	StatusRejected DeliveryStatus = "rejected"
)

// validTransitions definit les transitions autorisees pour chaque statut.
// La machine a etats est stricte : seules les transitions listees sont valides.
var validTransitions = map[DeliveryStatus][]DeliveryStatus{
	StatusPending:   {StatusSent, StatusFailed, StatusRejected},
	StatusSent:      {StatusDelivered, StatusFailed, StatusRejected},
	StatusDelivered: {}, // etat terminal
	StatusFailed:    {}, // etat terminal
	StatusRejected:  {}, // etat terminal
}

// CanTransitionTo retourne true si la transition depuis le statut courant
// vers next est autorisee.
func (s DeliveryStatus) CanTransitionTo(next DeliveryStatus) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == next {
			return true
		}
	}
	return false
}

// TransitionTo retourne le nouveau statut si la transition est valide.
// Retourne une erreur si la transition est interdite.
func (s DeliveryStatus) TransitionTo(next DeliveryStatus) (DeliveryStatus, error) {
	if !s.CanTransitionTo(next) {
		return s, fmt.Errorf("transition invalide: %s → %s", s, next)
	}
	return next, nil
}

// IsTerminal retourne true si le statut est un etat terminal (aucune transition possible).
func (s DeliveryStatus) IsTerminal() bool {
	transitions, ok := validTransitions[s]
	return ok && len(transitions) == 0
}
