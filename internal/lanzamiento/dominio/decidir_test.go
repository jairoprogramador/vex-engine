package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

func TestDecidirSiLanzarEnNombreDelActorAusente(t *testing.T) {
	casos := map[string]struct {
		parametros dominio.ParametrosDecisionDeLanzamiento
		lanza      bool
	}{
		"reservado nunca lanza en su nombre, aunque nunca se haya lanzado nada": {
			parametros: dominio.ParametrosDecisionDeLanzamiento{
				Reservado: true, HayUltimoLanzamiento: false, DespliegueNuevo: idDespliegue(t, "d1"),
			},
			lanza: false,
		},
		"no reservado y nunca lanzado, lanza": {
			parametros: dominio.ParametrosDecisionDeLanzamiento{
				Reservado: false, HayUltimoLanzamiento: false, DespliegueNuevo: idDespliegue(t, "d1"),
			},
			lanza: true,
		},
		"no reservado y el último lanzamiento es de otro despliegue, lanza": {
			parametros: dominio.ParametrosDecisionDeLanzamiento{
				Reservado: false, HayUltimoLanzamiento: true,
				DespliegueDelUltimoLanzamiento: idDespliegue(t, "d0"), DespliegueNuevo: idDespliegue(t, "d1"),
			},
			lanza: true,
		},
		"no reservado y ya lanzado este mismo despliegue, no vuelve a lanzar": {
			parametros: dominio.ParametrosDecisionDeLanzamiento{
				Reservado: false, HayUltimoLanzamiento: true,
				DespliegueDelUltimoLanzamiento: idDespliegue(t, "d1"), DespliegueNuevo: idDespliegue(t, "d1"),
			},
			lanza: false,
		},
		"reservado con el mismo despliegue ya lanzado, tampoco lanza": {
			parametros: dominio.ParametrosDecisionDeLanzamiento{
				Reservado: true, HayUltimoLanzamiento: true,
				DespliegueDelUltimoLanzamiento: idDespliegue(t, "d1"), DespliegueNuevo: idDespliegue(t, "d1"),
			},
			lanza: false,
		},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			require.Equal(t, caso.lanza, dominio.DecidirSiLanzarEnNombreDelActorAusente(caso.parametros))
		})
	}
}
