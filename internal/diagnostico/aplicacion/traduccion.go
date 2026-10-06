package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

func respuestaAPublicado(r dominio.Respuesta) publicado.Respuesta {
	resultado := publicado.Respuesta{Forma: publicado.FormaDeRespuesta(r.Forma()), Mensaje: r.Mensaje()}
	atribucion, sustento, ok := r.AtribucionConSustento()
	if !ok {
		return resultado
	}
	resultado.Atribucion = ejesAPublicado(atribucion.Ejes())
	resultado.Sustento = sustentoAPublicado(sustento)
	return resultado
}

func ejesAPublicado(ejes []dominio.Eje) []publicado.Eje {
	resultado := make([]publicado.Eje, 0, len(ejes))
	for _, e := range ejes {
		resultado = append(resultado, publicado.Eje(e))
	}
	return resultado
}

func sustentoAPublicado(s dominio.Sustento) publicado.Sustento {
	resultado := publicado.Sustento{
		IntentoQueFalla:              s.IntentoQueFalla.String(),
		InstanteDelIntentoQueFalla:   s.InstanteDelIntentoQueFalla,
		VariablesDeclaradasCambiadas: cambiosDeVariableAPublicado(s.VariablesDeclaradasCambiadas),
		VariablesProducidasCambiadas: cambiosDeVariableAPublicado(s.VariablesProducidasCambiadas),
	}
	for _, p := range s.PasosComparadosPorEvidencia {
		resultado.PasosComparadosPorEvidencia = append(resultado.PasosComparadosPorEvidencia, p.String())
	}
	for _, ec := range s.EjesCambiados {
		cambio := publicado.CambioDeEje{Eje: publicado.Eje(ec.Eje)}
		for _, p := range ec.Pasos {
			cambio.Pasos = append(cambio.Pasos, p.String())
		}
		resultado.EjesCambiados = append(resultado.EjesCambiados, cambio)
	}
	for _, c := range s.Comparaciones {
		resultado.Comparaciones = append(resultado.Comparaciones, comparacionAPublicado(c))
	}
	return resultado
}

func cambiosDeVariableAPublicado(cambios []dominio.CambioDeVariable) []publicado.CambioDeVariable {
	resultado := make([]publicado.CambioDeVariable, 0, len(cambios))
	for _, c := range cambios {
		resultado = append(resultado, publicado.CambioDeVariable{Paso: c.Paso.String(), Nombre: c.Nombre.String()})
	}
	return resultado
}

func comparacionAPublicado(c dominio.Comparacion) publicado.Comparacion {
	resultado := publicado.Comparacion{
		Despliegue: c.Referencia().Despliegue().String(),
		Ambiente:   c.Referencia().Ambiente().String(),
		Razon:      publicado.RazonDeReferencia(c.Referencia().Razon()),
		Instante:   c.Referencia().Instante(),
		Intento:    c.IntentoDeLaReferencia().String(),
	}
	cantidad, hay := c.CantidadDeIntentos()
	resultado.CantidadDeIntentos, resultado.HayCantidadDeIntentos = cantidad, hay
	for _, p := range c.Pasos() {
		if p.ComparadoPorEvidencia {
			resultado.PasosComparadosPorEvidencia = append(resultado.PasosComparadosPorEvidencia, p.Paso.String())
		}
	}
	return resultado
}

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, dominio.ErrInvalido) {
		return &errorTraducido{publicado: publicado.ErrInvalido, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
