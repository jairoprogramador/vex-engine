// Package cache es el ÍNDICE en disco de contenido → registro de estado.
//
// Hereda el layout que la spec 10 construyó para las entradas de caché; lo que
// cambia con la spec 11 es qué contiene y para qué sirve. No participa en
// ninguna decisión del motor: borrar este directorio entero no cambia una sola.
// La verdad vive en `infrastructure/state`, y esto se reconstruye recorriéndola.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
)

// shardLen son los caracteres del hash que dan nombre al directorio intermedio.
//
// Sin él, un proyecto con muchos pasos y muchos ambientes acabaría con miles de
// archivos en un solo directorio. Con dos caracteres hexadecimales hay 256
// cajones, que es de sobra para lo que un caché local acumula.
const shardLen = 2

// entryFileExt es `.json` y no `.gob` a propósito: esto es lo que un humano mira
// cuando quiere saber si un contenido ya corrió alguna vez. Un gob no es
// inspeccionable ni portable, y aquí la portabilidad importa porque desde la
// spec 16 estas entradas se comparten entre máquinas.
const entryFileExt = ".json"

var _ domCache.Entries = (*FileEntriesRepository)(nil)

// FileEntriesRepository guarda una entrada por clave, direccionada por
// contenido: `<base>/<versión>/<2 primeros del hash>/<resto>.json`.
//
// La ruta NO lleva ni el proyecto, ni el pipeline, ni el ambiente, ni el paso:
// los cuatro están DENTRO del hash. Esa es la propiedad que lo hace útil como
// índice —«¿este contenido exacto corrió ALGUNA vez?» se responde con un
// `stat`— y es también la que lo inhabilita para decidir, porque la decisión
// pregunta por una POSICIÓN y no por un contenido.
type FileEntriesRepository struct {
	basePath string
	writer   persistence.AtomicFileWriter
}

func NewFileEntriesRepository(basePath string) domCache.Entries {
	return &FileEntriesRepository{
		basePath: basePath,
		writer:   persistence.NewAtomicFileWriter(),
	}
}

func (r *FileEntriesRepository) filePath(key domCache.CacheKey) string {
	hash := key.Hash()
	return filepath.Join(r.basePath, key.Version(), hash[:shardLen], hash[shardLen:]+entryFileExt)
}

// Get NO crea la entrada que consulta. Es la idempotencia de la consulta, y es
// una red de regresión, no una comodidad: si consultar escribiera, un proceso
// muerto entre la consulta y el final del paso dejaría grabado «ya se hizo»
// para un paso que nunca terminó (spec 09 §9.10).
func (r *FileEntriesRepository) Get(_ *context.Context, key domCache.CacheKey) (domCache.Entry, bool, error) {
	if key.IsZero() {
		return domCache.Entry{}, false, errors.New("file entries repository: clave vacía")
	}

	path := r.filePath(key)
	file, err := os.Open(path)
	if err != nil {
		// Ausencia NO es error: es la respuesta «no consta», y aguas arriba
		// significa ejecutar.
		if errors.Is(err, os.ErrNotExist) {
			return domCache.Entry{}, false, nil
		}
		return domCache.Entry{}, false, fmt.Errorf("file entries repository: abrir %s: %w", path, err)
	}
	defer file.Close()

	// ILEGIBLE ⇒ AUSENTE, y es una decisión, no una omisión (spec 11 §5.6).
	//
	// Hasta la spec 10 esto era un error, y con razón: la entrada ERA la
	// respuesta a «¿hay que re-ejecutar?», así que confundir «rota» con «no
	// consta» habría sido fail-open silencioso sobre una decisión real. Desde la
	// 11 no decide nada y es derivable, así que una entrada rota es exactamente
	// una entrada que todavía no se ha reconstruido. El almacén de registros
	// sostiene la asimetría contraria: allí ilegible es error.
	var dto FileCacheEntryDTO
	if err := json.NewDecoder(file).Decode(&dto); err != nil {
		return domCache.Entry{}, false, nil
	}

	entry, err := dto.ToDomain()
	if err != nil {
		return domCache.Entry{}, false, nil
	}
	return entry, true, nil
}

// Put publica la entrada con el escritor atómico de la spec 02: un lector nunca
// ve un archivo a medias.
func (r *FileEntriesRepository) Put(_ *context.Context, key domCache.CacheKey, entry domCache.Entry) error {
	if key.IsZero() {
		return errors.New("file entries repository: clave vacía")
	}

	path := r.filePath(key)
	dto := ToFileCacheEntryDTO(key, entry)

	err := r.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("file entries repository: escribir %s: %w", path, err)
	}
	return nil
}
