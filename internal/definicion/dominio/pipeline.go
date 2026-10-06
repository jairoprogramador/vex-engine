package dominio

import (
	"fmt"
	"slices"
	"time"
)

type PipelineComprobado struct {
	version   string
	commit    string
	hash      string
	ambientes []AmbienteComprobado
	pasos     []PasoComprobado
	variables []VariableDePipelineComprobada
}

type AmbienteComprobado struct {
	Nombre      string
	Descripcion string
	Valor       string
}

type PasoComprobado struct {
	Nombre     string
	Orden      int
	Reglas     []Regla
	EdadMaxima time.Duration
	Comandos   []ComandoComprobado
	Material   []FicheroComprobado
	Ambito     *Ambito
}

type ComandoComprobado struct {
	Nombre            string
	Descripcion       string
	Linea             string
	Directorio        string
	Plantillas        []string
	VariablesDeSalida []VariableDeComandoComprobada
	Aserciones        []AsercionComprobada
}

type VariableDeComandoComprobada struct {
	Nombre      string
	Descripcion string
	Expresion   string
	Ambito      *Ambito
}

type AsercionComprobada struct {
	Descripcion string
	Expresion   string
}

type VariableDePipelineComprobada struct {
	Nombre      string
	Descripcion string
	Ambito      Ambito
	Valor       string
}

type FicheroComprobado struct {
	Ruta       string
	Contenido  string
	Ejecutable bool
	Enlace     string
	Plantilla  bool
}

func (p *PipelineComprobado) Version() string { return p.version }

func (p *PipelineComprobado) Commit() string { return p.commit }

func (p *PipelineComprobado) Hash() string { return p.hash }

func (p *PipelineComprobado) Ambientes() []AmbienteComprobado { return slices.Clone(p.ambientes) }

func (p *PipelineComprobado) Variables() []VariableDePipelineComprobada {
	return slices.Clone(p.variables)
}

func (p *PipelineComprobado) Pasos() []PasoComprobado {
	pasos := make([]PasoComprobado, len(p.pasos))
	for i, paso := range p.pasos {
		pasos[i] = paso.copia()
	}
	return pasos
}

func (p *PipelineComprobado) Paso(nombre string) (PasoComprobado, bool) {
	i := slices.IndexFunc(p.pasos, func(paso PasoComprobado) bool { return paso.Nombre == nombre })
	if i < 0 {
		return PasoComprobado{}, false
	}
	return p.pasos[i].copia(), true
}

func (p PasoComprobado) Directorio() string { return fmt.Sprintf("steps/%02d-%s", p.Orden, p.Nombre) }

func (p PasoComprobado) AmbitoEfectivo(ambiente Ambito) Ambito {
	if p.Ambito != nil {
		return *p.Ambito
	}
	return ambiente
}

func (p PasoComprobado) copia() PasoComprobado {
	p.Reglas = slices.Clone(p.Reglas)
	p.Material = slices.Clone(p.Material)
	p.Ambito = clonarAmbito(p.Ambito)
	comandos := make([]ComandoComprobado, len(p.Comandos))
	for i, c := range p.Comandos {
		c.Plantillas = slices.Clone(c.Plantillas)
		salidas := make([]VariableDeComandoComprobada, len(c.VariablesDeSalida))
		for j, s := range c.VariablesDeSalida {
			s.Ambito = clonarAmbito(s.Ambito)
			salidas[j] = s
		}
		c.VariablesDeSalida = salidas
		c.Aserciones = slices.Clone(c.Aserciones)
		comandos[i] = c
	}
	p.Comandos = comandos
	return p
}

func (v VariableDeComandoComprobada) AmbitoEfectivo(ambitoDelPaso Ambito) Ambito {
	if v.Ambito != nil {
		return *v.Ambito
	}
	return ambitoDelPaso
}

// EsCompartida dice si esta variable de salida declaró su propio ámbito compartido, sin heredarlo de su paso.
// Hoy es el único valor que un *Ambito no-nil puede tener aquí (asignarAmbito no fija otro), pero nombrarlo
// evita que quien lo lea tenga que saberlo.
func (v VariableDeComandoComprobada) EsCompartida() bool { return v.Ambito != nil }
