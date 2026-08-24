package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.6"
	UpdatedAt = "2026-08-25 00:28"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
