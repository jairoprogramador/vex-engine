package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/shared"
)

const MaxCommitsForVersioning = 200

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

type VersionCalculatorHandler struct {
	PipelineBaseHandler
	repository ProjectTagRepository
	clock      shared.Clock
}

var _ PipelineHandler = (*VersionCalculatorHandler)(nil)

func NewVersionCalculatorHandler(repository ProjectTagRepository, clock shared.Clock) PipelineHandler {
	return &VersionCalculatorHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
		clock:               clock,
	}
}

// Handle calcula la versión del proyecto. Las dos ramas —la de semver y la de
// fecha— toman el instante de la MISMA fuente (spec 07 §5.1): antes una usaba
// `time.Now()` y la otra el `startedAt` de la ejecución, así que la misma
// decisión se tomaba con dos relojes según por dónde entrara.
func (h *VersionCalculatorHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("calculating version")
	now := h.clock.Now()
	projectVersion, projectHeadHash, err := h.NextVersion(ctx, request.ProjectLocalPath(), now)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if projectHeadHash == "" {
		return fmt.Errorf("hash del proyecto no puede estar vacío")
	}

	if projectVersion.String() == "" {
		return fmt.Errorf("versión del proyecto no puede estar vacía")
	}

	request.SetProjectHeadHash(projectHeadHash)
	if request.StepName() == string(command.StepDeploy) {
		request.SetProjectVersion(projectVersion.String())
	} else {
		projectVersionDate := NewDateVersion(now)
		request.SetProjectVersion(projectVersionDate.String())
	}
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

func (h *VersionCalculatorHandler) NextVersion(ctx *context.Context, projectLocalPath string, time time.Time) (ProjectVersion, string, error) {
	lastTag, err := h.repository.LastTag(ctx, projectLocalPath)
	if err != nil {
		return ProjectVersion{}, "", fmt.Errorf("obtener último tag: %w", err)
	}

	headHash, messages, err := h.repository.RecentCommits(ctx, projectLocalPath, lastTag, MaxCommitsForVersioning)
	if err != nil {
		return ProjectVersion{}, "", fmt.Errorf("obtener commits recientes: %w", err)
	}

	version := h.calculateVersion(lastTag, messages, time)
	return version, headHash, nil
}

func (h *VersionCalculatorHandler) calculateVersion(lastTag string, messages []string, time time.Time) ProjectVersion {
	if lastTag == "" {
		return defaultVersion()
	}
	level := h.detectChangeLevel(messages)
	if level == changeNone {
		return NewDateVersion(time)
	}
	return h.increment(h.parseVersion(lastTag), level)
}

func (h *VersionCalculatorHandler) detectChangeLevel(messages []string) changeLevel {
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

func (h *VersionCalculatorHandler) parseVersion(tag string) ProjectVersion {
	m := semverTagRegex.FindStringSubmatch(tag)
	if len(m) != 4 {
		return defaultVersion()
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return NewProjectVersion(major, minor, patch)
}

func (h *VersionCalculatorHandler) increment(v ProjectVersion, level changeLevel) ProjectVersion {
	switch level {
	case changeMajor:
		return NewProjectVersion(v.Major()+1, 0, 0)
	case changeMinor:
		return NewProjectVersion(v.Major(), v.Minor()+1, 0)
	default:
		return NewProjectVersion(v.Major(), v.Minor(), v.Patch()+1)
	}
}

func defaultVersion() ProjectVersion {
	return NewProjectVersion(0, 1, 0)
}
