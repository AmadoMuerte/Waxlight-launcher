package updater

import (
	"context"
	"testing"
)

func TestNopSignatureVerifierReturnsErrorOnAllPlatforms(t *testing.T) {
	verifier := NewSignatureVerifier(nil)
	err := verifier.Verify(context.Background(), "/nonexistent/file.exe")
	if err == nil {
		t.Fatal("expected error from NopSignatureVerifier")
	}
}

func TestNopSignatureVerifierWithPublishers(t *testing.T) {
	verifier := NewSignatureVerifier([]string{"CN=Test Publisher"})
	err := verifier.Verify(context.Background(), "/nonexistent/file.exe")
	if err == nil {
		t.Fatal("expected error from NopSignatureVerifier")
	}
}
