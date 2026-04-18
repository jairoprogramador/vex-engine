package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	httpsURLRegex = regexp.MustCompile(`^https://[^/]+/.+`)
	sshURLRegex   = regexp.MustCompile(`^git@[^:]+:.+`)
)

type RepositoryURL struct {
	raw string
}

func NewRepositoryURL(raw string) (RepositoryURL, error) {
	if !httpsURLRegex.MatchString(raw) && !sshURLRegex.MatchString(raw) {
		return RepositoryURL{}, fmt.Errorf("url de repositorio inválida '%s': debe ser https:// o git@", raw)
	}
	return RepositoryURL{raw: raw}, nil
}

// Name extrae owner/repo como identificador único del repositorio.
func (r RepositoryURL) Name() string {
	s := r.raw
	s = strings.TrimSuffix(s, ".git")
	if httpsURLRegex.MatchString(s) {
		// https://host/owner/repo → owner/repo
		parts := strings.SplitN(s, "/", 4)
		if len(parts) >= 4 {
			return parts[2] + "/" + parts[3]
		}
		return parts[len(parts)-1]
	}
	// git@host:owner/repo → owner/repo
	if idx := strings.Index(s, ":"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func (r RepositoryURL) String() string {
	return r.raw
}
