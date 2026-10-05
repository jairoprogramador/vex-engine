package main

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	lanzamientopublicado "github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
	simulacionpublicado "github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Códigos propios del motor para el error de una respuesta (docs/rediseno/RD-13-protocolo.md). Los del propio
// protocolo son los de JSON-RPC y viven en internal/protocolo. Son estables: se añaden, no se cambian.
const (
	codigoInterno               = -32000
	codigoVersionNoSoportada    = -32001
	codigoRechazado             = -32002
	codigoNoExiste              = -32003
	codigoAmbienteOcupado       = -32004
	codigoNoDisponible          = -32005
	codigoConfiguracionInvalida = -32006
	codigoCancelado             = -32007
)

// Tipos de error: el nombre estable que un cliente puede comparar, en data.tipo.
const (
	tipoParametrosInvalidos   = "parametros_invalidos"
	tipoVersionNoSoportada    = "version_no_soportada"
	tipoRechazado             = "rechazado"
	tipoNoExiste              = "no_existe"
	tipoAmbienteOcupado       = "ambiente_ocupado"
	tipoNoDisponible          = "no_disponible"
	tipoConfiguracionInvalida = "configuracion_invalida"
	tipoCancelado             = "cancelado"
	tipoInterno               = "interno"
	tipoOperacionDesconocida  = "operacion_desconocida"
	tipoPeticionInvalida      = "peticion_invalida"
)

// errParametros: los params de la petición no se pudieron leer: JSON mal formado para la operación o un campo que
// el lenguaje publicado no tiene. Es un error de quien invoca, no de la operación.
var errParametros = errors.New("parámetros ilegibles")

// errConfiguracion: falta algo de la configuración del proceso (las variables VEX_*) o no se puede usar. No es
// culpa de la petición: es de quien lanzó el contenedor.
var errConfiguracion = errors.New("configuración del proceso")

// fallo es un error ya clasificado: lo que se responde y cómo sale el proceso.
type fallo struct {
	codigo  int
	tipo    string
	salida  int
	mensaje string
	datos   map[string]string
	// causa es el error original, que solo se escribe en la salida de error y solo si es interno.
	causa error
}

// aError es el error de la respuesta. Un error interno no cuenta su causa: va a la salida de error, no a quien
// invoca.
func (f fallo) aError() protocolo.Error {
	datos := map[string]string{"tipo": f.tipo}
	for clave, valor := range f.datos {
		datos[clave] = valor
	}
	return protocolo.Error{Code: f.codigo, Message: f.mensaje, Data: datos}
}

// clasificar traduce un error de la operación a lo que el protocolo responde: es un ACL de salida, la única
// parte que conoce los errores que publican los contextos de entrada. El orden importa:
// *AmbienteOcupadoError también es un ErrRechazado, y mirado después perdería el intento que ocupa el ambiente.
func clasificar(err error) fallo {
	var ocupado *historialpublicado.AmbienteOcupadoError
	switch {
	case errors.As(err, &ocupado):
		return fallo{codigo: codigoAmbienteOcupado, tipo: tipoAmbienteOcupado, salida: salidaFallo, mensaje: err.Error(),
			datos: map[string]string{"ambiente": ocupado.Ambiente, "intento": ocupado.Intento}}
	case errors.Is(err, errConfiguracion):
		return fallo{codigo: codigoConfiguracionInvalida, tipo: tipoConfiguracionInvalida, salida: salidaInvalida, mensaje: err.Error()}
	case errors.Is(err, borde.ErrVersionNoSoportada):
		return fallo{codigo: codigoVersionNoSoportada, tipo: tipoVersionNoSoportada, salida: salidaInvalida, mensaje: err.Error()}
	case esPeticionInvalida(err):
		return fallo{codigo: protocolo.CodigoParametrosInvalidos, tipo: tipoParametrosInvalidos, salida: salidaInvalida, mensaje: err.Error()}
	case errors.Is(err, historialpublicado.ErrNoExiste):
		return fallo{codigo: codigoNoExiste, tipo: tipoNoExiste, salida: salidaFallo, mensaje: err.Error()}
	case errors.Is(err, ejecucionpublicado.ErrRechazado), errors.Is(err, historialpublicado.ErrRechazado):
		return fallo{codigo: codigoRechazado, tipo: tipoRechazado, salida: salidaFallo, mensaje: err.Error()}
	case errors.Is(err, ejecucionpublicado.ErrNoDisponible):
		return fallo{codigo: codigoNoDisponible, tipo: tipoNoDisponible, salida: salidaFallo, mensaje: err.Error()}
	case errors.Is(err, context.Canceled):
		return fallo{codigo: codigoCancelado, tipo: tipoCancelado, salida: salidaCancelado, mensaje: "cancelado antes de abrir el intento"}
	default:
		return fallo{codigo: codigoInterno, tipo: tipoInterno, salida: salidaFallo, mensaje: "error interno"}
	}
}

// tipoDelProtocolo es el data.tipo de un error que puso internal/protocolo.
func tipoDelProtocolo(e *protocolo.Error) string {
	if datos, ok := e.Data.(map[string]string); ok {
		return datos["tipo"]
	}
	return tipoPeticionInvalida
}

// esPeticionInvalida: los parámetros se leyeron, pero lo pedido no se puede pedir.
func esPeticionInvalida(err error) bool {
	return errors.Is(err, errParametros) ||
		errors.Is(err, borde.ErrPeticionInvalida) ||
		errors.Is(err, ejecucionpublicado.ErrInvalido) ||
		errors.Is(err, simulacionpublicado.ErrInvalido) ||
		errors.Is(err, lanzamientopublicado.ErrInvalido) ||
		errors.Is(err, diagnosticopublicado.ErrInvalido)
}

// codigoDeLaRespuesta traduce cómo terminó un intento, o cómo terminaría uno simulado, en el código de salida:
// la operación se atendió bien, pero el pipeline pudo fallar o cancelarse, y quien invoca desde un script lo
// necesita sin leer el JSON.
func codigoDeLaRespuesta(respuesta any) int {
	switch r := respuesta.(type) {
	case ejecucionpublicado.Resultado:
		return codigoDelEstado(string(r.Estado))
	case simulacionpublicado.Resultado:
		return codigoDelEstado(string(r.Estado))
	default:
		return salidaBien
	}
}

func codigoDelEstado(estado string) int {
	switch estado {
	case estadoExitoso:
		return salidaBien
	case estadoCancelado:
		return salidaCancelado
	default:
		return salidaFallo
	}
}
