package step_test

// CONTRACT TEST del puerto domStep.VarsStoreRepository (spec 02 §5.3').
//
// La misma suite corre contra las DOS implementaciones del puerto: la de
// archivo y la de Supabase (contra un servidor HTTP de prueba). El defecto que
// motiva el test es una violación de sustitución de Liskov: el adaptador de
// archivo perdía `isShared` y el de Supabase lo derivaba del scope, así que el
// mismo proyecto sobre el mismo árbol producía una huella distinta según dónde
// corriera.
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
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const (
	projectUrl  = "https://vex.test/org/proyecto.git"
	pipelineUrl = "https://vex.test/org/pipeline.git"
	ambiente    = "prod"
)

type implementacion struct {
	nombre    string
	construir func(t *testing.T) domStep.VarsStoreRepository
}

var implementaciones = []implementacion{
	{
		nombre: "archivo",
		construir: func(t *testing.T) domStep.VarsStoreRepository {
			return stepInfra.NewFileVarsStoreRepository(t.TempDir())
		},
	},
	{
		nombre: "supabase",
		construir: func(t *testing.T) domStep.VarsStoreRepository {
			servidor := httptest.NewServer(nuevaEdgeFunctionDeVariables())
			t.Cleanup(servidor.Close)
			return stepInfra.NewSupabaseVarsStoreRepository(servidor.URL, "token-de-prueba", "exec-1")
		},
	},
}

func TestVarsStoreRepository_Contrato(t *testing.T) {
	for _, impl := range implementaciones {
		t.Run(impl.nombre, func(t *testing.T) {
			t.Run("un ámbito nunca escrito está vacío", func(t *testing.T) {
				repo := impl.construir(t)
				vars, err := repo.Get(ctx(), projectUrl, pipelineUrl, ambiente, "supply")
				require.NoError(t, err)
				require.Empty(t, vars)
			})

			// EL caso de la spec: isShared tiene que sobrevivir al viaje, porque
			// es material de la huella de variables.
			t.Run("el ámbito compartido devuelve isShared=true", func(t *testing.T) {
				repo := impl.construir(t)
				guardar(t, repo, command.SharedScopeName, "supply", variable(t, "artifact_url", "s3://x", true))

				vars := obtener(t, repo, command.SharedScopeName, "supply")
				require.Len(t, vars, 1)
				require.Equal(t, "artifact_url", vars[0].Name())
				require.Equal(t, "s3://x", vars[0].Value())
				require.True(t, vars[0].IsShared(), "isShared se pierde: la huella dependería del adaptador")
			})

			t.Run("un ámbito de ambiente devuelve isShared=false", func(t *testing.T) {
				repo := impl.construir(t)
				guardar(t, repo, ambiente, "deploy", variable(t, "arn", "arn:aws:x", false))

				vars := obtener(t, repo, ambiente, "deploy")
				require.Len(t, vars, 1)
				require.False(t, vars[0].IsShared())
			})

			// Save persiste el conjunto completo del ámbito: guardar cero es
			// decir «este ámbito ya no tiene variables», no un no-op.
			t.Run("guardar una lista vacía deja el ámbito sin variables", func(t *testing.T) {
				repo := impl.construir(t)
				guardar(t, repo, ambiente, "package",
					variable(t, "image", "vex:1", false),
					variable(t, "digest", "sha256:ab", false))
				require.Len(t, obtener(t, repo, ambiente, "package"), 2)

				guardar(t, repo, ambiente, "package")
				require.Empty(t, obtener(t, repo, ambiente, "package"),
					"las variables viejas se siguen cargando en cada ejecución")
			})

			t.Run("guardar reemplaza el conjunto, no lo acumula", func(t *testing.T) {
				repo := impl.construir(t)
				guardar(t, repo, ambiente, "test", variable(t, "vieja", "1", false))
				guardar(t, repo, ambiente, "test", variable(t, "nueva", "2", false))

				vars := obtener(t, repo, ambiente, "test")
				require.Len(t, vars, 1)
				require.Equal(t, "nueva", vars[0].Name())
			})

			// De esta igualdad depende que StepExecutable.saveScopeVars deje de
			// guardar en cada corrida: compara lo leído con lo acumulado usando
			// reflect.DeepEqual sobre command.Variable, campos no exportados
			// incluidos. Un adaptador que pierda uno hace que la comparación dé
			// siempre distinto (spec 02 §5.2).
			t.Run("el round-trip devuelve la variable idéntica", func(t *testing.T) {
				repo := impl.construir(t)
				original := variable(t, "artifact_url", "s3://x", true)
				guardar(t, repo, command.SharedScopeName, "supply", original)

				recuperada := obtener(t, repo, command.SharedScopeName, "supply")
				require.True(t, reflect.DeepEqual([]command.Variable{original}, recuperada),
					"el Save del ámbito compartido se repetiría en cada ejecución")
			})

			t.Run("los ámbitos y los steps no se mezclan", func(t *testing.T) {
				repo := impl.construir(t)
				guardar(t, repo, ambiente, "supply", variable(t, "de_ambiente", "1", false))
				guardar(t, repo, command.SharedScopeName, "supply", variable(t, "de_shared", "2", true))
				guardar(t, repo, ambiente, "deploy", variable(t, "de_otro_step", "3", false))

				require.Equal(t, []string{"de_ambiente"}, nombres(obtener(t, repo, ambiente, "supply")))
				require.Equal(t, []string{"de_shared"}, nombres(obtener(t, repo, command.SharedScopeName, "supply")))
				require.Equal(t, []string{"de_otro_step"}, nombres(obtener(t, repo, ambiente, "deploy")))
			})
		})
	}
}

// ── Casos propios del adaptador de archivo ──────────────────────────────────

// Un archivo de cero bytes solo puede venir de una corrupción: un ámbito sin
// variables se escribe como una lista gob vacía, que no son cero bytes. Tratar
// io.EOF como «almacén vacío» convertía la corrupción en silencio.
func TestFileVarsStore_ArchivoTruncadoEsUnError(t *testing.T) {
	raiz := t.TempDir()
	repo := stepInfra.NewFileVarsStoreRepository(raiz)
	guardar(t, repo, ambiente, "supply", variable(t, "arn", "arn:aws:x", false))

	ruta := filepath.Join(directorioDelAlmacen(raiz, ambiente), "supply.vars")
	require.NoError(t, os.WriteFile(ruta, nil, 0644))

	_, err := repo.Get(ctx(), projectUrl, pipelineUrl, ambiente, "supply")
	require.Error(t, err, "un archivo truncado no puede leerse como almacén vacío")
	require.Contains(t, err.Error(), "truncado")
}

func TestFileVarsStore_LaEscrituraEsAtomica(t *testing.T) {
	raiz := t.TempDir()
	repo := stepInfra.NewFileVarsStoreRepository(raiz)
	guardar(t, repo, ambiente, "supply", variable(t, "arn", "arn:aws:x", false))

	entradas, err := os.ReadDir(directorioDelAlmacen(raiz, ambiente))
	require.NoError(t, err)
	require.Len(t, entradas, 1, "no puede quedar ningún temporal")
	require.Equal(t, "supply.vars", entradas[0].Name())
}

// directorioDelAlmacen reproduce el esquema de rutas de
// FileVarsStoreRepository: <raiz>/<proyecto>/store/<pipeline>/<scope>/.
func directorioDelAlmacen(raiz, scope string) string {
	return filepath.Join(
		raiz,
		utils.GetDirNameFromUrl(projectUrl),
		"store",
		utils.GetDirNameFromUrl(pipelineUrl),
		scope,
	)
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
// (scope, step). Igual que la función real, el flag isShared no viaja.
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

func variable(t *testing.T, nombre, valor string, compartida bool) command.Variable {
	t.Helper()
	v, err := command.NewVariable(nombre, valor, compartida)
	require.NoError(t, err)
	return v
}

func guardar(t *testing.T, repo domStep.VarsStoreRepository, scope, step string, vars ...command.Variable) {
	t.Helper()
	require.NoError(t, repo.Save(ctx(), projectUrl, pipelineUrl, scope, step, vars))
}

func obtener(t *testing.T, repo domStep.VarsStoreRepository, scope, step string) []command.Variable {
	t.Helper()
	vars, err := repo.Get(ctx(), projectUrl, pipelineUrl, scope, step)
	require.NoError(t, err)
	return vars
}

func nombres(vars []command.Variable) []string {
	out := make([]string, 0, len(vars))
	for i := range vars {
		out = append(out, vars[i].Name())
	}
	return out
}
