package repopath

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

func TestForClone_stableAndDeterministic(t *testing.T) {
	raw := "https://github.com/acme/my-app"
	u, err := shared.ParseRepoURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	base := "/var/cache/vex"
	a := ForClone(base, u)
	b := ForClone(base, u)
	if a != b {
		t.Fatalf("ForClone not deterministic: %q vs %q", a, b)
	}
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:8]
	wantName := "my-app-" + wantSuffix
	if filepath.Base(a) != wantName {
		t.Errorf("basename got %q want %q", filepath.Base(a), wantName)
	}
	if filepath.Dir(a) != base {
		t.Errorf("dir got %q want %q", filepath.Dir(a), base)
	}
}

func TestForClone_sshURL(t *testing.T) {
	raw := "git@github.com:org/repo.git"
	u, err := shared.ParseRepoURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := ForClone("/base", u)
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:8]
	want := filepath.Join("/base", "repo-"+wantSuffix)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestForClone_trimsGitSuffixOnLastSegment(t *testing.T) {
	// Name() ya normaliza muchas URLs; este doble trim cubre segmentos raros.
	u := stubURL{name: "owner/foo.git", raw: "https://example.com/x"}
	got := filepath.Base(ForClone("/tmp", u))
	wantSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(u.raw)))[:8]
	if got != "foo-"+wantSuffix {
		t.Errorf("got %q want foo-%s", got, wantSuffix)
	}
}

type stubURL struct {
	name, raw string
}

func (s stubURL) Name() string   { return s.name }
func (s stubURL) String() string { return s.raw }
