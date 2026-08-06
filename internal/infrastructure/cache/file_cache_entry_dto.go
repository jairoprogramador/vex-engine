package cache

import (
	"fmt"
	"time"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
)

// fileCacheEntrySchemaVersion versiona la FORMA del archivo, no la regla de la
// clave. Son dos cosas distintas y conviene no fundirlas: `ck-v1` dice cómo se
// compuso la clave —cambiarlo invalida el caché entero—, mientras que esto dice
// cómo está serializada la entrada, y podría subir sin que ninguna clave cambie.
const fileCacheEntrySchemaVersion = 1

// entryTimeLayout es RFC3339Nano porque va y vuelve sin perder precisión: el
// instante que se lee es exactamente el que se escribió.
const entryTimeLayout = time.RFC3339Nano

// FileCacheEntryDTO es la forma en disco de una entrada.
//
// Lleva la clave DENTRO además de en la ruta, aunque sea redundante: un archivo
// de caché es lo que alguien abre cuando el motor se saltó un paso y no entiende
// por qué, y un archivo que no dice de qué es no responde nada.
type FileCacheEntryDTO struct {
	SchemaVersion int               `json:"schema_version"`
	CacheKey      string            `json:"cache_key"`
	ExpiresAt     string            `json:"expires_at,omitempty"`
	ProducedBy    FileProvenanceDTO `json:"produced_by"`
}

type FileProvenanceDTO struct {
	ExecutionID string `json:"execution_id"`
	At          string `json:"at"`
}

func ToFileCacheEntryDTO(key domCache.CacheKey, entry domCache.Entry) FileCacheEntryDTO {
	dto := FileCacheEntryDTO{
		SchemaVersion: fileCacheEntrySchemaVersion,
		CacheKey:      key.String(),
		ProducedBy: FileProvenanceDTO{
			ExecutionID: entry.ProducedBy.ExecutionID,
			At:          entry.ProducedBy.At.UTC().Format(entryTimeLayout),
		},
	}
	if entry.ExpiresAt != nil {
		dto.ExpiresAt = entry.ExpiresAt.UTC().Format(entryTimeLayout)
	}
	return dto
}

func (dto FileCacheEntryDTO) ToDomain() (domCache.Entry, error) {
	if dto.SchemaVersion != fileCacheEntrySchemaVersion {
		return domCache.Entry{}, fmt.Errorf(
			"esquema de entrada %d no soportado (este binario lee el %d)",
			dto.SchemaVersion, fileCacheEntrySchemaVersion)
	}

	producedAt, err := time.Parse(entryTimeLayout, dto.ProducedBy.At)
	if err != nil {
		return domCache.Entry{}, fmt.Errorf("interpretar produced_by.at %q: %w", dto.ProducedBy.At, err)
	}

	entry := domCache.Entry{
		ProducedBy: domCache.Provenance{
			ExecutionID: dto.ProducedBy.ExecutionID,
			At:          producedAt,
		},
	}

	if dto.ExpiresAt != "" {
		expiresAt, err := time.Parse(entryTimeLayout, dto.ExpiresAt)
		if err != nil {
			return domCache.Entry{}, fmt.Errorf("interpretar expires_at %q: %w", dto.ExpiresAt, err)
		}
		entry.ExpiresAt = &expiresAt
	}

	return entry, nil
}
