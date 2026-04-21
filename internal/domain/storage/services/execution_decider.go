package services

import (
	"context"
	"errors"
	"fmt"

	storageDomain "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/ports"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type ExecutionDecider struct {
	repository ports.HistoryRepository
	catalog    StepPolicyCatalog
	clock      ports.Clock
}

func NewExecutionDecider(repository ports.HistoryRepository, catalog StepPolicyCatalog, clock ports.Clock) *ExecutionDecider {
	return &ExecutionDecider{
		repository: repository,
		catalog:    catalog,
		clock:      clock,
	}
}

func (d *ExecutionDecider) Decide(ctx context.Context, key vos.StorageKey, current vos.FingerprintSet) (vos.Decision, error) {
	history, err := d.loadOrNew(ctx, key)
	if err != nil {
		if errors.Is(err, storageDomain.ErrHistoryCorrupted) {
			return vos.DecisionRun("historial corrupto; se re-ejecuta el paso de forma segura"), nil
		}
		return vos.Decision{}, fmt.Errorf("execution decider: cargar historial: %w", err)
	}

	policy, err := d.catalog.Lookup(key.Step())
	if err != nil {
		return vos.Decision{}, fmt.Errorf("execution decider: obtener política del paso: %w", err)
	}

	return history.Decide(policy.Rule(), current, d.clock.Now()), nil
}

func (d *ExecutionDecider) RecordSuccess(ctx context.Context, key vos.StorageKey, current vos.FingerprintSet) error {
	history, err := d.loadOrNew(ctx, key)
	if err != nil {
		if errors.Is(err, storageDomain.ErrHistoryCorrupted) {
			// Si el historial estaba corrupto, empezamos uno nuevo.
			history = aggregates.NewExecutionHistory(key)
		} else {
			return fmt.Errorf("execution decider: cargar historial para persistir: %w", err)
		}
	}

	history.Append(current, d.clock.Now())

	if err := d.repository.Save(ctx, history); err != nil {
		return fmt.Errorf("execution decider: guardar historial: %w", err)
	}
	return nil
}

func (d *ExecutionDecider) loadOrNew(ctx context.Context, key vos.StorageKey) (*aggregates.ExecutionHistory, error) {
	history, err := d.repository.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if history == nil {
		return aggregates.NewExecutionHistory(key), nil
	}
	return history, nil
}
