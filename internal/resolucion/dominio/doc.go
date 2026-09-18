// Package dominio contiene el modelo del contexto: sus conceptos, sus reglas y los puertos que necesita.
// Solo usa su propio dominio y la biblioteca estándar.
//
// Contexto: Resolución de Variables: los valores efectivos de un intento, la interpolación y el hash de variable.
// Modelo: docs/modelo/contextos/resolucion.md · Capas: docs/modelo/arquitectura.md.
//
// # Invariantes del modelo
//
// Una variable efectiva pertenece a un Ambito, nunca a un paso: el compartido se ve desde cualquier ambiente,
// el de un ambiente concreto solo se ve desde ese mismo ambiente (Ambito.Ve). El agregado
// VariablesDeUnaInvocacion acumula las variables efectivas mientras dura una invocación — un intento real o
// una petición sin intento, el dominio no distingue entre las dos (DEC-08.3, DEC-04.10).
//
// La precedencia es de dos niveles (DEC-08.4): una producida siempre gana a una declarada, y entre producidas
// gana la del paso más reciente. El agregado lo aplica con dos operaciones distintas: Declarar nunca
// sobreescribe (una declarada llega siempre "por si acaso"), Producir siempre sobreescribe (llamarla en el
// orden en que se ejecutan los comandos basta para que gane la más reciente).
//
// El valor en claro no circula hacia lo publicado (IT-04 DEC-04.7): VariableEfectiva.String() no lo incluye,
// y ningún tipo de internal/resolucion/publicado tiene dónde ponerlo. Solo sale de Valor() hacia dos sitios:
// Interpolar, y la relación reservada de Historial (historial/reservado.Valores), a través de la
// infraestructura de este contexto.
//
// El hash de variable (CalcularHashDeVariable) es simple y sin clave a propósito (DEC-08.8): el mismo valor da
// siempre el mismo hash, en cualquier ambiente de un mismo proyecto (DEC-08.6). No es una promesa de
// seguridad — un valor corto o predecible se puede fuerza-bruta a partir de su hash.
//
// Un paso que no se re-ejecuta no tiene un origen propio: sus variables entran como Producida, con el ámbito
// que tenían cuando se produjeron de verdad — aportan estructuralmente lo mismo que si se hubieran producido
// ahora (ver aplicacion.NoReejecutado).
package dominio
