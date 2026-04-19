package project

import (
	"fmt"
	"time"
)

type ProjectVersion struct {
	major  int
	minor  int
	patch  int
	raw    string
	isDate bool
}

func NewProjectVersion(major, minor, patch int) ProjectVersion {
	return ProjectVersion{
		major: major, minor: minor, patch: patch,
		raw: fmt.Sprintf("v%d.%d.%d", major, minor, patch),
	}
}

// NewDateVersion produce una versión basada en fecha con formato v{yy}{mm}{dd}{HH}{MM}.
// Ejemplo: 2025-06-11 16:25 UTC → v2506111625
func NewDateVersion(time time.Time) ProjectVersion {
	return ProjectVersion{
		raw:    time.UTC().Format("v0601021504"),
		isDate: true,
	}
}

func (v ProjectVersion) Major() int     { return v.major }
func (v ProjectVersion) Minor() int     { return v.minor }
func (v ProjectVersion) Patch() int     { return v.patch }
func (v ProjectVersion) String() string { return v.raw }
func (v ProjectVersion) IsDate() bool   { return v.isDate }
