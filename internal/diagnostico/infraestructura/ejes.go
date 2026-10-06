package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// EjesDelIntento es la factoría del ACL para el intento que falla (docs/modelo/contextos/diagnostico.md,
// «Factoría»).
func (h *Historial) EjesDelIntento(
	ctx context.Context, intento dominio.IdIntento,
) ([]dominio.EjesDeUnPaso, []dominio.ProducidasDeUnPaso, error) {
	i, err := h.registros.Intento(ctx, intento.String())
	if err != nil {
		return nil, nil, fmt.Errorf("diagnóstico: los ejes del intento %s: %w", intento, err)
	}
	ejes, producidas, err := h.ejesYProducidas(ctx, i)
	if err != nil {
		return nil, nil, fmt.Errorf("diagnóstico: los ejes del intento %s: %w", intento, err)
	}
	return ejes, producidas, nil
}

// Referencia es la factoría del ACL para un despliegue de referencia: arma la dominio.Referencia a partir
// de los registros del intento con el que se hizo ese despliegue.
func (h *Historial) Referencia(
	ctx context.Context, despliegue dominio.DespliegueDeDiagnostico, razon dominio.RazonDeReferencia,
) (dominio.Referencia, []dominio.ProducidasDeUnPaso, error) {
	i, err := h.registros.Intento(ctx, despliegue.Intento.String())
	if err != nil {
		return dominio.Referencia{}, nil, fmt.Errorf("diagnóstico: la referencia %s: %w", despliegue.Id, err)
	}
	ejes, producidas, err := h.ejesYProducidas(ctx, i)
	if err != nil {
		return dominio.Referencia{}, nil, fmt.Errorf("diagnóstico: la referencia %s: %w", despliegue.Id, err)
	}
	referencia := dominio.NuevaReferencia(
		despliegue.Id, despliegue.Ambiente, despliegue.Instante, razon, ejes, producidas,
	)
	return referencia, producidas, nil
}

// ejesYProducidas recorre los pasos que el intento alcanzó, eligiendo por cada uno el registro con que de
// verdad se hizo (DEC-06.13) y separando las variables declaradas de las producidas (DEC-06.10).
func (h *Historial) ejesYProducidas(
	ctx context.Context, i historialpublicado.Intento,
) ([]dominio.EjesDeUnPaso, []dominio.ProducidasDeUnPaso, error) {
	registrosPorPaso := elegirRegistroDeVerdad(i.Registros)

	var ejes []dominio.EjesDeUnPaso
	var producidas []dominio.ProducidasDeUnPaso
	for _, registro := range registrosPorPaso {
		nombrePaso, err := dominio.NuevoNombrePaso(registro.Paso)
		if err != nil {
			return nil, nil, err
		}

		recurso, comparadoPorEvidencia, err := h.seguirEvidencia(ctx, i, registro)
		if err != nil {
			return nil, nil, err
		}

		hashCodigo, hashInstrucciones, err := decodificarRecursosDePaso(recurso.Contenido)
		if err != nil {
			return nil, nil, err
		}
		codigo, err := dominio.NuevoHashDelCodigo(hashCodigo)
		if err != nil {
			return nil, nil, err
		}
		instrucciones, err := dominio.NuevoHashDeInstrucciones(hashInstrucciones)
		if err != nil {
			return nil, nil, err
		}

		declaradas, producidasDelPaso, err := h.variablesDelPaso(ctx, recurso.Intento, recurso.Paso)
		if err != nil {
			return nil, nil, err
		}

		ejes = append(ejes, dominio.NuevosEjesDeUnPaso(nombrePaso, codigo, instrucciones, declaradas, comparadoPorEvidencia))
		producidas = append(producidas, dominio.NuevasProducidasDeUnPaso(nombrePaso, producidasDelPaso))
	}
	return ejes, producidas, nil
}

// elegirRegistroDeVerdad da, por cada paso que el intento alcanzó, el registro con que de verdad se hizo:
// Final si lo hay, si no NoReejecucion, si no Comienzo (un paso que empezó y nunca terminó).
func elegirRegistroDeVerdad(registros []historialpublicado.RegistroDePaso) map[string]historialpublicado.RegistroDePaso {
	prioridad := map[historialpublicado.TipoDeRegistroDePaso]int{
		historialpublicado.Comienzo:      0,
		historialpublicado.NoReejecucion: 1,
		historialpublicado.Final:         2,
	}
	elegidos := map[string]historialpublicado.RegistroDePaso{}
	for _, r := range registros {
		actual, hay := elegidos[r.Paso]
		if !hay || prioridad[r.Tipo] > prioridad[actual.Tipo] {
			elegidos[r.Paso] = r
		}
	}
	return elegidos
}

// seguirEvidencia sigue el salto de evidencia si el registro real de un paso es una no-reejecución: los
// recursos están en el registro al que apunta, que puede venir de otro ambiente (DEC-06.5, ES-4). Nunca
// hace falta más de un salto: una no-reejecución nunca apunta a otra (RegistrarNoReejecucion en Historial
// lo exige).
func (h *Historial) seguirEvidencia(
	ctx context.Context, i historialpublicado.Intento, registro historialpublicado.RegistroDePaso,
) (historialpublicado.RegistroDePaso, bool, error) {
	if registro.Tipo != historialpublicado.NoReejecucion {
		return registro, false, nil
	}
	origen := i
	if registro.Evidencia.Intento != i.Id {
		otro, err := h.registros.Intento(ctx, registro.Evidencia.Intento)
		if err != nil {
			return historialpublicado.RegistroDePaso{}, false, fmt.Errorf(
				"diagnóstico: seguir la evidencia del paso %q: %w", registro.Paso, err,
			)
		}
		origen = otro
	}
	for _, r := range origen.Registros {
		if r.Paso == registro.Evidencia.Paso && r.Tipo == historialpublicado.Final {
			return r, true, nil
		}
	}
	return historialpublicado.RegistroDePaso{}, false, fmt.Errorf(
		"diagnóstico: la evidencia del paso %q no encontró su registro final", registro.Paso,
	)
}

func (h *Historial) variablesDelPaso(
	ctx context.Context, intento, paso string,
) (declaradas, producidas map[dominio.NombreDeVariable]dominio.HashDeVariable, err error) {
	variables, err := h.registros.VariablesDeUnPaso(ctx, intento, paso)
	if err != nil {
		return nil, nil, fmt.Errorf("diagnóstico: las variables del paso %q: %w", paso, err)
	}
	declaradas = map[dominio.NombreDeVariable]dominio.HashDeVariable{}
	producidas = map[dominio.NombreDeVariable]dominio.HashDeVariable{}
	for _, v := range variables {
		hash, origen, err := decodificarVariable(v.Contenido)
		if err != nil {
			return nil, nil, err
		}
		nombre, err := dominio.NuevoNombreDeVariable(v.Nombre)
		if err != nil {
			return nil, nil, err
		}
		hashDeVariable, err := dominio.NuevoHashDeVariable(hash)
		if err != nil {
			return nil, nil, err
		}
		if origen == origenDeclarada {
			declaradas[nombre] = hashDeVariable
		} else {
			producidas[nombre] = hashDeVariable
		}
	}
	return declaradas, producidas, nil
}
