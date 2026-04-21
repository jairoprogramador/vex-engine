package vos

// FingerprintSet agrupa los fingerprints conocidos de un paso junto con el entorno.
// Las ausencias son legítimas: un paso puede omitir KindCode si no lo necesita.
type FingerprintSet struct {
	fingerprints map[FingerprintKind]Fingerprint
	environment  Environment
}

func NewFingerprintSet(fps map[FingerprintKind]Fingerprint, env Environment) FingerprintSet {
	copied := make(map[FingerprintKind]Fingerprint, len(fps))
	for k, v := range fps {
		copied[k] = v
	}
	return FingerprintSet{
		fingerprints: copied,
		environment:  env,
	}
}

// Get retorna el fingerprint para el kind dado. El segundo retorno indica si existe.
func (fs FingerprintSet) Get(kind FingerprintKind) (Fingerprint, bool) {
	fp, ok := fs.fingerprints[kind]
	return fp, ok
}

func (fs FingerprintSet) Environment() Environment {
	return fs.environment
}
