package aplicacion

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// ejecutarPaso interpola las plantillas del paso —justo antes de correr, para que puedan usar lo que produjeron
// pasos anteriores—, interpola y ejecuta sus comandos en orden, y entrega a Resolución lo que producen. Se
// detiene en el primer comando que no sale exitoso: exitoso=false y err=nil. Un error de verdad (interpolar,
// ejecutar, registrar) siempre es err != nil. La salida de cada comando se entrega al Historial al terminar,
// salga como salga.
func (s *Servicio) ejecutarPaso(
	ctx context.Context, intento string, ubicacion dominio.Ubicacion, paso dominio.PasoDeEjecucion, ambito dominio.Ambito,
	entorno dominio.Entorno,
) (bool, error) {
	interpolar := func(texto string) (string, error) {
		return s.d.Variables.Interpolar(ctx, intento, paso.Nombre(), ambito, texto)
	}
	if err := s.d.EspacioDeTrabajo.InterpolarPlantillas(ctx, ubicacion, paso, interpolar); err != nil {
		return false, fmt.Errorf("ejecución: interpolar las plantillas del paso %q: %w", paso.Nombre(), err)
	}

	directorio := s.d.EspacioDeTrabajo.DirectorioDelPaso(ubicacion, paso.Nombre())

	for _, comando := range paso.Comandos {
		lineaInterpolada, err := interpolar(comando.Linea())
		if err != nil {
			return false, fmt.Errorf("ejecución: interpolar el comando %q del paso %q: %w", comando.Nombre(), paso.Nombre(), err)
		}

		var salida bytes.Buffer
		resultado, err := s.d.Comandos.Ejecutar(ctx, directorio, lineaInterpolada, comando, entorno, &salida)
		if err != nil {
			err = fmt.Errorf("ejecución: el comando %q del paso %q: %w", comando.Nombre(), paso.Nombre(), err)
		}
		salioBien := err == nil && resultado.Exitoso
		if errSalida := s.registrarSalida(ctx, intento, paso.Nombre(), comando.Nombre(), salioBien, salida.String()); errSalida != nil {
			return false, errors.Join(err, errSalida)
		}
		// Su salida ya está en el Historial: quien reciba este aviso puede pedir logs y encontrarla.
		s.emitir(ctx, dominio.EventoDeProgreso{
			Tipo: dominio.ComandoTerminado, Intento: intento, Paso: paso.Nombre(), Comando: comando.Nombre(),
			Estado: estadoDelComando(salioBien),
		})
		if err != nil {
			return false, err
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

func estadoDelComando(salioBien bool) string {
	if salioBien {
		return dominio.Exitoso.String()
	}
	return dominio.Fallido.String()
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

// registrarSalida entrega la salida de un comando al Historial. Sin depender del ctx: si lo que terminó el
// comando fue la cancelación, su salida es justo la que más interesa conservar.
func (s *Servicio) registrarSalida(ctx context.Context, intento, paso, comando string, exitoso bool, texto string) error {
	return s.d.Historial.RegistrarSalida(context.WithoutCancel(ctx), intento, paso, comando, exitoso, texto)
}
