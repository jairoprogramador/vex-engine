package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

// DeclarationResolver satisface UNA declaración: le da valor a lo que el
// consumidor declaró sin decir cuánto vale (spec 14 §5.5).
//
// Es un Strategy y no un `switch`, y se justifica solo: cada origen tiene campos
// obligatorios distintos, validación distinta y sitio de lectura distinto, así
// que una función que los mezclara crecería con cada origen nuevo. El
// vocabulario cerrado de `VariableSource` es lo que hace que la selección sea
// total —no hay caso «otro»— y la versión de esquema es lo que permite que
// crecer no rompa.
type DeclarationResolver interface {
	Source() VariableSource
	Resolve(ctx *context.Context, request *StepRequestHandler, declaration VariableDeclaration) (string, error)
}

// DeclarationResolvers elige el resolutor por el origen declarado.
type DeclarationResolvers map[VariableSource]DeclarationResolver

// NewDeclarationResolvers arma el vocabulario vigente. Añadir un origen es
// añadir un resolutor aquí y una constante en `VariableSource`; el resto del
// motor no cambia.
func NewDeclarationResolvers(records RecordProvider) DeclarationResolvers {
	resolvers := make(DeclarationResolvers, 2)
	for _, resolver := range []DeclarationResolver{
		StepOutputResolver{},
		StateResolver{records: records},
	} {
		resolvers[resolver.Source()] = resolver
	}
	return resolvers
}

// Resolve devuelve el valor de la declaración, o un error que NOMBRA LA FUENTE.
//
// Ésa es la diferencia observable de §5.5: hasta esta spec el motor no podía
// distinguir «error real de plantilla» de «variable que llegará más tarde»
// porque nadie había declarado cuál es cuál, así que las dos terminaban en
// «variable no existe» — un mensaje que no dice a quién reclamarle.
func (r DeclarationResolvers) Resolve(
	ctx *context.Context,
	request *StepRequestHandler,
	declaration VariableDeclaration) (string, error) {

	resolver, ok := r[declaration.Source()]
	if !ok {
		return "", fmt.Errorf(
			"la variable '%s' declara un origen sin resolutor: '%s'",
			declaration.Name(), declaration.Source())
	}

	value, err := resolver.Resolve(ctx, request, declaration)
	if err != nil {
		return "", fmt.Errorf("resolver '%s': %w", declaration.Name(), err)
	}
	return value, nil
}

// StepOutputResolver lee del MAPA ACUMULADO, que es exactamente lo que el mapa
// acumulado pasa a ser con esta spec: una caché de resolución (§5.1').
//
// Y no del registro de estado del step productor, aunque parezca más específico:
// un step sin `config.yaml` no deja registro (spec 13 §5.3) y sus outputs sólo
// viven en el mapa, así que leer del almacén rompería el caso legítimo. El mapa
// tiene lo que el productor dejó en ESTA corrida y lo que su propio registro
// aportó cuando le tocó su turno en la cadena, que es lo mismo que el consumidor
// habría visto por nombre — la diferencia es que ahora está declarado, validado
// contra los `outputs` reales de `from` y con la garantía de que `from` corre
// antes.
type StepOutputResolver struct{}

var _ DeclarationResolver = StepOutputResolver{}

func (StepOutputResolver) Source() VariableSource { return SourceStepOutput }

func (StepOutputResolver) Resolve(
	_ *context.Context,
	request *StepRequestHandler,
	declaration VariableDeclaration) (string, error) {

	variable, found := request.AccumulatedVars().Get(declaration.Key())
	if !found {
		return "", fmt.Errorf("%s no llegó a producirse", declaration.SourceDescription())
	}
	return variable.Value(), nil
}

// StateResolver lee el registro VIGENTE de la clave de posición de ESTE step
// bajo el ámbito declarado (spec 11).
//
// «Vigente» es el último en una ejecución normal y el ANCLADO en un rollback
// (spec 28 §5.3): es una de las tres lecturas que el ancla tiene que alcanzar, y
// la más directa de las tres — lo que resuelve es con qué valor se va a
// desplegar, que es literalmente lo que un rollback existe para devolver a donde
// estaba.
//
// Es la mitad lectora de la asimetría de la spec 13 §5.4 —se leen los dos
// ámbitos, se escribe en uno— dicha en voz alta: un step de ambiente que declara
// `resolve: state, scope: project` está diciendo «esto lo dejé yo mismo en el
// ámbito común», que hasta ahora ocurría solo, por el orden de dos cargas, y sin
// que nadie pudiera saberlo leyendo el pipelinecode.
//
// Un registro ilegible es un ERROR y no una ausencia, igual que en la decisión de
// re-ejecutar (spec 11 §5.6): esa duda no se resuelve ejecutando sin arriesgar un
// recurso duplicado.
type StateResolver struct {
	records RecordProvider
}

var _ DeclarationResolver = StateResolver{}

func (StateResolver) Source() VariableSource { return SourceState }

func (r StateResolver) Resolve(
	ctx *context.Context,
	request *StepRequestHandler,
	declaration VariableDeclaration) (string, error) {

	scope, err := declaration.Scope().StateScope(request.Environment())
	if err != nil {
		return "", err
	}
	key, err := state.NewKey(request.ProjectUrl(), scope, request.StepFullName())
	if err != nil {
		return "", err
	}

	record, found, err := r.records.Current(ctx, key)
	if err != nil {
		return "", fmt.Errorf("leer %s: %w", declaration.SourceDescription(), err)
	}
	if !found {
		return "", fmt.Errorf("%s todavía no existe", declaration.SourceDescription())
	}

	for _, variable := range record.Variables() {
		if variable.Name() == declaration.Key() {
			return variable.Value(), nil
		}
	}
	return "", fmt.Errorf("%s no está en el registro", declaration.SourceDescription())
}
