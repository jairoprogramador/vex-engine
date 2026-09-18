package publicado

// Material es lo que Suministro pone delante.
type Material struct {
	// Directorio es solo de quien lo pidió, y para leer: no cambia aunque la fuente cambie, hasta que se retira.
	Directorio string
	// Hash cambia si cambia el contenido. Es el hash del código o el del pipeline, según la fuente.
	Hash string
	// Commit es con el que se trabaja. Está vacío si el material es una copia de trabajo.
	Commit string
}
