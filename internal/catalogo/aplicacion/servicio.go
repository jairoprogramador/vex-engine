package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Pipelines dominio.Pipelines
	Reservas  dominio.Reservas
}

// Servicio atiende las consultas que Catálogo publica hacia el borde.
type Servicio struct {
	d Dependencias
}

var _ publicado.ParaBorde = (*Servicio)(nil)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}

func (s *Servicio) pipeline(ctx context.Context, p publicado.PeticionDeCatalogo) (dominio.Pipeline, error) {
	fuente, err := dominio.NuevaFuente(p.FuenteDelPipeline)
	if err != nil {
		return dominio.Pipeline{}, err
	}
	return s.d.Pipelines.Pipeline(ctx, fuente, dominio.Commit(p.Commit))
}
