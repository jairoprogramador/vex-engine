package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

func nuevoServicio(pipeline dominio.Pipeline, err error) (*aplicacion.Servicio, *variablesFalsas, *espacioTemporalFalso) {
	variables := nuevasVariablesFalsas()
	espacio := &espacioTemporalFalso{}
	s := aplicacion.NuevoServicio(aplicacion.Dependencias{
		Pipelines:       &pipelinesFalsos{pipeline: pipeline, err: err},
		Variables:       variables,
		EspacioTemporal: espacio,
	})
	return s, variables, espacio
}

func TestSimularCuandoLaComprobacionFalla(t *testing.T) {
	fallos := definicionpublicado.NuevosFallosDeComprobacion(
		[]definicionpublicado.Fallo{{Invariante: "formato", Fichero: "config.yaml", Detalle: `schema_version "2" no se lee`}},
		"",
	)
	s, _, espacio := nuevoServicio(dominio.Pipeline{}, fallos)

	informe, err := s.Simular(context.Background(), publicado.PeticionDeSimulacion{Fuente: "p", Commit: "c1"})
	require.NoError(t, err)
	require.False(t, informe.Comprobacion.Paso)
	require.Equal(t, []publicado.Fallo{{Invariante: "formato", Fichero: "config.yaml", Detalle: `schema_version "2" no se lee`}}, informe.Comprobacion.Fallos)
	require.Empty(t, informe.Ambientes)
	require.Zero(t, espacio.contador(), "ningún ambiente se recorre si la comprobación falla")
}

func pipelineDeUnAmbiente(nombre, valor string, variables []dominio.VariableDeclarada, pasos []dominio.Paso) dominio.Pipeline {
	return dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: nombre, Valor: valor}},
		Pasos:     pasos,
		Variables: variables,
	}
}

func TestSimularRecorreTodosLosPasosYFabricaSalidasQueCumplenSuExpresion(t *testing.T) {
	ambito, err := dominio.AmbitoDeAmbiente("sand")
	require.NoError(t, err)
	pipeline := pipelineDeUnAmbiente("sandbox", "sand",
		[]dominio.VariableDeclarada{{Nombre: "sku", Ambito: dominio.AmbitoCompartido(), Valor: "Basic"}},
		[]dominio.Paso{{
			Nombre: "registro",
			Comandos: []dominio.Comando{{
				Nombre: "crear", Linea: "crear ${var.sku}",
				Salidas: []dominio.VariableDeSalida{{Nombre: "registro", Expresion: "v[0-9]+"}},
			}},
		}},
	)
	s, variables, espacio := nuevoServicio(pipeline, nil)

	informe, err := s.Simular(context.Background(), publicado.PeticionDeSimulacion{Fuente: "p", Commit: "c1"})
	require.NoError(t, err)
	require.True(t, informe.Comprobacion.Paso)
	require.Len(t, informe.Ambientes, 1)
	require.Equal(t, "sandbox", informe.Ambientes[0].Ambiente)
	require.Len(t, informe.Ambientes[0].Pasos, 1)
	require.Equal(t, []string{"registro"}, informe.Ambientes[0].Pasos[0].Interpolado)
	require.Empty(t, informe.Ambientes[0].Pasos[0].Faltante)
	require.Equal(t, 1, espacio.contador())

	valor, ok := variables.valores[clave{"sim-1", ambito.String(), "registro"}]
	require.True(t, ok)
	require.Regexp(t, "^v[0-9]+$", valor)
}

func TestSimularAislaLasVariablesDeCadaAmbiente(t *testing.T) {
	pipeline := dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: "sandbox", Valor: "sand"}, {Nombre: "produccion", Valor: "prod"}},
		Pasos: []dominio.Paso{{
			Nombre:   "desplegar",
			Comandos: []dominio.Comando{{Nombre: "aplicar", Linea: "replicas=${var.replicas}"}},
		}},
		Variables: []dominio.VariableDeclarada{
			{Nombre: "replicas", Ambito: mustAmbito(t, "sand"), Valor: "3"},
			{Nombre: "replicas", Ambito: mustAmbito(t, "prod"), Valor: "5"},
		},
	}
	s, variables, espacio := nuevoServicio(pipeline, nil)

	informe, err := s.Simular(context.Background(), publicado.PeticionDeSimulacion{Fuente: "p", Commit: "c1"})
	require.NoError(t, err)
	require.True(t, informe.Comprobacion.Paso)
	require.Equal(t, 2, espacio.contador(), "un id de simulación por ambiente")
	require.ElementsMatch(t, []string{
		"sim-1:sand:replicas=3",
		"sim-2:prod:replicas=5",
	}, variables.interpolaciones, "cada ambiente interpola con su propio valor, sin fuga entre ambos")
	require.ElementsMatch(t, []string{"sim-1", "sim-2"}, variables.cerradas, "cada simulación de ambiente se cierra")
}

func TestSimularCuandoUnaVariableNoSeResuelveElAmbienteSeAbandonaYSigueElSiguiente(t *testing.T) {
	pipeline := dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: "sandbox", Valor: "sand"}, {Nombre: "produccion", Valor: "prod"}},
		Pasos: []dominio.Paso{
			{
				Nombre:   "roto",
				Material: []dominio.Fichero{{Ruta: "d.yaml", Contenido: "a: ${var.no_declarada}", Plantilla: true}},
				Comandos: []dominio.Comando{{Nombre: "aplicar", Linea: "usa ${var.tampoco_esta}"}},
			},
			{Nombre: "nunca_llega", Comandos: []dominio.Comando{{Nombre: "final", Linea: "listo"}}},
		},
	}
	s, _, espacio := nuevoServicio(pipeline, nil)

	informe, err := s.Simular(context.Background(), publicado.PeticionDeSimulacion{Fuente: "p", Commit: "c1"})
	require.NoError(t, err)
	require.True(t, informe.Comprobacion.Paso)
	require.Len(t, informe.Ambientes, 2, "el segundo ambiente se procesa igual")

	primero := informe.Ambientes[0]
	require.Equal(t, "sandbox", primero.Ambiente)
	require.Len(t, primero.Pasos, 1, "el paso roto es el último: el resto del ambiente se abandona")
	require.ElementsMatch(t, []string{"no_declarada", "tampoco_esta"}, primero.Pasos[0].Faltante, "se acumulan todos los nombres que faltan en el paso")

	segundo := informe.Ambientes[1]
	require.Equal(t, "produccion", segundo.Ambiente)
	require.Len(t, segundo.Pasos, 1)
	require.ElementsMatch(t, []string{"no_declarada", "tampoco_esta"}, segundo.Pasos[0].Faltante, "el mismo pipeline falla igual en el segundo ambiente")
	require.Equal(t, 2, espacio.contador())
}

func mustAmbito(t *testing.T, ambiente string) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente(ambiente)
	require.NoError(t, err)
	return a
}
