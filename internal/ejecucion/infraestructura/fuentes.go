package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	suministropublicado "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// Fuentes implementa dominio.Fuentes sobre lo que publica Suministro de Fuentes: el material del código del
// proyecto. El del pipeline lo trae Definición por su cuenta (pipelines.go); son las dos fuentes de distinto
// dueño de docs/modelo/dominio.md, «Suministro de Fuentes».
type Fuentes struct {
	fuentes suministropublicado.ParaEjecucion
}

var _ dominio.Fuentes = (*Fuentes)(nil)

func NuevasFuentes(f suministropublicado.ParaEjecucion) *Fuentes {
	return &Fuentes{fuentes: f}
}

func (f *Fuentes) TraerDeHoy(ctx context.Context, fuente string) (dominio.Material, error) {
	m, err := f.fuentes.TraerDeHoy(ctx, fuente)
	if err != nil {
		return dominio.Material{}, fmt.Errorf("ejecución: el material de %q: %w", fuente, err)
	}
	return materialDeDominio(m)
}

func (f *Fuentes) TraerDeUnCommit(ctx context.Context, fuente, commit string) (dominio.Material, error) {
	m, err := f.fuentes.TraerDeUnCommit(ctx, fuente, commit)
	if err != nil {
		return dominio.Material{}, fmt.Errorf("ejecución: el material de %s@%s: %w", fuente, commit, err)
	}
	return materialDeDominio(m)
}

func (f *Fuentes) TraerCopiaDeTrabajo(ctx context.Context, directorio string) (dominio.Material, error) {
	m, err := f.fuentes.TraerCopiaDeTrabajo(ctx, directorio)
	if err != nil {
		return dominio.Material{}, fmt.Errorf("ejecución: la copia de trabajo de %q: %w", directorio, err)
	}
	return materialDeDominio(m)
}

func (f *Fuentes) Retirar(ctx context.Context, material dominio.Material) error {
	if err := f.fuentes.Retirar(ctx, suministropublicado.Material{
		Directorio: material.Directorio, Hash: material.Hash.String(), Commit: material.Commit,
	}); err != nil {
		return fmt.Errorf("ejecución: retirar el material de %q: %w", material.Directorio, err)
	}
	return nil
}

func materialDeDominio(m suministropublicado.Material) (dominio.Material, error) {
	hash, err := dominio.NuevoHashDeCodigo(m.Hash)
	if err != nil {
		return dominio.Material{}, err
	}
	return dominio.Material{Directorio: m.Directorio, Hash: hash, Commit: m.Commit}, nil
}
