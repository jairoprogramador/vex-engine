package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/infraestructura"
)

// registrosFalsos es el doble de historial/publicado.ParaLanzamiento: la frontera que este adaptador
// traduce, para comprobar que envuelve sus errores sin depender del Historial real.
type registrosFalsos struct {
	fallar      error
	lanzamiento historialpublicado.Lanzamiento
}

func (r *registrosFalsos) RegistrarLanzamiento(
	context.Context, string, string, historialpublicado.Contenido,
) (historialpublicado.Lanzamiento, error) {
	return historialpublicado.Lanzamiento{}, r.fallar
}

func (r *registrosFalsos) RegistrarReserva(context.Context, string, bool) error { return r.fallar }

func (r *registrosFalsos) UltimoDespliegue(
	context.Context, string,
) (historialpublicado.Despliegue, bool, error) {
	return historialpublicado.Despliegue{}, false, r.fallar
}

func (r *registrosFalsos) UltimaReserva(context.Context, string) (historialpublicado.Reserva, bool, error) {
	return historialpublicado.Reserva{}, false, r.fallar
}

func (r *registrosFalsos) UltimoLanzamiento(
	context.Context, string,
) (historialpublicado.Lanzamiento, bool, error) {
	return historialpublicado.Lanzamiento{}, false, r.fallar
}

func (r *registrosFalsos) HashDelCodigoDeUnDespliegue(context.Context, string) (string, error) {
	return "", r.fallar
}

func (r *registrosFalsos) TodosLosLanzamientos(context.Context) ([]historialpublicado.Lanzamiento, error) {
	if r.fallar != nil {
		return nil, r.fallar
	}
	return []historialpublicado.Lanzamiento{r.lanzamiento}, nil
}

var _ historialpublicado.ParaLanzamiento = (*registrosFalsos)(nil)

func TestAdaptadorDeHistorial_EnvuelveLosErroresDelHistorial(t *testing.T) {
	falla := errors.New("falla simulada del historial")
	adaptador := infraestructura.NuevoHistorial(&registrosFalsos{fallar: falla})
	ctx := context.Background()
	ambiente := mustAmbiente(t, "staging")

	_, err := adaptador.Reservado(ctx, ambiente)
	require.ErrorIs(t, err, falla)

	_, _, err = adaptador.UltimoLanzamientoDeUnAmbiente(ctx, ambiente)
	require.ErrorIs(t, err, falla)

	_, err = adaptador.HashDelCodigoDeUnDespliegue(ctx, mustIdDespliegue(t, "d1"))
	require.ErrorIs(t, err, falla)

	_, err = adaptador.VersionesConocidas(ctx)
	require.ErrorIs(t, err, falla)

	lanzamiento := dominio.NuevoLanzamiento(mustIdDespliegue(t, "d1"), mustHash(t, "h1"), mustVersion(t, 1), "")
	_, err = adaptador.RegistrarLanzamiento(ctx, ambiente, lanzamiento)
	require.ErrorIs(t, err, falla)

	err = adaptador.RegistrarReserva(ctx, ambiente, true)
	require.ErrorIs(t, err, falla)
}

func TestAdaptadorDeHistorial_VersionesConocidasRechazaUnContenidoDeOtroContexto(t *testing.T) {
	adaptador := infraestructura.NuevoHistorial(&registrosFalsos{
		lanzamiento: historialpublicado.Lanzamiento{
			Contenido: historialpublicado.Contenido{Contexto: "otro/contexto-v1", Datos: []byte(`{}`)},
		},
	})

	_, err := adaptador.VersionesConocidas(context.Background())

	require.Error(t, err)
}
