/*
File: internal/i18n/i18n_test.go
Description: Unit tests for language normalization and translation lookups.
*/

package i18n

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", En},
		{"en", En},
		{"EN", En},
		{"ru", Ru},
		{"RU", Ru},
		{"ru_RU", Ru},
		{"ru-RU", Ru},
		{"fr", En},
	}

	for _, c := range cases {
		got := Normalize(c.input)
		if got != c.expected {
			t.Errorf("Normalize(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestT(t *testing.T) {
	if got := T(En, "status_ready"); got != "Ready" {
		t.Errorf("T(En, status_ready) = %q; want 'Ready'", got)
	}
	if got := T(Ru, "status_ready"); got != "Готово" {
		t.Errorf("T(Ru, status_ready) = %q; want 'Готово'", got)
	}
	if got := T(Ru, "selected_chats", 5); got != "Выбрано: 5 чатов" {
		t.Errorf("T(Ru, selected_chats, 5) = %q; want 'Выбрано: 5 чатов'", got)
	}
	// Fallback to English when key missing in Russian
	if got := T(Ru, "non_existent_key"); got != "non_existent_key" {
		t.Errorf("T(Ru, non_existent_key) = %q; want 'non_existent_key'", got)
	}
}
