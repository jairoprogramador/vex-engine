package vos

import (
	"fmt"

	"github.com/google/uuid"
)

// ExecutionID es el identificador único de una ejecución de pipeline.
type ExecutionID struct {
	value uuid.UUID
}

// NewExecutionID genera un nuevo ExecutionID aleatorio.
func NewExecutionID() ExecutionID {
	return ExecutionID{value: uuid.New()}
}

// ExecutionIDFromString parsea un ExecutionID desde su representación en string.
func ExecutionIDFromString(s string) (ExecutionID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return ExecutionID{}, fmt.Errorf("execution id inválido %q: %w", s, err)
	}
	return ExecutionID{value: parsed}, nil
}

// String devuelve la representación en string del ExecutionID.
func (id ExecutionID) String() string {
	return id.value.String()
}
