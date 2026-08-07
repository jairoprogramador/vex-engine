// Package state es el bounded context del ESTADO de un step: qué dejó cada
// ejecución real de un step, bajo su clave de posición, para siempre.
//
// Hasta la spec 11 el estado era un VALOR sobrescribible —`store/<pipeline>/
// <scope>/<step>.vars`, reescrito tras cada step exitoso—. Aquí pasa a ser un
// HECHO FECHADO: un `StepRecord` por ejecución real, añadido bajo
// `(subject, scope, step_id)` y nunca sobrescrito.
//
// La diferencia no es de almacenamiento. Un `.vars` responde «¿cuánto vale X?»;
// un registro responde «¿qué dejó esta ejecución?», que es una pregunta con
// sujeto — y es la única forma de la que cuelgan el rollback (spec 28), la
// trazabilidad (spec 17) y la evidencia reutilizada.
//
// # La regla de vida
//
// Esta tienda NO es desechable. El índice de `domain/cache` sí lo es, y esa
// asimetría es deliberada: el índice se reconstruye recorriendo los registros;
// un registro perdido no se reconstruye con nada, porque describe un efecto que
// ocurrió en el mundo.
//
// # Criterio de clasificación (P8, spec 11 §2) — NORMATIVO, todavía sin código
//
// El día que el pipelinecode pueda declarar la naturaleza de cada output
// (spec 14), la pregunta con la que se clasifica una variable es ésta, y está
// escrita aquí para que quien la retome no tenga que redescubrirla:
//
// **Si se borra el almacén y el step vuelve a correr desde cero, ¿el valor sale
// idéntico, o puede salir distinto?**
//
// DETERMINISTA —un hash de artefacto, un cálculo sobre las entradas del step—
// sería desechable. DEPENDE DE UN EFECTO REAL —un ARN, una IP asignada, un
// recurso creado en la nube— NO: borrar no puede implicar perder de vista un
// recurso que sigue existiendo, ni crear uno duplicado la próxima vez.
//
// Mientras no haya información para clasificar, **todo** se guarda aquí:
// **ante la duda, no desechable**. El coste de guardar de más es espacio; el de
// guardar de menos es un recurso duplicado en la nube. No son comparables.
package state
