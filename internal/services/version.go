package services

// AppVersion constants track the current semantic version and update timestamp.
const (
	Version   = "v1.0.14"
	UpdatedAt = "2026-10-14 00:48"
)

// GetFooterVersion returns the formatted version string for the HTML footer.
func GetFooterVersion() string {
	return Version + " (updated " + UpdatedAt + ")"
}
