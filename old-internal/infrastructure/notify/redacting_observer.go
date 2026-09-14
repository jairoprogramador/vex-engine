package notify

import (
	"sort"
	"strings"
	"sync"

	domNotify "github.com/jairoprogramador/vex-engine/old-internal/domain/notify"
)

// MinRedactableLength es la longitud a partir de la cual un valor se redacta
// (spec 20 §5.3, regla 1).
//
// Sin umbral el mecanismo destroza cualquier salida: el mapa acumulado lleva
// `environment=sand` y `project_status=clean`, así que cada aparición de `sand`
// o de `clean` en el stdout de un comando —dentro de una ruta, de un nombre de
// recurso, de una frase— se convertiría en `${var.environment}`. Ocho es el
// punto donde una coincidencia accidental deja de ser probable.
//
// Es público porque es una REGLA, no un detalle: los tests de la §7 la nombran y
// quien la cambie tiene que ver que hay un caso que la fija.
const MinRedactableLength = 8

// RedactingObserver sustituye los valores conocidos por su referencia antes de
// que la línea salga del proceso.
//
// # Es un decorador, y se aplica en el BORDE (spec 20 §5.4)
//
// Envuelve al `MultiObserver` entero, no a cada observador concreto: stdout y
// Supabase reciben exactamente la misma línea redactada, y añadir un tercer
// destino no añade una tercera oportunidad de olvidarse. La alternativa
// —redactar en las llamadas a `Emit`— garantizaría que la número 35 lo olvide.
//
// # Lo que la redacción NO cubre, escrito en vez de disimulado
//
//   - **Apariciones LITERALES solamente.** Un valor codificado en base64,
//     url-encoded o escapado dentro de un JSON no se detecta. No se disfraza con
//     heurísticas que darían falsa confianza: es la razón por la que el registro
//     no guarda extractos (§5.1), porque un mecanismo best-effort es aceptable
//     para un canal rotable y no para uno permanente.
//   - **La línea que REVELA el valor por primera vez.** Un `show: true` imprime
//     el stdout del comando en el handler 03 y la extracción ocurre en el 05, así
//     que en el instante en que la línea sale el valor todavía no es un valor
//     conocido. Lo que se protege es toda aparición POSTERIOR —el resto del step,
//     los steps siguientes y todas las ejecuciones que lean ese valor del
//     almacén—, que es donde un secreto se repite N veces. La primera aparición
//     sólo la evita no imprimirla: `show` es una decisión del pipelinecode.
type RedactingObserver struct {
	inner domNotify.LogObserver

	// El vocabulario llega POR EJECUCIÓN —lo tiene el mapa acumulado, que nace
	// con el `ExecutionContext`— y el observador se construye antes. El mutex es
	// por eso y no por concurrencia de escritura, igual que en `EventRenderer`.
	mu         sync.Mutex
	vocabulary domNotify.Vocabulary
}

var (
	_ domNotify.LogObserver     = (*RedactingObserver)(nil)
	_ domNotify.VocabularyAware = (*RedactingObserver)(nil)
)

func NewRedactingObserver(inner domNotify.LogObserver) *RedactingObserver {
	return &RedactingObserver{inner: inner}
}

// UseVocabulary instala los valores que esta ejecución conoce. Sin él el
// observador es un paso a través: no hay nada que redactar porque no se sabe qué
// es un valor.
func (r *RedactingObserver) UseVocabulary(vocabulary domNotify.Vocabulary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vocabulary = vocabulary
}

func (r *RedactingObserver) Notify(executionID string, line string) {
	if r.inner == nil {
		return
	}
	r.inner.Notify(executionID, r.Redact(line))
}

// Close propaga el cierre al envuelto. `LogObserver` no lo define —lo descubre
// `MultiObserver` con un type assertion— y sin esto un decorador silenciaría el
// flush del observador de Supabase.
func (r *RedactingObserver) Close() {
	if c, ok := r.inner.(interface{ Close() }); ok {
		c.Close()
	}
}

// Redact es la política, expuesta para poder fijarla en un test sin montar un
// observador de mentira detrás.
func (r *RedactingObserver) Redact(line string) string {
	pares := r.pairs()
	if len(pares) == 0 || line == "" {
		return line
	}

	// Un recorrido de izquierda a derecha quedándose con la coincidencia MÁS
	// LARGA en cada posición, en vez de N pasadas de `strings.ReplaceAll`.
	//
	// Resuelve las dos cosas de golpe: la regla 2 de §5.3 —si un valor contiene a
	// otro gana el largo, porque al revés quedaría un reemplazo parcial dentro de
	// otro valor, ilegible y potencialmente revelador del resto— y el efecto
	// cascada que las pasadas sucesivas tienen y nadie pide, donde un valor cuyo
	// texto coincide con el nombre de otra variable acaba reemplazándose DENTRO
	// del `${var.…}` que la pasada anterior escribió.
	var out strings.Builder
	out.Grow(len(line))
	for i := 0; i < len(line); {
		reemplazo, largo := match(line[i:], pares)
		if largo == 0 {
			out.WriteByte(line[i])
			i++
			continue
		}
		out.WriteString(reemplazo)
		i += largo
	}
	return out.String()
}

// pair es un valor conocido y el nombre por el que se le sustituye.
type pair struct {
	name  string
	value string
}

// match devuelve la referencia con la que empieza `s`, si empieza por alguna.
// Los pares llegan ordenados de más largo a más corto, así que el primero que
// encaja es el más largo.
func match(s string, pares []pair) (string, int) {
	for _, p := range pares {
		if strings.HasPrefix(s, p.value) {
			return "${var." + p.name + "}", len(p.value)
		}
	}
	return "", 0
}

// pairs compone el vocabulario redactable de este instante: los valores de al
// menos `MinRedactableLength` bytes, de más largo a más corto.
//
// Se recompone en cada línea porque el mapa CRECE mientras la ejecución corre:
// cachearlo dejaría fuera justo los valores que la ejecución produjo, que son
// los que D-A9 describe. El coste es una copia y una ordenación de unas decenas
// de entradas por línea de log.
func (r *RedactingObserver) pairs() []pair {
	r.mu.Lock()
	vocabulary := r.vocabulary
	r.mu.Unlock()

	if vocabulary == nil {
		return nil
	}

	values := vocabulary.Values()
	pares := make([]pair, 0, len(values))
	for name, value := range values {
		if len(value) < MinRedactableLength {
			continue
		}
		pares = append(pares, pair{name: name, value: value})
	}

	// El desempate por valor y por nombre no es cosmético: dos variables pueden
	// compartir valor —`project_name` y `artifact_name` lo hacen a diario— y sin
	// un orden total la misma línea se redactaría con un nombre u otro según cómo
	// recorriera el mapa el runtime.
	sort.Slice(pares, func(i, j int) bool {
		if len(pares[i].value) != len(pares[j].value) {
			return len(pares[i].value) > len(pares[j].value)
		}
		if pares[i].value != pares[j].value {
			return pares[i].value < pares[j].value
		}
		return pares[i].name < pares[j].name
	})
	return pares
}
