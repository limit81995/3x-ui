package panel

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type panelReleaseSource struct {
	repository string
	scriptRef  string
}

var (
	releaseRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	releaseRefPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
)

func configuredPanelReleaseSource() (panelReleaseSource, error) {
	source := panelReleaseSource{os.Getenv("XUI_RELEASE_REPO"), os.Getenv("XUI_RELEASE_REF")}
	if source.repository == "" {
		source.repository = "limit81995/3x-ui"
	}
	if source.scriptRef == "" {
		source.scriptRef = "main"
	}
	if !releaseRepositoryPattern.MatchString(source.repository) {
		return source, fmt.Errorf("XUI_RELEASE_REPO must be owner/repository")
	}
	if !releaseRefPattern.MatchString(source.scriptRef) || strings.Contains(source.scriptRef, "..") {
		return source, fmt.Errorf("XUI_RELEASE_REF must be a branch or tag name")
	}
	return source, nil
}

func (s panelReleaseSource) updaterURL() string {
	return "https://raw.githubusercontent.com/" + s.repository + "/" + s.scriptRef + "/update.sh"
}

func (s panelReleaseSource) releaseURL(tag string) string {
	base := "https://api.github.com/repos/" + s.repository + "/releases/"
	if tag == "" {
		return base + "latest"
	}
	return base + "tags/" + tag
}

// systemd-run does not inherit the panel's environment; pin both download sources.
func (s panelReleaseSource) env() []string {
	return []string{"XUI_RELEASE_REPO=" + s.repository, "XUI_RELEASE_REF=" + s.scriptRef}
}
