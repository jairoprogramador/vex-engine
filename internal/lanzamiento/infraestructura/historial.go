package infraestructura

import (
	"context"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

// Historial implementa dominio.Historial sobre lo que publica el contexto Historial. El dominio de
// Lanzamiento nunca ve un intento ni un registro: esa traducción vive aquí.
type Historial struct {
	registros historialpublicado.ParaLanzamiento
}

var _ dominio.Historial = (*Historial)(nil)

func NuevoHistorial(registros historialpublicado.ParaLanzamiento) *Historial {
	return &Historial{registros: registros}
}

func (h *Historial) Reservado(ctx context.Context, ambiente dominio.Ambiente) (bool, error) {
	reserva, hay, err := h.registros.UltimaReserva(ctx, ambiente.String())
	if err != nil {
		return false, fmt.Errorf("lanzamiento: la última reserva de %q: %w", ambiente, err)
	}
	return hay && reserva.Reservado, nil
}

func (h *Historial) UltimoLanzamientoDeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) (dominio.IdDespliegue, bool, error) {
	ultimo, hay, err := h.registros.UltimoLanzamiento(ctx, ambiente.String())
	if err != nil {
		return dominio.IdDespliegue{}, false, fmt.Errorf("lanzamiento: el último lanzamiento de %q: %w", ambiente, err)
	}
	if !hay {
		return dominio.IdDespliegue{}, false, nil
	}
	despliegue, err := dominio.NuevoIdDespliegue(ultimo.Despliegue)
	if err != nil {
		return dominio.IdDespliegue{}, false, err
	}
	return despliegue, true, nil
}

func (h *Historial) HashDelCodigoDeUnDespliegue(
	ctx context.Context, despliegue dominio.IdDespliegue,
) (dominio.HashDelCodigo, error) {
	hash, err := h.registros.HashDelCodigoDeUnDespliegue(ctx, despliegue.String())
	if err != nil {
		return dominio.HashDelCodigo{}, fmt.Errorf("lanzamiento: el hash del código de %s: %w", despliegue, err)
	}
	return dominio.NuevoHashDelCodigo(hash)
}

func (h *Historial) VersionesConocidas(ctx context.Context) ([]dominio.VersionConocida, error) {
	todos, err := h.registros.TodosLosLanzamientos(ctx)
	if err != nil {
		return nil, fmt.Errorf("lanzamiento: todos los lanzamientos: %w", err)
	}
	conocidas := make([]dominio.VersionConocida, 0, len(todos))
	for _, l := range todos {
		conocida, err := decodificarVersionConocida(l.Contenido)
		if err != nil {
			return nil, err
		}
		conocidas = append(conocidas, conocida)
	}
	return conocidas, nil
}

func (h *Historial) LanzamientosDeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) ([]dominio.LanzamientoRegistrado, error) {
	todos, err := h.registros.TodosLosLanzamientos(ctx)
	if err != nil {
		return nil, fmt.Errorf("lanzamiento: los lanzamientos de %q: %w", ambiente, err)
	}
	registrados := make([]dominio.LanzamientoRegistrado, 0, len(todos))
	for _, l := range todos {
		if l.Ambiente != ambiente.String() {
			continue
		}
		despliegue, err := dominio.NuevoIdDespliegue(l.Despliegue)
		if err != nil {
			return nil, err
		}
		version, nombre, err := decodificarLanzamiento(l.Contenido)
		if err != nil {
			return nil, err
		}
		registrados = append(registrados, dominio.LanzamientoRegistrado{
			Id: l.Id, Ambiente: ambiente, Despliegue: despliegue, Version: version, Nombre: nombre, Instante: l.Instante,
		})
	}
	return registrados, nil
}

func (h *Historial) RegistrarLanzamiento(
	ctx context.Context, ambiente dominio.Ambiente, lanzamiento dominio.Lanzamiento,
) (dominio.LanzamientoRegistrado, error) {
	contenido, err := codificarContenido(lanzamiento)
	if err != nil {
		return dominio.LanzamientoRegistrado{}, err
	}
	registrado, err := h.registros.RegistrarLanzamiento(
		ctx, ambiente.String(), lanzamiento.Despliegue().String(), contenido)
	if err != nil {
		return dominio.LanzamientoRegistrado{}, fmt.Errorf("lanzamiento: registrar en %q: %w", ambiente, err)
	}
	return dominio.LanzamientoRegistrado{
		Id: registrado.Id, Ambiente: ambiente, Despliegue: lanzamiento.Despliegue(),
		Version: lanzamiento.Version(), Nombre: lanzamiento.Nombre(), Instante: registrado.Instante,
	}, nil
}

func (h *Historial) RegistrarReserva(ctx context.Context, ambiente dominio.Ambiente, reservado bool) error {
	if err := h.registros.RegistrarReserva(ctx, ambiente.String(), reservado); err != nil {
		return fmt.Errorf("lanzamiento: registrar la reserva de %q: %w", ambiente, err)
	}
	return nil
}
