package aplicacion_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

type dependenciasDePrueba struct {
	pipelines        *pipelinesFalsos
	fuentes          *fuentesFalsas
	variables        *variablesFalsas
	historial        *historialFalso
	comandos         *comandosFalsos
	espacioDeTrabajo *espacioDeTrabajoFalso
	progreso         *progresoFalso
}

func nuevasDependenciasDePrueba(t *testing.T, nombresDePasos ...string) (aplicacion.Dependencias, *dependenciasDePrueba) {
	t.Helper()
	hashDelCodigo, err := dominio.NuevoHashDeCodigo("contenido-v1:abc")
	require.NoError(t, err)

	pasos := make([]dominio.PasoDeEjecucion, 0, len(nombresDePasos))
	for _, nombre := range nombresDePasos {
		pasos = append(pasos, pasoSimple(t, nombre, false))
	}

	d := &dependenciasDePrueba{
		pipelines:        &pipelinesFalsos{pipeline: dominio.Pipeline{Commit: "c-pipeline", Pasos: pasos}},
		fuentes:          &fuentesFalsas{material: dominio.Material{Directorio: "/material", Hash: hashDelCodigo, Commit: "c-proyecto"}},
		variables:        &variablesFalsas{},
		historial:        nuevoHistorialFalso(t),
		comandos:         &comandosFalsos{resultado: dominio.ResultadoDeUnComando{Exitoso: true}},
		espacioDeTrabajo: &espacioDeTrabajoFalso{},
		progreso:         &progresoFalso{},
	}
	return aplicacion.Dependencias{
		Pipelines: d.pipelines, Fuentes: d.fuentes, Variables: d.variables, Historial: d.historial,
		Comandos: d.comandos, EspacioDeTrabajo: d.espacioDeTrabajo, Progreso: d.progreso, NombreDeLaHerramienta: "vexd",
	}, d
}

func peticionDePrueba() publicado.PeticionDeIntento {
	return publicado.PeticionDeIntento{
		Version: "1", Ambiente: "prod", Solicitante: "ana",
		FuenteDelProyecto: "git@proyecto", FuenteDelPipeline: "git@pipeline",
	}
}

func TestIntentar_UnFalloDeUnComandoCierraElIntentoComoFallido(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "fallido", resultado.Estado)
	require.Len(t, d.comandos.llamados, 1, "un fallo cierra el intento: no empieza ningún paso más")
	require.True(t, d.historial.cerrado)
	require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
	require.Empty(t, d.historial.causaCerrada, "un comando que falla se explica por su propia salida: no lleva causa")
}

func TestIntentar_LaCancelacionMidComandoGanaAlFalloQueEllaMismaProvoca(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	ctx, cancelar := context.WithCancel(context.Background())
	d.comandos.alEjecutar = cancelar
	d.comandos.err = context.Canceled
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(ctx, peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "cancelado", resultado.Estado)
	require.Len(t, d.comandos.llamados, 1, "el segundo paso no debería ni empezar tras la cancelación")
	require.True(t, d.historial.cerrado)
	require.Equal(t, dominio.Cancelado, d.historial.desenlaceCerrado)
}

func TestIntentar_UnHistorialQueNoAceptaUnRegistroSeDetieneYCierraElIntentoComoFallido(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.historial.fallarRegistrar["02-despliegue"] = true
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.ErrorContains(t, err, "falla simulada de RegistrarComienzo", "el error original es lo que hay que contar")
	require.True(t, d.historial.cerrado, "un registro rechazado no puede dejar el ambiente ocupado")
	require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
	require.Empty(t, d.historial.abandonados, "el intento ya empezó: se cierra, no se abandona")
	require.Len(t, d.comandos.llamados, 1, "el primer paso sí llegó a ejecutarse; el segundo ni empezó, porque su propio comienzo no se pudo escribir")
}

func TestIntentar_EJ4_SiElAlmacenNoRespondeElIntentoQuedaSinDesenlace(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.historial.fallarRegistrar["02-despliegue"] = true
	d.historial.errCerrar = errors.New("el almacén no responde")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.ErrorContains(t, err, "falla simulada de RegistrarComienzo", "la causa original no se pierde")
	require.ErrorContains(t, err, "no se pudo cerrar como fallido")
	require.ErrorContains(t, err, "el almacén no responde")
	require.False(t, d.historial.cerrado, "EJ-4: si el almacén no responde, el intento queda sin desenlace")
}

func TestIntentar_UnErrorDeInterpolacionCierraElIntentoComoFallidoYDevuelveSuCausa(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.variables.errInterpolar = fmt.Errorf("la variable %q no está disponible: %w", "test", dominio.ErrRechazado)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.ErrorIs(t, err, publicado.ErrRechazado, "el cliente sigue viendo por qué falló")
	require.ErrorContains(t, err, `la variable "test" no está disponible`)
	require.True(t, d.historial.cerrado, "el ambiente no puede quedar ocupado por un error del pipeline")
	require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
	require.Equal(t, dominio.CausaError, d.historial.causaCerrada, "se cerró por un error, no por un comando")
	require.Equal(t, []registroDeHistorial{{"comienzo", "01-pruebas"}}, d.historial.registros,
		"el paso empezó y nunca terminó: es una forma de intento que Diagnóstico ya contempla")
	require.Empty(t, d.comandos.llamados)
	require.Empty(t, d.historial.abandonados)
}

func TestIntentar_UnErrorAntesDelPrimerPasoTambienCierraElIntento(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.variables.err = errors.New("no se pudieron declarar las variables")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.ErrorContains(t, err, "no se pudieron declarar las variables")
	require.True(t, d.historial.cerrado)
	require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
	require.Empty(t, d.historial.registros, "ningún paso llegó a empezar")
}

func TestIntentar_LaCancelacionGanaAUnErrorQueImpideSeguir(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	ctx, cancelar := context.WithCancel(context.Background())
	d.variables.alUsarVariables = cancelar
	d.variables.err = context.Canceled
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(ctx, peticionDePrueba(), nil)

	require.ErrorIs(t, err, context.Canceled)
	require.True(t, d.historial.cerrado, "cerrar no depende del ctx cancelado")
	require.Equal(t, dominio.Cancelado, d.historial.desenlaceCerrado)
	require.Empty(t, d.historial.causaCerrada, "la cancelación ya lo explica")
}

func TestIntentar_UnPasoConUltimaVezValidaYSinCambiosNoSeReejecuta(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	paso := pasoSimple(t, "01-pruebas", false)
	hashDeInstrucciones := dominio.CalcularHashDeInstrucciones(paso.Comandos, paso.Material)
	d.historial.ultimaVezPorPaso["01-pruebas"] = dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true,
		Recursos:  dominio.NuevosRecursosDeUnPaso(d.fuentes.material.Hash, hashDeInstrucciones, dominio.NuevoHashDeVariables(hashDeVariablesDePrueba)),
		Evidencia: dominio.Evidencia{Intento: "int-viejo", Paso: "01-pruebas"},
	}
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Empty(t, d.comandos.llamados, "no debería ejecutarse ningún comando")
	require.Equal(t, []string{"01-pruebas"}, d.variables.noReejecutados)
	require.Equal(t, []registroDeHistorial{{"no-reejecucion", "01-pruebas"}}, d.historial.registros)
}

func TestIntentar_UnPasoNoEmpiezaSinElRegistroDelAnteriorEscrito(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	// Que RegistrarComienzo del primer paso falle basta para que el segundo nunca se intente: el bucle
	// propaga el error antes de volver a pedir el siguiente paso.
	d.historial.fallarRegistrar["01-pruebas"] = true
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.Error(t, err)
	require.Empty(t, d.comandos.llamados, "el comienzo del primer paso no se escribió: no debería ni ejecutarse")
	require.True(t, d.historial.cerrado, "el intento ya estaba abierto: se cierra como fallido y libera el ambiente")
	require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
}

func TestIntentar_LateMientrasCorreUnComandoLargoYDejaDeLatirAlTerminar(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.intervaloDeLatido = 5 * time.Millisecond
	d.comandos.alEjecutar = func() { time.Sleep(80 * time.Millisecond) } // un comando que no escribe nada
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	latidos := d.historial.cantidadDeLatidos()
	require.GreaterOrEqual(t, latidos, 5, "el latido no depende de que el comando escriba: lo escribe una rutina aparte")
	time.Sleep(40 * time.Millisecond)
	require.Equal(t, latidos, d.historial.cantidadDeLatidos(), "ningún latido se escribe después de cerrar el intento")
}

func TestIntentar_ElLatidoEmpiezaAlAbrirAntesDeRehacerElEspacio(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.intervaloDeLatido = time.Hour // solo el latido inmediato
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, 1, d.historial.cantidadDeLatidos(), "late nada más abrir, aunque el intervalo no haya pasado")
}

func TestIntentar_SinIntervaloNoLate(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Zero(t, d.historial.cantidadDeLatidos())
}

func TestIntentar_UnLatidoQueFallaNoDetieneElIntento(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.intervaloDeLatido = 5 * time.Millisecond
	d.historial.errLatir = errors.New("almacén lento")
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
}

func TestIntentar_SiSeAbandonaAntesDeEmpezarNoQuedaLatiendo(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.intervaloDeLatido = 5 * time.Millisecond
	d.espacioDeTrabajo.errRehacer = fmt.Errorf("disco lleno: %w", dominio.ErrNoDisponible)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.Error(t, err)
	latidos := d.historial.cantidadDeLatidos()
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, latidos, d.historial.cantidadDeLatidos(), "un intento abandonado deja de latir")
}
