package protocolo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func lector(s string) *bufio.Reader { return bufio.NewReaderSize(strings.NewReader(s), 16) }

func TestLeerLinea_LeeUnaLineaSinSuSalto(t *testing.T) {
	r := lector("uno\r\ndos\n")

	primera, err := LeerLinea(r, 100)
	require.NoError(t, err)
	segunda, err := LeerLinea(r, 100)
	require.NoError(t, err)

	require.Equal(t, "uno", string(primera))
	require.Equal(t, "dos", string(segunda))
}

func TestLeerLinea_UnaUltimaLineaSinSaltoTambienVale(t *testing.T) {
	linea, err := LeerLinea(lector("sin salto"), 100)

	require.NoError(t, err)
	require.Equal(t, "sin salto", string(linea))
}

func TestLeerLinea_SinNadaQueLeerEsEOF(t *testing.T) {
	_, err := LeerLinea(lector(""), 100)

	require.ErrorIs(t, err, io.EOF)
}

func TestLeerLinea_UnaLineaMasLargaQueElBufferSeLee(t *testing.T) {
	larga := strings.Repeat("x", 1000)

	linea, err := LeerLinea(lector(larga+"\n"), 1000)

	require.NoError(t, err)
	require.Equal(t, larga, string(linea))
}

func TestLeerLinea_PasarDelMaximoFalla(t *testing.T) {
	for nombre, entrada := range map[string]string{
		"con salto": strings.Repeat("x", 11) + "\n",
		"sin salto": strings.Repeat("x", 11),
	} {
		t.Run(nombre, func(t *testing.T) {
			_, err := LeerLinea(lector(entrada), 10)

			require.ErrorIs(t, err, ErrLineaDemasiadoLarga)
		})
	}
}

func TestLeerLinea_UnaLineaDeExactamenteElMaximoVale(t *testing.T) {
	linea, err := LeerLinea(lector(strings.Repeat("x", 10)+"\n"), 10)

	require.NoError(t, err)
	require.Len(t, linea, 10)
}

func TestDecodificar_UnaPeticionValida(t *testing.T) {
	p, e := Decodificar([]byte(`{"jsonrpc":"2.0","id":"7","method":"intentar","params":{"Version":"1"},"entorno":{"A":"1"}}`))

	require.Nil(t, e)
	require.Equal(t, "intentar", p.Method)
	require.JSONEq(t, `"7"`, string(p.ID))
	require.JSONEq(t, `{"Version":"1"}`, string(p.Params))
	require.Equal(t, map[string]string{"A": "1"}, p.Entorno)
}

func TestDecodificar_ElIdPuedeSerUnNumero(t *testing.T) {
	p, e := Decodificar([]byte(`{"jsonrpc":"2.0","id":42,"method":"describir"}`))

	require.Nil(t, e)
	require.JSONEq(t, `42`, string(p.ID))
}

func TestDecodificar_LoQueNoEsUnaPeticionSeRechazaConSuCodigo(t *testing.T) {
	casos := map[string]struct {
		linea  string
		codigo int
		tipo   string
		id     string
	}{
		"no es JSON":          {`{`, CodigoJSONInvalido, "json_invalido", ""},
		"no es un objeto":     {`[1]`, CodigoPeticionInvalida, "peticion_invalida", ""},
		"campo desconocido":   {`{"jsonrpc":"2.0","id":"1","method":"x","otro":1}`, CodigoPeticionInvalida, "peticion_invalida", ""},
		"versión de jsonrpc":  {`{"jsonrpc":"1.0","id":"1","method":"x"}`, CodigoPeticionInvalida, "peticion_invalida", `"1"`},
		"sin id":              {`{"jsonrpc":"2.0","method":"x"}`, CodigoPeticionInvalida, "peticion_invalida", ""},
		"id null":             {`{"jsonrpc":"2.0","id":null,"method":"x"}`, CodigoPeticionInvalida, "peticion_invalida", ""},
		"id que es un objeto": {`{"jsonrpc":"2.0","id":{},"method":"x"}`, CodigoPeticionInvalida, "peticion_invalida", ""},
		"sin method":          {`{"jsonrpc":"2.0","id":"1"}`, CodigoPeticionInvalida, "peticion_invalida", `"1"`},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			p, e := Decodificar([]byte(c.linea))

			require.NotNil(t, e)
			require.Equal(t, c.codigo, e.Code)
			require.Equal(t, map[string]string{"tipo": c.tipo}, e.Data)
			require.Equal(t, c.id, string(p.ID), "el id que se alcanzó a leer, para responder con él")
		})
	}
}

func TestExito_UnValorSinContenidoSeResponde(t *testing.T) {
	r, err := Exito(json.RawMessage(`"1"`), struct{}{})
	require.NoError(t, err)

	var salida bytes.Buffer
	require.NoError(t, NuevoEmisor(&salida).Enviar(r))

	require.Equal(t, `{"jsonrpc":"2.0","id":"1","result":{}}`+"\n", salida.String())
}

func TestExito_UnaListaSinElementosEsUnaListaVacia_NoNull(t *testing.T) {
	var lista []string
	r, err := Exito(json.RawMessage(`"1"`), lista)
	require.NoError(t, err)

	var salida bytes.Buffer
	require.NoError(t, NuevoEmisor(&salida).Enviar(r))

	require.Equal(t, `{"jsonrpc":"2.0","id":"1","result":[]}`+"\n", salida.String())
}

func TestExito_UnValorNuloSeResponde_Null(t *testing.T) {
	r, err := Exito(json.RawMessage(`"1"`), nil)
	require.NoError(t, err)

	var salida bytes.Buffer
	require.NoError(t, NuevoEmisor(&salida).Enviar(r))

	require.Equal(t, `{"jsonrpc":"2.0","id":"1","result":null}`+"\n", salida.String())
}

func TestFallo_SinIdResponde_Null(t *testing.T) {
	r := Fallo(nil, Error{Code: CodigoJSONInvalido, Message: "la línea no es JSON"})

	var salida bytes.Buffer
	require.NoError(t, NuevoEmisor(&salida).Enviar(r))

	require.Equal(t, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"la línea no es JSON"}}`+"\n", salida.String())
}

func TestEmisor_NoEscapaLoQueNoHaceFalta(t *testing.T) {
	var salida bytes.Buffer

	require.NoError(t, NuevoEmisor(&salida).Enviar(NuevaNotificacion("progreso", map[string]string{"paso": "a<b&c"})))

	require.Contains(t, salida.String(), "a<b&c")
}

func TestEmisor_VariasGoroutinesNoMezclanLineas(t *testing.T) {
	var salida bytes.Buffer
	emisor := NuevoEmisor(&salida)
	const mensajes = 200

	var esperar sync.WaitGroup
	for i := range mensajes {
		esperar.Add(1)
		go func() {
			defer esperar.Done()
			require.NoError(t, emisor.Enviar(NuevaNotificacion("progreso", map[string]int{"n": i})))
		}()
	}
	esperar.Wait()

	lineas := strings.Split(strings.TrimSuffix(salida.String(), "\n"), "\n")
	require.Len(t, lineas, mensajes)
	for _, linea := range lineas {
		require.True(t, json.Valid([]byte(linea)), linea)
	}
}

type escritorRoto struct{}

func (escritorRoto) Write([]byte) (int, error) { return 0, errors.New("tubería rota") }

func TestEmisor_UnaEscrituraQueFallaSeDice(t *testing.T) {
	err := NuevoEmisor(escritorRoto{}).Enviar(NuevaNotificacion("progreso", nil))

	require.ErrorContains(t, err, "tubería rota")
}
