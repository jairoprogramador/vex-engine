package aplicacion

import (
	"context"
	"errors"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
	"github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Repositorios dominio.Repositorios
	Hashes       dominio.Hashes
}

// Servicio es traer una fuente: el acceso a los repositorios pone delante el material, y el hash se calcula
// después sobre lo que quedó delante. Así el mismo contenido da el mismo hash venga de hoy, de un commit o de
// una copia de trabajo, y sea cual sea el acceso.
type Servicio struct {
	d Dependencias
}

var (
	_ publicado.ParaEjecucion  = (*Servicio)(nil)
	_ publicado.ParaDefinicion = (*Servicio)(nil)
	_ publicado.ParaSimulacion = (*Servicio)(nil)
)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}

func (s *Servicio) TraerDeHoy(ctx context.Context, fuente string) (publicado.Material, error) {
	f, err := dominio.NuevaFuente(fuente)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	directorio, commit, err := s.d.Repositorios.PonerDeHoy(ctx, f)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	return s.conSuHash(ctx, directorio, func(hash dominio.Hash) (dominio.Material, error) {
		return dominio.MaterialDeUnCommit(directorio, hash, commit)
	})
}

func (s *Servicio) TraerDeUnCommit(ctx context.Context, fuente, commit string) (publicado.Material, error) {
	f, err := dominio.NuevaFuente(fuente)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	c, err := dominio.NuevoCommit(commit)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	directorio, err := s.d.Repositorios.PonerDeUnCommit(ctx, f, c)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	return s.conSuHash(ctx, directorio, func(hash dominio.Hash) (dominio.Material, error) {
		return dominio.MaterialDeUnCommit(directorio, hash, c)
	})
}

func (s *Servicio) TraerCopiaDeTrabajo(ctx context.Context, directorio string) (publicado.Material, error) {
	copia, err := dominio.NuevaCopiaDeTrabajo(directorio)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	puesto, err := s.d.Repositorios.PonerCopiaDeTrabajo(ctx, copia)
	if err != nil {
		return publicado.Material{}, traducir(err)
	}
	return s.conSuHash(ctx, puesto, func(hash dominio.Hash) (dominio.Material, error) {
		return dominio.MaterialDeUnaCopiaDeTrabajo(puesto, hash)
	})
}

func (s *Servicio) Retirar(ctx context.Context, material publicado.Material) error {
	if material.Directorio == "" {
		return traducir(fmt.Errorf("%w: el material no dice dónde está", dominio.ErrInvalido))
	}
	return traducir(s.d.Repositorios.Retirar(ctx, material.Directorio))
}

// conSuHash calcula el hash de lo que se puso delante y compone el material. Si no puede, lo retira: quien lo
// pidió no recibe nada, así que nadie más lo retiraría.
func (s *Servicio) conSuHash(
	ctx context.Context, directorio string, componer func(dominio.Hash) (dominio.Material, error),
) (publicado.Material, error) {
	hash, err := s.d.Hashes.DeUnDirectorio(ctx, directorio)
	var material dominio.Material
	if err == nil {
		material, err = componer(hash)
	}
	if err != nil {
		errRetirar := s.d.Repositorios.Retirar(context.WithoutCancel(ctx), directorio)
		return publicado.Material{}, traducir(errors.Join(err, errRetirar))
	}
	commit, _ := material.Commit()
	return publicado.Material{
		Directorio: material.Directorio(),
		Hash:       material.Hash().String(),
		Commit:     commit.String(),
	}, nil
}

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dominio.ErrInvalido):
		return &errorTraducido{publicado: publicado.ErrInvalido, causa: err}
	case errors.Is(err, dominio.ErrNoExiste):
		return &errorTraducido{publicado: publicado.ErrNoExiste, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
