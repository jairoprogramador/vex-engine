package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/infraestructura"
)

// historialFalso es el doble del puerto que declara el dominio de Lanzamiento — la misma frontera que usan
// las pruebas de aplicación — para comprobar que la escucha traduce el evento y no le importa el error.
type historialFalso struct {
	llamado    bool
	ambiente   string
	despliegue string
	fallar     error
}

func (h *historialFalso) Reservado(context.Context, dominio.Ambiente) (bool, error) {
	return false, nil
}

func (h *historialFalso) UltimoLanzamientoDeUnAmbiente(
	context.Context, dominio.Ambiente,
) (dominio.IdDespliegue, bool, error) {
	return dominio.IdDespliegue{}, false, nil
}

func (h *historialFalso) HashDelCodigoDeUnDespliegue(
	_ context.Context, despliegue dominio.IdDespliegue,
) (dominio.HashDelCodigo, error) {
	h.llamado, h.despliegue = true, despliegue.String()
	if h.fallar != nil {
		return dominio.HashDelCodigo{}, h.fallar
	}
	hash, err := dominio.NuevoHashDelCodigo("h1")
	return hash, err
}

func (h *historialFalso) VersionesConocidas(context.Context) ([]dominio.VersionConocida, error) {
	return nil, nil
}

func (h *historialFalso) LanzamientosDeUnAmbiente(
	context.Context, dominio.Ambiente,
) ([]dominio.LanzamientoRegistrado, error) {
	return nil, nil
}

func (h *historialFalso) RegistrarLanzamiento(
	_ context.Context, ambiente dominio.Ambiente, lanzamiento dominio.Lanzamiento,
) (dominio.LanzamientoRegistrado, error) {
	h.ambiente = ambiente.String()
	return dominio.LanzamientoRegistrado{Ambiente: ambiente, Despliegue: lanzamiento.Despliegue()}, nil
}

func (h *historialFalso) RegistrarReserva(context.Context, dominio.Ambiente, bool) error { return nil }

var _ dominio.Historial = (*historialFalso)(nil)

func TestEscucha_TraduceElEventoALosCamposDelDespliegue(t *testing.T) {
	h := &historialFalso{}
	e := infraestructura.NuevaEscucha(aplicacion.NuevoServicio(aplicacion.Dependencias{Historial: h}))

	e.DespliegueRegistrado(context.Background(), historialpublicado.DespliegueRegistrado{
		Despliegue: historialpublicado.Despliegue{Id: "d1", Ambiente: "staging"},
	})

	require.True(t, h.llamado)
	require.Equal(t, "d1", h.despliegue)
	require.Equal(t, "staging", h.ambiente)
}

func TestEscucha_UnErrorDeLaAplicacionNoSePropaga(t *testing.T) {
	h := &historialFalso{fallar: errors.New("falla simulada")}
	e := infraestructura.NuevaEscucha(aplicacion.NuevoServicio(aplicacion.Dependencias{Historial: h}))

	require.NotPanics(t, func() {
		e.DespliegueRegistrado(context.Background(), historialpublicado.DespliegueRegistrado{
			Despliegue: historialpublicado.Despliegue{Id: "d1", Ambiente: "staging"},
		})
	})
}
