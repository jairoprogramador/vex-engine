package pipeline

import "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"

// ContentFingerprint calcula la huella de contenido de un árbol que vive en una
// ruta del sistema de archivos.
//
// Se llama «de contenido» y no «de proyecto» a propósito: la misma regla se
// aplica al repositorio de pipeline (spec 18). Un nombre que dijera «proyecto»
// invitaría a escribir una segunda huella para el pipeline, con otra regla.
//
// La regla en sí es de dominio y no está aquí: vive en el paquete fingerprint,
// sobre el puerto TreeSource. Este puerto existe sólo porque resolver una ruta
// del sistema de archivos es infraestructura.
//
// FromFile no existe: no tenía llamadores y devolvía ("", nil) ante un archivo
// inexistente — una huella vacía indistinguible de un error, que es la clase de
// primitiva que la spec 08 existe para erradicar.
type ContentFingerprint interface {
	FromDirectory(dirPath string) (fingerprint.Fingerprint, error)
}
