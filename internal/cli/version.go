package cli

// Build is supplied by the process entry point; local results carry no API metadata.
type Build struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}
