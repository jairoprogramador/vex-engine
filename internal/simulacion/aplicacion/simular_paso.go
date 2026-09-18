package aplicacion

import (
	"context"
	"errors"
	"slices"

	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// simularPaso interpola de verdad las plantillas del paso y la línea de cada comando (§2 de SIM-1), y por cada
// variable de salida que un comando declara, fabrica una salida simulada que cumple su expresión regular y la
// registra en el ámbito que le toca — el compartido si la declaró compartida, o si no, el ámbito de este
// ambiente (mismo criterio que ejecucion/aplicacion/ejecutar_paso.go:registrarLoProducido).
//
// SIM-2: si una interpolación usa un nombre que no está disponible, se sigue con las demás interpolaciones de
// este paso para juntar todos los nombres que faltan — el informe los dice todos, no solo el primero — y
// solo entonces se deja de fabricar salidas: quien llama abandona el resto del ambiente al ver Faltante no
// vacío.
func (s *Servicio) simularPaso(ctx context.Context, simulacion string, ambito dominio.Ambito, paso dominio.Paso) (publicado.InformeDePaso, error) {
	informe := publicado.InformeDePaso{Paso: paso.Nombre}

	for _, fichero := range paso.Material {
		if !fichero.Plantilla {
			continue
		}
		if err := s.interpolarOAnotarFaltante(ctx, simulacion, ambito, fichero.Contenido, &informe); err != nil {
			return publicado.InformeDePaso{}, err
		}
	}

	for _, comando := range paso.Comandos {
		if err := s.simularComando(ctx, simulacion, ambito, comando, &informe); err != nil {
			return publicado.InformeDePaso{}, err
		}
	}

	return informe, nil
}

func (s *Servicio) simularComando(
	ctx context.Context, simulacion string, ambito dominio.Ambito, comando dominio.Comando, informe *publicado.InformeDePaso,
) error {
	antes := len(informe.Faltante)
	if err := s.interpolarOAnotarFaltante(ctx, simulacion, ambito, comando.Linea, informe); err != nil {
		return err
	}
	if len(informe.Faltante) > antes {
		return nil // este comando no interpola: no se fabrican sus salidas (DEC-10.4, nada se inventa de más).
	}

	for _, salida := range comando.Salidas {
		fabricada, err := dominio.FabricarSalidaSimulada(salida.Expresion)
		if err != nil {
			return err
		}
		ambitoDeLaSalida := ambito
		if salida.Compartida {
			ambitoDeLaSalida = dominio.AmbitoCompartido()
		}
		if err := s.d.Variables.RegistrarProducido(ctx, simulacion, salida.Nombre, string(fabricada), ambitoDeLaSalida); err != nil {
			return err
		}
		informe.Interpolado = append(informe.Interpolado, salida.Nombre)
	}
	return nil
}

// interpolarOAnotarFaltante interpola texto; si falta un nombre, lo anota en informe.Faltante (sin duplicar)
// y no propaga el fallo — sigue con lo siguiente del mismo paso (SIM-2). Cualquier otro error sí se propaga.
func (s *Servicio) interpolarOAnotarFaltante(
	ctx context.Context, simulacion string, ambito dominio.Ambito, texto string, informe *publicado.InformeDePaso,
) error {
	_, err := s.d.Variables.Interpolar(ctx, simulacion, ambito, texto)
	if err == nil {
		return nil
	}
	var faltante *resolucionpublicado.VariableNoEncontradaError
	if errors.As(err, &faltante) {
		if !slices.Contains(informe.Faltante, faltante.Nombre) {
			informe.Faltante = append(informe.Faltante, faltante.Nombre)
		}
		return nil
	}
	return err
}
