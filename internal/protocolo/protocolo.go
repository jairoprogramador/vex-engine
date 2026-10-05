// Package protocolo es el transporte del motor: JSON-RPC 2.0 con un mensaje JSON por línea (NDJSON), el sobre
// de las peticiones y las respuestas, y la lectura y escritura de líneas (docs/rediseno/RD-13-protocolo.md).
//
// No es del dominio ni conoce ningún contexto: solo usa la biblioteca estándar. Qué operaciones hay y qué error
// de dominio es qué código es de quien lo usa (cmd/vexd).
package protocolo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
)

// VersionJSONRPC es la única versión de JSON-RPC que se habla.
const VersionJSONRPC = "2.0"

// Códigos que JSON-RPC 2.0 reserva para los fallos del propio protocolo.
const (
	CodigoJSONInvalido        = -32700
	CodigoPeticionInvalida    = -32600
	CodigoMetodoDesconocido   = -32601
	CodigoParametrosInvalidos = -32602
)

// MetodoCancelar es la notificación con la que quien invoca pide cancelar lo que está en curso.
const MetodoCancelar = "cancelar"

// ErrLineaDemasiadoLarga: una línea pasó del máximo que se admite.
var ErrLineaDemasiadoLarga = errors.New("protocolo: línea demasiado larga")

// Peticion es lo que quien invoca envía. Entorno es la única extensión sobre JSON-RPC: variables de entorno
// para los comandos (RD-13, P-6), fuera de Params para no tocar los tipos que Params lleva.
type Peticion struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Method  string            `json:"method"`
	Params  json.RawMessage   `json:"params"`
	Entorno map[string]string `json:"entorno"`
}

// Error es el fallo de una respuesta. Data lleva el tipo estable del error y lo que ayude a quien lo recibe.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// Respuesta es lo que el motor devuelve a una petición: o un resultado o un error, nunca los dos.
type Respuesta struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        json.RawMessage `json:"id"`
	Resultado json.RawMessage `json:"result,omitempty"`
	Error     *Error          `json:"error,omitempty"`
}

// Notificacion es un mensaje del motor que no espera respuesta, como el progreso.
type Notificacion struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// NuevaNotificacion arma una notificación con la versión de JSON-RPC puesta.
func NuevaNotificacion(metodo string, params any) Notificacion {
	return Notificacion{JSONRPC: VersionJSONRPC, Method: metodo, Params: params}
}

// Exito es la respuesta de una petición que se atendió. Un valor sin contenido se responde, no se omite, y una
// lista sin elementos es [], nunca null: quien la lee no tiene que distinguir los dos vacíos.
func Exito(id json.RawMessage, valor any) (Respuesta, error) {
	if enumerable := reflect.ValueOf(valor); enumerable.Kind() == reflect.Slice && enumerable.IsNil() {
		valor = []any{}
	}
	resultado, err := json.Marshal(valor)
	if err != nil {
		return Respuesta{}, fmt.Errorf("protocolo: el resultado: %w", err)
	}
	return Respuesta{JSONRPC: VersionJSONRPC, ID: id, Resultado: resultado}, nil
}

// Fallo es la respuesta de una petición que no se pudo atender. Sin id (una petición que no se pudo leer), JSON-RPC
// pide responder null.
func Fallo(id json.RawMessage, e Error) Respuesta {
	return Respuesta{JSONRPC: VersionJSONRPC, ID: id, Error: &e}
}

// Decodificar lee una línea como petición. Si no es JSON, o no es una petición válida, devuelve el error que hay
// que responder y, cuando se alcanzó a leer, la petición con su id para responder con él.
func Decodificar(linea []byte) (Peticion, *Error) {
	if !json.Valid(linea) {
		return Peticion{}, &Error{Code: CodigoJSONInvalido, Message: "la línea no es JSON",
			Data: map[string]string{"tipo": "json_invalido"}}
	}
	var p Peticion
	d := json.NewDecoder(bytes.NewReader(linea))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return Peticion{}, invalida("no es una petición JSON-RPC: %v", err)
	}
	if p.JSONRPC != VersionJSONRPC {
		return p, invalida("falta \"jsonrpc\":\"2.0\"")
	}
	if !esIdentificador(p.ID) {
		p.ID = nil
		return p, invalida("falta \"id\": una cadena o un número")
	}
	if p.Method == "" {
		return p, invalida("falta \"method\": la operación")
	}
	return p, nil
}

func invalida(formato string, args ...any) *Error {
	return &Error{Code: CodigoPeticionInvalida, Message: fmt.Sprintf(formato, args...),
		Data: map[string]string{"tipo": "peticion_invalida"}}
}

// esIdentificador: el id de una petición es una cadena o un número; sin él sería una notificación.
func esIdentificador(id json.RawMessage) bool {
	if len(id) == 0 {
		return false
	}
	return id[0] == '"' || id[0] == '-' || (id[0] >= '0' && id[0] <= '9')
}

// LeerLinea lee una línea de r sin su salto, y falla con ErrLineaDemasiadoLarga si pasa de maximo bytes. Una
// última línea sin salto también vale; sin nada que leer, devuelve io.EOF.
func LeerLinea(r *bufio.Reader, maximo int) ([]byte, error) {
	var linea []byte
	for {
		trozo, err := r.ReadSlice('\n')
		linea = append(linea, trozo...)
		sinSalto := bytes.TrimRight(linea, "\r\n")
		if len(sinSalto) > maximo {
			return nil, ErrLineaDemasiadoLarga
		}
		switch {
		case err == nil:
			return sinSalto, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(linea) > 0:
			return sinSalto, nil
		default:
			return nil, err
		}
	}
}

// Emisor escribe mensajes en w, uno por línea. Varias goroutines pueden enviar a la vez (la respuesta y el
// progreso comparten la salida) sin que se mezclen dos mensajes en una línea.
type Emisor struct {
	mu sync.Mutex
	w  io.Writer
}

func NuevoEmisor(w io.Writer) *Emisor { return &Emisor{w: w} }

// Enviar escribe v como una línea JSON compacta, en una sola escritura.
func (e *Emisor) Enviar(v any) error {
	var linea bytes.Buffer
	codificador := json.NewEncoder(&linea)
	codificador.SetEscapeHTML(false)
	if err := codificador.Encode(v); err != nil {
		return fmt.Errorf("protocolo: codificar el mensaje: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := e.w.Write(linea.Bytes()); err != nil {
		return fmt.Errorf("protocolo: escribir el mensaje: %w", err)
	}
	return nil
}
