package ports

import "context"

// ClonerTemplate define el contrato para garantizar que un repositorio git
// esté disponible en una ruta local. Se limita a una única responsabilidad:
// clonar si no existe, no-op si ya existe.
type ClonerTemplate interface {
	EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
}
