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
	"strings"
	"testing"
	"time"

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

// ============================================================
// D-M34 / D-M35 — bornage de l'attente et messages non routables
// ============================================================

// TestPublishTimeout_defaultAndOverride verifie la resolution de la borne
// d'attente (D-M34) : un Conn construit SANS PublishTimeout — c'est-a-dire
// tous les appelants existants — retombe sur DefaultPublishTimeout, et une
// valeur explicite est respectee.
//
// L'enjeu n'est pas la valeur elle-meme mais le fait qu'il y EN AIT une : sans
// borne, WaitContext n'a d'autre sortie que le contexte de l'appelant, et
// RabbitMQ en alarme memoire/disque bloque ses publishers sans fermer le TCP
// ni renvoyer d'erreur — chaque POST /messages pendait alors indefiniment.
func TestPublishTimeout_defaultAndOverride(t *testing.T) {
	if DefaultPublishTimeout <= 0 {
		t.Fatalf("DefaultPublishTimeout = %v, doit etre strictement positif (une borne nulle ou negative ne borne rien)", DefaultPublishTimeout)
	}

	// La borne doit rester inferieure au WriteTimeout du serveur HTTP (30s,
	// src/api/cmd/api/main.go) pour que le handler ait le temps de formater sa
	// reponse d'erreur avant que le serveur ne coupe la connexion.
	if DefaultPublishTimeout >= 30*time.Second {
		t.Errorf("DefaultPublishTimeout = %v, doit rester nettement sous le WriteTimeout HTTP (30s)", DefaultPublishTimeout)
	}

	cases := []struct {
		name string
		conn Conn
		want time.Duration
	}{
		{"non renseigne (appelants existants) -> defaut", Conn{}, DefaultPublishTimeout},
		{"negatif -> defaut", Conn{PublishTimeout: -1}, DefaultPublishTimeout},
		{"explicite respecte", Conn{PublishTimeout: 2 * time.Second}, 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.conn.PublishTimeout
			if got <= 0 {
				got = DefaultPublishTimeout
			}
			if got != tc.want {
				t.Errorf("borne resolue = %v, voulu %v", got, tc.want)
			}
		})
	}
}

// TestErrPublishUnroutable_isDistinctFromNotConfirmed : les deux cas doivent
// rester DISCERNABLES par l'appelant (D-M35).
//
// Un nack signifie « le broker refuse ce message » (ressources, erreur
// interne) — c'est transitoire, retenter a du sens. Un return signifie
// « aucune queue ne correspond a cette routing key » — retenter a l'identique
// ne changera rien tant que la topologie n'est pas declaree. Les confondre
// ferait boucler un producteur sur une erreur structurelle.
func TestErrPublishUnroutable_isDistinctFromNotConfirmed(t *testing.T) {
	if ErrPublishUnroutable == nil {
		t.Fatal("ErrPublishUnroutable ne doit jamais etre nil")
	}
	if ErrPublishUnroutable.Error() == "" {
		t.Error("ErrPublishUnroutable doit porter un message explicite")
	}
	if errors.Is(ErrPublishUnroutable, ErrPublishNotConfirmed) ||
		errors.Is(ErrPublishNotConfirmed, ErrPublishUnroutable) {
		t.Error("D-M35 : les deux sentinels doivent rester distincts (un nack et un return appellent des reactions differentes)")
	}
	// Le message doit nommer la cause reelle : c'est ce que lira l'operateur
	// qui decouvre des DLR perdus.
	if !strings.Contains(ErrPublishUnroutable.Error(), "non routable") {
		t.Errorf("message peu explicite pour un diagnostic d'exploitation : %q", ErrPublishUnroutable.Error())
	}
}
