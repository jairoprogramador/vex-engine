package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// decidirPaso junta los recursos de ahora — el hash del código, que ya trae el intento, el de las
// instrucciones, que calcula esta unidad, y si cambiaron las variables, que dice Resolución — con la última
// vez, que dice el Historial, e invoca dominio.DecidirPaso (DEC-09.3: el propio servicio de dominio no lee, no
// hashea y no escribe nada). Devuelve también los recursos de ahora: el registro que se escriba después de
// ejecutar (o de no re-ejecutar) los necesita.
//
// El contexto del intento se mezcla en la huella de variables, y solo en ella: como esa huella es la que se
// registra, la próxima comparación ya la lleva, y un paso que no mira variables no se entera.
func (s *Servicio) decidirPaso(
	ctx context.Context, intento string, paso dominio.PasoDeEjecucion, ambito dominio.Ambito, hashDelCodigo dominio.HashDeCodigo,
	contexto dominio.ContextoDeEjecucion,
) (dominio.Decision, dominio.RecursosDeUnPaso, error) {
	hashDeInstrucciones := dominio.CalcularHashDeInstrucciones(paso.Comandos, paso.Material)

	hashDeVariables, err := s.d.Variables.HashDeLasVariables(ctx, intento, ambito, dominio.TextosInterpolables(paso.Comandos, paso.Material))
	if err != nil {
		return dominio.Decision{}, dominio.RecursosDeUnPaso{}, fmt.Errorf("ejecución: decidir el paso %q: %w", paso.Nombre(), err)
	}
	ahora := dominio.NuevosRecursosDeUnPaso(hashDelCodigo, hashDeInstrucciones, contexto.ConLasVariables(hashDeVariables))

	ultimaVez, err := s.d.Historial.UltimaVezDeUnPaso(ctx, paso.Nombre(), ambito)
	if err != nil {
		return dominio.Decision{}, dominio.RecursosDeUnPaso{}, fmt.Errorf("ejecución: decidir el paso %q: %w", paso.Nombre(), err)
	}

	return dominio.DecidirPaso(paso.Regla, ahora, ultimaVez), ahora, nil
}
