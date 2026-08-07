package command

// Origin dice de dónde llegó el valor de una variable, y con ello quién gana
// cuando dos fuentes aportan el mismo nombre.
//
// **El orden del `iota` ES la precedencia.** Que sea un valor ordenado y no un
// conjunto de banderas es deliberado: hace la regla comparable con `>=` e
// imposible de expresar a medias (spec 12 §5.1).
//
// Hasta la spec 12 la precedencia era «el último handler en escribir gana», o
// sea una propiedad emergente del orden de `chainStepHandlers` en
// `internal/interfaces/cli/factory.go`: una regla de negocio que solo se podía
// leer siguiendo el cableado de la infraestructura. Ahora vive en `Add`, y el
// orden de la cadena pasa a ser una optimización de cuándo se carga cada cosa.
type Origin uint8

const (
	// OriginDeclared es el literal de `variables/<ambiente>/<paso>.yaml`: lo que
	// el autor sabía AL ESCRIBIR el pipeline. Es un valor por DEFECTO, y por eso
	// va abajo del todo (P3).
	OriginDeclared Origin = iota

	// OriginState es lo que una ejecución ANTERIOR dejó en el almacén, leído del
	// último registro de la clave de posición del step: primero el ámbito de
	// proyecto, luego el del ambiente.
	//
	// Son dos cargas mientras sean dos CLAVES (spec 11); cuando la spec 13 le dé
	// al step un solo ámbito será una, y este enum no cambia por ello: lo que
	// cambia es cuántas veces se alimenta esta posición del orden.
	OriginState

	// OriginInjected es lo que el motor DERIVA de la ejecución en curso
	// (`project_revision`, `project_workdir`, `step_workdir`…). Va por encima del
	// almacén: un hecho de ESTA ejecución no puede ser pisado por uno de una
	// corrida anterior.
	//
	// Hasta la spec 12 la protección era la lista de volátiles —que no se
	// persisten— y no el orden; y esa lista ya demostró ser frágil (spec 11 §b).
	OriginInjected

	// OriginResolved es lo que satisfizo una DECLARACIÓN del consumidor: la
	// variable que dice `resolve: step-output` o `resolve: state` y nombra su
	// fuente (spec 14 §5.2).
	//
	// Su POSICIÓN importa poco y su EXISTENCIA importa mucho. Poco, porque una
	// variable con `resolve` no compite: tiene un único proveedor por
	// construcción, así que nunca hay dos candidatos para su nombre. Mucho,
	// porque es lo que permite decir «esta la resolvió una declaración» sin mirar
	// el cableado — y sobre eso se apoya que su VALOR no entre en la identidad y
	// sí lo haga su declaración (§5.3).
	//
	// Va por encima del almacén y de lo inyectado —una declaración explícita gana
	// a lo que llega por ambiente— y por debajo de runtime, que sigue ganando
	// siempre: lo que el mundo real devolvió al ejecutar no lo puede pisar una
	// resolución hecha antes de ejecutarlo.
	OriginResolved

	// OriginRuntime es lo extraído del stdout de un comando: lo que el mundo real
	// devolvió AL EJECUTAR. Siempre gana.
	OriginRuntime
)

func (o Origin) String() string {
	switch o {
	case OriginDeclared:
		return "declared"
	case OriginState:
		return "state"
	case OriginInjected:
		return "injected"
	case OriginResolved:
		return "resolved"
	case OriginRuntime:
		return "runtime"
	default:
		return "unknown"
	}
}
