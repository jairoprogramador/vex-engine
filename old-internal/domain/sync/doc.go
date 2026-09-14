// Package sync es el EMPUJE de lo registrado hacia el destino configurado.
//
// # Qué se empuja, y qué no
//
// Sólo dos de las tiendas: `objects/` y `events/`. Las otras tres del destino no
// pasan por aquí, y la diferencia no es de gusto sino de QUIÉN LAS LEE
// (spec 21 §5.1):
//
//	state/  cache/  lineage/  →  van DIRECTAS al destino. Hay que leerlas ANTES
//	                             de decidir si un step revive o en qué posición
//	                             de la historia cae este despliegue; un búfer que
//	                             se empuja al terminar el step llegaría tarde a
//	                             su propia pregunta.
//	objects/  events/         →  nadie las lee durante la ejecución, así que se
//	                             bufferizan en el área de trabajo y se empujan
//	                             desde aquí. Es lo que permite que un fallo de
//	                             red no pare el pipeline.
//	keys/                     →  NO es registro. Es el secreto local del que se
//	                             deriva la clave de resumen (spec 20 §5.2) y no
//	                             está en la lista de nadie: ni se lee para
//	                             decidir, ni se empuja a ninguna parte.
//
// Quien escriba un `Sink` tiene que poder decir que `keys/` no está en su lista,
// igual que puede decirlo de `.clones/` y `.copias/`, que son del área de clones
// y son derivables.
//
// # Registrar es incondicional; empujar es lo que este paquete añade
//
// La escritura local ocurre pase lo que pase (spec 19 §5.6). Un destino que
// falla produce un `sync_failed` y devuelve el control: **el registro nunca
// puede hacer fallar lo que observa**. Es lo contrario del criterio del
// `attempt_finished`, y la asimetría es deliberada — allí lo que se pierde es la
// historia entera del intento, aquí lo que se pierde es una copia de algo que
// sigue estando en disco.
package sync
