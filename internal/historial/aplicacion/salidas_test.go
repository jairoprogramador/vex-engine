package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

func TestRegistrarSalida_SeConsultaEnElOrdenEnQueTerminaronLosComandos(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	require.NoError(t, h.RegistrarSalida(ctx, id, "supply", "construir", true, "compilando\n"))
	require.NoError(t, h.RegistrarSalida(ctx, id, "deploy", "publicar", false, "falló\n"))

	salidas, err := h.SalidasDeUnIntento(ctx, id, publicado.TodasLasSalidas)

	require.NoError(t, err)
	require.Len(t, salidas, 2)
	require.Equal(t, publicado.Salida{Paso: "supply", Comando: "construir", Exitoso: true, Texto: "compilando\n", Instante: salidas[0].Instante}, salidas[0])
	require.Equal(t, "publicar", salidas[1].Comando)
	require.False(t, salidas[1].Exitoso)
	require.True(t, salidas[1].Instante.After(salidas[0].Instante))
}

func TestRegistrarSalida_NoCambiaLosRegistrosDelIntento(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	antes, err := h.Intento(ctx, id)
	require.NoError(t, err)

	require.NoError(t, h.RegistrarSalida(ctx, id, "supply", "construir", true, "texto"))

	despues, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.Equal(t, antes, despues)
}

func TestRegistrarSalida_ConservaLaSalidaTalCual(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	texto := "línea 1\n\x00\xff bytes raros \"comillas\"\n"

	require.NoError(t, h.RegistrarSalida(ctx, id, "supply", "construir", true, texto))

	salidas, err := h.SalidasDeUnIntento(ctx, id, publicado.TodasLasSalidas)
	require.NoError(t, err)
	require.Equal(t, texto, salidas[0].Texto)
}

func TestRegistrarSalida_Rechazos(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	require.ErrorIs(t, h.RegistrarSalida(ctx, id, "", "construir", true, "x"), publicado.ErrRechazado, "sin paso")
	require.ErrorIs(t, h.RegistrarSalida(ctx, id, "supply", "", true, "x"), publicado.ErrRechazado, "sin comando")
	require.ErrorIs(t, h.RegistrarSalida(ctx, "no-existe", "supply", "construir", true, "x"), publicado.ErrNoExiste, "sin intento")
}

func TestSalidasDeUnIntento_FiltraPorComoTerminoElComando(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarSalida(ctx, id, "supply", "bien-1", true, "a"))
	require.NoError(t, h.RegistrarSalida(ctx, id, "supply", "mal", false, "b"))
	require.NoError(t, h.RegistrarSalida(ctx, id, "deploy", "bien-2", true, "c"))

	comandos := func(filtro publicado.FiltroDeSalidas) []string {
		salidas, err := h.SalidasDeUnIntento(ctx, id, filtro)
		require.NoError(t, err)
		var nombres []string
		for _, s := range salidas {
			nombres = append(nombres, s.Comando)
		}
		return nombres
	}

	require.Equal(t, []string{"bien-1", "mal", "bien-2"}, comandos(publicado.TodasLasSalidas))
	require.Equal(t, []string{"bien-1", "bien-2"}, comandos(publicado.SoloLasExitosas))
	require.Equal(t, []string{"mal"}, comandos(publicado.SoloLasFallidas))
}

func TestSalidasDeUnIntento_UnIntentoSinSalidasEsUnaListaVacia(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	salidas, err := h.SalidasDeUnIntento(ctx, id, publicado.TodasLasSalidas)

	require.NoError(t, err)
	require.Empty(t, salidas)
}

func TestSalidasDeUnIntento_UnIntentoQueNoExisteSeDice(t *testing.T) {
	h, ctx := nuevoHistorial()

	_, err := h.SalidasDeUnIntento(ctx, "no-existe", publicado.TodasLasSalidas)

	require.ErrorIs(t, err, publicado.ErrNoExiste)
}

func TestSalidasDeUnIntento_CadaIntentoTieneLasSuyas(t *testing.T) {
	h, ctx := nuevoHistorial()
	uno, err := h.AbrirIntento(ctx, apertura("dev"))
	require.NoError(t, err)
	otro, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarSalida(ctx, uno, "supply", "de-uno", true, "1"))
	require.NoError(t, h.RegistrarSalida(ctx, otro, "supply", "de-otro", true, "2"))

	salidas, err := h.SalidasDeUnIntento(ctx, uno, publicado.TodasLasSalidas)

	require.NoError(t, err)
	require.Len(t, salidas, 1)
	require.Equal(t, "de-uno", salidas[0].Comando)
}

func TestUltimoIntento_EsElUltimoQueSeAbrioEnCualquierAmbiente(t *testing.T) {
	h, ctx := nuevoHistorial()
	_, hay, err := h.UltimoIntento(ctx)
	require.NoError(t, err)
	require.False(t, hay, "sin intentos")

	_, err = h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	siguiente, err := h.AbrirIntento(ctx, apertura("dev"))
	require.NoError(t, err)

	ultimo, hay, err := h.UltimoIntento(ctx)

	require.NoError(t, err)
	require.True(t, hay)
	require.Equal(t, siguiente, ultimo.Id)
	require.Equal(t, "dev", ultimo.Apertura.Ambiente)
}

func TestRegistrarSalida_VariasSalidasSeLeenDesdeOtroServicioSobreElMismoAlmacen(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	h, ctx := historialSobre(almacen), context.Background()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	for _, comando := range []string{"a", "b", "c"} {
		require.NoError(t, h.RegistrarSalida(ctx, id, "supply", comando, true, comando))
	}

	salidas, err := historialSobre(almacen).SalidasDeUnIntento(ctx, id, publicado.TodasLasSalidas)
	require.NoError(t, err)
	require.Len(t, salidas, 3)
}
