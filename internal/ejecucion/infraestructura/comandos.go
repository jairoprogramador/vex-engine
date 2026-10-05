package infraestructura

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

const (
	// plazoDeGraciaPorDefecto es lo que tiene un comando para terminar por su cuenta cuando se cancela: se le pide
	// con SIGTERM, que es lo que permite a terraform soltar el bloqueo de su estado, y pasado el plazo se acaba con
	// él. Es menor que los 10 s que docker stop da antes de matar el contenedor.
	plazoDeGraciaPorDefecto = 5 * time.Second

	// margenDeLaEspera es lo que se espera de más, tras el plazo de gracia, a que las tuberías de salida se cierren
	// solas antes de cerrarlas a la fuerza.
	margenDeLaEspera = time.Second
)

// Comandos ejecuta un comando de verdad, con el shell del sistema, en el directorio de su paso dentro del
// espacio de trabajo del ambiente.
type Comandos struct {
	plazoDeGracia time.Duration
}

var _ dominio.Comandos = Comandos{}

func NuevosComandos() Comandos { return Comandos{plazoDeGracia: plazoDeGraciaPorDefecto} }

// NuevosComandosConPlazoDeGracia cambia lo que un comando tiene para terminar tras una cancelación, antes de que se
// acabe con él (por defecto, 5 s).
func NuevosComandosConPlazoDeGracia(plazo time.Duration) Comandos {
	return Comandos{plazoDeGracia: plazo}
}

func (c Comandos) plazo() time.Duration {
	if c.plazoDeGracia <= 0 {
		return plazoDeGraciaPorDefecto
	}
	return c.plazoDeGracia
}

// Ejecutar corre el comando y reenvía su salida a salida, a la vez que la acumula en un
// búfer transitorio — nunca expuesto fuera de esta función — para comprobar sus aserciones y capturar sus
// variables de salida una vez termina. Si ctx se cancela, se acaba con el comando y con todo lo que lanzó (no
// solo con el shell) y el error se propaga: es la aplicación quien lo traduce en una cancelación del intento
// (EJ-3), no este puerto.
func (c Comandos) Ejecutar(
	ctx context.Context, directorio, lineaInterpolada string, comando dominio.ComandoDeclarado, entorno dominio.Entorno,
	salida io.Writer,
) (dominio.ResultadoDeUnComando, error) {
	nombreDelShell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		nombreDelShell, flag = "cmd", "/C"
	}
	cmd := exec.CommandContext(ctx, nombreDelShell, flag, lineaInterpolada)
	cmd.Dir = filepath.Join(directorio, filepath.FromSlash(comando.Directorio()))
	if !entorno.Vacio() {
		// Con una variable repetida vale la última: las que se piden pisan las que el proceso ya tenía.
		cmd.Env = append(os.Environ(), entorno.Lista()...)
	}
	terminar := prepararProceso(cmd, c.plazo())

	var capturada bytes.Buffer
	// Un solo escritor para los dos flujos: exec lo reconoce y los lleva por la misma tubería, así que no hay
	// escrituras concurrentes sobre salida ni sobre capturada, y salida y error quedan en el orden en que se
	// produjeron.
	escritor := io.MultiWriter(salida, &capturada)
	cmd.Stdout = escritor
	cmd.Stderr = escritor

	err := cmd.Run()
	terminar(ctx.Err() != nil)
	if ctx.Err() != nil {
		// La cancelación mató el proceso: es un error de este puerto, nunca un resultado no exitoso — es la
		// aplicación quien la traduce en la cancelación del intento (EJ-3), no este puerto (DEC-09.2).
		return dominio.ResultadoDeUnComando{}, fmt.Errorf("ejecución: %q: %w", comando.Nombre(), ctx.Err())
	}
	var salioConError *exec.ExitError
	switch {
	case err == nil:
		// código de salida 0
	case errors.Is(err, exec.ErrWaitDelay):
		// terminó con código 0, pero dejó un proceso en segundo plano con la tubería abierta: no es un fallo
		// suyo, y lo escrito hasta aquí es todo lo que va a haber.
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
