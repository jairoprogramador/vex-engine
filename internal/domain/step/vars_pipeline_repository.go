package step

import (
	"context"
)

// VarsPipelineRepository lee lo que el pipelinecode DECLARA sobre las variables
// de un step en un ambiente.
//
// Devolvía `[]command.Variable` hasta la spec 14, y ahí se perdía la mitad del
// concepto: una variable ya resuelta no puede decir de dónde salió, así que la
// declaración no llegaba al motor ni podía entrar en ninguna identidad. Ahora el
// puerto habla de declaraciones y resolverlas es trabajo del handler 02.
type VarsPipelineRepository interface {
	Get(ctx *context.Context, pipelineLocalPath, environment, step string) ([]VariableDeclaration, error)
}
