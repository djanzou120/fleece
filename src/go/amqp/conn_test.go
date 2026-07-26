package goamqp

// conn_test.go — tests unitaires des briques PURES de ce paquet.
//
// LIMITE ASSUMEE (D-M29, Phase 3) : Conn.Init/Channel/Publish/Consume/
// DeclareExchange/DeclareQueue/BindQueue/EnsureQueueBound reposent tous sur
// une connexion amqp091.Connection RÉELLE (établie par amqp091.Dial dans
// Init) — le paquet github.com/rabbitmq/amqp091-go n'expose aucune interface
// de canal/connexion mockable, et ce dépôt ne dispose d'aucun faux serveur
// AMQP (contrairement au faux driver database/sql/driver réutilisé par
// src/core-processor et src/intelligence-processor pour PostgreSQL). Ces
// méthodes ne sont donc PAS unit-testables ici sans une instance RabbitMQ
// réelle (absente de cet environnement offline) ; c'était déjà le cas AVANT
// ce correctif (0 fichier de test dans ce paquet). Seule la logique PURE
// extraite de Publish (buildPersistentPublishing) est testée ci-dessous — une
// vérification d'intégration réelle (topologie déclarée, publication
// confirmée/rejetée) resterait à couvrir séparément (docker-compose RabbitMQ
// + `go test -tags=integration`, hors périmètre de ce round offline).
import (
	"errors"
	"testing"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

func TestBuildPersistentPublishing(t *testing.T) {
	body := []byte(`{"message_id":"x"}`)
	got := buildPersistentPublishing(body)

	if got.DeliveryMode != amqp091.Persistent {
		t.Errorf("DeliveryMode = %v, voulu amqp091.Persistent (%v) — D-M29 : un message non persistant est perdu au redemarrage du broker", got.DeliveryMode, amqp091.Persistent)
	}
	if got.ContentType != "application/json" {
		t.Errorf("ContentType = %q, voulu %q", got.ContentType, "application/json")
	}
	if string(got.Body) != string(body) {
		t.Errorf("Body = %q, voulu %q", got.Body, body)
	}
}

func TestErrPublishNotConfirmed_isDistinctSentinel(t *testing.T) {
	if ErrPublishNotConfirmed == nil {
		t.Fatal("ErrPublishNotConfirmed ne doit jamais etre nil")
	}
	if ErrPublishNotConfirmed.Error() == "" {
		t.Error("ErrPublishNotConfirmed doit porter un message explicite")
	}
}

// TestInterpretConfirmation couvre le coeur du correctif D-M29 : une
// publication qui n'a PAS ete confirmee par le broker (nack explicite, ou une
// erreur survenue pendant l'attente de la confirmation) DOIT desormais
// retourner une erreur — jamais nil, contrairement au comportement AVANT ce
// correctif (PublishWithContext retournait nil des que le message etait
// accepte par le canal LOCAL, avant toute confirmation reelle du broker).
func TestInterpretConfirmation(t *testing.T) {
	t.Run("ack recu -> nil (chemin nominal inchange)", func(t *testing.T) {
		if err := interpretConfirmation(true, nil); err != nil {
			t.Errorf("interpretConfirmation(true, nil) = %v, voulu nil", err)
		}
	})

	t.Run("nack explicite -> ErrPublishNotConfirmed", func(t *testing.T) {
		err := interpretConfirmation(false, nil)
		if !errors.Is(err, ErrPublishNotConfirmed) {
			t.Errorf("interpretConfirmation(false, nil) = %v, voulu ErrPublishNotConfirmed", err)
		}
	})

	t.Run("erreur pendant l'attente -> erreur non nil (jamais nil silencieux)", func(t *testing.T) {
		wantCause := errors.New("connexion perdue pendant l'attente")
		err := interpretConfirmation(false, wantCause)
		if err == nil {
			t.Fatal("interpretConfirmation(false, err) attendu en erreur")
		}
		if !errors.Is(err, wantCause) {
			t.Errorf("interpretConfirmation(false, err) = %v, ne wrap pas la cause %v", err, wantCause)
		}
	})
}
