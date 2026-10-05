package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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

	"github.com/jairoprogramador/vex-engine/internal/borde"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

// Prueba de la raíz de composición entera: el protocolo, el borde y los seis contextos con sus adaptadores de
// verdad (almacén en disco, repositorios git, comandos del shell).

type invocacion struct {
	codigo          int
	salida, errores string
}

// lineaDeRespuesta es la respuesta del protocolo leída como JSON.
type lineaDeRespuesta struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int
		Message string
		Data    map[string]string
	} `json:"error"`
}

// peticion es la línea que se envía a vexd para una operación.
func peticion(metodo, params string) string {
	var compactos bytes.Buffer
	if err := json.Compact(&compactos, []byte(params)); err != nil {
		compactos.Reset()
		compactos.WriteString(params) // una prueba de params rotos los envía tal cual
	}
	return `{"jsonrpc":"2.0","id":"1","method":` + quote(metodo) + `,"params":` + compactos.String() + `}`
}

func invocar(t *testing.T, r rutas, metodo, params string) invocacion {
	t.Helper()
	return invocarLinea(t, r, peticion(metodo, params))
}

func invocarLinea(t *testing.T, r rutas, linea string) invocacion {
	t.Helper()
	return invocarCon(t, context.Background(), r, strings.NewReader(linea+"\n"))
}

func invocarCon(t *testing.T, ctx context.Context, r rutas, entrada io.Reader) invocacion {
	t.Helper()
	var salida, errores bytes.Buffer
	codigo := ejecutar(ctx, r, entrada, &salida, &errores)
	return invocacion{codigo, salida.String(), errores.String()}
}

// lineas son los mensajes de la salida estándar. Todos son protocolo, uno por línea y cada uno entero.
func (i invocacion) lineas(t *testing.T) []string {
	t.Helper()
	require.True(t, strings.HasSuffix(i.salida, "\n"), "una línea completa: %q", i.salida)
	lineas := strings.Split(strings.TrimSuffix(i.salida, "\n"), "\n")
	for _, linea := range lineas {
		require.True(t, json.Valid([]byte(linea)), "la salida estándar es solo protocolo: %q", linea)
	}
	return lineas
}

// respuesta lee la respuesta de la salida estándar: es la última línea. Las anteriores, si las hay, son
// notificaciones de progreso (lo que el motor cuenta mientras avanza), nunca otra cosa.
func (i invocacion) respuesta(t *testing.T) lineaDeRespuesta {
	t.Helper()
	lineas := i.lineas(t)
	i.progresoDe(t, lineas[:len(lineas)-1])
	var r lineaDeRespuesta
	require.NoError(t, json.Unmarshal([]byte(lineas[len(lineas)-1]), &r))
	require.Equal(t, "2.0", r.JSONRPC)
	require.NotEmpty(t, r.ID, "la última línea es la respuesta: lleva id")
	return r
}

// progreso son los eventos que el motor contó antes de responder, en orden.
func (i invocacion) progreso(t *testing.T) []paramsDeProgreso {
	t.Helper()
	lineas := i.lineas(t)
	return i.progresoDe(t, lineas[:len(lineas)-1])
}

func (i invocacion) progresoDe(t *testing.T, lineas []string) []paramsDeProgreso {
	t.Helper()
	eventos := make([]paramsDeProgreso, 0, len(lineas))
	for _, linea := range lineas {
		var n struct {
			JSONRPC string           `json:"jsonrpc"`
			ID      *json.RawMessage `json:"id"`
			Method  string           `json:"method"`
			Params  paramsDeProgreso `json:"params"`
		}
		require.NoError(t, json.Unmarshal([]byte(linea), &n))
		require.Equal(t, "2.0", n.JSONRPC)
		require.Nil(t, n.ID, "una notificación no lleva id: %s", linea)
		require.Equal(t, metodoProgreso, n.Method, "antes de la respuesta solo hay progreso: %s", linea)
		eventos = append(eventos, n.Params)
	}
	return eventos
}

// resultado lee el result de una respuesta que salió bien.
func (i invocacion) resultado(t *testing.T, destino any) {
	t.Helper()
	r := i.respuesta(t)
	require.Nil(t, r.Error, "se esperaba un resultado: %s", i.salida)
	require.NoError(t, json.Unmarshal(r.Result, destino))
}

// error lee el error de una respuesta que falló.
func (i invocacion) error(t *testing.T) lineaDeRespuesta {
	t.Helper()
	r := i.respuesta(t)
	require.NotNil(t, r.Error, "se esperaba un error: %s", i.salida)
	require.Nil(t, r.Result)
	return r
}

type entorno struct {
	rutas                      rutas
	repoProyecto, repoPipeline string
}

func nuevoEntorno(t *testing.T) entorno {
	t.Helper()
	return nuevoEntornoConPipeline(t, func(string) {})
}

// nuevoEntornoConPipeline parte del pipeline de ejemplo y deja que la prueba lo cambie antes de commitearlo.
func nuevoEntornoConPipeline(t *testing.T, cambiar func(dir string)) entorno {
	t.Helper()
	return entorno{
		rutas:        rutas{almacen: t.TempDir(), espacio: t.TempDir(), material: t.TempDir()},
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

func (e entorno) intento() string {
	return `{"Version":"1","Ambiente":"prod","Solicitante":"ana",
	  "FuenteDelProyecto":` + quote(e.repoProyecto) + `,"FuenteDelPipeline":` + quote(e.repoPipeline) + `,
	  "Metadatos":{"ProjectName":"vex-demo","ProjectId":"p1"}}`
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

// intentar abre un intento y devuelve su id; el intento tiene que terminar con el código que se espera.
func intentar(t *testing.T, e entorno, params string, codigoEsperado int) string {
	t.Helper()
	r := invocar(t, e.rutas, "intentar", params)
	require.Equal(t, codigoEsperado, r.codigo, r.errores)
	var resultado struct{ Intento string }
	r.resultado(t, &resultado)
	return resultado.Intento
}

func TestIntentar_LlegaADespliegueYNoMuestraLaSalidaDeLosComandos(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var resultado struct {
		Intento, Estado, Despliegue string
		Detalle                     struct {
			Tiempo string
			Pasos  []struct{ Nombre, Estado string }
		}
	}
	r.resultado(t, &resultado)
	require.Equal(t, "exitoso", resultado.Estado)
	require.NotEmpty(t, resultado.Despliegue)
	require.NotEmpty(t, resultado.Detalle.Tiempo)
	require.NotEmpty(t, resultado.Detalle.Pasos, "el detalle lista los pasos que se dieron")
	for _, paso := range resultado.Detalle.Pasos {
		require.NotEmpty(t, paso.Nombre)
		require.Equal(t, "ejecutado", paso.Estado, "en el primer intento nada se precarga: %s", paso.Nombre)
	}
	require.JSONEq(t, `"1"`, string(r.respuesta(t).ID), "se responde con el id de la petición")
	require.Empty(t, r.errores, "lo que imprimen los comandos no se muestra: se consulta con logs")
	require.NotContains(t, r.salida, "etiqueta=v1.0.0", "ni se mezcla con la respuesta")
}

func logs(t *testing.T, e entorno, params string) invocacion {
	t.Helper()
	return invocar(t, e.rutas, "logs", params)
}

func leerLogs(t *testing.T, r invocacion) borde.RespuestaDeLogs {
	t.Helper()
	require.Equal(t, salidaBien, r.codigo, r.errores)
	var l borde.RespuestaDeLogs
	r.resultado(t, &l)
	require.NotEmpty(t, l.Intento)
	return l
}

// delPaso son las salidas de un paso, en el orden en que corrieron sus comandos.
func delPaso(l borde.RespuestaDeLogs, paso string) []historialpublicado.Salida {
	var salidas []historialpublicado.Salida
	for _, s := range l.Salidas {
		if s.Paso == paso {
			salidas = append(salidas, s)
		}
	}
	return salidas
}

func comandosDe(salidas []historialpublicado.Salida) []string {
	nombres := make([]string, 0, len(salidas))
	for _, s := range salidas {
		nombres = append(nombres, s.Comando)
	}
	return nombres
}

func primeraSalidaDe(l borde.RespuestaDeLogs, paso string) int {
	for i, s := range l.Salidas {
		if s.Paso == paso {
			return i
		}
	}
	return -1
}

func TestLogs_SinIntentoMuestraLosComandosDelUltimoConSuSalida(t *testing.T) {
	e := nuevoEntorno(t)
	intentar(t, e, e.intento(), salidaBien)

	r := logs(t, e, `{"Version":"1"}`)

	require.Empty(t, r.errores)
	l := leerLogs(t, r)
	test := delPaso(l, "test")
	require.Equal(t, comandosDeclarados(t, "01-test"), comandosDe(test), "los que el paso declara, en el orden en que corrieron")
	require.Equal(t, "hola vex-demo", strings.TrimSpace(test[0].Texto))
	require.True(t, test[0].Exitoso)
	require.Equal(t, "etiqueta=v1.0.0", strings.TrimSpace(test[1].Texto))
	require.Equal(t, "despliegue exitoso", strings.TrimSpace(delPaso(l, "deploy")[0].Texto))
	require.Equal(t, "prod", l.Ambiente)
	for _, s := range l.Salidas {
		require.NotContains(t, s.Texto, "${var.", "las variables se interpolaron")
	}
	require.Less(t, primeraSalidaDe(l, "test"), primeraSalidaDe(l, "acr"), "los pasos, en el orden en que corrieron")
	require.Less(t, primeraSalidaDe(l, "acr"), primeraSalidaDe(l, "deploy"))
}

func TestLogs_ConIntentoMuestraLosDeEseIntento(t *testing.T) {
	e := nuevoEntorno(t)
	intento := intentar(t, e, e.intento(), salidaBien)

	l := leerLogs(t, logs(t, e, `{"Version":"1","Intento":`+quote(intento)+`}`))

	require.Equal(t, intento, l.Intento)
	require.Equal(t, "comando-test-01", delPaso(l, "test")[0].Comando)
}

func TestLogs_SinIntentoDiceCualEsElUltimo(t *testing.T) {
	e := nuevoEntorno(t)
	var ultimo string
	for range 2 {
		ultimo = intentar(t, e, e.intento(), salidaBien)
	}

	l := leerLogs(t, logs(t, e, `{"Version":"1"}`))

	require.Equal(t, ultimo, l.Intento)
}

func TestLogs_SoloMuestraLosPasosHastaDondeLlegoElIntento(t *testing.T) {
	e := nuevoEntorno(t)
	hasta := strings.Replace(e.intento(), `"Ambiente":"prod",`, `"Ambiente":"prod","HastaPaso":"test",`, 1)
	intentar(t, e, hasta, salidaBien)

	l := leerLogs(t, logs(t, e, `{"Version":"1"}`))

	require.Equal(t, comandosDeclarados(t, "01-test"), comandosDe(l.Salidas), "solo los del paso al que llegó")
}

func TestLogs_UnIntentoQueNoExisteEsNoExiste(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1","Intento":"no-existe"}`)

	require.Equal(t, salidaFallo, r.codigo)
	require.Equal(t, codigoNoExiste, r.error(t).Error.Code)
}

func TestLogs_SinNingunIntentoEsNoExiste_ElMotorNoTieneTextoParaHumanos(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1"}`)

	require.Equal(t, salidaFallo, r.codigo, r.errores)
	respuesta := r.error(t)
	require.Equal(t, codigoNoExiste, respuesta.Error.Code)
	require.Equal(t, tipoNoExiste, respuesta.Error.Data["tipo"])
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
	intentar(t, e, e.intento(), salidaFallo)

	todos := leerLogs(t, logs(t, e, `{"Version":"1"}`))
	fallidos := leerLogs(t, logs(t, e, `{"Version":"1","Resultado":"fallido"}`))
	exitosos := leerLogs(t, logs(t, e, `{"Version":"1","Resultado":"exitoso"}`))

	require.Equal(t, []string{"comando-deploy-ok", "comando-deploy-roto"}, comandosDe(delPaso(todos, "deploy")))
	require.Equal(t, []string{"comando-deploy-roto"}, comandosDe(fallidos.Salidas))
	require.False(t, fallidos.Salidas[0].Exitoso)
	require.Equal(t, "explotó", strings.TrimSpace(fallidos.Salidas[0].Texto))
	require.Contains(t, comandosDe(exitosos.Salidas), "comando-deploy-ok")
	require.NotContains(t, comandosDe(exitosos.Salidas), "comando-deploy-roto")
}

func TestLogs_UnResultadoDesconocidoEsUnaPeticionInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := logs(t, e, `{"Version":"1","Resultado":"roto"}`)

	require.Equal(t, salidaInvalida, r.codigo, r.errores)
	respuesta := r.error(t)
	require.Equal(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
	require.Contains(t, respuesta.Error.Message, "Resultado")
}

func TestLogs_SinLogsQueMostrarNoHayNingunaSalida(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: comando-deploy-ok\n  description: sale bien\n  cmd: echo todo bien\n")
	})
	intentar(t, e, e.intento(), salidaBien)

	l := leerLogs(t, logs(t, e, `{"Version":"1","Resultado":"fallido"}`))

	require.Empty(t, l.Salidas)
}

func TestConsultas_VenLoQueIntentarDejoEnElHistorial(t *testing.T) {
	e := nuevoEntorno(t)
	intentar(t, e, e.intento(), salidaBien)

	despliegues := invocar(t, e.rutas, "despliegues", `{"Version":"1","Ambiente":"prod"}`)

	require.Equal(t, salidaBien, despliegues.codigo, despliegues.errores)
	var lista []map[string]any
	despliegues.resultado(t, &lista)
	require.Len(t, lista, 1)
}

func TestReservarYLiberar_ResponderUnObjetoVacio(t *testing.T) {
	e := nuevoEntorno(t)
	for _, op := range []string{"reservar", "liberar"} {
		r := invocar(t, e.rutas, op, `{"Version":"1","Ambiente":"prod"}`)

		require.Equal(t, salidaBien, r.codigo, r.errores)
		require.JSONEq(t, `{}`, string(r.respuesta(t).Result))
	}
}

func TestUnaVersionNoSoportadaSeRechazaConCodigoDeInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "reservar", `{"Version":"9","Ambiente":"prod"}`)

	require.Equal(t, salidaInvalida, r.codigo)
	respuesta := r.error(t)
	require.Equal(t, codigoVersionNoSoportada, respuesta.Error.Code)
	require.Equal(t, tipoVersionNoSoportada, respuesta.Error.Data["tipo"])
	require.Contains(t, respuesta.Error.Message, "versión no soportada")
}

func TestPeticionesInvalidas(t *testing.T) {
	e := nuevoEntorno(t)
	sinAlmacen := rutas{}
	sinEspacio := rutas{almacen: t.TempDir()}
	almacenInexistente := rutas{almacen: filepath.Join(t.TempDir(), "no-existe")}
	casos := map[string]struct {
		linea    string
		rutas    rutas
		codigo   int
		tipo     string
		contiene string
	}{
		"JSON mal formado":      {`{`, e.rutas, protocolo.CodigoJSONInvalido, "json_invalido", "JSON"},
		"no es una petición":    {`[1]`, e.rutas, protocolo.CodigoPeticionInvalida, tipoPeticionInvalida, "petición"},
		"sin id":                {`{"jsonrpc":"2.0","method":"intentos","params":{}}`, e.rutas, protocolo.CodigoPeticionInvalida, tipoPeticionInvalida, "id"},
		"sin operación":         {`{"jsonrpc":"2.0","id":"1","params":{}}`, e.rutas, protocolo.CodigoPeticionInvalida, tipoPeticionInvalida, "method"},
		"operación desconocida": {peticion("bailar", `{}`), e.rutas, protocolo.CodigoMetodoDesconocido, tipoOperacionDesconocida, "bailar"},
		"campo desconocido":     {peticion("intentos", `{"Version":"1","Ambient":"prod"}`), e.rutas, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, "Ambient"},
		"entorno sin comandos":  {`{"jsonrpc":"2.0","id":"1","method":"intentos","params":{},"entorno":{"A":"1"}}`, e.rutas, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, "no ejecuta comandos"},
		"sin almacén":           {peticion("intentos", `{"Version":"1","Ambiente":"prod"}`), sinAlmacen, codigoConfiguracionInvalida, tipoConfiguracionInvalida, nombreAlmacen},
		"intentar sin espacio":  {peticion("intentar", `{}`), sinEspacio, codigoConfiguracionInvalida, tipoConfiguracionInvalida, nombreEspacio},
		"almacén inexistente":   {peticion("intentos", `{"Version":"1","Ambiente":"prod"}`), almacenInexistente, codigoConfiguracionInvalida, tipoConfiguracionInvalida, nombreAlmacen},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := invocarLinea(t, c.rutas, c.linea)

			require.Equal(t, salidaInvalida, r.codigo, r.salida)
			respuesta := r.error(t)
			require.Equal(t, c.codigo, respuesta.Error.Code)
			require.Equal(t, c.tipo, respuesta.Error.Data["tipo"])
			require.Contains(t, respuesta.Error.Message, c.contiene)
			require.Empty(t, r.errores, "un error de quien invoca no es un error interno")
		})
	}
}

func TestUnaPeticionSinSaltoFinalTambienSeAtiende(t *testing.T) {
	r := invocarCon(t, context.Background(), rutas{}, strings.NewReader(peticion("describir", `{}`)))

	require.Equal(t, salidaBien, r.codigo, r.errores)
}

func TestSinPeticionEsInvalido(t *testing.T) {
	r := invocarCon(t, context.Background(), rutas{}, strings.NewReader(""))

	require.Equal(t, salidaInvalida, r.codigo)
	respuesta := r.error(t)
	require.Equal(t, protocolo.CodigoPeticionInvalida, respuesta.Error.Code)
	require.JSONEq(t, `null`, string(respuesta.ID), "sin petición no hay id al que responder")
}

func TestUnaLineaDemasiadoLargaEsInvalida(t *testing.T) {
	r := invocarLinea(t, rutas{}, strings.Repeat("x", maximoDeLinea+1))

	require.Equal(t, salidaInvalida, r.codigo)
	require.Equal(t, protocolo.CodigoPeticionInvalida, r.error(t).Error.Code)
}

func TestElIdDeLaPeticionSeDevuelveEnElError(t *testing.T) {
	r := invocarLinea(t, rutas{}, `{"jsonrpc":"2.0","id":42,"method":"bailar"}`)

	require.JSONEq(t, `42`, string(r.error(t).ID))
}

func TestUnaFuenteQueNoExisteEsUnaPeticionInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "intentar",
		`{"Version":"1","Ambiente":"prod","Solicitante":"ana","FuenteDelProyecto":"/no/existe","FuenteDelPipeline":"/no/existe"}`)

	require.Equal(t, salidaInvalida, r.codigo, "quien invoca lo puede corregir")
	respuesta := r.error(t)
	require.Equal(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
	require.Equal(t, tipoParametrosInvalidos, respuesta.Error.Data["tipo"])
	require.Contains(t, respuesta.Error.Message, "/no/existe")
	require.Empty(t, r.errores, "no es un error interno")
}

func TestSimularConUnaFuenteQueNoExisteEsUnaPeticionInvalida(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "simular",
		`{"Version":"1","Ambiente":"prod","Solicitante":"ana","HastaPaso":"test","CopiaDeTrabajo":"/no/existe"}`)

	require.Equal(t, salidaInvalida, r.codigo)
	require.Equal(t, protocolo.CodigoParametrosInvalidos, r.error(t).Error.Code)
}

func TestUnPipelineQueNoPasaLaComprobacionEsRechazado(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "config.yaml", "schema_version: 99\n")
	})

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaFallo, r.codigo, "la petición era válida: lo que falla es el pipeline")
	respuesta := r.error(t)
	require.Equal(t, codigoRechazado, respuesta.Error.Code)
	require.Equal(t, tipoRechazado, respuesta.Error.Data["tipo"])
	require.Contains(t, respuesta.Error.Message, "config.yaml", "dice qué fichero falla")
	require.Empty(t, r.errores)
}

func TestUnErrorInternoNoCuentaSuCausaPeroLaEscribeEnLaSalidaDeError(t *testing.T) {
	var salida, errores bytes.Buffer
	causa := errors.New("el disco /var/almacen se llenó")
	f := clasificar(causa)
	f.causa = causa

	codigo := responder(protocolo.NuevoEmisor(&salida), &errores, protocolo.Peticion{ID: json.RawMessage(`"9"`), Method: "intentar"}, f)

	require.Equal(t, salidaFallo, codigo)
	require.Contains(t, salida.String(), `"error interno"`)
	require.NotContains(t, salida.String(), "disco", "la causa no llega a quien invoca")
	require.Contains(t, errores.String(), "el disco /var/almacen se llenó", "va a la salida de error")
	require.Contains(t, errores.String(), "intentar")
}

func TestDescribir_NoNecesitaElMotor(t *testing.T) {
	r := invocar(t, rutas{}, "describir", `{}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var d descripcion
	r.resultado(t, &d)
	require.Equal(t, version, d.VersionDelMotor)
	require.Equal(t, borde.VersionesSoportadas, d.VersionesDelLenguaje)
	require.Contains(t, d.Operaciones, "intentar")
	require.Contains(t, d.Operaciones, "describir")
	require.IsIncreasing(t, d.Operaciones, "ordenadas, para que no cambien de una invocación a otra")
}

func TestDescribir_SinParametrosTambien(t *testing.T) {
	r := invocarLinea(t, rutas{}, `{"jsonrpc":"2.0","id":"1","method":"describir"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
}

func TestDescribir_NoTieneParametros(t *testing.T) {
	r := invocar(t, rutas{}, "describir", `{"Version":"1"}`)

	require.Equal(t, salidaInvalida, r.codigo)
	require.Equal(t, protocolo.CodigoParametrosInvalidos, r.error(t).Error.Code)
}

func TestCancelacion_UnContextoCancelado(t *testing.T) {
	e := nuevoEntorno(t)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	r := invocarCon(t, ctx, e.rutas, strings.NewReader(peticion("intentar", e.intento())+"\n"))

	require.Equal(t, salidaCancelado, r.codigo, r.salida)
}

func TestCancelacion_LaNotificacionCancelarCancelaElIntentoAbierto(t *testing.T) {
	listo := filepath.Join(t.TempDir(), "listo")
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml",
			"- name: comando-deploy-lento\n  description: tarda\n  cmd: touch "+listo+"; exec sleep 30\n")
	})
	entrada, escribirEnLaEntrada := io.Pipe()
	var salida, errores bytes.Buffer
	terminado := make(chan int, 1)
	go func() { terminado <- ejecutar(context.Background(), e.rutas, entrada, &salida, &errores) }()

	_, err := io.WriteString(escribirEnLaEntrada, peticion("intentar", e.intento())+"\n")
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		20*time.Second, 50*time.Millisecond, "el comando lento no llegó a correr")
	_, err = io.WriteString(escribirEnLaEntrada, `{"jsonrpc":"2.0","method":"cancelar"}`+"\n")
	require.NoError(t, err)

	select {
	case codigo := <-terminado:
		require.Equal(t, salidaCancelado, codigo, salida.String())
	case <-time.After(20 * time.Second):
		t.Fatal("cancelar no detuvo el intento")
	}
	r := invocacion{salidaCancelado, salida.String(), errores.String()}
	var resultado struct{ Estado string }
	r.resultado(t, &resultado)
	require.Equal(t, "cancelado", resultado.Estado, "el intento ya estaba abierto: es un resultado, no un error")
	require.NoError(t, escribirEnLaEntrada.Close())
}

func TestCerrarLaEntradaNoCancela(t *testing.T) {
	// Un pipe (`echo '…' | vexd`) cierra la entrada nada más escribir: es «no envío más», no «cancela».
	e := nuevoEntorno(t)

	r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(peticion("intentar", e.intento())+"\n"))

	require.Equal(t, salidaBien, r.codigo, r.salida)
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
