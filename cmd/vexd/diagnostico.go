package main

import (
	"time"

	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

// vistaDiagnostico es la estructura compacta con que diagnosticar responde la Respuesta que publica el borde.
// Solo lleva datos, nunca texto para el usuario final: quien presenta (vex) decide qué decir a partir de ellos.
//
// Hay dos tipos de respuesta, y se distinguen con una regla: si trae SinDiagnostico, no hay diagnóstico y su valor
// dice por qué; si no, lo hay.
//   - Con diagnóstico (el intento falla y hay con qué comparar): Ambiente, IntentoExitoso, IntentoFallido y Sustento,
//     sin discriminador, porque la presencia de esos campos ya lo dice. Cada eje de Sustento aparece solo si
//     cambió; un Sustento sin ejes es un hecho (nada de lo que mira cada paso cambió), no la falta de diagnóstico.
//   - Sin diagnóstico: solo SinDiagnostico, con el motivo: sin_referencia (no hay historial previo con el que
//     comparar) o no_se_atribuye (el intento está cancelado o sin desenlace, no se atribuye una causa).
type vistaDiagnostico struct {
	SinDiagnostico diagnosticopublicado.FormaDeRespuesta `json:",omitempty"`
	Ambiente       string                                `json:",omitempty"`
	IntentoExitoso *vistaIntentoExitoso                  `json:",omitempty"`
	IntentoFallido *vistaIntentoFallido                  `json:",omitempty"`
	Sustento       *vistaSustento                        `json:",omitempty"`
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

// vistaEjeCambiado son los pasos en que cambió un eje.
type vistaEjeCambiado struct {
	Pasos []string
}

// vistaVariablesCambiadas son las variables que cambiaron, por paso y nombre; nunca su valor (DEC-04.7).
type vistaVariablesCambiadas struct {
	DeclaradasCambiadas []diagnosticopublicado.CambioDeVariable `json:",omitempty"`
	ProducidasCambiadas []diagnosticopublicado.CambioDeVariable `json:",omitempty"`
}

func presentarDiagnostico(r diagnosticopublicado.Respuesta) vistaDiagnostico {
	if r.Forma != diagnosticopublicado.ConAtribucion {
		// El Diagnóstico trae un texto para sin_referencia; el motor no lo lleva: el motivo ya dice qué pasó.
		return vistaDiagnostico{SinDiagnostico: r.Forma}
	}
	return presentarConAtribucion(r.Sustento)
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
		IntentoExitoso: &vistaIntentoExitoso{Id: referencia.Intento, Fecha: referencia.Instante},
		IntentoFallido: &fallido,
		Sustento: &vistaSustento{
			Codigo:        presentarEje(s, diagnosticopublicado.Codigo),
			Instrucciones: presentarEje(s, diagnosticopublicado.Instrucciones),
			Variables:     presentarVariables(s),
		},
	}
}

func presentarEje(s diagnosticopublicado.Sustento, eje diagnosticopublicado.Eje) *vistaEjeCambiado {
	for _, cambio := range s.EjesCambiados {
		if cambio.Eje == eje {
			return &vistaEjeCambiado{Pasos: cambio.Pasos}
		}
	}
	return nil
}

func presentarVariables(s diagnosticopublicado.Sustento) *vistaVariablesCambiadas {
	if len(s.VariablesDeclaradasCambiadas) == 0 && len(s.VariablesProducidasCambiadas) == 0 {
		return nil
	}
	return &vistaVariablesCambiadas{
		DeclaradasCambiadas: s.VariablesDeclaradasCambiadas,
		ProducidasCambiadas: s.VariablesProducidasCambiadas,
	}
}
