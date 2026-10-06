package dominio

import (
	"context"
	"time"
)

// EstadoDeIntento es cómo terminó un intento, o vacío si está sin desenlace.
type EstadoDeIntento string

const (
	Exitoso   EstadoDeIntento = "exitoso"
	Fallido   EstadoDeIntento = "fallido"
	Cancelado EstadoDeIntento = "cancelado"
)

func (e EstadoDeIntento) SinDesenlace() bool { return e == "" }

// IntentoDeDiagnostico es un intento en lo que este contexto necesita de él.
type IntentoDeDiagnostico struct {
	Id       IdIntento
	Ambiente Ambiente
	Instante time.Time
	Estado   EstadoDeIntento
	// Interrumpido: su proceso murió sin cerrarlo y otro intento lo cerró como fallido. Falló, pero no por el
	// pipeline: buscarle causa en el código o las variables daría un diagnóstico engañoso.
	Interrumpido bool
}

// NoSeAtribuye: ES-7. Un intento sin desenlace, cancelado o interrumpido no tiene causa que buscar: de uno no se
// sabe cómo terminó, y los otros dos no fallaron por nada que el pipeline haya cambiado.
func (i IntentoDeDiagnostico) NoSeAtribuye() bool {
	return i.Estado.SinDesenlace() || i.Estado == Cancelado || i.Interrumpido
}

// Historial es lo que la aplicación necesita del Historial, en el lenguaje de este dominio. Todo el I/O
// vive aquí (DEC-06.14): los servicios de dominio solo deciden con datos ya obtenidos.
type Historial interface {
	Intento(ctx context.Context, id IdIntento) (IntentoDeDiagnostico, error)
	// UltimoIntentoDeUnAmbiente es el que se toma cuando no se indica ninguno (DEC-06.16).
	UltimoIntentoDeUnAmbiente(ctx context.Context, ambiente Ambiente) (IdIntento, bool, error)
	// DespliegueDeUnLanzamiento es la puerta de entrada de ES-3.
	DespliegueDeUnLanzamiento(ctx context.Context, lanzamiento IdLanzamiento) (IdDespliegue, error)
	Despliegue(ctx context.Context, id IdDespliegue) (DespliegueDeDiagnostico, error)
	// UltimoDespliegueAnteriorA es la referencia por defecto del mismo ambiente (DEC-06.6).
	UltimoDespliegueAnteriorA(ctx context.Context, ambiente Ambiente, antesDe time.Time) (DespliegueDeDiagnostico, bool, error)
	// EjesDelIntento y Referencia son la factoría del ACL (DEC-06.5, DEC-06.13): siguen la evidencia y
	// separan declaradas de producidas.
	EjesDelIntento(ctx context.Context, intento IdIntento) ([]EjesDeUnPaso, []ProducidasDeUnPaso, error)
	Referencia(ctx context.Context, despliegue DespliegueDeDiagnostico, razon RazonDeReferencia) (Referencia, []ProducidasDeUnPaso, error)
	// CantidadDeIntentos es para ES-8.
	CantidadDeIntentos(ctx context.Context, despliegue IdDespliegue, intento IdIntento) (int, error)
}
