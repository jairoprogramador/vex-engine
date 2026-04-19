package shared

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	httpsURLRegex = regexp.MustCompile(`^https://[^/]+/.+`)
	sshURLRegex   = regexp.MustCompile(`^git@[^:]+:.+`)
)

// RepoURL representa una URL de clonación de repositorio Git (https o ssh).
type RepoURL struct {
	raw string
}

// ParseRepoURL valida y construye un RepoURL a partir del texto crudo.
func ParseRepoURL(raw string) (RepoURL, error) {
	if !httpsURLRegex.MatchString(raw) && !sshURLRegex.MatchString(raw) {
		return RepoURL{}, fmt.Errorf("url de repositorio inválida '%s': debe ser https:// o git@", raw)
	}
	return RepoURL{raw: raw}, nil
}

// Name extrae owner/repo (https) o la parte tras el host (ssh) como identificador del repositorio.
func (r RepoURL) Name() string {
	s := r.raw
	s = strings.TrimSuffix(s, ".git")
	if httpsURLRegex.MatchString(s) {
		parts := strings.SplitN(s, "/", 4)
		if len(parts) >= 4 {
			return parts[2] + "/" + parts[3]
		}
		return parts[len(parts)-1]
	}
	if idx := strings.Index(s, ":"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func (r RepoURL) String() string {
	return r.raw
}
