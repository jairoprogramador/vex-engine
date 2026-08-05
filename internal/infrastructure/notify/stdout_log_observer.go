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

// idAbreviableMinimo es la longitud a partir de la cual abreviar un id dice
// algo: por debajo, las dos mitades se solapan —y con menos de 4 caracteres el
// recorte panicaba directamente, en el camino del log, o sea en el camino feliz
// (spec 07 §5.5).
const idAbreviableMinimo = 8

func (e *StdoutLogObserver) Notify(executionID string, line string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fmt.Fprintf(os.Stdout, "[deploy %s] %s\n", abreviar(executionID), line)
}

func abreviar(executionID string) string {
	if len(executionID) < idAbreviableMinimo {
		return executionID
	}
	return executionID[:4] + "..." + executionID[len(executionID)-4:]
}

var _ domNotify.LogObserver = (*StdoutLogObserver)(nil)
