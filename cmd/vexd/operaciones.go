package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// errEntrada es una petición que no se pudo leer: JSON mal formado o con campos que el lenguaje publicado no
// tiene. Es un error de quien invoca, no de la operación.
var errEntrada = errors.New("petición ilegible")

// atender lee la petición de una operación, la envía al borde y devuelve su respuesta. salida es a donde van,
// en vivo, los comandos de los pasos (DEC-12.5); solo las operaciones que ejecutan comandos la usan.
type atender func(ctx context.Context, s *borde.Servicio, peticion []byte, salida ejecucionpublicado.Salida) (any, error)

// operacion es una operación del lenguaje publicado (docs/modelo/lenguaje-publicado.md) con su nombre en la
// línea de comandos.
type operacion struct {
	nombre string
	// usaEspacio: la operación ejecuta comandos, así que necesita el espacio de trabajo de los ambientes.
	usaEspacio bool
	atender    atender
}

var operaciones = []operacion{
	{"intentar", true, conSalida((*borde.Servicio).Intentar)},
	{"rollback", true, conSalida((*borde.Servicio).HacerRollback)},
	{"simular", false, consulta((*borde.Servicio).Simular)},
	{"lanzar", false, consulta((*borde.Servicio).Lanzar)},
	{"reservar", false, sinRespuesta((*borde.Servicio).Reservar)},
	{"liberar", false, sinRespuesta((*borde.Servicio).Liberar)},
	{"diagnosticar", false, consulta((*borde.Servicio).PreguntarLaCausa)},
	{"abandonar", false, sinRespuesta((*borde.Servicio).AbandonarIntento)},
	{"intento", false, consultaResumida((*borde.Servicio).Intento, resumir)},
	{"intentos", false, consultaResumida((*borde.Servicio).IntentosDeUnAmbiente, resumirTodos)},
	{"despliegues", false, consulta((*borde.Servicio).DesplieguesDeUnAmbiente)},
}

func buscar(nombre string) (operacion, bool) {
	for _, o := range operaciones {
		if o.nombre == nombre {
			return o, true
		}
	}
	return operacion{}, false
}

func conSalida[P, R any](
	op func(*borde.Servicio, context.Context, P, ejecucionpublicado.Salida) (R, error),
) atender {
	return func(ctx context.Context, s *borde.Servicio, peticion []byte, salida ejecucionpublicado.Salida) (any, error) {
		var p P
		if err := decodificar(peticion, &p); err != nil {
			return nil, err
		}
		return op(s, ctx, p, salida)
	}
}

func consulta[P, R any](op func(*borde.Servicio, context.Context, P) (R, error)) atender {
	return func(ctx context.Context, s *borde.Servicio, peticion []byte, _ ejecucionpublicado.Salida) (any, error) {
		var p P
		if err := decodificar(peticion, &p); err != nil {
			return nil, err
		}
		return op(s, ctx, p)
	}
}

// consultaResumida es una consulta cuya respuesta se resume antes de imprimirse: el borde publica el detalle
// completo, pero la línea de comandos solo muestra lo que resumir deja.
func consultaResumida[P, R, V any](
	op func(*borde.Servicio, context.Context, P) (R, error), resumir func(R) V,
) atender {
	return consulta(func(s *borde.Servicio, ctx context.Context, p P) (V, error) {
		r, err := op(s, ctx, p)
		if err != nil {
			var vacio V
			return vacio, err
		}
		return resumir(r), nil
	})
}

// sinRespuesta atiende las operaciones que solo dicen si salieron bien: su respuesta es un objeto vacío, para
// que quien invoca pueda leer siempre una respuesta.
func sinRespuesta[P any](op func(*borde.Servicio, context.Context, P) error) atender {
	return consulta(func(s *borde.Servicio, ctx context.Context, p P) (struct{}, error) {
		return struct{}{}, op(s, ctx, p)
	})
}

// decodificar es estricta: un campo que el lenguaje publicado no tiene es casi siempre un error de escritura,
// y ignorarlo haría que el motor hiciera otra cosa de la que se pidió.
func decodificar(peticion []byte, destino any) error {
	d := json.NewDecoder(bytes.NewReader(peticion))
	d.DisallowUnknownFields()
	if err := d.Decode(destino); err != nil {
		return fmt.Errorf("%w: %w", errEntrada, err)
	}
	return nil
}
