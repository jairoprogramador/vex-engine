package infraestructura

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

// AlmacenEnMemoria cumple las mismas garantías que el local dentro de un proceso. Es para las pruebas.
type AlmacenEnMemoria struct {
	mu         sync.Mutex
	secuencias map[Secuencia][][]byte
}

var _ Almacen = (*AlmacenEnMemoria)(nil)

func NuevoAlmacenEnMemoria() *AlmacenEnMemoria {
	return &AlmacenEnMemoria{secuencias: map[Secuencia][][]byte{}}
}

func (a *AlmacenEnMemoria) Leer(_ context.Context, s Secuencia) ([][]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	registros := make([][]byte, 0, len(a.secuencias[s]))
	for _, r := range a.secuencias[s] {
		registros = append(registros, slices.Clone(r))
	}
	return registros, nil
}

func (a *AlmacenEnMemoria) Anadir(_ context.Context, s Secuencia, posicion int, registro []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	actuales := a.secuencias[s]
	switch {
	case posicion < 1:
		return fmt.Errorf("almacén en memoria: secuencia %s: la posición empieza en 1, no en %d", s, posicion)
	case posicion <= len(actuales):
		return fmt.Errorf("almacén en memoria: secuencia %s, posición %d: %w", s, posicion, dominio.ErrConflicto)
	case posicion > len(actuales)+1:
		return fmt.Errorf("almacén en memoria: secuencia %s: no se añade la posición %d sin la anterior", s, posicion)
	}
	a.secuencias[s] = append(actuales, slices.Clone(registro))
	return nil
}

func (a *AlmacenEnMemoria) Nombres(_ context.Context, familia string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var nombres []string
	for s := range a.secuencias {
		if s.Familia == familia && s.Nombre != "" {
			nombres = append(nombres, s.Nombre)
		}
	}
	sort.Strings(nombres)
	return nombres, nil
}
