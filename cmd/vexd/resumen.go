package main

import (
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// resumenDeIntento es lo que la lista de intentos responde de cada uno: la estructura compacta. El Historial sigue
// guardando, y el borde sigue publicando, el intento completo, que se pide con la operación intento.
type resumenDeIntento struct {
	Id          string
	Ambiente    string
	Solicitante string
	HastaPaso   string
	Estado      historialpublicado.Estado // vacío si el intento no tiene desenlace
	Causa       historialpublicado.Causa  // por qué terminó, si no fue por un comando; vacía en el caso normal
}

func resumir(i historialpublicado.Intento) resumenDeIntento {
	return resumenDeIntento{
		Id:          i.Id,
		Ambiente:    i.Apertura.Ambiente,
		Solicitante: i.Apertura.Solicitante,
		HastaPaso:   i.Apertura.HastaPaso,
		Estado:      i.Estado,
		Causa:       i.Causa,
	}
}

// resumirTodos es la lista de resúmenes, en el orden del Historial. Sin intentos es una lista vacía: el motor no
// tiene texto para humanos, y que no haya intentos no es un fallo.
func resumirTodos(intentos []historialpublicado.Intento) []resumenDeIntento {
	resumenes := make([]resumenDeIntento, 0, len(intentos))
	for _, i := range intentos {
		resumenes = append(resumenes, resumir(i))
	}
	return resumenes
}
