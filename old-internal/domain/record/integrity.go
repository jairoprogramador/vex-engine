package record

import (
	"fmt"
	"slices"
)

// FaultKind es el vocabulario CERRADO de lo que puede estar mal en una tira.
//
// Cerrado por lo mismo que `EventType` y `ErrorClass`: **texto libre no es
// agregable**. Es la diferencia entre «3 tiras con huecos de `seq`» y una lista
// de frases que nadie puede contar.
//
// Y hay una distinción que este vocabulario existe para no perder: un
// `content_id` que NO recomputa y un `content_id` que no se puede recomputar
// AQUÍ no son el mismo hecho (spec 22 §5.1'). El primero es corrupción; el
// segundo es un límite del binario que verifica, y reportarlo como corrupción
// sería la peor forma de fallo posible para esta comprobación — señalar un
// registro sano.
type FaultKind string

const (
	// FaultSeqGap es un hueco en la numeración: falta al menos un hecho entre dos
	// que sí están. Sólo se ve recorriendo la tira, porque `Fold` ignora los
	// huecos a propósito: ordena por `seq` y no le importa que falte uno.
	FaultSeqGap FaultKind = "seq_gap"

	// FaultSeqRepetida es la misma posición dos veces. `Fold` la tolera —el orden
	// estable conserva el de llegada— pero significa que dos hechos distintos
	// afirman ocupar el mismo lugar, y eso no lo puede producir un emisor sano.
	FaultSeqRepetida FaultKind = "seq_duplicated"

	// FaultParAbierto es un `*_started` sin su `*_finished` en un intento que SÍ
	// se cerró. En un intento `interrupted` no es un defecto: es exactamente lo
	// que la interrupción significa.
	FaultParAbierto FaultKind = "unclosed_pair"

	// FaultLineaIlegible es una línea que no se pudo decodificar. Se cuenta y no
	// se salta en silencio: es la cola de una muerte dura, o —contra el destino—
	// una línea rota EN MEDIO que el empuje conservó a propósito.
	FaultLineaIlegible FaultKind = "unreadable_line"

	// FaultContentIDNoRecomputa es el objeto cuyo `content_id` declarado no
	// coincide con el resumen de su forma canónica. Es CORRUPCIÓN: la dirección
	// del objeto es su contenido, así que el archivo afirma ser algo que no es.
	FaultContentIDNoRecomputa FaultKind = "content_id_mismatch"

	// FaultNoComparable es «no puedo recomputarlo con la regla que tengo». Un
	// registro emitido con `v1:` y verificado por un binario que ya calcula `v2:`
	// no debe recomputar y fallar: debe leer el prefijo y decirlo. Es para lo que
	// existe el prefijo de versión, y es aquí donde se cobra.
	FaultNoComparable FaultKind = "not_comparable"

	// FaultObjetoAusente es el despliegue del que hay hechos y cuyo objeto no se
	// consigue ENLAZAR.
	//
	// **No cuenta como corrupción**, y la razón es la misma que la de
	// `not_comparable`: el enlace despliegue→objeto no está escrito en ninguna parte
	// —se deriva recomputando `dep-v1`—, así que hace falta conocer el PADRE. Un
	// destino al que un empuje anterior no llegó tiene el objeto y no tiene la tira
	// del padre, y ahí el enlace no se puede derivar aunque todo esté sano.
	//
	// Marcarlo como corrupción sería el fallo que este comando existe para no
	// cometer: señalar un registro que no la tiene. Se REPORTA —es una tienda que
	// hay que mirar— y no cambia el exit code.
	FaultObjetoAusente FaultKind = "object_missing"

	// FaultObjetoIlegible es el objeto que está y no se puede decodificar. Va en el
	// vocabulario y no como una cadena inventada en el sitio donde se detecta,
	// porque un vocabulario cerrado en el que el detector pueda añadir valores no
	// está cerrado — y lo que se pierde es la capacidad de CONTAR.
	FaultObjetoIlegible FaultKind = "unreadable_object"

	// FaultTiraIlegible es la tira que no se puede ni abrir, que es otra cosa que una
	// línea rota dentro de ella: de la línea se sabe qué falta y de esto no se sabe
	// nada. Se distingue para que «una tira inaccesible» no se cuente como «una
	// tira con una línea mala».
	FaultTiraIlegible FaultKind = "unreadable_strip"

	// FaultObjetoMalformado es el objeto que decodifica y no declara con qué regla se
	// identificó: un prefijo de versión ausente, vacío, o con un hash que no tiene
	// forma de hash.
	//
	// **Es corrupción, y separarlo de `not_comparable` es el punto.** Un motor más
	// nuevo cambiaría la REGLA; no dejaría de escribir un `sha256` en hexadecimal.
	// Sin la distinción, borrar el prefijo de un objeto lo volvería «no comparable» y
	// `verify` diría que la tienda está bien.
	FaultObjetoMalformado FaultKind = "malformed_object"
)

func (k FaultKind) String() string { return string(k) }

// EsCorrupcion separa lo que acusa AL REGISTRO de lo que acusa al lector: a la
// regla que este binario no tiene, o al dato que desde aquí no se puede derivar.
//
// Los dos que no cuentan son los dos límites del lector, y en ambos casos el
// registro puede estar perfectamente sano:
//
//	not_comparable   el objeto se identificó con una regla que este binario no
//	                 calcula. Contarla convertiría la actualización del motor en
//	                 una alarma de corrupción masiva.
//	object_missing   el enlace despliegue→objeto se DERIVA, y derivarlo necesita el
//	                 padre. Un destino al que un empuje anterior no llegó lo tiene
//	                 todo menos ese padre.
//
// Los dos se REPORTAN igual: son cosas que hay que mirar. Lo que no hacen es
// cambiar el exit code, que es lo que un operador usa para decidir si el registro
// está roto.
func (k FaultKind) EsCorrupcion() bool {
	switch k {
	case FaultNoComparable, FaultObjetoAusente:
		return false
	default:
		return true
	}
}

// Fault es un defecto localizado.
//
// `Detail` es para el humano y `Kind` para el agregado: nadie cuenta por la
// frase.
type Fault struct {
	Kind   FaultKind
	Detail string
}

func (f Fault) String() string { return f.Kind.String() + ": " + f.Detail }

// CheckStrip comprueba los invariantes que la tira puede violar (spec 22 §5.1').
//
// # Es una función de dominio PURA, como `Fold`
//
// Sin estado, sin I/O, determinista. Y por la misma razón: un invariante que sólo
// se puede comprobar con un disco delante no se puede comprobar en el ingestor
// del backend (spec 26), que es el otro sitio donde estas mismas reglas tienen
// que valer.
//
// # El trabajo se reparte en dos mitades que no se parecen (spec 17 §9)
//
//	`seq` sin huecos        →  EXIGE recorrer la tira. `Fold` ignora los huecos
//	                          a propósito.
//	pares abiertos de STEP  →  sale GRATIS del pliegue: `Finished == false` ya
//	                          distingue «empezó y no terminó» de «terminó mal».
//	pares abiertos de COMANDO → exige recorrer: el pliegue sólo los CUENTA.
//
// De ahí que reciba las dos cosas —los hechos y su pliegue— en vez de recomputar
// uno de los dos: plegar dos veces la misma tira daría dos oportunidades de que
// las dos lecturas discrepen.
//
// # Lo que NO comprueba, y no es un olvido
//
// Que `attempt_finished` sea el ÚLTIMO hecho. No lo es por construcción: un
// `sync_failed` del empuje de cierre se emite DESPUÉS del desenlace (spec 21
// §9.3). Exigir el orden convertiría un empuje fallido en una acusación de
// corrupción.
//
// Y no comprueba las huellas del índice: es desechable y no tiene invariantes
// que preservar — una entrada que no recompute simplemente no se acierta, y el
// step se ejecuta.
func CheckStrip(events []Event, result AttemptResult) []Fault {
	faults := make([]Fault, 0, 4)
	faults = append(faults, checkSeq(events)...)
	faults = append(faults, checkPares(events, result)...)
	return faults
}

// checkSeq recorre las posiciones ordenadas y exige 1, 2, 3… sin saltos.
//
// Empezar en algo distinto de 1 es un hueco por delante, y es el caso real de un
// archivo truncado por el principio o de una tira leída desde el destino a la que
// le faltan los primeros hechos. Se reporta como hueco y no como un caso propio:
// lo que falta es lo mismo.
func checkSeq(events []Event) []Fault {
	posiciones := make([]uint64, 0, len(events))
	for _, event := range events {
		if event.IsZero() {
			continue
		}
		posiciones = append(posiciones, event.Seq().Position())
	}
	if len(posiciones) == 0 {
		return nil
	}
	slices.Sort(posiciones)

	faults := make([]Fault, 0, 2)
	esperada := uint64(1)
	for i, posicion := range posiciones {
		switch {
		case i > 0 && posicion == posiciones[i-1]:
			faults = append(faults, Fault{
				Kind:   FaultSeqRepetida,
				Detail: fmt.Sprintf("la posición %d aparece más de una vez", posicion),
			})
			continue
		case posicion > esperada:
			// El detalle nombra el HUECO entero y no sólo su primera posición: «falta la
			// 2» cuando faltan la 2 y la 3 subestima lo que se perdió, y quien lee un
			// informe de integridad decide con el tamaño.
			faltan := posicion - esperada
			detalle := fmt.Sprintf(
				"falta la posición %d (el hecho siguiente ocupa la %d)", esperada, posicion)
			if faltan > 1 {
				detalle = fmt.Sprintf(
					"faltan %d posiciones, de la %d a la %d (el hecho siguiente ocupa la %d)",
					faltan, esperada, posicion-1, posicion)
			}
			faults = append(faults, Fault{Kind: FaultSeqGap, Detail: detalle})
		}
		esperada = posicion + 1
	}
	return faults
}

// checkPares exige que todo lo que se abrió se haya cerrado, y sólo en un intento
// que llegó a cerrarse él mismo.
//
// El pliegue basta para los steps; los comandos se cuentan aquí porque
// `AttemptResult` guarda conteos y no una lista —duplicar el detalle por comando
// sería la divergencia que el resultado derivado existe para evitar—.
func checkPares(events []Event, result AttemptResult) []Fault {
	if result.Status == AttemptInterrupted {
		// No hay nada que acusar: un par abierto ES lo que la interrupción
		// significa, y el propio `interrupted` ya lo dice.
		return nil
	}

	faults := make([]Fault, 0, 2)
	for _, step := range result.Steps {
		if !step.Finished {
			faults = append(faults, Fault{
				Kind: FaultParAbierto,
				Detail: fmt.Sprintf(
					"el step '%s' abrió y no cerró en un intento '%s'", step.StepID, result.Status),
			})
		}
	}

	// El orden de aparición se lleva en su propio conjunto y no se deduce del
	// contador: un `command_finished` sin su apertura —lo que deja una tira truncada
	// POR DELANTE— pone el contador en negativo, y deducir «es la primera vez que
	// veo esta clave» de `contador == 0` haría que la apertura siguiente no se
	// registrara. El defecto que se dejaría de ver es justo el que se busca.
	abiertos := make(map[string]int, 8)
	vistas := make(map[string]struct{}, 8)
	orden := make([]string, 0, 8)
	anotar := func(clave string) {
		if _, vista := vistas[clave]; !vista {
			vistas[clave] = struct{}{}
			orden = append(orden, clave)
		}
	}

	for _, event := range events {
		if event.IsZero() {
			continue
		}
		switch payload := event.Payload().(type) {
		case CommandStarted:
			clave := payload.StepID + "/" + payload.CommandName
			anotar(clave)
			abiertos[clave]++
		case CommandFinished:
			clave := payload.StepID + "/" + payload.CommandName
			anotar(clave)
			abiertos[clave]--
		}
	}
	for _, clave := range orden {
		if abiertos[clave] > 0 {
			faults = append(faults, Fault{
				Kind: FaultParAbierto,
				Detail: fmt.Sprintf(
					"el comando '%s' abrió y no cerró en un intento '%s'", clave, result.Status),
			})
		}
	}
	return faults
}
