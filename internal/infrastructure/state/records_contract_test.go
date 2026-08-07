package state_test

// CONTRACT TEST del puerto domState.Records, heredero del de
// `VarsStoreRepository` (spec 02 §5.3').
//
// La misma suite corre contra las DOS implementaciones: la de archivo y la de
// Supabase (contra un servidor HTTP de prueba). El defecto que lo motivó era una
// violación de sustitución de Liskov: el adaptador de archivo perdía `isShared`
// y el de Supabase lo derivaba del ámbito, así que el mismo proyecto sobre el
// mismo árbol producía una huella distinta según dónde corriera.
//
// Desde la spec 13 §5.6 ESE VIAJE YA NO EXISTE: el ámbito es del step, así que
// una variable no lo lleva y no hay nada que un adaptador pueda perder. Los dos
// casos que lo medían —«el ámbito de proyecto devuelve isShared=true» y su
// pareja— se retiran aquí, y su desaparición es parte del entregable. Lo que
// queda mide lo que sigue siendo verdad, empezando por que los ámbitos no se
// mezclan: eso lo garantiza ahora la CLAVE, que es donde debía estar.
//
// Lo que el contrato NO exige es historia: el adaptador de Supabase guarda el
// último conjunto por (ámbito, step) y lo dice devolviendo registros SIN
// ATRIBUIR. Las propiedades del append-only se prueban donde existen, sobre el
// adaptador de archivo, más abajo.
//
// Cualquier tercer adaptador (spec 21) se añade a la tabla `implementaciones` y
// tiene que pasar tal cual.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const (
	projectUrl = "https://vex.test/org/proyecto.git"
	ambiente   = "prod"
)

var instante = time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

type implementacion struct {
	nombre    string
	construir func(t *testing.T) domState.Records
}

var implementaciones = []implementacion{
	{
		nombre: "archivo",
		construir: func(t *testing.T) domState.Records {
			return stateInfra.NewFileRecordsRepository(t.TempDir())
		},
	},
	{
		nombre: "supabase",
		construir: func(t *testing.T) domState.Records {
			servidor := httptest.NewServer(nuevaEdgeFunctionDeVariables())
			t.Cleanup(servidor.Close)
			return stateInfra.NewSupabaseRecordsRepository(servidor.URL, "token-de-prueba", "exec-1")
		},
	},
}

func TestRecords_Contrato(t *testing.T) {
	for _, impl := range implementaciones {
		t.Run(impl.nombre, func(t *testing.T) {
			t.Run("una clave que nadie escribió NO consta, y no es un error", func(t *testing.T) {
				// Es la mitad que sostiene todo lo demás: ausencia de registro ⇒
				// ejecutar. Si esto devolviera error, el motor confundiría «este
				// step nunca corrió aquí» con «no pude averiguarlo».
				repo := impl.construir(t)

				_, existe, err := repo.Last(ctx(), claveDeAmbiente(t, "02-supply"))

				require.NoError(t, err)
				assert.False(t, existe)
			})

			// Lo que la spec 02 pedía que sobreviviera al viaje, sin el campo que
			// ya no viaja: el par (nombre, valor) llega intacto por los dos
			// adaptadores.
			t.Run("el par nombre/valor sobrevive al viaje", func(t *testing.T) {
				repo := impl.construir(t)
				clave := claveDeProyecto(t, "02-supply")
				anadir(t, repo, clave, variable(t, "artifact_url", "s3://x"))

				vars := ultimasVariables(t, repo, clave)
				require.Len(t, vars, 1)
				assert.Equal(t, "artifact_url", vars[0].Name())
				assert.Equal(t, "s3://x", vars[0].Value())
			})

			t.Run("el último registro es el que se lee", func(t *testing.T) {
				repo := impl.construir(t)
				clave := claveDeAmbiente(t, "01-test")

				anadirEn(t, repo, clave, instante, variable(t, "vieja", "1"))
				anadirEn(t, repo, clave, instante.Add(time.Second), variable(t, "nueva", "2"))

				vars := ultimasVariables(t, repo, clave)
				require.Len(t, vars, 1)
				assert.Equal(t, "nueva", vars[0].Name(),
					"el estado vigente es el del último registro, no la unión de todos")
			})

			t.Run("un registro sin variables deja el ámbito sin variables", func(t *testing.T) {
				// Es la forma de decir «este ámbito ya no tiene variables», que en
				// el almacén viejo obligaba a guardar una lista vacía sobre la
				// anterior. Aquí no se borra nada: se añade un hecho nuevo.
				repo := impl.construir(t)
				clave := claveDeAmbiente(t, "03-package")

				anadirEn(t, repo, clave, instante,
					variable(t, "image", "vex:1"),
					variable(t, "digest", "sha256:ab"))
				require.Len(t, ultimasVariables(t, repo, clave), 2)

				anadirEn(t, repo, clave, instante.Add(time.Second))
				assert.Empty(t, ultimasVariables(t, repo, clave),
					"las variables viejas se seguirían cargando en cada ejecución")
			})

			t.Run("los ámbitos y los steps no se mezclan", func(t *testing.T) {
				repo := impl.construir(t)
				deAmbiente := claveDeAmbiente(t, "02-supply")
				deProyecto := claveDeProyecto(t, "02-supply")
				deOtroStep := claveDeAmbiente(t, "04-deploy")

				anadir(t, repo, deAmbiente, variable(t, "de_ambiente", "1"))
				anadir(t, repo, deProyecto, variable(t, "de_proyecto", "2"))
				anadir(t, repo, deOtroStep, variable(t, "de_otro_step", "3"))

				assert.Equal(t, []string{"de_ambiente"}, nombres(ultimasVariables(t, repo, deAmbiente)))
				assert.Equal(t, []string{"de_proyecto"}, nombres(ultimasVariables(t, repo, deProyecto)))
				assert.Equal(t, []string{"de_otro_step"}, nombres(ultimasVariables(t, repo, deOtroStep)))
			})
		})
	}
}

// ── Casos propios del adaptador de archivo: el append-only ──────────────────

// LA prueba que da nombre a la spec: dos ejecuciones dejan DOS registros. Hasta
// la spec 11 la primera ya no estaba.
func TestFileRecords_NadaSeSobrescribe(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)
	clave := claveDeAmbiente(t, "02-supply")

	anadirEn(t, repo, clave, instante, variable(t, "acr_name", "acme1"))
	anadirEn(t, repo, clave, instante.Add(time.Second), variable(t, "acr_name", "acme2"))

	archivos := archivosDe(t, directorioDelRegistro(raiz, "environment", ambiente, "02-supply"))
	assert.Len(t, archivos, 2, "el contador de archivos bajo una clave sólo sube")

	// Y el de ayer se puede seguir leyendo tal como fue: es la precondición del
	// rollback de la spec 28.
	primero := leerRegistro(t, archivos[0])
	assert.Equal(t, "acme1", primero.Variables[0].Value)
}

// Un registro no se sobrescribe ni por error del llamador: el almacén lo impide
// en su borde, que es más fuerte que documentar que no se debe hacer.
func TestFileRecords_EscribirDosVecesElMismoIdEsUnError(t *testing.T) {
	repo := stateInfra.NewFileRecordsRepository(t.TempDir())
	clave := claveDeAmbiente(t, "02-supply")
	registro := registro(t, instante, "ck-v1:aaa")

	require.NoError(t, repo.Append(ctx(), clave, registro))
	assert.Error(t, repo.Append(ctx(), clave, registro))
}

// «El último registro se obtiene sin leer los otros»: basta el orden de los
// ULID. Se observa dejando ILEGIBLE un registro anterior — si `Last` lo abriera,
// esto fallaría.
func TestFileRecords_ElUltimoSeObtieneSinLeerLosOtros(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)
	clave := claveDeAmbiente(t, "02-supply")

	anadirEn(t, repo, clave, instante, variable(t, "acr_name", "acme1"))
	anadirEn(t, repo, clave, instante.Add(time.Second), variable(t, "acr_name", "acme2"))

	archivos := archivosDe(t, directorioDelRegistro(raiz, "environment", ambiente, "02-supply"))
	require.Len(t, archivos, 2)
	require.NoError(t, os.WriteFile(archivos[0], []byte("{no soy json"), 0o644))

	vars := ultimasVariables(t, repo, clave)
	require.Len(t, vars, 1)
	assert.Equal(t, "acme2", vars[0].Value())
}

// El registro ILEGIBLE es un error, no una ausencia (spec 11 §5.6). Es la
// asimetría con el índice, donde ilegible ⇒ ausente: aquí la duda no se puede
// resolver ejecutando sin arriesgar un recurso duplicado.
func TestFileRecords_UnRegistroIlegibleEsUnError(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)
	clave := claveDeAmbiente(t, "02-supply")
	anadir(t, repo, clave, variable(t, "arn", "arn:aws:x"))

	archivos := archivosDe(t, directorioDelRegistro(raiz, "environment", ambiente, "02-supply"))
	require.Len(t, archivos, 1)

	t.Run("truncado", func(t *testing.T) {
		require.NoError(t, os.WriteFile(archivos[0], nil, 0o644))
		_, _, err := repo.Last(ctx(), clave)
		assert.Error(t, err, "un archivo truncado no puede leerse como almacén vacío")
	})

	t.Run("con una variable que el dominio rechaza", func(t *testing.T) {
		require.NoError(t, os.WriteFile(archivos[0], []byte(`{
  "schema_version": 1,
  "record_id": "01KZBF3MG0000G40R40M30E209",
  "step_fingerprint": "ck-v1:aaa",
  "variables": [{"name": "", "value": "x"}],
  "produced_by": {"execution_id": "exec-1", "at": "2026-08-06T12:00:00Z"}
}`), 0o644))
		_, _, err := repo.Last(ctx(), clave)
		assert.Error(t, err, "el invariante de Variable se aplica también al leer")
	})
}

// El archivo se explica solo y está donde la spec 11 §5.4 dice, con el ámbito
// en DOS segmentos: `:` es ilegal en rutas de Windows.
func TestFileRecords_LaRutaYLaFormaEnDisco(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)

	require.NoError(t, repo.Append(ctx(), claveDeAmbiente(t, "02-supply"),
		registro(t, instante, "ck-v1:aaa", variable(t, "acr_name", "acme.azurecr.io"))))

	archivos := archivosDe(t, directorioDelRegistro(raiz, "environment", ambiente, "02-supply"))
	require.Len(t, archivos, 1)
	assert.Equal(t, "01KZBF3MG0000G40R40M30E209.json", filepath.Base(archivos[0]))

	dto := leerRegistro(t, archivos[0])
	assert.Equal(t, 2, dto.SchemaVersion,
		"sube a 2 con la spec 13: `variables[].shared` desaparece de la forma del archivo")
	assert.Equal(t, "ck-v1:aaa", dto.StepFingerprint)
	assert.Equal(t, "exec-1", dto.ProducedBy.ExecutionID)
	assert.Equal(t, "acr_name", dto.Variables[0].Name)
	assert.Equal(t, "acme.azurecr.io", dto.Variables[0].Value)

	// Y el campo se fue del JSON, no sólo del DTO: un registro nuevo no lo lleva.
	crudo, err := os.ReadFile(archivos[0])
	require.NoError(t, err)
	assert.NotContains(t, string(crudo), `"shared"`)
}

// LA DECISIÓN QUE LA SPEC 13 EXIGE TOMAR EXPLÍCITAMENTE, hecha ejecutable: este
// binario LEE los registros v1 en vez de rechazarlos.
//
// No es compatibilidad por cortesía. `state/` es la verdad y no se borra nunca,
// así que negarse a leerlo dejaría huérfanos los identificadores de recursos que
// existen de verdad en la nube y el motor volvería a crearlos. Se puede leer sin
// pérdida porque lo que se quitó es un campo y el lector ya no tiene dónde
// ponerlo: el ámbito lo dice la CLAVE bajo la que está el registro.
func TestFileRecords_UnRegistroV1SeSigueLeyendo(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)
	clave := claveDeProyecto(t, "02-supply")

	dir := directorioDelRegistro(raiz, "project", "02-supply")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "01KZBF3MG0000G40R40M30E209.json"), []byte(`{
  "schema_version": 1,
  "record_id": "01KZBF3MG0000G40R40M30E209",
  "step_fingerprint": "ck-v1:aaa",
  "variables": [{"name": "artifact_url", "value": "s3://x", "shared": true}],
  "produced_by": {"execution_id": "exec-viejo", "at": "2026-08-06T12:00:00Z"}
}`), 0o644))

	record, existe, err := repo.Last(ctx(), clave)

	require.NoError(t, err, "un ARN escrito por el binario anterior no puede volverse ilegible")
	require.True(t, existe)
	vars := record.Variables()
	require.Len(t, vars, 1)
	assert.Equal(t, "artifact_url", vars[0].Name())
	assert.Equal(t, "s3://x", vars[0].Value())

	// Una versión DESCONOCIDA sigue siendo un error: una versión conocida no es
	// un registro ilegible, y esa asimetría (spec 11 §5.6) no se afloja.
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "01KZBF3MG1000G40R40M30E209.json"), []byte(`{
  "schema_version": 99,
  "record_id": "01KZBF3MG1000G40R40M30E209",
  "step_fingerprint": "ck-v1:aaa",
  "variables": [],
  "produced_by": {"execution_id": "exec-1", "at": "2026-08-06T12:00:00Z"}
}`), 0o644))

	_, _, err = repo.Last(ctx(), clave)
	assert.Error(t, err)
}

func TestFileRecords_LaEscrituraEsAtomica(t *testing.T) {
	raiz := t.TempDir()
	repo := stateInfra.NewFileRecordsRepository(raiz)
	anadir(t, repo, claveDeAmbiente(t, "02-supply"), variable(t, "arn", "arn:aws:x"))

	entradas, err := os.ReadDir(directorioDelRegistro(raiz, "environment", ambiente, "02-supply"))
	require.NoError(t, err)
	require.Len(t, entradas, 1, "no puede quedar ningún temporal")
}

// ── Fake de la edge function store-vars ─────────────────────────────────────

type varDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type peticionStoreVars struct {
	ExecutionID string   `json:"execution_id"`
	Scope       string   `json:"scope"`
	StepName    string   `json:"step_name"`
	Operation   string   `json:"operation"`
	Variables   []varDTO `json:"variables"`
}

// nuevaEdgeFunctionDeVariables reproduce el contrato que el adaptador Supabase
// espera: get devuelve {"variables": [...]}, set reemplaza el conjunto del par
// (scope, step). Igual que la función real, el flag isShared no viaja y no hay
// historia.
func nuevaEdgeFunctionDeVariables() http.Handler {
	almacen := map[string][]varDTO{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var peticion peticionStoreVars
		if err := json.NewDecoder(r.Body).Decode(&peticion); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		clave := peticion.ExecutionID + "|" + peticion.Scope + "|" + peticion.StepName

		switch peticion.Operation {
		case "set":
			almacen[clave] = peticion.Variables
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "get":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"variables": almacen[clave]})
		default:
			http.Error(w, "operación desconocida: "+peticion.Operation, http.StatusBadRequest)
		}
	})
}

// ── Ayudas ──────────────────────────────────────────────────────────────────

func ctx() *context.Context {
	c := context.Background()
	return &c
}

func claveDeAmbiente(t *testing.T, stepID string) domState.Key {
	t.Helper()
	scope, err := domState.NewEnvironmentScope(ambiente)
	require.NoError(t, err)
	key, err := domState.NewKey(projectUrl, scope, stepID)
	require.NoError(t, err)
	return key
}

func claveDeProyecto(t *testing.T, stepID string) domState.Key {
	t.Helper()
	key, err := domState.NewKey(projectUrl, domState.NewProjectScope(), stepID)
	require.NoError(t, err)
	return key
}

func variable(t *testing.T, nombre, valor string) command.Variable {
	t.Helper()
	// El origen no viaja al registro y vuelve siempre como `OriginState`: lo que
	// se guardó fue un valor (spec 12 §5.1). Aquí se construyen como producidas
	// porque es lo que un step deja en el mapa acumulado.
	v, err := command.NewVariable(nombre, valor, command.OriginRuntime)
	require.NoError(t, err)
	return v
}

// registro compone uno con el ULID derivado del instante: dos instantes
// distintos dan dos identificadores ordenados, que es lo que los tests
// necesitan fijar sin depender de la aleatoriedad.
func registro(t *testing.T, at time.Time, huella string, vars ...command.Variable) domState.StepRecord {
	t.Helper()
	id, err := domState.NewRecordID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)
	r, err := domState.NewStepRecord(id, huella, vars,
		domState.Provenance{ExecutionID: "exec-1", At: at})
	require.NoError(t, err)
	return r
}

func anadir(t *testing.T, repo domState.Records, key domState.Key, vars ...command.Variable) {
	t.Helper()
	anadirEn(t, repo, key, instante, vars...)
}

func anadirEn(t *testing.T, repo domState.Records, key domState.Key, at time.Time, vars ...command.Variable) {
	t.Helper()
	require.NoError(t, repo.Append(ctx(), key, registro(t, at, "ck-v1:aaa", vars...)))
}

func ultimasVariables(t *testing.T, repo domState.Records, key domState.Key) []command.Variable {
	t.Helper()
	record, found, err := repo.Last(ctx(), key)
	require.NoError(t, err)
	if !found {
		return nil
	}
	return record.Variables()
}

func nombres(vars []command.Variable) []string {
	out := make([]string, 0, len(vars))
	for i := range vars {
		out = append(out, vars[i].Name())
	}
	return out
}

// directorioDelRegistro reproduce el esquema de rutas de
// FileRecordsRepository: <raiz>/<proyecto>/<ámbito…>/<step_id>/.
func directorioDelRegistro(raiz string, scope ...string) string {
	segmentos := append([]string{raiz, utils.GetDirNameFromUrl(projectUrl)}, scope...)
	return filepath.Join(segmentos...)
}

func archivosDe(t *testing.T, dir string) []string {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	require.NoError(t, err)
	encontrados := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		if !entrada.IsDir() {
			encontrados = append(encontrados, filepath.Join(dir, entrada.Name()))
		}
	}
	return encontrados
}

func leerRegistro(t *testing.T, path string) stateInfra.FileStepRecordDTO {
	t.Helper()
	datos, err := os.ReadFile(path)
	require.NoError(t, err)
	var dto stateInfra.FileStepRecordDTO
	require.NoError(t, json.Unmarshal(datos, &dto))
	return dto
}
