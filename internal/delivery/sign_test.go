package delivery

import (
	"testing"
)

func TestSign(t *testing.T) {
	payload := []byte(`{"amount":5000,"currency":"USD"}`)
	secret := "test-secret"

	sig := Sign(payload, secret)

	if sig[:7] != "sha256=" {
		t.Errorf("signature should start with 'sha256=', got %s", sig)
	}

	// Same input should produce same output
	sig2 := Sign(payload, secret)
	if sig != sig2 {
		t.Error("same input should produce same signature")
	}

	// Different secret should produce different signature
	sig3 := Sign(payload, "other-secret")
	if sig == sig3 {
		t.Error("different secret should produce different signature")
	}
}

func TestVerify(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "my-secret"

	sig := Sign(payload, secret)

	if !Verify(payload, sig, secret) {
		t.Error("valid signature should verify")
	}

	if Verify(payload, "sha256=invalid", secret) {
		t.Error("invalid signature should not verify")
	}

	if Verify([]byte("tampered"), sig, secret) {
		t.Error("tampered payload should not verify")
	}
}
