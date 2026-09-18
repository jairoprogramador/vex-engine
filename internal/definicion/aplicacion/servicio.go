package aplicacion

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Pipelines dominio.Pipelines
}

// Servicio es traer el pipeline comprobado. No comprueba nada él: el repositorio solo puede devolver un Pipeline
// que salió de la comprobación, y el servicio lo traduce al lenguaje publicado.
type Servicio struct {
	d Dependencias
}

var (
	_ publicado.ParaEjecucion  = (*Servicio)(nil)
	_ publicado.ParaResolucion = (*Servicio)(nil)
	_ publicado.ParaSimulacion = (*Servicio)(nil)
)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}

func (s *Servicio) DeHoy(ctx context.Context, fuente string) (publicado.Pipeline, error) {
	return publicar(s.d.Pipelines.DeHoy(ctx, fuente))
}

func (s *Servicio) DeUnCommit(ctx context.Context, fuente, commit string) (publicado.Pipeline, error) {
	return publicar(s.d.Pipelines.DeUnCommit(ctx, fuente, commit))
}

func (s *Servicio) VariablesEstandar() []publicado.VariableEstandar {
	estandar := dominio.VariablesEstandar()
	variables := make([]publicado.VariableEstandar, len(estandar))
	for i, v := range estandar {
		variables[i] = publicado.VariableEstandar{
			Nombre: v.Nombre, Metadato: v.Origen == dominio.Metadato, DelPaso: v.DelPaso,
		}
	}
	return variables
}

func publicar(p *dominio.PipelineComprobado, err error) (publicado.Pipeline, error) {
	if err != nil {
		return publicado.Pipeline{}, traducir(err)
	}
	if p == nil {
		return publicado.Pipeline{}, errors.New("definicion: el repositorio no devolvió pipeline ni error")
	}
	return traducirPipeline(p), nil
}

func traducirPipeline(p *dominio.PipelineComprobado) publicado.Pipeline {
	resultado := publicado.Pipeline{Version: p.Version(), Commit: p.Commit(), Hash: p.Hash()}
	for _, a := range p.Ambientes() {
		resultado.Ambientes = append(resultado.Ambientes, publicado.Ambiente(a))
	}
	for _, paso := range p.Pasos() {
		resultado.Pasos = append(resultado.Pasos, traducirPaso(paso))
	}
	for _, v := range p.Variables() {
		resultado.Variables = append(resultado.Variables, publicado.VariableDeclarada{
			Nombre: v.Nombre, Descripcion: v.Descripcion, Ambito: string(v.Ambito), Valor: v.Valor,
		})
	}
	return resultado
}

func traducirPaso(paso dominio.PasoComprobado) publicado.Paso {
	resultado := publicado.Paso{
		Nombre:     paso.Nombre,
		Orden:      paso.Orden,
		Reglas:     []publicado.Regla{},
		EdadMaxima: paso.EdadMaxima,
		Compartido: paso.Ambito != nil,
	}
	for _, r := range paso.Reglas {
		resultado.Reglas = append(resultado.Reglas, publicado.Regla(r))
	}
	for _, c := range paso.Comandos {
		comando := publicado.Comando{
			Nombre: c.Nombre, Descripcion: c.Descripcion, Linea: c.Linea, Directorio: c.Directorio,
			Plantillas: c.Plantillas,
		}
		for _, s := range c.VariablesDeSalida {
			comando.Salidas = append(comando.Salidas, publicado.VariableDeSalida{
				Nombre: s.Nombre, Descripcion: s.Descripcion, Expresion: s.Expresion, Compartida: s.Ambito != nil,
			})
		}
		for _, a := range c.Aserciones {
			comando.Aserciones = append(comando.Aserciones, publicado.Asercion(a))
		}
		resultado.Comandos = append(resultado.Comandos, comando)
	}
	for _, f := range paso.Material {
		resultado.Material = append(resultado.Material, publicado.Fichero(f))
	}
	return resultado
}

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	var fallos *dominio.FallosDeComprobacion
	switch {
	case errors.As(err, &fallos):
		publicados := make([]publicado.Fallo, len(fallos.Fallos))
		for i, f := range fallos.Fallos {
			publicados[i] = publicado.Fallo{
				Invariante: string(f.Invariante), Fichero: f.Fichero, Paso: f.Paso, Ambiente: f.Ambiente,
				Detalle: f.Detalle,
			}
		}
		return publicado.NuevosFallosDeComprobacion(publicados, err.Error())
	case errors.Is(err, dominio.ErrInvalido):
		return &errorTraducido{publicado: publicado.ErrInvalido, causa: err}
	case errors.Is(err, dominio.ErrNoExiste):
		return &errorTraducido{publicado: publicado.ErrNoExiste, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
