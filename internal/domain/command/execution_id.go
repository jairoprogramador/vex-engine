package command

import (
	"fmt"

	"github.com/google/uuid"
)

type ExecutionID struct {
	value uuid.UUID
}

func NewExecutionID() ExecutionID {
	return ExecutionID{value: uuid.New()}
}

func ExecutionIDFromString(uuidStr string) (ExecutionID, error) {
	parsed, err := uuid.Parse(uuidStr)
	if err != nil {
		return ExecutionID{}, fmt.Errorf("execution id inválido %q: %w", uuidStr, err)
	}
	return ExecutionID{value: parsed}, nil
}

func (id ExecutionID) String() string {
	return id.value.String()
}
