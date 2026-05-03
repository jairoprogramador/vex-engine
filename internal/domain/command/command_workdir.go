package command

import (
	"path/filepath"
	"strings"
)

type WorkdirScope int

const (
	ScopeEnvironment WorkdirScope = iota
	ScopeShared      WorkdirScope = iota
)

const SharedScopeName = "shared"

type CommandWorkdir struct {
	relativePath string
}

func NewCommandWorkdir(relativePath string) CommandWorkdir {
	return CommandWorkdir{relativePath: relativePath}
}

func (cw CommandWorkdir) String() string {
	return cw.relativePath
}

func (cw CommandWorkdir) scope() WorkdirScope {
	if cw.relativePath == "" {
		return ScopeEnvironment
	}
	normalized := filepath.ToSlash(cw.relativePath)
	firstSegment := strings.SplitN(normalized, "/", 2)[0]
	if firstSegment == SharedScopeName {
		return ScopeShared
	}
	return ScopeEnvironment
}

func (cw CommandWorkdir) IsShared() bool {
	scopePath := cw.scope()
	return scopePath == ScopeShared
}
