package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func TestElegirReferencias(t *testing.T) {
	intento := intentoDePrueba(t, "i-falla")
	usuario := despliegueDePrueba(t, "d-usuario", "i-usuario")
	mismo := despliegueDePrueba(t, "d-mismo", "i-mismo")
	delPropioIntento := despliegueDePrueba(t, "d-propio", "i-falla")

	casos := map[string]struct {
		candidatos dominio.CandidatosParaElegirReferencias
		esperadas  []dominio.CandidataDeReferencia
	}{
		"el usuario elige y gana solo, aunque haya default": {
			candidatos: dominio.CandidatosParaElegirReferencias{ElegidaPorElUsuario: &usuario, UltimoDelMismoAmbiente: &mismo},
			esperadas:  []dominio.CandidataDeReferencia{{Despliegue: usuario, Razon: dominio.ElegidaPorElUsuario}},
		},
		"solo el mismo ambiente": {
			candidatos: dominio.CandidatosParaElegirReferencias{UltimoDelMismoAmbiente: &mismo},
			esperadas:  []dominio.CandidataDeReferencia{{Despliegue: mismo, Razon: dominio.MismoAmbiente}},
		},
		"ninguna referencia, ES-6": {
			candidatos: dominio.CandidatosParaElegirReferencias{},
			esperadas:  nil,
		},
		"la elegida por el usuario es del propio intento, ES-6": {
			candidatos: dominio.CandidatosParaElegirReferencias{ElegidaPorElUsuario: &delPropioIntento, UltimoDelMismoAmbiente: &mismo},
			esperadas:  nil,
		},
		"la del mismo ambiente es del propio intento, ES-6": {
			candidatos: dominio.CandidatosParaElegirReferencias{UltimoDelMismoAmbiente: &delPropioIntento},
			esperadas:  nil,
		},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			require.Equal(t, caso.esperadas, dominio.ElegirReferencias(caso.candidatos, intento))
		})
	}
}

func despliegueDePrueba(t *testing.T, id, intento string) dominio.DespliegueDeDiagnostico {
	t.Helper()
	idd, err := dominio.NuevoIdDespliegue(id)
	require.NoError(t, err)
	return dominio.DespliegueDeDiagnostico{Id: idd, Intento: intentoDePrueba(t, intento)}
}

func intentoDePrueba(t *testing.T, id string) dominio.IdIntento {
	t.Helper()
	i, err := dominio.NuevoIdIntento(id)
	require.NoError(t, err)
	return i
}
