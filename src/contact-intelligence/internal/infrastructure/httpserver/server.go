// Package httpserver configure et demarre le serveur HTTP du service contact-intelligence.
// Couche 4 (Infrastructure).
package httpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"
)

// Server encapsule un *http.Server avec un mux et des timeouts configures.
type Server struct {
	inner *http.Server
	mux   *http.ServeMux
}

// New cree un Server avec les timeouts recommandes.
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

// HandleFunc enregistre une fonction handler sur le pattern donne.
func (s *Server) HandleFunc(pattern string, fn http.HandlerFunc) {
	s.mux.HandleFunc(pattern, fn)
}

// Start demarre le serveur HTTP. Bloque jusqu'a ce que le serveur s'arrete.
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

// Shutdown arrete le serveur proprement.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Printf("httpserver: shutting down")
	return s.inner.Shutdown(ctx)
}
