package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.19"
	UpdatedAt = "2026-11-09 22:30"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
