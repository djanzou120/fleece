// Package httpserver configure et démarre le serveur HTTP du service messaging.
// Couche 4 (Infrastructure).
package httpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"
)

// Server encapsule un *http.Server avec un mux et des timeouts configurés.
type Server struct {
	inner *http.Server
	mux   *http.ServeMux
}

// New crée un Server avec les timeouts recommandés.
func New(addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{
		mux: mux,
		inner: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
	return s
}

// Handle enregistre un handler sur le pattern donné.
func (s *Server) Handle(pattern string, h http.Handler) {
	s.mux.Handle(pattern, h)
}

// HandleFunc enregistre une fonction handler sur le pattern donné.
func (s *Server) HandleFunc(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, fn)
}

// Start démarre le serveur HTTP. Bloque jusqu'à ce que le serveur s'arrête.
// Retourne nil si l'arrêt est propre (ErrServerClosed).
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.inner.Addr)
	if err != nil {
		return err
	}
	log.Printf("httpserver: listening on %s", ln.Addr())
	if err := s.inner.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown arrête le serveur proprement en attendant la fin des connexions actives.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Printf("httpserver: shutting down")
	return s.inner.Shutdown(ctx)
}
