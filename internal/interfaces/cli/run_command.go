package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
)

// Exit codes (alineados con la convención unix de los procesos one-shot):
//
//	0   → succeeded
//	1   → execution failed (la pipeline falló)
//	2   → input error (malformed JSON, schema_version no soportado, etc.)
//	130 → cancelled (SIGINT/SIGTERM); 128+SIGINT, la convención del shell
//
// El 130 existe para que una interrupción voluntaria no se lea como un fallo
// de la pipeline (spec 07 §5.4).
const (
	ExitSucceeded  = 0
	ExitFailed     = 1
	ExitInputError = 2
	ExitCancelled  = 130
)

// supportedSchemaVersion es el contrato de RequestInput que este binario entiende.
// El campo es obligatorio: cualquier valor distinto se rechaza como input error.
const supportedSchemaVersion = 1

// RunArgs son los flags de `vexd run` mapeados desde Cobra.
type RunArgs struct {
	InputFile             string
	InputEnv              string
	LogEndpoint           string
	StatusEndpoint        string
	StepCodeEndpoint      string // endpoint de la edge fn status-code (o equivalente)
	StepInstEndpoint      string // endpoint de la edge fn status-instructions
	StepTimeEndpoint      string // endpoint de la edge fn status-time
	StepVarsEndpoint      string // endpoint de la edge fn status-vars
	StepDeleteEndpoint    string // endpoint de la edge fn status-delete-step
	StepStoreVarsEndpoint string // endpoint de la edge fn store-vars (almacén de variables)
	LogToken              string
	ExecutionID           string
	Quiet                 bool
	Mode                  string
}

// RunCommand orquesta la ejecución one-shot del engine. Es la única superficie
// CLI que invoca al use case CreateExecution y reemplaza al antiguo HTTP server.
type RunCommand struct {
	createExec *usecase.CreateExecutionUseCase
}

func NewRunCommand(createExec *usecase.CreateExecutionUseCase) *RunCommand {
	return &RunCommand{createExec: createExec}
}

// Execute lee el RequestInput del primer source disponible (file > env > stdin),
// valida el schema, ejecuta la pipeline reportando stages, y reporta el status
// terminal vía SupabaseStatusReporter (si hay endpoint).
//
// `ctx` es el contexto del proceso: cancelarlo (lo hace el manejador de señales
// de cmd/vexd) aborta la ejecución y la deja registrada como `canceled` en vez
// de como un fallo cualquiera.
//
// Retorna el exit code que el proceso debe emitir.
func (c *RunCommand) Execute(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, args RunArgs) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	rawInput, err := readInput(stdin, args)
	if err != nil {
		fmt.Fprintf(stderr, "vexd run: read input: %v\n", err)
		return ExitInputError
	}

	var requestInput dto.RequestInput
	if err := json.Unmarshal(rawInput, &requestInput); err != nil {
		fmt.Fprintf(stderr, "vexd run: parse input: %v\n", err)
		return ExitInputError
	}

	if requestInput.SchemaVersion != supportedSchemaVersion {
		fmt.Fprintf(stderr, "vexd run: unsupported schema_version: %d (this binary supports v%d)\n",
			requestInput.SchemaVersion, supportedSchemaVersion)
		return ExitInputError
	}

	logObservers := make([]domNotify.LogObserver, 0, 2)
	statusObservers := make([]domNotify.StatusObserver, 0, 2)

	if !args.Quiet {
		logObservers = append(logObservers, notify.NewStdoutLogObserver())
		statusObservers = append(statusObservers, notify.NewStdoutStatusObserverTo(stdout))
	}

	var supabaseLogs *notify.SupabaseLogObserver
	if args.LogEndpoint != "" {
		supabaseLogs = notify.NewSupabaseLogObserver(args.LogEndpoint, args.LogToken, args.ExecutionID)
		logObservers = append(logObservers, supabaseLogs)
	}

	var statusReporter *notify.SupabaseStatusReporter
	if args.StatusEndpoint != "" {
		statusReporter = notify.NewSupabaseStatusReporter(args.StatusEndpoint, args.LogToken, args.ExecutionID)
		statusObservers = append(statusObservers, statusReporter)
	}

	multiLogs := notify.NewMultiObserver(logObservers...)
	multiStatus := notify.NewMultiStatusObserver(statusObservers...)

	createExec := c.createExec.WithObservers(multiLogs, multiStatus)

	output, runErr := createExec.Execute(ctx, requestInput, args.ExecutionID)

	multiLogs.Close()

	logsLost := false
	if supabaseLogs != nil {
		logsLost = supabaseLogs.LogsLost()
	}

	// El status terminal ya no se deduce aquí a partir del error: lo publica el
	// agregado, que es quien lo sabe (spec 07 §5.2). Lo que esta capa decide es
	// sólo con qué exit code sale el PROCESO, que es asunto suyo.
	terminalStatus := output.Status
	if terminalStatus == "" {
		// Un request rechazado en la validación no llega a tener agregado, así
		// que no hay estado que publicar: es un fallo y así se reporta.
		terminalStatus = command.StatusFailed.String()
	}
	exitCode := ExitSucceeded
	errMsg := ""
	switch {
	case terminalStatus == command.StatusCancelled.String():
		exitCode = ExitCancelled
		if runErr != nil {
			errMsg = runErr.Error()
		}
		fmt.Fprintf(stderr, "vexd run: execution %s cancelled\n", output.ExecutionID)
	case runErr != nil:
		exitCode = ExitFailed
		errMsg = runErr.Error()
		fmt.Fprintf(stderr, "vexd run: execution %s failed: %v\n", output.ExecutionID, runErr)
	}

	if statusReporter != nil {
		if err := statusReporter.ReportTerminal(terminalStatus, exitCode, logsLost, errMsg); err != nil {
			fmt.Fprintf(stderr, "vexd run: report terminal status: %v\n", err)
		}
	}

	return exitCode
}

// readInput resuelve la prioridad: --input <file> > env var > stdin.
// La env var se acepta tanto en raw JSON como en base64+JSON: si el primer
// byte no es '{' se intenta decodificar base64 antes de fallar.
func readInput(stdin io.Reader, args RunArgs) ([]byte, error) {
	if args.InputFile != "" {
		data, err := os.ReadFile(args.InputFile)
		if err != nil {
			return nil, fmt.Errorf("read input file %s: %w", args.InputFile, err)
		}
		return data, nil
	}

	envVar := args.InputEnv
	if envVar == "" {
		envVar = "VEX_REQUEST_INPUT"
	}
	if raw := os.Getenv(envVar); raw != "" {
		trimmed := strings.TrimSpace(raw)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			return []byte(trimmed), nil
		}
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("env var %s: not JSON nor valid base64: %w", envVar, err)
		}
		return decoded, nil
	}

	if stdin == nil {
		return nil, errors.New("no input source: --input, env var, and stdin are all empty")
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("no input source: --input, env var, and stdin are all empty")
	}
	return data, nil
}
