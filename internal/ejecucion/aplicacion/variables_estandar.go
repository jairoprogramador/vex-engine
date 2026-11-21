package aplicacion

import (
	"maps"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// El puente hacia Resolución: construye los valores de las variables estándar que
// resolucion/publicado.ParaEjecucion.VariablesDeUnPaso recibe en estandar — el parámetro que existía solo
// "mientras RD-06 no exista para darlos de otra forma" (ver resolucion/publicado/operaciones.go y
// resolucion/dominio/puertos.go). Los nombres son los de RD-04 §9, hallazgo 1: metadatos que da quien invoca, y
// los que genera el motor — compartidos durante todo el intento, o propios de cada paso mientras corre.

// estandarCompartidas se calcula una vez por intento: no cambia entre pasos. directorioDelProyecto es el
// material del proyecto que trajo Suministro (Material.Directorio) — no el espacio de trabajo del pipeline:
// project_workdir es de donde vive el código a desplegar, para que un comando pueda `cd` ahí.
func estandarCompartidas(
	meta publicado.Metadatos, ambiente, hashDelCodigo, commitDelProyecto, directorioDelProyecto, herramienta string,
) map[string]string {
	return map[string]string{
		"project_id":           meta.ProjectId,
		"project_name":         meta.ProjectName,
		"project_organization": meta.ProjectOrganization,
		"project_team":         meta.ProjectTeam,
		"environment":          ambiente,
		"project_hash":         hashDelCodigo,
		"project_version":      commitDelProyecto,
		"project_workdir":      directorioDelProyecto,
		"tool_name":            herramienta,
	}
}

// comprobarMetadatos rechaza, antes de abrir ni traer nada, un metadato que no sea un nombre simple
// (dominio.ComprobarMetadato): llegan de quien invoca y se interpolan tal cual en la línea de un comando, que
// corre con `sh -c`. Lo demás que se interpola no lo escribe quien invoca: el hash y el commit los calcula
// Suministro, y los nombres de los pasos son del pipeline.
func comprobarMetadatos(meta publicado.Metadatos) error {
	for _, m := range []struct{ campo, valor string }{
		{"Metadatos.ProjectId", meta.ProjectId},
		{"Metadatos.ProjectName", meta.ProjectName},
		{"Metadatos.ProjectOrganization", meta.ProjectOrganization},
		{"Metadatos.ProjectTeam", meta.ProjectTeam},
	} {
		if err := dominio.ComprobarMetadato(m.campo, m.valor); err != nil {
			return err
		}
	}
	return nil
}

// conElPaso añade las dos que son de cada paso, sin mutar el mapa compartido.
func conElPaso(compartidas map[string]string, nombreDelPaso, directorioDelPaso string) map[string]string {
	estandar := make(map[string]string, len(compartidas)+2)
	maps.Copy(estandar, compartidas)
	estandar["step_name"] = nombreDelPaso
	estandar["step_workdir"] = directorioDelPaso
	return estandar
}
