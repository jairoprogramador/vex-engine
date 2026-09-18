package reservado

import "context"

// Valores es la relación reservada: el valor de una variable va al Historial, que lo guarda ofuscado, y solo
// vuelve a Resolución de Variables (IT-04 DEC-04.7).
type Valores interface {
	// GuardarValor registra el valor de una variable bajo un paso en curso.
	GuardarValor(ctx context.Context, intento, paso, nombre, valor string) error
	// ValoresDeUnPaso es el último valor de cada variable de un paso.
	ValoresDeUnPaso(ctx context.Context, intento, paso string) (map[string]string, error)
}
