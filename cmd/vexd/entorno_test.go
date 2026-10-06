package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

const valorSensible = "valor-sensible-123"

// peticionConEntorno es la línea de una operación con variables de entorno, que van fuera de params.
func peticionConEntorno(metodo, params string, entorno map[string]string) string {
	variables, _ := json.Marshal(entorno)
	base := peticion(metodo, params)
	return strings.TrimSuffix(base, "}") + `,"entorno":` + string(variables) + `}`
}

func entornoQueImprimeLaVariable(t *testing.T) entorno {
	t.Helper()
	return nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: ver-entorno\n  description: lee la variable\n  cmd: echo visto=$VAR_DE_PRUEBA\n")
	})
}

func TestEntorno_LoVeElComandoYNoSaleEnNingunMensajeDelProtocolo(t *testing.T) {
	e := entornoQueImprimeLaVariable(t)

	r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(
		peticionConEntorno("intentar", e.intento(), map[string]string{"VAR_DE_PRUEBA": valorSensible})+"\n"))

	require.Equal(t, salidaBien, r.codigo, r.salida+r.errores)
	require.NotEmpty(t, r.progreso(t), "hubo avance")
	require.NotContains(t, r.salida, valorSensible, "ni en el avance ni en la respuesta")
	require.NotContains(t, r.errores, valorSensible)

	// El comando lo imprimió, y por eso está en su salida: eso se lee con logs, que es donde van las salidas.
	var l borde.RespuestaDeLogs
	invocar(t, e.rutas, "logs", `{"Version":"1"}`).resultado(t, &l)
	var vista bool
	for _, salida := range l.Salidas {
		if salida.Comando == "ver-entorno" {
			require.Equal(t, "visto="+valorSensible, strings.TrimSpace(salida.Texto))
			vista = true
		}
	}
	require.True(t, vista, "el comando vio la variable")
}

func TestEntorno_UnRollbackLoAdmiteComoUnIntento(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(
		peticionConEntorno("rollback", `{"Version":"1","Despliegue":"no-existe","Solicitante":"ana"}`, map[string]string{"A": "1"})+"\n"))

	respuesta := r.error(t)
	require.NotContains(t, respuesta.Error.Message, "no ejecuta comandos", "rollback ejecuta comandos: lo que falla es el despliegue")
	require.NotEqual(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
}

func TestEntorno_UnNombreInvalidoEsUnParametroInvalidoQueNoCuentaElValor(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(
		peticionConEntorno("intentar", e.intento(), map[string]string{"NOMBRE-MALO": valorSensible})+"\n"))

	require.Equal(t, salidaInvalida, r.codigo)
	respuesta := r.error(t)
	require.Equal(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
	require.Equal(t, tipoParametrosInvalidos, respuesta.Error.Data["tipo"])
	require.Contains(t, respuesta.Error.Message, "NOMBRE-MALO")
	require.NotContains(t, r.salida, valorSensible)
	require.NotContains(t, r.errores, valorSensible)

	var intentos []map[string]any
	invocar(t, e.rutas, "intentos", `{"Version":"1","Ambiente":"prod"}`).resultado(t, &intentos)
	require.Empty(t, intentos, "no se abrió ningún intento")
}

func TestEntorno_LasOperacionesQueNoEjecutanComandosNoLoAdmiten(t *testing.T) {
	e := nuevoEntorno(t)
	for _, operacion := range []string{"logs", "intentos", "despliegues", "simular", "lanzar", "reservar", "liberar", "diagnosticar", "abandonar", "intento", "describir"} {
		t.Run(operacion, func(t *testing.T) {
			r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(
				peticionConEntorno(operacion, `{}`, map[string]string{"A": "1"})+"\n"))

			require.Equal(t, salidaInvalida, r.codigo)
			respuesta := r.error(t)
			require.Equal(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
			require.Contains(t, respuesta.Error.Message, "no ejecuta comandos")
		})
	}
}

func TestEntorno_UnValorQueNoEsUnaCadenaNoEsUnaPeticion(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocarLinea(t, e.rutas, `{"jsonrpc":"2.0","id":"1","method":"intentar","params":{},"entorno":{"A":1}}`)

	require.Equal(t, salidaInvalida, r.codigo)
	require.Equal(t, protocolo.CodigoPeticionInvalida, r.error(t).Error.Code)
}

func TestEntorno_UnEntornoVacioEsComoNoPonerNinguno(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocarCon(t, context.Background(), e.rutas, strings.NewReader(
		peticionConEntorno("logs", `{"Version":"1"}`, map[string]string{})+"\n"))

	require.NotEqual(t, protocolo.CodigoParametrosInvalidos, r.error(t).Error.Code, "no hay variables: no hay nada que rechazar")
}
