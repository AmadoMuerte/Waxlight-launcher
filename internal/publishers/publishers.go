package publishers

import "fmt"

var (
	TrustedWindowsPublisher      = ""
	TrustedWindowsCertThumbprint = ""
)

func GetTrustedWindowsPublishers() []string {
	publishers := []string{}
	if TrustedWindowsPublisher != "" {
		publishers = append(publishers, TrustedWindowsPublisher)
	}
	if TrustedWindowsCertThumbprint != "" {
		publishers = append(publishers, TrustedWindowsCertThumbprint)
	}
	return publishers
}

func ValidateTrustedPublisher() error {
	publishers := GetTrustedWindowsPublishers()
	if len(publishers) == 0 {
		return fmt.Errorf("no trusted Windows publisher configured; set TrustedWindowsPublisher or TrustedWindowsCertThumbprint at build time")
	}
	return nil
}
