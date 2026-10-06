package infraestructura

import (
	"encoding/json"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Los dos contenidos que Ejecución escribe en el Historial: el de la apertura de un intento (las dos fuentes
// con las que se abrió, para que un rollback futuro pueda reconstruirlas — DEC-03.9) y el de un registro de
// paso (sus recursos: el hash del código, el de sus instrucciones — DEC-08.7 — y el de sus variables). El Historial no interpreta
// ninguno de los dos: nunca lee su Contexto ni sus Datos (docs/modelo/contextos/historial.md).

const contextoApertura = "ejecucion/apertura-v1"

type contenidoApertura struct {
	FuenteDelProyecto string   `json:"fuente_del_proyecto"`
	CommitDelProyecto string   `json:"commit_del_proyecto"`
	FuenteDelPipeline string   `json:"fuente_del_pipeline"`
	CommitDelPipeline string   `json:"commit_del_pipeline"`
	OrdenDeAmbientes  []string `json:"orden_de_ambientes,omitempty"`
}

func codificarContenidoDeApertura(a dominio.AperturaDeIntento) (historialpublicado.Contenido, error) {
	datos, err := json.Marshal(contenidoApertura{
		FuenteDelProyecto: a.FuenteDelProyecto, CommitDelProyecto: a.CommitDelProyecto,
		FuenteDelPipeline: a.FuenteDelPipeline, CommitDelPipeline: a.CommitDelPipeline,
		OrdenDeAmbientes: a.OrdenDeAmbientes,
	})
	if err != nil {
		return historialpublicado.Contenido{}, fmt.Errorf("ejecución: codificar el contenido de una apertura: %w", err)
	}
	return historialpublicado.Contenido{Contexto: contextoApertura, Datos: datos}, nil
}

// decodificarContenidoDeApertura da las dos fuentes con las que se abrió un intento. Quien llama (historial.go,
// DespliegueParaRollback) las combina con el despliegue y el ambiente, que no van en este contenido: ya los da
// el propio Despliegue del Historial.
func decodificarContenidoDeApertura(c historialpublicado.Contenido) (contenidoApertura, error) {
	if c.Contexto != contextoApertura {
		return contenidoApertura{}, fmt.Errorf("ejecución: contenido de contexto %q, se esperaba %q", c.Contexto, contextoApertura)
	}
	var ca contenidoApertura
	if err := json.Unmarshal(c.Datos, &ca); err != nil {
		return contenidoApertura{}, fmt.Errorf("ejecución: decodificar el contenido de una apertura: %w", err)
	}
	return ca, nil
}

const contextoRegistro = "ejecucion/registro-v1"

type contenidoRegistro struct {
	HashDelCodigo       string `json:"hash_del_codigo"`
	HashDeInstrucciones string `json:"hash_de_instrucciones"`
	HashDeVariables     string `json:"hash_de_variables"`
}

func codificarContenidoDeRegistro(recursos dominio.RecursosDeUnPaso) (historialpublicado.Contenido, error) {
	datos, err := json.Marshal(contenidoRegistro{
		HashDelCodigo:       recursos.HashDelCodigo().String(),
		HashDeInstrucciones: recursos.HashDeInstrucciones().String(),
		HashDeVariables:     recursos.HashDeVariables().String(),
	})
	if err != nil {
		return historialpublicado.Contenido{}, fmt.Errorf("ejecución: codificar el contenido de un registro: %w", err)
	}
	return historialpublicado.Contenido{Contexto: contextoRegistro, Datos: datos}, nil
}

func decodificarContenidoDeRegistro(c historialpublicado.Contenido) (dominio.RecursosDeUnPaso, error) {
	if c.Contexto != contextoRegistro {
		return dominio.RecursosDeUnPaso{},
			fmt.Errorf("ejecución: contenido de contexto %q, se esperaba %q", c.Contexto, contextoRegistro)
	}
	var cr contenidoRegistro
	if err := json.Unmarshal(c.Datos, &cr); err != nil {
		return dominio.RecursosDeUnPaso{}, fmt.Errorf("ejecución: decodificar el contenido de un registro: %w", err)
	}
	codigo, err := dominio.NuevoHashDeCodigo(cr.HashDelCodigo)
	if err != nil {
		return dominio.RecursosDeUnPaso{}, err
	}
	instrucciones, err := dominio.NuevoHashDeInstrucciones(cr.HashDeInstrucciones)
	if err != nil {
		return dominio.RecursosDeUnPaso{}, err
	}
	return dominio.NuevosRecursosDeUnPaso(codigo, instrucciones, dominio.NuevoHashDeVariables(cr.HashDeVariables)), nil
}
