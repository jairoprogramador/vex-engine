package aplicacion_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// Los dobles de este fichero son los puertos que el propio dominio de Ejecución declara (dominio.Pipelines,
// dominio.Fuentes, dominio.Variables, dominio.Historial, dominio.Comandos, dominio.EspacioDeTrabajo) — nunca
// los servicios reales de Definición, Suministro, Resolución o Historial: probar contra la frontera que
// Ejecución ya decidió es lo que mantiene un contexto aislado del otro (DEC-11.3).

func pasoSimple(t *testing.T, nombre string, compartido bool) dominio.PasoDeEjecucion {
	t.Helper()
	base, err := dominio.NuevoPasoDelPipeline(nombre, compartido)
	require.NoError(t, err)
	regla, err := dominio.NuevaRegla(true, true, true, 0)
	require.NoError(t, err)
	comando, err := dominio.NuevoComandoDeclarado(nombre, "echo "+nombre, "", nil, nil, nil)
	require.NoError(t, err)
	return dominio.PasoDeEjecucion{PasoDelPipeline: base, Regla: regla, Comandos: []dominio.ComandoDeclarado{comando}}
}

type pipelinesFalsos struct {
	pipeline dominio.Pipeline
	err      error
	estandar []dominio.VariableEstandar
}

func (p *pipelinesFalsos) DeHoy(context.Context, string) (dominio.Pipeline, error) {
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) DeUnCommit(context.Context, string, string) (dominio.Pipeline, error) {
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) VariablesEstandar() []dominio.VariableEstandar { return p.estandar }

var _ dominio.Pipelines = (*pipelinesFalsos)(nil)

type fuentesFalsas struct {
	material  dominio.Material
	err       error
	retirados []dominio.Material
}

func (f *fuentesFalsas) TraerDeHoy(context.Context, string) (dominio.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) TraerDeUnCommit(context.Context, string, string) (dominio.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) TraerCopiaDeTrabajo(context.Context, string) (dominio.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) Retirar(_ context.Context, m dominio.Material) error {
	f.retirados = append(f.retirados, m)
	return nil
}

var _ dominio.Fuentes = (*fuentesFalsas)(nil)

type variablesFalsas struct {
	mu             sync.Mutex
	err            error
	noReejecutados []string
	producidas     []string

	// errInterpolar hace fallar solo a Interpolar —no a declarar variables—, que es lo que pasa cuando un
	// comando usa una variable que no está disponible: el paso ya empezó cuando se descubre.
	errInterpolar error
	// interpolados son los textos que Interpolar resuelve a otro valor; los demás los devuelve tal cual.
	interpolados map[string]string
	// errPorTexto hace fallar a Interpolar solo con esos textos.
	errPorTexto map[string]error
	// alUsarVariables, si no es nil, se llama la primera vez que se pide algo a Resolución, para cancelar el
	// ctx a mitad de un paso, como haría una cancelación de verdad.
	alUsarVariables func()
}

func (v *variablesFalsas) DeclararVariablesDeUnPaso(
	context.Context, string, string, dominio.Ambito, string, string, map[string]string,
) error {
	if v.alUsarVariables != nil {
		v.alUsarVariables()
	}
	return v.err
}

func (v *variablesFalsas) Interpolar(_ context.Context, _, _ string, _ dominio.Ambito, texto string) (string, error) {
	if v.alUsarVariables != nil {
		v.alUsarVariables()
	}
	if v.errInterpolar != nil {
		return "", v.errInterpolar
	}
	if err := v.errPorTexto[texto]; err != nil {
		return "", err
	}
	if valor, hay := v.interpolados[texto]; hay {
		return valor, v.err
	}
	return texto, v.err
}

// hashDeVariablesDePrueba es el que da variablesFalsas: siempre el mismo, así que las variables «no cambian» salvo
// que una prueba guarde otro en la última vez del paso.
const hashDeVariablesDePrueba = "variables-de-prueba"

func (v *variablesFalsas) HashDeLasVariables(_ context.Context, _ string, _ dominio.Ambito, _ []string) (dominio.HashDeVariables, error) {
	return dominio.NuevoHashDeVariables(hashDeVariablesDePrueba), v.err
}

func (v *variablesFalsas) RegistrarProducido(_ context.Context, _, paso, nombre, valor string, _ dominio.Ambito) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.producidas = append(v.producidas, paso+":"+nombre+"="+valor)
	return v.err
}

func (v *variablesFalsas) NoReejecutado(_ context.Context, _, paso string, _ dominio.Ambito) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.noReejecutados = append(v.noReejecutados, paso)
	return v.err
}

var _ dominio.Variables = (*variablesFalsas)(nil)

type registroDeHistorial struct{ tipo, paso string }

type salidaRegistrada struct {
	paso, comando string
	exitoso       bool
	texto         string
}

// historialFalso guarda en memoria lo que se le registra, para que las pruebas comprueben el orden de las
// escrituras y qué se cerró, sin depender del Historial real.
type historialFalso struct {
	mu sync.Mutex
	t  *testing.T

	errAbrir  error
	idAbierto string
	// destinoConsultado dice si se llegó a pedir el despliegue de un rollback.
	destinoConsultado bool
	aperturas         []dominio.AperturaDeIntento

	abandonados  []string
	errAbandonar error

	ultimaVezPorPaso map[string]dominio.UltimaVezDeUnPaso
	fallarRegistrar  map[string]bool
	registros        []registroDeHistorial
	salidas          []salidaRegistrada

	// recuerda hace que cada paso que termina bien, o que no se re-ejecuta, quede como su última vez, con los
	// recursos con que se registró: así un segundo intento decide contra lo que dejó el primero.
	recuerda bool

	// intervaloDeLatido es el que publica el Historial; cero desactiva el latido.
	intervaloDeLatido time.Duration
	latidos           int
	errLatir          error
	causaCerrada      dominio.Causa

	// errCerrar simula un almacén que no responde al cerrar: el intento no llega a cerrarse.
	errCerrar         error
	cerrado           bool
	desenlaceCerrado  dominio.Desenlace
	destinoCerrado    string
	despliegueACerrar string

	detalle dominio.DetalleDelIntento

	destinoParaRollback    dominio.Destino
	errDestinoParaRollback error
}

func nuevoHistorialFalso(t *testing.T) *historialFalso {
	t.Helper()
	return &historialFalso{t: t, idAbierto: "int-1", ultimaVezPorPaso: map[string]dominio.UltimaVezDeUnPaso{}, fallarRegistrar: map[string]bool{}}
}

func (h *historialFalso) AbrirIntento(_ context.Context, a dominio.AperturaDeIntento) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.aperturas = append(h.aperturas, a)
	if h.errAbrir != nil {
		return "", h.errAbrir
	}
	return h.idAbierto, nil
}

func (h *historialFalso) AbandonarIntento(_ context.Context, intento string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.abandonados = append(h.abandonados, intento)
	return h.errAbandonar
}

func (h *historialFalso) RegistrarComienzo(_ context.Context, _, paso string, _ dominio.RecursosDeUnPaso) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallarRegistrar[paso] {
		return errors.New("falla simulada de RegistrarComienzo")
	}
	h.registros = append(h.registros, registroDeHistorial{"comienzo", paso})
	return nil
}

func (h *historialFalso) RegistrarFinal(_ context.Context, _, paso string, exitoso bool, recursos dominio.RecursosDeUnPaso) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallarRegistrar[paso] {
		return errors.New("falla simulada de RegistrarFinal")
	}
	tipo := "final-exitoso"
	if !exitoso {
		tipo = "final-fallido"
	}
	h.registros = append(h.registros, registroDeHistorial{tipo, paso})
	if exitoso {
		h.recordar(paso, recursos)
	}
	return nil
}

func (h *historialFalso) recordar(paso string, recursos dominio.RecursosDeUnPaso) {
	if !h.recuerda {
		return
	}
	h.ultimaVezPorPaso[paso] = dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true, Recursos: recursos, Evidencia: dominio.Evidencia{Intento: h.idAbierto, Paso: paso},
	}
}

func (h *historialFalso) RegistrarSalida(_ context.Context, _, paso, comando string, exitoso bool, texto string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.salidas = append(h.salidas, salidaRegistrada{paso, comando, exitoso, texto})
	return nil
}

func (h *historialFalso) RegistrarNoReejecucion(
	_ context.Context, _, paso string, _ dominio.Evidencia, recursos dominio.RecursosDeUnPaso,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallarRegistrar[paso] {
		return errors.New("falla simulada de RegistrarNoReejecucion")
	}
	h.registros = append(h.registros, registroDeHistorial{"no-reejecucion", paso})
	h.recordar(paso, recursos)
	return nil
}

func (h *historialFalso) cantidadDeLatidos() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.latidos
}

func (h *historialFalso) IntervaloDeLatido() time.Duration { return h.intervaloDeLatido }

func (h *historialFalso) Latir(context.Context, string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latidos++
	return h.errLatir
}

func (h *historialFalso) CerrarIntento(
	_ context.Context, _ string, desenlace dominio.Desenlace, causa dominio.Causa, destino string,
) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.errCerrar != nil {
		return "", false, h.errCerrar
	}
	h.cerrado = true
	h.desenlaceCerrado = desenlace
	h.causaCerrada = causa
	h.destinoCerrado = destino
	return h.despliegueACerrar, h.despliegueACerrar != "", nil
}

func (h *historialFalso) UltimaVezDeUnPaso(_ context.Context, paso string, _ dominio.Ambito) (dominio.UltimaVezDeUnPaso, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ultimaVezPorPaso[paso], nil
}

func (h *historialFalso) DespliegueParaRollback(context.Context, string) (dominio.Destino, error) {
	h.destinoConsultado = true
	return h.destinoParaRollback, h.errDestinoParaRollback
}

func (h *historialFalso) DetalleDelIntento(context.Context, string) (dominio.DetalleDelIntento, error) {
	return h.detalle, nil
}

var _ dominio.Historial = (*historialFalso)(nil)

type comandosFalsos struct {
	mu        sync.Mutex
	resultado dominio.ResultadoDeUnComando
	err       error
	llamados  []string
	entornos  []dominio.Entorno // el que recibió cada llamada

	// trozos, si no está vacío, es lo que el comando escribe en su salida, una escritura por trozo, en vez de
	// «salida de <línea>».
	trozos []string

	// bitacora, si no es nil, recibe «comando <línea>» en cada llamada: para comprobar el orden con otros dobles.
	bitacora *[]string

	// alEjecutar, si no es nil, se llama antes de devolver — la prueba de EJ-3 lo usa para cancelar el ctx a
	// mitad de un comando, como haría una cancelación de verdad.
	alEjecutar func()
}

func (c *comandosFalsos) Ejecutar(
	_ context.Context, _, lineaInterpolada string, _ dominio.ComandoDeclarado, entorno dominio.Entorno, salida *dominio.SalidaDeComando,
) (dominio.ResultadoDeUnComando, error) {
	c.mu.Lock()
	c.llamados = append(c.llamados, lineaInterpolada)
	c.entornos = append(c.entornos, entorno)
	if c.bitacora != nil {
		*c.bitacora = append(*c.bitacora, "comando "+lineaInterpolada)
	}
	c.mu.Unlock()
	if salida != nil {
		trozos := c.trozos
		if len(trozos) == 0 {
			trozos = []string{"salida de " + lineaInterpolada}
		}
		for _, trozo := range trozos {
			_, _ = salida.Write([]byte(trozo))
		}
	}
	if c.alEjecutar != nil {
		c.alEjecutar()
	}
	return c.resultado, c.err
}

var _ dominio.Comandos = (*comandosFalsos)(nil)

type espacioDeTrabajoFalso struct {
	errRehacer  error
	rehecho     bool
	interpolado bool

	// pasosRehechos son los pasos cuyo directorio se rehízo cuando les tocó, en orden.
	pasosRehechos    []string
	errRehacerElPaso error
	// bitacora, si no es nil, recibe «rehacer <paso>» cada vez: para comprobar el orden con otros dobles.
	bitacora *[]string
}

func (e *espacioDeTrabajoFalso) RehacerElPaso(_ context.Context, _ dominio.Ubicacion, paso dominio.PasoDeEjecucion) error {
	e.pasosRehechos = append(e.pasosRehechos, paso.Nombre())
	if e.bitacora != nil {
		*e.bitacora = append(*e.bitacora, "rehacer "+paso.Nombre())
	}
	return e.errRehacerElPaso
}

func (e *espacioDeTrabajoFalso) Ubicar(_, _, ambiente string) (dominio.Ubicacion, error) {
	return dominio.Ubicacion{Proyecto: "proyecto", Pipeline: "pipeline", Ambiente: ambiente}, nil
}

func (e *espacioDeTrabajoFalso) RehacerParteDelMotor(context.Context, dominio.Ubicacion, []dominio.PasoDeEjecucion) error {
	e.rehecho = true
	return e.errRehacer
}

func (e *espacioDeTrabajoFalso) DirectorioDelPaso(u dominio.Ubicacion, paso string) string {
	return "/ws/" + u.Proyecto + "/" + u.Pipeline + "/" + u.Ambiente + "/" + paso
}

func (e *espacioDeTrabajoFalso) InterpolarPlantillas(context.Context, dominio.Ubicacion, dominio.PasoDeEjecucion, dominio.Interpolador) error {
	e.interpolado = true
	return nil
}

var _ dominio.EspacioDeTrabajo = (*espacioDeTrabajoFalso)(nil)

// rutaDelEfimeroDePrueba es la ruta que da efimeroFalso: siempre la misma, como el de verdad.
const rutaDelEfimeroDePrueba = "/tmp/efimero-de-prueba"

// efimeroFalso apunta en la bitácora cuándo se crea y se borra el directorio efímero.
type efimeroFalso struct {
	bitacora  *[]string
	errCrear  error
	errBorrar error
	// ctxDeBorrar es el estado del ctx con que se pidió el último borrado: nil si no estaba cancelado.
	ctxDeBorrar error
}

func (e *efimeroFalso) Crear(context.Context) (string, error) {
	*e.bitacora = append(*e.bitacora, "crear efímero")
	return rutaDelEfimeroDePrueba, e.errCrear
}

func (e *efimeroFalso) Borrar(ctx context.Context) error {
	*e.bitacora = append(*e.bitacora, "borrar efímero")
	e.ctxDeBorrar = ctx.Err()
	return e.errBorrar
}

var _ dominio.DirectorioEfimero = (*efimeroFalso)(nil)

// supervivientesFalsos apunta en la bitácora cuándo se acaba con los procesos que dejó un paso.
type supervivientesFalsos struct {
	bitacora *[]string
	err      error
}

func (s *supervivientesFalsos) Acabar(context.Context) error {
	*s.bitacora = append(*s.bitacora, "acabar con los supervivientes")
	return s.err
}

var _ dominio.Supervivientes = (*supervivientesFalsos)(nil)
