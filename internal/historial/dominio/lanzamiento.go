package dominio

import (
	"slices"
	"time"
)

// Lanzamiento es un despliegue que se hace visible, con fecha. Su versión y su nombre son contenido de
// Lanzamiento.
type Lanzamiento struct {
	id         IdLanzamiento
	ambiente   Ambiente
	despliegue IdDespliegue
	instante   time.Time
	contenido  Contenido
}

// ReconstituirLanzamiento rehace un lanzamiento leído del almacén.
func ReconstituirLanzamiento(
	id IdLanzamiento, ambiente Ambiente, despliegue IdDespliegue, instante time.Time, contenido Contenido,
) Lanzamiento {
	return Lanzamiento{id: id, ambiente: ambiente, despliegue: despliegue, instante: instante, contenido: contenido}
}

func (l Lanzamiento) Id() IdLanzamiento        { return l.id }
func (l Lanzamiento) Ambiente() Ambiente       { return l.ambiente }
func (l Lanzamiento) Despliegue() IdDespliegue { return l.despliegue }
func (l Lanzamiento) Instante() time.Time      { return l.instante }
func (l Lanzamiento) Contenido() Contenido     { return l.contenido }

// LanzamientosDelHistorial son todos los lanzamientos, de todos los ambientes, en orden. Van juntos porque la
// versión es un número por proyecto que no se puede repetir (IT-10 DEC-10.8), y la escritura condicional
// sobre todos ellos es lo que lo evita.
type LanzamientosDelHistorial struct {
	lanzamientos []Lanzamiento
	leidos       int
}

// ReconstituirLanzamientos rehace los lanzamientos leídos del almacén. Su despliegue se comprobó al lanzar.
func ReconstituirLanzamientos(lanzamientos []Lanzamiento) *LanzamientosDelHistorial {
	return &LanzamientosDelHistorial{lanzamientos: slices.Clone(lanzamientos), leidos: len(lanzamientos)}
}

func (l *LanzamientosDelHistorial) Leidos() int { return l.leidos }

// Todos, del primero al último.
func (l *LanzamientosDelHistorial) Todos() []Lanzamiento { return slices.Clone(l.lanzamientos) }

// Nuevos son los añadidos desde que se leyeron.
func (l *LanzamientosDelHistorial) Nuevos() []Lanzamiento {
	return slices.Clone(l.lanzamientos[l.leidos:])
}

// UltimoDeUnAmbiente es el lanzamiento más reciente de un ambiente.
func (l *LanzamientosDelHistorial) UltimoDeUnAmbiente(ambiente Ambiente) (Lanzamiento, bool) {
	for k := len(l.lanzamientos) - 1; k >= 0; k-- {
		if l.lanzamientos[k].ambiente == ambiente {
			return l.lanzamientos[k], true
		}
	}
	return Lanzamiento{}, false
}

// Lanzar un despliegue, que tiene que existir y ser de ese ambiente.
func (l *LanzamientosDelHistorial) Lanzar(
	id IdLanzamiento, despliegues *DesplieguesDeUnAmbiente, despliegue IdDespliegue,
	instante time.Time, contenido Contenido,
) (Lanzamiento, error) {
	if id == "" || instante.IsZero() || despliegues == nil {
		return Lanzamiento{}, rechazo("un lanzamiento necesita identidad, instante y los despliegues de su ambiente")
	}
	if _, ok := despliegues.Buscar(despliegue); !ok {
		return Lanzamiento{}, rechazo("el despliegue %s no es de %q", despliegue, despliegues.Ambiente())
	}
	nuevo := Lanzamiento{
		id: id, ambiente: despliegues.Ambiente(), despliegue: despliegue, instante: instante, contenido: contenido,
	}
	l.lanzamientos = append(l.lanzamientos, nuevo)
	return nuevo, nil
}
