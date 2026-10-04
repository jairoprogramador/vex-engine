package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

// PreguntarLaCausa es la única operación publicada del core (docs/modelo/contextos/diagnostico.md,
// «Servicio de aplicación: preguntar la causa»):
//  1. determina el intento que falla;
//  2. si está cancelado o sin desenlace, no se atribuye (ES-7) — sin tocar el ACL;
//  3. elige la referencia; si no hay ninguna anterior al intento, sin referencia y con su mensaje (ES-6);
//  4. pide los ejes de cada paso, del que falla y de la referencia, a través del ACL;
//  5. compara la referencia y arma la cantidad de intentos si es la del mismo ambiente (ES-8);
//  6. elimina y arma el sustento.
func (s *Servicio) PreguntarLaCausa(ctx context.Context, p publicado.PeticionDeDiagnostico) (publicado.Respuesta, error) {
	intentoId, err := s.resolverIntentoQueFalla(ctx, p)
	if err != nil {
		return publicado.Respuesta{}, traducir(err)
	}
	intento, err := s.d.Historial.Intento(ctx, intentoId)
	if err != nil {
		return publicado.Respuesta{}, traducir(err)
	}

	if intento.Estado.SinDesenlace() || intento.Estado == dominio.Cancelado {
		return respuestaAPublicado(dominio.RespuestaNoSeAtribuye()), nil
	}

	candidatos, err := s.resolverCandidatos(ctx, p, intento)
	if err != nil {
		return publicado.Respuesta{}, traducir(err)
	}
	candidatas := dominio.ElegirReferencias(candidatos, intento.Id)
	if len(candidatas) == 0 {
		return respuestaAPublicado(dominio.RespuestaSinReferencia()), nil
	}

	ejesDelQueFalla, producidasDelQueFalla, err := s.d.Historial.EjesDelIntento(ctx, intento.Id)
	if err != nil {
		return publicado.Respuesta{}, traducir(err)
	}

	comparaciones := make([]dominio.Comparacion, 0, len(candidatas))
	for _, candidata := range candidatas {
		referencia, producidasDeLaReferencia, err := s.d.Historial.Referencia(ctx, candidata.Despliegue, candidata.Razon)
		if err != nil {
			return publicado.Respuesta{}, traducir(err)
		}
		pasos := dominio.CompararPasos(ejesDelQueFalla, referencia.Ejes())
		producidasCambiadas := dominio.CompararProducidas(producidasDelQueFalla, producidasDeLaReferencia)
		comparacion := dominio.NuevaComparacion(referencia, pasos, producidasCambiadas)
		if candidata.Razon == dominio.MismoAmbiente {
			cantidad, err := s.d.Historial.CantidadDeIntentos(ctx, candidata.Despliegue.Id, intento.Id)
			if err != nil {
				return publicado.Respuesta{}, traducir(err)
			}
			comparacion = comparacion.ConCantidadDeIntentos(cantidad)
		}
		comparaciones = append(comparaciones, comparacion)
	}

	atribucion := dominio.Eliminacion(comparaciones)
	sustento := dominio.NuevoSustento(intento.Instante, comparaciones)
	return respuestaAPublicado(dominio.NuevaRespuestaConAtribucion(atribucion, sustento)), nil
}

// resolverIntentoQueFalla: el que se indica, o el de un lanzamiento, o el último del ambiente si no se
// indica ninguno (DEC-06.16).
func (s *Servicio) resolverIntentoQueFalla(ctx context.Context, p publicado.PeticionDeDiagnostico) (dominio.IdIntento, error) {
	switch {
	case p.Intento != "":
		return dominio.NuevoIdIntento(p.Intento)
	case p.Lanzamiento != "":
		lanzamientoId, err := dominio.NuevoIdLanzamiento(p.Lanzamiento)
		if err != nil {
			return dominio.IdIntento{}, err
		}
		despliegueId, err := s.d.Historial.DespliegueDeUnLanzamiento(ctx, lanzamientoId)
		if err != nil {
			return dominio.IdIntento{}, err
		}
		despliegue, err := s.d.Historial.Despliegue(ctx, despliegueId)
		if err != nil {
			return dominio.IdIntento{}, err
		}
		return despliegue.Intento, nil
	default:
		ambiente, err := dominio.NuevaAmbiente(p.Ambiente)
		if err != nil {
			return dominio.IdIntento{}, err
		}
		id, hay, err := s.d.Historial.UltimoIntentoDeUnAmbiente(ctx, ambiente)
		if err != nil {
			return dominio.IdIntento{}, err
		}
		if !hay {
			return dominio.IdIntento{}, fmt.Errorf("%w: el ambiente %q no tiene ningún intento", dominio.ErrInvalido, ambiente)
		}
		return id, nil
	}
}

// resolverCandidatos arma lo que ElegirReferencias necesita: si el usuario eligió una, solo esa; si no,
// el último despliegue del mismo ambiente anterior al intento, si existe (DEC-06.6).
func (s *Servicio) resolverCandidatos(
	ctx context.Context, p publicado.PeticionDeDiagnostico, intento dominio.IntentoDeDiagnostico,
) (dominio.CandidatosParaElegirReferencias, error) {
	if p.Referencia != "" {
		id, err := dominio.NuevoIdDespliegue(p.Referencia)
		if err != nil {
			return dominio.CandidatosParaElegirReferencias{}, err
		}
		elegida, err := s.d.Historial.Despliegue(ctx, id)
		if err != nil {
			return dominio.CandidatosParaElegirReferencias{}, err
		}
		return dominio.CandidatosParaElegirReferencias{ElegidaPorElUsuario: &elegida}, nil
	}

	mismoAmbiente, hay, err := s.d.Historial.UltimoDespliegueAnteriorA(ctx, intento.Ambiente, intento.Instante)
	if err != nil {
		return dominio.CandidatosParaElegirReferencias{}, err
	}
	if !hay {
		return dominio.CandidatosParaElegirReferencias{}, nil
	}
	return dominio.CandidatosParaElegirReferencias{UltimoDelMismoAmbiente: &mismoAmbiente}, nil
}
