package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

const mensajeSinHistorial = "no hay historial previo para poder diagnosticar"

func TestPresentarDiagnostico_SinReferenciaMuestraSoloElMensaje(t *testing.T) {
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.SinReferencia, Mensaje: mensajeSinHistorial}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{"Mensaje":"`+mensajeSinHistorial+`"}`, string(salida))
}

func TestPresentarDiagnostico_LasOtrasFormasNoLlevanMensaje(t *testing.T) {
	casos := map[string]diagnosticopublicado.Respuesta{
		"con atribución": {
			Forma:      diagnosticopublicado.ConAtribucion,
			Atribucion: []diagnosticopublicado.Eje{diagnosticopublicado.Codigo},
		},
		"no se atribuye": {Forma: diagnosticopublicado.NoSeAtribuye},
	}
	for nombre, r := range casos {
		t.Run(nombre, func(t *testing.T) {
			salida, err := json.Marshal(presentarDiagnostico(r))

			require.NoError(t, err)
			var vista map[string]any
			require.NoError(t, json.Unmarshal(salida, &vista))
			require.ElementsMatch(t, []string{"Forma", "Atribucion", "Sustento"}, claves(vista))
			require.Equal(t, string(r.Forma), vista["Forma"])
		})
	}
}

func TestDiagnosticar_SinHistorialPrevioMuestraSoloElMensaje(t *testing.T) {
	e := nuevoEntorno(t)
	intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	require.Equal(t, salidaBien, intentar.codigo, intentar.errores)
	var intentado struct{ Intento string }
	require.NoError(t, json.Unmarshal([]byte(intentar.salida), &intentado))

	r := invocar(t, `{"Version":"1","Ambiente":"prod","Intento":`+quote(intentado.Intento)+`}`,
		append([]string{"diagnosticar"}, e.banderas...)...)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var vista map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.salida), &vista))
	require.Equal(t, []string{"Mensaje"}, claves(vista))
	require.NotEmpty(t, vista["Mensaje"])
}
