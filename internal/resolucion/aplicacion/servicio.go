package aplicacion

import (
	"context"
	"sync"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Historial  dominio.Historial
	Definicion dominio.Definicion
}

// Servicio guarda, mientras dura cada invocación (DEC-08.3), su agregado VariablesDeUnaInvocacion — uno por
// intento real y uno por simulación, bajo la misma llave: el dominio no distingue entre las dos (DEC-04.10).
// Servicio no implementa ningún puerto publicado directamente: ParaEjecucion.Interpolar y
// ParaSimulacion.Interpolar (lo mismo con RegistrarProducido) tienen firmas distintas y Go no permite dos
// métodos con el mismo nombre en un solo tipo, así que cada puerto lo implementa su propia vista delgada
// (paraEjecucion, paraSimulacion) que delega en Servicio.
type Servicio struct {
	d            Dependencias
	mu           sync.Mutex
	invocaciones map[string]*dominio.VariablesDeUnaInvocacion
}

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d, invocaciones: map[string]*dominio.VariablesDeUnaInvocacion{}}
}

// ParaEjecucion da la vista de este servicio que implementa publicado.ParaEjecucion.
func (s *Servicio) ParaEjecucion() publicado.ParaEjecucion { return paraEjecucion{s} }

// ParaSimulacion da la vista de este servicio que implementa publicado.ParaSimulacion.
func (s *Servicio) ParaSimulacion() publicado.ParaSimulacion { return paraSimulacion{s} }

type paraEjecucion struct{ s *Servicio }
type paraSimulacion struct{ s *Servicio }

var (
	_ publicado.ParaEjecucion  = paraEjecucion{}
	_ publicado.ParaSimulacion = paraSimulacion{}
)

func (p paraEjecucion) VariablesDeUnPaso(
	ctx context.Context, intento, paso string, ambito publicado.Ambito, fuente, commit string,
	estandar map[string]string,
) ([]publicado.Variable, error) {
	return p.s.variablesDeUnPaso(ctx, intento, paso, ambito, fuente, commit, estandar)
}

func (p paraEjecucion) Interpolar(
	ctx context.Context, intento, paso string, ambito publicado.Ambito, texto string,
) (string, error) {
	return p.s.interpolar(ctx, intento, ambito, texto)
}

func (p paraEjecucion) HashDeLasVariables(
	ctx context.Context, intento string, ambito publicado.Ambito, textos []string,
) (string, error) {
	return p.s.hashDeLasVariables(ctx, intento, ambito, textos)
}

func (p paraEjecucion) RegistrarProducido(
	ctx context.Context, intento, paso, nombre, valor string, ambito publicado.Ambito,
) error {
	return p.s.registrarProducido(ctx, intento, paso, nombre, valor, ambito)
}

func (p paraEjecucion) NoReejecutado(ctx context.Context, intento, paso string, ambito publicado.Ambito) error {
	return p.s.noReejecutado(ctx, intento, paso, ambito)
}

func (p paraSimulacion) Declarar(
	ctx context.Context, simulacion string, ambito publicado.Ambito, declaradas []publicado.VariableDeclarada,
) error {
	return p.s.declarar(ctx, simulacion, ambito, declaradas)
}

func (p paraSimulacion) Interpolar(
	ctx context.Context, simulacion string, ambito publicado.Ambito, texto string,
) (string, error) {
	return p.s.interpolar(ctx, simulacion, ambito, texto)
}

func (p paraSimulacion) RegistrarProducido(
	ctx context.Context, simulacion, nombre, valor string, ambito publicado.Ambito,
) error {
	return p.s.producirEnMemoria(simulacion, nombre, valor, ambito)
}

func (p paraSimulacion) Cerrar(ctx context.Context, simulacion string) error {
	return p.s.cerrar(simulacion)
}

// invocacion da el agregado de esa invocación, creándolo si es la primera vez que se pide.
func (s *Servicio) invocacion(id string) *dominio.VariablesDeUnaInvocacion {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.invocaciones[id]
	if !ok {
		inv = dominio.NuevaInvocacion()
		s.invocaciones[id] = inv
	}
	return inv
}

func (s *Servicio) cerrar(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.invocaciones, id)
	return nil
}
