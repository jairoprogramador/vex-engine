package infraestructura_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	suministropublicado "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// El adaptador de Fuentes se prueba contra un doble de suministro/publicado.ParaEjecucion (DEC-11.3): el
// material real, traído de un repositorio de verdad, se ejercita en la prueba de punta a punta.
type fuentesFalsas struct {
	material  suministropublicado.Material
	err       error
	retirados []suministropublicado.Material
}

func (f *fuentesFalsas) TraerDeHoy(_ context.Context, _ string) (suministropublicado.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) TraerDeUnCommit(_ context.Context, _, _ string) (suministropublicado.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) TraerCopiaDeTrabajo(_ context.Context, _ string) (suministropublicado.Material, error) {
	return f.material, f.err
}

func (f *fuentesFalsas) Retirar(_ context.Context, m suministropublicado.Material) error {
	f.retirados = append(f.retirados, m)
	return nil
}

func TestAdaptadorDeFuentes_TraduceElMaterial(t *testing.T) {
	falsas := &fuentesFalsas{material: suministropublicado.Material{Directorio: "/tmp/x", Hash: "contenido-v1:abc", Commit: "c1"}}
	adaptador := infraestructura.NuevasFuentes(falsas)

	m, err := adaptador.TraerDeHoy(context.Background(), "fuente")
	require.NoError(t, err)
	require.Equal(t, "/tmp/x", m.Directorio)
	require.Equal(t, "contenido-v1:abc", m.Hash.String())
	require.Equal(t, "c1", m.Commit)
}

func TestAdaptadorDeFuentes_UnaCopiaDeTrabajoNoTraeCommit(t *testing.T) {
	falsas := &fuentesFalsas{material: suministropublicado.Material{Directorio: "/tmp/x", Hash: "contenido-v1:abc"}}
	adaptador := infraestructura.NuevasFuentes(falsas)

	m, err := adaptador.TraerCopiaDeTrabajo(context.Background(), "/tmp/x")
	require.NoError(t, err)
	require.Empty(t, m.Commit)
}

func TestAdaptadorDeFuentes_RetirarPasaElDirectorioYElHash(t *testing.T) {
	falsas := &fuentesFalsas{}
	adaptador := infraestructura.NuevasFuentes(falsas)
	hash, err := dominio.NuevoHashDeCodigo("contenido-v1:abc")
	require.NoError(t, err)

	require.NoError(t, adaptador.Retirar(context.Background(), dominio.Material{Directorio: "/tmp/x", Hash: hash, Commit: "c1"}))
	require.Len(t, falsas.retirados, 1)
	require.Equal(t, "/tmp/x", falsas.retirados[0].Directorio)
	require.Equal(t, "contenido-v1:abc", falsas.retirados[0].Hash)
}

func TestAdaptadorDeFuentes_PropagaElErrorDeSuministro(t *testing.T) {
	falsas := &fuentesFalsas{err: suministropublicado.ErrNoExiste}
	adaptador := infraestructura.NuevasFuentes(falsas)

	_, err := adaptador.TraerDeHoy(context.Background(), "fuente")
	require.ErrorIs(t, err, suministropublicado.ErrNoExiste)
}
