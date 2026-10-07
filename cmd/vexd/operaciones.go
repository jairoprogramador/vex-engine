package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

const nombreDescribir = "describir"

// atender lee los parámetros de una operación, la envía al borde y devuelve su respuesta. El servicio es nil en
// las operaciones que no usan el motor. entorno son las variables de entorno de la petición: solo las
// operaciones que ejecutan comandos las usan, y las demás las rechaza el servidor antes de llegar aquí.
type atender func(ctx context.Context, s *borde.Servicio, params []byte, entorno map[string]string) (any, error)

// operacion es una operación del lenguaje publicado (docs/modelo/lenguaje-publicado.md) con el nombre con que se
// pide: el method de la petición.
type operacion struct {
	nombre string
	// usaMotor: la operación necesita el almacén y los contextos compuestos. describir no: sirve para saber si
	// este motor y quien invoca se entienden antes de pedirle nada.
	usaMotor bool
	// ejecutaComandos: la operación ejecuta comandos, así que necesita el espacio de trabajo de los ambientes
	// (VEX_ESPACIO) y admite variables de entorno para ellos.
	ejecutaComandos bool
	atender         atender
}

func delMotor(nombre string, atender atender) operacion {
	return operacion{nombre: nombre, usaMotor: true, atender: atender}
}

func ejecutandoComandos(nombre string, atender atender) operacion {
	return operacion{nombre: nombre, usaMotor: true, ejecutaComandos: true, atender: atender}
}

var operaciones = registrar(
	ejecutandoComandos("intentar", consultaConEntorno((*borde.Servicio).Intentar)),
	ejecutandoComandos("rollback", consultaConEntorno((*borde.Servicio).HacerRollback)),
	delMotor("simular", consulta((*borde.Servicio).Simular)),
	delMotor("lanzar", consulta((*borde.Servicio).Lanzar)),
	delMotor("reservar", sinRespuesta((*borde.Servicio).Reservar)),
	delMotor("liberar", sinRespuesta((*borde.Servicio).Liberar)),
	delMotor("diagnosticar", consultaResumida((*borde.Servicio).PreguntarLaCausa, presentarDiagnostico)),
	delMotor("abandonar", sinRespuesta((*borde.Servicio).AbandonarIntento)),
	delMotor("intento", consulta((*borde.Servicio).Intento)),
	delMotor("intentos", consultaResumida((*borde.Servicio).IntentosDeUnAmbiente, resumirTodos)),
	delMotor("despliegues", consulta((*borde.Servicio).DesplieguesDeUnAmbiente)),
	delMotor("lanzamientos", consulta((*borde.Servicio).LanzamientosDeUnAmbiente)),
	delMotor("ambientes", consulta((*borde.Servicio).Ambientes)),
	delMotor("pasos", consulta((*borde.Servicio).Pasos)),
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
	return func(_ context.Context, _ *borde.Servicio, params []byte, _ map[string]string) (any, error) {
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
	return func(ctx context.Context, s *borde.Servicio, params []byte, _ map[string]string) (any, error) {
		var p P
		if err := decodificar(params, &p); err != nil {
			return nil, err
		}
		return op(s, ctx, p)
	}
}

// consultaResumida es una consulta cuya respuesta se resume antes de responderse: el borde publica el detalle
// completo, pero la operación responde solo lo que resumir deja.
func consultaResumida[P, R, V any](
	op func(*borde.Servicio, context.Context, P) (R, error), resumir func(R) V,
) atender {
	return consulta(func(s *borde.Servicio, ctx context.Context, p P) (V, error) {
		respuesta, err := op(s, ctx, p)
		if err != nil {
			var vacio V
			return vacio, err
		}
		return resumir(respuesta), nil
	})
}

// consultaConEntorno es una consulta que, además de sus parámetros, recibe las variables de entorno de la
// petición: las que ejecutan comandos.
func consultaConEntorno[P, R any](
	op func(*borde.Servicio, context.Context, P, ejecucionpublicado.Entorno) (R, error),
) atender {
	return func(ctx context.Context, s *borde.Servicio, params []byte, entorno map[string]string) (any, error) {
		var p P
		if err := decodificar(params, &p); err != nil {
			return nil, err
		}
		return op(s, ctx, p, entorno)
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
		return ilegible(err)
	}
	return nil
}

// parametroIlegibleError es errParametros con el campo que no se pudo leer, si se sabe cuál es: el cliente lo
// recibe en data.campo y no tiene que leer el texto. El valor es lo que dice el JSON de ese campo, no su
// contenido: «string» si se pidió un número, por ejemplo.
type parametroIlegibleError struct {
	campo string
	valor string
	causa error
}

func (e *parametroIlegibleError) Error() string {
	return errParametros.Error() + ": " + e.causa.Error()
}

func (e *parametroIlegibleError) Is(destino error) bool { return destino == errParametros }

func (e *parametroIlegibleError) Unwrap() error { return e.causa }

func (e *parametroIlegibleError) ParametroInvalido() (campo, valor string) { return e.campo, e.valor }

// ilegible clasifica un fallo de la lectura de los params. Un JSON mal formado no apunta a ningún campo.
func ilegible(err error) error {
	e := &parametroIlegibleError{causa: err}
	var tipo *json.UnmarshalTypeError
	if errors.As(err, &tipo) {
		e.campo, e.valor = tipo.Field, tipo.Value
	} else if campo, ok := campoDesconocido(err); ok {
		e.campo = campo
	}
	return e
}

// campoDesconocido saca el nombre del campo del error de DisallowUnknownFields, que la biblioteca estándar solo
// da como texto.
func campoDesconocido(err error) (string, bool) {
	const prefijo = "json: unknown field "
	texto, ok := strings.CutPrefix(err.Error(), prefijo)
	if !ok {
		return "", false
	}
	campo, errComillas := strconv.Unquote(texto)
	return campo, errComillas == nil
}
