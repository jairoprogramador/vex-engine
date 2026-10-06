package dominio

// Desenlace es cómo termina un Intento en curso: exitoso, fallido o cancelado, y una sola vez.
type Desenlace int

const (
	Exitoso Desenlace = iota + 1
	Fallido
	Cancelado
)

var nombresDeDesenlace = map[Desenlace]string{
	Exitoso:   "exitoso",
	Fallido:   "fallido",
	Cancelado: "cancelado",
}

func (d Desenlace) String() string {
	if nombre, ok := nombresDeDesenlace[d]; ok {
		return nombre
	}
	return "desenlace desconocido"
}

// Causa es por qué un intento termina sin que un comando lo decida; vacía si el desenlace salió de los comandos.
type Causa string

// CausaError: algo impidió seguir —interpolar, un puerto que falló— sin que ningún comando fallara (EJ-6).
const CausaError Causa = "error"

// IntentoEnCurso es el intento mientras se lleva a cabo (DEC-09.2): recorre sus pasos en orden y hace cumplir,
// por construcción, las cinco invariantes del modelo (docs/modelo/contextos/ejecucion.md, «Agregado: Intento en
// curso»):
//
//   - orden: SiguientePaso() solo puede dar el paso que toca, y Completar exige que sea ese.
//   - nada sin escribir: el agregado no hace I/O — es la aplicación quien solo llama a Completar después de
//     confirmar el registro del paso en el Historial, así que un paso siguiente es inalcanzable sin él.
//   - un fallo cierra: Completar(paso, false) fija el fallo —o Fallar(), si lo que impide seguir es un error y
//     no un comando—, y desde ahí SiguientePaso() no vuelve a dar nada.
//   - la cancelación gana: Cancelar() se puede llamar en cualquier momento, incluso después de un fallo que
//     ella misma provocó, y Desenlace() la prioriza siempre.
//   - un solo desenlace: Desenlace() es una función pura sobre un estado que, una vez fijado, no cambia.
//
// No tiene protección de concurrencia: un intento no lo comparte más de un hilo (DEC-09.2).
type IntentoEnCurso struct {
	ambiente  string
	pasos     []PasoDelPipeline // todos los del pipeline, en orden
	limite    int               // índice, en pasos, del último paso a intentar (inclusive)
	siguiente int               // índice del próximo paso a dar en SiguientePaso
	fallo     bool
	cancelado bool
}

// NuevoIntentoEnCurso abre un intento sobre todos los pasos del pipeline, hasta hastaPaso inclusive. hastaPaso
// vacío es hasta el último — así EJ-2 (rollback, que siempre hace todos los pasos) no necesita un caso aparte.
func NuevoIntentoEnCurso(ambiente string, pasos []PasoDelPipeline, hastaPaso string) (*IntentoEnCurso, error) {
	if ambiente == "" {
		return nil, invalido("un intento no puede tener el ambiente vacío")
	}
	if len(pasos) == 0 {
		return nil, invalido("un intento necesita, al menos, un paso")
	}
	limite := len(pasos) - 1
	if hastaPaso != "" {
		indice := indiceDelPaso(pasos, hastaPaso)
		if indice < 0 {
			return nil, invalido("%q no es un paso de este pipeline", hastaPaso)
		}
		limite = indice
	}
	return &IntentoEnCurso{ambiente: ambiente, pasos: pasos, limite: limite}, nil
}

func indiceDelPaso(pasos []PasoDelPipeline, nombre string) int {
	for i, p := range pasos {
		if p.Nombre() == nombre {
			return i
		}
	}
	return -1
}

func (i *IntentoEnCurso) Ambiente() string { return i.ambiente }

// Pasos son todos los del pipeline, en orden — lo que hace falta para abrir el intento en el Historial.
func (i *IntentoEnCurso) Pasos() []PasoDelPipeline { return i.pasos }

// SiguientePaso da el próximo paso a intentar, o false si el intento ya se detuvo (por un fallo o una
// cancelación) o ya dio todos los pasos pedidos.
func (i *IntentoEnCurso) SiguientePaso() (PasoDelPipeline, bool) {
	if i.fallo || i.cancelado || i.siguiente > i.limite {
		return PasoDelPipeline{}, false
	}
	return i.pasos[i.siguiente], true
}

// Completar registra que un paso terminó, con éxito o no, y avanza el intento. Rechaza un paso que no es el que
// tocaba: eso es un error de quien llama, nunca una decisión de negocio.
func (i *IntentoEnCurso) Completar(paso string, exitoso bool) error {
	if i.fallo || i.cancelado {
		return rechazo("%q: el intento ya se detuvo, no acepta más pasos", paso)
	}
	if i.siguiente > i.limite || i.pasos[i.siguiente].Nombre() != paso {
		return rechazo("%q no es el paso que tocaba", paso)
	}
	i.siguiente++
	if !exitoso {
		i.fallo = true
	}
	return nil
}

// Fallar da el intento por fallido cuando lo que impide seguir no es un comando que salió mal sino un error
// (interpolar, un puerto que falla): sin él, el intento quedaría sin desenlace y su ambiente ocupado. Como
// Cancelar, se puede llamar en cualquier momento, y la cancelación sigue ganando en Desenlace().
func (i *IntentoEnCurso) Fallar() { i.fallo = true }

// Cancelar pide la cancelación. Es idempotente y se puede llamar en cualquier momento — incluso después de que
// ella misma haya hecho fallar el paso en curso — porque la cancelación siempre gana en Desenlace().
func (i *IntentoEnCurso) Cancelar() { i.cancelado = true }

// Desenlace da cómo terminó el intento, y false si todavía no hay uno: ni se canceló, ni falló, ni llegó al
// último paso pedido.
func (i *IntentoEnCurso) Desenlace() (Desenlace, bool) {
	switch {
	case i.cancelado:
		return Cancelado, true
	case i.fallo:
		return Fallido, true
	case i.siguiente > i.limite:
		return Exitoso, true
	default:
		return 0, false
	}
}
