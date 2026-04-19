package services_test

import (
	"context"
	"regexp"
	"testing"

	projDomain "github.com/jairoprogramador/vex-engine/internal/domain/project"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/services"
)

var dateVersionPattern = regexp.MustCompile(`^v\d{10}$`)

type mockFetcher struct {
	headHash   string
	messages   []string
	lastTag    string
	lastTagErr error
}

func (m *mockFetcher) Fetch(_ context.Context, _ projDomain.ProjectURL, _ projDomain.ProjectRef) (string, error) {
	return "/fake/path", nil
}
func (m *mockFetcher) LastTag(_ context.Context, _ string) (string, error) {
	return m.lastTag, m.lastTagErr
}
func (m *mockFetcher) RecentCommits(_ context.Context, _, _ string, _ int) (string, []string, error) {
	return m.headHash, m.messages, nil
}
func (m *mockFetcher) CreateTagForCommit(_ context.Context, _, _, _ string) error {
	return nil
}

func makeURL(s string) projDomain.ProjectURL {
	u, _ := projDomain.NewProjectURL(s)
	return u
}
func makeRef(s string) projDomain.ProjectRef {
	r, _ := projDomain.NewProjectRef(s)
	return r
}

var (
	testURL = makeURL("https://github.com/org/repo")
	testRef = makeRef("main")
)

func TestVersionResolver_NextVersion(t *testing.T) {
	tests := []struct {
		name            string
		mock            *mockFetcher
		wantVersionStr  string
		wantVersionDate bool
		wantErr         bool
	}{
		{
			name:           "fix commit, no prior tag → v0.0.1",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"fix: some bug"}, lastTag: ""},
			wantVersionStr: "v0.0.1",
		},
		{
			name:           "feat commit, no prior tag → v0.1.0",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"feat: add feature"}, lastTag: ""},
			wantVersionStr: "v0.1.0",
		},
		{
			name:           "feat! commit, no prior tag → v1.0.0",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"feat!: breaking api"}, lastTag: ""},
			wantVersionStr: "v1.0.0",
		},
		{
			name:           "BREAKING CHANGE in body → v1.0.0",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"feat: new\n\nBREAKING CHANGE: removes old api"}, lastTag: ""},
			wantVersionStr: "v1.0.0",
		},
		{
			name:            "only chore, no prior tag → date version",
			mock:            &mockFetcher{headHash: "abc123", messages: []string{"chore: update deps"}, lastTag: ""},
			wantVersionDate: true,
		},
		{
			name:           "fix from v1.2.3 → v1.2.4",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"fix: bug"}, lastTag: "v1.2.3"},
			wantVersionStr: "v1.2.4",
		},
		{
			name:           "feat from v1.2.3 → v1.3.0",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"feat: feature"}, lastTag: "v1.2.3"},
			wantVersionStr: "v1.3.0",
		},
		{
			name:           "major from v1.2.3 → v2.0.0",
			mock:           &mockFetcher{headHash: "abc123", messages: []string{"feat!: breaking"}, lastTag: "v1.2.3"},
			wantVersionStr: "v2.0.0",
		},
		{
			name:            "no commits after tag → date version (changeNone)",
			mock:            &mockFetcher{headHash: "abc123", messages: nil, lastTag: "v1.2.3"},
			wantVersionDate: true,
		},
		{
			name:            "no semver bump, tag no semver → date version",
			mock:            &mockFetcher{headHash: "abc123", messages: []string{"chore: tidy"}, lastTag: "release-candidate"},
			wantVersionDate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := services.NewVersionResolver(tt.mock)
			ver, commitHash, localPath, err := resolver.NextVersion(context.Background(), testURL, testRef)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if commitHash != "abc123" {
				t.Errorf("commitHash got %q, want abc123", commitHash)
			}
			if localPath != "/fake/path" {
				t.Errorf("localPath got %q, want /fake/path", localPath)
			}
			if tt.wantVersionDate {
				if !ver.IsDate() || !dateVersionPattern.MatchString(ver.String()) {
					t.Errorf("expected date version matching v{10 digits}, got %q", ver.String())
				}
				return
			}
			if ver.String() != tt.wantVersionStr {
				t.Errorf("got %q, want %q", ver.String(), tt.wantVersionStr)
			}
		})
	}
}
