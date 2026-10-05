package aplicacion_test

import (
	"context"
	"testing"

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

func TestIntentar_UnHistorialQueNoAceptaUnRegistroSeDetieneSinCerrar(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.historial.fallarRegistrar["02-despliegue"] = true
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.Error(t, err)
	require.False(t, d.historial.cerrado, "EJ-4: el intento queda sin desenlace, nunca se cierra")
	require.Len(t, d.comandos.llamados, 1, "el primer paso sí llegó a ejecutarse; el segundo ni empezó, porque su propio comienzo no se pudo escribir")
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
	require.False(t, d.historial.cerrado)
}
