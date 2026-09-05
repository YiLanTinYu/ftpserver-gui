package main

import (
	"strings"
	"testing"
)

func TestGeneratePassword(t *testing.T) {
	password, err := generatePassword(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != 16 {
		t.Fatalf("password length = %d, want 16", len(password))
	}
	for _, char := range password {
		if !strings.ContainsRune(passwordAlphabet, char) {
			t.Fatalf("password contains unexpected character %q", char)
		}
	}
}
