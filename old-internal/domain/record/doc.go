// Package record es el bounded context de la CIRCUNSTANCIA: qué pasó, paso a
// paso, mientras se intentaba ejecutar un despliegue.
//
// Es dominio puro y no escribe un byte. El sink que los persiste es de la
// spec 19 y el que los empuja de la 21; aquí no hay ni un `os.` ni un
// `net/http`.
//
// # Hechos, nunca conclusiones
//
// Es la disciplina que gobierna todo el paquete y se aplica hasta donde duele:
// `ErrorClass` tiene `unknown` como valor LEGÍTIMO y preferible a forzar una
// clasificación, y un step que se ejecutó porque no se pudo componer su huella
// no es «se ejecutó porque cambió» — eso sería guardar una conclusión falsa.
//
// # Por qué event sourcing, y no un JSON con el resultado
//
// Porque **una ejecución interrumpida debe dejar datos útiles**, y en este motor
// la interrupción es el caso normal y no el excepcional: en remoto la máquina es
// efímera, en local el usuario mata el contenedor. Un JSON escrito al final
// pierde la ejecución entera; una tira de hechos append-only dice exactamente
// hasta dónde se llegó.
//
// De ahí sale la propiedad central: **el resultado no se guarda, se deriva**.
// `Fold` es una función pura sobre un slice, sin estado y sin dependencias, y
// eso es lo que permite ejecutarla idéntica en el motor y en el backend
// (spec 26). Guardar además el resultado sería tener dos fuentes de verdad para
// el mismo dato, que divergen — por eso se descarta Memento.
//
// # El vocabulario es cerrado
//
// Por la misma razón que las clases de fallo: **texto libre no es agregable**.
// Es la diferencia entre «3 fallos por `source_unavailable`» y no poder contar
// nada. Cerrado en VALORES y abierto en EXTENSIÓN: añadir un tipo de evento no
// toca `Fold` para los existentes.
package record
