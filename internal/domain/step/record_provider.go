package step

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// El proveedor del registro VIGENTE (spec 28 §5.2', §5.3').
//
// # La pregunta es una y las respuestas son dos
//
// «¿Qué registro está vigente para este step bajo esta clave?» tiene dos
// respuestas legítimas —*el último* y *el anclado*— y son intercambiables sin
// que el step lo note. Un `if` dentro del handler ocultaría que son dos formas
// de responder lo MISMO, y haría que una tercera —anclar a una etiqueta, a una
// fecha— exigiera un `if` más.
//
// De ahí Strategy y no una bandera. Y de ahí que el bucle de decisión de la
// spec 15 §5.6 no se modifique: se le inyecta otro proveedor. Si implementar el
// rollback obligara a tocar las reglas, el modelo estaría mal repartido.
//
// # Y alcanza a las TRES lecturas, no sólo a la decisión
//
// El bucle de decisión no es el único sitio donde el motor pregunta por el
// registro de una clave (spec 25, recuadro):
//
//	01_vars_store_handler   llena el mapa acumulado con lo que este step dejó
//	StateResolver           resuelve una declaración `resolve: state`
//	03_step_runner_handler  decide si el step se re-ejecuta
//
// Si el ancla alcanzara sólo a la tercera, un rollback **decidiría** con el
// registro anclado y **resolvería** con los valores de hoy: la partición de §1
// —«la aplicación vuelve atrás y la infraestructura no»— reaparecida un piso más
// abajo y dentro de una sola ejecución. Por eso las tres dependen de este puerto
// y ninguna de `state.Records`.
//
// La ESCRITURA no pasa por aquí y no debe: un rollback es una ejecución nueva
// que escribe registros nuevos (§5.4), así que `StepExecutable` sigue hablando
// con `state.Records` directamente. Que este puerto no tenga `Append` es el tipo
// diciendo esa regla.
type RecordProvider interface {
	// Current es el registro vigente para una clave.
	//
	// Misma firma y misma asimetría que `state.Records.Last`, que es lo que hace
	// que las dos políticas sean sustituibles: ausencia NO es error —aguas arriba
	// significa ejecutar—, ilegible SÍ.
	Current(ctx *context.Context, key state.Key) (state.StepRecord, bool, error)
}

// LastRecordProvider es la política normal: el registro vigente es el ÚLTIMO de
// la clave.
type LastRecordProvider struct {
	records state.Records
}

var _ RecordProvider = LastRecordProvider{}

func NewLastRecordProvider(records state.Records) LastRecordProvider {
	return LastRecordProvider{records: records}
}

func (p LastRecordProvider) Current(
	ctx *context.Context, key state.Key) (state.StepRecord, bool, error) {
	return p.records.Last(ctx, key)
}

// AnchorLookup es lo ÚNICO que la cadena de step necesita saber de un ancla de
// rollback: qué registro estuvo vigente para un step.
//
// Se declara aquí, en el paquete que lo consume, y no se importa de
// `deployment`, porque el import va en el otro sentido —`deployment` compone su
// objeto con `step.StepConfig`— y nombrarlo desde aquí cerraría el ciclo. Es el
// mismo reparto que `step.FactSink` con `record.Facts` (spec 19 §9.1):
// `deployment.RollbackAnchor` lo satisface estructuralmente y la comprobación de
// contrato vive donde se cablea, que es el único sitio que ve los dos lados.
type AnchorLookup interface {
	// AnchoredRecord es la referencia al registro vigente para un step, si lo
	// hubo. Falso significa «no hay a qué anclar este step», y aguas arriba eso
	// se comporta como una ausencia de registro: se ejecuta.
	AnchoredRecord(stepID string) (state.Key, state.RecordID, bool)
}

// AnchoredRecordProvider es la política del rollback: el registro vigente es el
// que estuvo vigente en la ejecución destino.
//
// # No relaja ninguna comprobación, y eso es comprobable
//
// Devuelve un registro con la misma forma que el otro proveedor, así que el
// bucle evalúa sobre él las MISMAS reglas con la MISMA huella. Si el trabajo
// cambió respecto del registro anclado, el step se ejecuta — que es lo correcto
// (§5.3). La sustituibilidad no es una propiedad estética: es la prueba de que
// el rollback no se está saltando nada.
type AnchoredRecordProvider struct {
	records state.Records
	anchor  AnchorLookup
}

var _ RecordProvider = AnchoredRecordProvider{}

func NewAnchoredRecordProvider(records state.Records, anchor AnchorLookup) AnchoredRecordProvider {
	return AnchoredRecordProvider{records: records, anchor: anchor}
}

// Current resuelve la clave contra el ancla.
//
// Dos ausencias, y las dos significan lo mismo aguas arriba —ejecutar—:
//
//   - el step no dejó registro en la ejecución anclada (sin `config.yaml`, sin
//     comandos, sin `rules`), así que no hay a qué volver **y no hace falta**:
//     son steps que se ejecutan siempre;
//   - la clave que se consulta NO es la que el ancla apunta. Ocurre en la carga,
//     que lee los dos ámbitos y sólo uno es el declarado (spec 13 §5.4): el otro
//     no tuvo registro vigente en aquel intento, y devolver el del ámbito
//     equivocado sería inventarse un ancla. Que el ámbito DECLARADO no haya
//     cambiado entre las dos ejecuciones ya lo comprobó `RollbackAnchor.Validate`
//     antes del primer step.
func (p AnchoredRecordProvider) Current(
	ctx *context.Context, key state.Key) (state.StepRecord, bool, error) {

	anchored, recordID, found := p.anchor.AnchoredRecord(key.StepID())
	if !found || !anchored.Equals(key) {
		return state.StepRecord{}, false, nil
	}
	return p.records.Get(ctx, key, recordID)
}

// CurrentRecords es el punto de INYECCIÓN de la política, y es lo que hace que
// los tres handlers dependan del puerto y no de una de sus dos implementaciones.
//
// # Por qué hay una indirección y no se elige al cablear
//
// Porque el cableado ocurre ANTES de leer el `RequestInput`: `BuildRunCommand`
// arma las tres cadenas sin saber todavía si esta ejecución es un rollback.
// Es el mismo reparto que `LoadedPipelinecode` —construido al cablear, llenado
// por el handler 09— y que `record.ParameterDigester`, que nace sin clave y la
// recibe al ejecutar.
//
// La política se elige UNA vez, antes del primer step, y a partir de ahí no
// cambia: el ancla es inmutable y quien la instala es el mismo handler que la
// resolvió. Arranca en «el último», que es la ejecución normal.
type CurrentRecords struct {
	records  state.Records
	provider RecordProvider
}

var _ RecordProvider = (*CurrentRecords)(nil)

func NewCurrentRecords(records state.Records) *CurrentRecords {
	return &CurrentRecords{records: records, provider: NewLastRecordProvider(records)}
}

// UseAnchor instala la política del rollback.
//
// Compone el proveedor con el MISMO almacén con el que se construyó, y eso no es
// comodidad: un ancla que resolviera sus `record_id` contra un almacén que no es
// el que los escribió apunta a nada (spec 21, recuadro). Que el llamador no
// pueda pasar otro es la forma de que esa pareja no se pueda romper.
func (c *CurrentRecords) UseAnchor(anchor AnchorLookup) {
	c.provider = NewAnchoredRecordProvider(c.records, anchor)
}

func (c *CurrentRecords) Current(
	ctx *context.Context, key state.Key) (state.StepRecord, bool, error) {
	return c.provider.Current(ctx, key)
}
