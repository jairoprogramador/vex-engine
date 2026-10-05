package aplicacion

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// HacerRollback es EJ-2: como Intentar, pero con el material y las declaraciones del commit del despliegue
// destino (DEC-03.9), con todos los pasos del pipeline, y cerrando con el destino, que será el padre del
// despliegue nuevo.
func (s *Servicio) HacerRollback(ctx context.Context, p publicado.PeticionDeRollback) (resultado publicado.Resultado, err error) {
	destino, err := s.d.Historial.DespliegueParaRollback(ctx, p.Despliegue)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	material, err := s.d.Fuentes.TraerDeUnCommit(ctx, destino.FuenteDelProyecto(), destino.CommitDelProyecto())
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	defer func() {
		if errRetirar := s.d.Fuentes.Retirar(context.WithoutCancel(ctx), material); errRetirar != nil {
			err = errors.Join(err, traducir(errRetirar))
		}
	}()

	pipeline, err := s.d.Pipelines.DeUnCommit(ctx, destino.FuenteDelPipeline(), destino.CommitDelPipeline())
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	ubicacion, err := s.d.EspacioDeTrabajo.Ubicar(destino.FuenteDelProyecto(), destino.FuenteDelPipeline(), destino.Ambiente())
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	// EJ-2 hace todos los pasos del pipeline: hasta el último, explícito (resolverHastaPaso).
	pasos, pasosPorNombre := indicePasos(pipeline.Pasos)
	hastaPaso := resolverHastaPaso("", pasos)

	id, err := s.d.Historial.AbrirIntento(ctx, dominio.AperturaDeIntento{
		Ambiente: destino.Ambiente(), Solicitante: p.Solicitante, Pasos: pasos, HastaPaso: hastaPaso,
		ConCommits: true, HashDelCodigo: material.Hash,
		FuenteDelProyecto: destino.FuenteDelProyecto(), CommitDelProyecto: destino.CommitDelProyecto(),
		FuenteDelPipeline: destino.FuenteDelPipeline(), CommitDelPipeline: destino.CommitDelPipeline(),
		OrdenDeAmbientes: pipeline.Ambientes,
	})
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}

	// Primero el Historial, que decide quién ocupa el ambiente, y solo entonces el espacio de trabajo (ver Intentar).
	if err := s.d.EspacioDeTrabajo.RehacerParteDelMotor(ctx, ubicacion, pipeline.Pasos); err != nil {
		return publicado.Resultado{}, traducir(s.abandonar(ctx, id, err))
	}

	intento, err := dominio.NuevoIntentoEnCurso(destino.Ambiente(), pasos, hastaPaso)
	if err != nil {
		return publicado.Resultado{}, traducir(s.abandonar(ctx, id, err))
	}

	c := contextoDelIntento{
		id: id, ambiente: destino.Ambiente(), ubicacion: ubicacion, fuenteDelPipeline: destino.FuenteDelPipeline(), commitDelPipeline: destino.CommitDelPipeline(),
		hashDelCodigo: material.Hash, pasosPorNombre: pasosPorNombre,
		estandarCompartidas: estandarCompartidas(
			p.Metadatos, destino.Ambiente(), material.Hash.String(), material.Commit, material.Directorio, s.d.NombreDeLaHerramienta,
		),
	}
	if err := s.recorrer(ctx, c, intento); err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	return s.cerrar(ctx, id, intento, destino.Despliegue())
}
