package main

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	catalogoaplicacion "github.com/jairoprogramador/vex-engine/internal/catalogo/aplicacion"
	cataloginfraestructura "github.com/jairoprogramador/vex-engine/internal/catalogo/infraestructura"
	definicionaplicacion "github.com/jairoprogramador/vex-engine/internal/definicion/aplicacion"
	definicioninfraestructura "github.com/jairoprogramador/vex-engine/internal/definicion/infraestructura"
	diagnosticoaplicacion "github.com/jairoprogramador/vex-engine/internal/diagnostico/aplicacion"
	diagnosticoinfraestructura "github.com/jairoprogramador/vex-engine/internal/diagnostico/infraestructura"
	ejecucionaplicacion "github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	ejecuciondominio "github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	ejecucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
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

// validar dice qué falta de la configuración del proceso para atender la operación: el almacén siempre, el
// espacio solo si la operación ejecuta comandos. El mensaje nombra la variable.
func (r rutas) validar(op operacion) error {
	if r.almacen == "" {
		return fmt.Errorf("%w: falta %s: dónde está el historial", errConfiguracion, nombreAlmacen)
	}
	if op.ejecutaComandos && r.espacio == "" {
		return fmt.Errorf("%w: falta %s: dónde trabajan los pasos de cada ambiente", errConfiguracion, nombreEspacio)
	}
	return nil
}

// componer es la raíz de composición: el único sitio que conoce todos los contextos y los conecta. Los de
// abajo (Historial, Suministro) se construyen primero; los de entrada, con lo publicado de los de abajo.
// progreso es a dónde cuenta Ejecución cómo avanza un intento.
func componer(r rutas, progreso ejecuciondominio.Progreso) (*borde.Servicio, error) {
	almacen, err := historialinfraestructura.NuevoAlmacenLocal(r.almacen)
	if err != nil {
		return nil, fmt.Errorf("%w: el almacén del historial (%s): %w", errConfiguracion, nombreAlmacen, err)
	}
	historial := historialaplicacion.NuevoServicio(historialaplicacion.Dependencias{
		Intentos:     historialinfraestructura.NuevosIntentos(almacen),
		Despliegues:  historialinfraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  historialinfraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: historialinfraestructura.NuevosLanzamientos(almacen),
		Reservas:     historialinfraestructura.NuevasReservas(almacen),
		Salidas:      historialinfraestructura.NuevasSalidas(almacen),
		Latidos:      historialinfraestructura.NuevosLatidos(almacen),
		Reloj:        historialinfraestructura.RelojDelSistema{},
		Identidades:  historialinfraestructura.IdentidadesUUID{},

		VentanaDeVida: historialpublicado.VentanaDeVida,
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
		Progreso:              progreso,
		NombreDeLaHerramienta: "vexd",
	})
	simulacion := simulacionaplicacion.NuevoServicio(simulacionaplicacion.Dependencias{
		Pipelines:       simulacioninfraestructura.NuevosPipelines(definicion),
		Variables:       simulacioninfraestructura.NuevasVariables(resolucion.ParaSimulacion()),
		EspacioTemporal: simulacioninfraestructura.EspacioTemporal{},
	})
	catalogo := catalogoaplicacion.NuevoServicio(catalogoaplicacion.Dependencias{
		Pipelines: cataloginfraestructura.NuevosPipelines(definicion),
		Reservas:  cataloginfraestructura.NuevasReservas(historial),
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
		Catalogo:    catalogo,
		Diagnostico: diagnostico,
		Historial:   historial,
	}), nil
}
