package command

type CommandTemplatePath struct {
	relativePath string
}

func NewCommandTemplatePath(relativePath string) CommandTemplatePath {
	return CommandTemplatePath{relativePath: relativePath}
}

func (c CommandTemplatePath) String() string {
	return c.relativePath
}
