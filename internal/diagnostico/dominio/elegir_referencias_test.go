package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func TestElegirReferencias(t *testing.T) {
	usuario := despliegueDePrueba(t, "d-usuario")
	mismo := despliegueDePrueba(t, "d-mismo")
	anterior := despliegueDePrueba(t, "d-anterior")

	casos := map[string]struct {
		candidatos dominio.CandidatosParaElegirReferencias
		esperadas  []dominio.CandidataDeReferencia
	}{
		"el usuario elige y gana solo, aunque haya defaults": {
			candidatos: dominio.CandidatosParaElegirReferencias{
				ElegidaPorElUsuario: &usuario, UltimoDelMismoAmbiente: &mismo, UltimoDelAmbienteAnteriorConMismoCodigo: &anterior,
			},
			esperadas: []dominio.CandidataDeReferencia{{Despliegue: usuario, Razon: dominio.ElegidaPorElUsuario}},
		},
		"solo el mismo ambiente": {
			candidatos: dominio.CandidatosParaElegirReferencias{UltimoDelMismoAmbiente: &mismo},
			esperadas:  []dominio.CandidataDeReferencia{{Despliegue: mismo, Razon: dominio.MismoAmbiente}},
		},
		"solo el ambiente anterior": {
			candidatos: dominio.CandidatosParaElegirReferencias{UltimoDelAmbienteAnteriorConMismoCodigo: &anterior},
			esperadas:  []dominio.CandidataDeReferencia{{Despliegue: anterior, Razon: dominio.AmbienteAnterior}},
		},
		"las dos por defecto": {
			candidatos: dominio.CandidatosParaElegirReferencias{
				UltimoDelMismoAmbiente: &mismo, UltimoDelAmbienteAnteriorConMismoCodigo: &anterior,
			},
			esperadas: []dominio.CandidataDeReferencia{
				{Despliegue: mismo, Razon: dominio.MismoAmbiente},
				{Despliegue: anterior, Razon: dominio.AmbienteAnterior},
			},
		},
		"ninguna referencia, ES-6": {
			candidatos: dominio.CandidatosParaElegirReferencias{},
			esperadas:  nil,
		},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			require.Equal(t, caso.esperadas, dominio.ElegirReferencias(caso.candidatos))
		})
	}
}

func TestEncontrarAmbienteAnterior(t *testing.T) {
	sand := ambienteDePrueba(t, "sand")
	stag := ambienteDePrueba(t, "stag")
	prod := ambienteDePrueba(t, "prod")
	orden := []dominio.Ambiente{sand, stag, prod}

	casos := map[string]struct {
		actual   dominio.Ambiente
		esperado dominio.Ambiente
		hay      bool
	}{
		"el primero del orden no tiene anterior":        {actual: sand, hay: false},
		"el del medio tiene al primero":                 {actual: stag, esperado: sand, hay: true},
		"el último tiene al del medio":                  {actual: prod, esperado: stag, hay: true},
		"un ambiente fuera del orden no tiene anterior": {actual: ambienteDePrueba(t, "otro"), hay: false},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			resultado, hay := dominio.EncontrarAmbienteAnterior(orden, caso.actual)
			require.Equal(t, caso.hay, hay)
			if hay {
				require.Equal(t, caso.esperado, resultado)
			}
		})
	}
}

func despliegueDePrueba(t *testing.T, id string) dominio.DespliegueDeDiagnostico {
	t.Helper()
	idd, err := dominio.NuevoIdDespliegue(id)
	require.NoError(t, err)
	return dominio.DespliegueDeDiagnostico{Id: idd}
}

func ambienteDePrueba(t *testing.T, valor string) dominio.Ambiente {
	t.Helper()
	a, err := dominio.NuevaAmbiente(valor)
	require.NoError(t, err)
	return a
}
