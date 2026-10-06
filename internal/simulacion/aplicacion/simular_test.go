package aplicacion_test

import (
	"context"
	"errors"
	"fmt"
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

func peticion(ambiente, hastaPaso string) publicado.PeticionDeSimulacion {
	return publicado.PeticionDeSimulacion{
		Ambiente: ambiente, Solicitante: "jailux", HastaPaso: hastaPaso, Fuente: "p", Commit: "c1",
	}
}

func TestSimularCuandoLaComprobacionFalla(t *testing.T) {
	fallos := definicionpublicado.NuevosFallosDeComprobacion(
		[]definicionpublicado.Fallo{{Invariante: "formato", Fichero: "config.yaml", Detalle: `schema_version "2" no se lee`}},
		"",
	)
	s, _, espacio := nuevoServicio(dominio.Pipeline{}, fallos)

	resultado, err := s.Simular(context.Background(), peticion("sand", "test"))
	require.NoError(t, err)
	require.Equal(t, publicado.Resultado{
		Ambiente: "sand", Solicitante: "jailux", HastaPaso: "test", Estado: publicado.EstadoFallido,
		Causa: &publicado.Causa{Fallos: []publicado.Fallo{{Invariante: "formato", Fichero: "config.yaml", Detalle: `schema_version "2" no se lee`}}},
	}, resultado)
	require.Zero(t, espacio.contador(), "nada se recorre si la comprobación falla")
}

func TestSimularLlevaLosErroresDelDominioAlLenguajePublicado(t *testing.T) {
	invalido := fmt.Errorf("%w: el pipeline de p@c1 no está", dominio.ErrInvalido)
	s, _, _ := nuevoServicio(dominio.Pipeline{}, invalido)

	_, err := s.Simular(context.Background(), peticion("sand", "test"))

	require.ErrorIs(t, err, publicado.ErrInvalido, "una fuente que no está donde se dice es una petición inválida")
	require.ErrorIs(t, err, dominio.ErrInvalido, "y sigue siendo el error de dominio que la causó")
	require.Equal(t, invalido.Error(), err.Error(), "sin cambiar lo que dice")
}

func TestSimularNoConvierteUnFalloDeVerdadEnUnaPeticionInvalida(t *testing.T) {
	roto := errors.New("el disco se llenó")
	s, _, _ := nuevoServicio(dominio.Pipeline{}, roto)

	_, err := s.Simular(context.Background(), peticion("sand", "test"))

	require.ErrorIs(t, err, roto)
	require.NotErrorIs(t, err, publicado.ErrInvalido)
}

func pipelineDeUnAmbiente(nombre, valor string, variables []dominio.VariableDeclarada, pasos []dominio.Paso) dominio.Pipeline {
	return dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: nombre, Valor: valor}},
		Pasos:     pasos,
		Variables: variables,
	}
}

func TestSimularRechazaLoQueElPipelineNoTiene(t *testing.T) {
	pipeline := pipelineDeUnAmbiente("sandbox", "sand", nil, []dominio.Paso{{Nombre: "test"}})
	s, _, espacio := nuevoServicio(pipeline, nil)

	casos := map[string]publicado.PeticionDeSimulacion{
		"ambiente que no es un valor del pipeline (el nombre no vale)": peticion("sandbox", "test"),
		"ambiente desconocido": peticion("prod", "test"),
		"paso desconocido":     peticion("sand", "deploy"),
		"sin ambiente":         peticion("", "test"),
		"sin paso":             peticion("sand", ""),
		"sin solicitante":      {Ambiente: "sand", HastaPaso: "test", Fuente: "p", Commit: "c1"},
	}
	for nombre, p := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := s.Simular(context.Background(), p)
			require.ErrorIs(t, err, publicado.ErrInvalido)
		})
	}
	require.Zero(t, espacio.contador(), "una petición inválida no recorre nada")
}

func TestSimularRecorreHastaElPasoYFabricaSalidasQueCumplenSuExpresion(t *testing.T) {
	ambito := mustAmbito(t, "sand")
	pipeline := pipelineDeUnAmbiente("sandbox", "sand",
		[]dominio.VariableDeclarada{{Nombre: "sku", Ambito: dominio.AmbitoCompartido(), Valor: "Basic"}},
		[]dominio.Paso{
			{
				Nombre: "registro",
				Comandos: []dominio.Comando{{
					Nombre: "crear", Linea: "crear ${var.sku}",
					Salidas: []dominio.VariableDeSalida{{Nombre: "registro", Expresion: "v[0-9]+"}},
				}},
			},
			{Nombre: "despues", Comandos: []dominio.Comando{{Nombre: "roto", Linea: "usa ${var.no_declarada}"}}},
		},
	)
	s, variables, espacio := nuevoServicio(pipeline, nil)

	resultado, err := s.Simular(context.Background(), peticion("sand", "registro"))
	require.NoError(t, err)
	require.Equal(t, publicado.Resultado{
		Ambiente: "sand", Solicitante: "jailux", HastaPaso: "registro", Estado: publicado.EstadoExitoso,
	}, resultado, "el paso roto queda después de HastaPaso: no se simula")
	require.Equal(t, 1, espacio.contador())

	valor, ok := variables.valores[clave{"sim-1", ambito.String(), "registro"}]
	require.True(t, ok)
	require.Regexp(t, "^v[0-9]+$", valor)
}

func TestSimularDeclaraLasVariablesEstandarComoUnIntento(t *testing.T) {
	pipeline := dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: "sandbox", Valor: "sand"}},
		Pasos: []dominio.Paso{{
			Nombre: "test",
			Material: []dominio.Fichero{{
				Ruta: "plantilla.txt", Contenido: "${var.project_name}/${var.environment}", Plantilla: true,
			}},
			Comandos: []dominio.Comando{{
				Nombre: "probar",
				Linea:  "${var.project_id} ${var.project_organization} ${var.project_team} ${var.project_hash} ${var.project_version} ${var.project_workdir} ${var.tool_name} ${var.step_name} ${var.step_workdir} ${var.etiqueta}",
			}},
		}},
		Variables: []dominio.VariableDeclarada{
			{Nombre: "etiqueta", Ambito: dominio.AmbitoCompartido(), Valor: "${var.project_name}-global"},
		},
	}
	s, variables, _ := nuevoServicio(pipeline, nil)
	p := peticion("sand", "test")
	p.Metadatos = publicado.Metadatos{ProjectId: "p1", ProjectName: "vex-demo", ProjectOrganization: "org", ProjectTeam: "eq"}

	resultado, err := s.Simular(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, publicado.EstadoExitoso, resultado.Estado, "ninguna estándar ni global queda sin declarar: %+v", resultado.Causa)

	require.Contains(t, variables.interpolaciones, "sim-1:sand:vex-demo/sand", "los metadatos y environment son los reales")
	require.Equal(t, "test", variables.valores[clave{"sim-1", mustAmbito(t, "sand").String(), "step_name"}], "las del paso viven en el ámbito del ambiente")
}

func TestSimularSoloRecorreElAmbientePedido(t *testing.T) {
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

	resultado, err := s.Simular(context.Background(), peticion("prod", "desplegar"))
	require.NoError(t, err)
	require.Equal(t, publicado.EstadoExitoso, resultado.Estado)
	require.Equal(t, 1, espacio.contador())
	require.Equal(t, []string{"sim-1:prod:replicas=5"}, variables.interpolaciones, "solo el ambiente pedido, con su propio valor")
	require.Equal(t, []string{"sim-1"}, variables.cerradas, "la simulación se cierra")
}

func TestSimularCuandoUnaVariableNoSeResuelveFallaConLaCausaDelPrimerPasoRoto(t *testing.T) {
	pipeline := dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: "sandbox", Valor: "sand"}},
		Pasos: []dominio.Paso{
			{
				Nombre:   "roto",
				Material: []dominio.Fichero{{Ruta: "d.yaml", Contenido: "a: ${var.no_declarada}", Plantilla: true}},
				Comandos: []dominio.Comando{{Nombre: "aplicar", Linea: "usa ${var.tampoco_esta}"}},
			},
			{Nombre: "nunca_llega", Comandos: []dominio.Comando{{Nombre: "final", Linea: "usa ${var.otra}"}}},
		},
	}
	s, variables, _ := nuevoServicio(pipeline, nil)

	resultado, err := s.Simular(context.Background(), peticion("sand", "nunca_llega"))
	require.NoError(t, err)
	require.Equal(t, publicado.EstadoFallido, resultado.Estado)
	require.NotNil(t, resultado.Causa)
	require.Empty(t, resultado.Causa.Fallos)
	require.Len(t, resultado.Causa.Faltante, 1, "el primer paso roto abandona el resto")
	require.Equal(t, "roto", resultado.Causa.Faltante[0].Paso)
	require.ElementsMatch(t, []string{"no_declarada", "tampoco_esta"}, resultado.Causa.Faltante[0].Variables, "se acumulan todos los nombres que faltan en el paso")
	require.Equal(t, []string{"sim-1"}, variables.cerradas, "la simulación se cierra aunque se abandone")
}

func mustAmbito(t *testing.T, ambiente string) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente(ambiente)
	require.NoError(t, err)
	return a
}
