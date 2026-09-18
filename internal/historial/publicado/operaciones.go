package publicado

import "context"

// Lo que el Historial publica, por cliente (modelo/contextos/historial.md, «Lo que publica»). Cada escritura
// añade registros y nunca cambia uno. Una escritura que falla no ha escrito nada que se pueda leer.

// ParaEjecucion es lo que usa Ejecución de Pipeline.
type ParaEjecucion interface {
	// AbrirIntento ocupa el ambiente y abre el intento, y devuelve su identidad. Si el ambiente tiene otro
	// intento en curso, devuelve *AmbienteOcupadoError.
	AbrirIntento(ctx context.Context, apertura Apertura) (string, error)
	RegistrarComienzo(ctx context.Context, intento, paso string, contenido Contenido) error
	RegistrarFinal(ctx context.Context, intento, paso string, exitoso bool, contenido Contenido) error
	// RegistrarNoReejecucion exige que la evidencia apunte a un final exitoso.
	RegistrarNoReejecucion(ctx context.Context, intento, paso string, evidencia Evidencia, contenido Contenido) error
	// CerrarIntento registra el desenlace y, si el intento llega a despliegue, lo crea y lo anuncia. El
	// destino de un rollback será su padre. Repetir el mismo cierre no escribe otro: completa lo que faltara.
	CerrarIntento(ctx context.Context, intento string, estado Estado, destino string) (Despliegue, bool, error)
	UltimaVezDeUnPaso(ctx context.Context, paso string, ambito Ambito) (RegistroDePaso, bool, error)
	DespliegueYSuIntento(ctx context.Context, despliegue string) (Despliegue, Intento, error)
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

// ParaDiagnostico son las consultas que pide el core. RD-08 las completa con las de su tabla de requisitos.
type ParaDiagnostico interface {
	Intento(ctx context.Context, id string) (Intento, error)
	IntentosDeUnAmbiente(ctx context.Context, ambiente string) ([]Intento, error)
	DesplieguesDeUnAmbiente(ctx context.Context, ambiente string) ([]Despliegue, error)
	UltimoDespliegueConHashDelCodigo(ctx context.Context, ambiente, hash string) (Despliegue, bool, error)
	// CantidadDeIntentos cuenta los intentos del ambiente después del de un despliegue, hasta uno posterior
	// incluido.
	CantidadDeIntentos(ctx context.Context, despliegue, intento string) (int, error)
	VariablesDeUnPaso(ctx context.Context, intento, paso string) ([]Variable, error)
}

// ParaBorde es lo que el motor expone al CLI y al portal.
type ParaBorde interface {
	// AbandonarIntento da por abandonado un intento sin desenlace, y libera su ambiente.
	AbandonarIntento(ctx context.Context, intento string) error
	Intento(ctx context.Context, id string) (Intento, error)
	IntentosDeUnAmbiente(ctx context.Context, ambiente string) ([]Intento, error)
	DesplieguesDeUnAmbiente(ctx context.Context, ambiente string) ([]Despliegue, error)
}

// EscuchaDespliegueRegistrado es quien escucha el evento. Lo registra la raíz de composición: el Historial no
// sabe quién es. Si el aviso se pierde, no se repite (IT-05 DEC-05.4).
type EscuchaDespliegueRegistrado interface {
	DespliegueRegistrado(ctx context.Context, evento DespliegueRegistrado)
}
