package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// salidaDePrueba es una variable de salida declarada, marcada o no con show.
func salidaDePrueba(t *testing.T, nombre, descripcion string, mostrar bool) dominio.VariableDeSalidaDeclarada {
	t.Helper()
	salida, err := dominio.NuevaVariableDeSalidaDeclarada(nombre, nombre+`=(\S+)`, false)
	require.NoError(t, err)
	return salida.ConVisibilidad(mostrar, descripcion)
}

// pasoConSalidas es pasoSimple con un comando que declara esas variables de salida.
func pasoConSalidas(t *testing.T, nombre string, salidas ...dominio.VariableDeSalidaDeclarada) dominio.PasoDeEjecucion {
	t.Helper()
	paso := pasoSimple(t, nombre, false)
	comando, err := dominio.NuevoComandoDeclarado(nombre, "echo "+nombre, "", nil, salidas, nil)
	require.NoError(t, err)
	paso.Comandos = []dominio.ComandoDeclarado{comando}
	return paso
}

// ultimaVezValidaDe es lo que el Historial diría de ese paso si ya se hizo bien y no cambió.
func ultimaVezValidaDe(d *dependenciasDePrueba, paso dominio.PasoDeEjecucion) dominio.UltimaVezDeUnPaso {
	return dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true,
		Recursos: dominio.NuevosRecursosDeUnPaso(
			d.fuentes.material.Hash, dominio.CalcularHashDeInstrucciones(paso.Comandos, paso.Material),
			dominio.NuevoHashDeVariables(hashDeVariablesDePrueba)),
		Evidencia: dominio.Evidencia{Intento: "int-viejo", Paso: paso.Nombre()},
	}
}

// visiblesPorPaso son los visibles que llevó el paso_terminado de cada paso. Ningún otro evento puede llevarlos.
func visiblesPorPaso(t *testing.T, p *progresoFalso) map[string][]dominio.VariableVisible {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	porPaso := map[string][]dominio.VariableVisible{}
	for _, e := range p.eventos {
		if e.Tipo != dominio.PasoTerminado {
			require.Empty(t, e.Visibles, "solo paso_terminado lleva visibles: %s", e.Tipo)
			continue
		}
		porPaso[e.Paso] = e.Visibles
	}
	return porPaso
}

// conUnPasoVisible monta un pipeline de un paso, «supply», que produce app_url (con show) y grupo (sin show).
func conUnPasoVisible(t *testing.T) (aplicacion.Dependencias, *dependenciasDePrueba, dominio.PasoDeEjecucion) {
	t.Helper()
	deps, d := nuevasDependenciasDePrueba(t)
	paso := pasoConSalidas(t, "supply",
		salidaDePrueba(t, "app_url", "URL pública de la aplicación", true),
		salidaDePrueba(t, "grupo", "no se enseña", false))
	d.pipelines.pipeline.Pasos = []dominio.PasoDeEjecucion{paso}
	d.variables.interpolados = map[string]string{"${var.app_url}": "https://x.invalid", "${var.grupo}": "rg-1"}
	return deps, d, paso
}

var urlVisible = dominio.VariableVisible{Nombre: "app_url", Descripcion: "URL pública de la aplicación", Valor: "https://x.invalid"}

func TestVisibles_UnPasoEjecutadoLosEntregaEnElEventoYEnElResultado(t *testing.T) {
	deps, d, _ := conUnPasoVisible(t)

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Equal(t, map[string][]dominio.VariableVisible{"supply": {urlVisible}}, visiblesPorPaso(t, d.progreso),
		"solo la variable marcada con show, con su descripción y su valor")
	require.Equal(t, []publicado.VariableVisible{
		{Nombre: "app_url", Descripcion: "URL pública de la aplicación", Valor: "https://x.invalid"},
	}, resultado.Visibles)
}

func TestVisibles_UnPasoQueNoSeReejecutaTambienLosEntrega(t *testing.T) {
	deps, d, paso := conUnPasoVisible(t)
	d.historial.ultimaVezPorPaso["supply"] = ultimaVezValidaDe(d, paso)

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Empty(t, d.comandos.llamados, "el paso no se re-ejecutó")
	require.Equal(t, []string{"intento_iniciado", "paso_terminado:supply:precargado"}, d.progreso.descritos())
	require.Equal(t, map[string][]dominio.VariableVisible{"supply": {urlVisible}}, visiblesPorPaso(t, d.progreso),
		"la URL la produjo un intento anterior, y sigue siendo el resultado de este")
	require.Len(t, resultado.Visibles, 1)
	require.Equal(t, "https://x.invalid", resultado.Visibles[0].Valor)
}

func TestVisibles_UnPasoFallidoOCanceladoNoEntregaNinguno(t *testing.T) {
	casos := map[string]struct {
		preparar func(*dependenciasDePrueba) context.Context
		estado   string
	}{
		"fallido": {
			preparar: func(d *dependenciasDePrueba) context.Context {
				d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}
				return context.Background()
			},
			estado: "fallido",
		},
		"cancelado": {
			preparar: func(d *dependenciasDePrueba) context.Context {
				ctx, cancelar := context.WithCancel(context.Background())
				d.comandos.alEjecutar = cancelar
				d.comandos.err = context.Canceled
				return ctx
			},
			estado: "cancelado",
		},
	}
	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			deps, d, _ := conUnPasoVisible(t)
			ctx := caso.preparar(d)

			resultado, err := aplicacion.NuevoServicio(deps).Intentar(ctx, peticionDePrueba(), nil)

			require.NoError(t, err)
			require.Equal(t, caso.estado, resultado.Estado)
			require.Equal(t, map[string][]dominio.VariableVisible{"supply": nil}, visiblesPorPaso(t, d.progreso))
			require.Empty(t, resultado.Visibles)
		})
	}
}

func TestVisibles_SiElIntentoNoTerminaExitosoElResultadoNoLosLleva(t *testing.T) {
	deps, d, _ := conUnPasoVisible(t)
	d.pipelines.pipeline.Pasos = append(d.pipelines.pipeline.Pasos, pasoSimple(t, "deploy", false))
	// supply ya se hizo bien; deploy se ejecuta y falla.
	d.historial.ultimaVezPorPaso["supply"] = ultimaVezValidaDe(d, d.pipelines.pipeline.Pasos[0])
	d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "fallido", resultado.Estado)
	require.Equal(t, []dominio.VariableVisible{urlVisible}, visiblesPorPaso(t, d.progreso)["supply"],
		"el paso que terminó bien sí lo contó en su momento")
	require.Empty(t, resultado.Visibles, "el resultado de un intento fallido no enseña nada")
}

func TestVisibles_UnaVariableQueNoSePuedeInterpolarSeOmiteYElIntentoSigue(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t)
	d.pipelines.pipeline.Pasos = []dominio.PasoDeEjecucion{pasoConSalidas(t, "supply",
		salidaDePrueba(t, "rota", "", true), salidaDePrueba(t, "app_url", "", true))}
	d.variables.interpolados = map[string]string{"${var.app_url}": "https://x.invalid"}
	d.variables.errPorTexto = map[string]error{"${var.rota}": errors.New("la variable no está disponible")}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err, "enseñar un valor es una cortesía: no hace fallar el intento")
	require.Equal(t, "exitoso", resultado.Estado)
	require.Equal(t, []publicado.VariableVisible{{Nombre: "app_url", Valor: "https://x.invalid"}}, resultado.Visibles)
	require.True(t, d.historial.cerrado)
}

func TestVisibles_UnValorConFormaDeJWTSeEntregaRedactado(t *testing.T) {
	deps, d, _ := conUnPasoVisible(t)
	d.variables.interpolados["${var.app_url}"] = "https://x.invalid/?id_token=" + jwtDePrueba

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	const redactado = "https://x.invalid/?id_token=[REDACTADO]"
	require.Equal(t, redactado, visiblesPorPaso(t, d.progreso)["supply"][0].Valor, "en el evento")
	require.Equal(t, redactado, resultado.Visibles[0].Valor, "y en el resultado")
}

func TestVisibles_VariosPasosLosEntreganEnElOrdenEnQueTerminan(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t)
	d.pipelines.pipeline.Pasos = []dominio.PasoDeEjecucion{
		pasoConSalidas(t, "supply", salidaDePrueba(t, "app_url", "", true)),
		pasoSimple(t, "package", false),
		pasoConSalidas(t, "deploy", salidaDePrueba(t, "revision", "", true), salidaDePrueba(t, "panel", "", true)),
	}
	d.variables.interpolados = map[string]string{"${var.app_url}": "u", "${var.revision}": "r", "${var.panel}": "p"}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, []publicado.VariableVisible{
		{Nombre: "app_url", Valor: "u"}, {Nombre: "revision", Valor: "r"}, {Nombre: "panel", Valor: "p"},
	}, resultado.Visibles)
	require.Empty(t, visiblesPorPaso(t, d.progreso)["package"], "un paso sin show no lleva visibles")
}

func TestVisibles_SinNingunaVariableConShowElResultadoTraeUnaListaVacia(t *testing.T) {
	deps, _ := nuevasDependenciasDePrueba(t, "01-pruebas")

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.NotNil(t, resultado.Visibles, "en el cable es [], nunca null")
	require.Empty(t, resultado.Visibles)
}

func TestVisibles_UnRollbackTambienLosEntrega(t *testing.T) {
	deps, d, _ := conUnPasoVisible(t)
	d.historial.destinoParaRollback = destinoDePrueba(t)

	resultado, err := aplicacion.NuevoServicio(deps).HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, map[string][]dominio.VariableVisible{"supply": {urlVisible}}, visiblesPorPaso(t, d.progreso))
	require.Len(t, resultado.Visibles, 1)
}
