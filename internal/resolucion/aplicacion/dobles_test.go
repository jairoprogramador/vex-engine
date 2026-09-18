package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// Los dobles de este fichero son los puertos que el propio dominio de Resolución declara
// (dominio.Historial, dominio.Definicion) — nunca los servicios reales de Historial o Definición: probar
// contra la frontera que Resolución ya decidió es lo que mantiene un contexto aislado del otro (DEC-11.3).

func ambitoProdPublicado() publicado.Ambito { return publicado.Ambito{Ambiente: "prod"} }

func ambitoCompartidoPublicado() publicado.Ambito { return publicado.Ambito{Compartido: true} }

func mustAmbito(t *testing.T, ambiente string) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente(ambiente)
	require.NoError(t, err)
	return a
}

func nuevoServicio(t *testing.T, historial *historialFalso, def *definicionFalsa) *aplicacion.Servicio {
	t.Helper()
	if historial == nil {
		historial = nuevoHistorialFalso(t)
	}
	if def == nil {
		def = &definicionFalsa{}
	}
	return aplicacion.NuevoServicio(aplicacion.Dependencias{Historial: historial, Definicion: def})
}

// definicionFalsa da las declaradas y las estándar que la prueba configure, sin ficheros de por medio.
type definicionFalsa struct {
	declaradas []dominio.VariableDeclarada
	estandar   []dominio.VariableEstandar
}

func (d *definicionFalsa) VariablesDeclaradas(
	_ context.Context, _, _ string,
) ([]dominio.VariableDeclarada, error) {
	return d.declaradas, nil
}

func (d *definicionFalsa) VariablesEstandar() []dominio.VariableEstandar { return d.estandar }

var _ dominio.Definicion = (*definicionFalsa)(nil)

// historialFalso guarda en memoria lo que se le registra, para que las pruebas comprueben el orden y el
// contenido de las escrituras sin depender del Historial real. prohibido hace fallar la prueba si se le
// llama a cualquier método — es el doble que exige DEC-04.10: una petición sin intento no toca Historial.
type historialFalso struct {
	t *testing.T

	ultimaVez        map[string]map[string]dominio.HashDeVariable
	valoresUltimaVez map[string]map[string]dominio.ValorDeLaUltimaVez

	registros        []variableRegistrada
	valoresGuardados []valorGuardado
	orden            []string

	fallarGuardarValor      bool
	fallarRegistrarVariable bool
	prohibido               bool
}

type variableRegistrada struct {
	intento, paso, nombre string
	hash                  dominio.HashDeVariable
	origen                dominio.Origen
	ambito                dominio.Ambito
}

type valorGuardado struct{ intento, paso, nombre, valor string }

func nuevoHistorialFalso(t *testing.T) *historialFalso {
	t.Helper()
	return &historialFalso{
		t:                t,
		ultimaVez:        map[string]map[string]dominio.HashDeVariable{},
		valoresUltimaVez: map[string]map[string]dominio.ValorDeLaUltimaVez{},
	}
}

func (h *historialFalso) clave(paso string, ambito dominio.Ambito) string {
	return paso + "|" + ambito.String()
}

func (h *historialFalso) RegistrarVariable(
	_ context.Context, intento, paso, nombre string, hash dominio.HashDeVariable, origen dominio.Origen, ambito dominio.Ambito,
) error {
	if h.prohibido {
		h.t.Fatalf("no se esperaba llamar a Historial.RegistrarVariable (%q)", nombre)
	}
	if h.fallarRegistrarVariable {
		return errors.New("falla simulada de RegistrarVariable")
	}
	h.registros = append(h.registros, variableRegistrada{intento, paso, nombre, hash, origen, ambito})
	h.orden = append(h.orden, "hash:"+nombre)
	return nil
}

func (h *historialFalso) GuardarValor(_ context.Context, intento, paso, nombre, valor string) error {
	if h.prohibido {
		h.t.Fatalf("no se esperaba llamar a Historial.GuardarValor (%q)", nombre)
	}
	if h.fallarGuardarValor {
		return errors.New("falla simulada de GuardarValor")
	}
	h.valoresGuardados = append(h.valoresGuardados, valorGuardado{intento, paso, nombre, valor})
	h.orden = append(h.orden, "valor:"+nombre)
	return nil
}

func (h *historialFalso) UltimaVezDeUnPaso(
	_ context.Context, paso string, ambito dominio.Ambito,
) (map[string]dominio.HashDeVariable, bool, error) {
	if h.prohibido {
		h.t.Fatalf("no se esperaba llamar a Historial.UltimaVezDeUnPaso (%q)", paso)
	}
	hashes, ok := h.ultimaVez[h.clave(paso, ambito)]
	return hashes, ok, nil
}

func (h *historialFalso) ValoresDeLaUltimaVez(
	_ context.Context, paso string, ambito dominio.Ambito,
) (map[string]dominio.ValorDeLaUltimaVez, bool, error) {
	if h.prohibido {
		h.t.Fatalf("no se esperaba llamar a Historial.ValoresDeLaUltimaVez (%q)", paso)
	}
	valores, ok := h.valoresUltimaVez[h.clave(paso, ambito)]
	return valores, ok, nil
}

var _ dominio.Historial = (*historialFalso)(nil)
