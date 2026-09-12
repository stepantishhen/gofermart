package auth

import (
	"testing"
	"time"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "s3cret") {
		t.Fatal("valid password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestTokenIssueParse(t *testing.T) {
	m := NewTokenManager("test-secret", time.Hour)
	tok, err := m.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := m.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 42 {
		t.Fatalf("uid = %d, want 42", uid)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	tok, _ := NewTokenManager("secret-a", time.Hour).Issue(7)
	if _, err := NewTokenManager("secret-b", time.Hour).Parse(tok); err == nil {
		t.Fatal("token with wrong secret accepted")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	m := NewTokenManager("test-secret", -time.Minute)
	tok, _ := m.Issue(7)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := NewTokenManager("s", time.Hour).Parse("not-a-token"); err == nil {
		t.Fatal("garbage token accepted")
	}
}
