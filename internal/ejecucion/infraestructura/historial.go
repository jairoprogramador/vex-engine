package infraestructura

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Historial implementa dominio.Historial sobre lo que publica el contexto Historial. El dominio de Ejecución
// nunca ve un Estado, una Apertura ni un RegistroDePaso de historial/publicado: esa traducción vive aquí,
// incluido el salto de evidencia que enlaza una no-reejecución con el registro con el que de verdad se hizo
// (nunca más de uno: una no-reejecución no apunta a otra — la misma regla que sigue resolucion/infraestructura).
type Historial struct {
	historial historialpublicado.ParaEjecucion
}

var _ dominio.Historial = (*Historial)(nil)

func NuevoHistorial(h historialpublicado.ParaEjecucion) *Historial {
	return &Historial{historial: h}
}

func (h *Historial) AbrirIntento(ctx context.Context, a dominio.AperturaDeIntento) (string, error) {
	contenido, err := codificarContenidoDeApertura(a)
	if err != nil {
		return "", err
	}
	pasos := make([]historialpublicado.PasoDeclarado, 0, len(a.Pasos))
	for _, p := range a.Pasos {
		pasos = append(pasos, historialpublicado.PasoDeclarado{Nombre: p.Nombre(), Compartido: p.Compartido()})
	}
	id, err := h.historial.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: a.Ambiente, Solicitante: a.Solicitante, Pasos: pasos, HastaPaso: a.HastaPaso,
		ConCommits: a.ConCommits, HashDelCodigo: a.HashDelCodigo.String(), Contenido: contenido,
	})
	if err != nil {
		return "", fmt.Errorf("ejecución: abrir el intento del ambiente %q: %w", a.Ambiente, err)
	}
	return id, nil
}

func (h *Historial) RegistrarComienzo(ctx context.Context, intento, paso string, recursos dominio.RecursosDeUnPaso) error {
	contenido, err := codificarContenidoDeRegistro(recursos)
	if err != nil {
		return err
	}
	if err := h.historial.RegistrarComienzo(ctx, intento, paso, contenido); err != nil {
		return fmt.Errorf("ejecución: registrar el comienzo del paso %q: %w", paso, err)
	}
	return nil
}

func (h *Historial) RegistrarFinal(
	ctx context.Context, intento, paso string, exitoso bool, recursos dominio.RecursosDeUnPaso,
) error {
	contenido, err := codificarContenidoDeRegistro(recursos)
	if err != nil {
		return err
	}
	if err := h.historial.RegistrarFinal(ctx, intento, paso, exitoso, contenido); err != nil {
		return fmt.Errorf("ejecución: registrar el final del paso %q: %w", paso, err)
	}
	return nil
}

func (h *Historial) RegistrarNoReejecucion(
	ctx context.Context, intento, paso string, evidencia dominio.Evidencia, recursos dominio.RecursosDeUnPaso,
) error {
	contenido, err := codificarContenidoDeRegistro(recursos)
	if err != nil {
		return err
	}
	ev := historialpublicado.Evidencia{Intento: evidencia.Intento, Paso: evidencia.Paso}
	if err := h.historial.RegistrarNoReejecucion(ctx, intento, paso, ev, contenido); err != nil {
		return fmt.Errorf("ejecución: registrar la no-reejecución del paso %q: %w", paso, err)
	}
	return nil
}

func (h *Historial) CerrarIntento(
	ctx context.Context, intento string, desenlace dominio.Desenlace, destino string,
) (string, bool, error) {
	estado, err := estadoAPublicado(desenlace)
	if err != nil {
		return "", false, err
	}
	despliegue, huboDespliegue, err := h.historial.CerrarIntento(ctx, intento, estado, destino)
	if err != nil {
		return "", false, fmt.Errorf("ejecución: cerrar el intento %q: %w", intento, err)
	}
	return despliegue.Id, huboDespliegue, nil
}

func (h *Historial) UltimaVezDeUnPaso(
	ctx context.Context, paso string, ambito dominio.Ambito,
) (dominio.UltimaVezDeUnPaso, error) {
	registro, hay, err := h.historial.UltimaVezDeUnPaso(ctx, paso, ambitoAPublicado(ambito))
	if err != nil {
		return dominio.UltimaVezDeUnPaso{}, fmt.Errorf("ejecución: última vez del paso %q: %w", paso, err)
	}
	if !hay {
		return dominio.UltimaVezDeUnPaso{Hay: false}, nil
	}

	switch registro.Tipo {
	case historialpublicado.Comienzo:
		return dominio.UltimaVezDeUnPaso{Hay: true, Valida: false}, nil
	case historialpublicado.Final:
		if !registro.Exitoso {
			return dominio.UltimaVezDeUnPaso{Hay: true, Valida: false}, nil
		}
		recursos, err := decodificarContenidoDeRegistro(registro.Contenido)
		if err != nil {
			return dominio.UltimaVezDeUnPaso{}, err
		}
		return dominio.UltimaVezDeUnPaso{
			Hay: true, Valida: true, Recursos: recursos, Edad: time.Since(registro.Instante),
			Evidencia: dominio.Evidencia{Intento: registro.Intento, Paso: registro.Paso},
		}, nil
	case historialpublicado.NoReejecucion:
		recursos, err := decodificarContenidoDeRegistro(registro.Contenido)
		if err != nil {
			return dominio.UltimaVezDeUnPaso{}, err
		}
		return dominio.UltimaVezDeUnPaso{
			Hay: true, Valida: true, Recursos: recursos, Edad: time.Since(registro.Instante),
			Evidencia: dominio.Evidencia{Intento: registro.Evidencia.Intento, Paso: registro.Evidencia.Paso},
		}, nil
	default:
		return dominio.UltimaVezDeUnPaso{}, fmt.Errorf("ejecución: tipo de registro desconocido: %q", registro.Tipo)
	}
}

func (h *Historial) DespliegueParaRollback(ctx context.Context, despliegue string) (dominio.Destino, error) {
	d, intento, err := h.historial.DespliegueYSuIntento(ctx, despliegue)
	if err != nil {
		return dominio.Destino{}, fmt.Errorf("ejecución: el despliegue %q: %w", despliegue, err)
	}
	ca, err := decodificarContenidoDeApertura(intento.Apertura.Contenido)
	if err != nil {
		return dominio.Destino{}, err
	}
	destino, err := dominio.NuevoDestino(
		d.Id, d.Ambiente, ca.FuenteDelProyecto, ca.CommitDelProyecto, ca.FuenteDelPipeline, ca.CommitDelPipeline,
	)
	if err != nil {
		return dominio.Destino{}, fmt.Errorf("ejecución: el despliegue %q: %w", despliegue, err)
	}
	return destino, nil
}

func ambitoAPublicado(a dominio.Ambito) historialpublicado.Ambito {
	return historialpublicado.Ambito{Compartido: a.EsCompartido(), Ambiente: a.Ambiente()}
}

func estadoAPublicado(d dominio.Desenlace) (historialpublicado.Estado, error) {
	switch d {
	case dominio.Exitoso:
		return historialpublicado.Exitoso, nil
	case dominio.Fallido:
		return historialpublicado.Fallido, nil
	case dominio.Cancelado:
		return historialpublicado.Cancelado, nil
	default:
		return "", fmt.Errorf("ejecución: desenlace desconocido: %v", d)
	}
}
