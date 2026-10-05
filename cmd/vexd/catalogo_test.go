package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	lanzamientopublicado "github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
	simulacionpublicado "github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

func TestClasificar(t *testing.T) {
	casos := map[string]struct {
		err    error
		codigo int
		tipo   string
		salida int
	}{
		"parámetros ilegibles":                {errParametros, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"petición inválida del borde":         {borde.ErrPeticionInvalida, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"inválido de ejecución":               {ejecucionpublicado.ErrInvalido, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"inválido de simulación":              {simulacionpublicado.ErrInvalido, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"inválido de lanzamiento":             {lanzamientopublicado.ErrInvalido, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"inválido de diagnóstico":             {diagnosticopublicado.ErrInvalido, protocolo.CodigoParametrosInvalidos, tipoParametrosInvalidos, salidaInvalida},
		"versión no soportada":                {borde.ErrVersionNoSoportada, codigoVersionNoSoportada, tipoVersionNoSoportada, salidaInvalida},
		"configuración del proceso":           {errConfiguracion, codigoConfiguracionInvalida, tipoConfiguracionInvalida, salidaInvalida},
		"no existe en el historial":           {historialpublicado.ErrNoExiste, codigoNoExiste, tipoNoExiste, salidaFallo},
		"historial sin intentos":              {borde.ErrHistorialSinIntentos, codigoNoExiste, tipoNoExiste, salidaFallo},
		"rechazado por ejecución":             {ejecucionpublicado.ErrRechazado, codigoRechazado, tipoRechazado, salidaFallo},
		"rechazado por el historial":          {historialpublicado.ErrRechazado, codigoRechazado, tipoRechazado, salidaFallo},
		"espacio de trabajo no disponible":    {ejecucionpublicado.ErrNoDisponible, codigoNoDisponible, tipoNoDisponible, salidaFallo},
		"cancelado antes de abrir el intento": {context.Canceled, codigoCancelado, tipoCancelado, salidaCancelado},
		"cualquier otro":                      {errors.New("se rompió algo"), codigoInterno, tipoInterno, salidaFallo},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			f := clasificar(fmt.Errorf("al atender: %w", c.err))

			require.Equal(t, c.codigo, f.codigo)
			require.Equal(t, c.tipo, f.tipo)
			require.Equal(t, c.salida, f.salida)
		})
	}
}

func TestClasificar_UnAmbienteOcupadoDiceElIntentoAunqueTambienSeaUnRechazo(t *testing.T) {
	ocupado := fmt.Errorf("abrir: %w", &historialpublicado.AmbienteOcupadoError{Ambiente: "prod", Intento: "01a1"})
	require.ErrorIs(t, ocupado, historialpublicado.ErrRechazado, "es también un rechazo: por eso se mira primero")

	f := clasificar(ocupado)

	require.Equal(t, codigoAmbienteOcupado, f.codigo)
	require.Equal(t, tipoAmbienteOcupado, f.tipo)
	require.Equal(t, salidaFallo, f.salida)
	require.Equal(t, map[string]string{"tipo": tipoAmbienteOcupado, "ambiente": "prod", "intento": "01a1"}, f.aError().Data)
}

func TestClasificar_UnErrorInternoNoCuentaSuCausa(t *testing.T) {
	f := clasificar(errors.New("el disco /var/secreto se llenó"))

	require.Equal(t, "error interno", f.aError().Message)
}

func TestCodigoDeLaRespuesta(t *testing.T) {
	require.Equal(t, salidaBien, codigoDeLaRespuesta(ejecucionpublicado.Resultado{Estado: estadoExitoso}))
	require.Equal(t, salidaFallo, codigoDeLaRespuesta(ejecucionpublicado.Resultado{Estado: estadoFallido}))
	require.Equal(t, salidaCancelado, codigoDeLaRespuesta(ejecucionpublicado.Resultado{Estado: estadoCancelado}))
	require.Equal(t, salidaFallo, codigoDeLaRespuesta(simulacionpublicado.Resultado{Estado: estadoFallido}))
	require.Equal(t, salidaBien, codigoDeLaRespuesta(struct{}{}), "lo que no es un intento sale bien si se atendió")
}
