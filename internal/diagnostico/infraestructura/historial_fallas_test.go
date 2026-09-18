package infraestructura_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	diagnosticodominio "github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	diagnosticoinfraestructura "github.com/jairoprogramador/vex-engine/internal/diagnostico/infraestructura"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

var errFalla = errors.New("falla simulada del historial")

// registrosFalsos hace fallar todo lo que Diagnóstico le pide, para comprobar solo el envoltorio de
// errores del ACL (mismo patrón que lanzamiento/infraestructura/historial_fallas_test.go).
type registrosFalsos struct{}

func (registrosFalsos) Intento(context.Context, string) (historialpublicado.Intento, error) {
	return historialpublicado.Intento{}, errFalla
}
func (registrosFalsos) IntentosDeUnAmbiente(context.Context, string) ([]historialpublicado.Intento, error) {
	return nil, errFalla
}
func (registrosFalsos) DesplieguesDeUnAmbiente(context.Context, string) ([]historialpublicado.Despliegue, error) {
	return nil, errFalla
}
func (registrosFalsos) UltimoDespliegueConHashDelCodigo(
	context.Context, string, string,
) (historialpublicado.Despliegue, bool, error) {
	return historialpublicado.Despliegue{}, false, errFalla
}
func (registrosFalsos) CantidadDeIntentos(context.Context, string, string) (int, error) {
	return 0, errFalla
}
func (registrosFalsos) VariablesDeUnPaso(context.Context, string, string) ([]historialpublicado.Variable, error) {
	return nil, errFalla
}
func (registrosFalsos) Lanzamiento(context.Context, string) (historialpublicado.Lanzamiento, error) {
	return historialpublicado.Lanzamiento{}, errFalla
}
func (registrosFalsos) Despliegue(context.Context, string) (historialpublicado.Despliegue, error) {
	return historialpublicado.Despliegue{}, errFalla
}

var _ historialpublicado.ParaDiagnostico = registrosFalsos{}

func TestHistorial_EnvuelveLosErroresDelHistorial(t *testing.T) {
	h := diagnosticoinfraestructura.NuevoHistorial(registrosFalsos{})
	ctx := context.Background()

	idIntento, err := diagnosticodominio.NuevoIdIntento("i1")
	require.NoError(t, err)
	idDespliegue, err := diagnosticodominio.NuevoIdDespliegue("d1")
	require.NoError(t, err)
	idLanzamiento, err := diagnosticodominio.NuevoIdLanzamiento("l1")
	require.NoError(t, err)
	ambiente, err := diagnosticodominio.NuevaAmbiente("prod")
	require.NoError(t, err)
	hash, err := diagnosticodominio.NuevoHashDelCodigo("h1")
	require.NoError(t, err)

	_, err = h.Intento(ctx, idIntento)
	require.ErrorIs(t, err, errFalla)

	_, _, err = h.UltimoIntentoDeUnAmbiente(ctx, ambiente)
	require.ErrorIs(t, err, errFalla)

	_, err = h.DespliegueDeUnLanzamiento(ctx, idLanzamiento)
	require.ErrorIs(t, err, errFalla)

	_, err = h.Despliegue(ctx, idDespliegue)
	require.ErrorIs(t, err, errFalla)

	_, _, err = h.UltimoDespliegueAnteriorA(ctx, ambiente, time.Now())
	require.ErrorIs(t, err, errFalla)

	_, _, err = h.UltimoDespliegueConHashDelCodigo(ctx, ambiente, hash)
	require.ErrorIs(t, err, errFalla)

	_, _, err = h.EjesDelIntento(ctx, idIntento)
	require.ErrorIs(t, err, errFalla)

	_, err = h.CantidadDeIntentos(ctx, idDespliegue, idIntento)
	require.ErrorIs(t, err, errFalla)
}
