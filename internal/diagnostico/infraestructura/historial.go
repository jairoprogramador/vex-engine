package infraestructura

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Historial implementa dominio.Historial sobre lo que publica el contexto Historial. El dominio de
// Diagnóstico nunca ve un Intento, un Despliegue ni un Contenido de historial/publicado: esa traducción
// vive aquí, incluida la factoría (ejes.go) que construye ejes de un paso y referencia a partir de
// registros (docs/modelo/contextos/diagnostico.md, «Factoría»).
type Historial struct {
	registros historialpublicado.ParaDiagnostico
}

var _ dominio.Historial = (*Historial)(nil)

func NuevoHistorial(registros historialpublicado.ParaDiagnostico) *Historial {
	return &Historial{registros: registros}
}

func (h *Historial) Intento(ctx context.Context, id dominio.IdIntento) (dominio.IntentoDeDiagnostico, error) {
	i, err := h.registros.Intento(ctx, id.String())
	if err != nil {
		return dominio.IntentoDeDiagnostico{}, fmt.Errorf("diagnóstico: el intento %s: %w", id, err)
	}
	return intentoDeDominio(i)
}

func (h *Historial) UltimoIntentoDeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) (dominio.IdIntento, bool, error) {
	intentos, err := h.registros.IntentosDeUnAmbiente(ctx, ambiente.String())
	if err != nil {
		return dominio.IdIntento{}, false, fmt.Errorf("diagnóstico: los intentos de %q: %w", ambiente, err)
	}
	if len(intentos) == 0 {
		return dominio.IdIntento{}, false, nil
	}
	id, err := dominio.NuevoIdIntento(intentos[len(intentos)-1].Id)
	if err != nil {
		return dominio.IdIntento{}, false, err
	}
	return id, true, nil
}

func (h *Historial) DespliegueDeUnLanzamiento(
	ctx context.Context, lanzamiento dominio.IdLanzamiento,
) (dominio.IdDespliegue, error) {
	l, err := h.registros.Lanzamiento(ctx, lanzamiento.String())
	if err != nil {
		return dominio.IdDespliegue{}, fmt.Errorf("diagnóstico: el lanzamiento %s: %w", lanzamiento, err)
	}
	return dominio.NuevoIdDespliegue(l.Despliegue)
}

func (h *Historial) Despliegue(ctx context.Context, id dominio.IdDespliegue) (dominio.DespliegueDeDiagnostico, error) {
	d, err := h.registros.Despliegue(ctx, id.String())
	if err != nil {
		return dominio.DespliegueDeDiagnostico{}, fmt.Errorf("diagnóstico: el despliegue %s: %w", id, err)
	}
	return despliegueDeDominio(d)
}

// UltimoDespliegueAnteriorA es la referencia por defecto del mismo ambiente (DEC-06.6): el último
// despliegue de ese ambiente anterior al instante dado.
func (h *Historial) UltimoDespliegueAnteriorA(
	ctx context.Context, ambiente dominio.Ambiente, antesDe time.Time,
) (dominio.DespliegueDeDiagnostico, bool, error) {
	despliegues, err := h.registros.DesplieguesDeUnAmbiente(ctx, ambiente.String())
	if err != nil {
		return dominio.DespliegueDeDiagnostico{}, false, fmt.Errorf("diagnóstico: los despliegues de %q: %w", ambiente, err)
	}
	for k := len(despliegues) - 1; k >= 0; k-- {
		if despliegues[k].Instante.Before(antesDe) {
			d, err := despliegueDeDominio(despliegues[k])
			return d, true, err
		}
	}
	return dominio.DespliegueDeDiagnostico{}, false, nil
}

func (h *Historial) CantidadDeIntentos(
	ctx context.Context, despliegue dominio.IdDespliegue, intento dominio.IdIntento,
) (int, error) {
	cantidad, err := h.registros.CantidadDeIntentos(ctx, despliegue.String(), intento.String())
	if err != nil {
		return 0, fmt.Errorf("diagnóstico: la cantidad de intentos desde %s hasta %s: %w", despliegue, intento, err)
	}
	return cantidad, nil
}

func intentoDeDominio(i historialpublicado.Intento) (dominio.IntentoDeDiagnostico, error) {
	id, err := dominio.NuevoIdIntento(i.Id)
	if err != nil {
		return dominio.IntentoDeDiagnostico{}, err
	}
	ambiente, err := dominio.NuevaAmbiente(i.Apertura.Ambiente)
	if err != nil {
		return dominio.IntentoDeDiagnostico{}, err
	}
	return dominio.IntentoDeDiagnostico{
		Id: id, Ambiente: ambiente, Instante: i.Instante, Estado: dominio.EstadoDeIntento(i.Estado),
	}, nil
}

func despliegueDeDominio(d historialpublicado.Despliegue) (dominio.DespliegueDeDiagnostico, error) {
	id, err := dominio.NuevoIdDespliegue(d.Id)
	if err != nil {
		return dominio.DespliegueDeDiagnostico{}, err
	}
	ambiente, err := dominio.NuevaAmbiente(d.Ambiente)
	if err != nil {
		return dominio.DespliegueDeDiagnostico{}, err
	}
	intento, err := dominio.NuevoIdIntento(d.Intento)
	if err != nil {
		return dominio.DespliegueDeDiagnostico{}, err
	}
	return dominio.DespliegueDeDiagnostico{Id: id, Ambiente: ambiente, Intento: intento, Instante: d.Instante}, nil
}
