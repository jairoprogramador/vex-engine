package aplicacion_test

import (
	"context"
	"fmt"
	"strings"
	"sync"

	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// Los dobles de este fichero son los puertos que el propio dominio de Simulación declara (dominio.Pipelines,
// dominio.Variables, dominio.EspacioTemporal) — nunca los servicios reales de Definición o Resolución: probar
// contra la frontera que Simulación ya decidió es lo que mantiene un contexto aislado del otro (DEC-11.3).

type pipelinesFalsos struct {
	pipeline dominio.Pipeline
	err      error
}

func (p *pipelinesFalsos) DeUnCommit(context.Context, string, string) (dominio.Pipeline, error) {
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) DeUnaCopiaDeTrabajo(context.Context, string) (dominio.Pipeline, error) {
	return p.pipeline, p.err
}

var _ dominio.Pipelines = (*pipelinesFalsos)(nil)

// clave identifica una variable dentro de una simulación: su id, su ámbito y su nombre.
type clave struct{ simulacion, ambito, nombre string }

// variablesFalsas imita lo bastante de resolucion.ParaSimulacion para probar de verdad la interpolación y el
// aislamiento entre simulaciones: declara sin sobreescribir, produce siempre sobreescribiendo, interpola
// sustituyendo ${var.<nombre>}, y falla con *resolucionpublicado.VariableNoEncontradaError —el mismo tipo que
// usa la ACL real— cuando falta un nombre.
type variablesFalsas struct {
	mu              sync.Mutex
	valores         map[clave]string
	interpolaciones []string // "<simulacion>:<ambito>:<resultado>", para comprobar qué valor se usó de verdad
	cerradas        []string
}

func nuevasVariablesFalsas() *variablesFalsas {
	return &variablesFalsas{valores: map[clave]string{}}
}

func (v *variablesFalsas) Declarar(_ context.Context, simulacion string, ambito dominio.Ambito, declaradas []dominio.VariableDeclarada) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, d := range declaradas {
		c := clave{simulacion, ambito.String(), d.Nombre}
		if _, ya := v.valores[c]; ya {
			continue
		}
		v.valores[c] = d.Valor
	}
	return nil
}

func (v *variablesFalsas) Interpolar(_ context.Context, simulacion string, ambito dominio.Ambito, texto string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	resultado := texto
	for {
		inicio := strings.Index(resultado, "${var.")
		if inicio < 0 {
			break
		}
		fin := strings.Index(resultado[inicio:], "}")
		if fin < 0 {
			break
		}
		nombre := resultado[inicio+len("${var.") : inicio+fin]
		valor, ok := v.buscar(simulacion, ambito, nombre)
		if !ok {
			return "", &resolucionpublicado.VariableNoEncontradaError{Nombre: nombre}
		}
		resultado = resultado[:inicio] + valor + resultado[inicio+fin+1:]
	}
	v.interpolaciones = append(v.interpolaciones, simulacion+":"+ambito.String()+":"+resultado)
	return resultado, nil
}

func (v *variablesFalsas) buscar(simulacion string, ambito dominio.Ambito, nombre string) (string, bool) {
	if valor, ok := v.valores[clave{simulacion, ambito.String(), nombre}]; ok {
		return valor, true
	}
	valor, ok := v.valores[clave{simulacion, dominio.AmbitoCompartido().String(), nombre}]
	return valor, ok
}

func (v *variablesFalsas) RegistrarProducido(_ context.Context, simulacion, nombre, valor string, ambito dominio.Ambito) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.valores[clave{simulacion, ambito.String(), nombre}] = valor
	return nil
}

func (v *variablesFalsas) Cerrar(_ context.Context, simulacion string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cerradas = append(v.cerradas, simulacion)
	return nil
}

var _ dominio.Variables = (*variablesFalsas)(nil)

// espacioTemporalFalso da ids incrementales y cuenta cuántas veces se llamó Nuevo() — el caso crítico de
// RD-09 §9 es que se llama una vez por ambiente, no una por llamada a Simular.
type espacioTemporalFalso struct {
	mu       sync.Mutex
	llamadas int
}

func (e *espacioTemporalFalso) Nuevo() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.llamadas++
	return fmt.Sprintf("sim-%d", e.llamadas), nil
}

func (e *espacioTemporalFalso) contador() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.llamadas
}

var _ dominio.EspacioTemporal = (*espacioTemporalFalso)(nil)
