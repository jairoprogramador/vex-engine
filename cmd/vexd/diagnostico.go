package main

import (
	"time"

	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

const (
	mensajeNoSeAtribuye        = "el intento está cancelado o sin desenlace: no se atribuye una causa"
	mensajeCodigoModificado    = "El codigo fue modificado"
	mensajeInstruccionesCambio = "Las instrucciones fueron modificadas"
	mensajeVariablesCambiaron  = "Las variables fueron modificadas"
)

// vistaSoloMensaje es lo que la línea de comandos muestra cuando no hay diagnóstico que dar (sin historial
// previo, o un intento que no se atribuye): solo el mensaje.
// Es solo presentación: el borde sigue publicando la Respuesta completa.
type vistaSoloMensaje struct {
	Mensaje string
}

// vistaDiagnostico compacta la Respuesta con atribución para leerla de un vistazo: contra qué intento
// exitoso se comparó, qué intento falla y qué cambió. Cada eje de Sustento aparece solo si cambió.
type vistaDiagnostico struct {
	Ambiente       string
	IntentoExitoso vistaIntentoExitoso
	IntentoFallido vistaIntentoFallido
	Sustento       vistaSustento
}

type vistaIntentoExitoso struct {
	Id    string
	Fecha time.Time
}

// CantidadDeIntentos solo existe para la referencia del mismo ambiente (ES-8).
type vistaIntentoFallido struct {
	Id                 string
	Fecha              time.Time
	CantidadDeIntentos *int `json:",omitempty"`
}

type vistaSustento struct {
	Codigo        *vistaEjeCambiado        `json:",omitempty"`
	Instrucciones *vistaEjeCambiado        `json:",omitempty"`
	Variables     *vistaVariablesCambiadas `json:",omitempty"`
}

type vistaEjeCambiado struct {
	Mensaje string
	Pasos   []string
}

type vistaVariablesCambiadas struct {
	Mensaje             string
	DeclaradasCambiadas []diagnosticopublicado.CambioDeVariable `json:",omitempty"`
	ProducidasCambiadas []diagnosticopublicado.CambioDeVariable `json:",omitempty"`
}

func presentarDiagnostico(r diagnosticopublicado.Respuesta) any {
	switch r.Forma {
	case diagnosticopublicado.SinReferencia:
		return vistaSoloMensaje{Mensaje: r.Mensaje}
	case diagnosticopublicado.NoSeAtribuye:
		return vistaSoloMensaje{Mensaje: mensajeNoSeAtribuye}
	default:
		return presentarConAtribucion(r.Sustento)
	}
}

func presentarConAtribucion(s diagnosticopublicado.Sustento) vistaDiagnostico {
	// La primera comparación es la referencia por defecto (el último despliegue del mismo ambiente).
	var referencia diagnosticopublicado.Comparacion
	if len(s.Comparaciones) > 0 {
		referencia = s.Comparaciones[0]
	}

	fallido := vistaIntentoFallido{Id: s.IntentoQueFalla, Fecha: s.InstanteDelIntentoQueFalla}
	if referencia.HayCantidadDeIntentos {
		fallido.CantidadDeIntentos = &referencia.CantidadDeIntentos
	}

	return vistaDiagnostico{
		Ambiente:       referencia.Ambiente,
		IntentoExitoso: vistaIntentoExitoso{Id: referencia.Intento, Fecha: referencia.Instante},
		IntentoFallido: fallido,
		Sustento: vistaSustento{
			Codigo:        presentarEje(s, diagnosticopublicado.Codigo, mensajeCodigoModificado),
			Instrucciones: presentarEje(s, diagnosticopublicado.Instrucciones, mensajeInstruccionesCambio),
			Variables:     presentarVariables(s),
		},
	}
}

func presentarEje(s diagnosticopublicado.Sustento, eje diagnosticopublicado.Eje, mensaje string) *vistaEjeCambiado {
	for _, cambio := range s.EjesCambiados {
		if cambio.Eje == eje {
			return &vistaEjeCambiado{Mensaje: mensaje, Pasos: cambio.Pasos}
		}
	}
	return nil
}

func presentarVariables(s diagnosticopublicado.Sustento) *vistaVariablesCambiadas {
	if len(s.VariablesDeclaradasCambiadas) == 0 && len(s.VariablesProducidasCambiadas) == 0 {
		return nil
	}
	return &vistaVariablesCambiadas{
		Mensaje:             mensajeVariablesCambiaron,
		DeclaradasCambiadas: s.VariablesDeclaradasCambiadas,
		ProducidasCambiadas: s.VariablesProducidasCambiadas,
	}
}
