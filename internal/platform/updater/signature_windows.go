//go:build windows

package updater

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type powershellSignatureStatus string

const (
	signatureStatusValid                  powershellSignatureStatus = "Valid"
	signatureStatusUnknownError           powershellSignatureStatus = "UnknownError"
	signatureStatusNotSigned              powershellSignatureStatus = "NotSigned"
	signatureStatusHashMismatch           powershellSignatureStatus = "HashMismatch"
	signatureStatusNotTrusted             powershellSignatureStatus = "NotTrusted"
	signatureStatusNotSupportedFileFormat powershellSignatureStatus = "NotSupportedFileFormat"
	signatureStatusIncompatible           powershellSignatureStatus = "Incompatible"
)

var numericSignatureStatuses = map[int]powershellSignatureStatus{
	0: signatureStatusValid,
	1: signatureStatusUnknownError,
	2: signatureStatusNotSigned,
	3: signatureStatusHashMismatch,
	4: signatureStatusNotTrusted,
	5: signatureStatusNotSupportedFileFormat,
	6: signatureStatusIncompatible,
}

// UnmarshalJSON accepts both PowerShell 5.1's numeric enum representation and
// PowerShell 7's/string-projected representation. Unknown values are retained
// and rejected later by Verify instead of causing a misleading JSON error.
func (status *powershellSignatureStatus) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*status = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		text = strings.TrimSpace(text)
		if number, ok := parseSignatureStatusNumber(text); ok {
			*status = signatureStatusFromNumber(number)
			return nil
		}
		*status = powershellSignatureStatus(text)
		return nil
	}

	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("decode signature status: %w", err)
	}
	*status = signatureStatusFromNumber(number)
	return nil
}

func parseSignatureStatusNumber(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	number := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
		number = number*10 + int(char-'0')
	}
	return number, true
}

func signatureStatusFromNumber(number int) powershellSignatureStatus {
	if status, ok := numericSignatureStatuses[number]; ok {
		return status
	}
	return powershellSignatureStatus(fmt.Sprintf("Unknown(%d)", number))
}

type powershellSignatureResult struct {
	Status            powershellSignatureStatus `json:"Status"`
	StatusMessage     string                    `json:"StatusMessage"`
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
	cleaned := make([]string, 0, len(trustedPublishers))
	for _, publisher := range trustedPublishers {
		publisher = strings.TrimSpace(publisher)
		if publisher != "" {
			cleaned = append(cleaned, publisher)
		}
	}
	return &WindowsSignatureVerifier{TrustedPublishers: cleaned}
}

func (v *WindowsSignatureVerifier) Verify(ctx context.Context, executablePath string) error {
	if len(v.TrustedPublishers) == 0 {
		return ErrNoTrustedPublisher
	}

	result, err := v.getSignature(ctx, executablePath)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	switch result.Status {
	case signatureStatusValid:
		// Signature is cryptographically valid. Publisher pinning is checked below.
	case signatureStatusNotSigned:
		return ErrSignatureMissing
	case signatureStatusHashMismatch:
		return ErrSignatureInvalid
	case signatureStatusNotTrusted:
		return ErrSignatureNotTrusted
	case signatureStatusNotSupportedFileFormat:
		return ErrSignatureUnsupported
	case signatureStatusIncompatible:
		return ErrSignatureIncompatible
	default:
		return signatureStatusError(result)
	}

	for _, trusted := range v.TrustedPublishers {
		if publisherMatches(result.SignerCertificate.Subject, result.SignerCertificate.Thumbprint, trusted) {
			return nil
		}
	}

	publisher := strings.TrimSpace(result.SignerCertificate.Subject)
	if publisher == "" {
		publisher = "missing signer certificate"
	}
	return fmt.Errorf("%w: %s", ErrPublisherMismatch, publisher)
}

func signatureStatusError(result *powershellSignatureResult) error {
	status := strings.TrimSpace(string(result.Status))
	if status == "" {
		status = "empty status"
	}
	message := strings.TrimSpace(result.StatusMessage)
	if message == "" {
		return fmt.Errorf("%w: %s", ErrSignatureInvalid, status)
	}
	return fmt.Errorf("%w: %s: %s", ErrSignatureInvalid, status, message)
}

func publisherMatches(subject, thumbprint, trusted string) bool {
	trusted = strings.TrimSpace(trusted)
	if trusted == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(subject), trusted) {
		return true
	}

	actualThumbprint := normalizeThumbprint(thumbprint)
	trustedThumbprint := normalizeThumbprint(trusted)
	return actualThumbprint != "" && trustedThumbprint != "" && actualThumbprint == trustedThumbprint
}

func normalizeThumbprint(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, char := range value {
		switch char {
		case ' ', '\t', '\r', '\n', ':', '-':
			continue
		}
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			builder.WriteRune(char)
			continue
		}
		return ""
	}
	return strings.ToUpper(builder.String())
}

func (v *WindowsSignatureVerifier) getSignature(ctx context.Context, executablePath string) (*powershellSignatureResult, error) {
	escapedPath := strings.ReplaceAll(executablePath, "'", "''")
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$signature = Get-AuthenticodeSignature -LiteralPath '%s'
$certificate = $null
if ($null -ne $signature.SignerCertificate) {
    $certificate = [PSCustomObject]@{
        Subject = [string]$signature.SignerCertificate.Subject
        Thumbprint = [string]$signature.SignerCertificate.Thumbprint
    }
}
[PSCustomObject]@{
    Status = [string]$signature.Status
    StatusMessage = [string]$signature.StatusMessage
    SignerCertificate = $certificate
    Path = [string]$signature.Path
} | ConvertTo-Json -Compress -Depth 3
`, escapedPath)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		script,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, fmt.Errorf("execute signature check: %w", err)
		}
		return nil, fmt.Errorf("execute signature check: %w: %s", err, message)
	}

	output = bytes.TrimSpace(bytes.TrimPrefix(output, []byte{0xEF, 0xBB, 0xBF}))
	var result powershellSignatureResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse signature result: %w", err)
	}
	return &result, nil
}

var (
	ErrSignatureMissing      = fmt.Errorf("file is not signed")
	ErrSignatureInvalid      = fmt.Errorf("signature is invalid or tampered")
	ErrSignatureNotTrusted   = fmt.Errorf("signature is not trusted by Windows")
	ErrSignatureUnsupported  = fmt.Errorf("file format does not support Authenticode verification")
	ErrSignatureIncompatible = fmt.Errorf("signature is incompatible with this Windows system")
	ErrNoTrustedPublisher    = fmt.Errorf("no trusted Windows publisher is configured")
	ErrPublisherMismatch     = fmt.Errorf("publisher does not match trusted list")
)
