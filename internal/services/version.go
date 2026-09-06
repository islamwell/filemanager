package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.7"
	UpdatedAt = "2026-09-06 22:55"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
