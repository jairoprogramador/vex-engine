package aplicacion_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
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
	mu               sync.Mutex
	cambiaronPorPaso map[string]bool
	err              error
	noReejecutados   []string
	producidas       []string
}

func (v *variablesFalsas) DeclararVariablesDeUnPaso(
	context.Context, string, string, dominio.Ambito, string, string, map[string]string,
) error {
	return v.err
}

func (v *variablesFalsas) Interpolar(_ context.Context, _, _ string, _ dominio.Ambito, texto string) (string, error) {
	return texto, v.err
}

func (v *variablesFalsas) CambiaronLasVariables(_ context.Context, _, paso string, _ dominio.Ambito) (bool, error) {
	return v.cambiaronPorPaso[paso], v.err
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

// historialFalso guarda en memoria lo que se le registra, para que las pruebas comprueben el orden de las
// escrituras y qué se cerró, sin depender del Historial real.
type historialFalso struct {
	mu sync.Mutex
	t  *testing.T

	errAbrir  error
	idAbierto string
	aperturas []dominio.AperturaDeIntento

	ultimaVezPorPaso map[string]dominio.UltimaVezDeUnPaso
	fallarRegistrar  map[string]bool
	registros        []registroDeHistorial

	cerrado           bool
	desenlaceCerrado  dominio.Desenlace
	destinoCerrado    string
	despliegueACerrar string

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

func (h *historialFalso) RegistrarComienzo(_ context.Context, _, paso string, _ dominio.RecursosDeUnPaso) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallarRegistrar[paso] {
		return errors.New("falla simulada de RegistrarComienzo")
	}
	h.registros = append(h.registros, registroDeHistorial{"comienzo", paso})
	return nil
}

func (h *historialFalso) RegistrarFinal(_ context.Context, _, paso string, exitoso bool, _ dominio.RecursosDeUnPaso) error {
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
	return nil
}

func (h *historialFalso) RegistrarNoReejecucion(
	_ context.Context, _, paso string, _ dominio.Evidencia, _ dominio.RecursosDeUnPaso,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fallarRegistrar[paso] {
		return errors.New("falla simulada de RegistrarNoReejecucion")
	}
	h.registros = append(h.registros, registroDeHistorial{"no-reejecucion", paso})
	return nil
}

func (h *historialFalso) CerrarIntento(
	_ context.Context, _ string, desenlace dominio.Desenlace, destino string,
) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cerrado = true
	h.desenlaceCerrado = desenlace
	h.destinoCerrado = destino
	return h.despliegueACerrar, h.despliegueACerrar != "", nil
}

func (h *historialFalso) UltimaVezDeUnPaso(_ context.Context, paso string, _ dominio.Ambito) (dominio.UltimaVezDeUnPaso, error) {
	return h.ultimaVezPorPaso[paso], nil
}

func (h *historialFalso) DespliegueParaRollback(context.Context, string) (dominio.Destino, error) {
	return h.destinoParaRollback, h.errDestinoParaRollback
}

var _ dominio.Historial = (*historialFalso)(nil)

type comandosFalsos struct {
	mu        sync.Mutex
	resultado dominio.ResultadoDeUnComando
	err       error
	llamados  []string

	// alEjecutar, si no es nil, se llama antes de devolver — la prueba de EJ-3 lo usa para cancelar el ctx a
	// mitad de un comando, como haría una cancelación de verdad.
	alEjecutar func()
}

func (c *comandosFalsos) Ejecutar(
	_ context.Context, _, lineaInterpolada string, _ dominio.ComandoDeclarado, salida io.Writer,
) (dominio.ResultadoDeUnComando, error) {
	c.mu.Lock()
	c.llamados = append(c.llamados, lineaInterpolada)
	c.mu.Unlock()
	if salida != nil {
		_, _ = salida.Write([]byte("salida de " + lineaInterpolada))
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

type salidaFalsa struct {
	mu       sync.Mutex
	recibido []string
}

func (s *salidaFalsa) Escribir(paso string, datos []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recibido = append(s.recibido, paso+":"+string(datos))
	return nil
}

var _ publicado.Salida = (*salidaFalsa)(nil)
