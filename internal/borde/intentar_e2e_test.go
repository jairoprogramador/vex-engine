package borde_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	ejecucionServicio := ejecucionaplicacion.NuevoServicio(ejecucionaplicacion.Dependencias{
		Pipelines:             ejecucioninfraestructura.NuevosPipelines(definicionServicio),
		Fuentes:               ejecucioninfraestructura.NuevasFuentes(suministroServicio),
		Variables:             ejecucioninfraestructura.NuevasVariables(resolucionServicio.ParaEjecucion()),
		Historial:             ejecucioninfraestructura.NuevoHistorial(historialServicio),
		Comandos:              ejecucioninfraestructura.NuevosComandos(),
		EspacioDeTrabajo:      ejecucioninfraestructura.NuevoEspacioDeTrabajo(raizDelEspacio),
		NombreDeLaHerramienta: "vexd-e2e",
	})

	return &sistema{
		borde:        borde.NuevoServicio(ejecucionServicio),
		historial:    historialServicio,
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

// salidaDePrueba recoge en orden todo lo que llega por paso, para comprobar el streaming en vivo (DEC-12.5).
type salidaDePrueba struct {
	mu       sync.Mutex
	recibido bytes.Buffer
}

func (s *salidaDePrueba) Escribir(paso string, datos []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recibido.WriteString(paso + ":")
	s.recibido.Write(datos)
	return nil
}

func (s *salidaDePrueba) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recibido.String()
}

func TestE2E_UnPrimerIntentoLlegaADespliegue(t *testing.T) {
	s := montarSistema(t)
	salida := &salidaDePrueba{}

	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t), salida)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.NotEmpty(t, resultado.Despliegue)
	require.Contains(t, salida.String(), "hola vex-demo", "la plantilla se interpoló con el metadato del proyecto")
	require.Contains(t, salida.String(), "etiqueta=v1.0.0")
	require.Contains(t, salida.String(), "desplegando v1.0.0 en prod")
}

func TestE2E_UnSegundoIntentoSinCambiosNoReejecutaNada(t *testing.T) {
	s := montarSistema(t)
	primero, err := s.borde.Intentar(context.Background(), s.peticion(t), &salidaDePrueba{})
	require.NoError(t, err)
	require.Equal(t, "exitoso", primero.Estado)

	salida := &salidaDePrueba{}
	segundo, err := s.borde.Intentar(context.Background(), s.peticion(t), salida)

	require.NoError(t, err)
	require.Equal(t, "exitoso", segundo.Estado)
	require.Empty(t, salida.String(), "ningún comando debería haberse ejecutado")
}

func TestE2E_CambiarUnaPlantillaReejecutaSoloEsePaso(t *testing.T) {
	s := montarSistema(t)
	_, err := s.borde.Intentar(context.Background(), s.peticion(t), &salidaDePrueba{})
	require.NoError(t, err)

	repo, err := git.PlainOpen(s.repoPipeline)
	require.NoError(t, err)
	escribirFichero(t, s.repoPipeline, "steps/01-preparar/plantilla.txt", "hola de nuevo ${var.project_name}\n")
	commitear(t, repo)

	salida := &salidaDePrueba{}
	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t), salida)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Contains(t, salida.String(), "hola de nuevo vex-demo", "el paso con la plantilla cambiada sí se re-ejecuta")
	require.Contains(t, salida.String(), "etiqueta=v1.0.0", "el mismo paso re-ejecuta TODOS sus comandos, no solo el de la plantilla")
	require.NotContains(t, salida.String(), "desplegando", "el otro paso no debería re-ejecutarse: sus instrucciones no cambiaron")
}

func TestE2E_UnIntentoConCopiaDeTrabajoNuncaLlegaADespliegue(t *testing.T) {
	s := montarSistema(t)
	peticion := s.peticion(t)
	peticion.CopiaDeTrabajo = s.repoProyecto

	resultado, err := s.borde.Intentar(context.Background(), peticion, &salidaDePrueba{})

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Empty(t, resultado.Despliegue)
}

func TestE2E_UnSegundoIntentoEnElMismoAmbienteSeRechaza(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	_, err := s.historial.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: "prod", Solicitante: "otro", Pasos: []historialpublicado.PasoDeclarado{{Nombre: "01-preparar"}},
		HastaPaso: "01-preparar", HashDelCodigo: "h", ConCommits: true,
	})
	require.NoError(t, err)

	_, err = s.borde.Intentar(ctx, s.peticion(t), &salidaDePrueba{})

	require.Error(t, err)
	var ocupado *historialpublicado.AmbienteOcupadoError
	require.True(t, errors.As(err, &ocupado), "se esperaba un *AmbienteOcupadoError, se obtuvo: %v", err)
}

func TestE2E_LoQueImprimeUnComandoNoQuedaEnNingunRegistro(t *testing.T) {
	s := montarSistema(t)
	resultado, err := s.borde.Intentar(context.Background(), s.peticion(t), &salidaDePrueba{})
	require.NoError(t, err)

	intento, err := s.historial.Intento(context.Background(), resultado.Intento)
	require.NoError(t, err)
	for _, r := range intento.Registros {
		require.NotContains(t, string(r.Contenido.Datos), "etiqueta=v1.0.0")
		require.NotContains(t, string(r.Contenido.Datos), "hola vex-demo")
	}
	require.NotContains(t, fmt.Sprintf("%+v", intento.Apertura), "etiqueta=v1.0.0")
}
