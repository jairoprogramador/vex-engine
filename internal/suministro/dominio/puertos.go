package dominio

import "context"

// Repositorios pone delante el material de una fuente, y es lo que se compra (IT-10 DEC-10.2). Cada material va
// a un directorio propio, que no cambia hasta que se retira aunque la fuente cambie mientras tanto. Si la
// fuente, la copia de trabajo o el commit no están, devuelve ErrNoExiste.
type Repositorios interface {
	// PonerDeHoy pone delante la fuente como está hoy, y dice con qué commit.
	PonerDeHoy(ctx context.Context, fuente Fuente) (directorio string, commit Commit, err error)
	PonerDeUnCommit(ctx context.Context, fuente Fuente, commit Commit) (directorio string, err error)
	PonerCopiaDeTrabajo(ctx context.Context, copia CopiaDeTrabajo) (directorio string, err error)
	// Retirar borra un directorio que puso delante, y se niega a borrar cualquier otro.
	Retirar(ctx context.Context, directorio string) error
}

// Hashes dice el hash de lo que hay en un directorio. Qué entra es técnica (IT-02 DEC-02.18): la única regla es
// que cambie si cambia el contenido. Va aparte de Repositorios para que el hash no dependa de qué acceso puso
// delante el material, ni de si vino de hoy, de un commit o de una copia de trabajo.
type Hashes interface {
	DeUnDirectorio(ctx context.Context, directorio string) (Hash, error)
}
