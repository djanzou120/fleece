// Package domain — entites, value objects, erreurs metier (couche 1).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package domain

import "errors"

// Erreurs metier du domaine routing.
var (
	// ErrNoProviderAvailable est retournee quand aucun provider eligible n'est disponible
	// pour le canal et le pays demandes.
	ErrNoProviderAvailable = errors.New("no_provider_available")

	// ErrInvalidStrategy est retournee quand la strategie de routage fournie
	// n'est pas reconnue.
	ErrInvalidStrategy = errors.New("invalid_routing_strategy")

	// ErrRuleNotFound est retournee quand aucune regle de routage n'est configuree
	// pour un workspace donne. Le use case retombe alors sur la strategie par defaut.
	ErrRuleNotFound = errors.New("routing_rule_not_found")

	// ErrInvalidAmount est retournee quand un montant negatif est utilise
	// dans le value object Money.
	ErrInvalidAmount = errors.New("invalid_amount")
)
