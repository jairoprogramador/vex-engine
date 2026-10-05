package borde_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	definicionaplicacion "github.com/jairoprogramador/vex-engine/internal/definicion/aplicacion"
	definicioninfraestructura "github.com/jairoprogramador/vex-engine/internal/definicion/infraestructura"
	ejecucionaplicacion "github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	ejecucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	resolucionaplicacion "github.com/jairoprogramador/vex-engine/internal/resolucion/aplicacion"
	resolucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/resolucion/infraestructura"
	suministroaplicacion "github.com/jairoprogramador/vex-engine/internal/suministro/aplicacion"
	suministroinfraestructura "github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
)

// Prueba de punta a punta de RD-06 (§7 de la ficha): compone los cuatro contextos de arriba de verdad —
// Historial (en memoria), Suministro (repositorios git de verdad), Definición y Resolución — junto con
// Ejecución y el borde mínimo. Es la única prueba que valida que el diseño de las cuatro capas encaja.

var firmante = object.Signature{Name: "ana", Email: "ana@vex.test", When: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)}

// sistema es todo lo que monta una prueba de punta a punta, listo para llamar borde.Intentar/HacerRollback.
type sistema struct {
	borde     *borde.Servicio
	historial *historialaplicacion.Servicio
	espacio   *ejecucioninfraestructura.EspacioDeTrabajo
	raiz      string // la raíz del espacio de trabajo

	repoProyecto string
	repoPipeline string
}

func montarSistema(t *testing.T) *sistema {
	t.Helper()

	almacen := historialinfraestructura.NuevoAlmacenEnMemoria()
	historialServicio := historialaplicacion.NuevoServicio(historialaplicacion.Dependencias{
		Intentos:     historialinfraestructura.NuevosIntentos(almacen),
		Despliegues:  historialinfraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  historialinfraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: historialinfraestructura.NuevosLanzamientos(almacen),
		Reservas:     historialinfraestructura.NuevasReservas(almacen),
		Salidas:      historialinfraestructura.NuevasSalidas(almacen),
		Reloj:        historialinfraestructura.RelojDelSistema{},
		Identidades:  historialinfraestructura.IdentidadesUUID{},
	})

	suministroServicio := suministroaplicacion.NuevoServicio(suministroaplicacion.Dependencias{
		Repositorios: suministroinfraestructura.NuevosRepositoriosLocales(t.TempDir()),
		Hashes:       suministroinfraestructura.HashDeContenido{},
	})

	definicionServicio := definicionaplicacion.NuevoServicio(definicionaplicacion.Dependencias{
		Pipelines: definicioninfraestructura.NuevosPipelinesDeSuministro(suministroServicio),
	})

	resolucionServicio := resolucionaplicacion.NuevoServicio(resolucionaplicacion.Dependencias{
		Historial:  resolucioninfraestructura.NuevoHistorial(historialServicio, historialServicio),
		Definicion: resolucioninfraestructura.NuevaDefinicion(definicionServicio),
	})

	raizDelEspacio := t.TempDir()
	espacio := ejecucioninfraestructura.NuevoEspacioDeTrabajo(raizDelEspacio)
	ejecucionServicio := ejecucionaplicacion.NuevoServicio(ejecucionaplicacion.Dependencias{
		Pipelines:             ejecucioninfraestructura.NuevosPipelines(definicionServicio),
		Fuentes:               ejecucioninfraestructura.NuevasFuentes(suministroServicio),
		Variables:             ejecucioninfraestructura.NuevasVariables(resolucionServicio.ParaEjecucion()),
		Historial:             ejecucioninfraestructura.NuevoHistorial(historialServicio),
		Comandos:              ejecucioninfraestructura.NuevosComandos(),
		EspacioDeTrabajo:      espacio,
		NombreDeLaHerramienta: "vexd-e2e",
	})

	return &sistema{
		borde:        borde.NuevoServicio(borde.Dependencias{Ejecucion: ejecucionServicio, Historial: historialServicio}),
		historial:    historialServicio,
		espacio:      espacio,
		raiz:         raizDelEspacio,
		repoProyecto: nuevoRepoDeProyecto(t),
		repoPipeline: nuevoRepoDePipeline(t),
	}
}

func (s *sistema) peticion(t *testing.T) ejecucionpublicado.PeticionDeIntento {
	t.Helper()
	return ejecucionpublicado.PeticionDeIntento{
		Version: "1", Ambiente: "prod", Solicitante: "ana",
		FuenteDelProyecto: s.repoProyecto, FuenteDelPipeline: s.repoPipeline,
		Metadatos: ejecucionpublicado.Metadatos{ProjectName: "vex-demo", ProjectId: "p1"},
	}
}

// nuevoRepoDeProyecto es un repositorio git mínimo que hace de "código del proyecto": su contenido no importa
// para estas pruebas, solo que exista y tenga un commit.
func nuevoRepoDeProyecto(t *testing.T) string {
	t.Helper()
	dir, repo := nuevoRepoVacio(t)
	escribirFichero(t, dir, "README.md", "proyecto de prueba")
	commitear(t, repo)
	return dir
}

// nuevoRepoDePipeline copia el pipeline de ejemplo de este mismo directorio (testdata/ejemplo) a un
// repositorio git nuevo, con un commit.
func nuevoRepoDePipeline(t *testing.T) string {
	t.Helper()
	dir, repo := nuevoRepoVacio(t)
	copiarArbol(t, filepath.Join("..", "ejecucion", "testdata", "ejemplo"), dir)
	commitear(t, repo)
	return dir
}

func nuevoRepoVacio(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return dir, repo
}

func escribirFichero(t *testing.T, raiz, ruta, contenido string) {
	t.Helper()
	camino := filepath.Join(raiz, filepath.FromSlash(ruta))
	require.NoError(t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(t, os.WriteFile(camino, []byte(contenido), 0o644))
}

func copiarArbol(t *testing.T, origen, destino string) {
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
		escribirFichero(t, destino, relativa, string(contenido))
		return nil
	}))
}

func commitear(t *testing.T, repo *git.Repository) string {
	t.Helper()
	w, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, w.AddWithOptions(&git.AddOptions{All: true}))
	firma := firmante
	firma.When = firma.When.Add(time.Duration(commitId(t)) * time.Second) // cada commit, un instante distinto
	id, err := w.Commit("commit de prueba", &git.CommitOptions{Author: &firma})
	require.NoError(t, err)
	return id.String()
}

var contadorDeCommits int
var muContador sync.Mutex

func commitId(t *testing.T) int {
	t.Helper()
	muContador.Lock()
	defer muContador.Unlock()
	contadorDeCommits++
	return contadorDeCommits
}

func TestE2E_UnPrimerIntentoLlegaADespliegue(t *testing.T) {
	s := montarSistema(t)
	pasos := pasosDelEjemplo(t)

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.NotEmpty(t, resultado.Despliegue)
	require.NotEmpty(t, resultado.Detalle.Tiempo)
	require.Equal(t, nombresDeLosPasos(pasos), nombresDelDetalle(resultado.Detalle), "todos los pasos, en su orden")
	for _, paso := range resultado.Detalle.Pasos {
		require.Equal(t, "ejecutado", paso.Estado, "en el primer intento nada se puede precargar: %s", paso.Nombre)
	}

	salidas, err := s.historial.SalidasDeUnIntento(context.Background(), resultado.Intento, historialpublicado.TodasLasSalidas)
	require.NoError(t, err)
	pasosConSalida := map[string]bool{}
	for _, salida := range salidas {
		pasosConSalida[salida.Paso] = true
		require.True(t, salida.Exitoso, "%s/%s", salida.Paso, salida.Comando)
		require.NotContains(t, salida.Texto, "${var.", "las variables y las plantillas se interpolaron")
	}
	for _, paso := range pasos {
		if len(paso.Comandos) > 0 {
			require.True(t, pasosConSalida[paso.Nombre], "la salida de los comandos del paso %s quedó en el historial", paso.Nombre)
		}
	}
}

// Sin cambios, un segundo intento precarga todos los pasos, sea cual sea la regla de cada uno — también los que
// miran las variables, aunque el directorio del material (project_workdir) sea otro en cada intento.
func TestE2E_UnSegundoIntentoSinCambiosNoReejecutaNada(t *testing.T) {
	s := montarSistema(t)
	primero, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)
	require.Equal(t, "exitoso", primero.Estado)
	pasos := pasosDelEjemplo(t)

	segundo, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", segundo.Estado)
	require.Equal(t, nombresDeLosPasos(pasos), nombresDelDetalle(segundo.Detalle))
	for _, paso := range segundo.Detalle.Pasos {
		require.Equal(t, "precargado", paso.Estado, "paso %s", paso.Nombre)
	}
}

func TestE2E_CambiarUnaPlantillaReejecutaSoloEsePaso(t *testing.T) {
	s := montarSistema(t)
	_, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)

	// El paso a cambiar es uno con plantilla y con reglas que miran las instrucciones: cambiarla tiene
	// que reejecutarlo, y solo a él.
	var elegido pasoDelEjemplo
	otros := pasosDelEjemplo(t)
	for i, paso := range otros {
		if paso.plantilla() != "" && paso.mira("instructions") {
			elegido = paso
			otros = slices.Delete(otros, i, i+1)
			break
		}
	}
	require.NotEmpty(t, elegido.Nombre, "el ejemplo necesita un paso con plantilla y reglas que miren las instrucciones")

	repo, err := git.PlainOpen(s.repoPipeline)
	require.NoError(t, err)
	plantilla := elegido.plantilla()
	actual, err := os.ReadFile(filepath.Join(s.repoPipeline, filepath.FromSlash(plantilla)))
	require.NoError(t, err)
	escribirFichero(t, s.repoPipeline, plantilla, string(actual)+"\nlínea añadida por la prueba\n")
	commitear(t, repo)

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	estados := estadosPorPaso(resultado.Detalle)
	require.Equal(t, "ejecutado", estados[elegido.Nombre], "el paso con la plantilla cambiada sí se reejecuta")
	for _, paso := range otros {
		require.Equal(t, "precargado", estados[paso.Nombre], "nada de lo que ve cambió: %s", paso.Nombre)
	}
}

// Cambiar el valor de una variable que solo usa un paso reejecuta ese paso y a ningún otro: en el fixture cada
// paso usa la suya (${var.test}, ${var.acr}…) y todos usan además ${var.shared}.
func TestE2E_CambiarUnaVariableDelPipelineReejecutaSoloAlPasoQueLaUsa(t *testing.T) {
	s := montarSistema(t)
	_, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)

	repo, err := git.PlainOpen(s.repoPipeline)
	require.NoError(t, err)
	escribirFichero(t, s.repoPipeline, "variables/prod/test.yaml", "- name: test\n  value: \"otro-valor\"\n")
	commitear(t, repo)

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	estados := estadosPorPaso(resultado.Detalle)
	require.Equal(t, "ejecutado", estados["test"], "usa la variable que cambió")
	for _, paso := range pasosDelEjemplo(t) {
		if paso.Nombre != "test" {
			require.Equal(t, "precargado", estados[paso.Nombre], "no usa la variable que cambió: %s", paso.Nombre)
		}
	}
}

// Una variable compartida reejecuta a los pasos que la usan; aquí la usan todos.
func TestE2E_CambiarUnaVariableCompartidaReejecutaATodosLosQueLaUsan(t *testing.T) {
	s := montarSistema(t)
	_, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)

	repo, err := git.PlainOpen(s.repoPipeline)
	require.NoError(t, err)
	escribirFichero(t, s.repoPipeline, "variables/vars.yaml", "- name: shared\n  value: \"otro-valor\"\n")
	commitear(t, repo)

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	estados := estadosPorPaso(resultado.Detalle)
	for _, paso := range pasosDelEjemplo(t) {
		require.Equal(t, "ejecutado", estados[paso.Nombre], "usa ${var.shared}: %s", paso.Nombre)
	}
}

// Declarar una variable que ningún paso usa no reejecuta nada.
func TestE2E_UnaVariableNuevaQueNingunPasoUsaNoReejecutaNada(t *testing.T) {
	s := montarSistema(t)
	_, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)

	repo, err := git.PlainOpen(s.repoPipeline)
	require.NoError(t, err)
	escribirFichero(t, s.repoPipeline, "variables/prod/sin_uso.yaml", "- name: sin_uso\n  value: \"x\"\n")
	commitear(t, repo)

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	estados := estadosPorPaso(resultado.Detalle)
	for _, paso := range pasosDelEjemplo(t) {
		require.Equal(t, "precargado", estados[paso.Nombre], "nadie usa la variable nueva: %s", paso.Nombre)
	}
}

func TestE2E_UnIntentoConCopiaDeTrabajoNuncaLlegaADespliegue(t *testing.T) {
	s := montarSistema(t)
	peticion := s.peticion(t)
	peticion.CopiaDeTrabajo = s.repoProyecto

	resultado, err := s.borde.Intentar(context.Background(), peticion)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Empty(t, resultado.Despliegue)
}

func TestE2E_UnSegundoIntentoEnElMismoAmbienteSeRechaza(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	_, err := s.historial.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: "prod", Solicitante: "otro", Pasos: []historialpublicado.PasoDeclarado{{Nombre: primerPaso(t)}},
		HastaPaso: primerPaso(t), HashDelCodigo: "h", ConCommits: true,
	})
	require.NoError(t, err)

	_, err = s.borde.Intentar(ctx, s.peticion(t))

	require.Error(t, err)
	var ocupado *historialpublicado.AmbienteOcupadoError
	require.True(t, errors.As(err, &ocupado), "se esperaba un *AmbienteOcupadoError, se obtuvo: %v", err)
}

// El espacio de trabajo de un ambiente es de quien lo ocupa: si otro intento lo tiene, rehacerlo borraría los
// directorios de sus pasos mientras corre. Por eso el Historial, que decide quién ocupa el ambiente, se consulta
// antes de tocar el disco.
func TestE2E_UnAmbienteOcupadoNoTocaElEspacioDeTrabajoDeQuienLoOcupa(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	_, err := s.historial.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: "prod", Solicitante: "otro", Pasos: []historialpublicado.PasoDeclarado{{Nombre: primerPaso(t)}},
		HastaPaso: primerPaso(t), HashDelCodigo: "h", ConCommits: true,
	})
	require.NoError(t, err)
	ubicacion, err := s.espacio.Ubicar(s.repoProyecto, s.repoPipeline, "prod")
	require.NoError(t, err)
	directorioDelPaso := s.espacio.DirectorioDelPaso(ubicacion, primerPaso(t))
	require.NoError(t, os.MkdirAll(directorioDelPaso, 0o755))
	marca := filepath.Join(directorioDelPaso, "lo-que-escribe-el-intento-que-corre")
	require.NoError(t, os.WriteFile(marca, []byte("sigue aquí"), 0o644))

	_, err = s.borde.Intentar(ctx, s.peticion(t))

	var ocupado *historialpublicado.AmbienteOcupadoError
	require.ErrorAs(t, err, &ocupado)
	contenido, err := os.ReadFile(marca)
	require.NoError(t, err, "el espacio de trabajo del intento que ocupa el ambiente sigue ahí")
	require.Equal(t, "sigue aquí", string(contenido))
}

// EJ-5: si el espacio de trabajo no se puede preparar, el intento no empieza, y no deja el ambiente ocupado: el
// siguiente intento, con el espacio arreglado, no se rechaza.
func TestE2E_SiElEspacioNoSePuedePrepararElAmbienteQuedaLibre(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	ubicacion, err := s.espacio.Ubicar(s.repoProyecto, s.repoPipeline, "prod")
	require.NoError(t, err)
	// Un fichero donde tendría que haber un directorio: el espacio de este proyecto no se puede crear.
	bloqueo := filepath.Join(s.raiz, ubicacion.Proyecto)
	require.NoError(t, os.WriteFile(bloqueo, []byte("no soy un directorio"), 0o644))

	_, err = s.borde.Intentar(ctx, s.peticion(t))

	require.ErrorIs(t, err, ejecucionpublicado.ErrNoDisponible)
	intentos, err := s.historial.IntentosDeUnAmbiente(ctx, "prod")
	require.NoError(t, err)
	require.Len(t, intentos, 1)
	require.True(t, intentos[0].Abandonado, "el intento que no llegó a empezar se abandona")
	require.Empty(t, intentos[0].Registros, "no ejecutó ningún paso")
	require.True(t, intentos[0].SinDesenlace(), "no es un intento fallido: nunca ejecutó nada")

	require.NoError(t, os.Remove(bloqueo))
	resultado, err := s.borde.Intentar(ctx, s.peticion(t))
	require.NoError(t, err, "el ambiente quedó libre")
	require.Equal(t, "exitoso", string(resultado.Estado))
}

// Lo que un comando imprime se guarda aparte y no queda en ningún registro del intento: lo que se busca en
// ellos es lo que de verdad se imprimió, línea a línea, no un texto escrito a mano.
func TestE2E_LoQueImprimeUnComandoNoQuedaEnNingunRegistro(t *testing.T) {
	s := montarSistema(t)
	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t))
	require.NoError(t, err)
	salidas, err := s.historial.SalidasDeUnIntento(context.Background(), resultado.Intento, historialpublicado.TodasLasSalidas)
	require.NoError(t, err)
	var texto strings.Builder
	for _, salida := range salidas {
		texto.WriteString(salida.Texto)
	}

	impresas := lineasSignificativas(texto.String())
	require.NotEmpty(t, impresas, "la prueba solo vale si los comandos imprimieron algo")
	intento, err := s.historial.Intento(context.Background(), resultado.Intento)
	require.NoError(t, err)
	apertura := fmt.Sprintf("%+v", intento.Apertura)
	for _, linea := range impresas {
		for _, r := range intento.Registros {
			require.NotContains(t, string(r.Contenido.Datos), linea)
		}
		require.NotContains(t, apertura, linea)
	}
}

// lineasSignificativas son las líneas de la salida lo bastante largas para que encontrarlas en un registro no
// sea casualidad.
func lineasSignificativas(salida string) []string {
	const largoMinimo = 8
	var lineas []string
	for _, linea := range strings.Split(salida, "\n") {
		if linea = strings.TrimSpace(linea); len(linea) >= largoMinimo {
			lineas = append(lineas, linea)
		}
	}
	return lineas
}

// primerPaso es el nombre del primer paso del ejemplo, para las pruebas que abren un intento a mano.
func primerPaso(t *testing.T) string {
	t.Helper()
	return pasosDelEjemplo(t)[0].Nombre
}
