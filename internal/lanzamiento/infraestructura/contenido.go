package infraestructura

import (
	"encoding/json"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

// contextoLanzamiento identifica, ante Historial, el contenido de un lanzamiento (IT-04 DEC-04.7: Historial
// nunca lo interpreta, solo lo guarda). Lleva la versión, el nombre y el hash del código con el que se
// lanzó — este último para no tener que volver a consultar el Historial por cada lanzamiento pasado al
// decidir la versión de uno nuevo (IT-10 DEC-10.8).
const contextoLanzamiento = "lanzamiento/lanzamiento-v1"

type contenidoLanzamiento struct {
	Version int    `json:"version"`
	Nombre  string `json:"nombre"`
	Hash    string `json:"hash"`
}

func codificarContenido(l dominio.Lanzamiento) (historialpublicado.Contenido, error) {
	datos, err := json.Marshal(contenidoLanzamiento{
		Version: l.Version().Numero(), Nombre: l.Nombre().String(), Hash: l.HashDelCodigo().String(),
	})
	if err != nil {
		return historialpublicado.Contenido{}, fmt.Errorf("lanzamiento: codificar el contenido: %w", err)
	}
	return historialpublicado.Contenido{Contexto: contextoLanzamiento, Datos: datos}, nil
}

// decodificarVersionConocida decodifica solo lo que DecidirVersion necesita: el hash y la versión.
func decodificarVersionConocida(c historialpublicado.Contenido) (dominio.VersionConocida, error) {
	if c.Contexto != contextoLanzamiento {
		return dominio.VersionConocida{},
			fmt.Errorf("lanzamiento: contenido de contexto %q, se esperaba %q", c.Contexto, contextoLanzamiento)
	}
	var cl contenidoLanzamiento
	if err := json.Unmarshal(c.Datos, &cl); err != nil {
		return dominio.VersionConocida{}, fmt.Errorf("lanzamiento: decodificar el contenido: %w", err)
	}
	hash, err := dominio.NuevoHashDelCodigo(cl.Hash)
	if err != nil {
		return dominio.VersionConocida{}, err
	}
	version, err := dominio.NuevaVersion(cl.Version)
	if err != nil {
		return dominio.VersionConocida{}, err
	}
	return dominio.VersionConocida{Hash: hash, Version: version}, nil
}
