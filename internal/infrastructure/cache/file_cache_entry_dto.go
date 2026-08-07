package cache

import (
	"fmt"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// fileCacheEntrySchemaVersion versiona la FORMA del archivo, no la regla de la
// clave. Son dos cosas distintas y conviene no fundirlas: `ck-v1` dice cómo se
// compuso la clave —cambiarlo invalida el índice entero—, mientras que esto dice
// cómo está serializada la entrada.
//
// Sube a 2 con la spec 11: la entrada dejó de ser «presencia + caducidad +
// procedencia» y pasó a ser un puntero a un registro. Una entrada v1 no se puede
// interpretar como v2 —no apunta a nada—, y como el índice es derivable y no
// participa en ninguna decisión, la respuesta correcta a encontrarse una es
// ignorarla, no migrarla.
const fileCacheEntrySchemaVersion = 2

// FileCacheEntryDTO es la forma en disco de una entrada del índice.
//
// Lleva la clave DENTRO además de en la ruta, aunque sea redundante: la ruta es
// un hash, así que un archivo que no dice de qué clave es no responde nada.
type FileCacheEntryDTO struct {
	SchemaVersion int             `json:"schema_version"`
	CacheKey      string          `json:"cache_key"`
	StateKey      FileStateKeyDTO `json:"state_key"`
	RecordID      string          `json:"record_id"`
}

// FileStateKeyDTO guarda los tres componentes de la clave de estado POR
// SEPARADO, y no una cadena compuesta: así no hay que inventar un escape del
// separador para una url o un nombre de ambiente que lo contenga.
type FileStateKeyDTO struct {
	Subject string `json:"subject"`
	Scope   string `json:"scope"`
	StepID  string `json:"step_id"`
}

func ToFileCacheEntryDTO(key domCache.CacheKey, entry domCache.Entry) FileCacheEntryDTO {
	return FileCacheEntryDTO{
		SchemaVersion: fileCacheEntrySchemaVersion,
		CacheKey:      key.String(),
		StateKey: FileStateKeyDTO{
			Subject: entry.StateKey.Subject(),
			Scope:   entry.StateKey.Scope().String(),
			StepID:  entry.StateKey.StepID(),
		},
		RecordID: entry.RecordID.String(),
	}
}

func (dto FileCacheEntryDTO) ToDomain() (domCache.Entry, error) {
	if dto.SchemaVersion != fileCacheEntrySchemaVersion {
		return domCache.Entry{}, fmt.Errorf(
			"esquema de entrada %d no soportado (este binario lee el %d)",
			dto.SchemaVersion, fileCacheEntrySchemaVersion)
	}

	scope, err := domState.ParseScope(dto.StateKey.Scope)
	if err != nil {
		return domCache.Entry{}, err
	}
	stateKey, err := domState.NewKey(dto.StateKey.Subject, scope, dto.StateKey.StepID)
	if err != nil {
		return domCache.Entry{}, err
	}
	recordID, err := domState.ParseRecordID(dto.RecordID)
	if err != nil {
		return domCache.Entry{}, err
	}

	return domCache.NewEntry(stateKey, recordID)
}
