package command

type ExecutionRuntime struct {
	Image string
	Tag   string
}

func NewExecutionRuntime(image, tag string) ExecutionRuntime {
	if tag == "" {
		tag = "latest"
	}
	return ExecutionRuntime{
		Image: image,
		Tag:   tag,
	}
}

func (r ExecutionRuntime) IsEmpty() bool {
	return r.Image == ""
}
