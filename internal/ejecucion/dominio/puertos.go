package dominio

import (
	"context"
	"io"
	"time"
)

// Pipelines es lo que la aplicación necesita de Definición de Pipeline: el pipeline comprobado, de hoy o de un
// commit (para un rollback, DEC-03.9), y las variables estándar que declara sin resolver.
type Pipelines interface {
	DeHoy(ctx context.Context, fuente string) (Pipeline, error)
	DeUnCommit(ctx context.Context, fuente, commit string) (Pipeline, error)
	VariablesEstandar() []VariableEstandar
}

// Pipeline es el pipeline comprobado, en lo que este contexto necesita de él: el commit con el que se resolvió
// —el mismo que hay que guardar en la apertura del intento para que un rollback futuro lo pueda pedir de
// nuevo, DEC-03.9—, sus pasos, en orden, y el orden de sus ambientes, que este contexto no usa para nada
// propio: solo lo lleva hasta la apertura del intento para que quede en el Historial y Diagnóstico pueda
// encontrar el ambiente anterior (DEC-06.6).
type Pipeline struct {
	Commit    string
	Ambientes []string // el valor de cada ambiente, en su orden
	Pasos     []PasoDeEjecucion
}

// PasoDeEjecucion es un paso del pipeline con todo lo que un intento necesita de él: su lugar en el orden y su
// ámbito (PasoDelPipeline), su regla de re-ejecución, sus comandos y el material de su directorio.
type PasoDeEjecucion struct {
	PasoDelPipeline
	Regla    Regla
	Comandos []ComandoDeclarado
	Material []FicheroDeclarado
}

// VariableEstandar es una variable que el pipeline usa sin declararla: el motor garantiza que está siempre.
// Definición solo dice su nombre y su comportamiento; el valor lo da la aplicación de Ejecución, que es quien
// invoca a Resolución (cierra el puente que resolucion/publicado.VariablesDeUnPaso deja abierto hasta esta
// unidad).
type VariableEstandar struct {
	Nombre   string
	Metadato bool
	DelPaso  bool
}

// Fuentes es lo que la aplicación necesita de Suministro de Fuentes: el material del código del proyecto, de
// hoy, de un commit o de una copia de trabajo (DEC-10.7), con su hash.
type Fuentes interface {
	TraerDeHoy(ctx context.Context, fuente string) (Material, error)
	TraerDeUnCommit(ctx context.Context, fuente, commit string) (Material, error)
	TraerCopiaDeTrabajo(ctx context.Context, directorio string) (Material, error)
	Retirar(ctx context.Context, material Material) error
}

// Material es lo que Suministro pone delante: su propio directorio, para copiarlo al espacio de trabajo, su
// hash y el commit con el que se trabaja — vacío si es una copia de trabajo.
type Material struct {
	Directorio string
	Hash       HashDeCodigo
	Commit     string
}

// Variables es lo que la aplicación necesita de Resolución de Variables durante un intento real.
type Variables interface {
	// DeclararVariablesDeUnPaso declara, en el ámbito del paso, las variables estándar (con los valores que da
	// estandar) y los literales del pipeline visibles desde ese ámbito.
	DeclararVariablesDeUnPaso(
		ctx context.Context, intento, paso string, ambito Ambito, fuente, commit string, estandar map[string]string,
	) error
	Interpolar(ctx context.Context, intento, paso string, ambito Ambito, texto string) (string, error)
	// HashDeLasVariables resume las variables que el paso consume, visibles desde ambito: las que referencian
	// textos (ver TextosInterpolables). La regla lo compara con el que se guardó la última vez que el paso se
	// ejecutó.
	HashDeLasVariables(ctx context.Context, intento string, ambito Ambito, textos []string) (HashDeVariables, error)
	RegistrarProducido(ctx context.Context, intento, paso, nombre, valor string, ambito Ambito) error
	NoReejecutado(ctx context.Context, intento, paso string, ambito Ambito) error
}

// Historial es lo que la aplicación necesita de Historial de Cambios, en el lenguaje de este dominio.
type Historial interface {
	// AbrirIntento ocupa el ambiente y abre el intento. Si el ambiente tiene otro intento en curso, el error
	// envuelve *AmbienteOcupadoError (traducido de historial/publicado, sin que este dominio dependa de él).
	AbrirIntento(ctx context.Context, apertura AperturaDeIntento) (string, error)
	RegistrarComienzo(ctx context.Context, intento, paso string, recursos RecursosDeUnPaso) error
	RegistrarFinal(ctx context.Context, intento, paso string, exitoso bool, recursos RecursosDeUnPaso) error
	RegistrarNoReejecucion(ctx context.Context, intento, paso string, evidencia Evidencia, recursos RecursosDeUnPaso) error
	// RegistrarSalida guarda lo que escribió un comando de un paso al terminar, y si salió bien. Se guarda
	// también la de un comando que falló o que se canceló.
	RegistrarSalida(ctx context.Context, intento, paso, comando string, exitoso bool, texto string) error
	// IntervaloDeLatido es cada cuánto tiene que latir un intento en curso: lo fija el Historial, que es quien
	// decide cuánto espera antes de dar a un dueño por muerto. Cero desactiva el latido.
	IntervaloDeLatido() time.Duration
	// Latir deja constancia de que el proceso del intento sigue vivo. Es lo que permite a otro intento saber, si
	// encuentra el ambiente ocupado, que el dueño murió y puede cerrarlo.
	Latir(ctx context.Context, intento string) error
	// CerrarIntento registra el desenlace y, si el intento llega a despliegue, lo crea. causa es por qué terminó
	// si no fue por un comando (vacía en el caso normal). destino es el despliegue padre de un rollback, y vacío
	// si no lo es.
	CerrarIntento(ctx context.Context, intento string, desenlace Desenlace, causa Causa, destino string) (despliegue string, huboDespliegue bool, err error)
	// AbandonarIntento da por abandonado un intento que se abrió y no llegó a empezar —el espacio de trabajo no
	// se pudo preparar, EJ-5—, y libera su ambiente. No es un intento fallido: nunca ejecutó nada.
	AbandonarIntento(ctx context.Context, intento string) error
	UltimaVezDeUnPaso(ctx context.Context, paso string, ambito Ambito) (UltimaVezDeUnPaso, error)
	// DespliegueParaRollback da lo que EJ-2 necesita de un despliegue destino: su ambiente y las dos fuentes
	// con las que se hizo, ya decodificadas de su Contenido.
	DespliegueParaRollback(ctx context.Context, despliegue string) (Destino, error)
	// DetalleDelIntento arma, con lo que el Historial ya guarda del intento, cuánto tardó y qué pasó con cada
	// paso. No se escribe nada para esto.
	DetalleDelIntento(ctx context.Context, intento string) (DetalleDelIntento, error)
}

// AperturaDeIntento es lo que un intento declara al abrirse.
type AperturaDeIntento struct {
	Ambiente          string
	Solicitante       string
	Pasos             []PasoDelPipeline // todos los del pipeline, en orden
	HastaPaso         string
	ConCommits        bool
	HashDelCodigo     HashDeCodigo
	FuenteDelProyecto string
	CommitDelProyecto string
	FuenteDelPipeline string
	CommitDelPipeline string
	// OrdenDeAmbientes es el orden declarado en environments.yaml, para que Diagnóstico encuentre el
	// ambiente anterior (DEC-06.6). No lo usa este contexto.
	OrdenDeAmbientes []string
}

// Comandos ejecuta un comando en el espacio de trabajo de su ambiente y recoge su resultado. La salida se
// reenvía en vivo, tal cual, mientras el comando corre (DEC-12.5); io.Writer es lo mínimo que necesita, y ya lo
// entiende la biblioteca estándar.
//
// lineaInterpolada es la línea con sus ${var.<nombre>} ya resueltos — lo que de verdad corre. comando sigue
// siendo la forma declarada, sin resolver: de ahí salen su directorio y las expresiones con las que se
// comprueban sus aserciones y se capturan sus variables de salida, que nunca se interpolan. entorno son las
// variables de entorno que quien invoca pidió: se añaden a las que el proceso ya tiene (y las pisan), solo para
// este comando.
type Comandos interface {
	Ejecutar(
		ctx context.Context, directorio, lineaInterpolada string, comando ComandoDeclarado, entorno Entorno, salida io.Writer,
	) (ResultadoDeUnComando, error)
}

// EspacioDeTrabajo rehace la parte del motor: el material declarado, copiado e interpolado, sin tocar nunca lo
// que los comandos escriben fuera de los directorios de los pasos (DEC-09.5, DEC-06.19).
type EspacioDeTrabajo interface {
	// Ubicar da el lugar del espacio de trabajo de un ambiente: proyecto y pipeline salen de sus fuentes —la URL
	// del remoto «origin» si la tienen, o el nombre de su directorio—, no de los metadatos de quien invoca.
	// Siempre da un nombre válido o un error: nunca un nombre vacío.
	Ubicar(fuenteDelProyecto, fuenteDelPipeline, ambiente string) (Ubicacion, error)
	// RehacerParteDelMotor copia el material declarado de todos los pasos, entero, al empezar el intento. Si el
	// ambiente no se puede alcanzar, el error envuelve ErrNoDisponible (EJ-5, DEC-06.18).
	RehacerParteDelMotor(ctx context.Context, u Ubicacion, pasos []PasoDeEjecucion) error
	// DirectorioDelPaso es donde corren los comandos de un paso.
	DirectorioDelPaso(u Ubicacion, paso string) string
	// InterpolarPlantillas reescribe, en su sitio, los ficheros que el paso marca como plantilla. Se llama
	// justo antes de ejecutar sus comandos, para que puedan usar variables producidas por pasos anteriores.
	InterpolarPlantillas(ctx context.Context, u Ubicacion, paso PasoDeEjecucion, interpolar Interpolador) error
}

// Interpolador sustituye cada ${var.<nombre>} por su valor. Lo da Resolución, a través de la aplicación.
type Interpolador func(texto string) (string, error)
