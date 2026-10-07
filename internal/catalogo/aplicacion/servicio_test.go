package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// Los dobles son los puertos que el propio dominio de Catálogo declara: se prueba contra la frontera que
// Catálogo ya decidió, nunca contra Definición ni contra el Historial.

type pipelinesFalsos struct {
	pipeline dominio.Pipeline
	err      error
	fuente   dominio.Fuente
	commit   dominio.Commit
}

func (p *pipelinesFalsos) Pipeline(_ context.Context, f dominio.Fuente, c dominio.Commit) (dominio.Pipeline, error) {
	p.fuente, p.commit = f, c
	return p.pipeline, p.err
}

type reservasFalsas struct {
	reservados map[string]bool
	err        error
}

func (r *reservasFalsas) Reservado(_ context.Context, ambiente string) (bool, error) {
	return r.reservados[ambiente], r.err
}

var pipelineDeEjemplo = dominio.Pipeline{
	Ambientes: []dominio.Ambiente{
		{Nombre: "sandbox", Descripcion: "pruebas", Valor: "sand"},
		{Nombre: "produccion", Valor: "prod"},
	},
	Pasos: []dominio.Paso{
		{Nombre: "pruebas", Orden: 1},
		{Nombre: "infraestructura", Orden: 2, Compartido: true},
	},
}

func nuevoServicio(p *pipelinesFalsos, r *reservasFalsas) *aplicacion.Servicio {
	return aplicacion.NuevoServicio(aplicacion.Dependencias{Pipelines: p, Reservas: r})
}

func peticion() publicado.PeticionDeCatalogo {
	return publicado.PeticionDeCatalogo{FuenteDelPipeline: "git@ejemplo:pipeline.git"}
}

func TestAmbientes_EnElOrdenDelPipelineConSuReserva(t *testing.T) {
	p := &pipelinesFalsos{pipeline: pipelineDeEjemplo}
	r := &reservasFalsas{reservados: map[string]bool{"prod": true}}

	ambientes, err := nuevoServicio(p, r).Ambientes(context.Background(), peticion())

	require.NoError(t, err)
	require.Equal(t, []publicado.Ambiente{
		{Nombre: "sandbox", Descripcion: "pruebas", Valor: "sand", Reservado: false},
		{Nombre: "produccion", Valor: "prod", Reservado: true},
	}, ambientes)
}

func TestAmbientes_UnAmbienteQueNuncaSeReservoNoEstaReservado(t *testing.T) {
	p := &pipelinesFalsos{pipeline: pipelineDeEjemplo}

	ambientes, err := nuevoServicio(p, &reservasFalsas{}).Ambientes(context.Background(), peticion())

	require.NoError(t, err)
	for _, a := range ambientes {
		require.False(t, a.Reservado, a.Valor)
	}
}

func TestAmbientes_SinCommitPideElPipelineDeHoyYConCommitElDeEseCommit(t *testing.T) {
	p := &pipelinesFalsos{pipeline: pipelineDeEjemplo}
	s := nuevoServicio(p, &reservasFalsas{})

	_, err := s.Ambientes(context.Background(), peticion())
	require.NoError(t, err)
	require.True(t, p.commit.DeHoy())

	con := peticion()
	con.Commit = "abc123"
	_, err = s.Ambientes(context.Background(), con)
	require.NoError(t, err)
	require.Equal(t, "abc123", p.commit.String())
	require.Equal(t, "git@ejemplo:pipeline.git", p.fuente.String())
}

func TestAmbientes_SinFuenteEsInvalidoYDiceElCampo(t *testing.T) {
	_, err := nuevoServicio(&pipelinesFalsos{}, &reservasFalsas{}).Ambientes(
		context.Background(), publicado.PeticionDeCatalogo{})

	require.ErrorIs(t, err, publicado.ErrInvalido)
	var parametro interface{ ParametroInvalido() (string, string) }
	require.ErrorAs(t, err, &parametro)
	campo, _ := parametro.ParametroInvalido()
	require.Equal(t, "FuenteDelPipeline", campo)
}

func TestAmbientes_LosErroresDelDominioSeTraducenAlLenguajePublicado(t *testing.T) {
	casos := map[string]struct {
		err      error
		esperado error
	}{
		"inválido":  {dominio.ErrInvalido, publicado.ErrInvalido},
		"rechazado": {dominio.ErrRechazado, publicado.ErrRechazado},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			p := &pipelinesFalsos{err: c.err}

			_, err := nuevoServicio(p, &reservasFalsas{}).Ambientes(context.Background(), peticion())

			require.ErrorIs(t, err, c.esperado)
		})
	}
}

func TestAmbientes_UnaFallaDeLasReservasSePropaga(t *testing.T) {
	p := &pipelinesFalsos{pipeline: pipelineDeEjemplo}
	r := &reservasFalsas{err: errors.New("falla simulada")}

	_, err := nuevoServicio(p, r).Ambientes(context.Background(), peticion())

	require.Error(t, err)
	require.NotErrorIs(t, err, publicado.ErrInvalido)
}

func TestPasos_EnElOrdenDelPipelineConSuAmbito(t *testing.T) {
	p := &pipelinesFalsos{pipeline: pipelineDeEjemplo}

	pasos, err := nuevoServicio(p, &reservasFalsas{}).Pasos(context.Background(), peticion())

	require.NoError(t, err)
	require.Equal(t, []publicado.Paso{
		{Nombre: "pruebas", Orden: 1},
		{Nombre: "infraestructura", Orden: 2, Compartido: true},
	}, pasos)
}

func TestPasos_SinPasosEsUnaListaVacia(t *testing.T) {
	pasos, err := nuevoServicio(&pipelinesFalsos{}, &reservasFalsas{}).Pasos(context.Background(), peticion())

	require.NoError(t, err)
	require.NotNil(t, pasos)
	require.Empty(t, pasos)
}

func TestPasos_SinFuenteEsInvalido(t *testing.T) {
	_, err := nuevoServicio(&pipelinesFalsos{}, &reservasFalsas{}).Pasos(
		context.Background(), publicado.PeticionDeCatalogo{})

	require.ErrorIs(t, err, publicado.ErrInvalido)
}
