package pipeline

import (
	"context"
	"fmt"

	command "github.com/jairoprogramador/vex-engine/old-internal/domain/command"
)

type InitVarsHandler struct {
	PipelineBaseHandler
}

var _ PipelineHandler = (*InitVarsHandler)(nil)

func NewInitVarsHandler() PipelineHandler {
	return &InitVarsHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
	}
}

func (h *InitVarsHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	shortHash := request.ProjectHeadHash()
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}

	// Este handler es la capa anticorrupción entre el RequestInput externo y el
	// dominio: el invariante de Variable no se sostiene en el constructor —en Go
	// el valor cero siempre es construible— sino aquí, no dejando pasar lo que
	// no se pudo construir (spec 03 §5.1').
	for _, initVar := range []struct {
		name  string
		value string
	}{
		{command.VarProjectID, request.ProjectId()},
		{command.VarProjectName, request.ProjectName()},
		{command.VarProjectOrg, request.ProjectOrg()},
		{command.VarProjectTeam, request.ProjectTeam()},
		{command.VarProjectWorkdir, request.ProjectLocalPath()},

		{command.VarProjectVersion, request.ProjectVersion()},
		{command.VarProjectRevision, shortHash},
		{command.VarProjectRevisionFull, request.ProjectHeadHash()},
		{command.VarEnvironment, request.Environment()},
		{command.VarToolName, "vex"},
	} {
		if err := h.addInitVariable(request, initVar.name, initVar.value); err != nil {
			return err
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

// addInitVariable construye y delega, nada más. Antes construía, reportaba el
// error por el log y acumulaba el valor cero de todas formas, lo que metía una
// entrada bajo la clave "" en el mapa acumulado —y de ahí a la huella de
// variables y al material de identidad—. Ahora el error sube: con el invariante
// partido el único error posible es nombre vacío, o sea un defecto del motor.
//
// Las diez son `OriginInjected`: hechos que el motor deriva de ESTA ejecución.
// Van por encima del almacén y por debajo de runtime (spec 12 §5.3), así que un
// `project_name` guardado en una corrida anterior ya no puede pisar al que se
// acaba de calcular. Hasta la spec 12 sí podía, y lo único que lo evitaba era
// que las volátiles no se persisten — la protección era la lista, no el orden.
func (h *InitVarsHandler) addInitVariable(request *PipelineRequestHandler, name, value string) error {
	variable, err := command.NewVariable(name, value, command.OriginInjected)
	if err != nil {
		return fmt.Errorf("crear variable de ejecución %q: %w", name, err)
	}
	request.AddAccumulatedVars(variable)
	return nil
}
