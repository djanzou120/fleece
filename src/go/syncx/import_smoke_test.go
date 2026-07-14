// Package syncx — smoke test des imports des libs Go transverses.
// Ce fichier vérifie que les 5 packages src/go/* compilent et sont importables
// depuis un test Go standard. Aucune assertion d'exécution : la compilation
// seule est le critère (go build aurait déjà échoué si un import était cassé,
// mais ce fichier rend la vérification explicite et pérenne).
package syncx_test

import (
	_ "fleece/src/go/amqp"
	_ "fleece/src/go/app"
	_ "fleece/src/go/log"
	_ "fleece/src/go/sql"
)
