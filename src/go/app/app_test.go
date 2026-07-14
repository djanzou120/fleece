package app_test

import (
	"io"
	"testing"

	"fleece/src/go/app"
)

// mockCloser est un io.Closer factice pour tester Cleanup.
type mockCloser struct {
	closed bool
}

func (m *mockCloser) Close() error {
	m.closed = true
	return nil
}

// serviceWithClosers simule un service dont les champs sont des io.Closer.
type serviceWithClosers struct {
	A *mockCloser
	B *mockCloser
	C string // non-Closer, ne doit pas paniquer
}

// serviceWithInit simule un service qui expose Init() error.
type serviceWithInit struct {
	InitCalled bool
}

func (s *serviceWithInit) Init() error {
	s.InitCalled = true
	return nil
}

func TestBootstrap_LegacyString(t *testing.T) {
	// Comportement legacy : Bootstrap("service") ne retourne pas d'erreur.
	if err := app.Bootstrap("test-service"); err != nil {
		t.Fatalf("Bootstrap(string) unexpected error: %v", err)
	}
}

func TestCleanup_ClosesFields(t *testing.T) {
	a := &mockCloser{}
	b := &mockCloser{}
	svc := &serviceWithClosers{A: a, B: b, C: "ignored"}

	app.Cleanup(svc)

	if !a.closed {
		t.Error("Cleanup: field A (io.Closer) was not closed")
	}
	if !b.closed {
		t.Error("Cleanup: field B (io.Closer) was not closed")
	}
}

func TestCleanup_NonStructIgnored(t *testing.T) {
	// Ne doit pas paniquer sur un non-struct.
	app.Cleanup("not a struct")
}

func TestContext_ReturnsCancelFunc(t *testing.T) {
	ctx, cancel := app.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil context")
	}
	if cancel == nil {
		t.Fatal("Context() returned nil cancel func")
	}
	// Le contexte ne doit pas encore être annulé.
	select {
	case <-ctx.Done():
		t.Fatal("Context() is already cancelled")
	default:
	}
}

func TestContext_SameContext(t *testing.T) {
	ctx1, _ := app.Context()
	ctx2, _ := app.Context()
	if ctx1 != ctx2 {
		t.Error("Context() should return the same context on every call")
	}
}

// compile-time interface check: mockCloser must implement io.Closer
var _ io.Closer = (*mockCloser)(nil)
