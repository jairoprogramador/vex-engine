package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func nombrePaso(t *testing.T, valor string) dominio.NombrePaso {
	t.Helper()
	p, err := dominio.NuevoNombrePaso(valor)
	require.NoError(t, err)
	return p
}

func nombreDeVariable(t *testing.T, valor string) dominio.NombreDeVariable {
	t.Helper()
	n, err := dominio.NuevoNombreDeVariable(valor)
	require.NoError(t, err)
	return n
}

func hashDelCodigo(t *testing.T, valor string) dominio.HashDelCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDelCodigo(valor)
	require.NoError(t, err)
	return h
}

func hashDeInstrucciones(t *testing.T, valor string) dominio.HashDeInstrucciones {
	t.Helper()
	h, err := dominio.NuevoHashDeInstrucciones(valor)
	require.NoError(t, err)
	return h
}

func hashDeVariable(t *testing.T, valor string) dominio.HashDeVariable {
	t.Helper()
	h, err := dominio.NuevoHashDeVariable(valor)
	require.NoError(t, err)
	return h
}

// ejesDeUnPasoDePrueba construye EjesDeUnPaso con código y variables por defecto ("c1" y ninguna
// declarada), para que cada prueba solo tenga que fijar lo que le importa.
func ejesDeUnPasoDePrueba(
	t *testing.T, paso, codigo, instrucciones string, declaradas map[string]string,
) dominio.EjesDeUnPaso {
	t.Helper()
	mapa := map[dominio.NombreDeVariable]dominio.HashDeVariable{}
	for nombre, hash := range declaradas {
		mapa[nombreDeVariable(t, nombre)] = hashDeVariable(t, hash)
	}
	return dominio.NuevosEjesDeUnPaso(
		nombrePaso(t, paso), hashDelCodigo(t, codigo), hashDeInstrucciones(t, instrucciones), mapa, false,
	)
}
