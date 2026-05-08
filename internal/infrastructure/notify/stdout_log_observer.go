package notify

import (
	"fmt"
	"os"
	"sync"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

// StdoutLogObserver escribe cada línea de log en stdout (útil para depurar vexd).
type StdoutLogObserver struct {
	mu sync.Mutex
}

// NewStdoutLogObserver construye un emisor que imprime en la consola del proceso.
func NewStdoutLogObserver() domNotify.LogObserver {
	return &StdoutLogObserver{}
}

func (e *StdoutLogObserver) Notify(executionID string, line string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	executionID = executionID[:4] + "..." + executionID[len(executionID)-4:]
	fmt.Fprintf(os.Stdout, "[request %s] %s\n", executionID, line)
}

var _ domNotify.LogObserver = (*StdoutLogObserver)(nil)
