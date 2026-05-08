package shared_test

import (
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoURL_Valid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"https simple", "https://github.com/org/repo"},
		{"https con .git", "https://github.com/org/repo.git"},
		{"https subgrupo", "https://gitlab.com/org/subgroup/repo"},
		{"ssh github", "git@github.com:org/repo.git"},
		{"ssh gitlab", "git@gitlab.com:org/repo"},
		{"ssh custom host", "git@bitbucket.org:team/project.git"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := shared.NewRepositoryURL(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.raw, url.String())
		})
	}
}

func TestParseRepoURL_Invalid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"vacío", ""},
		{"solo texto", "not-a-url"},
		{"http sin doble slash", "http:/github.com/org/repo"},
		{"ftp", "ftp://github.com/org/repo"},
		{"https sin path", "https://github.com"},
		{"https sin org", "https://github.com/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := shared.NewRepositoryURL(tc.raw)
			require.Error(t, err)
		})
	}
}

func TestRepoURL_Name(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		expectedName string
	}{
		{
			name:         "https sin .git",
			raw:          "https://github.com/org/repo",
			expectedName: "github.com/org/repo",
		},
		{
			name:         "https con .git",
			raw:          "https://github.com/org/repo.git",
			expectedName: "github.com/org/repo",
		},
		{
			name:         "ssh con .git",
			raw:          "git@github.com:org/repo.git",
			expectedName: "org/repo",
		},
		{
			name:         "ssh sin .git",
			raw:          "git@gitlab.com:org/repo",
			expectedName: "org/repo",
		},
		{
			name:         "ssh custom host",
			raw:          "git@bitbucket.org:team/project.git",
			expectedName: "team/project",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := shared.NewRepositoryURL(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedName, url.Name())
		})
	}
}

func TestRepoURL_String(t *testing.T) {
	raw := "https://github.com/org/repo.git"
	url, err := shared.NewRepositoryURL(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, url.String())
}
