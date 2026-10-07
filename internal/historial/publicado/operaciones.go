package publicado

import "context"

// Lo que el Historial publica, por cliente (modelo/contextos/historial.md, «Lo que publica»). Cada escritura
// añade registros y nunca cambia uno. Una escritura que falla no ha escrito nada que se pueda leer.

// ParaEjecucion es lo que usa Ejecución de Pipeline.
type ParaEjecucion interface {
	// AbrirIntento ocupa el ambiente y abre el intento, y devuelve su identidad. Si el ambiente tiene otro
	// intento en curso, devuelve *AmbienteOcupadoError, salvo que ese intento esté interrumpido: sin latidos
	// nuevos durante la ventana de vida, se cierra como fallido (CausaInterrumpido) y el ambiente se ocupa.
	AbrirIntento(ctx context.Context, apertura Apertura) (string, error)
	RegistrarComienzo(ctx context.Context, intento, paso string, contenido Contenido) error
	RegistrarFinal(ctx context.Context, intento, paso string, exitoso bool, contenido Contenido) error
	// RegistrarNoReejecucion exige que la evidencia apunte a un final exitoso.
	RegistrarNoReejecucion(ctx context.Context, intento, paso string, evidencia Evidencia, contenido Contenido) error
	// RegistrarSalida guarda lo que escribió un comando de un paso al terminar, aparte de los registros del
	// intento. Se registra también la de un comando que falló.
	RegistrarSalida(ctx context.Context, intento, paso, comando string, exitoso bool, texto string) error
	// RegistrarLatido deja constancia de que el proceso del intento sigue vivo, cada IntervaloDeLatido. Quien
	// encuentra el ambiente ocupado y ve que no hay latidos nuevos da el intento por interrumpido y lo cierra.
	// Se rechaza si el intento ya terminó: un intento cerrado o abandonado no sigue escribiendo.
	RegistrarLatido(ctx context.Context, intento string) error
	// CerrarIntento registra el desenlace y, si el intento llega a despliegue, lo crea y lo anuncia. causa es
	// por qué terminó si no fue por un comando (vacía en el caso normal). El destino de un rollback será su
	// padre. Repetir el mismo cierre no escribe otro: completa lo que faltara.
	CerrarIntento(ctx context.Context, intento string, estado Estado, causa CausaDeCierre, destino string) (Despliegue, bool, error)
	// AbandonarIntento da por abandonado un intento sin desenlace y libera su ambiente. Ejecución lo usa para el
	// que abrió y no llegó a empezar; quien invoca lo usa para uno que se quedó colgado (ParaBorde).
	AbandonarIntento(ctx context.Context, intento string) error
	UltimaVezDeUnPaso(ctx context.Context, paso string, ambito Ambito) (RegistroDePaso, bool, error)
	DespliegueYSuIntento(ctx context.Context, despliegue string) (Despliegue, Intento, error)
	// Intento es uno por su identidad, con sus registros: de ahí sale el detalle de lo que hizo.
	Intento(ctx context.Context, id string) (Intento, error)
}

// ParaResolucion es lo que usa Resolución de Variables. El valor, por la relación reservada.
type ParaResolucion interface {
	// RegistrarVariable registra una variable bajo un paso en curso. Su hash va en el contenido.
	RegistrarVariable(ctx context.Context, intento, paso, nombre string, contenido Contenido) error
	UltimaVezDeUnPaso(ctx context.Context, paso string, ambito Ambito) (RegistroDePaso, bool, error)
	// VariablesDeUnPaso es la última de cada nombre, sin valores.
	VariablesDeUnPaso(ctx context.Context, intento, paso string) ([]Variable, error)
}

// ParaLanzamiento es lo que usa Lanzamiento. El evento se escucha con EscuchaDespliegueRegistrado.
type ParaLanzamiento interface {
	RegistrarLanzamiento(ctx context.Context, ambiente, despliegue string, contenido Contenido) (Lanzamiento, error)
	RegistrarReserva(ctx context.Context, ambiente string, reservado bool) error
	UltimoDespliegue(ctx context.Context, ambiente string) (Despliegue, bool, error)
	UltimaReserva(ctx context.Context, ambiente string) (Reserva, bool, error)
	UltimoLanzamiento(ctx context.Context, ambiente string) (Lanzamiento, bool, error)
	// HashDelCodigoDeUnDespliegue es el hash del código con el que se hizo un despliegue.
	HashDelCodigoDeUnDespliegue(ctx context.Context, despliegue string) (string, error)
	// TodosLosLanzamientos son todos, de todos los ambientes, en el orden en que se registraron. Lanzamiento
	// los recorre para decidir la versión de un código (IT-10 DEC-10.8): el Historial no puede, porque su
	// contenido es opaco (DEC-03.13).
	TodosLosLanzamientos(ctx context.Context) ([]Lanzamiento, error)
}

// ParaDiagnostico son las consultas que pide el core (RD-08).
type ParaDiagnostico interface {
	Intento(ctx context.Context, id string) (Intento, error)
	IntentosDeUnAmbiente(ctx context.Context, ambiente string) ([]Intento, error)
	DesplieguesDeUnAmbiente(ctx context.Context, ambiente string) ([]Despliegue, error)
	UltimoDespliegueConHashDelCodigo(ctx context.Context, ambiente, hash string) (Despliegue, bool, error)
	// CantidadDeIntentos cuenta los intentos del ambiente después del de un despliegue, hasta uno posterior
	// incluido.
	CantidadDeIntentos(ctx context.Context, despliegue, intento string) (int, error)
	VariablesDeUnPaso(ctx context.Context, intento, paso string) ([]Variable, error)
	// Lanzamiento es uno por su identidad, para entrar por ES-3: de cada lanzamiento, su despliegue.
	Lanzamiento(ctx context.Context, id string) (Lanzamiento, error)
	// Despliegue es uno por su identidad, para cuando el usuario elige la referencia a mano (DEC-06.6).
	Despliegue(ctx context.Context, id string) (Despliegue, error)
}

// ParaCatalogo es lo que usa Catálogo: si un ambiente está reservado.
type ParaCatalogo interface {
	UltimaReserva(ctx context.Context, ambiente string) (Reserva, bool, error)
}

// ParaBorde es lo que el motor expone al CLI y al portal.
type ParaBorde interface {
	// AbandonarIntento da por abandonado un intento sin desenlace, y libera su ambiente.
	AbandonarIntento(ctx context.Context, intento string) error
	Intento(ctx context.Context, id string) (Intento, error)
	IntentosDeUnAmbiente(ctx context.Context, ambiente string) ([]Intento, error)
	DesplieguesDeUnAmbiente(ctx context.Context, ambiente string) ([]Despliegue, error)
	// SalidasDeUnIntento son las de sus comandos, en el orden en que terminaron. Si el intento no existe,
	// devuelve ErrNoExiste.
	SalidasDeUnIntento(ctx context.Context, intento string, filtro FiltroDeSalidas) ([]Salida, error)
	// UltimoIntento es el último que se abrió, en cualquier ambiente. No hay ninguno: false.
	UltimoIntento(ctx context.Context) (Intento, bool, error)
}

// EscuchaDespliegueRegistrado es quien escucha el evento. Lo registra la raíz de composición: el Historial no
// sabe quién es. Si el aviso se pierde, no se repite (IT-05 DEC-05.4).
type EscuchaDespliegueRegistrado interface {
	DespliegueRegistrado(ctx context.Context, evento DespliegueRegistrado)
}
