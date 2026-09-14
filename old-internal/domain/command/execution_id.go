package command

import (
	"github.com/google/uuid"
)

type ExecutionID struct {
	value string
}

func NewExecutionID(id string) ExecutionID {
	if id == "" {
		id = uuid.New().String()
	}
	return ExecutionID{value: id}
}

func (id ExecutionID) String() string {
	return id.value
}
