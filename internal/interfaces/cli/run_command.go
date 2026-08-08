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
	"github.com/jairoprogramador/vex-engine/internal/domain/syncconfig"
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
//
// Sube a 2 con la spec 16, y subir es el punto: la ruptura es de CONTRATO y no
// de memoria de nadie. Un cliente de la v1 pasaba `--mode` y seis endpoints de
// Supabase, banderas que este binario ya no acepta; sin este número, ese cliente
// arrancaría el motor y descubriría el problema a mitad del despliegue.
const supportedSchemaVersion = 2

// RunArgs son los flags de `vexd run` mapeados desde Cobra.
//
// Lo que la spec 16 retiró de aquí, y por qué no queda nada de ello: `Mode`, que
// elegía familias enteras de repositorios y traía «Supabase» como default
// cableado dentro de la pieza que debe ser portable; y los seis endpoints de las
// edge functions del estado (`--step-{code,inst,time,vars,delete}-endpoint`,
// muertos desde las specs 09 y 10, y `--step-store-vars-endpoint`, que era el
// último con lector). Su reemplazo es `StateConfigFile`: dato, no bandera.
//
// `LogEndpoint` y `StatusEndpoint` SOBREVIVEN. La revisión los metía en el mismo
// saco que los seis y concluía que la pérdida era de rendimiento; el código dice
// otra cosa: `--status-endpoint` transporta el estado terminal y es la única
// señal que el portal tiene de que el contenedor terminó. Retirarlo con
// `type: http` congelado dejaría toda ejecución remota en `running` hasta que un
// TTL la marcara `error`. Se van cuando el sink `http` los subsuma (spec 26).
type RunArgs struct {
	InputFile      string
	InputEnv       string
	LogEndpoint    string
	StatusEndpoint string

	// StateConfigFile es la configuración de DESTINO del estado (`type` +
	// payload). Sin default: si está vacío se prueba la env var
	// VEX_STATE_CONFIG, y si tampoco, el motor no arranca (ver readStateConfig).
	StateConfigFile string

	// StagingDir es el área de trabajo del motor. A diferencia del destino, esto
	// SÍ tiene default —es dominio del motor, no del invocador— y vacío significa
	// la cadena XDG → $HOME/.local/state → TempDir (ver resolveStagingDir).
	StagingDir string

	LogToken    string
	ExecutionID string
	Quiet       bool
}

// RunCommand orquesta la ejecución one-shot del engine. Es la única superficie
// CLI que invoca al use case CreateExecution y reemplaza al antiguo HTTP server.
type RunCommand struct {
	createExec *usecase.CreateExecutionUseCase

	// destino y stagingDir no los usa Execute todavía: los escribe la spec 21,
	// que empuja lo registrado desde el área de trabajo hacia el destino. Están
	// aquí porque los dos se RESUELVEN al cablear —el destino se comprueba antes
	// del primer step y el área de trabajo se crea— y quien los resolvió es quien
	// tiene que poder decir cuáles son.
	destino    syncconfig.Config
	stagingDir string
}

func NewRunCommand(
	createExec *usecase.CreateExecutionUseCase,
	destino syncconfig.Config,
	stagingDir string,
) *RunCommand {
	return &RunCommand{createExec: createExec, destino: destino, stagingDir: stagingDir}
}

// Destino es el destino del estado con el que se cableó el motor.
func (c *RunCommand) Destino() syncconfig.Config { return c.destino }

// StagingDir es el área de trabajo propia del motor, ya creada y escribible.
func (c *RunCommand) StagingDir() string { return c.stagingDir }

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
		// El mensaje dice QUÉ cambió, no sólo que no coincide: quien llega aquí
		// con una v1 no tiene un JSON corrupto, tiene un cliente que aprendió a
		// hablar con un motor que ya no existe (spec 16 §5.7).
		fmt.Fprintf(stderr, "vexd run: unsupported schema_version: %d (this binary supports v%d)\n",
			requestInput.SchemaVersion, supportedSchemaVersion)
		if requestInput.SchemaVersion < supportedSchemaVersion {
			fmt.Fprintf(stderr,
				"vexd run: la v%d retira --mode y los seis --step-*-endpoint; el destino del estado se pasa"+
					" ahora con --state-config (type: local|http) o con la env var %s. La forma del"+
					" RequestInput no cambió: sólo su versión.\n",
				supportedSchemaVersion, StateConfigEnvVar)
		}
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
