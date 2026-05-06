package utils

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

func TestForClone_stableAndDeterministic(t *testing.T) {
	raw := "https://github.com/acme/my-app"
	repoURL, err := shared.NewRepositoryURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	base := "/var/cache/vex"
	namePath1 := LocalRepositoryPath(base, repoURL)
	namePath2 := LocalRepositoryPath(base, repoURL)
	if namePath1 != namePath2 {
		t.Fatalf("ForClone not deterministic: %q vs %q", namePath1, namePath2)
	}

	// The directory name is intentionally `<lastSegment><hash[:8]>` with no
	// separator; CLAUDE.md §"Storage en Filesystem" pins this format and
	// other repos depend on it, so the test asserts the same convention.
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:8]
	wantName := "my-app" + wantSuffix
	if filepath.Base(namePath1) != wantName {
		t.Errorf("basename got %q want %q", filepath.Base(namePath1), wantName)
	}
	if filepath.Dir(namePath1) != base {
		t.Errorf("dir got %q want %q", filepath.Dir(namePath1), base)
	}
}

func TestForClone_sshURL(t *testing.T) {
	raw := "git@github.com:org/repo.git"
	u, err := shared.NewRepositoryURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := LocalRepositoryPath("/base", u)
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:8]
	want := filepath.Join("/base", "repo"+wantSuffix)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestForClone_trimsGitSuffixOnLastSegment(t *testing.T) {
	// Name() ya normaliza muchas URLs; este doble trim cubre segmentos raros.
	u := stubURL{name: "owner/foo.git", raw: "https://example.com/x"}
	got := filepath.Base(LocalRepositoryPath("/tmp", u))
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(u.raw)))[:8]
	if got != "foo"+wantSuffix {
		t.Errorf("got %q want foo%s", got, wantSuffix)
	}
}

type stubURL struct {
	name, raw string
}

func (s stubURL) Name() string   { return s.name }
func (s stubURL) String() string { return s.raw }
