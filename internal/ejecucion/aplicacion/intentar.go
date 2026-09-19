package aplicacion

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Intentar es EJ-1: abre el intento en el Historial, que lo rechaza si el ambiente está ocupado (DEC-07.8);
// pide el pipeline comprobado y el material — de hoy, de un commit o de una copia de trabajo, que nunca llega a
// despliegue (DEC-10.7); pone el material del pipeline en el espacio de trabajo (EJ-5 si no se puede); y
// recorre sus pasos con el bucle explícito.
func (s *Servicio) Intentar(ctx context.Context, p publicado.PeticionDeIntento, salida publicado.Salida) (resultado publicado.Resultado, err error) {
	esCopiaDeTrabajo := p.CopiaDeTrabajo != ""

	material, err := s.traerMaterialDelProyecto(ctx, p)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	defer func() {
		if errRetirar := s.d.Fuentes.Retirar(context.WithoutCancel(ctx), material); errRetirar != nil {
			err = errors.Join(err, traducir(errRetirar))
		}
	}()

	pipeline, err := s.traerPipeline(ctx, p.FuenteDelPipeline, p.CommitDelPipeline)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	// Con una copia de trabajo, la fuente del proyecto puede venir vacía: es ella quien lo identifica.
	fuenteDelProyecto := p.FuenteDelProyecto
	if esCopiaDeTrabajo {
		fuenteDelProyecto = p.CopiaDeTrabajo
	}
	ubicacion, err := s.d.EspacioDeTrabajo.Ubicar(fuenteDelProyecto, p.FuenteDelPipeline, p.Ambiente)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	if err := s.d.EspacioDeTrabajo.RehacerParteDelMotor(ctx, ubicacion, pipeline.Pasos); err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	pasos, pasosPorNombre := indicePasos(pipeline.Pasos)
	hastaPaso := resolverHastaPaso(p.HastaPaso, pasos)

	id, err := s.d.Historial.AbrirIntento(ctx, dominio.AperturaDeIntento{
		Ambiente: p.Ambiente, Solicitante: p.Solicitante, Pasos: pasos, HastaPaso: hastaPaso,
		ConCommits: !esCopiaDeTrabajo, HashDelCodigo: material.Hash,
		FuenteDelProyecto: p.FuenteDelProyecto, CommitDelProyecto: material.Commit,
		FuenteDelPipeline: p.FuenteDelPipeline, CommitDelPipeline: pipeline.Commit,
		OrdenDeAmbientes: pipeline.Ambientes,
	})
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	intento, err := dominio.NuevoIntentoEnCurso(p.Ambiente, pasos, hastaPaso)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	c := contextoDelIntento{
		id: id, ambiente: p.Ambiente, ubicacion: ubicacion, fuenteDelPipeline: p.FuenteDelPipeline, commitDelPipeline: pipeline.Commit,
		hashDelCodigo: material.Hash, pasosPorNombre: pasosPorNombre, salida: salida,
		estandarCompartidas: estandarCompartidas(
			p.Metadatos, p.Ambiente, material.Hash.String(), material.Commit, material.Directorio, s.d.NombreDeLaHerramienta,
		),
	}
	if err := s.recorrer(ctx, c, intento); err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	return s.cerrar(ctx, id, intento, "")
}

func (s *Servicio) traerMaterialDelProyecto(ctx context.Context, p publicado.PeticionDeIntento) (dominio.Material, error) {
	switch {
	case p.CopiaDeTrabajo != "":
		return s.d.Fuentes.TraerCopiaDeTrabajo(ctx, p.CopiaDeTrabajo)
	case p.CommitDelProyecto != "":
		return s.d.Fuentes.TraerDeUnCommit(ctx, p.FuenteDelProyecto, p.CommitDelProyecto)
	default:
		return s.d.Fuentes.TraerDeHoy(ctx, p.FuenteDelProyecto)
	}
}

func (s *Servicio) traerPipeline(ctx context.Context, fuente, commit string) (dominio.Pipeline, error) {
	if commit != "" {
		return s.d.Pipelines.DeUnCommit(ctx, fuente, commit)
	}
	return s.d.Pipelines.DeHoy(ctx, fuente)
}

// indicePasos separa lo que el Historial necesita para abrir (el orden y el ámbito de todos los pasos) de lo
// que el bucle necesita de cada uno para decidirlo y ejecutarlo.
func indicePasos(pasos []dominio.PasoDeEjecucion) ([]dominio.PasoDelPipeline, map[string]dominio.PasoDeEjecucion) {
	base := make([]dominio.PasoDelPipeline, 0, len(pasos))
	porNombre := make(map[string]dominio.PasoDeEjecucion, len(pasos))
	for _, paso := range pasos {
		base = append(base, paso.PasoDelPipeline)
		porNombre[paso.Nombre()] = paso
	}
	return base, porNombre
}

// resolverHastaPaso: el Historial exige un nombre de paso explícito en su Apertura, así que "hasta el último"
// (hastaPaso vacío) se resuelve aquí, no se deja implícito — el mismo valor resuelto es el que recibe
// dominio.NuevoIntentoEnCurso.
func resolverHastaPaso(hastaPaso string, pasos []dominio.PasoDelPipeline) string {
	if hastaPaso != "" {
		return hastaPaso
	}
	return pasos[len(pasos)-1].Nombre()
}
