package dominio_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestSalidaDeComando_ConservaLoQueCabeTalCual(t *testing.T) {
	salida := dominio.NuevaSalidaDeComandoConLimite(10)

	n, err := salida.Write([]byte("hola "))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	_, _ = salida.Write([]byte("mundo"))

	require.False(t, salida.Truncada())
	require.Equal(t, "hola mundo", salida.Texto())
}

func TestSalidaDeComando_PasadoElLimiteDescartaSinDejarDeAceptar(t *testing.T) {
	salida := dominio.NuevaSalidaDeComandoConLimite(8)

	for range 1000 {
		n, err := salida.Write([]byte("0123456789"))
		require.NoError(t, err, "quien escribe no se entera del límite: la tubería se sigue vaciando")
		require.Equal(t, 10, n)
	}

	require.True(t, salida.Truncada())
	texto := salida.Texto()
	require.True(t, strings.HasPrefix(texto, "01234567\n[SALIDA TRUNCADA"), texto)
	require.Contains(t, texto, "9992 bytes más de los 8")
	require.Less(t, len(texto), 200, "lo descartado no ocupa memoria")
}

func TestSalidaDeComando_ElLimiteDelMotorEsDe16MiB(t *testing.T) {
	require.Equal(t, 16*1024*1024, dominio.LimiteDeSalidaDeUnComando)
	salida := dominio.NuevaSalidaDeComando()
	trozo := []byte(strings.Repeat("x", 1<<20))
	for range 17 {
		_, _ = salida.Write(trozo)
	}
	require.True(t, salida.Truncada())
	require.Len(t, salida.Texto(), dominio.LimiteDeSalidaDeUnComando+len("\n[SALIDA TRUNCADA: el comando escribió 1048576 bytes más de los 16777216 que se conservan]\n"))
}
