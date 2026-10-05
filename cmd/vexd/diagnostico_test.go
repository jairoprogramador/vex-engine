package main

import (
	"encoding/json"
	"testing"
	"time"

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

func TestPresentarDiagnostico_NoSeAtribuyeMuestraSoloUnMensaje(t *testing.T) {
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.NoSeAtribuye}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{"Mensaje":"`+mensajeNoSeAtribuye+`"}`, string(salida))
}

func sustentoDeEjemplo() diagnosticopublicado.Sustento {
	instante := time.Date(2026, 10, 4, 22, 4, 47, 0, time.UTC)
	return diagnosticopublicado.Sustento{
		IntentoQueFalla:            "fallido",
		InstanteDelIntentoQueFalla: instante.Add(time.Hour),
		EjesCambiados: []diagnosticopublicado.CambioDeEje{
			{Eje: diagnosticopublicado.Codigo, Pasos: []string{"test"}},
			{Eje: diagnosticopublicado.Instrucciones, Pasos: []string{"test"}},
		},
		Comparaciones: []diagnosticopublicado.Comparacion{{
			Despliegue: "despliegue", Ambiente: "sand", Intento: "exitoso", Instante: instante,
			CantidadDeIntentos: 3, HayCantidadDeIntentos: true,
		}},
	}
}

func TestPresentarDiagnostico_ConAtribucionEsCompacto(t *testing.T) {
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustentoDeEjemplo()}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{
		"Ambiente": "sand",
		"IntentoExitoso": {"Id": "exitoso", "Fecha": "2026-10-04T22:04:47Z"},
		"IntentoFallido": {"Id": "fallido", "Fecha": "2026-10-04T23:04:47Z", "CantidadDeIntentos": 3},
		"Sustento": {
			"Codigo": {"Mensaje": "`+mensajeCodigoModificado+`", "Pasos": ["test"]},
			"Instrucciones": {"Mensaje": "`+mensajeInstruccionesCambio+`", "Pasos": ["test"]}
		}
	}`, string(salida))
}

func TestPresentarDiagnostico_OmiteLoQueNoCambio(t *testing.T) {
	sustento := sustentoDeEjemplo()
	sustento.EjesCambiados = sustento.EjesCambiados[:1]
	sustento.Comparaciones[0].HayCantidadDeIntentos = false

	salida, err := json.Marshal(presentarDiagnostico(
		diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento}))

	require.NoError(t, err)
	var vista struct {
		IntentoFallido map[string]any
		Sustento       map[string]any
	}
	require.NoError(t, json.Unmarshal(salida, &vista))
	require.ElementsMatch(t, []string{"Id", "Fecha"}, claves(vista.IntentoFallido))
	require.Equal(t, []string{"Codigo"}, claves(vista.Sustento))
}

func TestPresentarDiagnostico_VariablesSoloConCambios(t *testing.T) {
	sustento := sustentoDeEjemplo()
	sustento.VariablesProducidasCambiadas = []diagnosticopublicado.CambioDeVariable{{Paso: "test", Nombre: "url"}}

	salida, err := json.Marshal(presentarDiagnostico(
		diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento}))

	require.NoError(t, err)
	var vista struct {
		Sustento struct{ Variables map[string]any }
	}
	require.NoError(t, json.Unmarshal(salida, &vista))
	require.ElementsMatch(t, []string{"Mensaje", "ProducidasCambiadas"}, claves(vista.Sustento.Variables))
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
