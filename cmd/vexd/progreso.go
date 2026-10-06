package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

const (
	// metodoProgreso es la notificación con la que vexd cuenta a quien invoca cómo avanza un intento.
	metodoProgreso = "progreso"

	// capacidadDeLaCola es cuántos eventos esperan a ser escritos. Un intento cuenta unas decenas: que se llene
	// quiere decir que quien invoca no está leyendo.
	capacidadDeLaCola = 256
)

// paramsDeProgreso es el cuerpo de la notificación progreso. Es del protocolo, no un tipo publicado: sus nombres
// van en minúscula, como los demás miembros del sobre.
type paramsDeProgreso struct {
	Evento  string `json:"evento"`
	Intento string `json:"intento,omitempty"`
	Paso    string `json:"paso,omitempty"`
	Comando string `json:"comando,omitempty"`
	Estado  string `json:"estado,omitempty"`
}

// progresoPorElProtocolo es el dominio.Progreso de vexd: manda cada evento a quien invoca como una notificación.
//
// Emitir no espera nunca: pone el evento en una cola y una sola goroutine los escribe, en orden. Si quien invoca no
// lee la salida, la escritura se bloquea, la cola se llena y los eventos nuevos se descartan: un destino que no
// lee no puede parar un intento. Al terminar, cerrar escribe lo que quedó en la cola, para que las notificaciones
// lleguen antes que la respuesta.
type progresoPorElProtocolo struct {
	emisor  *protocolo.Emisor
	errores io.Writer

	cerrojo sync.RWMutex
	cerrado bool
	cola    chan protocolo.Notificacion
	hecho   chan struct{}

	descartados     atomic.Int64
	falloAlEscribir error // solo lo toca la goroutine escritora; se lee cuando hecho está cerrado
}

var _ dominio.Progreso = (*progresoPorElProtocolo)(nil)

func nuevoProgreso(emisor *protocolo.Emisor, errores io.Writer) *progresoPorElProtocolo {
	p := &progresoPorElProtocolo{
		emisor: emisor, errores: errores,
		cola: make(chan protocolo.Notificacion, capacidadDeLaCola), hecho: make(chan struct{}),
	}
	go p.escribir()
	return p
}

func (p *progresoPorElProtocolo) escribir() {
	defer close(p.hecho)
	for notificacion := range p.cola {
		if p.falloAlEscribir != nil {
			continue // no se puede escribir: se vacía la cola para no bloquear a nadie
		}
		p.falloAlEscribir = p.emisor.Enviar(notificacion)
	}
}

func (p *progresoPorElProtocolo) Emitir(_ context.Context, e dominio.EventoDeProgreso) {
	notificacion := protocolo.NuevaNotificacion(metodoProgreso, paramsDeProgreso{
		Evento: string(e.Tipo), Intento: e.Intento, Paso: e.Paso, Comando: e.Comando, Estado: e.Estado,
	})
	p.cerrojo.RLock()
	defer p.cerrojo.RUnlock()
	if p.cerrado {
		return
	}
	select {
	case p.cola <- notificacion:
	default:
		p.descartados.Add(1)
	}
}

// cerrar escribe lo que queda en la cola y deja de aceptar eventos. Se puede llamar más de una vez. Dice, en la
// salida de error, si algo se perdió: es de quien lanzó el contenedor, no de quien invoca.
func (p *progresoPorElProtocolo) cerrar() {
	p.cerrojo.Lock()
	if p.cerrado {
		p.cerrojo.Unlock()
		return
	}
	p.cerrado = true
	close(p.cola)
	p.cerrojo.Unlock()

	<-p.hecho
	if n := p.descartados.Load(); n > 0 {
		fmt.Fprintf(p.errores, "%s: progreso: se descartaron %d eventos porque la salida no se leía\n", marcaDeOperacion, n)
	}
	if p.falloAlEscribir != nil {
		fmt.Fprintf(p.errores, "%s: progreso: %v\n", marcaDeOperacion, p.falloAlEscribir)
	}
}
