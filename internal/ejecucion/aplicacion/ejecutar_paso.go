package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// ejecutarPaso interpola las plantillas del paso —justo antes de correr, para que puedan usar lo que produjeron
// pasos anteriores—, interpola y ejecuta sus comandos en orden, y entrega a Resolución lo que producen. Se
// detiene en el primer comando que no sale exitoso: exitoso=false y err=nil. Un error de verdad (interpolar,
// ejecutar, registrar) siempre es err != nil.
func (s *Servicio) ejecutarPaso(
	ctx context.Context, intento string, ubicacion dominio.Ubicacion, paso dominio.PasoDeEjecucion, ambito dominio.Ambito, salida publicado.Salida,
) (bool, error) {
	interpolar := func(texto string) (string, error) {
		return s.d.Variables.Interpolar(ctx, intento, paso.Nombre(), ambito, texto)
	}
	if err := s.d.EspacioDeTrabajo.InterpolarPlantillas(ctx, ubicacion, paso, interpolar); err != nil {
		return false, fmt.Errorf("ejecución: interpolar las plantillas del paso %q: %w", paso.Nombre(), err)
	}

	directorio := s.d.EspacioDeTrabajo.DirectorioDelPaso(ubicacion, paso.Nombre())
	escritor := &escritorDeSalida{salida: salida, paso: paso.Nombre()}

	for _, comando := range paso.Comandos {
		lineaInterpolada, err := interpolar(comando.Linea())
		if err != nil {
			return false, fmt.Errorf("ejecución: interpolar el comando %q del paso %q: %w", comando.Nombre(), paso.Nombre(), err)
		}

		resultado, err := s.d.Comandos.Ejecutar(ctx, directorio, lineaInterpolada, comando, escritor)
		if err != nil {
			return false, fmt.Errorf("ejecución: el comando %q del paso %q: %w", comando.Nombre(), paso.Nombre(), err)
		}
		if !resultado.Exitoso {
			return false, nil
		}

		if err := s.registrarLoProducido(ctx, intento, paso.Nombre(), ambito, resultado.Producidas); err != nil {
			return false, err
		}
	}
	return true, nil
}

// registrarLoProducido entrega cada variable producida al ámbito que le toca: el compartido si el comando la
// declaró compartida, o si no, el del paso que la produjo (definicion/publicado.VariableDeSalida.Compartida).
func (s *Servicio) registrarLoProducido(
	ctx context.Context, intento, paso string, ambito dominio.Ambito, producidas []dominio.VariableProducida,
) error {
	for _, p := range producidas {
		ambitoDeLoProducido := ambito
		if p.Compartida {
			ambitoDeLoProducido = dominio.AmbitoCompartido()
		}
		if err := s.d.Variables.RegistrarProducido(ctx, intento, paso, p.Nombre, p.Valor, ambitoDeLoProducido); err != nil {
			return fmt.Errorf("ejecución: registrar lo que produjo %q del paso %q: %w", p.Nombre, paso, err)
		}
	}
	return nil
}

// escritorDeSalida adapta la salida por paso (lo que recibe la operación pública) a io.Writer, que es lo que
// pide el puerto dominio.Comandos.
type escritorDeSalida struct {
	salida publicado.Salida
	paso   string
}

func (e *escritorDeSalida) Write(datos []byte) (int, error) {
	if err := e.salida.Escribir(e.paso, datos); err != nil {
		return 0, err
	}
	return len(datos), nil
}
