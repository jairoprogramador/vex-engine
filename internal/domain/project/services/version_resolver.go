package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/project"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
)

const maxCommitsForVersioning = 200

var (
	semverTagRegex          = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	conventionalCommitRegex = regexp.MustCompile(`^(feat|fix|build|chore|ci|docs|style|refactor|perf|test)(\(.*\))?(!?):`)
)

type changeLevel int

const (
	changeNone changeLevel = iota
	changePatch
	changeMinor
	changeMajor
)

type VersionResolver struct {
	fetcher ports.RepositoryFetcher
}

func NewVersionResolver(fetcher ports.RepositoryFetcher) *VersionResolver {
	return &VersionResolver{fetcher: fetcher}
}

func (vr *VersionResolver) NextVersion(ctx context.Context, url project.ProjectURL, ref project.ProjectRef) (project.ProjectVersion, string, string, error) {
	localPath, err := vr.fetcher.Fetch(ctx, url, ref)
	if err != nil {
		return project.ProjectVersion{}, "", "", fmt.Errorf("fetch del repositorio: %w", err)
	}

	lastTag, err := vr.fetcher.LastTag(ctx, localPath)
	if err != nil {
		return project.ProjectVersion{}, "", "", fmt.Errorf("obtener último tag: %w", err)
	}

	headHash, messages, err := vr.fetcher.RecentCommits(ctx, localPath, lastTag, maxCommitsForVersioning)
	if err != nil {
		return project.ProjectVersion{}, "", "", fmt.Errorf("obtener commits recientes: %w", err)
	}

	version := vr.calculateVersion(lastTag, messages)
	return version, headHash, localPath, nil
}

func (vr *VersionResolver) calculateVersion(lastTag string, messages []string) project.ProjectVersion {
	if lastTag == "" {
		return project.NewProjectVersion(0, 1, 0)
	}
	level := vr.detectChangeLevel(messages)
	if level == changeNone {
		return project.NewDateVersion(time.Now())
	}
	return vr.increment(vr.parseVersion(lastTag), level)
}

func (vr *VersionResolver) detectChangeLevel(messages []string) changeLevel {
	highest := changeNone
	for _, msg := range messages {
		if strings.Contains(msg, "BREAKING CHANGE") {
			return changeMajor
		}
		m := conventionalCommitRegex.FindStringSubmatch(msg)
		if len(m) < 4 {
			continue
		}
		if m[3] == "!" {
			return changeMajor
		}
		switch m[1] {
		case "feat":
			if highest < changeMinor {
				highest = changeMinor
			}
		case "fix":
			if highest < changePatch {
				highest = changePatch
			}
		}
	}
	return highest
}

func (vr *VersionResolver) parseVersion(tag string) project.ProjectVersion {
	m := semverTagRegex.FindStringSubmatch(tag)
	if len(m) != 4 {
		return project.NewProjectVersion(0, 0, 0)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return project.NewProjectVersion(major, minor, patch)
}

func (vr *VersionResolver) increment(v project.ProjectVersion, level changeLevel) project.ProjectVersion {
	switch level {
	case changeMajor:
		return project.NewProjectVersion(v.Major()+1, 0, 0)
	case changeMinor:
		return project.NewProjectVersion(v.Major(), v.Minor()+1, 0)
	default:
		return project.NewProjectVersion(v.Major(), v.Minor(), v.Patch()+1)
	}
}
