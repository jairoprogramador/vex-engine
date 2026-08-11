package cli_test

// Andamiaje del harness de integración (spec 01).
//
// Todo lo que hay aquí existe para que `RunCommand.Execute` pueda correr con el
// cableado REAL —las tres cadenas, la policy y los repositorios de archivo— sin
// docker, sin la CLI `vex`, sin red y sin tocar el $HOME de quien corre los
// tests. Los casos viven en run_command_integration_test.go.
//
// Tres piezas:
//
//  1. transporte git en proceso (installGitTransport): el pipelinecode SÍ se
//     clona de verdad, pero contra un repositorio en un directorio temporal.
//  2. fixture (newHarness): materializa testdata/pipelinecode/ en un repo git
//     temporal y crea el repo del proyecto.
//  3. Object Mother (h.request / requestOption): construye el RequestInput con
//     defaults sensatos y una opción funcional por campo que varía.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
	infraCache "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	infraDeployment "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	infraRecord "github.com/jairoprogramador/vex-engine/internal/infrastructure/record"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	infraSync "github.com/jairoprogramador/vex-engine/internal/infrastructure/sync"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// ── 1. Transporte git en proceso ────────────────────────────────────────────

// gitHost es el host de las URLs de prueba. Tiene que ser https:// porque
// shared.NewRepositoryURL rechaza cualquier otra forma; el transporte instalado
// abajo hace que esas URLs nunca salgan del proceso.
const gitHost = "https://vex.test"

var (
	gitRepos     sync.Map // path del endpoint → directorio del repo en disco
	gitTransport sync.Once
)

// installGitTransport reemplaza el transporte https del proceso de test por el
// servidor git en memoria de go-git, resolviendo cada endpoint contra un
// directorio local. Se instala una sola vez para todo el binario de test.
func installGitTransport() {
	gitTransport.Do(func() {
		client.InstallProtocol("https", noShallowTransport{server.NewClient(localLoader{})})
	})
}

type localLoader struct{}

func (localLoader) Load(ep *transport.Endpoint) (storer.Storer, error) {
	dir, ok := gitRepos.Load(ep.Path)
	if !ok {
		return nil, transport.ErrRepositoryNotFound
	}
	gitDir := filepath.Join(dir.(string), ".git")
	return filesystem.NewStorage(osfs.New(gitDir), cache.NewObjectLRUDefault()), nil
}

// noShallowTransport degrada los clones shallow a clones completos. Los dos
// repositorios de clonación piden `Depth` (201 el proyecto, 1 el pipeline) y el
// servidor en proceso de go-git no implementa la capability `shallow`. Los
// repos del fixture tienen un puñado de commits, así que traerlos enteros es
// equivalente y no altera nada de lo que el harness observa.
type noShallowTransport struct{ transport.Transport }

func (t noShallowTransport) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	session, err := t.Transport.NewUploadPackSession(ep, auth)
	if err != nil {
		return nil, err
	}
	return noShallowSession{session}, nil
}

type noShallowSession struct{ transport.UploadPackSession }

func (s noShallowSession) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	req.Depth = packp.DepthCommits(0)
	req.Capabilities.Delete(capability.Shallow)
	return s.UploadPackSession.UploadPack(ctx, req)
}

// ── 2. Fixture ──────────────────────────────────────────────────────────────

const (
	fixtureRef         = "main"
	fixtureEnvironment = "sand"
	fixtureStep        = "supply"
)

// ventanaDelFixture es la ventana de reutilización del clon que declara el
// pipelinecode de prueba (spec 18 §5.4), y es diminuta a propósito: el fixture
// CAMBIA entre corridas —los casos commitean otro `commands.yaml` y vuelven a
// ejecutar— así que con la ventana por defecto de 24 h la segunda corrida
// reutilizaría el clon y no vería el cambio.
//
// Un pipelinecode que se está escribiendo declara una ventana corta, que es
// exactamente la razón por la que el parámetro lo declara el pipelinecode y no
// el motor. La ventana por defecto y la reutilización de verdad las miden los
// casos que hablan de ellas, borrando esta línea del fixture.
const ventanaDelFixture = "clone_window: 1ms\n"

var harnessSeq struct {
	sync.Mutex
	n int
}

func nextHarnessID() int {
	harnessSeq.Lock()
	defer harnessSeq.Unlock()
	harnessSeq.n++
	return harnessSeq.n
}

type harness struct {
	t *testing.T

	// root es la raíz de almacenamiento inyectada: hace de $HOME. De aquí
	// cuelgan los clones y los workdirs (root/.vex/), y desde la spec 16 NADA
	// más: el estado vive en el destino.
	root string

	// destino es la configuración de destino del estado hecha directorio: lo que
	// en producción es el volumen montado. `state/` y `cache/` cuelgan de aquí.
	destino string

	// stateConfig es el archivo que se le pasa por --state-config. El harness
	// ejercita el transporte real, que es lo que la spec 16 §7 pide: adaptar el
	// Object Mother, no inventar una vía sólo para los tests.
	stateConfig string

	// staging es el área de trabajo propia del motor. Se fija por flag para que
	// los tests no escriban en el $HOME real de quien los corre; la cadena de
	// resolución por defecto la prueba TestResolveStagingDir_*.
	staging string

	// projectDir es el proyecto a desplegar: en modo local el motor lo enlaza
	// en vez de clonarlo, pero necesita un repo git con HEAD para el versionado
	// y para ${var.project_revision}.
	projectDir string

	// pipelineDir es el repo fuente del pipelinecode; el motor lo clona.
	pipelineDir string

	projectID   string
	projectURL  string
	pipelineURL string

	// execLog lo alimenta cada comando del fixture vía `tee -a "$VEX_TEST_LOG"`.
	// Es la señal de "este step corrió" que distingue ejecución de skip.
	execLog string
}

// harnessOption modifica el pipelinecode del fixture antes de que se commitee,
// para los casos que necesitan un comando distinto (fallo, probe sin match…).
type harnessOption func(*harness)

// withPipelineFile sobrescribe un archivo del pipelinecode materializado.
func withPipelineFile(relPath, content string) harnessOption {
	return func(h *harness) {
		writeFile(h.t, filepath.Join(h.pipelineDir, relPath), content)
	}
}

// configConScope es el `config.yaml` de un step con el ámbito que se le diga y
// la regla de invalidación completa —`- state_changed`, que equivale a
// `[pipeline, project]`—, que es lo que declara el fixture.
//
// Existe desde la spec 15 porque hacen falta las DOS claves: un `config.yaml`
// con `scope` y sin `rules` declara dónde vive el estado y ninguna razón para
// desconfiar de él, así que el step se ejecuta siempre y no persiste nada (§5.5).
// Escribir sólo el `scope` en un caso que mide otra cosa cambiaría lo que ese
// caso mide.
func configConScope(scope string) string {
	return "scope: " + scope + "\nrules:\n  - state_changed\n"
}

// withoutPipelineFile borra un archivo del pipelinecode materializado. Existe
// para el caso de la spec 13 §5.3 —un step SIN `config.yaml`—, que es una
// AUSENCIA y no se puede montar escribiendo nada.
func withoutPipelineFile(relPath string) harnessOption {
	return func(h *harness) {
		require.NoError(h.t, os.Remove(filepath.Join(h.pipelineDir, relPath)))
	}
}

func newHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	installGitTransport()

	id := nextHarnessID()
	base := t.TempDir()

	h := &harness{
		t:           t,
		root:        filepath.Join(base, "home"),
		destino:     filepath.Join(base, "destino"),
		stateConfig: filepath.Join(base, "state-config.yaml"),
		staging:     filepath.Join(base, "staging"),
		projectDir:  filepath.Join(base, "project"),
		pipelineDir: filepath.Join(base, "pipelinecode"),
		// El identificador va POR HARNESS, igual que las urls: dos harness son dos
		// proyectos, y desde la spec 20 eso importa —la clave con la que se resume
		// el valor de un parámetro se deriva de este identificador, así que un id
		// compartido haría indistinguibles dos proyectos que no lo son—.
		projectID:   fmt.Sprintf("%08d-1111-1111-1111-111111111111", id),
		projectURL:  fmt.Sprintf("%s/vex-test-%d/demo-app", gitHost, id),
		pipelineURL: fmt.Sprintf("%s/vex-test-%d/pipelinecode", gitHost, id),
		execLog:     filepath.Join(base, "exec.log"),
	}

	require.NoError(t, os.MkdirAll(h.root, 0o755))
	h.prepararDestino()
	writeFile(t, h.execLog, "")
	t.Setenv("VEX_TEST_LOG", h.execLog)
	// El input siempre llega por --input o por stdin salvo en el caso que la
	// prueba explícitamente; con la env var a "" readInput la ignora. Lo mismo
	// vale para la configuración de destino, que llega por --state-config.
	t.Setenv("VEX_REQUEST_INPUT", "")
	t.Setenv(cli.StateConfigEnvVar, "")

	// Proyecto: dos archivos y un commit convencional. Sin tag, así que el
	// versionado cae en la versión por defecto y luego en la versión por fecha.
	writeFile(t, filepath.Join(h.projectDir, "README.md"), "# demo-app\n")
	writeFile(t, filepath.Join(h.projectDir, "src", "app.txt"), "v1\n")
	initRepo(t, h.projectDir, "feat: proyecto inicial")

	// Pipelinecode: testdata materializado en un repo temporal. No puede vivir
	// como repo dentro de testdata/ (git anidado), así que se copia.
	copyTree(t, filepath.Join("testdata", "pipelinecode"), h.pipelineDir)
	for _, opt := range opts {
		opt(h)
	}
	initRepo(t, h.pipelineDir, "feat: pipelinecode inicial")

	gitRepos.Store(endpointPath(h.projectURL), h.projectDir)
	gitRepos.Store(endpointPath(h.pipelineURL), h.pipelineDir)

	return h
}

func endpointPath(url string) string {
	return strings.TrimPrefix(url, gitHost)
}

// ── 3. Object Mother del RequestInput ───────────────────────────────────────

type requestOption func(*dto.RequestInput)

func withStep(step string) requestOption {
	return func(r *dto.RequestInput) { r.Execution.Step = step }
}

func withEnvironment(environment string) requestOption {
	return func(r *dto.RequestInput) { r.Execution.Environment = environment }
}

func withSchemaVersion(version int) requestOption {
	return func(r *dto.RequestInput) { r.SchemaVersion = version }
}

// withProjectTeam permite el campo vacío, que es el disparador cotidiano de la
// spec 03: un proyecto sin equipo asignado.
func withProjectTeam(team string) requestOption {
	return func(r *dto.RequestInput) { r.Project.Team = team }
}

// withProjectId cambia —o vacía— el identificador del proyecto. Importa desde la
// spec 20: de él se deriva la clave con la que se resume el valor de cada
// parámetro.
func withProjectId(id string) requestOption {
	return func(r *dto.RequestInput) { r.Project.Id = id }
}

func (h *harness) request(opts ...requestOption) dto.RequestInput {
	request := dto.RequestInput{
		SchemaVersion: 2,
		Project: dto.ProjectInput{
			Id:   h.projectID,
			Name: "demo-app",
			Team: "plataforma",
			Org:  "acme",
			Url:  h.projectURL,
			Ref:  fixtureRef,
		},
		Pipeline: dto.PipelineInput{
			Url: h.pipelineURL,
			Ref: fixtureRef,
		},
		Execution: dto.ExecutionInput{
			Step:        fixtureStep,
			Environment: fixtureEnvironment,
		},
	}
	for _, opt := range opts {
		opt(&request)
	}
	return request
}

// ── Ejecución ───────────────────────────────────────────────────────────────

type runResult struct {
	exitCode int
	stdout   string
	stderr   string
}

// prepararDestino materializa el volumen del destino y el archivo de
// configuración que lo apunta. Es el reemplazo del `Mode: ModeLocal` que el
// harness pasaba hasta la spec 16: lo que antes era un enum es ahora un dato con
// una ruta dentro.
func (h *harness) prepararDestino() {
	h.t.Helper()
	require.NoError(h.t, os.MkdirAll(h.destino, 0o755))
	writeFile(h.t, h.stateConfig, "type: local\nlocal:\n  path: "+h.destino+"\n")
}

func (h *harness) args() cli.RunArgs {
	// Quiet suprime el observer de stdout, que escribe en os.Stdout del proceso
	// y no en el writer que se le pasa a Execute.
	//
	// El área de trabajo va por flag: sin ella la cadena de resolución caería en
	// `$HOME/.local/state` y los tests escribirían en el home real.
	return cli.RunArgs{
		Quiet:           true,
		StateConfigFile: h.stateConfig,
		StagingDir:      h.staging,
	}
}

// run escribe el RequestInput en un archivo y ejecuta la vía --input.
func (h *harness) run(opts ...requestOption) runResult {
	h.t.Helper()
	args := h.args()
	args.InputFile = h.writeRequest(h.request(opts...))
	return h.execute(args, nil)
}

// writeRequest deja el RequestInput en un archivo temporal y devuelve su ruta.
func (h *harness) writeRequest(request dto.RequestInput) string {
	h.t.Helper()
	path := filepath.Join(h.t.TempDir(), fmt.Sprintf("request-%d.json", nextHarnessID()))
	writeFile(h.t, path, string(h.marshal(request)))
	return path
}

func (h *harness) execute(args cli.RunArgs, stdin io.Reader) runResult {
	return h.executeCtx(context.Background(), args, stdin)
}

// build cablea el motor sin ejecutarlo. Los casos de la spec 16 que fallan ANTES
// del primer step —destino ausente, congelado o inalcanzable— se observan aquí:
// el error del cableado y su exit code son el entregable.
func (h *harness) build(args cli.RunArgs) (*cli.RunCommand, error) {
	h.t.Helper()
	return cli.BuildRunCommand(cli.EngineConfig{
		RootVexPath:      h.root,
		LocalProjectPath: h.projectDir,
	}, args)
}

// estaBajoElVolumen replica la comprobación de §5.3 desde fuera del paquete, que
// es donde tiene valor: si `resolveStagingDir` se equivocara, un test que use su
// propia función lo detecta y uno que llame a la suya, no.
func estaBajoElVolumen(path, rootVexPath string) bool {
	volumen := filepath.Clean(filepath.Join(rootVexPath, cli.VexHomeDirName))
	path = filepath.Clean(path)
	return path == volumen || strings.HasPrefix(path, volumen+string(filepath.Separator))
}

// executeCtx es el único punto que construye el motor: mismo cableado que el
// binario, con las rutas del fixture en lugar de $HOME y /appProject.
//
// El contexto se recibe para poder cancelarlo: es lo que hace el manejador de
// señales de cmd/vexd al recibir un SIGINT (spec 07 §5.4), y lo único de esa
// ruta que no depende de mandarle una señal de verdad al proceso de test.
func (h *harness) executeCtx(ctx context.Context, args cli.RunArgs, stdin io.Reader) runResult {
	h.t.Helper()

	runCmd, err := h.build(args)
	require.NoError(h.t, err)
	require.Equal(h.t, h.staging, runCmd.StagingDir())
	require.False(h.t, estaBajoElVolumen(h.staging, h.root),
		"el área de trabajo del motor no puede caer bajo el volumen (spec 16 §5.3)")

	var stdout, stderr bytes.Buffer
	code := runCmd.Execute(ctx, stdin, &stdout, &stderr, args)
	h.assertNingunaVariableAnonima()
	return runResult{exitCode: code, stdout: stdout.String(), stderr: stderr.String()}
}

func (h *harness) marshal(request dto.RequestInput) []byte {
	h.t.Helper()
	data, err := json.Marshal(request)
	require.NoError(h.t, err)
	return data
}

// ── Observación ─────────────────────────────────────────────────────────────

// ranSteps devuelve el prefijo de step de cada línea que los comandos del
// fixture dejaron en el log ("01-test", "02-supply"), en orden.
func (h *harness) ranSteps() []string {
	h.t.Helper()
	steps := make([]string, 0, 4)
	for _, line := range h.logLines() {
		steps = append(steps, strings.SplitN(line, " ", 2)[0])
	}
	return steps
}

func (h *harness) logLines() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.execLog)
	require.NoError(h.t, err)
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (h *harness) resetLog() {
	h.t.Helper()
	writeFile(h.t, h.execLog, "")
}

// statePath es la raíz de la tienda NO desechable: un registro por ejecución
// real de un step, y de ahí no se borra nada (spec 11).
//
// Cuelga del DESTINO desde la spec 16, no del $HOME del proceso: es lo que
// permite que dos máquinas compartan almacén sin compartir home.
func (h *harness) statePath() string {
	return filepath.Join(h.destino, "state")
}

// cachePath es la raíz del ÍNDICE, que sí es desechable — y `borrarElIndice` lo
// demuestra borrándolo.
func (h *harness) cachePath() string {
	return filepath.Join(h.destino, "cache")
}

// storedVars lee el ÚLTIMO registro del ámbito de un ambiente, con el mismo
// repositorio de archivo que usa el motor.
//
// El step va con su prefijo de orden (`02-supply`): la identidad de un step es
// su ruta, y renumerarlo pierde su historia (spec 11 §5.3).
func (h *harness) storedVars(environment, stepID string) map[string]string {
	h.t.Helper()
	scope, err := state.NewEnvironmentScope(environment)
	require.NoError(h.t, err)
	return h.recordVars(scope, stepID)
}

// projectVars lee el ÚLTIMO registro del ámbito de PROYECTO: lo que produce un
// step que declara `scope: project` y es común a todos los ambientes (spec 13).
func (h *harness) projectVars(stepID string) map[string]string {
	h.t.Helper()
	return h.recordVars(state.NewProjectScope(), stepID)
}

func (h *harness) recordVars(scope state.Scope, stepID string) map[string]string {
	h.t.Helper()

	key, err := state.NewKey(h.projectURL, scope, stepID)
	require.NoError(h.t, err)

	repo := stateInfra.NewFileRecordsRepository(h.statePath())
	ctx := context.Background()
	record, found, err := repo.Last(&ctx, key)
	require.NoError(h.t, err)
	if !found {
		return map[string]string{}
	}

	vars := record.Variables()
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		out[v.Name()] = v.Value()
	}
	return out
}

// persistedStepState devuelve la ruta relativa de todo REGISTRO que el motor
// haya escrito para un step.
//
// Cambió tres veces de sujeto y conviene saber por qué. Hasta la spec 10 miraba
// las tres huellas de la policy (`inst<step>.status`…); la 10 las borró y dejó
// aquí el almacén de variables (`<step>.vars`); la 11 lo sustituye por el
// almacén de registros. Lo que mide es lo mismo desde el principio: la mitad de
// la observación de «no persiste estado de re-ejecución» de la spec 04 §5.3 —un
// step saltado por falta de comandos no deja rastro, así que la corrida
// siguiente vuelve a saltarlo por la misma razón y no por estar al día—.
//
// Con append-only pasa a medir algo más: el CONTADOR sólo sube. Dos ejecuciones
// reales del mismo step dejan dos rutas, no una pisada.
func (h *harness) persistedStepState(step string) []string {
	h.t.Helper()

	found := make([]string, 0, 4)
	err := filepath.WalkDir(h.statePath(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		// El step es el ÚLTIMO tramo de la ruta, no parte del nombre del archivo:
		// el archivo se llama como su ULID.
		if !strings.Contains(filepath.Base(filepath.Dir(path)), step) {
			return nil
		}
		rel, err := filepath.Rel(h.statePath(), path)
		if err != nil {
			return err
		}
		found = append(found, rel)
		return nil
	})
	if os.IsNotExist(err) {
		return found
	}
	require.NoError(h.t, err)
	sort.Strings(found)
	return found
}

// envejecerRegistros retrasa `produced_by.at` de TODOS los registros escritos,
// que es lo que mide `max_age` (spec 15 §5.2).
//
// Es la única forma de probar la expiración de punta a punta: el instante lo
// pone el reloj del proceso a través del agregado (spec 07), y el harness
// construye el motor con el cableado real, así que no hay dónde inyectar otro
// sin dejar de probar el cableado que se quiere probar. Envejecer lo escrito es
// equivalente y no toca nada del motor.
//
// El `record_id` NO se toca, y no hace falta: es un ULID y el orden lexicográfico
// decide cuál es el último, así que mover la fecha de dentro no reordena nada.
// Lo que se está falsificando es la EDAD del hecho, no cuál fue el último.
func (h *harness) envejecerRegistros(edad time.Duration) {
	h.t.Helper()

	tocados := 0
	err := filepath.WalkDir(h.statePath(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var stored stateInfra.FileStepRecordDTO
		if err := json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("decodificar %s: %w", path, err)
		}

		at, err := time.Parse(time.RFC3339Nano, stored.ProducedBy.At)
		if err != nil {
			return fmt.Errorf("leer produced_by.at de %s: %w", path, err)
		}
		stored.ProducedBy.At = at.Add(-edad).Format(time.RFC3339Nano)

		reescrito, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		tocados++
		return os.WriteFile(path, reescrito, 0o644)
	})
	require.NoError(h.t, err)
	require.NotZero(h.t, tocados, "no había ningún registro que envejecer")
}

// ── El resumen de los valores (spec 20) ─────────────────────────────────────

// rutaDelSecreto es donde el motor guarda el secreto local del que deriva las
// claves de resumen. Cuelga del DESTINO —es lo único que dos máquinas efímeras
// comparten— y de un directorio propio, fuera de todo lo que se sincroniza.
func (h *harness) rutaDelSecreto() string {
	return filepath.Join(h.destino, "keys", "digest-v1.key")
}

// digestDe es el resumen que la ÚLTIMA ejecución emitió para un parámetro.
//
// Se lee de la última tira y no de `hechos()` porque aquélla ACUMULA: dos
// corridas del mismo harness dejan dos `parameter_resolved` del mismo nombre, y
// lo que los casos comparan es el de cada corrida.
func (h *harness) digestDe(nombre string) string {
	h.t.Helper()

	for _, hecho := range deTipo(h.ultimaTira(), record.TypeParameterResolved.String()) {
		if texto(hecho.Payload, "name") == nombre {
			return texto(hecho.Payload, "digest")
		}
	}
	h.t.Fatalf("la última tira no tiene ningún parámetro llamado %q", nombre)
	return ""
}

// valoresDeLaEjecucion son los valores que la ejecución produjo o declaró, para
// poder afirmar que NINGUNO aparece en el registro (spec 20 §7).
//
// Los producidos se leen del almacén de estado, que es donde el mapa acumulado
// sobrevive a la ejecución; el literal del pipelinecode se nombra aquí porque no
// llega a persistirse desde la spec 14 —lo que el registro de un step guarda es
// lo que ese step PRODUJO— y aun así es un valor del que la promesa habla.
func (h *harness) valoresDeLaEjecucion() map[string]string {
	h.t.Helper()

	valores := map[string]string{"registry_prefix": "vexsand"}
	for _, stepID := range []string{"01-test", "02-supply"} {
		for nombre, valor := range h.storedVars(fixtureEnvironment, stepID) {
			valores[nombre] = valor
		}
	}
	return valores
}

// ── El registro de despliegue (spec 18) ─────────────────────────────────────

// objetos son los objetos de despliegue escritos, ordenados por `content_id`.
//
// Viven en el ÁREA DE TRABAJO y no en el destino: nadie los lee durante la
// ejecución, así que se bufferizan y los empuja la spec 21. El área es estable
// entre corridas del mismo harness, así que esta lista ACUMULA — y que dos
// corridas idénticas dejen UN objeto es la forma directa de observar que la
// tienda es write-once y direccionada por contenido.
func (h *harness) objetos() []infraDeployment.FileObjectDTO {
	h.t.Helper()
	return h.objetosEn(h.staging)
}

// objetosDelDestino son los objetos que el EMPUJE dejó en el destino (spec 21).
// Su lista tiene que acabar siendo la misma que la del área de trabajo: el
// empuje no filtra, transporta.
func (h *harness) objetosDelDestino() []infraDeployment.FileObjectDTO {
	h.t.Helper()
	return h.objetosEn(h.destino)
}

func (h *harness) objetosEn(base string) []infraDeployment.FileObjectDTO {
	h.t.Helper()

	objetos := make([]infraDeployment.FileObjectDTO, 0, 2)
	err := filepath.WalkDir(filepath.Join(base, "objects"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".json" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var dto infraDeployment.FileObjectDTO
			if err := json.Unmarshal(data, &dto); err != nil {
				return fmt.Errorf("decodificar %s: %w", path, err)
			}
			objetos = append(objetos, dto)
			return nil
		})
	if os.IsNotExist(err) {
		return objetos
	}
	require.NoError(h.t, err)

	sort.Slice(objetos, func(i, j int) bool { return objetos[i].ContentID < objetos[j].ContentID })
	return objetos
}

// elObjeto es el único objeto escrito. Falla si hay más de uno: los casos que
// esperan varios los enumeran con `objetos()`.
func (h *harness) elObjeto() infraDeployment.FileObjectDTO {
	h.t.Helper()
	objetos := h.objetos()
	require.Len(h.t, objetos, 1, "se esperaba exactamente un objeto de despliegue")
	return objetos[0]
}

// hechos son los eventos escritos, en el orden de `seq` dentro de cada tira.
//
// Se leen del JSONL con el DTO real: lo que el motor escribe es lo que un
// ingestor de otra plataforma va a leer, y una prueba que reconstruyera el
// evento por otra vía no estaría comprobando eso.
func (h *harness) hechos() []infraRecord.JSONLEventDTO {
	h.t.Helper()
	return h.hechosEn(h.staging)
}

// hechosDelDestino son los hechos que el EMPUJE dejó en el destino (spec 21).
func (h *harness) hechosDelDestino() []infraRecord.JSONLEventDTO {
	h.t.Helper()
	return h.hechosEn(h.destino)
}

func (h *harness) hechosEn(base string) []infraRecord.JSONLEventDTO {
	h.t.Helper()

	hechos := make([]infraRecord.JSONLEventDTO, 0, 4)
	err := filepath.WalkDir(filepath.Join(base, "events"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, linea := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(linea) == "" {
					continue
				}
				var dto infraRecord.JSONLEventDTO
				if err := json.Unmarshal([]byte(linea), &dto); err != nil {
					return fmt.Errorf("decodificar un hecho de %s: %w", path, err)
				}
				hechos = append(hechos, dto)
			}
			return nil
		})
	if os.IsNotExist(err) {
		return hechos
	}
	require.NoError(h.t, err)

	sort.SliceStable(hechos, func(i, j int) bool { return hechos[i].Seq < hechos[j].Seq })
	return hechos
}

// jsonlEvento es la línea del archivo de hechos, con el nombre corto que los
// casos usan.
type jsonlEvento = infraRecord.JSONLEventDTO

// ultimaTira son los hechos de la ÚLTIMA ejecución, en el orden de su `seq`.
//
// Hace falta porque `hechos()` ACUMULA —el área de trabajo es estable entre
// corridas del mismo harness— y `seq` es la posición dentro de UN intento, no
// una secuencia global: ordenar dos corridas por `seq` las intercala. Es la
// misma propiedad que hace útil el modelo (cada intento se numera solo) vista
// desde el lado incómodo.
//
// «Última» se resuelve por instante de modificación del archivo y no por su
// nombre: el archivo lo nombra el `execution_id`, que es único pero no
// ordenable.
func (h *harness) ultimaTira() []jsonlEvento {
	h.t.Helper()

	var ultimo string
	var cuando time.Time
	err := filepath.WalkDir(filepath.Join(h.staging, "events"),
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if ultimo == "" || info.ModTime().After(cuando) {
				ultimo, cuando = path, info.ModTime()
			}
			return nil
		})
	require.NoError(h.t, err)
	require.NotEmpty(h.t, ultimo, "no hay ninguna tira de hechos escrita")

	return h.tira(ultimo)
}

func (h *harness) tira(path string) []jsonlEvento {
	h.t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(h.t, err)

	hechos := make([]jsonlEvento, 0, 8)
	for _, linea := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(linea) == "" {
			continue
		}
		var dto jsonlEvento
		require.NoError(h.t, json.Unmarshal([]byte(linea), &dto))
		hechos = append(hechos, dto)
	}
	sort.SliceStable(hechos, func(i, j int) bool { return hechos[i].Seq < hechos[j].Seq })
	return hechos
}

// deTipo filtra una tira ya seleccionada.
func deTipo(hechos []jsonlEvento, tipo string) []jsonlEvento {
	filtrados := make([]jsonlEvento, 0, 2)
	for _, hecho := range hechos {
		if hecho.Type == tipo {
			filtrados = append(filtrados, hecho)
		}
	}
	return filtrados
}

// hechosDeTipo filtra el vocabulario.
func (h *harness) hechosDeTipo(tipo string) []jsonlEvento {
	h.t.Helper()
	filtrados := make([]jsonlEvento, 0, 2)
	for _, hecho := range h.hechos() {
		if hecho.Type == tipo {
			filtrados = append(filtrados, hecho)
		}
	}
	return filtrados
}

// eventos son los mismos hechos DEVUELTOS AL DOMINIO, para poder plegarlos.
//
// # Por qué el decodificador vive aquí y no en infraestructura
//
// Porque todavía no tiene otro consumidor. El motor sólo ESCRIBE hechos; quien
// los lee es `record log` (spec 22) y el pliegue del backend (spec 26), y
// escribir hoy el lector de producción sería fijar una superficie sin nadie
// detrás. Lo que la spec 19 §7 necesita es comprobar que **emisión y modelo
// encajan**, y para eso hace falta leer lo escrito — no publicar un lector.
//
// Tiene un efecto lateral que conviene: cada campo que un emisor añada y este
// decodificador no sepa leer se nota aquí, no seis meses después en el
// consumidor.
//
// El `event_id` se reconstruye del texto y el resto del sobre también, así que
// lo único que este decodificador no comprueba es la entropía — que no entra en
// ninguna decisión del pliegue.
func (h *harness) eventos() []record.Event {
	h.t.Helper()

	hechos := h.hechos()
	eventos := make([]record.Event, 0, len(hechos))
	for _, hecho := range hechos {
		eventos = append(eventos, h.evento(hecho))
	}
	return eventos
}

func (h *harness) evento(dto jsonlEvento) record.Event {
	h.t.Helper()

	id, err := record.ParseEventID(dto.EventID)
	require.NoError(h.t, err)
	seq, err := record.NewSeq(dto.Seq)
	require.NoError(h.t, err)
	at, err := time.Parse(time.RFC3339Nano, dto.At)
	require.NoError(h.t, err)
	attempt, err := deployment.NewAttempt(dto.Attempt)
	require.NoError(h.t, err)

	evento, err := record.NewEvent(id, seq, at, attempt, h.carga(dto))
	require.NoError(h.t, err,
		"el motor escribió un hecho que su propio modelo rechaza: %s", dto.Type)
	return evento
}

// carga reconstruye la carga útil de cada tipo. El `default` es lo que avisa de
// un tipo emitido y no contemplado, igual que en el traductor de salida.
func (h *harness) carga(dto jsonlEvento) record.Payload {
	h.t.Helper()

	p := dto.Payload
	switch record.EventType(dto.Type) {
	case record.TypeAttemptStarted:
		id, err := deployment.ParseDeploymentID(texto(p, "deployment_id"))
		require.NoError(h.t, err)
		return record.AttemptStarted{
			Deployment: id,
			Actor:      texto(p, "actor"),
			Runner:     texto(p, "runner"),
		}

	case record.TypeStaleCloneUsed:
		return record.StaleCloneUsed{
			Source:   texto(p, "source"),
			AgeHours: numero(p, "age_hours"),
		}

	case record.TypeStepStarted:
		return record.StepStarted{
			StepID:          texto(p, "step_id"),
			Scope:           h.ambito(texto(p, "scope")),
			StepFingerprint: texto(p, "step_fingerprint"),
		}

	case record.TypeStepFinished:
		return record.StepFinished{
			StepID:          texto(p, "step_id"),
			Scope:           h.ambito(texto(p, "scope")),
			Status:          command.StepStatus(texto(p, "status")),
			Duration:        time.Duration(numero(p, "duration_ms")) * time.Millisecond,
			FromCache:       p["from_cache"] == true,
			Reason:          command.StepReason(texto(p, "reason")),
			StepFingerprint: texto(p, "step_fingerprint"),
			Evidence:        h.evidencia(p["evidence_from"]),
			ExitCode:        entero(p, "exit_code"),
			ErrorClass:      record.ErrorClass(texto(p, "error_class")),
		}

	case record.TypeCommandStarted:
		return record.CommandStarted{
			StepID:      texto(p, "step_id"),
			CommandName: texto(p, "command"),
		}

	case record.TypeCommandFinished:
		return record.CommandFinished{
			StepID:      texto(p, "step_id"),
			CommandName: texto(p, "command"),
			Status:      command.CommandStatus(texto(p, "status")),
			Duration:    time.Duration(numero(p, "duration_ms")) * time.Millisecond,
			ExitCode:    int(numero(p, "exit_code")),
			ErrorClass:  record.ErrorClass(texto(p, "error_class")),
		}

	case record.TypeParameterResolved:
		return record.ParameterResolved{
			Name:   texto(p, "name"),
			Source: h.origen(texto(p, "source")),
			Digest: texto(p, "digest"),
		}

	case record.TypeArtifactProduced:
		return record.ArtifactProduced{
			StepID: texto(p, "step_id"),
			Kind:   texto(p, "type"),
			Digest: texto(p, "digest"),
		}

	case record.TypeSyncFailed:
		return record.SyncFailed{
			Destination: texto(p, "destination"),
			Cause:       texto(p, "cause"),
		}

	case record.TypeAttemptFinished:
		return record.AttemptFinished{Status: record.AttemptStatus(texto(p, "status"))}

	default:
		h.t.Fatalf("hecho de tipo desconocido en el archivo: %s", dto.Type)
		return nil
	}
}

func (h *harness) evidencia(valor any) record.EvidenceRef {
	h.t.Helper()

	crudo, ok := valor.(map[string]any)
	if !ok {
		return record.EvidenceRef{}
	}

	clave, ok := crudo["state_key"].(map[string]any)
	require.True(h.t, ok, "la evidencia no dice dónde vive el registro")
	key, err := state.NewKey(
		texto(clave, "subject"), h.ambito(texto(clave, "scope")), texto(clave, "step_id"))
	require.NoError(h.t, err)
	recordID, err := state.ParseRecordID(texto(crudo, "record_id"))
	require.NoError(h.t, err)
	at, err := time.Parse(time.RFC3339Nano, texto(crudo, "at"))
	require.NoError(h.t, err)

	return record.EvidenceRef{
		ExecutionID: texto(crudo, "execution_id"),
		At:          at,
		StateKey:    key,
		RecordID:    recordID,
	}
}

func (h *harness) ambito(texto string) state.Scope {
	h.t.Helper()
	if texto == "" {
		return state.Scope{}
	}
	scope, err := state.ParseScope(texto)
	require.NoError(h.t, err)
	return scope
}

// origen traduce la forma externa del `Origin` de vuelta al enum. El orden del
// enum ES la precedencia, así que no se puede derivar de un número: se compara
// contra el mismo `String()` que lo escribió.
func (h *harness) origen(nombre string) command.Origin {
	h.t.Helper()
	for _, origen := range []command.Origin{
		command.OriginDeclared, command.OriginState, command.OriginInjected,
		command.OriginResolved, command.OriginRuntime,
	} {
		if origen.String() == nombre {
			return origen
		}
	}
	h.t.Fatalf("origen desconocido en el archivo: %s", nombre)
	return command.OriginDeclared
}

func texto(payload map[string]any, clave string) string {
	valor, _ := payload[clave].(string)
	return valor
}

func numero(payload map[string]any, clave string) float64 {
	valor, _ := payload[clave].(float64)
	return valor
}

// entero devuelve nil cuando la clave no está, que es lo que la ausencia de
// `exit_code` significa: no hubo ningún proceso del que reportarlo.
func entero(payload map[string]any, clave string) *int {
	valor, ok := payload[clave].(float64)
	if !ok {
		return nil
	}
	convertido := int(valor)
	return &convertido
}

// cabezaDelLinaje es el último `deployment_id` registrado para un ambiente.
//
// Cuelga del DESTINO y no del área de trabajo: hay que leerla ANTES de decidir,
// y una historia que empezara vacía en cada máquina efímera derivaría dos veces
// la misma posición.
func (h *harness) cabezaDelLinaje(environment string) string {
	h.t.Helper()

	patron := filepath.Join(h.destino, "lineage", "*", environment+".json")
	matches, err := filepath.Glob(patron)
	require.NoError(h.t, err)
	if len(matches) == 0 {
		return ""
	}
	require.Len(h.t, matches, 1)

	data, err := os.ReadFile(matches[0])
	require.NoError(h.t, err)
	var dto infraDeployment.FileLineageDTO
	require.NoError(h.t, json.Unmarshal(data, &dto))
	return dto.Head
}

// ── El empuje hacia el destino (spec 21) ────────────────────────────────────

// bloquearElDestino deja el empuje inservible sin tocar nada de lo que el
// pipeline necesita para correr: ocupa `objects/` con un ARCHIVO, así que la
// primera escritura del sink falla con ENOTDIR y `state/`, `cache/` y `lineage/`
// siguen intactos.
//
// Es la forma de montar «un destino que siempre falla» sin romper el despliegue,
// que es exactamente el escenario que §7 pide observar: el pipeline termina como
// corresponda y el hueco queda explicado.
func (h *harness) bloquearElDestino() {
	h.t.Helper()
	writeFile(h.t, h.bloqueoDelDestino(), "no soy un directorio\n")
}

func (h *harness) bloqueoDelDestino() string {
	return filepath.Join(h.destino, "objects")
}

// ackDelDestino es el puntero de confirmación que el motor dejó en el área de
// trabajo, si lo dejó.
func (h *harness) ackDelDestino() (infraSync.FileAckDTO, bool) {
	h.t.Helper()

	matches, err := filepath.Glob(filepath.Join(h.staging, "ack", "*", "*", "*.json"))
	require.NoError(h.t, err)
	if len(matches) == 0 {
		return infraSync.FileAckDTO{}, false
	}
	require.Len(h.t, matches, 1, "un puntero por tira, y el harness corre una")

	data, err := os.ReadFile(matches[0])
	require.NoError(h.t, err)
	var dto infraSync.FileAckDTO
	require.NoError(h.t, json.Unmarshal(data, &dto))
	return dto, true
}

// directoriosDelDestino son los tramos de primer nivel que el destino tiene.
// Sirve para afirmar lo que la spec 21 promete que es ENUMERABLE: lo que se
// empuja son `objects/` y `events/`, y `keys/` no está en esa lista.
func (h *harness) directoriosDelDestino() []string {
	h.t.Helper()

	entradas, err := os.ReadDir(h.destino)
	require.NoError(h.t, err)

	nombres := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		nombres = append(nombres, entrada.Name())
	}
	sort.Strings(nombres)
	return nombres
}

// elRemotoNoResponde retira el pipelinecode del transporte en proceso: a partir
// de aquí, clonarlo falla como falla un remoto caído.
func (h *harness) elRemotoNoResponde() {
	h.t.Helper()
	gitRepos.Delete(endpointPath(h.pipelineURL))
}

// envejecerElClon retrasa la marca de clonación, que es lo que la ventana de
// reutilización mide (spec 18 §5.4).
//
// Es el gemelo de `envejecerRegistros` y existe por lo mismo: el instante lo
// pone el reloj del proceso a través del cableado real, así que no hay dónde
// inyectar otro sin dejar de probar el cableado que se quiere probar.
func (h *harness) envejecerElClon(edad time.Duration) {
	h.t.Helper()

	patron := filepath.Join(h.root, cli.VexHomeDirName, "pipelines", ".clones", "*.json")
	matches, err := filepath.Glob(patron)
	require.NoError(h.t, err)
	require.Len(h.t, matches, 1, "no había ninguna marca de clon que envejecer")

	data, err := os.ReadFile(matches[0])
	require.NoError(h.t, err)
	var dto infraPipeline.FileCloneMarkerDTO
	require.NoError(h.t, json.Unmarshal(data, &dto))

	clonedAt, err := time.Parse(time.RFC3339Nano, dto.ClonedAt)
	require.NoError(h.t, err)
	dto.ClonedAt = clonedAt.Add(-edad).Format(time.RFC3339Nano)

	reescrito, err := json.Marshal(dto)
	require.NoError(h.t, err)
	require.NoError(h.t, os.WriteFile(matches[0], reescrito, 0o644))
}

// borrarElIndice es la comprobación de la spec 11 §5.5.1 hecha ejecutable:
// `rm -rf $HOME/.vex/cache` no puede cambiar una sola decisión del motor.
func (h *harness) borrarElIndice() {
	h.t.Helper()
	require.NoError(h.t, os.RemoveAll(h.cachePath()))
}

// cacheEntries devuelve las claves de las entradas del ÍNDICE escritas, tal
// como cada archivo se identifica a sí mismo, ordenadas.
//
// Se lee la clave de DENTRO del archivo y no de su ruta a propósito: la ruta es
// un detalle del almacén, la clave es el contrato. Sigue habiendo un archivo por
// contenido —dos ejecuciones con el mismo material lo reescriben en su sitio—,
// lo que ya no hay es una decisión que dependa de ellas.
func (h *harness) cacheEntries() []string {
	h.t.Helper()

	claves := make([]string, 0, 4)
	err := filepath.WalkDir(h.cachePath(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var dto infraCache.FileCacheEntryDTO
		if err := json.Unmarshal(data, &dto); err != nil {
			return fmt.Errorf("decodificar %s: %w", path, err)
		}
		claves = append(claves, dto.StepFingerprint)
		return nil
	})
	if os.IsNotExist(err) {
		return claves
	}
	require.NoError(h.t, err)
	sort.Strings(claves)
	return claves
}

// workdirFile lee un archivo de la copia de trabajo del pipelinecode —el
// directorio donde el motor interpola las plantillas EN EL SITIO—. La ruta real
// lleva el hash de las dos URLs, así que se localiza por patrón en vez de
// recalcularla.
func (h *harness) workdirFile(relPath string) string {
	h.t.Helper()

	matches, err := filepath.Glob(filepath.Join(h.workdirRoot(), relPath))
	require.NoError(h.t, err)
	require.Len(h.t, matches, 1, "se esperaba exactamente un workdir con %s", relPath)

	data, err := os.ReadFile(matches[0])
	require.NoError(h.t, err)
	return string(data)
}

// workdirRoot es la copia de trabajo del pipelinecode para el ambiente del
// fixture.
func (h *harness) workdirRoot() string {
	h.t.Helper()

	patron := filepath.Join(
		h.root, cli.VexHomeDirName, "projects", "*", "workdirs", "*", fixtureEnvironment)
	matches, err := filepath.Glob(patron)
	require.NoError(h.t, err)
	require.Len(h.t, matches, 1, "se esperaba exactamente un workdir")
	return matches[0]
}

// workdirTiene dice si un archivo sigue en la copia de trabajo. Es lo que mide
// la poda de la spec 18 §5.4.
func (h *harness) workdirTiene(relPath string) bool {
	h.t.Helper()
	_, err := os.Stat(filepath.Join(h.workdirRoot(), relPath))
	if err == nil {
		return true
	}
	require.True(h.t, os.IsNotExist(err), "stat del workdir: %v", err)
	return false
}

// escribirEnElWorkdir simula lo que un comando GENERA dentro de su directorio de
// trabajo —el `.terraform/` de un `terraform init`, el `target/` de un build—.
// No es copia del pipelinecode, así que la poda no puede tocarlo.
func (h *harness) escribirEnElWorkdir(relPath, contenido string) {
	h.t.Helper()
	writeFile(h.t, filepath.Join(h.workdirRoot(), relPath), contenido)
}

// assertNingunaVariableAnonima es el invariante GLOBAL de la spec 03: ninguna
// variable acumulada tiene nombre vacío. Se comprueba tras CADA ejecución del
// harness, no en un caso suelto, porque lo que se afirma no es que un input
// concreto esté limpio sino que ninguna ejecución puede producir la entrada
// anónima.
//
// El almacén de registros es el único sitio donde el mapa acumulado sobrevive a
// la ejecución, así que es donde se observa. Se lee decodificando el JSON a mano
// en vez de por el repositorio: `NewVariable` rechaza el nombre vacío, así que
// pasar por él convertiría la entrada anónima en un error de lectura en vez de
// en la aserción que se quiere leer al fallar.
func (h *harness) assertNingunaVariableAnonima() {
	h.t.Helper()

	err := filepath.WalkDir(h.statePath(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var stored stateInfra.FileStepRecordDTO
		if err := json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("decodificar %s: %w", path, err)
		}
		for _, variable := range stored.Variables {
			require.NotEmpty(h.t, variable.Name,
				"variable con nombre vacío en %s (valor %q): un campo vacío del proyecto se coló como entrada anónima", path, variable.Value)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return // la ejecución no llegó a escribir nada
	}
	require.NoError(h.t, err)
}

// ── Mutación del fixture entre ejecuciones ──────────────────────────────────

// commitPipelineFile cambia un archivo del pipelinecode y lo commitea: la
// siguiente ejecución vuelve a clonar y lo ve.
func (h *harness) commitPipelineFile(relPath, content string) {
	h.t.Helper()
	writeFile(h.t, filepath.Join(h.pipelineDir, relPath), content)
	commitAll(h.t, h.pipelineDir, "fix: cambio de pipelinecode")
}

// borrarPipelineFile retira un archivo del pipelinecode y lo commitea. Es la
// mitad que `commitPipelineFile` no puede montar: una AUSENCIA.
func (h *harness) borrarPipelineFile(relPath string) {
	h.t.Helper()
	require.NoError(h.t, os.Remove(filepath.Join(h.pipelineDir, relPath)))
	commitAll(h.t, h.pipelineDir, "fix: retira un archivo del pipelinecode")
}

// pipelineHead es el commit del repositorio del pipelinecode. Lo que el motor
// guarda como METADATO del objeto —nunca como identidad— tiene que poder
// compararse con la fuente.
func (h *harness) pipelineHead() string {
	h.t.Helper()
	repo, err := gogit.PlainOpen(h.pipelineDir)
	require.NoError(h.t, err)
	head, err := repo.Head()
	require.NoError(h.t, err)
	return head.Hash().String()
}

// writeProjectFile cambia el árbol de trabajo del proyecto. En modo local el
// motor lo enlaza, no lo clona, así que no hace falta commitear para que la
// huella de código lo vea.
func (h *harness) writeProjectFile(relPath, content string) {
	h.t.Helper()
	writeFile(h.t, filepath.Join(h.projectDir, relPath), content)
}

// chmodProjectFile cambia los permisos de un archivo del árbol de trabajo. Es
// lo que la spec 08 corrige: antes, quitar o poner el bit de ejecución a un
// script de despliegue cambiaba lo que pasa al desplegar y NO cambiaba la
// huella, así que el motor concluía «el código no cambió» y saltaba el step.
func (h *harness) chmodProjectFile(relPath string, mode os.FileMode) {
	h.t.Helper()
	require.NoError(h.t, os.Chmod(filepath.Join(h.projectDir, relPath), mode))
}

// symlinkProjectFile crea —o reemplaza— un enlace simbólico en el árbol de
// trabajo del proyecto.
func (h *harness) symlinkProjectFile(relPath, target string) {
	h.t.Helper()
	link := filepath.Join(h.projectDir, relPath)
	require.NoError(h.t, os.MkdirAll(filepath.Dir(link), 0o755))
	if err := os.Remove(link); err != nil {
		require.True(h.t, os.IsNotExist(err), "eliminar el enlace previo: %v", err)
	}
	require.NoError(h.t, os.Symlink(target, link))
}

// otraMaquina devuelve un harness que ve EL MISMO proyecto y el mismo
// pipelinecode desde otra máquina: otro $HOME, otro destino y otra ruta absoluta
// para el árbol del proyecto, con las mismas urls.
//
// Es lo que permite observar la propiedad que hace que el destino compartido de
// la spec 16 signifique algo: la clave no depende de dónde estén los archivos. El
// árbol se copia byte a byte, incluido su `.git`, para que la versión y la
// revisión del proyecto salgan idénticas.
func (h *harness) otraMaquina() *harness {
	h.t.Helper()

	base := h.t.TempDir()
	otro := &harness{
		t:           h.t,
		root:        filepath.Join(base, "home"),
		destino:     filepath.Join(base, "destino"),
		stateConfig: filepath.Join(base, "state-config.yaml"),
		staging:     filepath.Join(base, "staging"),
		projectDir:  filepath.Join(base, "project"),
		pipelineDir: h.pipelineDir,
		projectID:   h.projectID,
		projectURL:  h.projectURL,
		pipelineURL: h.pipelineURL,
		execLog:     h.execLog,
	}

	require.NoError(h.t, os.MkdirAll(otro.root, 0o755))
	otro.prepararDestino()
	copyTree(h.t, h.projectDir, otro.projectDir)

	return otro
}

// conOtroSujeto devuelve un harness que despliega OTRO proyecto —otra url, otro
// identificador, otro directorio— con EL MISMO pipelinecode, en su propio $HOME
// y su propio destino.
//
// Existe para la mitad de §5.2bis que ningún otro montaje puede ver: que la
// DIRECCIÓN no entre en la huella. Hasta la spec 27 el sujeto era una dimensión
// de `cache.Material`, así que dos proyectos con el mismo pipelinecode producían
// declaraciones distintas y la pregunta del índice —«¿este contenido ya corrió en
// algún sitio?»— no tenía respuesta.
func (h *harness) conOtroSujeto() *harness {
	h.t.Helper()

	id := nextHarnessID()
	base := h.t.TempDir()
	otro := &harness{
		t:           h.t,
		root:        filepath.Join(base, "home"),
		destino:     filepath.Join(base, "destino"),
		stateConfig: filepath.Join(base, "state-config.yaml"),
		staging:     filepath.Join(base, "staging"),
		projectDir:  filepath.Join(base, "project"),
		pipelineDir: h.pipelineDir,
		projectID:   fmt.Sprintf("%08d-1111-1111-1111-111111111111", id),
		projectURL:  fmt.Sprintf("%s/vex-test-%d/otro-proyecto", gitHost, id),
		pipelineURL: h.pipelineURL,
		execLog:     h.execLog,
	}

	require.NoError(h.t, os.MkdirAll(otro.root, 0o755))
	otro.prepararDestino()
	copyTree(h.t, h.projectDir, otro.projectDir)
	gitRepos.Store(endpointPath(otro.projectURL), otro.projectDir)

	return otro
}

// compartiendoElDestinoCon apunta este harness al destino de otro. Es LA
// operación que la spec 16 hace posible y que hasta ahora no tenía forma: dos
// máquinas, dos $HOME, y un solo sitio donde vive el estado.
func (h *harness) compartiendoElDestinoCon(otro *harness) {
	h.t.Helper()
	h.destino = otro.destino
	writeFile(h.t, h.stateConfig, "type: local\nlocal:\n  path: "+h.destino+"\n")
}

// conOtroPipeline devuelve un harness que despliega EL MISMO proyecto, en el
// mismo $HOME, con OTRO pipelinecode.
//
// Es lo que hace observable la decisión con la que D-A14 se cerró al revés
// (spec 11 §5.2): la clave de estado no lleva el pipeline, así que los dos
// escriben bajo la MISMA clave — el ACR pertenece al proyecto.
//
// Lo que decide si se reviven entre sí es su CONTENIDO, no su url: desde la
// spec 27 la url del pipelinecode salió de la huella, porque su contenido ya está
// dentro (§5.5). Sin `opts` los dos son idénticos byte a byte; con ellos se
// materializa un pipelinecode distinto de verdad.
func (h *harness) conOtroPipeline(opts ...harnessOption) *harness {
	h.t.Helper()

	id := nextHarnessID()
	otro := &harness{
		t:           h.t,
		root:        h.root,
		destino:     h.destino,
		stateConfig: h.stateConfig,
		staging:     h.staging,
		projectDir:  h.projectDir,
		pipelineDir: filepath.Join(h.t.TempDir(), "pipelinecode"),
		projectID:   h.projectID,
		projectURL:  h.projectURL,
		pipelineURL: fmt.Sprintf("%s/vex-test-%d/pipelinecode", gitHost, id),
		execLog:     h.execLog,
	}

	copyTree(h.t, filepath.Join("testdata", "pipelinecode"), otro.pipelineDir)
	for _, opt := range opts {
		opt(otro)
	}
	initRepo(h.t, otro.pipelineDir, "feat: otro pipelinecode")
	gitRepos.Store(endpointPath(otro.pipelineURL), otro.pipelineDir)

	return otro
}

// ── Utilidades de disco y git ───────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}))
}

// initRepo inicializa el repo y deja HEAD en la rama `fixtureRef`. El nombre de
// la rama se fija a mano porque go-git inicializa en "master" y las URLs del
// fixture piden "main".
func initRepo(t *testing.T, dir, message string) {
	t.Helper()
	repo, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)

	hash := commit(t, repo, message)
	require.NoError(t, repo.Storer.SetReference(
		plumbing.NewHashReference(plumbing.NewBranchReferenceName(fixtureRef), hash)))
	require.NoError(t, repo.Storer.SetReference(
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(fixtureRef))))
}

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	repo, err := gogit.PlainOpen(dir)
	require.NoError(t, err)
	commit(t, repo, message)
}

func commit(t *testing.T, repo *gogit.Repository, message string) plumbing.Hash {
	t.Helper()
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, worktree.AddWithOptions(&gogit.AddOptions{All: true}))

	hash, err := worktree.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "harness",
			Email: "harness@vex.test",
			// Fecha fija: nada de lo que el harness observa depende de ella y
			// así dos corridas producen los mismos objetos git.
			When: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	return hash
}
