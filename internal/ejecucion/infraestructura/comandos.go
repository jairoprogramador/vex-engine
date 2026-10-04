package infraestructura

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// Comandos ejecuta un comando de verdad, con el shell del sistema, en el directorio de su paso dentro del
// espacio de trabajo del ambiente.
type Comandos struct{}

var _ dominio.Comandos = Comandos{}

func NuevosComandos() Comandos { return Comandos{} }

// Ejecutar corre el comando y reenvía su salida a salida, a la vez que la acumula en un
// búfer transitorio — nunca expuesto fuera de esta función — para comprobar sus aserciones y capturar sus
// variables de salida una vez termina. Si ctx se cancela, el proceso muere y el error se propaga: es la
// aplicación quien lo traduce en una cancelación del intento (EJ-3), no este puerto.
func (Comandos) Ejecutar(
	ctx context.Context, directorio, lineaInterpolada string, comando dominio.ComandoDeclarado, salida io.Writer,
) (dominio.ResultadoDeUnComando, error) {
	nombreDelShell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		nombreDelShell, flag = "cmd", "/C"
	}
	cmd := exec.CommandContext(ctx, nombreDelShell, flag, lineaInterpolada)
	cmd.Dir = filepath.Join(directorio, filepath.FromSlash(comando.Directorio()))

	var capturada bytes.Buffer
	// Un solo escritor para los dos flujos: exec lo reconoce y los lleva por la misma tubería, así que no hay
	// escrituras concurrentes sobre salida ni sobre capturada, y salida y error quedan en el orden en que se
	// produjeron.
	escritor := io.MultiWriter(salida, &capturada)
	cmd.Stdout = escritor
	cmd.Stderr = escritor

	err := cmd.Run()
	if ctx.Err() != nil {
		// La cancelación mató el proceso: es un error de este puerto, nunca un resultado no exitoso — es la
		// aplicación quien la traduce en la cancelación del intento (EJ-3), no este puerto (DEC-09.2).
		return dominio.ResultadoDeUnComando{}, fmt.Errorf("ejecución: %q: %w", comando.Nombre(), ctx.Err())
	}
	var salioConError *exec.ExitError
	switch {
	case err == nil:
		// código de salida 0
	case errors.As(err, &salioConError):
		// el comando corrió y terminó con un código distinto de 0: no exitoso, pero no es un error de este
		// puerto — lo dice el resultado.
		return dominio.ResultadoDeUnComando{Exitoso: false}, nil
	default:
		return dominio.ResultadoDeUnComando{}, fmt.Errorf("ejecución: ejecutar %q: %w", comando.Nombre(), err)
	}

	texto := capturada.String()
	for _, a := range comando.Aserciones() {
		cumple, err := regexp.MatchString(a.Expresion(), texto)
		if err != nil {
			return dominio.ResultadoDeUnComando{}, fmt.Errorf("ejecución: la aserción de %q: %w", comando.Nombre(), err)
		}
		if !cumple {
			return dominio.ResultadoDeUnComando{Exitoso: false}, nil
		}
	}

	producidas := make([]dominio.VariableProducida, 0, len(comando.Salidas()))
	for _, s := range comando.Salidas() {
		valor, capturado, err := capturar(s.Expresion(), texto)
		if err != nil {
			return dominio.ResultadoDeUnComando{}, fmt.Errorf("ejecución: la variable de salida %q de %q: %w", s.Nombre(), comando.Nombre(), err)
		}
		if !capturado {
			return dominio.ResultadoDeUnComando{Exitoso: false}, nil
		}
		producidas = append(producidas, dominio.VariableProducida{Nombre: s.Nombre(), Valor: valor, Compartida: s.Compartida()})
	}

	return dominio.ResultadoDeUnComando{Exitoso: true, Producidas: producidas}, nil
}

// capturar da el primer grupo de la expresión si tiene uno, o si no, la coincidencia entera.
func capturar(expresion, texto string) (valor string, capturado bool, err error) {
	regla, err := regexp.Compile(expresion)
	if err != nil {
		return "", false, err
	}
	coincidencia := regla.FindStringSubmatch(texto)
	if coincidencia == nil {
		return "", false, nil
	}
	if len(coincidencia) > 1 {
		return coincidencia[1], true, nil
	}
	return coincidencia[0], true, nil
}
