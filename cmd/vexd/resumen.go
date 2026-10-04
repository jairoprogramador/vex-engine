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

// resumirTodos devuelve siempre una lista, nunca nil, para que sin intentos se imprima [] y no null.
func resumirTodos(intentos []historialpublicado.Intento) []resumenDeIntento {
	resumenes := make([]resumenDeIntento, 0, len(intentos))
	for _, i := range intentos {
		resumenes = append(resumenes, resumir(i))
	}
	return resumenes
}
