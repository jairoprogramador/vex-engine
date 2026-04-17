package local

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	"github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// Executor lanza cada comando como proceso hijo en el host donde corre vexd.
// Se construye uno por ejecución para vincular executionID y emitter sin estado compartido.
type Executor struct {
	executionID vos.ExecutionID
	emitter     ports.LogEmitter
}

// New construye un Executor local asociado a una ejecución concreta.
func New(executionID vos.ExecutionID, emitter ports.LogEmitter) *Executor {
	return &Executor{
		executionID: executionID,
		emitter:     emitter,
	}
}

// Run lanza cmd.Line en el shell del sistema, emite cada línea de stdout/stderr
// al LogEmitter y acumula stdout en RunResult.Output.
// Un exit code no-cero no se propaga como error: se refleja en RunResult.ExitCode.
func (e *Executor) Run(ctx context.Context, cmd ports.ExecCommand, env map[string]string) (ports.RunResult, error) {
	c := e.buildCommand(ctx, cmd, env)

	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return ports.RunResult{}, fmt.Errorf("local executor: stdout pipe: %w", err)
	}

	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return ports.RunResult{}, fmt.Errorf("local executor: stderr pipe: %w", err)
	}

	if err := c.Start(); err != nil {
		return ports.RunResult{}, fmt.Errorf("local executor: start command: %w", err)
	}

	var (
		outputBuilder strings.Builder
		mu            sync.Mutex
		wg            sync.WaitGroup
	)

	wg.Add(2)

	// stdout: emite al LogEmitter y acumula en outputBuilder.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			e.emitter.Emit(e.executionID, line)
			mu.Lock()
			outputBuilder.WriteString(line)
			outputBuilder.WriteByte('\n')
			mu.Unlock()
		}
	}()

	// stderr: solo emite al LogEmitter, no va a RunResult.Output.
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			e.emitter.Emit(e.executionID, scanner.Text())
		}
	}()

	// Esperar a que ambas goroutines drenen los pipes antes de llamar Wait.
	// Si Wait se llama antes, los pipes se cierran y los scanners pierden datos.
	wg.Wait()

	if err := c.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return ports.RunResult{
				ExitCode: exitErr.ExitCode(),
				Output:   outputBuilder.String(),
			}, nil
		}
		return ports.RunResult{}, fmt.Errorf("local executor: wait command: %w", err)
	}

	return ports.RunResult{
		ExitCode: 0,
		Output:   outputBuilder.String(),
	}, nil
}

// buildCommand construye el *exec.Cmd con shell, directorio y variables de entorno resueltas.
func (e *Executor) buildCommand(ctx context.Context, cmd ports.ExecCommand, env map[string]string) *exec.Cmd {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd.Line)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", cmd.Line)
	}

	c.Dir = cmd.Workdir

	// Hereda el entorno del proceso padre, luego aplica cmd.Env, luego env (mayor precedencia).
	merged := os.Environ()
	for k, v := range cmd.Env {
		merged = append(merged, k+"="+v)
	}
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}
	c.Env = merged

	return c
}

var _ ports.Executor = (*Executor)(nil)
