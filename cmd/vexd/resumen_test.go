package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

func TestResumir_TomaLoDeLaAperturaYElEstado(t *testing.T) {
	intento := historialpublicado.Intento{
		Id:       "i-1",
		Instante: time.Date(2026, 10, 6, 9, 30, 0, 0, time.UTC),
		Apertura: historialpublicado.Apertura{
			Ambiente:    "sand",
			Solicitante: "jailux",
			HastaPaso:   "test",
			Pasos:       []historialpublicado.PasoDeclarado{{Nombre: "test"}},
			Contenido:   historialpublicado.Contenido{Contexto: "ejecucion/apertura-v1", Datos: []byte("x")},
		},
		Registros: []historialpublicado.RegistroDePaso{{Paso: "test"}},
		Estado:    historialpublicado.Exitoso,
	}

	salida, err := json.Marshal(resumir(intento))

	require.NoError(t, err)
	require.JSONEq(t,
		`{"Id":"i-1","Ambiente":"sand","Solicitante":"jailux","HastaPaso":"test","Instante":"2026-10-06T09:30:00Z","Estado":"exitoso","Causa":""}`,
		string(salida))
}

func TestResumir_UnIntentoSinDesenlaceTieneElEstadoVacio(t *testing.T) {
	salida, err := json.Marshal(resumir(historialpublicado.Intento{Id: "i-2"}))

	require.NoError(t, err)
	require.JSONEq(t, `{"Id":"i-2","Ambiente":"","Solicitante":"","HastaPaso":"","Instante":"0001-01-01T00:00:00Z","Estado":"","Causa":""}`, string(salida))
}

func TestResumir_UnIntentoInterrumpidoDiceSuCausa(t *testing.T) {
	intento := historialpublicado.Intento{
		Id: "i-3", Estado: historialpublicado.Fallido, Causa: historialpublicado.CausaInterrumpido,
	}

	salida, err := json.Marshal(resumir(intento))

	require.NoError(t, err)
	require.JSONEq(t,
		`{"Id":"i-3","Ambiente":"","Solicitante":"","HastaPaso":"","Instante":"0001-01-01T00:00:00Z","Estado":"fallido","Causa":"interrumpido"}`,
		string(salida))
}

func TestResumirTodos_SinIntentosEsUnaListaVacia(t *testing.T) {
	salida, err := json.Marshal(resumirTodos(nil))

	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(salida), "no hay texto para humanos: una lista sin elementos")
}

func TestResumirTodos_ConservaElOrdenDelHistorial(t *testing.T) {
	resumenes := resumirTodos([]historialpublicado.Intento{{Id: "a"}, {Id: "b"}, {Id: "c"}})

	require.Len(t, resumenes, 3)
	require.Equal(t, []string{"a", "b", "c"}, []string{resumenes[0].Id, resumenes[1].Id, resumenes[2].Id})
}

func claves(m map[string]any) []string {
	resultado := make([]string, 0, len(m))
	for k := range m {
		resultado = append(resultado, k)
	}
	return resultado
}

func TestIntentos_AmbienteSinIntentosResponde_UnaListaVacia(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "intentos", `{"Version":"1","Ambiente":"sand"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.JSONEq(t, `[]`, string(r.respuesta(t).Result))
}

func TestIntentos_RespondeLaEstructuraCompactaYIntentoElDetalleCompleto(t *testing.T) {
	e := nuevoEntorno(t)
	intento := intentar(t, e, e.intento(), salidaBien)

	lista := invocar(t, e.rutas, "intentos", `{"Version":"1","Ambiente":"prod"}`)

	require.Equal(t, salidaBien, lista.codigo, lista.errores)
	var intentos []map[string]any
	lista.resultado(t, &intentos)
	require.Len(t, intentos, 1)
	require.ElementsMatch(t, []string{"Id", "Ambiente", "Solicitante", "HastaPaso", "Instante", "Estado", "Causa"}, claves(intentos[0]),
		"la lista trae el resumen de cada intento, no el intento entero")
	require.Equal(t, intento, intentos[0]["Id"])
	require.Equal(t, "prod", intentos[0]["Ambiente"])
	require.Equal(t, "ana", intentos[0]["Solicitante"])
	require.Equal(t, "deploy", intentos[0]["HastaPaso"], "el último paso, ya resuelto")
	require.Equal(t, "exitoso", intentos[0]["Estado"])
	instante, err := time.Parse(time.RFC3339Nano, intentos[0]["Instante"].(string))
	require.NoError(t, err)
	require.False(t, instante.IsZero(), "el instante de la apertura, sin derivarlo del UUID")

	var detalle map[string]any
	invocar(t, e.rutas, "intento", `{"Version":"1","Intento":`+quote(intento)+`}`).resultado(t, &detalle)
	require.Contains(t, claves(detalle), "Registros", "el detalle completo sigue en la operación intento")
	require.Contains(t, claves(detalle), "Apertura")
	require.Equal(t, intento, detalle["Id"])
}
