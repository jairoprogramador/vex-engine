package main

import (
	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

// vistaSinReferencia es lo que la línea de comandos muestra cuando no hay historial previo: solo el mensaje.
// Es solo presentación: el borde sigue publicando la Respuesta completa.
type vistaSinReferencia struct {
	Mensaje string
}

// vistaConDesenlace es la Respuesta de las otras dos formas, sin Mensaje: allí siempre está vacío y se omite
// en vez de imprimir "".
type vistaConDesenlace struct {
	Forma      diagnosticopublicado.FormaDeRespuesta
	Atribucion []diagnosticopublicado.Eje
	Sustento   diagnosticopublicado.Sustento
}

func presentarDiagnostico(r diagnosticopublicado.Respuesta) any {
	if r.Forma == diagnosticopublicado.SinReferencia {
		return vistaSinReferencia{Mensaje: r.Mensaje}
	}
	return vistaConDesenlace{Forma: r.Forma, Atribucion: r.Atribucion, Sustento: r.Sustento}
}
