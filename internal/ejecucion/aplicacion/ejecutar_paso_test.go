package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// jwtDePrueba tiene la forma de un JWT, y no es de nadie.
const jwtDePrueba = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJwYWdvcy1hcGkifQ.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func TestEjecutarPaso_UnJWTEnLaSalidaSeGuardaRedactado(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.comandos.trozos = []string{"+ cat /var/run/token\n" + jwtDePrueba + "\nlisto\n"}

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Len(t, d.historial.salidas, 1)
	require.Equal(t, "+ cat /var/run/token\n[REDACTADO]\nlisto\n", d.historial.salidas[0].texto,
		"solo cambia el token: el resto de la salida queda tal cual")
}

func TestEjecutarPaso_UnJWTPartidoEntreDosEscriturasTambienSeRedacta(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	// El comando lo escribe en tres veces, cortado en mitad de la cabecera y del cuerpo: redactar cada escritura
	// por separado lo dejaría pasar.
	d.comandos.trozos = []string{"token=" + jwtDePrueba[:9], jwtDePrueba[9:40], jwtDePrueba[40:] + "\n"}

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "token=[REDACTADO]\n", d.historial.salidas[0].texto)
}

func TestEjecutarPaso_LoQueElComandoCapturaConservaElValorReal(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.comandos.trozos = []string{"token=" + jwtDePrueba + "\n"}
	// Comandos captura con la salida sin redactar: un pipeline puede producir un token y usarlo en el comando
	// siguiente.
	d.comandos.resultado = dominio.ResultadoDeUnComando{
		Exitoso: true, Producidas: []dominio.VariableProducida{{Nombre: "token", Valor: jwtDePrueba}},
	}

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, []string{"01-pruebas:token=" + jwtDePrueba}, d.variables.producidas, "a Resolución llega el valor real")
	require.NotContains(t, d.historial.salidas[0].texto, "eyJ", "y al Historial, la salida redactada")
	require.Contains(t, d.historial.salidas[0].texto, dominio.Redactado)
}

func TestEjecutarPaso_LaSalidaDeUnComandoFallidoOCanceladoTambienSeRedacta(t *testing.T) {
	casos := map[string]func(*dependenciasDePrueba) context.Context{
		"fallido": func(d *dependenciasDePrueba) context.Context {
			d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}
			return context.Background()
		},
		"cancelado": func(d *dependenciasDePrueba) context.Context {
			ctx, cancelar := context.WithCancel(context.Background())
			d.comandos.alEjecutar = cancelar
			d.comandos.err = context.Canceled
			return ctx
		},
	}
	for nombre, preparar := range casos {
		t.Run(nombre, func(t *testing.T) {
			deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
			d.comandos.trozos = []string{"error: el token " + jwtDePrueba + " no vale\n"}
			ctx := preparar(d)

			resultado, err := aplicacion.NuevoServicio(deps).Intentar(ctx, peticionDePrueba(), nil)

			require.NoError(t, err)
			require.Equal(t, nombre, resultado.Estado)
			require.Len(t, d.historial.salidas, 1)
			require.False(t, d.historial.salidas[0].exitoso)
			require.Equal(t, "error: el token [REDACTADO] no vale\n", d.historial.salidas[0].texto)
		})
	}
}
