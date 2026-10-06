package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

// --- el adaptador ---

// escritorBloqueable es una salida que nadie lee: bloquea cada escritura hasta que se la libera.
type escritorBloqueable struct {
	mu          sync.Mutex
	escrito     bytes.Buffer
	liberado    chan struct{}
	escribiendo chan struct{}
	unaVez      sync.Once
}

func nuevoEscritorBloqueable() *escritorBloqueable {
	return &escritorBloqueable{liberado: make(chan struct{}), escribiendo: make(chan struct{})}
}

func (e *escritorBloqueable) Write(p []byte) (int, error) {
	e.unaVez.Do(func() { close(e.escribiendo) })
	<-e.liberado
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.escrito.Write(p)
}

func (e *escritorBloqueable) liberar() { close(e.liberado) }

func (e *escritorBloqueable) lineas() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	texto := strings.TrimSuffix(e.escrito.String(), "\n")
	if texto == "" {
		return nil
	}
	return strings.Split(texto, "\n")
}

func eventoNumero(n int) dominio.EventoDeProgreso {
	return dominio.EventoDeProgreso{Tipo: dominio.PasoIniciado, Intento: "i-1", Paso: fmt.Sprintf("paso-%04d", n)}
}

func TestProgresoPorElProtocolo_LosEventosLlegaranEnOrdenYAntesDeQueCerrarVuelva(t *testing.T) {
	var salida, errores bytes.Buffer
	p := nuevoProgreso(protocolo.NuevoEmisor(&salida), &errores)

	for n := range 50 {
		p.Emitir(context.Background(), eventoNumero(n))
	}
	p.cerrar()

	lineas := strings.Split(strings.TrimSuffix(salida.String(), "\n"), "\n")
	require.Len(t, lineas, 50, "cerrar escribe todo lo que quedaba")
	for n, linea := range lineas {
		var notificacion struct {
			Method string
			Params paramsDeProgreso
		}
		require.NoError(t, json.Unmarshal([]byte(linea), &notificacion))
		require.Equal(t, metodoProgreso, notificacion.Method)
		require.Equal(t, "paso_iniciado", notificacion.Params.Evento)
		require.Equal(t, fmt.Sprintf("paso-%04d", n), notificacion.Params.Paso, "en el orden en que ocurrieron")
	}
	require.Empty(t, errores.String(), "no se perdió nada: no hay nada que decir")
}

func TestProgresoPorElProtocolo_UnaSalidaQueNadieLeeNoParaAQuienEmite(t *testing.T) {
	salida := nuevoEscritorBloqueable()
	var errores bytes.Buffer
	p := nuevoProgreso(protocolo.NuevoEmisor(salida), &errores)
	const emitidos = 5000

	terminado := make(chan struct{})
	go func() {
		for n := range emitidos {
			p.Emitir(context.Background(), eventoNumero(n))
		}
		close(terminado)
	}()

	select {
	case <-terminado:
	case <-time.After(5 * time.Second):
		t.Fatal("Emitir esperó a una salida que nadie lee: un intento se habría parado")
	}

	salida.liberar()
	p.cerrar()
	llegaron := salida.lineas()
	require.NotEmpty(t, llegaron)
	require.Less(t, len(llegaron), emitidos, "lo que no cupo en la cola se descartó")
	require.Contains(t, errores.String(), "se descartaron", "y se dice, para quien lanzó el contenedor")
	require.Contains(t, errores.String(), fmt.Sprint(emitidos-len(llegaron)), "cuántos")
	anterior := -1
	for _, linea := range llegaron {
		var n struct{ Params paramsDeProgreso }
		require.NoError(t, json.Unmarshal([]byte(linea), &n))
		var numero int
		_, err := fmt.Sscanf(n.Params.Paso, "paso-%d", &numero)
		require.NoError(t, err)
		require.Greater(t, numero, anterior, "los que llegaron siguen en orden")
		anterior = numero
	}
}

func TestProgresoPorElProtocolo_CerrarSeguidoDeEmitirNoHaceNada(t *testing.T) {
	var salida, errores bytes.Buffer
	p := nuevoProgreso(protocolo.NuevoEmisor(&salida), &errores)
	p.Emitir(context.Background(), eventoNumero(1))
	p.cerrar()

	require.NotPanics(t, func() {
		p.Emitir(context.Background(), eventoNumero(2))
		p.cerrar()
	})

	require.Equal(t, 1, strings.Count(salida.String(), "\n"))
}

func TestProgresoPorElProtocolo_UnaSalidaRotaSeDiceYNoDetieneNada(t *testing.T) {
	var errores bytes.Buffer
	p := nuevoProgreso(protocolo.NuevoEmisor(escritorRoto{}), &errores)

	for n := range 20 {
		p.Emitir(context.Background(), eventoNumero(n))
	}
	p.cerrar()

	require.Contains(t, errores.String(), "tubería rota")
}

type escritorRoto struct{}

func (escritorRoto) Write([]byte) (int, error) { return 0, fmt.Errorf("tubería rota") }

func TestProgresoPorElProtocolo_NoLlevaLaSalidaDeNingunComando(t *testing.T) {
	var salida, errores bytes.Buffer
	p := nuevoProgreso(protocolo.NuevoEmisor(&salida), &errores)

	p.Emitir(context.Background(), dominio.EventoDeProgreso{
		Tipo: dominio.ComandoTerminado, Intento: "i", Paso: "test", Comando: "c1", Estado: "exitoso",
	})
	p.cerrar()

	require.JSONEq(t,
		`{"jsonrpc":"2.0","method":"progreso","params":{"evento":"comando_terminado","intento":"i","paso":"test","comando":"c1","estado":"exitoso"}}`,
		strings.TrimSpace(salida.String()))
}

// --- de punta a punta, por el protocolo ---

func TestProgreso_UnIntentoCuentaSuAvanceAntesDeLaRespuesta(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var resultado struct{ Intento, Estado string }
	r.resultado(t, &resultado)
	eventos := r.progreso(t)
	require.NotEmpty(t, eventos)
	require.Equal(t, paramsDeProgreso{Evento: "intento_iniciado", Intento: resultado.Intento}, eventos[0], "lo primero que se cuenta")
	require.Equal(t, "paso_terminado", eventos[len(eventos)-1].Evento)
	require.Equal(t, "ejecutado", eventos[len(eventos)-1].Estado)
	for _, ev := range eventos {
		require.Equal(t, resultado.Intento, ev.Intento, "todos dicen de qué intento son")
	}

	var comandosDelPrimerPaso []string
	for _, ev := range eventos {
		if ev.Evento == "comando_terminado" && ev.Paso == "test" {
			require.Equal(t, "exitoso", ev.Estado)
			comandosDelPrimerPaso = append(comandosDelPrimerPaso, ev.Comando)
		}
	}
	require.Equal(t, comandosDeclarados(t, "01-test"), comandosDelPrimerPaso, "cada comando, en el orden en que corrió")
	require.NotContains(t, r.salida, "hola vex-demo", "el avance no lleva lo que imprimen los comandos: eso es de logs")
}

func TestProgreso_UnPasoQueNoSeReejecutaSoloDiceQueTermino(t *testing.T) {
	e := nuevoEntorno(t)
	intentar(t, e, e.intento(), salidaBien)

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaBien, r.codigo, r.errores)
	eventos := r.progreso(t)
	for _, ev := range eventos {
		require.NotEqual(t, "paso_iniciado", ev.Evento, "nada se reejecutó: ningún paso empezó")
		require.NotEqual(t, "comando_terminado", ev.Evento, "ni corrió ningún comando")
	}
	require.Equal(t, "precargado", eventos[len(eventos)-1].Estado)
}

func TestProgreso_UnFalloSeCuentaConElEstadoDelComandoYDelPaso(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: comando-roto\n  description: sale mal\n  cmd: exit 1\n")
	})

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaFallo, r.codigo, r.errores)
	eventos := r.progreso(t)
	require.Equal(t, paramsDeProgreso{Evento: "comando_terminado", Intento: eventos[0].Intento, Paso: "deploy", Comando: "comando-roto", Estado: "fallido"},
		eventos[len(eventos)-2])
	require.Equal(t, paramsDeProgreso{Evento: "paso_terminado", Intento: eventos[0].Intento, Paso: "deploy", Estado: "fallido"},
		eventos[len(eventos)-1])
}

func TestProgreso_LoQueNoEsUnIntentoNoCuentaNada(t *testing.T) {
	e := nuevoEntorno(t)
	casos := map[string]invocacion{
		"describir": invocar(t, rutas{}, "describir", `{}`),
		"consulta":  invocar(t, e.rutas, "intentos", `{"Version":"1","Ambiente":"prod"}`),
		"error":     invocar(t, e.rutas, "bailar", `{}`),
	}
	for nombre, r := range casos {
		t.Run(nombre, func(t *testing.T) {
			require.Empty(t, r.progreso(t))
			require.Equal(t, 1, strings.Count(r.salida, "\n"), "solo la respuesta")
		})
	}
}
