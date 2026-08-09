package publishers

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
