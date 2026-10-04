package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

func TestResumir_TomaLoDeLaAperturaYElEstado(t *testing.T) {
	intento := historialpublicado.Intento{
		Id: "i-1",
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
		`{"Id":"i-1","Ambiente":"sand","Solicitante":"jailux","HastaPaso":"test","Estado":"exitoso"}`,
		string(salida))
}

func TestResumirTodos_SinIntentosEsUnaListaVacia(t *testing.T) {
	salida, err := json.Marshal(resumirTodos(nil))

	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(salida))
}

func TestIntentoEIntentos_MuestranSoloElResumen(t *testing.T) {
	e := nuevoEntorno(t)
	intentar := invocar(t, e.intento(), append([]string{"intentar"}, e.banderas...)...)
	require.Equal(t, salidaBien, intentar.codigo, intentar.errores)
	clavesDelResumen := []string{"Id", "Ambiente", "Solicitante", "HastaPaso", "Estado"}

	lista := invocar(t, `{"Version":"1","Ambiente":"prod"}`, append([]string{"intentos"}, e.banderas...)...)

	require.Equal(t, salidaBien, lista.codigo, lista.errores)
	var intentos []map[string]any
	require.NoError(t, json.Unmarshal([]byte(lista.salida), &intentos))
	require.Len(t, intentos, 1)
	require.ElementsMatch(t, clavesDelResumen, claves(intentos[0]))
	require.Equal(t, "prod", intentos[0]["Ambiente"])
	require.Equal(t, "ana", intentos[0]["Solicitante"])

	uno := invocar(t, `{"Version":"1","Intento":`+quote(intentos[0]["Id"].(string))+`}`,
		append([]string{"intento"}, e.banderas...)...)

	require.Equal(t, salidaBien, uno.codigo, uno.errores)
	var intento map[string]any
	require.NoError(t, json.Unmarshal([]byte(uno.salida), &intento))
	require.ElementsMatch(t, clavesDelResumen, claves(intento))
	require.Equal(t, intentos[0], intento)
}

func claves(m map[string]any) []string {
	resultado := make([]string, 0, len(m))
	for k := range m {
		resultado = append(resultado, k)
	}
	return resultado
}
