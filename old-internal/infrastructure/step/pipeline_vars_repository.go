package step

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	"gopkg.in/yaml.v3"
)

type pipelineVarsRepository struct{}

func NewPipelineVarsRepository() domStep.VarsPipelineRepository {
	return &pipelineVarsRepository{}
}

// Get devuelve lo que el pipelinecode DECLARA para un step en un ambiente, no lo
// que valga.
//
// El puerto devolvía `[]command.Variable` —nombre y valor ya resueltos— y por eso
// la declaración no llegaba a ningún sitio: se perdía en el borde. Devuelve
// `[]VariableDeclaration` desde la spec 14, y quien la convierte en un valor es
// el handler 02, que es el que sabe contra qué resolverla.
func (r *pipelineVarsRepository) Get(ctx *context.Context, pipelineLocalPath, environment, step string) ([]domStep.VariableDeclaration, error) {
	return r.readDeclarationsFromFile(*ctx, filepath.Join(pipelineLocalPath, "variables", environment, step+".yaml"))
}

func (r *pipelineVarsRepository) readDeclarationsFromFile(_ context.Context, filePath string) ([]domStep.VariableDeclaration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []domStep.VariableDeclaration{}, nil
		}
		return nil, err
	}

	var variablesDTO []PipelineVariableDTO
	if err := yaml.Unmarshal(data, &variablesDTO); err != nil {
		return nil, fmt.Errorf("parsear YAML de variables '%s': %w", filePath, err)
	}

	declarations := make([]domStep.VariableDeclaration, 0, len(variablesDTO))
	for _, vDTO := range variablesDTO {
		declaration, err := declarationOf(vDTO)
		if err != nil {
			return nil, fmt.Errorf("variable inválida en '%s': %w", filePath, err)
		}
		declarations = append(declarations, declaration)
	}
	return declarations, nil
}

// declarationOf elige el constructor por el `resolve:` declarado, y cada uno
// exige SUS campos.
//
// Que los campos obligatorios los pida el constructor y no un `if` de aquí es lo
// que hace que el vocabulario crezca añadiendo un valor (OCP, §5.2') en vez de
// engordando una función. Desde la spec 03 una entrada con `value:` vacío es
// legítima —un parámetro declarado sin valor—, así que el único rechazo del caso
// literal sigue siendo la entrada sin `name:`.
func declarationOf(dto PipelineVariableDTO) (domStep.VariableDeclaration, error) {
	source, err := domStep.NewVariableSource(dto.Resolve)
	if err != nil {
		return domStep.VariableDeclaration{}, fmt.Errorf("variable '%s': %w", dto.Name, err)
	}

	switch source {
	case domStep.SourceStepOutput:
		return domStep.NewStepOutputDeclaration(dto.Name, dto.From, dto.Key)
	case domStep.SourceState:
		return domStep.NewStateDeclaration(dto.Name, dto.Scope, dto.Key)
	default:
		return domStep.NewLiteralDeclaration(dto.Name, dto.Value)
	}
}
