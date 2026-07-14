// Package app — configuration transverse via zconfig.
//
// Ce fichier reproduit le pattern winmarket/backend/src/go/app/config.go :
// un processeur zconfig global avec deux fournisseurs (args CLI, variables d'environnement)
// et le hook Initialize (qui appelle Init() sur les sous-champs le supportant).
//
// Bootstrap() appelle Configure() puis, si le service racine expose Init() error,
// l'invoque explicitement. zconfig gère l'init des sous-composants injectés via
// le hook Initialize ; Bootstrap n'appelle Init() que sur la racine pour éviter
// tout double-appel sur les sous-champs.
package app

import (
	"os"
	"strings"

	"github.com/synthesio/zconfig"
)

var (
	defaultRepository zconfig.Repository
	defaultProcessor  zconfig.Processor
	argsProvider      = NewArgsProvider()
	envProvider       = zconfig.NewEnvProvider()
)

func init() {
	defaultRepository.AddProviders(argsProvider, envProvider)
	defaultRepository.AddParsers(zconfig.ParseString)
	defaultProcessor.AddHooks(defaultRepository.Hook, zconfig.Initialize)
}

// Configure traite la struct service via zconfig (env + args CLI, hook Initialize).
func Configure(s interface{}) error {
	return defaultProcessor.Process(s)
}

// Args retourne les arguments positionnels (non-flags) de la ligne de commande.
func Args() []string {
	return argsProvider.PositionalArgs
}

// ArgsProvider est un fournisseur zconfig qui parse os.Args.
// Adapté depuis le dépôt synthesio/zconfig avec support des arguments positionnels.
type ArgsProvider struct {
	Args           map[string]string
	PositionalArgs []string
}

// NewArgsProvider construit et remplit un ArgsProvider depuis os.Args.
func NewArgsProvider() (p *ArgsProvider) {
	p = new(ArgsProvider)
	p.Args = make(map[string]string, len(os.Args))

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]

		if !strings.HasPrefix(arg, "--") {
			p.PositionalArgs = append(p.PositionalArgs, arg)
			continue
		}

		arg = strings.TrimPrefix(arg, "--")
		parts := strings.SplitN(arg, "=", 2)

		if len(parts) == 1 && i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "--") {
			parts = append(parts, os.Args[i+1])
			i++
		}

		parts = append(parts, "")
		p.Args[parts[0]] = parts[1]
	}

	return p
}

// Retrieve implémente l'interface zconfig.Provider.
func (p *ArgsProvider) Retrieve(key string) (value interface{}, found bool, err error) {
	value, found = p.Args[key]
	return value, found, nil
}

// Name implémente l'interface zconfig.Provider.
func (ArgsProvider) Name() string {
	return "args"
}

// Priority implémente l'interface zconfig.Provider (args > env).
func (ArgsProvider) Priority() int {
	return 1
}
