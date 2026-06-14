package http

import (
	"crypto/rand"
	"fmt"
)

// newUUID genere un identifiant UUID v4 aleatoire via crypto/rand (stdlib uniquement).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// Positionner les bits de version (4) et de variant (RFC 4122).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
