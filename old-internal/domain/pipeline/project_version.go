package pipeline

import (
	"fmt"
	"time"
)

type ProjectVersion struct {
	major int
	minor int
	patch int
	raw   string
}

func NewProjectVersion(major, minor, patch int) ProjectVersion {
	return ProjectVersion{
		major: major, minor: minor, patch: patch,
		raw: fmt.Sprintf("v%d.%d.%d", major, minor, patch),
	}
}

func NewDateVersion(time time.Time) ProjectVersion {
	return ProjectVersion{
		raw: time.UTC().Format("v0601021504"),
	}
}

func (v ProjectVersion) Major() int     { return v.major }
func (v ProjectVersion) Minor() int     { return v.minor }
func (v ProjectVersion) Patch() int     { return v.patch }
func (v ProjectVersion) String() string { return v.raw }
