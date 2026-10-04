package aplicacion

import (
	"context"
	"errors"
	"slices"

	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// simularPaso interpola de verdad las plantillas del paso y la línea de cada comando (§2 de SIM-1), y por cada
// variable de salida que un comando declara, fabrica una salida simulada que cumple su expresión regular y la
// registra en el ámbito que le toca — el compartido si la declaró compartida, o si no, el ámbito de este
// ambiente (mismo criterio que ejecucion/aplicacion/ejecutar_paso.go:registrarLoProducido).
//
// SIM-2: si una interpolación usa un nombre que no está disponible, se sigue con las demás interpolaciones de
// este paso para juntar todos los nombres que faltan — el resultado los dice todos, no solo el primero — y
// solo entonces se deja de fabricar salidas: quien llama abandona el resto de los pasos al recibir nombres.
// Devuelve esos nombres, vacíos si todo se interpoló.
func (s *Servicio) simularPaso(ctx context.Context, simulacion string, ambito dominio.Ambito, paso dominio.Paso) ([]string, error) {
	var faltante []string

	for _, fichero := range paso.Material {
		if !fichero.Plantilla {
			continue
		}
		if err := s.interpolarOAnotarFaltante(ctx, simulacion, ambito, fichero.Contenido, &faltante); err != nil {
			return nil, err
		}
	}

	for _, comando := range paso.Comandos {
		if err := s.simularComando(ctx, simulacion, ambito, comando, &faltante); err != nil {
			return nil, err
		}
	}

	return faltante, nil
}

func (s *Servicio) simularComando(
	ctx context.Context, simulacion string, ambito dominio.Ambito, comando dominio.Comando, faltante *[]string,
) error {
	antes := len(*faltante)
	if err := s.interpolarOAnotarFaltante(ctx, simulacion, ambito, comando.Linea, faltante); err != nil {
		return err
	}
	if len(*faltante) > antes {
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
	}
	return nil
}

// interpolarOAnotarFaltante interpola texto; si falta un nombre, lo anota en faltantes (sin duplicar)
// y no propaga el fallo — sigue con lo siguiente del mismo paso (SIM-2). Cualquier otro error sí se propaga.
func (s *Servicio) interpolarOAnotarFaltante(
	ctx context.Context, simulacion string, ambito dominio.Ambito, texto string, faltantes *[]string,
) error {
	_, err := s.d.Variables.Interpolar(ctx, simulacion, ambito, texto)
	if err == nil {
		return nil
	}
	var nombre *resolucionpublicado.VariableNoEncontradaError
	if errors.As(err, &nombre) {
		if !slices.Contains(*faltantes, nombre.Nombre) {
			*faltantes = append(*faltantes, nombre.Nombre)
		}
		return nil
	}
	return err
}
