package project_test

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/project"
)

func TestNewProjectID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid id", "abc-123", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := project.NewProjectID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id.String() != tt.input {
				t.Errorf("got %q, want %q", id.String(), tt.input)
			}
		})
	}
}

func TestNewProjectName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "my-project", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.input {
				t.Errorf("got %q, want %q", v.String(), tt.input)
			}
		})
	}
}

func TestNewProjectTeam(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid team", "platform", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectTeam(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.input {
				t.Errorf("got %q, want %q", v.String(), tt.input)
			}
		})
	}
}

func TestNewProjectOrg(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid org", "acme-corp", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectOrg(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.input {
				t.Errorf("got %q, want %q", v.String(), tt.input)
			}
		})
	}
}

func TestNewProjectURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"https url", "https://github.com/org/repo", false},
		{"ssh url", "git@github.com:org/repo", false},
		{"ftp url", "ftp://example.com/repo", true},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.input {
				t.Errorf("got %q, want %q", v.String(), tt.input)
			}
		})
	}
}

func TestProjectURL_Name(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
	}{
		{"https with .git", "https://github.com/org/repo.git", "github.com/org/repo"},
		{"https without .git", "https://github.com/org/repo", "github.com/org/repo"},
		{"ssh", "git@github.com:org/repo", "org/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectURL(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Name() != tt.wantName {
				t.Errorf("got %q, want %q", v.Name(), tt.wantName)
			}
		})
	}
}

func TestNewProjectRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid ref", "main", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := project.NewProjectRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.input {
				t.Errorf("got %q, want %q", v.String(), tt.input)
			}
		})
	}
}

func TestNewProject(t *testing.T) {
	t.Run("all valid VOs — success, getters work", func(t *testing.T) {
		id, _ := project.NewProjectID("proj-001")
		name, _ := project.NewProjectName("my-app")
		team, _ := project.NewProjectTeam("platform")
		org, _ := project.NewProjectOrg("acme")
		url, _ := project.NewProjectURL("https://github.com/acme/my-app")
		ref, _ := project.NewProjectRef("main")

		p, err := project.NewProject(id, name, team, org, url, ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ID().String() != "proj-001" {
			t.Errorf("ID got %q", p.ID().String())
		}
		if p.Name().String() != "my-app" {
			t.Errorf("Name got %q", p.Name().String())
		}
		if p.Team().String() != "platform" {
			t.Errorf("Team got %q", p.Team().String())
		}
		if p.Org().String() != "acme" {
			t.Errorf("Org got %q", p.Org().String())
		}
		if p.URL().String() != "https://github.com/acme/my-app" {
			t.Errorf("URL got %q", p.URL().String())
		}
		if p.Ref().String() != "main" {
			t.Errorf("Ref got %q", p.Ref().String())
		}
	})
}

func TestNewVersion(t *testing.T) {
	v := project.NewProjectVersion(1, 2, 3)
	if v.String() != "v1.2.3" {
		t.Errorf("got %q, want v1.2.3", v.String())
	}
	if v.Major() != 1 || v.Minor() != 2 || v.Patch() != 3 {
		t.Error("getters returned wrong values")
	}
	if v.IsDate() {
		t.Error("IsDate should be false for semver version")
	}
}

func TestNewDateVersion(t *testing.T) {
	// Fecha conocida: 2025-06-11 16:25 UTC → v2506111625
	known := time.Date(2025, 6, 11, 16, 25, 0, 0, time.UTC)
	v := project.NewDateVersion(known)
	if v.String() != "v2506111625" {
		t.Errorf("got %q, want v2506111625", v.String())
	}
	if !v.IsDate() {
		t.Error("IsDate should be true for date version")
	}
	sv := project.NewProjectVersion(0, 0, 1)
	if sv.IsDate() {
		t.Error("IsDate should be false for semver version")
	}
}
