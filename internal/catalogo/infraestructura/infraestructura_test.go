package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/infraestructura"
	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

type definicionFalsa struct {
	pipeline definicionpublicado.Pipeline
	err      error
	llamada  string
}

func (d *definicionFalsa) DeHoy(_ context.Context, fuente string) (definicionpublicado.Pipeline, error) {
	d.llamada = "hoy:" + fuente
	return d.pipeline, d.err
}

func (d *definicionFalsa) DeUnCommit(_ context.Context, fuente, commit string) (definicionpublicado.Pipeline, error) {
	d.llamada = "commit:" + fuente + "@" + commit
	return d.pipeline, d.err
}

func (d *definicionFalsa) VariablesEstandar() []definicionpublicado.VariableEstandar { return nil }

func fuente(t *testing.T) dominio.Fuente {
	t.Helper()
	f, err := dominio.NuevaFuente("f")
	require.NoError(t, err)
	return f
}

func TestPipelines_TraduceAmbientesYPasosEnSuOrden(t *testing.T) {
	d := &definicionFalsa{pipeline: definicionpublicado.Pipeline{
		Ambientes: []definicionpublicado.Ambiente{{Nombre: "sandbox", Descripcion: "x", Valor: "sand"}},
		Pasos:     []definicionpublicado.Paso{{Nombre: "uno", Orden: 1, Compartido: true}, {Nombre: "dos", Orden: 2}},
	}}

	p, err := infraestructura.NuevosPipelines(d).Pipeline(context.Background(), fuente(t), "")

	require.NoError(t, err)
	require.Equal(t, []dominio.Ambiente{{Nombre: "sandbox", Descripcion: "x", Valor: "sand"}}, p.Ambientes)
	require.Equal(t, []dominio.Paso{{Nombre: "uno", Orden: 1, Compartido: true}, {Nombre: "dos", Orden: 2}}, p.Pasos)
}

func TestPipelines_SinCommitPideElDeHoyYConCommitElDeEseCommit(t *testing.T) {
	d := &definicionFalsa{}
	pipelines := infraestructura.NuevosPipelines(d)

	_, err := pipelines.Pipeline(context.Background(), fuente(t), "")
	require.NoError(t, err)
	require.Equal(t, "hoy:f", d.llamada)

	_, err = pipelines.Pipeline(context.Background(), fuente(t), "abc")
	require.NoError(t, err)
	require.Equal(t, "commit:f@abc", d.llamada)
}

func TestPipelines_TraduceLosErroresDeDefinicionAlDominio(t *testing.T) {
	casos := map[string]struct{ origen, esperado error }{
		"no existe":     {definicionpublicado.ErrNoExiste, dominio.ErrInvalido},
		"invalido":      {definicionpublicado.ErrInvalido, dominio.ErrInvalido},
		"no comprobado": {definicionpublicado.NuevosFallosDeComprobacion(nil, "m"), dominio.ErrRechazado},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := infraestructura.NuevosPipelines(&definicionFalsa{err: c.origen}).
				Pipeline(context.Background(), fuente(t), "")

			require.ErrorIs(t, err, c.esperado)
			require.ErrorIs(t, err, c.origen, "sigue siendo el error de arriba")
		})
	}
}

func TestPipelines_UnErrorQueNoReconoceNoSeDisfraza(t *testing.T) {
	_, err := infraestructura.NuevosPipelines(&definicionFalsa{err: errors.New("disco roto")}).
		Pipeline(context.Background(), fuente(t), "")

	require.Error(t, err)
	require.NotErrorIs(t, err, dominio.ErrInvalido)
	require.NotErrorIs(t, err, dominio.ErrRechazado)
}

type historialFalso struct {
	reserva historialpublicado.Reserva
	hay     bool
	err     error
}

func (h historialFalso) UltimaReserva(context.Context, string) (historialpublicado.Reserva, bool, error) {
	return h.reserva, h.hay, h.err
}

func TestReservas_ElEstadoEsLaUltimaReserva(t *testing.T) {
	casos := map[string]struct {
		h        historialFalso
		esperado bool
	}{
		"nunca reservado": {historialFalso{}, false},
		"reservado":       {historialFalso{reserva: historialpublicado.Reserva{Reservado: true}, hay: true}, true},
		"liberado":        {historialFalso{reserva: historialpublicado.Reserva{Reservado: false}, hay: true}, false},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			reservado, err := infraestructura.NuevasReservas(c.h).Reservado(context.Background(), "prod")

			require.NoError(t, err)
			require.Equal(t, c.esperado, reservado)
		})
	}
}

func TestReservas_UnaFallaDelHistorialSePropaga(t *testing.T) {
	origen := errors.New("falla simulada")

	_, err := infraestructura.NuevasReservas(historialFalso{err: origen}).Reservado(context.Background(), "prod")

	require.ErrorIs(t, err, origen)
}
