package dominio

import (
	"slices"
	"time"
)

// RegistroDeOcupacion: un intento ocupó el ambiente.
type RegistroDeOcupacion struct {
	Intento  IdIntento
	Instante time.Time
}

// Ocupacion es qué intento tiene un ambiente en curso: como mucho uno (IT-07 DEC-07.8, DEC-07.9).
//
// Solo se registra que un intento ocupa. La liberación no es otro registro: el ambiente queda libre en
// cuanto el intento de la última ocupación está cerrado o abandonado. Así, cerrar y abandonar escriben un
// solo registro, y no puede quedar un ambiente ocupado por un intento que ya terminó.
//
// Las ocupaciones son también el orden de los intentos de un ambiente: la escritura condicional lo hace
// total aunque los relojes de dos máquinas no coincidan.
type Ocupacion struct {
	ambiente  Ambiente
	registros []RegistroDeOcupacion
	leidos    int
}

// NuevaOcupacion: un ambiente que nunca se ocupó.
func NuevaOcupacion(ambiente Ambiente) *Ocupacion {
	return &Ocupacion{ambiente: ambiente}
}

// ReconstituirOcupacion rehace las ocupaciones leídas del almacén.
func ReconstituirOcupacion(ambiente Ambiente, registros []RegistroDeOcupacion) (*Ocupacion, error) {
	o := NuevaOcupacion(ambiente)
	for n, r := range registros {
		if err := o.anadir(r); err != nil {
			return nil, rechazo("ocupaciones de %q, registro %d: %v", ambiente, n+1, err)
		}
	}
	o.leidos = len(o.registros)
	return o, nil
}

func (o *Ocupacion) Ambiente() Ambiente { return o.ambiente }
func (o *Ocupacion) Leidos() int        { return o.leidos }

// Nuevos son los registros añadidos desde que se leyó.
func (o *Ocupacion) Nuevos() []RegistroDeOcupacion { return slices.Clone(o.registros[o.leidos:]) }

// Ultima es la del intento que ocupa el ambiente, si ese intento no ha terminado.
func (o *Ocupacion) Ultima() (RegistroDeOcupacion, bool) {
	if len(o.registros) == 0 {
		return RegistroDeOcupacion{}, false
	}
	return o.registros[len(o.registros)-1], true
}

// Intentos del ambiente, en orden.
func (o *Ocupacion) Intentos() []IdIntento {
	ids := make([]IdIntento, len(o.registros))
	for k, r := range o.registros {
		ids[k] = r.Intento
	}
	return ids
}

// Ocupar el ambiente con un intento nuevo. Si hubo una ocupación antes, hace falta su intento, para ver
// que terminó.
func (o *Ocupacion) Ocupar(intento IdIntento, instante time.Time, anterior *Intento) error {
	if ultima, ok := o.Ultima(); ok {
		if anterior == nil || anterior.Id() != ultima.Intento {
			return rechazo("para ocupar %q hace falta el intento de su última ocupación, %s", o.ambiente, ultima.Intento)
		}
		if !anterior.Terminado() {
			return &AmbienteOcupadoError{Ambiente: o.ambiente, Intento: ultima.Intento}
		}
	}
	return o.anadir(RegistroDeOcupacion{Intento: intento, Instante: instante})
}

func (o *Ocupacion) anadir(r RegistroDeOcupacion) error {
	if r.Intento == "" || r.Instante.IsZero() {
		return rechazo("una ocupación necesita intento e instante")
	}
	if slices.Contains(o.Intentos(), r.Intento) {
		return rechazo("el intento %s ya ocupó %q", r.Intento, o.ambiente)
	}
	o.registros = append(o.registros, r)
	return nil
}
