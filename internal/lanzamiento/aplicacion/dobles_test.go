package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

// El doble de este fichero es el puerto que el propio dominio de Lanzamiento declara (dominio.Historial) —
// nunca el servicio real de Historial: probar contra la frontera que Lanzamiento ya decidió es lo que
// mantiene un contexto aislado del otro (DEC-11.3).

func nuevoServicio(h *historialFalso) *aplicacion.Servicio {
	return aplicacion.NuevoServicio(aplicacion.Dependencias{Historial: h})
}

func hash(t *testing.T, valor string) dominio.HashDelCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDelCodigo(valor)
	require.NoError(t, err)
	return h
}

func version(t *testing.T, numero int) dominio.Version {
	t.Helper()
	v, err := dominio.NuevaVersion(numero)
	require.NoError(t, err)
	return v
}

func idDespliegue(t *testing.T, valor string) dominio.IdDespliegue {
	t.Helper()
	d, err := dominio.NuevoIdDespliegue(valor)
	require.NoError(t, err)
	return d
}

func mustAmbiente(t *testing.T, valor string) dominio.Ambiente {
	t.Helper()
	a, err := dominio.NuevaAmbiente(valor)
	require.NoError(t, err)
	return a
}

// historialFalso guarda en memoria lo que Lanzamiento le pide y le registra, para que las pruebas
// comprueben la decisión sin depender del Historial real.
type historialFalso struct {
	reservado map[string]bool
	ultimo    map[string]dominio.IdDespliegue
	hashes    map[string]dominio.HashDelCodigo
	conocidas []dominio.VersionConocida

	lanzamientos []registroDeLanzamiento
	reservas     []registroDeReserva

	fallarRegistrarLanzamiento bool
}

type registroDeLanzamiento struct {
	ambiente    dominio.Ambiente
	lanzamiento dominio.Lanzamiento
}

type registroDeReserva struct {
	ambiente  dominio.Ambiente
	reservado bool
}

func nuevoHistorialFalso() *historialFalso {
	return &historialFalso{
		reservado: map[string]bool{},
		ultimo:    map[string]dominio.IdDespliegue{},
		hashes:    map[string]dominio.HashDelCodigo{},
	}
}

func (h *historialFalso) Reservado(_ context.Context, ambiente dominio.Ambiente) (bool, error) {
	return h.reservado[ambiente.String()], nil
}

func (h *historialFalso) UltimoLanzamientoDeUnAmbiente(
	_ context.Context, ambiente dominio.Ambiente,
) (dominio.IdDespliegue, bool, error) {
	d, ok := h.ultimo[ambiente.String()]
	return d, ok, nil
}

func (h *historialFalso) HashDelCodigoDeUnDespliegue(
	_ context.Context, despliegue dominio.IdDespliegue,
) (dominio.HashDelCodigo, error) {
	hash, ok := h.hashes[despliegue.String()]
	if !ok {
		return dominio.HashDelCodigo{}, errors.New("hash desconocido en la prueba")
	}
	return hash, nil
}

func (h *historialFalso) VersionesConocidas(_ context.Context) ([]dominio.VersionConocida, error) {
	return h.conocidas, nil
}

func (h *historialFalso) RegistrarLanzamiento(
	_ context.Context, ambiente dominio.Ambiente, lanzamiento dominio.Lanzamiento,
) (dominio.LanzamientoRegistrado, error) {
	if h.fallarRegistrarLanzamiento {
		return dominio.LanzamientoRegistrado{}, errors.New("falla simulada de RegistrarLanzamiento")
	}
	h.lanzamientos = append(h.lanzamientos, registroDeLanzamiento{ambiente: ambiente, lanzamiento: lanzamiento})
	return dominio.LanzamientoRegistrado{
		Id: "lz-1", Ambiente: ambiente, Despliegue: lanzamiento.Despliegue(),
		Version: lanzamiento.Version(), Nombre: lanzamiento.Nombre(),
	}, nil
}

func (h *historialFalso) RegistrarReserva(_ context.Context, ambiente dominio.Ambiente, reservado bool) error {
	h.reservas = append(h.reservas, registroDeReserva{ambiente: ambiente, reservado: reservado})
	return nil
}

var _ dominio.Historial = (*historialFalso)(nil)
