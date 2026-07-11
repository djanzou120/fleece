// Package domain contient les entites, value objects, regles metier et erreurs
// du service Contact Intelligence. Il est pur : zero import externe.
package domain

import "errors"

// Erreurs metier du domaine Contact Intelligence.
var (
	// ErrEmptyPhone est retournee quand le numero de telephone est vide.
	ErrEmptyPhone = errors.New("contact_intel: phone ne peut pas etre vide")

	// ErrInvalidChannel est retournee quand le canal est inconnu ou vide.
	ErrInvalidChannel = errors.New("contact_intel: canal inconnu ou vide")

	// ErrContactNotFound est retournee quand le contact n'existe pas en base.
	ErrContactNotFound = errors.New("contact_intel: contact introuvable")
)
