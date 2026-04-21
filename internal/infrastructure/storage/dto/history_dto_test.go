package dto

import (
	"errors"
	"testing"
	"time"

	storageDomain "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func testKey() vos.StorageKey {
	return vos.NewStorageKey("proj", "tmpl", vos.StepTest)
}

func TestFromDTO_NilDTO_ReturnsEmptyHistory(t *testing.T) {
	h, err := FromHistoryDTO(nil, testKey())
	if err != nil {
		t.Fatalf("nil DTO no debe retornar error: %v", err)
	}
	if h == nil {
		t.Fatal("debe retornar historial vacío, no nil")
	}
	if len(h.Entries()) != 0 {
		t.Errorf("se esperaban 0 entradas, obtuvo %d", len(h.Entries()))
	}
}

func TestFromDTO_CorruptedEnvironment_ReturnsErrHistoryCorrupted(t *testing.T) {
	dto := &HistoryDTO{
		Version: 1,
		Entries: []HistoryEntryDTO{
			{
				Fingerprints: map[string]string{"code": "abc123"},
				Environment:  "", // inválido
				CreatedAt:    time.Now(),
			},
		},
	}

	_, err := FromHistoryDTO(dto, testKey())
	if err == nil {
		t.Fatal("se esperaba error para environment vacío")
	}
	if !errors.Is(err, storageDomain.ErrHistoryCorrupted) {
		t.Errorf("se esperaba ErrHistoryCorrupted, obtuvo: %v", err)
	}
}

func TestFromDTO_EmptyFingerprint_ReturnsErrHistoryCorrupted(t *testing.T) {
	dto := &HistoryDTO{
		Version: 1,
		Entries: []HistoryEntryDTO{
			{
				Fingerprints: map[string]string{"code": ""}, // fingerprint vacío
				Environment:  "sand",
				CreatedAt:    time.Now(),
			},
		},
	}

	_, err := FromHistoryDTO(dto, testKey())
	if err == nil {
		t.Fatal("se esperaba error para fingerprint vacío")
	}
	if !errors.Is(err, storageDomain.ErrHistoryCorrupted) {
		t.Errorf("se esperaba ErrHistoryCorrupted, obtuvo: %v", err)
	}
}

func TestFromDTO_UnknownKind_ReturnsErrHistoryCorrupted(t *testing.T) {
	dto := &HistoryDTO{
		Version: 1,
		Entries: []HistoryEntryDTO{
			{
				Fingerprints: map[string]string{"unknown_kind": "abc123"},
				Environment:  "sand",
				CreatedAt:    time.Now(),
			},
		},
	}

	_, err := FromHistoryDTO(dto, testKey())
	if err == nil {
		t.Fatal("se esperaba error para kind desconocido")
	}
	if !errors.Is(err, storageDomain.ErrHistoryCorrupted) {
		t.Errorf("se esperaba ErrHistoryCorrupted, obtuvo: %v", err)
	}
}

func TestFromDTO_ValidEntry_RoundTrip(t *testing.T) {
	dto := &HistoryDTO{
		Version: 1,
		Entries: []HistoryEntryDTO{
			{
				Fingerprints: map[string]string{
					"code":        "abc123",
					"instruction": "def456",
					"vars":        "ghi789",
				},
				Environment: "prod",
				CreatedAt:   time.Now().UTC().Truncate(0),
			},
		},
	}

	h, err := FromHistoryDTO(dto, testKey())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(h.Entries()) != 1 {
		t.Fatalf("se esperaba 1 entrada, obtuvo %d", len(h.Entries()))
	}

	entry := h.Entries()[0]
	fp, ok := entry.FindFingerprintByKind(vos.KindCode)
	if !ok || fp.String() != "abc123" {
		t.Errorf("fingerprint code incorrecto: %v", fp)
	}
	if entry.Environment().String() != "prod" {
		t.Errorf("environment incorrecto: %s", entry.Environment().String())
	}
}
