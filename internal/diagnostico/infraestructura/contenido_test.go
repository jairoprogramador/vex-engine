package infraestructura

import (
	"testing"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Estas pruebas fijan el contrato de la copia privada y angosta que este ACL guarda de la forma que
// escriben Ejecución y Resolución — el JSON es literalmente el que producen sus propios contenido.go.

func TestDecodificarRecursosDePaso(t *testing.T) {
	c := historialpublicado.Contenido{
		Contexto: contextoRegistroDeEjecucion,
		Datos:    []byte(`{"hash_del_codigo":"c1","hash_de_instrucciones":"i1"}`),
	}
	codigo, instrucciones, err := decodificarRecursosDePaso(c)
	require.NoError(t, err)
	require.Equal(t, "c1", codigo)
	require.Equal(t, "i1", instrucciones)
}

func TestDecodificarRecursosDePaso_ContextoEquivocado(t *testing.T) {
	_, _, err := decodificarRecursosDePaso(historialpublicado.Contenido{Contexto: "otro/contexto-v1"})
	require.Error(t, err)
}

func TestDecodificarOrdenDeAmbientes(t *testing.T) {
	c := historialpublicado.Contenido{
		Contexto: contextoAperturaDeEjecucion,
		Datos: []byte(
			`{"fuente_del_proyecto":"x","commit_del_proyecto":"y","fuente_del_pipeline":"z",` +
				`"commit_del_pipeline":"w","orden_de_ambientes":["sand","stag","prod"]}`,
		),
	}
	orden, err := decodificarOrdenDeAmbientes(c)
	require.NoError(t, err)
	require.Equal(t, []string{"sand", "stag", "prod"}, orden)
}

func TestDecodificarOrdenDeAmbientes_SinElCampoDaVacio(t *testing.T) {
	c := historialpublicado.Contenido{
		Contexto: contextoAperturaDeEjecucion,
		Datos:    []byte(`{"fuente_del_proyecto":"x","commit_del_proyecto":"y","fuente_del_pipeline":"z","commit_del_pipeline":"w"}`),
	}
	orden, err := decodificarOrdenDeAmbientes(c)
	require.NoError(t, err)
	require.Empty(t, orden)
}

func TestDecodificarOrdenDeAmbientes_ContextoEquivocado(t *testing.T) {
	_, err := decodificarOrdenDeAmbientes(historialpublicado.Contenido{Contexto: "otro/contexto-v1"})
	require.Error(t, err)
}

func TestDecodificarVariable(t *testing.T) {
	c := historialpublicado.Contenido{
		Contexto: contextoVariableDeResolucion,
		Datos:    []byte(`{"hash":"variable-v1:abc","origen":"declarada","compartido":false,"ambiente":"prod"}`),
	}
	hash, origen, err := decodificarVariable(c)
	require.NoError(t, err)
	require.Equal(t, "variable-v1:abc", hash)
	require.Equal(t, origenDeclarada, origen)
}

func TestDecodificarVariable_ContextoEquivocado(t *testing.T) {
	_, _, err := decodificarVariable(historialpublicado.Contenido{Contexto: "otro/contexto-v1"})
	require.Error(t, err)
}
