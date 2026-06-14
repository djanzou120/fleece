package domain

import "errors"

// ErrEndpointNotFound est retourne quand un endpoint webhook est introuvable.
var ErrEndpointNotFound = errors.New("webhook: endpoint introuvable")

// ErrMaxRetriesExceeded est retourne quand le nombre maximum de tentatives est atteint.
var ErrMaxRetriesExceeded = errors.New("webhook: nombre maximum de tentatives depasse")

// ErrInvalidSignature est retourne quand la signature HMAC-SHA256 est invalide.
var ErrInvalidSignature = errors.New("webhook: signature invalide")

// ErrEmptyPayload est retourne quand le payload d'une delivery est vide.
// Cause principale : colonne "payload" absente du schema 0006 — le payload n'est
// pas persiste et ne peut donc pas etre recharge pour un retry.
// Necessite une migration (db-engineer) avant le cablage d'un scheduler de retry.
var ErrEmptyPayload = errors.New("webhook: payload vide — ne peut pas redispatcher un body vide")
