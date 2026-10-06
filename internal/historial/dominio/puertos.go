package dominio

import (
	"context"
	"time"
)

// Los repositorios son uno por agregado y solo añaden y recorren (IT-07 DEC-07.5): no hay actualizar ni
// borrar. Añadir escribe los registros nuevos de un agregado después de los que se leyeron, y devuelve
// ErrConflicto si alguien añadió otro desde entonces (la escritura condicional, IT-10 DEC-10.3).
//
// Leer algo que no existe no es un error: devuelve el agregado sin registros.

type Intentos interface {
	Intento(ctx context.Context, id IdIntento) (*Intento, error)
	Recorrer(ctx context.Context) ([]*Intento, error)
	Anadir(ctx context.Context, intento *Intento) error
}

type Despliegues interface {
	DeUnAmbiente(ctx context.Context, ambiente Ambiente) (*DesplieguesDeUnAmbiente, error)
	Recorrer(ctx context.Context) ([]*DesplieguesDeUnAmbiente, error)
	Anadir(ctx context.Context, despliegues *DesplieguesDeUnAmbiente) error
}

type Ocupaciones interface {
	DeUnAmbiente(ctx context.Context, ambiente Ambiente) (*Ocupacion, error)
	Recorrer(ctx context.Context) ([]*Ocupacion, error)
	Anadir(ctx context.Context, ocupacion *Ocupacion) error
}

type Lanzamientos interface {
	Todos(ctx context.Context) (*LanzamientosDelHistorial, error)
	Anadir(ctx context.Context, lanzamientos *LanzamientosDelHistorial) error
}

type Reservas interface {
	DeUnAmbiente(ctx context.Context, ambiente Ambiente) (*ReservasDeUnAmbiente, error)
	Anadir(ctx context.Context, reservas *ReservasDeUnAmbiente) error
}

// Salidas guarda lo que escribió cada comando de un intento, en el orden en que terminaron. Un intento sin
// salidas devuelve una lista vacía. Añadir escribe una salida después de las previas que se leyeron, y
// devuelve ErrConflicto si alguien añadió otra desde entonces.
type Salidas interface {
	DeUnIntento(ctx context.Context, intento IdIntento) ([]Salida, error)
	Anadir(ctx context.Context, intento IdIntento, previas int, salida Salida) error
}

// Latidos es la señal de vida de un intento en curso: una secuencia por intento, que solo escribe el proceso que
// lo lleva a cabo, mientras vive. Quien encuentra el ambiente ocupado la mira dos veces: si no creció, el
// dueño murió. Por eso es una familia aparte y no registros del intento: no son un hecho del negocio, y no
// compiten con sus registros. Añadir escribe un latido después de los previos que se leyeron, y devuelve
// ErrConflicto si alguien añadió otro desde entonces.
type Latidos interface {
	Cantidad(ctx context.Context, intento IdIntento) (int, error)
	// Ultimo es el instante, según el reloj de quien lo escribió, del último latido; false si no hay ninguno.
	// Solo sirve para saltarse la espera cuando está claro que el dueño vive: nunca decide que murió, porque los
	// relojes de dos máquinas no tienen por qué coincidir.
	Ultimo(ctx context.Context, intento IdIntento) (time.Time, bool, error)
	Anadir(ctx context.Context, intento IdIntento, previos int, instante time.Time) error
}

// Reloj da el instante de cada registro.
type Reloj interface {
	Ahora() time.Time
}

// Identidades da las identidades propias de intentos, despliegues y lanzamientos.
type Identidades interface {
	NuevoIntento() (IdIntento, error)
	NuevoDespliegue() (IdDespliegue, error)
	NuevoLanzamiento() (IdLanzamiento, error)
}

// DespliegueRegistrado es el evento que anuncia un despliegue nuevo, después de escribirlo. El Historial no
// sabe quién lo escucha (IT-04 DEC-04.8).
type DespliegueRegistrado struct {
	Despliegue Despliegue
}
