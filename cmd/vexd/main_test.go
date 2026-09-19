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
	almacen, espacio, material := t.TempDir(), t.TempDir(), t.TempDir()
	return entorno{
		banderas:     []string{"--almacen", almacen, "--espacio", espacio, "--material", material},
		repoProyecto: nuevoRepo(t, func(dir string) { escribir(t, dir, "README.md", "proyecto") }),
		repoPipeline: nuevoRepo(t, func(dir string) {
			copiar(t, filepath.Join("..", "..", "internal", "ejecucion", "testdata", "ejemplo"), dir)
		}),
	}
}

func (e entorno) intento() string {
	return `{"Version":"1","Ambiente":"prod","Solicitante":"ana",
	  "FuenteDelProyecto":` + quote(e.repoProyecto) + `,"FuenteDelPipeline":` + quote(e.repoPipeline) + `,
	  "Metadatos":{"ProjectName":"vex-demo","ProjectId":"p1"}}`
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestIntentar_LlegaADespliegueYLaSalidaDeLosComandosVaAlError(t *testing.T) {
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
	require.NotEmpty(t, r.errores, "lo que imprimen los comandos va al error")
	require.NotContains(t, r.errores, "${var.", "las variables se interpolaron")
	require.NotContains(t, r.salida, r.errores, "lo que imprimen los comandos no se mezcla con la respuesta")
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
