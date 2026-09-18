package infraestructura

import (
	"context"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/historial/reservado"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

// Historial implementa dominio.Historial sobre lo que publica el contexto Historial y su relación reservada.
// El dominio de Resolución nunca ve un intento ni una evidencia: esa traducción vive aquí.
type Historial struct {
	registros historialpublicado.ParaResolucion
	valores   reservado.Valores
}

var _ dominio.Historial = (*Historial)(nil)

func NuevoHistorial(registros historialpublicado.ParaResolucion, valores reservado.Valores) *Historial {
	return &Historial{registros: registros, valores: valores}
}

func (h *Historial) RegistrarVariable(
	ctx context.Context, intento, paso, nombre string, hash dominio.HashDeVariable, origen dominio.Origen, ambito dominio.Ambito,
) error {
	contenido, err := codificarContenido(hash, origen, ambito)
	if err != nil {
		return err
	}
	if err := h.registros.RegistrarVariable(ctx, intento, paso, nombre, contenido); err != nil {
		return fmt.Errorf("resolución: registrar la variable %q del paso %q: %w", nombre, paso, err)
	}
	return nil
}

func (h *Historial) GuardarValor(ctx context.Context, intento, paso, nombre, valor string) error {
	if err := h.valores.GuardarValor(ctx, intento, paso, nombre, valor); err != nil {
		return fmt.Errorf("resolución: guardar el valor de %q del paso %q: %w", nombre, paso, err)
	}
	return nil
}

func (h *Historial) UltimaVezDeUnPaso(
	ctx context.Context, paso string, ambito dominio.Ambito,
) (map[string]dominio.HashDeVariable, bool, error) {
	intento, pasoOrigen, ok, err := h.origenDeLaUltimaVez(ctx, paso, ambito)
	if err != nil || !ok {
		return nil, false, err
	}
	variables, err := h.registros.VariablesDeUnPaso(ctx, intento, pasoOrigen)
	if err != nil {
		return nil, false, fmt.Errorf("resolución: variables de la última vez del paso %q: %w", paso, err)
	}
	hashes := make(map[string]dominio.HashDeVariable, len(variables))
	for _, v := range variables {
		hash, _, err := decodificarContenido(v.Contenido)
		if err != nil {
			return nil, false, err
		}
		hashes[v.Nombre] = hash
	}
	return hashes, true, nil
}

func (h *Historial) ValoresDeLaUltimaVez(
	ctx context.Context, paso string, ambito dominio.Ambito,
) (map[string]dominio.ValorDeLaUltimaVez, bool, error) {
	intento, pasoOrigen, ok, err := h.origenDeLaUltimaVez(ctx, paso, ambito)
	if err != nil || !ok {
		return nil, false, err
	}
	variables, err := h.registros.VariablesDeUnPaso(ctx, intento, pasoOrigen)
	if err != nil {
		return nil, false, fmt.Errorf("resolución: variables de la última vez del paso %q: %w", paso, err)
	}
	ambitos := make(map[string]dominio.Ambito, len(variables))
	for _, v := range variables {
		_, a, err := decodificarContenido(v.Contenido)
		if err != nil {
			return nil, false, err
		}
		ambitos[v.Nombre] = a
	}

	valores, err := h.valores.ValoresDeUnPaso(ctx, intento, pasoOrigen)
	if err != nil {
		return nil, false, fmt.Errorf("resolución: valores de la última vez del paso %q: %w", paso, err)
	}
	resultado := make(map[string]dominio.ValorDeLaUltimaVez, len(valores))
	for nombre, valor := range valores {
		a, ok := ambitos[nombre]
		if !ok {
			return nil, false, fmt.Errorf("resolución: el valor de %q no tiene hash registrado con su ámbito", nombre)
		}
		resultado[nombre] = dominio.ValorDeLaUltimaVez{Valor: valor, Ambito: a}
	}
	return resultado, true, nil
}

// origenDeLaUltimaVez sigue el salto de evidencia: si la última vez es una no re-ejecución, el registro real
// —con las variables— vive en el intento y el paso que apunta su evidencia. Nunca hace falta más de un
// salto: una no re-ejecución nunca apunta a otra (Intento.NoReejecutar en historial/dominio lo exige).
func (h *Historial) origenDeLaUltimaVez(
	ctx context.Context, paso string, ambito dominio.Ambito,
) (intento, pasoOrigen string, ok bool, err error) {
	registro, hay, err := h.registros.UltimaVezDeUnPaso(ctx, paso, ambitoAPublicado(ambito))
	if err != nil {
		return "", "", false, fmt.Errorf("resolución: última vez del paso %q: %w", paso, err)
	}
	if !hay {
		return "", "", false, nil
	}
	if registro.Tipo == historialpublicado.NoReejecucion {
		return registro.Evidencia.Intento, registro.Evidencia.Paso, true, nil
	}
	return registro.Intento, registro.Paso, true, nil
}

func ambitoAPublicado(a dominio.Ambito) historialpublicado.Ambito {
	return historialpublicado.Ambito{Compartido: a.EsCompartido(), Ambiente: a.Ambiente()}
}
