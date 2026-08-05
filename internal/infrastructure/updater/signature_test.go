package updater

import (
	"context"
	"testing"
)

func TestSignatureVerifierAllowsChecksumOnlyModeWithoutPublishers(t *testing.T) {
	verifier := NewSignatureVerifier(nil)
	if err := verifier.Verify(context.Background(), "/nonexistent/file.exe"); err != nil {
		t.Fatalf("checksum-only mode must not require Authenticode: %v", err)
	}
}

func TestSignatureVerifierIgnoresBlankPublisherConfiguration(t *testing.T) {
	verifier := NewSignatureVerifier([]string{"", "   ", "\t"})
	if err := verifier.Verify(context.Background(), "/nonexistent/file.exe"); err != nil {
		t.Fatalf("blank publisher entries must keep checksum-only mode enabled: %v", err)
	}
}
