package publicado

import "context"

// Lo que Suministro de Fuentes publica, por cliente (modelo/contextos/suministro.md, «Lo que le piden»). Cada
// vez que se trae una fuente se pone delante un material propio, que quien lo pidió retira cuando termina.
//
// Una fuente es, por ahora, el directorio de un repositorio local (RD-11 compra el acceso a los remotos). Un
// commit es su identificador completo. Lo que no está devuelve ErrNoExiste, y lo que no se puede pedir,
// ErrInvalido.

// ParaEjecucion es lo que usa Ejecución de Pipeline: el material de hoy, de un commit o de una copia de trabajo
// (IT-10 DEC-10.7), con el hash del código y el commit con el que se trabaja.
type ParaEjecucion interface {
	TraerDeHoy(ctx context.Context, fuente string) (Material, error)
	TraerDeUnCommit(ctx context.Context, fuente, commit string) (Material, error)
	TraerCopiaDeTrabajo(ctx context.Context, directorio string) (Material, error)
	Retirar(ctx context.Context, material Material) error
}

// ParaDefinicion es lo que usa Definición de Pipeline: el pipeline como fuente, el de hoy o el de un commit
// (IT-03 DEC-03.9).
type ParaDefinicion interface {
	TraerDeHoy(ctx context.Context, fuente string) (Material, error)
	TraerDeUnCommit(ctx context.Context, fuente, commit string) (Material, error)
	Retirar(ctx context.Context, material Material) error
}

// ParaSimulacion es lo que usa Simulación de Pipeline: el pipeline que se quiere simular, de una copia de
// trabajo o de un commit (IT-10 DEC-10.6).
type ParaSimulacion interface {
	TraerDeUnCommit(ctx context.Context, fuente, commit string) (Material, error)
	TraerCopiaDeTrabajo(ctx context.Context, directorio string) (Material, error)
	Retirar(ctx context.Context, material Material) error
}
