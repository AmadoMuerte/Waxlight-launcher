//go:build windows

package updater

import (
	"encoding/json"
	"testing"
)

func TestPowerShellSignatureStatusAcceptsNumericEnum(t *testing.T) {
	var result powershellSignatureResult
	if err := json.Unmarshal([]byte(`{"Status":2,"StatusMessage":"Not signed","SignerCertificate":null,"Path":"C:\\\\update.exe"}`), &result); err != nil {
		t.Fatalf("numeric PowerShell status must parse: %v", err)
	}
	if result.Status != signatureStatusNotSigned {
		t.Fatalf("expected %q, got %q", signatureStatusNotSigned, result.Status)
	}
}

func TestPowerShellSignatureStatusAcceptsStringEnum(t *testing.T) {
	var result powershellSignatureResult
	if err := json.Unmarshal([]byte(`{"Status":"Valid","StatusMessage":"","SignerCertificate":{"Subject":"CN=Waxlight","Thumbprint":"AA BB"},"Path":"C:\\\\update.exe"}`), &result); err != nil {
		t.Fatalf("string PowerShell status must parse: %v", err)
	}
	if result.Status != signatureStatusValid {
		t.Fatalf("expected %q, got %q", signatureStatusValid, result.Status)
	}
}

func TestUnknownNumericSignatureStatusFailsClosed(t *testing.T) {
	var result powershellSignatureResult
	if err := json.Unmarshal([]byte(`{"Status":99}`), &result); err != nil {
		t.Fatalf("unknown numeric status should still parse: %v", err)
	}
	if result.Status != "Unknown(99)" {
		t.Fatalf("expected unknown status marker, got %q", result.Status)
	}
}

func TestPublisherMatchesSubjectCaseInsensitively(t *testing.T) {
	if !publisherMatches("CN=Waxlight Launcher", "", "cn=wAXLIGHT launcher") {
		t.Fatal("publisher subject should match case-insensitively")
	}
}

func TestPublisherMatchesNormalizedThumbprint(t *testing.T) {
	if !publisherMatches("", "AA BB-CC:DD", "aabbccdd") {
		t.Fatal("certificate thumbprints should ignore common separators")
	}
}
