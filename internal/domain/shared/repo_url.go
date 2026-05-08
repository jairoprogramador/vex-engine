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

type RepositoryUrl struct {
	raw string
}

func NewRepositoryURL(raw string) (RepositoryUrl, error) {
	if !httpsURLRegex.MatchString(raw) && !sshURLRegex.MatchString(raw) {
		return RepositoryUrl{}, fmt.Errorf("url de repositorio inválida '%s': debe ser https:// o git@", raw)
	}
	return RepositoryUrl{raw: raw}, nil
}

func (r RepositoryUrl) Name() string {
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

func (r RepositoryUrl) String() string {
	return r.raw
}
