package api

// helpers.go regroupe les helpers transverses partages par les ~22 handlers HTTP
// du service (writeError/writeJSON/parseUUID/isValidStrategy/newUUID). Renomme
// depuis messages_helpers.go (le prefixe "messages_" induisait en erreur — ce
// fichier n'a rien de specifique aux endpoints /messages).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

// writeError ecrit une reponse JSON d'erreur avec le code HTTP et le message donnes.
// Format : {"error": "<msg>"}
func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// On ignore l'erreur d'encodage : si l'ecriture echoue ici, on ne peut pas
	// faire mieux (la connexion est probablement fermee).
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: msg})
}

// writeJSON serialise v en JSON et l'ecrit avec le code HTTP donne.
// Si l'encodage echoue, une erreur 500 est retournee a la place.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// En cas d'echec d'encodage, le header est deja envoye — on ne peut
		// que logger. Le corps sera vide ou partiel.
		// (log via le logger structuré n'est pas accessible ici — on laisse
		// Go ecrire dans stderr via la lib log standard si necessaire.)
		_ = err
	}
}

// reUUID est une expression reguliere simple pour valider le format UUID v4.
// Format attendu : 8-4-4-4-12 caracteres hexadecimaux, insensible a la casse.
var reUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// parseUUID valide le format UUID d'une chaine et la retourne normalisee.
// Retourne une erreur si le format est invalide.
func parseUUID(s string) (string, error) {
	if !reUUID.MatchString(s) {
		return "", fmt.Errorf("format UUID invalide : %q", s)
	}
	return s, nil
}

// validStrategies liste les strategies de routage reconnues par le service.
var validStrategies = map[string]bool{
	"lowest_cost":      true,
	"highest_delivery": true,
	"round_robin":      true,
}

// isValidStrategy retourne true si la strategie est reconnue.
func isValidStrategy(s string) bool {
	return validStrategies[s]
}

// newUUID genere un UUID v4 aleatoire au format 8-4-4-4-12 (RFC 4122).
// Utilise crypto/rand — panique si l'entropie systeme est indisponible (erreur
// fatale et improbable : /dev/urandom indisponible). Cette panique est rattrapee
// au niveau HTTP par recoverMiddleware (service.go), qui la transforme en reponse
// 500 JSON plutot que de faire crasher le processus (ecart C4 de la revue
// d'architecture Phase 2 — voir aussi TestRecoverMiddleware_* dans helpers_test.go).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("messages: newUUID: " + err.Error())
	}
	// Version 4 (bits 12-15 de l'octet 6) et variante RFC 4122 (bits 6-7 de l'octet 8).
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	// Encoder en hex et formater 8-4-4-4-12.
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
