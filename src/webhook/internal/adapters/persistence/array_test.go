package persistence

import (
	"reflect"
	"testing"
)

// TestArrayLiteral_Nominal verifie que arrayLiteral encode correctement une slice.
func TestArrayLiteral_Nominal(t *testing.T) {
	cases := []struct {
		input    []string
		expected string
	}{
		{[]string{}, "{}"},
		{nil, "{}"},
		{[]string{"message.delivered"}, `{"message.delivered"}`},
		{[]string{"message.delivered", "wallet.debited"}, `{"message.delivered","wallet.debited"}`},
		{[]string{"a", "b", "c"}, `{"a","b","c"}`},
	}

	for _, tc := range cases {
		got := arrayLiteral(tc.input)
		if got != tc.expected {
			t.Errorf("arrayLiteral(%v) = %q, attendu %q", tc.input, got, tc.expected)
		}
	}
}

// TestArrayLiteral_EchappementGuillemets verifie que les guillemets internes sont echappes.
func TestArrayLiteral_EchappementGuillemets(t *testing.T) {
	input := []string{`say "hello"`, "normal"}
	got := arrayLiteral(input)
	expected := `{"say \"hello\"","normal"}`
	if got != expected {
		t.Errorf("arrayLiteral avec guillemets = %q, attendu %q", got, expected)
	}
}

// TestParseArrayLiteral_Nominal verifie que parseArrayLiteral decode correctement.
func TestParseArrayLiteral_Nominal(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"{}", nil},
		{`{"message.delivered"}`, []string{"message.delivered"}},
		{`{"message.delivered","wallet.debited"}`, []string{"message.delivered", "wallet.debited"}},
		{`{message.delivered,wallet.debited}`, []string{"message.delivered", "wallet.debited"}}, // sans guillemets
		{`{"a","b","c"}`, []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		got := parseArrayLiteral(tc.input)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("parseArrayLiteral(%q) = %v, attendu %v", tc.input, got, tc.expected)
		}
	}
}

// TestArrayLiteralRoundTrip verifie que encode -> decode est idempotent.
func TestArrayLiteralRoundTrip(t *testing.T) {
	inputs := [][]string{
		{"message.delivered"},
		{"message.created", "message.sent", "message.delivered", "message.failed"},
		{"wallet.debited", "wallet.refunded"},
	}

	for _, original := range inputs {
		encoded := arrayLiteral(original)
		decoded := parseArrayLiteral(encoded)
		if !reflect.DeepEqual(original, decoded) {
			t.Errorf("round-trip %v → %q → %v : perte de donnees", original, encoded, decoded)
		}
	}
}
