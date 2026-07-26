package intelligenceprocessor

import "testing"

// ============================================================
// CountryFromRecipient — fonction pure, couverture exhaustive de la table
// countryPrefixesLongestFirst (voir country.go, doc de tete de fichier).
// ============================================================

func TestCountryFromRecipient(t *testing.T) {
	cases := []struct {
		name      string
		recipient string
		want      string
	}{
		// Chaque prefixe de la table (un representant par pays).
		{"Luxembourg", "+352621234567", "LU"},
		{"Cameroun", "+237699112233", "CM"},
		{"Cote d'Ivoire", "+225071234567", "CI"},
		{"Senegal", "+221771234567", "SN"},
		{"Benin", "+229961234567", "BJ"},
		{"Burkina Faso", "+22670123456", "BF"},
		{"Mali", "+22376012345", "ML"},
		{"Togo", "+22890123456", "TG"},
		{"Niger", "+22790123456", "NE"},
		{"Tchad", "+23566012345", "TD"},
		{"Gabon", "+24107012345", "GA"},
		{"Congo-Brazzaville", "+24206012345", "CG"},
		{"Congo-Kinshasa (RDC)", "+24399012345", "CD"},
		{"Madagascar", "+26134012345", "MG"},
		{"France", "+33612345678", "FR"},
		{"Belgique", "+32470123456", "BE"},
		{"Suisse", "+41791234567", "CH"},
		{"Canada", "+15145551234", "CA"},

		// Longest-prefix match : +1 (Canada, 1 chiffre) ne doit JAMAIS capturer
		// un numero commencant par +225 (Cote d'Ivoire, 3 chiffres) — les deux
		// prefixes partagent le chiffre "1"... non, +225 ne commence pas par
		// "+1" litteralement ("+225" vs "+1" : HasPrefix("+225...", "+1") est
		// faux car le 2e caractere differe ('2' != rien, en fait "+1" a pour
		// prefixe litteral les deux caracteres "+1", et "+225..." commence par
		// "+2" donc HasPrefix est deja faux) — le cas reellement dangereux
		// serait un numero +1... qui ressemblerait a +12... Le test ci-dessous
		// verifie explicitement qu'un numero CI (+225) n'est jamais classe CA.
		{"CI n'est jamais capture par le prefixe court +1 (Canada)", "+225071234567", "CI"},

		// Fallback "unknown".
		{"chaine vide", "", unknownCountry},
		{"non numerique / forme invalide", "not-a-number", unknownCountry},
		{"pas de + en tete", "225071234567", unknownCountry},
		{"chat_id Telegram numerique nu (cas documente, comportement attendu)", "123456789", unknownCountry},
		{"prefixe inconnu (pays hors PRD)", "+8613800000000", unknownCountry},
		{"juste un +", "+", unknownCountry},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountryFromRecipient(tc.recipient)
			if got != tc.want {
				t.Errorf("CountryFromRecipient(%q) = %q, voulu %q", tc.recipient, got, tc.want)
			}
			if got == "" {
				t.Errorf("CountryFromRecipient(%q) = %q, ne doit JAMAIS retourner une chaine vide (colonne NOT NULL, membre de la PK)", tc.recipient, got)
			}
		})
	}
}

// TestMatchLongestPrefix_ambiguousTable verifie, avec une table synthetique
// volontairement ambigue (un prefixe court qui est litteralement le debut
// d'un prefixe plus long), que l'ORDRE de la table (plus long d'abord) est
// bien ce qui determine le resultat — matchLongestPrefix ne fait que suivre
// l'ordre fourni, la responsabilite du tri revient a l'appelant
// (countryPrefixesLongestFirst, doc de tete de fichier country.go).
func TestMatchLongestPrefix_ambiguousTable(t *testing.T) {
	// "+12" (fictif) est litteralement le prefixe de tout numero "+1" ET
	// serait aussi capture par "+1" si "+1" etait teste en premier.
	longestFirst := []countryPrefix{
		{"+12", "XX"}, // pays fictif a 2 chiffres, plus specifique
		{"+1", "CA"},  // Canada, plus general
	}

	if got := matchLongestPrefix("+12345678", longestFirst); got != "XX" {
		t.Errorf("matchLongestPrefix avec table longest-first = %q, voulu %q (le prefixe le plus specifique doit gagner)", got, "XX")
	}
	if got := matchLongestPrefix("+19995551234", longestFirst); got != "CA" {
		t.Errorf("matchLongestPrefix(%q) = %q, voulu %q (ne matche pas +12, retombe sur +1)", "+19995551234", got, "CA")
	}

	// Si la table est fournie dans le MAUVAIS ordre (le plus court en tete),
	// le prefixe specifique "+12" n'est jamais atteint : documente le
	// comportement (matchLongestPrefix ne trie pas lui-meme, c'est une
	// responsabilite de l'appelant).
	shortestFirst := []countryPrefix{
		{"+1", "CA"},
		{"+12", "XX"},
	}
	if got := matchLongestPrefix("+12345678", shortestFirst); got != "CA" {
		t.Errorf("matchLongestPrefix avec table mal ordonnee = %q, voulu %q (demontre l'importance de l'ordre longest-first)", got, "CA")
	}
}

// TestCountryPrefixesLongestFirst_neverEmpty garde de non-regression : la
// table de production n'est jamais vide et chaque entree a un prefixe et un
// pays non vides.
func TestCountryPrefixesLongestFirst_neverEmpty(t *testing.T) {
	if len(countryPrefixesLongestFirst) == 0 {
		t.Fatal("countryPrefixesLongestFirst est vide")
	}
	for _, cp := range countryPrefixesLongestFirst {
		if cp.prefix == "" || cp.country == "" {
			t.Errorf("entree invalide dans countryPrefixesLongestFirst : %+v", cp)
		}
	}
}
