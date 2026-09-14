package deployment

import "fmt"

// Format es la versión del formato del pipelinecode con la que se leyó el
// material, y si alguien la DECLARÓ.
//
// Es el dato con el que la spec 18 §5.5 marca un objeto como INCOMPLETO en vez
// de emitirlo como si fuera completo: un pipelinecode sin `vexpipeline.yaml` no
// declara orígenes, así que una variable producida por otro step aparece en su
// material como si no viniera de ninguna parte. «Este material está incompleto»
// es un hecho; omitir el objeto sería perder la historia, y emitirlo como
// completo sería mentir.
//
// # Por qué no se importa `pipeline.Manifest`
//
// Sería el tipo exacto, y no se puede: la spec 18 mete el resolutor del objeto
// DENTRO de la cadena de pipeline, así que `pipeline` va a importar `deployment`
// y el import de vuelta sería un ciclo. Lo que viaja es el dato —la versión y si
// se declaró—, que es lo que `Manifest.SchemaVersion()` e `IsDeclared()` ya
// responden.
//
// # Y por qué entra en la identidad
//
// Porque si no entrara, un pipelinecode que sube de la v1 a la v2 sin tocar
// ninguna otra línea produciría el MISMO `content_id` con distinta marca de
// completitud — dos objetos con la misma identidad y afirmaciones distintas
// sobre ella, que es la única cosa que un registro direccionado por contenido no
// puede permitirse.
type Format struct {
	schemaVersion int
	declared      bool
}

// NewFormat construye la versión del formato tal como se leyó.
//
// No valida contra el vocabulario de versiones conocidas: eso es de
// `pipeline.NewManifest`, que es quien lee el archivo y quien tiene que decir
// «este motor no entiende esa versión». Aquí una versión desconocida no puede
// llegar, y si llegara, el objeto debe poder representarla — un registro que no
// sabe describir lo que pasó no sirve de registro.
func NewFormat(schemaVersion int, declared bool) (Format, error) {
	if schemaVersion < 1 {
		return Format{}, fmt.Errorf(
			"deployment: 'schema_version: %d' no es una versión de formato", schemaVersion)
	}
	return Format{schemaVersion: schemaVersion, declared: declared}, nil
}

// SchemaVersion es la versión vigente del formato del pipelinecode.
func (f Format) SchemaVersion() int { return f.schemaVersion }

// IsDeclared dice si el pipelinecode trae `vexpipeline.yaml`.
func (f Format) IsDeclared() bool { return f.declared }

// IsZero indica que no hay formato.
func (f Format) IsZero() bool { return f.schemaVersion == 0 }

// Equals es la regla de igualdad del value object.
func (f Format) Equals(other Format) bool {
	return f.schemaVersion == other.schemaVersion && f.declared == other.declared
}
