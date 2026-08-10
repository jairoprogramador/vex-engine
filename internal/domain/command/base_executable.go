package command

import "errors"

type BaseExecutable struct{}

// Run es el Template Method de las tres cadenas: before → exec → after, y desde
// la spec 19 con un cuarto tiempo, `closing`, que corre SIEMPRE.
//
// `after` está en un `defer` y no detrás del camino feliz (spec 06 §5.1). Es la
// única limpieza que existe —restaurar las plantillas interpoladas en el workdir
// y retirar `step_workdir` del mapa acumulado— y condicionarla al éxito de
// `exec` era dejarla sin hacer justo en el caso que la necesita: el fallo.
//
// Fail-fast y limpieza garantizada no están en conflicto; `defer` los concilia.
// El error de `exec` sigue abortando la cadena, y ahora también sigue siendo el
// que encabeza el resultado.
//
// # `closing` y por qué el par de hechos no puede colgar de `after`
//
// La garantía de la spec 06 es sobre LA LIMPIEZA, y por decisión explícita de su
// §8 **`after` no corre si `before` falló** —`before` es quien crea lo que
// `after` limpia—. Pero `before` puede fallar DESPUÉS de haber emitido: el hecho
// de apertura es su primera línea y lo que viene detrás puede devolver error.
// Colgar `*_started` de `before` y `*_finished` de `after` deja ese camino con un
// par ABIERTO, que es justo el agujero que la spec 19 cierra (§5.2', 06 §9.5).
//
// De las tres salidas que la spec plantea se elige ésta —cerrar desde `Run`—
// porque mantiene la simetría con la 06: **un solo sitio decide que el ciclo se
// cierra**. El precio, declarado, es que `Run` deja de ser un secuenciador puro y
// pasa a conocer que hay un ciclo que cerrar.
//
// `closing` se registra ANTES que `after` para que corra DESPUÉS de él: recibe el
// error definitivo, incluido el de la limpieza, que es el que el hecho tiene que
// contar. Y su propio error se compone con la misma regla —la causa antes que la
// consecuencia— porque un hecho que no se pudo escribir no es un detalle
// cosmético: el registro es la fuente de la que se deriva todo lo demás.
func (b *BaseExecutable) Run(
	executionContext *ExecutionContext,
	before func() error,
	exec func() error,
	after func() error,
	closing func(err error) error,
) (err error) {

	if closing != nil {
		defer func() {
			closingErr := closing(err)
			if closingErr == nil {
				return
			}
			if err == nil {
				err = closingErr
				return
			}
			err = errors.Join(err, closingErr)
		}()
	}

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
