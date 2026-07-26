package intelligenceprocessor

// country.go — CountryFromRecipient : heuristique PURE de derivation du code
// pays ISO 3166-1 alpha-2 depuis un destinataire E.164, utilisee pour
// alimenter analytics.message_daily.country (colonne `text NOT NULL`, membre
// de la cle primaire (day, workspace_id, country, channel) — migration
// 0009_analytics.sql).
//
// DECISION PM (Decision B, M-021) — PAS DE MIGRATION : messaging.messages n'a
// aucune colonne "country" (verifie : migrations 0001..0019, aucune
// n'introduit une telle colonne). Plutot que d'ajouter une colonne, la
// derivation se fait ICI, a la volee, a partir de messaging.messages.recipient
// (E.164) relu de facon autonome (voir message_row.go). Cette fonction EST
// une heuristique, documentee comme telle : la source de verite serait une
// colonne `country` additive sur messaging.messages, alimentee au moment de
// l'envoi (le provider/l'operateur connait parfois le pays reel avec plus de
// certitude qu'un simple prefixe E.164 — numeros portes, VOIP, MVNO
// internationaux). Cette dette est tracee par le PM sous la reference D-M23
// (voir .ia/MIGRATION_PLAN.md, rapport de tache M-021).
//
// Fallback : "unknown" — JAMAIS "" ni NULL. La colonne est NOT NULL et fait
// partie de la cle primaire : une chaine vide y romprait la lisibilite des
// KPIs (une ligne "pays = ''" est indiscernable d'un bug de troncature) sans
// pour autant etre rejetee par la contrainte NOT NULL (une chaine vide reste
// une valeur valide au sens SQL) — "unknown" est explicite et sans ambiguite.
//
// CAS TELEGRAM (important, ne pas contourner) : un destinataire Telegram est
// un chat_id NUMERIQUE SANS "+" (ex. "123456789"), pas un numero de telephone.
// Il ne matche donc aucun prefixe E.164 ci-dessous et retourne "unknown" —
// c'est le comportement CORRECT et attendu (Telegram est aujourd'hui le seul
// provider cable, voir src/api/providers/telegram.go — la tres grande
// majorite des lignes analytics.message_daily aura donc country="unknown"
// tant qu'un provider SMS/WhatsApp reel n'est pas cable, ce qui est normal et
// pas un bug de cette fonction).
import "strings"

// countryPrefix associe un prefixe E.164 (avec le "+") a un code pays ISO
// 3166-1 alpha-2.
type countryPrefix struct {
	prefix  string
	country string
}

// countryPrefixesLongestFirst couvre les marches cibles du PRD (Afrique
// francophone + Europe, + Canada francophone) — liste minimale imposee par le
// PM (M-021) : CM, CI, SN, BJ, BF, ML, TG, NE, TD, GA, CG, CD, MG, FR, BE, CH,
// LU, CA.
//
// ORDRE : du prefixe le plus long (en nombre de chiffres, sans compter le
// "+") au plus court — "longest-prefix match". Necessaire en general des lors
// qu'un prefixe court peut etre le debut litteral d'un prefixe plus long
// present dans la table (ex. si un jour "+1" et "+12" coexistaient, il
// faudrait tester "+12" AVANT "+1" pour ne pas capturer a tort tous les
// numeros commencant par "+12" sous le code du prefixe court). Dans la liste
// actuelle aucun prefixe n'est litteralement le prefixe d'un autre, mais
// l'ordre est neanmoins applique par discipline defensive — voir
// matchLongestPrefix (country_test.go) qui verifie ce comportement avec une
// table synthetique volontairement ambigue.
var countryPrefixesLongestFirst = []countryPrefix{
	// Prefixes a 3 chiffres.
	{"+352", "LU"}, // Luxembourg
	{"+237", "CM"}, // Cameroun
	{"+225", "CI"}, // Côte d'Ivoire
	{"+221", "SN"}, // Sénégal
	{"+229", "BJ"}, // Bénin
	{"+226", "BF"}, // Burkina Faso
	{"+223", "ML"}, // Mali
	{"+228", "TG"}, // Togo
	{"+227", "NE"}, // Niger
	{"+235", "TD"}, // Tchad
	{"+241", "GA"}, // Gabon
	{"+242", "CG"}, // Congo-Brazzaville
	{"+243", "CD"}, // Congo-Kinshasa (RDC)
	{"+261", "MG"}, // Madagascar
	// Prefixes a 2 chiffres.
	{"+33", "FR"}, // France
	{"+32", "BE"}, // Belgique
	{"+41", "CH"}, // Suisse
	// Prefixe a 1 chiffre.
	{"+1", "CA"}, // Canada (marche cible PRD : Canada francophone)
}

// unknownCountry est le fallback retourne quand aucun prefixe ne matche, ou
// que le destinataire n'a pas la forme d'un numero E.164 (ex. chat_id
// Telegram numerique sans "+", chaine vide).
const unknownCountry = "unknown"

// CountryFromRecipient derive le code pays ISO 3166-1 alpha-2 d'un
// destinataire E.164. Fonction pure (aucune I/O), fallback "unknown" — jamais
// "" ni NULL (voir doc de tete de fichier).
func CountryFromRecipient(recipient string) string {
	if recipient == "" || !strings.HasPrefix(recipient, "+") {
		return unknownCountry
	}
	return matchLongestPrefix(recipient, countryPrefixesLongestFirst)
}

// matchLongestPrefix retourne le pays du PREMIER prefixe de la table qui
// matche recipient. Fonction pure extraite pour etre testable avec une table
// synthetique (voir country_test.go, cas "prefixe ambigu") independamment de
// countryPrefixesLongestFirst.
func matchLongestPrefix(recipient string, table []countryPrefix) string {
	for _, cp := range table {
		if strings.HasPrefix(recipient, cp.prefix) {
			return cp.country
		}
	}
	return unknownCountry
}
