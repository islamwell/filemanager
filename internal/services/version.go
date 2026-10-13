package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.11"
	UpdatedAt = "2026-10-13 17:42"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
