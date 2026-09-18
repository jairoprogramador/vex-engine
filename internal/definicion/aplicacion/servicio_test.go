package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/definicion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
)

// Pipelines en memoria: devuelve lo que salga de comprobar una declaración, o un error. El servicio no puede
// devolver nada que no haya salido de la comprobación.
type pipelinesEnMemoria struct {
	declaracion dominio.PipelineDeclarado
	err         error
}

func (p pipelinesEnMemoria) DeHoy(context.Context, string) (*dominio.PipelineComprobado, error) {
	if p.err != nil {
		return nil, p.err
	}
	return dominio.Comprobar(p.declaracion)
}

func (p pipelinesEnMemoria) DeUnCommit(ctx context.Context, fuente, _ string) (*dominio.PipelineComprobado, error) {
	return p.DeHoy(ctx, fuente)
}

func declaracion() dominio.PipelineDeclarado {
	uno, valor, imagen := "1", "Basic", "${var.registro}"
	return dominio.PipelineDeclarado{
		Commit: "c1", Hash: "h1",
		Configuracion: dominio.Declarado[dominio.ConfiguracionDeclarada]{Existe: true, Datos: dominio.ConfiguracionDeclarada{
			Version: &uno,
			Pasos: map[string]dominio.ConfiguracionDePasoDeclarada{
				"registro": {Reglas: []string{"instructions"}, ReglasEscritas: true, Ambito: "shared"},
			},
		}},
		Ambientes: dominio.Declarado[[]dominio.AmbienteDeclarado]{Existe: true, Datos: []dominio.AmbienteDeclarado{{Nombre: "sandbox", Valor: "sand"}}},
		Pasos: []dominio.PasoDeclarado{
			{
				Directorio: "01-registro",
				Comandos: dominio.Declarado[[]dominio.ComandoDeclarado]{Existe: true, Datos: []dominio.ComandoDeclarado{{
					Nombre: "crear", Linea: "crear ${var.sku}",
					Variables: []dominio.VariableDeComandoDeclarada{
						{Nombre: "registro", Expresion: "(.+)", Ambito: "shared"},
						{Expresion: "listo"},
					},
				}}},
			},
			{
				Directorio: "02-despliegue",
				Comandos: dominio.Declarado[[]dominio.ComandoDeclarado]{Existe: true, Datos: []dominio.ComandoDeclarado{
					{Linea: "kubectl apply", Directorio: "k8s", Plantillas: []string{"d.yaml"}},
				}},
				Material: []dominio.FicheroDeclarado{{Ruta: "k8s/d.yaml", Contenido: "image: ${var.imagen}"}},
			},
		},
		Variables: []dominio.VariablesDePipelineDeclarada{
			{Fichero: "variables/compartidas.yaml", Variables: []dominio.VariableDePipelineDeclarada{{Nombre: "sku", Valor: &valor}}},
			{Fichero: "variables/sand/despliegue.yaml", Ambito: "sand", Variables: []dominio.VariableDePipelineDeclarada{
				{Nombre: "imagen", Valor: &imagen},
			}},
		},
	}
}

func TestTraeElPipelineComprobadoEnElLenguajePublicado(t *testing.T) {
	s := aplicacion.NuevoServicio(aplicacion.Dependencias{Pipelines: pipelinesEnMemoria{declaracion: declaracion()}})

	p, err := s.DeHoy(context.Background(), "pipeline")
	require.NoError(t, err)
	require.Equal(t, publicado.Pipeline{
		Version: "1", Commit: "c1", Hash: "h1",
		Ambientes: []publicado.Ambiente{{Nombre: "sandbox", Valor: "sand"}},
		Pasos: []publicado.Paso{
			{
				Nombre: "registro", Orden: 1, Reglas: []publicado.Regla{publicado.ReglaInstrucciones}, Compartido: true,
				Comandos: []publicado.Comando{{
					Nombre: "crear", Linea: "crear ${var.sku}",
					Salidas:    []publicado.VariableDeSalida{{Nombre: "registro", Expresion: "(.+)", Compartida: true}},
					Aserciones: []publicado.Asercion{{Expresion: "listo"}},
				}},
			},
			{
				Nombre: "despliegue", Orden: 2,
				Reglas: []publicado.Regla{publicado.ReglaCodigo, publicado.ReglaInstrucciones, publicado.ReglaVariables},
				Comandos: []publicado.Comando{{
					Linea: "kubectl apply", Directorio: "k8s", Plantillas: []string{"k8s/d.yaml"},
				}},
				Material: []publicado.Fichero{{Ruta: "k8s/d.yaml", Contenido: "image: ${var.imagen}", Plantilla: true}},
			},
		},
		Variables: []publicado.VariableDeclarada{
			{Nombre: "sku", Valor: "Basic"},
			{Nombre: "imagen", Ambito: "sand", Valor: "${var.registro}"},
		},
	}, p)
}

func TestLosFallosDeLaComprobacionLleganConSuLista(t *testing.T) {
	d := declaracion()
	dos := "2"
	d.Configuracion.Datos.Version = &dos
	s := aplicacion.NuevoServicio(aplicacion.Dependencias{Pipelines: pipelinesEnMemoria{declaracion: d}})

	p, err := s.DeUnCommit(context.Background(), "pipeline", "c1")
	require.Zero(t, p)
	require.ErrorIs(t, err, publicado.ErrNoComprobado)
	var fallos *publicado.FallosDeComprobacion
	require.True(t, errors.As(err, &fallos))
	require.Len(t, fallos.Fallos, 1)
	require.Equal(t, "formato", fallos.Fallos[0].Invariante)
	require.Equal(t, "config.yaml", fallos.Fallos[0].Fichero)
	require.Contains(t, err.Error(), `schema_version "2" no se lee`)
}

func TestLoQueNoExisteYLoQueNoSePuedePedir(t *testing.T) {
	ctx := context.Background()
	for _, caso := range []struct {
		dominio, publicado error
	}{
		{dominio.ErrNoExiste, publicado.ErrNoExiste},
		{dominio.ErrInvalido, publicado.ErrInvalido},
	} {
		causa := errors.Join(caso.dominio, errors.New("detalle"))
		s := aplicacion.NuevoServicio(aplicacion.Dependencias{Pipelines: pipelinesEnMemoria{err: causa}})
		_, err := s.DeHoy(ctx, "pipeline")
		require.ErrorIs(t, err, caso.publicado)
		require.Contains(t, err.Error(), "detalle", "sin perder su mensaje")
	}
}

func TestLasVariablesEstandarSePublicanSinValores(t *testing.T) {
	s := aplicacion.NuevoServicio(aplicacion.Dependencias{})
	variables := s.VariablesEstandar()
	require.Contains(t, variables, publicado.VariableEstandar{Nombre: "project_name", Metadato: true})
	require.Contains(t, variables, publicado.VariableEstandar{Nombre: "environment", Metadato: true})
	require.Contains(t, variables, publicado.VariableEstandar{Nombre: "project_hash"})
	require.Contains(t, variables, publicado.VariableEstandar{Nombre: "step_workdir", DelPaso: true})
}
