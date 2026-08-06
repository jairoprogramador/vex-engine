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
	"encoding/gob"
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
	infraCache "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
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

	// root es la raíz de almacenamiento inyectada: hace de $HOME. Todo lo que
	// el motor persiste cuelga de root/.vex/.
	root string

	// projectDir es el proyecto a desplegar: en modo local el motor lo enlaza
	// en vez de clonarlo, pero necesita un repo git con HEAD para el versionado
	// y para ${var.project_revision}.
	projectDir string

	// pipelineDir es el repo fuente del pipelinecode; el motor lo clona.
	pipelineDir string

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

func newHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	installGitTransport()

	id := nextHarnessID()
	base := t.TempDir()

	h := &harness{
		t:           t,
		root:        filepath.Join(base, "home"),
		projectDir:  filepath.Join(base, "project"),
		pipelineDir: filepath.Join(base, "pipelinecode"),
		projectURL:  fmt.Sprintf("%s/vex-test-%d/demo-app", gitHost, id),
		pipelineURL: fmt.Sprintf("%s/vex-test-%d/pipelinecode", gitHost, id),
		execLog:     filepath.Join(base, "exec.log"),
	}

	require.NoError(t, os.MkdirAll(h.root, 0o755))
	writeFile(t, h.execLog, "")
	t.Setenv("VEX_TEST_LOG", h.execLog)
	// El input siempre llega por --input o por stdin salvo en el caso que la
	// prueba explícitamente; con la env var a "" readInput la ignora.
	t.Setenv("VEX_REQUEST_INPUT", "")

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

func (h *harness) request(opts ...requestOption) dto.RequestInput {
	request := dto.RequestInput{
		SchemaVersion: 1,
		Project: dto.ProjectInput{
			Id:   "11111111-1111-1111-1111-111111111111",
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

func (h *harness) args() cli.RunArgs {
	// Modo local: es el que cablea los repositorios de archivo. Quiet suprime
	// el observer de stdout, que escribe en os.Stdout del proceso y no en el
	// writer que se le pasa a Execute.
	return cli.RunArgs{Mode: cli.ModeLocal, Quiet: true}
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

// executeCtx es el único punto que construye el motor: mismo cableado que el
// binario, con las rutas del fixture en lugar de $HOME y /appProject.
//
// El contexto se recibe para poder cancelarlo: es lo que hace el manejador de
// señales de cmd/vexd al recibir un SIGINT (spec 07 §5.4), y lo único de esa
// ruta que no depende de mandarle una señal de verdad al proceso de test.
func (h *harness) executeCtx(ctx context.Context, args cli.RunArgs, stdin io.Reader) runResult {
	h.t.Helper()

	runCmd, err := cli.BuildRunCommand(cli.EngineConfig{
		RootVexPath:      h.root,
		LocalProjectPath: h.projectDir,
	}, args)
	require.NoError(h.t, err)

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

// storedVars lee el almacén de variables con el mismo repositorio de archivo
// que usa el motor.
func (h *harness) storedVars(scope, step string) map[string]string {
	h.t.Helper()
	repo := stepInfra.NewFileVarsStoreRepository(filepath.Join(h.root, cli.VexHomeDirName, "projects"))
	ctx := context.Background()
	vars, err := repo.Get(&ctx, h.projectURL, h.pipelineURL, scope, step)
	require.NoError(h.t, err)

	out := make(map[string]string, len(vars))
	for _, v := range vars {
		out[v.Name()] = v.Value()
	}
	return out
}

// persistedStepState devuelve la ruta relativa de todo archivo del ALMACÉN DE
// VARIABLES que el motor haya escrito para un step (`<step>.vars`).
//
// Hasta la spec 10 encontraba además las tres huellas de la policy
// (`inst<step>.status`, `code<step>.status`, `<step>.status`), que llevaban el
// nombre del paso en el nombre del archivo. Ya no existen: la entrada de caché
// está direccionada por CONTENIDO, así que el paso va dentro del hash y no en la
// ruta. Lo que la sustituye como observación es `cacheEntries`.
//
// Sigue siendo la mitad de la observación de «no persiste estado de
// re-ejecución» de la spec 04 §5.3: un step saltado por falta de comandos no
// deja rastro, así que la corrida siguiente vuelve a saltarlo por la misma razón
// y no por caché.
func (h *harness) persistedStepState(step string) []string {
	h.t.Helper()

	projects := filepath.Join(h.root, cli.VexHomeDirName, "projects")
	found := make([]string, 0, 4)
	err := filepath.WalkDir(projects, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.Contains(d.Name(), step) {
			return nil
		}
		// El pipelinecode copiado al workdir también menciona el step; lo que se
		// busca aquí es estado persistido, que siempre es .vars.
		if filepath.Ext(path) != ".vars" {
			return nil
		}
		rel, err := filepath.Rel(projects, path)
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

// cacheEntries devuelve las claves de las entradas de caché escritas, tal como
// cada archivo se identifica a sí mismo, ordenadas.
//
// Se lee la clave de DENTRO del archivo y no de su ruta a propósito: la ruta es
// un detalle del almacén, la clave es el contrato. Y como está direccionada por
// contenido, dos ejecuciones que dan la misma clave producen un solo archivo
// —que es la mitad de lo que la spec 10 promete—, mientras que dos estados
// distintos conviven.
func (h *harness) cacheEntries() []string {
	h.t.Helper()

	base := filepath.Join(h.root, cli.VexHomeDirName, "cache")
	claves := make([]string, 0, 4)
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
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
		claves = append(claves, dto.CacheKey)
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

	patron := filepath.Join(h.root, cli.VexHomeDirName, "projects", "*", "workdirs", "*", fixtureEnvironment, relPath)
	matches, err := filepath.Glob(patron)
	require.NoError(h.t, err)
	require.Len(h.t, matches, 1, "se esperaba exactamente un workdir con %s", relPath)

	data, err := os.ReadFile(matches[0])
	require.NoError(h.t, err)
	return string(data)
}

// assertNingunaVariableAnonima es el invariante GLOBAL de la spec 03: ninguna
// variable acumulada tiene nombre vacío. Se comprueba tras CADA ejecución del
// harness, no en un caso suelto, porque lo que se afirma no es que un input
// concreto esté limpio sino que ninguna ejecución puede producir la entrada
// anónima.
//
// El almacén es el único sitio donde el mapa acumulado sobrevive a la
// ejecución, así que es donde se observa. Se lee decodificando el gob a mano en
// vez de por el repositorio: `NewVariable` rechaza el nombre vacío, así que
// pasar por él convertiría la entrada anónima en un error de lectura en vez de
// en la aserción que se quiere leer al fallar.
func (h *harness) assertNingunaVariableAnonima() {
	h.t.Helper()

	projects := filepath.Join(h.root, cli.VexHomeDirName, "projects")
	err := filepath.WalkDir(projects, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".vars" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		var stored []stepInfra.FileVarStoreDTO
		if err := gob.NewDecoder(file).Decode(&stored); err != nil {
			return fmt.Errorf("decodificar %s: %w", path, err)
		}
		for _, variable := range stored {
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
// pipelinecode desde otra máquina: otro $HOME y otra ruta absoluta para el árbol
// del proyecto, con las mismas urls.
//
// Es lo que permite observar la propiedad que hace que el caché compartido de la
// spec 16 signifique algo: la clave no depende de dónde estén los archivos. El
// árbol se copia byte a byte, incluido su `.git`, para que la versión y la
// revisión del proyecto salgan idénticas.
func (h *harness) otraMaquina() *harness {
	h.t.Helper()

	base := h.t.TempDir()
	otro := &harness{
		t:           h.t,
		root:        filepath.Join(base, "home"),
		projectDir:  filepath.Join(base, "project"),
		pipelineDir: h.pipelineDir,
		projectURL:  h.projectURL,
		pipelineURL: h.pipelineURL,
		execLog:     h.execLog,
	}

	require.NoError(h.t, os.MkdirAll(otro.root, 0o755))
	copyTree(h.t, h.projectDir, otro.projectDir)

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
