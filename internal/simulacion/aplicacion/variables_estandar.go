package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Las variables estándar de una simulación, como las de un intento (ejecucion/aplicacion/variables_estandar.go):
// los metadatos los da quien invoca; environment y step_name son los reales; las demás las genera el motor a
// partir de lo que un intento trae de verdad — el material del proyecto, el espacio de trabajo —, que una
// simulación no toca. Esas llevan un valor simulado: la simulación nunca devuelve valores, solo nombres, y lo
// único que importa es que existan para interpolar.
const (
	hashSimulado       = "contenido-v1:simulado"
	versionSimulada    = "simulada"
	directorioSimulado = "/simulado"
	herramienta        = "vexd"
)

// estandarCompartidas se calcula una vez por ambiente: no cambia entre pasos.
func estandarCompartidas(meta publicado.Metadatos, ambiente string) map[string]string {
	return map[string]string{
		"project_id":           meta.ProjectId,
		"project_name":         meta.ProjectName,
		"project_organization": meta.ProjectOrganization,
		"project_team":         meta.ProjectTeam,
		"environment":          ambiente,
		"project_hash":         hashSimulado,
		"project_version":      versionSimulada,
		"project_workdir":      directorioSimulado,
		"tool_name":            herramienta,
	}
}

// estandarDelPaso son las dos que son de cada paso.
func estandarDelPaso(paso dominio.Paso) map[string]string {
	return map[string]string{
		"step_name":    paso.Nombre,
		"step_workdir": directorioSimulado + "/" + paso.Nombre,
	}
}

// comoDeclaradas lleva los valores a literales, cada uno en su ámbito: las compartidas en el compartido y las
// del paso en el del ambiente — el mismo reparto que hace Resolución en un intento real. Con delPaso se
// eligen unas u otras.
func comoDeclaradas(
	estandar []dominio.VariableEstandar, valores map[string]string, delPaso bool, ambito dominio.Ambito,
) []dominio.VariableDeclarada {
	destino := dominio.AmbitoCompartido()
	if delPaso {
		destino = ambito
	}
	var declaradas []dominio.VariableDeclarada
	for _, e := range estandar {
		if e.DelPaso != delPaso {
			continue
		}
		declaradas = append(declaradas, dominio.VariableDeclarada{Nombre: e.Nombre, Ambito: destino, Valor: valores[e.Nombre]})
	}
	return declaradas
}
