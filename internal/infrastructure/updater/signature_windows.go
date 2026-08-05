//go:build windows

package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type powershellSignatureResult struct {
	Status            string `json:"Status"`
	StatusMessage     string `json:"StatusMessage"`
	SignerCertificate struct {
		Subject    string `json:"Subject"`
		Thumbprint string `json:"Thumbprint"`
	} `json:"SignerCertificate"`
	Path string `json:"Path"`
}

type WindowsSignatureVerifier struct {
	TrustedPublishers []string
}

func NewSignatureVerifier(trustedPublishers []string) *WindowsSignatureVerifier {
	return &WindowsSignatureVerifier{TrustedPublishers: trustedPublishers}
}

func (v *WindowsSignatureVerifier) Verify(ctx context.Context, executablePath string) error {
	result, err := v.getSignature(ctx, executablePath)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	switch result.Status {
	case "Valid":
		// Signature is cryptographically valid. Now check publisher.
	case "NotSigned":
		return ErrSignatureMissing
	case "HashMismatch":
		return ErrSignatureInvalid
	case "NotTrusted":
		return ErrSignatureNotTrusted
	default:
		return fmt.Errorf("%w: %s", ErrSignatureInvalid, result.Status)
	}

	if len(v.TrustedPublishers) == 0 {
		return ErrNoTrustedPublisher
	}

	matched := false
	for _, trusted := range v.TrustedPublishers {
		if result.SignerCertificate.Subject == trusted ||
			result.SignerCertificate.Thumbprint == trusted {
			matched = true
			break
		}
	}
	if !matched {
		return fmt.Errorf("%w: %s", ErrPublisherMismatch, result.SignerCertificate.Subject)
	}

	return nil
}

func (v *WindowsSignatureVerifier) getSignature(ctx context.Context, executablePath string) (*powershellSignatureResult, error) {
	script := fmt.Sprintf(
		`Get-AuthenticodeSignature -LiteralPath '%s' | ConvertTo-Json -Compress`,
		strings.ReplaceAll(executablePath, "'", "''"),
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execute signature check: %w", err)
	}

	var result powershellSignatureResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse signature result: %w", err)
	}

	return &result, nil
}

var (
	ErrSignatureMissing    = fmt.Errorf("file is not signed")
	ErrSignatureInvalid    = fmt.Errorf("signature is invalid or tampered")
	ErrSignatureNotTrusted = fmt.Errorf("signature is not trusted by Windows")
	ErrNoTrustedPublisher  = fmt.Errorf("no trusted publisher configured")
	ErrPublisherMismatch   = fmt.Errorf("publisher does not match trusted list")
)
