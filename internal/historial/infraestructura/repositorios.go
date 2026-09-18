package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

// Los cinco repositorios sobre un Almacen. Todos escriben en el mismo sitio, detrás de la interfaz del
// Historial (IT-06 DEC-06.18). Cada agregado es una secuencia, y Anadir escribe sus registros nuevos a partir
// de los que se leyeron: si alguien añadió otro antes, el almacén devuelve dominio.ErrConflicto.

type IntentosEnAlmacen struct{ almacen Almacen }

type DesplieguesEnAlmacen struct{ almacen Almacen }

type OcupacionesEnAlmacen struct{ almacen Almacen }

type LanzamientosEnAlmacen struct{ almacen Almacen }

type ReservasEnAlmacen struct{ almacen Almacen }

var (
	_ dominio.Intentos     = IntentosEnAlmacen{}
	_ dominio.Despliegues  = DesplieguesEnAlmacen{}
	_ dominio.Ocupaciones  = OcupacionesEnAlmacen{}
	_ dominio.Lanzamientos = LanzamientosEnAlmacen{}
	_ dominio.Reservas     = ReservasEnAlmacen{}
)

func NuevosIntentos(a Almacen) IntentosEnAlmacen         { return IntentosEnAlmacen{almacen: a} }
func NuevosDespliegues(a Almacen) DesplieguesEnAlmacen   { return DesplieguesEnAlmacen{almacen: a} }
func NuevasOcupaciones(a Almacen) OcupacionesEnAlmacen   { return OcupacionesEnAlmacen{almacen: a} }
func NuevosLanzamientos(a Almacen) LanzamientosEnAlmacen { return LanzamientosEnAlmacen{almacen: a} }
func NuevasReservas(a Almacen) ReservasEnAlmacen         { return ReservasEnAlmacen{almacen: a} }

func (r IntentosEnAlmacen) Intento(ctx context.Context, id dominio.IdIntento) (*dominio.Intento, error) {
	registros, err := leer(ctx, r.almacen, Secuencia{Familia: familiaIntentos, Nombre: string(id)},
		decodificarRegistroDeIntento)
	if err != nil {
		return nil, err
	}
	return dominio.ReconstituirIntento(id, registros)
}

func (r IntentosEnAlmacen) Recorrer(ctx context.Context) ([]*dominio.Intento, error) {
	nombres, err := r.almacen.Nombres(ctx, familiaIntentos)
	if err != nil {
		return nil, err
	}
	intentos := make([]*dominio.Intento, 0, len(nombres))
	for _, nombre := range nombres {
		intento, err := r.Intento(ctx, dominio.IdIntento(nombre))
		if err != nil {
			return nil, err
		}
		intentos = append(intentos, intento)
	}
	return intentos, nil
}

func (r IntentosEnAlmacen) Anadir(ctx context.Context, intento *dominio.Intento) error {
	return anadir(ctx, r.almacen, Secuencia{Familia: familiaIntentos, Nombre: string(intento.Id())},
		intento.Leidos(), intento.Nuevos(), codificarRegistroDeIntento)
}

func (r DesplieguesEnAlmacen) DeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) (*dominio.DesplieguesDeUnAmbiente, error) {
	despliegues, err := leer(ctx, r.almacen, Secuencia{Familia: familiaDespliegues, Nombre: string(ambiente)},
		decodificarDespliegue)
	if err != nil {
		return nil, err
	}
	return dominio.ReconstituirDesplieguesDeUnAmbiente(ambiente, despliegues)
}

func (r DesplieguesEnAlmacen) Recorrer(ctx context.Context) ([]*dominio.DesplieguesDeUnAmbiente, error) {
	nombres, err := r.almacen.Nombres(ctx, familiaDespliegues)
	if err != nil {
		return nil, err
	}
	todos := make([]*dominio.DesplieguesDeUnAmbiente, 0, len(nombres))
	for _, nombre := range nombres {
		despliegues, err := r.DeUnAmbiente(ctx, dominio.Ambiente(nombre))
		if err != nil {
			return nil, err
		}
		todos = append(todos, despliegues)
	}
	return todos, nil
}

func (r DesplieguesEnAlmacen) Anadir(ctx context.Context, despliegues *dominio.DesplieguesDeUnAmbiente) error {
	return anadir(ctx, r.almacen, Secuencia{Familia: familiaDespliegues, Nombre: string(despliegues.Ambiente())},
		despliegues.Leidos(), despliegues.Nuevos(), codificarDespliegue)
}

func (r OcupacionesEnAlmacen) DeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) (*dominio.Ocupacion, error) {
	registros, err := leer(ctx, r.almacen, Secuencia{Familia: familiaOcupaciones, Nombre: string(ambiente)},
		decodificarOcupacion)
	if err != nil {
		return nil, err
	}
	return dominio.ReconstituirOcupacion(ambiente, registros)
}

func (r OcupacionesEnAlmacen) Recorrer(ctx context.Context) ([]*dominio.Ocupacion, error) {
	nombres, err := r.almacen.Nombres(ctx, familiaOcupaciones)
	if err != nil {
		return nil, err
	}
	todas := make([]*dominio.Ocupacion, 0, len(nombres))
	for _, nombre := range nombres {
		ocupacion, err := r.DeUnAmbiente(ctx, dominio.Ambiente(nombre))
		if err != nil {
			return nil, err
		}
		todas = append(todas, ocupacion)
	}
	return todas, nil
}

func (r OcupacionesEnAlmacen) Anadir(ctx context.Context, ocupacion *dominio.Ocupacion) error {
	return anadir(ctx, r.almacen, Secuencia{Familia: familiaOcupaciones, Nombre: string(ocupacion.Ambiente())},
		ocupacion.Leidos(), ocupacion.Nuevos(), codificarOcupacion)
}

func (r LanzamientosEnAlmacen) Todos(ctx context.Context) (*dominio.LanzamientosDelHistorial, error) {
	lanzamientos, err := leer(ctx, r.almacen, Secuencia{Familia: familiaLanzamientos}, decodificarLanzamiento)
	if err != nil {
		return nil, err
	}
	return dominio.ReconstituirLanzamientos(lanzamientos), nil
}

func (r LanzamientosEnAlmacen) Anadir(ctx context.Context, lanzamientos *dominio.LanzamientosDelHistorial) error {
	return anadir(ctx, r.almacen, Secuencia{Familia: familiaLanzamientos},
		lanzamientos.Leidos(), lanzamientos.Nuevos(), codificarLanzamiento)
}

func (r ReservasEnAlmacen) DeUnAmbiente(
	ctx context.Context, ambiente dominio.Ambiente,
) (*dominio.ReservasDeUnAmbiente, error) {
	reservas, err := leer(ctx, r.almacen, Secuencia{Familia: familiaReservas, Nombre: string(ambiente)},
		decodificarReserva)
	if err != nil {
		return nil, err
	}
	return dominio.ReconstituirReservas(ambiente, reservas), nil
}

func (r ReservasEnAlmacen) Anadir(ctx context.Context, reservas *dominio.ReservasDeUnAmbiente) error {
	return anadir(ctx, r.almacen, Secuencia{Familia: familiaReservas, Nombre: string(reservas.Ambiente())},
		reservas.Leidos(), reservas.Nuevas(), codificarReserva)
}

func leer[T any](
	ctx context.Context, almacen Almacen, s Secuencia, decodificarUno func([]byte) (T, error),
) ([]T, error) {
	datos, err := almacen.Leer(ctx, s)
	if err != nil {
		return nil, err
	}
	resultado := make([]T, 0, len(datos))
	for k, d := range datos {
		uno, err := decodificarUno(d)
		if err != nil {
			return nil, fmt.Errorf("secuencia %s, posición %d: %w", s, k+1, err)
		}
		resultado = append(resultado, uno)
	}
	return resultado, nil
}

// anadir escribe los nuevos uno a uno. Si falla a mitad, lo escrito es un prefijo válido: cada registro se
// comprobó contra los anteriores.
func anadir[T any](
	ctx context.Context, almacen Almacen, s Secuencia, leidos int, nuevos []T, codificarUno func(T) ([]byte, error),
) error {
	for k, uno := range nuevos {
		datos, err := codificarUno(uno)
		if err != nil {
			return fmt.Errorf("secuencia %s: %w", s, err)
		}
		if err := almacen.Anadir(ctx, s, leidos+k+1, datos); err != nil {
			return err
		}
	}
	return nil
}
