package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool // true → a and b should normalise to the same value
	}{
		{
			name: "identical URLs",
			a:    "https://github.com/org/repo",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "trailing .git on one side",
			a:    "https://github.com/org/repo.git",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "SCP-style SSH vs HTTPS",
			a:    "git@github.com:org/repo",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "SCP-style SSH with .git vs HTTPS",
			a:    "git@github.com:org/repo.git",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "uppercase host",
			a:    "https://GitHub.com/Org/Repo",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "trailing slash",
			a:    "https://github.com/org/repo/",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "git:// scheme",
			a:    "git://github.com/org/repo.git",
			b:    "https://github.com/org/repo",
			want: true,
		},
		{
			name: "different repositories",
			a:    "https://github.com/org/repo-a",
			b:    "https://github.com/org/repo-b",
			want: false,
		},
		{
			name: "different organisations",
			a:    "https://github.com/org-a/repo",
			b:    "https://github.com/org-b/repo",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitURL(tt.a) == normalizeGitURL(tt.b)
			assert.Equal(t, tt.want, got,
				"normalizeGitURL(%q) == normalizeGitURL(%q): got %v, want %v\n  a→%q\n  b→%q",
				tt.a, tt.b, got, tt.want,
				normalizeGitURL(tt.a), normalizeGitURL(tt.b),
			)
		})
	}
}

func TestResolveReference(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want plumbing.ReferenceName
	}{
		{
			name: "full refs/ path is kept verbatim",
			ref:  "refs/heads/main",
			want: plumbing.ReferenceName("refs/heads/main"),
		},
		{
			name: "full 40-char SHA is kept verbatim",
			ref:  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			want: plumbing.ReferenceName("a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"),
		},
		{
			name: "short 7-char SHA is treated as a branch name (only full 40-char SHAs are recognised)",
			ref:  "a1b2c3d",
			want: plumbing.NewBranchReferenceName("a1b2c3d"),
		},
		{
			name: "plain branch name becomes branch reference",
			ref:  "main",
			want: plumbing.NewBranchReferenceName("main"),
		},
		{
			name: "tag-like name without refs/ prefix becomes branch reference (retry handles tags)",
			ref:  "v1.0.0",
			want: plumbing.NewBranchReferenceName("v1.0.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveReference(tt.ref)
			assert.Equal(t, tt.want, got)
		})
	}
}
