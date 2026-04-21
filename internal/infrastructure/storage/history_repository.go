package storage

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	storag "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	stoAgg "github.com/jairoprogramador/vex-engine/internal/domain/storage/aggregates"
	stoPor "github.com/jairoprogramador/vex-engine/internal/domain/storage/ports"
	stoVos "github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/storage/dto"
)

type HistoryRepository struct {
	resolver PathResolver
}

func NewHistoryRepository(resolver PathResolver) stoPor.HistoryRepository {
	return &HistoryRepository{resolver: resolver}
}

func (r *HistoryRepository) FindByKey(_ context.Context, key stoVos.StorageKey) (*stoAgg.ExecutionHistory, error) {
	filePath := r.resolver.Resolve(key)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("gob repository: abrir historial: %w", err)
	}
	defer file.Close()

	var historyDTO dto.HistoryDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&historyDTO); err != nil {
		if errors.Is(err, io.EOF) {
			// Archivo vacío → sin historial, no es error fatal.
			return nil, nil
		}
		// Cualquier otro error de decode = archivo corrupto.
		return nil, fmt.Errorf("%w: decodificación fallida: %v", storag.ErrHistoryCorrupted, err)
	}

	history, err := dto.FromHistoryDTO(&historyDTO, key)
	if err != nil {
		return nil, err
	}
	return history, nil
}

func (r *HistoryRepository) Save(_ context.Context, history *stoAgg.ExecutionHistory) error {
	if history == nil {
		return errors.New("gob repository: no se puede guardar un historial nil")
	}

	filePath := r.resolver.Resolve(history.Key())

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("gob repository: crear directorio %s: %w", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("gob repository: crear archivo: %w", err)
	}
	defer file.Close()

	dto := dto.ToHistoryDTO(history)
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(dto); err != nil {
		return fmt.Errorf("gob repository: codificar historial: %w", err)
	}

	return nil
}

var _ stoPor.HistoryRepository = (*HistoryRepository)(nil)
