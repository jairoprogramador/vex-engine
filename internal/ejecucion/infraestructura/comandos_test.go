package infraestructura_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

func comandoDeclarado(t *testing.T, linea string, salidas []dominio.VariableDeSalidaDeclarada, aserciones []dominio.AsercionDeclarada) dominio.ComandoDeclarado {
	t.Helper()
	c, err := dominio.NuevoComandoDeclarado("prueba", linea, "", nil, salidas, aserciones)
	require.NoError(t, err)
	return c
}

func TestComandos_UnComandoExitosoLlegaALaSalidaEnVivo(t *testing.T) {
	var recibido bytes.Buffer
	comandos := infraestructura.NuevosComandos()
	linea := "echo marcador-de-salida"

	resultado, err := comandos.Ejecutar(context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), &recibido)

	require.NoError(t, err)
	require.True(t, resultado.Exitoso)
	require.Contains(t, recibido.String(), "marcador-de-salida")
}

func TestComandos_CorreLaLineaInterpoladaYNoLaDeclarada(t *testing.T) {
	var recibido bytes.Buffer
	comandos := infraestructura.NuevosComandos()

	resultado, err := comandos.Ejecutar(
		context.Background(), t.TempDir(), "echo interpolada",
		comandoDeclarado(t, "echo ${var.no-deberia-correr}", nil, nil), &recibido,
	)

	require.NoError(t, err)
	require.True(t, resultado.Exitoso)
	require.Contains(t, recibido.String(), "interpolada")
}

func TestComandos_UnComandoQueSaleConErrorNoEsExitoso(t *testing.T) {
	comandos := infraestructura.NuevosComandos()
	linea := "exit 1"

	resultado, err := comandos.Ejecutar(context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), &bytes.Buffer{})

	require.NoError(t, err)
	require.False(t, resultado.Exitoso)
}

func TestComandos_CapturaUnaVariableDeSalidaDeclarada(t *testing.T) {
	salida, err := dominio.NuevaVariableDeSalidaDeclarada("tag", `tag=(\S+)`, true)
	require.NoError(t, err)
	comandos := infraestructura.NuevosComandos()
	linea := "echo tag=v1.2.3"

	resultado, err := comandos.Ejecutar(
		context.Background(), t.TempDir(), linea,
		comandoDeclarado(t, linea, []dominio.VariableDeSalidaDeclarada{salida}, nil), &bytes.Buffer{},
	)

	require.NoError(t, err)
	require.True(t, resultado.Exitoso)
	require.Len(t, resultado.Producidas, 1)
	require.Equal(t, "tag", resultado.Producidas[0].Nombre)
	require.Equal(t, "v1.2.3", resultado.Producidas[0].Valor)
	require.True(t, resultado.Producidas[0].Compartida)
}

func TestComandos_UnaVariableDeSalidaQueNoMatcheaNoEsExitoso(t *testing.T) {
	salida, err := dominio.NuevaVariableDeSalidaDeclarada("tag", `tag=(\S+)`, false)
	require.NoError(t, err)
	comandos := infraestructura.NuevosComandos()
	linea := "echo nada-que-ver"

	resultado, err := comandos.Ejecutar(
		context.Background(), t.TempDir(), linea,
		comandoDeclarado(t, linea, []dominio.VariableDeSalidaDeclarada{salida}, nil), &bytes.Buffer{},
	)

	require.NoError(t, err)
	require.False(t, resultado.Exitoso)
}

func TestComandos_UnaAsercionQueNoCumpleNoEsExitoso(t *testing.T) {
	asercion, err := dominio.NuevaAsercionDeclarada("OK")
	require.NoError(t, err)
	comandos := infraestructura.NuevosComandos()
	linea := "echo FALLA"

	resultado, err := comandos.Ejecutar(
		context.Background(), t.TempDir(), linea,
		comandoDeclarado(t, linea, nil, []dominio.AsercionDeclarada{asercion}), &bytes.Buffer{},
	)

	require.NoError(t, err)
	require.False(t, resultado.Exitoso)
}

func TestComandos_LaCancelacionMataElProcesoYPropagaElError(t *testing.T) {
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelar()
	comandos := infraestructura.NuevosComandos()
	linea := "sleep 5"

	_, err := comandos.Ejecutar(ctx, t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), &bytes.Buffer{})

	require.Error(t, err)
}
