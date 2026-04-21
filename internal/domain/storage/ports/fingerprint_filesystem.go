package ports

import "github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"

type FingerprintFilesystem interface {
	FromFile(filePath string) (vos.Fingerprint, error)
	FromDirectory(dirPath string) (vos.Fingerprint, error)
}
