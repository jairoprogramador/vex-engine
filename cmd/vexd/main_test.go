package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Prueba de la raíz de composición entera: la línea de comandos, el borde y los seis contextos con sus
// adaptadores de verdad (almacén en disco, repositorios git, comandos del shell).

type invocacion struct {
	codigo          int
	salida, errores string
}

func invocar(t *testing.T, peticion string, args ...string) invocacion {
	t.Helper()
	var salida, errores bytes.Buffer
	codigo := ejecutar(context.Background(), args, strings.NewReader(peticion), &salida, &errores)
	return invocacion{codigo, salida.String(), errores.String()}
}

type entorno struct {
	banderas                   []string
	repoProyecto, repoPipeline string
}

func nuevoEntorno(t *testing.T) entorno {
	t.Helper()
	return nuevoEntornoConPipeline(t, func(string) {})
}

// nuevoEntornoConPipeline parte del pipeline de ejemplo y deja que la prueba lo cambie antes de commitearlo.
func nuevoEntornoConPipeline(t *testing.T, cambiar func(dir string)) entorno {
	t.Helper()
	almacen, espacio, material := t.TempDir(), t.TempDir(), t.TempDir()
	return entorno{
		banderas:     []string{"--almacen", almacen, "--espacio", espacio, "--material", material},
		repoProyecto: nuevoRepo(t, func(dir string) { escribir(t, dir, "README.md", "proyecto") }),
		repoPipeline: nuevoRepo(t, func(dir string) {
			copiar(t, pipelineDeEjemplo, dir)
			cambiar(dir)
		}),
	}
}

// pipelineDeEjemplo es el pipeline con el que se prueba; lo que las pruebas esperan de sus comandos sale de lo
// que él declara, no de un número escrito aquí.
var pipelineDeEjemplo = filepath.Join("..", "..", "internal", "ejecucion", "testdata", "ejemplo")

// comandosDeclarados son los nombres de los comandos que el pipeline de ejemplo declara para un paso (su
// carpeta en steps/), en el orden en que los declara.
func comandosDeclarados(t *testing.T, carpetaDelPaso string) []string {
	t.Helper()
	contenido, err := os.ReadFile(filepath.Join(pipelineDeEjemplo, "steps", carpetaDelPaso, "commands.yaml"))
	require.NoError(t, err)
	var comandos []struct {
		Name string `yaml:"name"`
	}
	require.NoError(t, yaml.Unmarshal(contenido, &comandos))
	nombres := make([]string, 0, len(comandos))
	for _, c := range comandos {
		nombres = append(nombres, c.Name)
	}
	return nombres
}

// nombresDe son los nombres de los comandos, en el orden en que aparecen.
func nombresDe(comandos []vistaDeLog) []string {
	nombres := make([]string, 0, len(comandos))
	for _, c := range comandos {
		nombres = append(nombres, c.Comando)
	}
	return nombres
}

func (e entorno) intento() string {
	return `{"Version":"1","Ambiente":"prod","Solicitante":"ana",
	  "FuenteDelProyecto":` + quote(e.repoProyecto) + `,"FuenteDelPipeline":` + quote(e.repoPipeline) + `,
	  "Metadatos":{"ProjectName":"vex-demo","ProjectId":"p1"}}`
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestIntentar_LlegaADespliegueYNoMuestraLaSalidaDeLosComandos(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var resultado struct {
		Intento, Estado, Despliegue string
		Detalle                     struct {
			Tiempo string
			Pasos  []struct{ Nombre, Estado string }
		}
	}
	require.NoError(t, json.Unmarshal([]byte(r.salida), &resultado), "la salida estándar es solo la respuesta")
	require.Equal(t, "exitoso", resultado.Estado)
	require.NotEmpty(t, resultado.Despliegue)
	require.NotEmpty(t, resultado.Detalle.Tiempo)
	require.NotEmpty(t, resultado.Detalle.Pasos, "el detalle lista los pasos que se dieron")
	for _, paso := range resultado.Detalle.Pasos {
		require.NotEmpty(t, paso.Nombre)
		require.Equal(t, "ejecutado", paso.Estado, "en el primer intento nada se precarga: %s", paso.Nombre)
	}
	require.Empty(t, r.errores, "lo que imprimen los comandos no se muestra: se consulta con logs")
	require.NotContains(t, r.salida, "etiqueta=v1.0.0", "ni se mezcla con la respuesta")
}

func logs(t *testing.T, e entorno, peticion string) invocacion {
	t.Helper()
	return invocar(t, peticion, append([]string{"logs"}, e.banderas...)...)
}

// respuestaDeLogs es la respuesta de logs leída como JSON.
type respuestaDeLogs struct {
	IntentoId string
	Salidas   map[string][]vistaDeLog
}

func leerLogs(t *testing.T, r invocacion) respuestaDeLogs {
	t.Helper()
	require.Equal(t, salidaBien, r.codigo, r.errores)
	var logs respuestaDeLogs
	require.NoError(t, json.Unmarshal([]byte(r.salida), &logs), "la respuesta de logs es JSON")
	require.NotEmpty(t, logs.IntentoId)
	return logs
}

// comandosDe son todos los comandos de la respuesta, de todos los pasos.
func (l respuestaDeLogs) comandosDe() []vistaDeLog {
	var todos []vistaDeLog
	for _, comandos := range l.Salidas {
		todos = append(todos, comandos...)
	}
	return todos
}

func TestLogs_SinIntentoMuestraLosComandosDelUltimoConSuSalida(t *testing.T) {
	e := nuevoEntorno(t)
	intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	require.Equal(t, salidaBien, intentar.codigo, intentar.errores)

	r := logs(t, e, `{"Version":"1"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.Empty(t, r.errores)
	logs := leerLogs(t, r)
	require.Equal(t, vistaDeLog{"comando-test-01", "hola vex-demo", "exitoso"}, logs.Salidas["test"][0], "en el orden en que corrieron")
	require.Equal(t, vistaDeLog{"comando-test-02", "etiqueta=v1.0.0", "exitoso"}, logs.Salidas["test"][1])
	require.Equal(t, comandosDeclarados(t, "01-test"), nombresDe(logs.Salidas["test"]), "los que el paso declara")
	require.Equal(t, vistaDeLog{"comando-deploy-01", "despliegue exitoso", "exitoso"}, logs.Salidas["deploy"][0])
	require.NotContains(t, r.salida, "${var.", "las variables se interpolaron")
	require.Less(t, strings.Index(r.salida, `"test"`), strings.Index(r.salida, `"acr"`), "los pasos, en el orden en que corrieron")
	require.Less(t, strings.Index(r.salida, `"acr"`), strings.Index(r.salida, `"deploy"`))
}

func TestLogs_ConIntentoMuestraLosDeEseIntento(t *testing.T) {
	e := nuevoEntorno(t)
	intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	var resultado struct{ Intento string }
	require.NoError(t, json.Unmarshal([]byte(intentar.salida), &resultado))

	r := logs(t, e, `{"Version":"1","Intento":`+quote(resultado.Intento)+`}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	logs := leerLogs(t, r)
	require.Equal(t, resultado.Intento, logs.IntentoId)
	require.Equal(t, "comando-test-01", logs.Salidas["test"][0].Comando)
}

func TestLogs_SinIntentoDiceCualEsElUltimo(t *testing.T) {
	e := nuevoEntorno(t)
	var ultimo struct{ Intento string }
	for range 2 {
		intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
		require.NoError(t, json.Unmarshal([]byte(intentar.salida), &ultimo))
	}

	logs := leerLogs(t, logs(t, e, `{"Version":"1"}`))

	require.Equal(t, ultimo.Intento, logs.IntentoId)
}

func TestLogs_SoloMuestraLosPasosHastaDondeLlegoElIntento(t *testing.T) {
	e := nuevoEntorno(t)
	hasta := strings.Replace(e.intento(), `"Ambiente":"prod",`, `"Ambiente":"prod","HastaPaso":"test",`, 1)
	require.Equal(t, salidaBien, invocar(t, hasta, append([]string{"intentar"}, e.banderas...)...).codigo)

	logs := leerLogs(t, logs(t, e, `{"Version":"1"}`))

	require.Len(t, logs.Salidas, 1)
	require.Equal(t, comandosDeclarados(t, "01-test"), nombresDe(logs.Salidas["test"]), "los que el paso declara")
}

func TestLogs_UnIntentoQueNoExisteFalla(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1","Intento":"no-existe"}`)

	require.Equal(t, salidaFallo, r.codigo)
	require.Empty(t, r.salida)
}

func TestLogs_SinNingunIntentoRespondeUnMensajeSinFallar(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.JSONEq(t, `{"Mensaje":`+quote(mensajeLogsSinHistorial)+`}`, r.salida)
}

func TestLogs_ElResultadoFiltraLosComandosExitososOFallidos(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", `- name: comando-deploy-ok
  description: sale bien
  cmd: echo todo bien

- name: comando-deploy-roto
  description: sale mal
  cmd: echo explotó; exit 1
`)
	})
	intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	require.Equal(t, salidaFallo, intentar.codigo, intentar.errores)

	todos := logs(t, e, `{"Version":"1"}`)
	fallidos := logs(t, e, `{"Version":"1","Resultado":"fallido"}`)
	exitosos := logs(t, e, `{"Version":"1","Resultado":"exitoso"}`)

	ok := vistaDeLog{"comando-deploy-ok", "todo bien", "exitoso"}
	roto := vistaDeLog{"comando-deploy-roto", "explotó", "fallido"}
	require.Equal(t, []vistaDeLog{ok, roto}, leerLogs(t, todos).Salidas["deploy"])
	require.Equal(t, map[string][]vistaDeLog{"deploy": {roto}}, leerLogs(t, fallidos).Salidas)
	require.Contains(t, leerLogs(t, exitosos).comandosDe(), ok)
	require.NotContains(t, leerLogs(t, exitosos).comandosDe(), roto)
}

func TestLogs_UnResultadoDesconocidoEsUnaPeticionInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1","Resultado":"roto"}`)

	require.Equal(t, salidaInvalida, r.codigo, r.errores)
	require.Contains(t, r.errores, "Resultado")
}

func TestConsultas_VenLoQueIntentarDejoEnElHistorial(t *testing.T) {
	e := nuevoEntorno(t)
	intento := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	require.Equal(t, salidaBien, intento.codigo, intento.errores)

	despliegues := invocar(t, `{"Version":"1","Ambiente":"prod"}`, append([]string{"despliegues"}, e.banderas...)...)

	require.Equal(t, salidaBien, despliegues.codigo, despliegues.errores)
	var lista []map[string]any
	require.NoError(t, json.Unmarshal([]byte(despliegues.salida), &lista))
	require.Len(t, lista, 1)
}

func TestReservarYLiberar_ResponderUnObjetoVacio(t *testing.T) {
	e := nuevoEntorno(t)
	for _, op := range []string{"reservar", "liberar"} {
		r := invocar(t, `{"Version":"1","Ambiente":"prod"}`, append([]string{op}, e.banderas...)...)
		require.Equal(t, salidaBien, r.codigo, r.errores)
		require.JSONEq(t, `{}`, r.salida)
	}
}

func TestUnaVersionNoSoportadaSeRechazaConCodigoDeInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, `{"Version":"9","Ambiente":"prod"}`, append([]string{"reservar"}, e.banderas...)...)

	require.Equal(t, salidaInvalida, r.codigo)
	require.Contains(t, r.errores, "versión no soportada")
	require.Empty(t, r.salida)
}

func TestInvocacionesInvalidas(t *testing.T) {
	e := nuevoEntorno(t)
	casos := map[string]struct {
		peticion string
		args     []string
		contiene string
	}{
		"sin operación":         {"", nil, "uso:"},
		"operación desconocida": {"", []string{"bailar"}, "desconocida"},
		"sin almacén":           {"{}", []string{"intentos"}, "--almacen"},
		"intentar sin espacio":  {"{}", []string{"intentar", "--almacen", t.TempDir()}, "--espacio"},
		"almacén inexistente":   {`{"Version":"1"}`, []string{"intentos", "--almacen", filepath.Join(t.TempDir(), "no-existe")}, "almacén"},
		"JSON mal formado":      {"{", append([]string{"intentos"}, e.banderas...), "ilegible"},
		"campo desconocido":     {`{"Version":"1","Ambient":"prod"}`, append([]string{"intentos"}, e.banderas...), "ilegible"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := invocar(t, c.peticion, c.args...)
			require.Equal(t, salidaInvalida, r.codigo, r.errores)
			require.Contains(t, r.errores, c.contiene)
		})
	}
}

func TestUnaFuenteQueNoExisteFallaConCodigoDeFallo(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, `{"Version":"1","Ambiente":"prod","Solicitante":"ana","FuenteDelProyecto":"/no/existe","FuenteDelPipeline":"/no/existe"}`,
		append([]string{"intentar"}, e.banderas...)...)

	require.Equal(t, salidaFallo, r.codigo)
	require.NotEmpty(t, r.errores)
	require.Empty(t, r.salida)
}

func TestVersion(t *testing.T) {
	r := invocar(t, "", "version")
	require.Equal(t, salidaBien, r.codigo)
	require.Equal(t, version+"\n", r.salida)
}

var firmante = object.Signature{Name: "ana", Email: "ana@vex.test", When: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)}

func nuevoRepo(t *testing.T, llenar func(dir string)) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	llenar(dir)
	w, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, w.AddWithOptions(&git.AddOptions{All: true}))
	firma := firmante
	_, err = w.Commit("commit de prueba", &git.CommitOptions{Author: &firma})
	require.NoError(t, err)
	return dir
}

func escribir(t *testing.T, raiz, ruta, contenido string) {
	t.Helper()
	camino := filepath.Join(raiz, filepath.FromSlash(ruta))
	require.NoError(t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(t, os.WriteFile(camino, []byte(contenido), 0o644))
}

func copiar(t *testing.T, origen, destino string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(origen, func(camino string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relativa, err := filepath.Rel(origen, camino)
		if err != nil {
			return err
		}
		contenido, err := os.ReadFile(camino)
		if err != nil {
			return err
		}
		escribir(t, destino, relativa, string(contenido))
		return nil
	}))
}

func TestLogs_SinLogsQueMostrarRespondeUnaListaVacia(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: comando-deploy-ok\n  description: sale bien\n  cmd: echo todo bien\n")
	})
	require.Equal(t, salidaBien, invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...).codigo)

	r := logs(t, e, `{"Version":"1","Resultado":"fallido"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.Empty(t, leerLogs(t, r).Salidas)
	require.Contains(t, r.salida, `"Salidas": {}`)
}
