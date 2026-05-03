package pipeline

type ProjectFingerprint interface {
	FromFile(filePath string) (string, error)
	FromDirectory(dirPath string) (string, error)
}
