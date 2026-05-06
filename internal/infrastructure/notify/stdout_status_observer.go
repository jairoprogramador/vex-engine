package notify

import (
	"fmt"
	"io"
	"os"
	"sync"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

// StdoutStatusObserver imprime cada transición de stage como una línea
// "→ <stage>" en el writer dado (por defecto os.Stdout). Es el observer
// usado en modo local para que el usuario vea el progreso en tiempo real.
type StdoutStatusObserver struct {
	mu sync.Mutex
	w  io.Writer
}

func NewStdoutStatusObserver() *StdoutStatusObserver {
	return &StdoutStatusObserver{w: os.Stdout}
}

func NewStdoutStatusObserverTo(w io.Writer) *StdoutStatusObserver {
	if w == nil {
		w = os.Stdout
	}
	return &StdoutStatusObserver{w: w}
}

func (o *StdoutStatusObserver) Notify(_ string, stage string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	fmt.Fprintf(o.w, "→ %s\n", stage)
}

var _ domNotify.StatusObserver = (*StdoutStatusObserver)(nil)
