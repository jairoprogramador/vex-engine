package dominio

import (
	"slices"
	"time"
)

// Despliegue es un intento exitoso de todos los pasos de un pipeline, con identidad propia y un padre. Solo
// lo crea la factoría Intento.Desplegar.
type Despliegue struct {
	id       IdDespliegue
	ambiente Ambiente
	intento  IdIntento
	padre    IdDespliegue
	instante time.Time
}

// ReconstituirDespliegue rehace un despliegue leído del almacén. Se comprueba al añadirlo a los despliegues
// de su ambiente.
func ReconstituirDespliegue(
	id IdDespliegue, ambiente Ambiente, intento IdIntento, padre IdDespliegue, instante time.Time,
) Despliegue {
	return Despliegue{id: id, ambiente: ambiente, intento: intento, padre: padre, instante: instante}
}

func (d Despliegue) Id() IdDespliegue    { return d.id }
func (d Despliegue) Ambiente() Ambiente  { return d.ambiente }
func (d Despliegue) Intento() IdIntento  { return d.intento }
func (d Despliegue) Padre() IdDespliegue { return d.padre }
func (d Despliegue) Instante() time.Time { return d.instante }

// DesplieguesDeUnAmbiente son los despliegues de un ambiente, en el orden en que se registraron. Es contra lo
// que se comprueba el padre de uno nuevo, y por eso la escritura condicional se hace sobre todos ellos.
type DesplieguesDeUnAmbiente struct {
	ambiente    Ambiente
	despliegues []Despliegue
	leidos      int
}

// NuevosDesplieguesDeUnAmbiente: un ambiente sin despliegues.
func NuevosDesplieguesDeUnAmbiente(ambiente Ambiente) *DesplieguesDeUnAmbiente {
	return &DesplieguesDeUnAmbiente{ambiente: ambiente}
}

// ReconstituirDesplieguesDeUnAmbiente rehace los despliegues leídos del almacén, comprobándolos en orden.
func ReconstituirDesplieguesDeUnAmbiente(
	ambiente Ambiente, despliegues []Despliegue,
) (*DesplieguesDeUnAmbiente, error) {
	d := NuevosDesplieguesDeUnAmbiente(ambiente)
	for n, despliegue := range despliegues {
		if err := d.anadir(despliegue); err != nil {
			return nil, rechazo("despliegues de %q, registro %d: %v", ambiente, n+1, err)
		}
	}
	d.leidos = len(d.despliegues)
	return d, nil
}

func (d *DesplieguesDeUnAmbiente) Ambiente() Ambiente { return d.ambiente }
func (d *DesplieguesDeUnAmbiente) Leidos() int        { return d.leidos }

// Todos, del primero al último.
func (d *DesplieguesDeUnAmbiente) Todos() []Despliegue { return slices.Clone(d.despliegues) }

// Nuevos son los añadidos desde que se leyeron.
func (d *DesplieguesDeUnAmbiente) Nuevos() []Despliegue {
	return slices.Clone(d.despliegues[d.leidos:])
}

// Ultimo es único porque en un ambiente el tiempo es un orden total.
func (d *DesplieguesDeUnAmbiente) Ultimo() (Despliegue, bool) {
	if len(d.despliegues) == 0 {
		return Despliegue{}, false
	}
	return d.despliegues[len(d.despliegues)-1], true
}

func (d *DesplieguesDeUnAmbiente) Buscar(id IdDespliegue) (Despliegue, bool) {
	for _, despliegue := range d.despliegues {
		if despliegue.id == id {
			return despliegue, true
		}
	}
	return Despliegue{}, false
}

func (d *DesplieguesDeUnAmbiente) DeUnIntento(intento IdIntento) (Despliegue, bool) {
	for _, despliegue := range d.despliegues {
		if despliegue.intento == intento {
			return despliegue, true
		}
	}
	return Despliegue{}, false
}

// anadir comprueba las invariantes del despliegue que se pueden ver desde su ambiente: es de este ambiente,
// su intento no tiene otro, y su padre es uno anterior o ninguno si es el primero. Las de su intento las
// comprueba la factoría.
func (d *DesplieguesDeUnAmbiente) anadir(nuevo Despliegue) error {
	switch {
	case nuevo.id == "" || nuevo.intento == "" || nuevo.instante.IsZero():
		return rechazo("un despliegue necesita identidad, intento e instante")
	case nuevo.ambiente != d.ambiente:
		return rechazo("el despliegue %s es de %q, no de %q", nuevo.id, nuevo.ambiente, d.ambiente)
	}
	if _, ok := d.Buscar(nuevo.id); ok {
		return rechazo("el despliegue %s ya existe", nuevo.id)
	}
	if otro, ok := d.DeUnIntento(nuevo.intento); ok {
		return rechazo("el intento %s ya tiene el despliegue %s", nuevo.intento, otro.id)
	}
	if nuevo.padre == "" && len(d.despliegues) > 0 {
		return rechazo("el despliegue %s no es el primero de %q, y necesita padre", nuevo.id, d.ambiente)
	}
	if _, ok := d.Buscar(nuevo.padre); nuevo.padre != "" && !ok {
		return rechazo("el padre %s no es un despliegue anterior de %q", nuevo.padre, d.ambiente)
	}
	d.despliegues = append(d.despliegues, nuevo)
	return nil
}
