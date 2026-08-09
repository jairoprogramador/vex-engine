// Package deployment es el bounded context de la INTENCIÓN: qué se pretende
// ejecutar, y en qué posición de la historia de un ambiente cae.
//
// Es dominio puro y no escribe un byte. Si algún día aparece aquí una llamada a
// `os.` o a `net/http`, este paquete se salió de su alcance (spec 17 §6).
//
// # Las dos identidades que viven aquí, y la tercera que no
//
//	content_id        ¿QUÉ se pretende ejecutar?          se calcula ANTES de ejecutar
//	deployment_id     ¿en qué POSICIÓN de la historia?    se calcula ANTES de ejecutar
//	step_fingerprint  ¿cambió el trabajo de este step?    vive en `state`/`cache`
//
// **Regla de independencia: ninguno se calcula a partir de otro de una fila
// distinta.** En particular, la decisión de saltar o ejecutar un step NUNCA
// consulta `deployment_id` ni `content_id` —y no podría, porque `deployment_id`
// cambia en cada ejecución nueva al mismo ambiente aunque el step no haya
// cambiado nada—. Lo que una regla de re-ejecución puede mirar es
// `step.RuleSubject`, que tiene tres campos y ninguno es un identificador: la
// independencia la sostiene un tipo, no la disciplina.
//
// # Intención congelada
//
// `Content` es un agregado INMUTABLE cuya identidad ES su contenido: no tiene
// setters, no tiene ciclo de vida, y esa rigidez es la funcionalidad. Lo que NO
// entra —`timestamp`, `actor`, `runner`, `parent`— es la mitad del valor: sin esa
// exclusión el direccionamiento por contenido no existe, porque dos ejecuciones
// idénticas darían identidades distintas y toda comparación se caería.
//
// Qué pasó de verdad —con su instante, su duración y su resultado— es del
// paquete `record`, y son dos modelos porque son dos razones de cambio.
package deployment
