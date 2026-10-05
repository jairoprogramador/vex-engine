package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jairoprogramador/vex-engine/internal/borde"
)

const nombreDescribir = "describir"

// atender lee los parámetros de una operación, la envía al borde y devuelve su respuesta. El servicio es nil en
// las operaciones que no usan el motor.
type atender func(ctx context.Context, s *borde.Servicio, params []byte) (any, error)

// operacion es una operación del lenguaje publicado (docs/modelo/lenguaje-publicado.md) con el nombre con que se
// pide: el method de la petición.
type operacion struct {
	nombre string
	// usaMotor: la operación necesita el almacén y los contextos compuestos. describir no: sirve para saber si
	// este motor y quien invoca se entienden antes de pedirle nada.
	usaMotor bool
	// usaEspacio: la operación ejecuta comandos, así que necesita el espacio de trabajo de los ambientes.
	usaEspacio bool
	atender    atender
}

func delMotor(nombre string, atender atender) operacion {
	return operacion{nombre: nombre, usaMotor: true, atender: atender}
}

func delMotorConEspacio(nombre string, atender atender) operacion {
	return operacion{nombre: nombre, usaMotor: true, usaEspacio: true, atender: atender}
}

var operaciones = registrar(
	delMotorConEspacio("intentar", consulta((*borde.Servicio).Intentar)),
	delMotorConEspacio("rollback", consulta((*borde.Servicio).HacerRollback)),
	delMotor("simular", consulta((*borde.Servicio).Simular)),
	delMotor("lanzar", consulta((*borde.Servicio).Lanzar)),
	delMotor("reservar", sinRespuesta((*borde.Servicio).Reservar)),
	delMotor("liberar", sinRespuesta((*borde.Servicio).Liberar)),
	delMotor("diagnosticar", consulta((*borde.Servicio).PreguntarLaCausa)),
	delMotor("abandonar", sinRespuesta((*borde.Servicio).AbandonarIntento)),
	delMotor("intento", consulta((*borde.Servicio).Intento)),
	delMotor("intentos", consulta((*borde.Servicio).IntentosDeUnAmbiente)),
	delMotor("despliegues", consulta((*borde.Servicio).DesplieguesDeUnAmbiente)),
	delMotor("logs", consulta((*borde.Servicio).Logs)),
)

// registrar añade describir, que cuenta las operaciones y por eso no puede ser una fila más de la tabla.
func registrar(ops ...operacion) []operacion {
	nombres := make([]string, 0, len(ops)+1)
	for _, o := range ops {
		nombres = append(nombres, o.nombre)
	}
	nombres = append(nombres, nombreDescribir)
	sort.Strings(nombres)
	return append(ops, operacion{nombre: nombreDescribir, atender: atenderDescribir(nombres)})
}

func buscar(nombre string) (operacion, bool) {
	for _, o := range operaciones {
		if o.nombre == nombre {
			return o, true
		}
	}
	return operacion{}, false
}

// descripcion es lo que dice este motor de sí mismo: la imagen del contenedor y quien invoca se versionan por
// separado, y así se puede comprobar que se entienden.
type descripcion struct {
	VersionDelMotor      string
	VersionesDelLenguaje []string
	Operaciones          []string
}

type sinParametros struct{}

func atenderDescribir(nombres []string) atender {
	return func(_ context.Context, _ *borde.Servicio, params []byte) (any, error) {
		var vacios sinParametros
		if err := decodificar(params, &vacios); err != nil {
			return nil, err
		}
		return descripcion{
			VersionDelMotor:      version,
			VersionesDelLenguaje: borde.VersionesSoportadas,
			Operaciones:          nombres,
		}, nil
	}
}

func consulta[P, R any](op func(*borde.Servicio, context.Context, P) (R, error)) atender {
	return func(ctx context.Context, s *borde.Servicio, params []byte) (any, error) {
		var p P
		if err := decodificar(params, &p); err != nil {
			return nil, err
		}
		return op(s, ctx, p)
	}
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
func decodificar(params []byte, destino any) error {
	d := json.NewDecoder(bytes.NewReader(params))
	d.DisallowUnknownFields()
	if err := d.Decode(destino); err != nil {
		return fmt.Errorf("%w: %w", errParametros, err)
	}
	return nil
}
