package command

import "errors"

type BaseExecutable struct{}

// Run es el Template Method de las tres cadenas: before → exec → after.
//
// `after` está en un `defer` y no detrás del camino feliz (spec 06 §5.1). Es la
// única limpieza que existe —restaurar las plantillas interpoladas en el workdir
// y retirar `step_workdir` del mapa acumulado— y condicionarla al éxito de
// `exec` era dejarla sin hacer justo en el caso que la necesita: el fallo.
//
// Fail-fast y limpieza garantizada no están en conflicto; `defer` los concilia.
// El error de `exec` sigue abortando la cadena, y ahora también sigue siendo el
// que encabeza el resultado.
func (b *BaseExecutable) Run(
	executionContext *ExecutionContext,
	before func() error,
	exec func() error,
	after func() error,
) (err error) {

	if before != nil {
		// `after` NO corre si `before` falló: `before` es quien crea lo que
		// `after` limpia (las FileInterpolatorSession). Restaurar sesiones que
		// no se abrieron no es limpieza, es operar sobre estado inexistente.
		if err := before(); err != nil {
			return err
		}
	}

	if after != nil {
		defer func() {
			afterErr := after()
			if afterErr == nil {
				return
			}
			// La causa antes que la consecuencia: quien vio fallar su
			// `terraform apply` necesita leer primero ese fallo, no el de la
			// restauración que vino después. El de `after` se acompaña vía
			// errors.Join, así que sigue siendo inspeccionable con errors.Is.
			if err == nil {
				err = afterErr
				return
			}
			err = errors.Join(err, afterErr)
		}()
	}

	return exec()
}
