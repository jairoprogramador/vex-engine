package main

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	definicionaplicacion "github.com/jairoprogramador/vex-engine/internal/definicion/aplicacion"
	definicioninfraestructura "github.com/jairoprogramador/vex-engine/internal/definicion/infraestructura"
	diagnosticoaplicacion "github.com/jairoprogramador/vex-engine/internal/diagnostico/aplicacion"
	diagnosticoinfraestructura "github.com/jairoprogramador/vex-engine/internal/diagnostico/infraestructura"
	ejecucionaplicacion "github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	ejecucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	lanzamientoaplicacion "github.com/jairoprogramador/vex-engine/internal/lanzamiento/aplicacion"
	lanzamientoinfraestructura "github.com/jairoprogramador/vex-engine/internal/lanzamiento/infraestructura"
	resolucionaplicacion "github.com/jairoprogramador/vex-engine/internal/resolucion/aplicacion"
	resolucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/resolucion/infraestructura"
	simulacionaplicacion "github.com/jairoprogramador/vex-engine/internal/simulacion/aplicacion"
	simulacioninfraestructura "github.com/jairoprogramador/vex-engine/internal/simulacion/infraestructura"
	suministroaplicacion "github.com/jairoprogramador/vex-engine/internal/suministro/aplicacion"
	suministroinfraestructura "github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
)

// rutas son los tres lugares del disco que da quien invoca (DEC-06.18): el almacén del Historial, el espacio
// de trabajo de los ambientes y donde Suministro pone el material de las fuentes. Solo el almacén es la
// verdad; el material es una copia que se puede borrar sin que cambie ninguna decisión.
type rutas struct {
	almacen  string
	espacio  string
	material string
}

// componer es la raíz de composición: el único sitio que conoce todos los contextos y los conecta. Los de
// abajo (Historial, Suministro) se construyen primero; los de entrada, con lo publicado de los de abajo.
func componer(r rutas) (*borde.Servicio, error) {
	almacen, err := historialinfraestructura.NuevoAlmacenLocal(r.almacen)
	if err != nil {
		return nil, fmt.Errorf("el almacén del historial: %w", err)
	}
	historial := historialaplicacion.NuevoServicio(historialaplicacion.Dependencias{
		Intentos:     historialinfraestructura.NuevosIntentos(almacen),
		Despliegues:  historialinfraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  historialinfraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: historialinfraestructura.NuevosLanzamientos(almacen),
		Reservas:     historialinfraestructura.NuevasReservas(almacen),
		Reloj:        historialinfraestructura.RelojDelSistema{},
		Identidades:  historialinfraestructura.IdentidadesUUID{},
	})

	suministro := suministroaplicacion.NuevoServicio(suministroaplicacion.Dependencias{
		Repositorios: suministroinfraestructura.NuevosRepositoriosLocales(r.material),
		Hashes:       suministroinfraestructura.HashDeContenido{},
	})
	definicion := definicionaplicacion.NuevoServicio(definicionaplicacion.Dependencias{
		Pipelines: definicioninfraestructura.NuevosPipelinesDeSuministro(suministro),
	})
	resolucion := resolucionaplicacion.NuevoServicio(resolucionaplicacion.Dependencias{
		Historial:  resolucioninfraestructura.NuevoHistorial(historial, historial),
		Definicion: resolucioninfraestructura.NuevaDefinicion(definicion),
	})

	ejecucion := ejecucionaplicacion.NuevoServicio(ejecucionaplicacion.Dependencias{
		Pipelines:             ejecucioninfraestructura.NuevosPipelines(definicion),
		Fuentes:               ejecucioninfraestructura.NuevasFuentes(suministro),
		Variables:             ejecucioninfraestructura.NuevasVariables(resolucion.ParaEjecucion()),
		Historial:             ejecucioninfraestructura.NuevoHistorial(historial),
		Comandos:              ejecucioninfraestructura.NuevosComandos(),
		EspacioDeTrabajo:      ejecucioninfraestructura.NuevoEspacioDeTrabajo(r.espacio),
		NombreDeLaHerramienta: "vexd",
	})
	simulacion := simulacionaplicacion.NuevoServicio(simulacionaplicacion.Dependencias{
		Pipelines:       simulacioninfraestructura.NuevosPipelines(definicion),
		Variables:       simulacioninfraestructura.NuevasVariables(resolucion.ParaSimulacion()),
		EspacioTemporal: simulacioninfraestructura.EspacioTemporal{},
	})
	lanzamiento := lanzamientoaplicacion.NuevoServicio(lanzamientoaplicacion.Dependencias{
		Historial: lanzamientoinfraestructura.NuevoHistorial(historial),
	})
	// El Historial no sabe quién escucha «despliegue registrado» (DEC-05.4): lo registra esta raíz.
	historial.Escuchar(lanzamientoinfraestructura.NuevaEscucha(lanzamiento))
	diagnostico := diagnosticoaplicacion.NuevoServicio(diagnosticoaplicacion.Dependencias{
		Historial: diagnosticoinfraestructura.NuevoHistorial(historial),
	})

	return borde.NuevoServicio(borde.Dependencias{
		Ejecucion:   ejecucion,
		Simulacion:  simulacion,
		Lanzamiento: lanzamiento,
		Diagnostico: diagnostico,
		Historial:   historial,
	}), nil
}
