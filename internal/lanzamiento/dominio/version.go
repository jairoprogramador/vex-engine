package dominio

// VersionConocida es lo que ya se sabe de un código lanzado antes: su hash y la versión que se le dio.
type VersionConocida struct {
	Hash    HashDelCodigo
	Version Version
}

// DecidirVersion calcula la versión de un código, recorriendo las que ya se conocen: si el hash ya tiene
// una asignada, la reutiliza — el mismo código conserva su versión en todos los ambientes — y si no, es la
// siguiente después de la mayor conocida (IT-10 DEC-10.8). Sin conocidas, empieza en uno.
func DecidirVersion(hash HashDelCodigo, conocidas []VersionConocida) Version {
	siguiente := 1
	for _, c := range conocidas {
		if c.Hash == hash {
			return c.Version
		}
		if c.Version.Numero() >= siguiente {
			siguiente = c.Version.Numero() + 1
		}
	}
	version, _ := NuevaVersion(siguiente)
	return version
}
