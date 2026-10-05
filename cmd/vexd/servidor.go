package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	ejecuciondominio "github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

// maximoDeLinea es lo más largo que puede ser un mensaje: una línea que lo pase es una petición inválida.
const maximoDeLinea = 1 << 20

// ejecutar atiende una petición del protocolo (docs/rediseno/RD-13-protocolo.md): lee una línea de la entrada,
// responde con una línea en la salida y termina. Es main sin el proceso: recibe todo lo que toca, para poder
// probarlo entero. Lo único que va a la salida de error es lo que no debe llegar a quien invoca.
func ejecutar(ctx context.Context, r rutas, entrada io.Reader, salida, errores io.Writer) int {
	emisor := protocolo.NuevoEmisor(salida)
	lector := bufio.NewReader(entrada)

	peticion, rechazo := leerPeticion(lector)
	if rechazo != nil {
		return responder(emisor, errores, peticion, *rechazo)
	}

	ctx, cancelar := context.WithCancel(ctx)
	defer cancelar()
	go vigilarCancelacion(lector, cancelar)

	return atenderPeticion(ctx, r, peticion, emisor, errores)
}

// leerPeticion lee la única petición del proceso. Si no se puede, devuelve lo que hay que responder y, si se
// alcanzó a leer, la petición con su id.
func leerPeticion(lector *bufio.Reader) (protocolo.Peticion, *fallo) {
	linea, err := protocolo.LeerLinea(lector, maximoDeLinea)
	switch {
	case errors.Is(err, io.EOF):
		return protocolo.Peticion{}, &fallo{codigo: protocolo.CodigoPeticionInvalida, tipo: tipoPeticionInvalida,
			salida: salidaInvalida, mensaje: "no llegó ninguna petición: la entrada se cerró antes"}
	case errors.Is(err, protocolo.ErrLineaDemasiadoLarga):
		return protocolo.Peticion{}, &fallo{codigo: protocolo.CodigoPeticionInvalida, tipo: tipoPeticionInvalida,
			salida: salidaInvalida, mensaje: fmt.Sprintf("la línea pasa de %d bytes", maximoDeLinea)}
	case err != nil:
		f := clasificar(err)
		f.causa = err
		return protocolo.Peticion{}, &f
	}

	peticion, malaPeticion := protocolo.Decodificar(linea)
	if malaPeticion != nil {
		return peticion, &fallo{codigo: malaPeticion.Code, tipo: tipoDelProtocolo(malaPeticion),
			salida: salidaInvalida, mensaje: malaPeticion.Message}
	}
	return peticion, nil
}

func atenderPeticion(ctx context.Context, r rutas, p protocolo.Peticion, emisor *protocolo.Emisor, errores io.Writer) int {
	op, hay := buscar(p.Method)
	if !hay {
		return responder(emisor, errores, p, fallo{codigo: protocolo.CodigoMetodoDesconocido, tipo: tipoOperacionDesconocida,
			salida: salidaInvalida, mensaje: fmt.Sprintf("operación desconocida %q", p.Method)})
	}
	if len(p.Entorno) > 0 && !op.ejecutaComandos {
		return responder(emisor, errores, p, fallo{codigo: protocolo.CodigoParametrosInvalidos, tipo: tipoParametrosInvalidos,
			salida: salidaInvalida, mensaje: fmt.Sprintf("entorno: la operación %q no ejecuta comandos, no tiene a quién dárselo", op.nombre)})
	}

	respuesta, err := atenderContandoElProgreso(ctx, r, op, p, emisor, errores)
	if err != nil {
		return responderError(emisor, errores, p, err)
	}
	return responderExito(emisor, errores, p, respuesta)
}

// atenderContandoElProgreso atiende la operación mientras cuenta a quien invoca cómo avanza. Lo que cuenta se
// escribe del todo antes de volver: las notificaciones van siempre antes que la respuesta.
func atenderContandoElProgreso(
	ctx context.Context, r rutas, op operacion, p protocolo.Peticion, emisor *protocolo.Emisor, errores io.Writer,
) (any, error) {
	progreso := nuevoProgreso(emisor, errores)
	defer progreso.cerrar()

	servicio, err := servicioPara(r, op, progreso)
	if err != nil {
		return nil, err
	}
	return op.atender(ctx, servicio, parametrosDe(p), p.Entorno)
}

// servicioPara compone el motor para la operación, si lo necesita. La configuración que falta se dice antes de
// tocar ningún contexto.
func servicioPara(r rutas, op operacion, progreso ejecuciondominio.Progreso) (*borde.Servicio, error) {
	if !op.usaMotor {
		return nil, nil
	}
	if err := r.validar(op); err != nil {
		return nil, err
	}
	return componer(r, progreso)
}

// parametrosDe son los params de la petición; sin ellos, un objeto vacío, para que cada operación diga qué le falta.
func parametrosDe(p protocolo.Peticion) []byte {
	if len(p.Params) == 0 {
		return []byte("{}")
	}
	return p.Params
}

func responderExito(emisor *protocolo.Emisor, errores io.Writer, p protocolo.Peticion, respuesta any) int {
	exito, err := protocolo.Exito(p.ID, respuesta)
	if err != nil {
		return responderError(emisor, errores, p, err)
	}
	if err := emisor.Enviar(exito); err != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, p.Method, err)
		return salidaFallo
	}
	return codigoDeLaRespuesta(respuesta)
}

func responderError(emisor *protocolo.Emisor, errores io.Writer, p protocolo.Peticion, err error) int {
	f := clasificar(err)
	f.causa = err
	return responder(emisor, errores, p, f)
}

// responder envía el error de una petición y devuelve cómo sale el proceso. La causa de un error interno no se
// le cuenta a quien invoca: se escribe en la salida de error.
func responder(emisor *protocolo.Emisor, errores io.Writer, p protocolo.Peticion, f fallo) int {
	if f.tipo == tipoInterno && f.causa != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, p.Method, f.causa)
	}
	if err := emisor.Enviar(protocolo.Fallo(p.ID, f.aError())); err != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, p.Method, err)
		return salidaFallo
	}
	return f.salida
}

// vigilarCancelacion espera, en lo que queda de la entrada, la notificación cancelar. Que la entrada se cierre
// no cancela nada: quien invoca solo dijo que no enviará más (así funciona un pipe), y la cancelación es
// explícita o es una señal.
func vigilarCancelacion(lector *bufio.Reader, cancelar context.CancelFunc) {
	for {
		linea, err := protocolo.LeerLinea(lector, maximoDeLinea)
		if err != nil {
			return
		}
		var mensaje struct {
			Method string `json:"method"`
		}
		if json.Unmarshal(linea, &mensaje) == nil && mensaje.Method == protocolo.MetodoCancelar {
			cancelar()
			return
		}
	}
}
