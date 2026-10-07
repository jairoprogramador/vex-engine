package dominio

import (
	"context"
	"time"
)

// Historial es lo que la aplicación necesita del Historial, en el lenguaje de este dominio: nunca intentos,
// nunca registros de paso — solo lo que decide si se lanza, y lo que registra cuando se lanza.
type Historial interface {
	// Reservado dice si el dueño del negocio se reservó este ambiente ahora mismo.
	Reservado(ctx context.Context, ambiente Ambiente) (bool, error)
	// UltimoLanzamientoDeUnAmbiente es el despliegue del último lanzamiento de un ambiente, si tiene alguno.
	UltimoLanzamientoDeUnAmbiente(ctx context.Context, ambiente Ambiente) (IdDespliegue, bool, error)
	// HashDelCodigoDeUnDespliegue es el hash del código con el que se hizo un despliegue.
	HashDelCodigoDeUnDespliegue(ctx context.Context, despliegue IdDespliegue) (HashDelCodigo, error)
	// VersionesConocidas son el hash y la versión de cada código que ya se lanzó antes, de cualquier
	// ambiente — lo que DecidirVersion necesita recorrer (IT-10 DEC-10.8).
	VersionesConocidas(ctx context.Context) ([]VersionConocida, error)
	// LanzamientosDeUnAmbiente son los de un ambiente, en el orden en que se registraron.
	LanzamientosDeUnAmbiente(ctx context.Context, ambiente Ambiente) ([]LanzamientoRegistrado, error)
	// RegistrarLanzamiento escribe el lanzamiento en el ambiente dado.
	RegistrarLanzamiento(ctx context.Context, ambiente Ambiente, lanzamiento Lanzamiento) (LanzamientoRegistrado, error)
	// RegistrarReserva registra la reserva o la liberación de un ambiente.
	RegistrarReserva(ctx context.Context, ambiente Ambiente, reservado bool) error
}

// LanzamientoRegistrado es lo que decide el Historial al registrar un lanzamiento: su identidad propia y su
// instante — lo único que Lanzamiento no decide por sí mismo.
type LanzamientoRegistrado struct {
	Id         string
	Ambiente   Ambiente
	Despliegue IdDespliegue
	Version    Version
	Nombre     Nombre
	Instante   time.Time
}
