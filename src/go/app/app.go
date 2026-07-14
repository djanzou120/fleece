// Package app regroupe les métadonnées et l'amorçage communs aux services Go.
//
// Version et Name sont injectés au build via -ldflags (voir mk/go.mk).
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package app

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"reflect"
	"syscall"
)

var (
	// Version est injectée au build (-ldflags).
	Version = "dev"
	// Name est injecté au build (nom du service).
	Name = "unknown"
)

// appCtx est le contexte racine de l'application, annulé sur SIGINT/SIGTERM.
// Il est initialisé une seule fois dans init() et partagé par tous les appels à Context().
var appCtx context.Context
var appCancel context.CancelFunc

func init() {
	appCtx, appCancel = context.WithCancel(context.Background())

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
		<-done
		appCancel()
	}()
}

// Bootstrap configure le service via zconfig puis appelle sa méthode Init() error
// si elle est définie sur le service racine.
//
// Choix double-init : zconfig avec le hook Initialize appelle Init() sur les
// sous-champs injectés qui implémentent l'interface zconfig.Initializer. Bootstrap
// appelle explicitement Init() uniquement sur le service racine, qui n'est pas
// lui-même traité par le hook Initialize de zconfig. Il n'y a donc pas de
// double-appel sur les sous-composants.
//
// Compatibilité descendante : les appelants existants passent une string
// (ex. app.Bootstrap("messaging")) ; string satisfait interface{}, Configure() ne
// retourne pas d'erreur sur une valeur scalaire non-struct, et l'erreur peut être
// ignorée sans casser le build Go. La valeur de retour est ignorable.
func Bootstrap(service interface{}) error {
	// Si service est une string (appel legacy), utiliser comme nom de service.
	if name, ok := service.(string); ok {
		if Name == "unknown" {
			Name = name
		}
		log.Printf("starting service=%s version=%s", Name, Version)
		return nil
	}

	// Pour une struct, configurer via zconfig.
	if err := Configure(service); err != nil {
		return err
	}

	// Appeler Init() sur le service racine s'il l'expose.
	// Les sous-champs sont déjà initialisés par le hook zconfig.Initialize.
	v := reflect.ValueOf(service)
	if m := v.MethodByName("Init"); m.IsValid() {
		results := m.Call(nil)
		if len(results) == 1 && !results[0].IsNil() {
			return results[0].Interface().(error)
		}
	}

	log.Printf("starting service=%s version=%s", Name, Version)
	return nil
}

// Cleanup ferme tous les champs du service qui implémentent io.Closer.
// L'opération est best-effort : les erreurs de fermeture sont ignorées car
// Cleanup est destiné à l'arrêt gracieux du processus.
//
// Implémentation : pattern winmarket — itère sur les champs directs (non récursif)
// de la struct (déréférence les pointeurs) et ferme chaque Closer. Le service
// racine lui-même n'est pas fermé si son type implémente io.Closer ; seuls les
// champs sont concernés.
func Cleanup(s interface{}) {
	v := reflect.ValueOf(s)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	typeCloser := reflect.TypeOf((*io.Closer)(nil)).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.Type().Implements(typeCloser) {
			continue
		}
		_ = f.Interface().(io.Closer).Close()
	}
}

// Context retourne le contexte racine de l'application et sa fonction d'annulation.
// Le contexte est créé une seule fois (dans init()) et annulé automatiquement
// à la réception de SIGINT ou SIGTERM.
//
// La CancelFunc retournée permet à l'appelant d'annuler manuellement le contexte
// (ex. en cas d'erreur fatale au démarrage). Plusieurs appels à cancel() sont
// sans effet après le premier.
func Context() (context.Context, context.CancelFunc) {
	return appCtx, appCancel
}
