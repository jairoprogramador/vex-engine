package main

import (
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// resumenDeIntento es lo que la línea de comandos muestra de un intento. Es solo presentación: el Historial
// sigue guardando, y el borde sigue publicando, el intento completo.
type resumenDeIntento struct {
	Id          string
	Ambiente    string
	Solicitante string
	HastaPaso   string
	Estado      historialpublicado.Estado
}

func resumir(i historialpublicado.Intento) resumenDeIntento {
	return resumenDeIntento{
		Id:          i.Id,
		Ambiente:    i.Apertura.Ambiente,
		Solicitante: i.Apertura.Solicitante,
		HastaPaso:   i.Apertura.HastaPaso,
		Estado:      i.Estado,
	}
}

const mensajeSinIntentos = "no hay intentos para mostrar"

// resumirTodos es la lista de resúmenes, o solo un mensaje si no hay ningún intento: que no haya no es un fallo.
func resumirTodos(intentos []historialpublicado.Intento) any {
	if len(intentos) == 0 {
		return vistaSoloMensaje{Mensaje: mensajeSinIntentos}
	}
	resumenes := make([]resumenDeIntento, 0, len(intentos))
	for _, i := range intentos {
		resumenes = append(resumenes, resumir(i))
	}
	return resumenes
}

const mensajeSinDespliegues = "no hay despliegues para mostrar"

// mostrarDespliegues es la lista de despliegues tal como la publica el borde, o solo un mensaje si no hay
// ninguno: que no haya no es un fallo.
func mostrarDespliegues(despliegues []historialpublicado.Despliegue) any {
	if len(despliegues) == 0 {
		return vistaSoloMensaje{Mensaje: mensajeSinDespliegues}
	}
	return despliegues
}
