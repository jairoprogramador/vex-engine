package dto

import (
	"fmt"
	"time"

	storageDomain "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// historyDTO es la estructura serializable con gob del historial completo.
// Version permite evolucionar el formato sin romper compatibilidad.
type HistoryDTO struct {
	Version int
	Entries []HistoryEntryDTO
}

// historyEntryDTO representa una entrada con fingerprints como map[string]string
// donde la clave es FingerprintKind.String() (ej. "code", "instruction", "vars").
// Usar map en lugar de campos fijos permite omitir fingerprints que un paso no necesita.
type HistoryEntryDTO struct {
	Fingerprints map[string]string
	Environment  string
	CreatedAt    time.Time
}

func ToHistoryDTO(history *aggregates.ExecutionHistory) *HistoryDTO {
	dtoEntries := make([]HistoryEntryDTO, 0, len(history.Entries()))
	for _, entry := range history.Entries() {
		fps := make(map[string]string)
		for _, kind := range []vos.FingerprintKind{vos.KindCode, vos.KindInstruction, vos.KindVars} {
			if fp, ok := entry.FindFingerprintByKind(kind); ok {
				fps[kind.String()] = fp.String()
			}
		}
		dtoEntries = append(dtoEntries, HistoryEntryDTO{
			Fingerprints: fps,
			Environment:  entry.Environment().String(),
			CreatedAt:    entry.CreatedAt(),
		})
	}
	return &HistoryDTO{Version: 1, Entries: dtoEntries}
}

func FromHistoryDTO(dto *HistoryDTO, key vos.StorageKey) (*aggregates.ExecutionHistory, error) {
	if dto == nil {
		return aggregates.NewExecutionHistory(key), nil
	}

	entries := make([]aggregates.HistoryEntry, 0, len(dto.Entries))
	for i, e := range dto.Entries {
		entry, err := buildEntry(e)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, storageDomain.ErrHistoryCorrupted)
		}
		entries = append(entries, entry)
	}

	history, err := aggregates.LoadStepHistory(key, entries)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", storageDomain.ErrHistoryCorrupted, err)
	}
	return history, nil
}

var kindByString = map[string]vos.FingerprintKind{
	vos.KindCodeString:        vos.KindCode,
	vos.KindInstructionString: vos.KindInstruction,
	vos.KindVarsString:        vos.KindVars,
}

func buildEntry(entryHistoryDTO HistoryEntryDTO) (aggregates.HistoryEntry, error) {
	env, err := vos.NewEnvironment(entryHistoryDTO.Environment)
	if err != nil {
		return aggregates.HistoryEntry{}, fmt.Errorf("environment inválido %q: %w", entryHistoryDTO.Environment, err)
	}

	fps := make(map[vos.FingerprintKind]vos.Fingerprint)
	for kindStr, fpStr := range entryHistoryDTO.Fingerprints {
		kind, ok := kindByString[kindStr]
		if !ok {
			return aggregates.HistoryEntry{}, fmt.Errorf("kind desconocido en DTO: %q", kindStr)
		}
		fp, err := vos.NewFingerprint(fpStr)
		if err != nil {
			return aggregates.HistoryEntry{}, fmt.Errorf("fingerprint inválido para kind %q: %w", kindStr, err)
		}
		fps[kind] = fp
	}

	set := vos.NewFingerprintSet(fps, env)
	return aggregates.NewHistoryEntry(set, entryHistoryDTO.CreatedAt), nil
}
