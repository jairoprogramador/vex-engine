package aplicacion_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// progresoFalso recoge lo que Ejecución cuenta de su avance, en el orden en que lo cuenta.
type progresoFalso struct {
	mu      sync.Mutex
	eventos []dominio.EventoDeProgreso
	// contextosCancelados cuenta los eventos que llegaron con un contexto ya cancelado: el avance de una
	// cancelación es justo el que más interesa que llegue.
	contextosCancelados int
	// alEmitir, si no es nil, se llama con cada evento antes de guardarlo.
	alEmitir func(dominio.EventoDeProgreso)
}

func (p *progresoFalso) Emitir(ctx context.Context, e dominio.EventoDeProgreso) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		p.contextosCancelados++
	}
	if p.alEmitir != nil {
		p.alEmitir(e)
	}
	p.eventos = append(p.eventos, e)
}

var _ dominio.Progreso = (*progresoFalso)(nil)

// descritos son los eventos como texto corto: el tipo, y lo que lleven de paso, comando y estado.
func (p *progresoFalso) descritos() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	textos := make([]string, 0, len(p.eventos))
	for _, e := range p.eventos {
		texto := string(e.Tipo)
		for _, parte := range []string{e.Paso, e.Comando, e.Estado} {
			if parte != "" {
				texto += ":" + parte
			}
		}
		textos = append(textos, texto)
	}
	return textos
}

// ultimaVezValidaDeUnPaso es lo que el Historial diría de un paso que ya se hizo bien y no cambió.
func ultimaVezValidaDeUnPaso(t *testing.T, d *dependenciasDePrueba, nombre string) dominio.UltimaVezDeUnPaso {
	t.Helper()
	paso := pasoSimple(t, nombre, false)
	hashDeInstrucciones := dominio.CalcularHashDeInstrucciones(paso.Comandos, paso.Material)
	return dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true,
		Recursos:  dominio.NuevosRecursosDeUnPaso(d.fuentes.material.Hash, hashDeInstrucciones, dominio.NuevoHashDeVariables(hashDeVariablesDePrueba)),
		Evidencia: dominio.Evidencia{Intento: "int-viejo", Paso: nombre},
	}
}

func TestProgreso_UnIntentoExitosoCuentaCadaPasoYCadaComandoEnOrden(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, []string{
		"intento_iniciado",
		"paso_iniciado:01-pruebas",
		"comando_terminado:01-pruebas:01-pruebas:exitoso",
		"paso_terminado:01-pruebas:ejecutado",
		"paso_iniciado:02-despliegue",
		"comando_terminado:02-despliegue:02-despliegue:exitoso",
		"paso_terminado:02-despliegue:ejecutado",
	}, d.progreso.descritos())
	for _, e := range d.progreso.eventos {
		require.Equal(t, "int-1", e.Intento, "todos dicen de qué intento son")
	}
}

func TestProgreso_UnComandoQueFallaCuentaElFalloDelComandoYDelPaso(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, []string{
		"intento_iniciado",
		"paso_iniciado:01-pruebas",
		"comando_terminado:01-pruebas:01-pruebas:fallido",
		"paso_terminado:01-pruebas:fallido",
	}, d.progreso.descritos(), "un fallo cierra el intento: el segundo paso no empieza")
}

func TestProgreso_LaCancelacionSeCuentaYLosEventosLlegan(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	ctx, cancelar := context.WithCancel(context.Background())
	d.comandos.alEjecutar = cancelar
	d.comandos.err = context.Canceled
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(ctx, peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, []string{
		"intento_iniciado",
		"paso_iniciado:01-pruebas",
		"comando_terminado:01-pruebas:01-pruebas:fallido",
		"paso_terminado:01-pruebas:cancelado",
	}, d.progreso.descritos())
	require.Zero(t, d.progreso.contextosCancelados, "se emiten sin depender del contexto cancelado, como el registro")
}

func TestProgreso_UnPasoQueNoSeReejecutaSoloCuentaQueTermino(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.ultimaVezPorPaso["01-pruebas"] = ultimaVezValidaDeUnPaso(t, d, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, []string{
		"intento_iniciado",
		"paso_terminado:01-pruebas:precargado",
	}, d.progreso.descritos(), "no empezó ni corrió comandos: lo que hubo se da por bueno")
}

func TestProgreso_ElComandoTerminadoSeCuentaCuandoSuSalidaYaEstaEnElHistorial(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	var salidasAlAvisar []int
	d.progreso.alEmitir = func(e dominio.EventoDeProgreso) {
		if e.Tipo == dominio.ComandoTerminado {
			salidasAlAvisar = append(salidasAlAvisar, len(d.historial.salidas))
		}
	}
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, []int{1}, salidasAlAvisar, "quien lo ve puede pedir logs y encontrar ya esa salida")
}

func TestProgreso_SiElIntentoNoLlegaAEmpezarNoSeCuentaNada(t *testing.T) {
	casos := map[string]func(*dependenciasDePrueba){
		"el ambiente está ocupado": func(d *dependenciasDePrueba) { d.historial.errAbrir = fmt.Errorf("ambiente ocupado") },
		"el espacio de trabajo no está disponible": func(d *dependenciasDePrueba) {
			d.espacioDeTrabajo.errRehacer = fmt.Errorf("%w: disco lleno", dominio.ErrNoDisponible)
		},
	}
	for nombre, preparar := range casos {
		t.Run(nombre, func(t *testing.T) {
			deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
			preparar(d)

			_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba())

			require.Error(t, err)
			require.Empty(t, d.progreso.descritos(), "EJ-5: el intento no empieza")
		})
	}
}

func TestProgreso_UnRollbackTambienCuentaSuAvance(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	})

	require.NoError(t, err)
	descritos := d.progreso.descritos()
	require.Equal(t, "intento_iniciado", descritos[0])
	require.Equal(t, "paso_terminado:01-pruebas:ejecutado", descritos[len(descritos)-1])
}

func TestProgreso_SinPuertoDeProgresoElIntentoFunciona(t *testing.T) {
	deps, _ := nuevasDependenciasDePrueba(t, "01-pruebas")
	deps.Progreso = nil

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado, "el progreso es una cortesía: no es obligatorio")
}
