package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.0"
	UpdatedAt = "2026-08-24 01:45"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
