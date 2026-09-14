package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jairoprogramador/vex-engine/old-internal/interfaces/cli"
)

// cancelGracePeriod es el plazo que se le da al trabajo en curso, tras la
// señal, para desenredarse y terminar de escribir lo que tenga a medias
// (restaurar las plantillas del workdir, revertir el estado del step). Pasado
// ese plazo el proceso sale igual: la cancelación es best-effort y así se
// documenta (spec 07 §5.4).
const cancelGracePeriod = 10 * time.Second

// runWithSignalHandling ejecuta `run` bajo un contexto que se cancela con
// SIGINT o SIGTERM.
//
// Cancelar el contexto es lo que convierte un Ctrl-C —la forma normal de
// abortar en local— en un estado terminal con nombre: la cadena aborta, el use
// case ve que el contexto fue cancelado y marca la ejecución como `canceled`
// en vez de como un fallo. Lo que esta spec añade no es sobrevivir a la señal,
// es que la interrupción VOLUNTARIA deje de disfrazarse de accidente.
//
// Un SIGKILL o un OOM siguen sin dejar rastro, y eso es correcto: para esos
// casos el modelo de eventos pliega a `interrupted`, que es la respuesta
// honesta.
func runWithSignalHandling(run func(ctx context.Context) int) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
		}

		fmt.Fprintf(os.Stderr, "vexd run: señal recibida, cancelando (hasta %s para terminar)\n", cancelGracePeriod)

		select {
		case <-done:
		case <-time.After(cancelGracePeriod):
			fmt.Fprintln(os.Stderr, "vexd run: la ejecución no terminó dentro del plazo; saliendo")
			os.Exit(cli.ExitCancelled)
		}
	}()

	return run(ctx)
}
