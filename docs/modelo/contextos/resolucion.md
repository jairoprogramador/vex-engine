# Resolución de Variables — modelo del contexto

> **Supporting.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; sus relaciones, en `context-map.md` (filas #3, #5, #6 y #8).
>
> **Vigente** desde el cierre de IT-08 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Con qué valor concreto se despliega cada variable aquí, y si cambió, sin que el valor circule.** Es el
único sitio donde un valor se convierte en hash de variable (`DEC-03.5`), y, junto con el comando que se
ejecuta, el único que tiene un valor en claro.

---

## Lo que le piden

| Quién | Qué necesita |
|---|---|
| **Ejecución** | las variables de un paso · interpolar un comando o el material de un paso · *¿cambiaron las variables de este paso desde su última vez?* · registrar lo que produjo un comando |
| **Simulación** | interpolar en una petición sin intento (`DEC-04.10`) |

| Lee de | Qué |
|---|---|
| **Definición** | las variables declaradas · las formas de ámbito y de variable de salida |
| **Historial** | el hash y el valor de la última vez de cada variable. El valor, por la relación reservada (`DEC-04.7`) |

Y **escribe en el Historial** el hash de cada variable, bajo paso e intento, y el valor ofuscado, por la
relación reservada.

---

## Modelo táctico

### Agregado: **Variables de un intento**

**Las variables efectivas de cada paso de un intento**, o de una petición sin intento. Vive mientras dura
la invocación (`DEC-08.3`), y lo que tiene que quedar se escribe en el Historial, un registro por hecho.

| Invariante | Qué dice |
|---|---|
| ámbito | un paso solo ve las variables de su ámbito: su ambiente o el compartido (`DEC-06.12`) |
| orden | una variable producida solo existe para los pasos posteriores al que la produjo |
| precedencia | si dos orígenes dan valor al mismo nombre, **gana la producida**; entre producidas, **la del paso más reciente** (`DEC-08.4`) |
| paso que no se re-ejecuta | aporta las variables que produjo la última vez, con su valor pedido al Historial por la relación reservada |
| el valor no circula | solo sale hacia el comando que se ejecuta y, ofuscado, hacia el Historial |
| sin intento | no calcula hashes, no escribe nada y no conserva nada (`DEC-04.10`) |

### Value objects

| | Qué es |
|---|---|
| **variable efectiva** | nombre, valor, origen y ámbito. El valor es sensible y **no se muestra nunca** |
| **origen** | declarada · producida |
| **ámbito** | su ambiente · compartido |
| **hash de variable** | dice si una variable cambió sin mostrar su valor. **Comparable entre ambientes de un mismo proyecto** (`DEC-08.6`). **Simple, sin clave**: no es una garantía de seguridad (`DEC-08.8`) |

### Servicios de dominio

| Servicio | Qué hace |
|---|---|
| **interpolación** | sustituye cada variable por su valor dentro de un comando o del material de un paso |
| **calcular el hash de una variable** | el único punto donde un valor se convierte en hash |
| **¿cambiaron las variables de un paso?** | compara los hashes de ahora con los de la última vez del paso en su ámbito |

### Factoría

**Las variables de un paso** se construyen a partir de tres cosas: las declaradas (de Definición), las
producidas en este intento (de Ejecución) y las de la última vez de los pasos que no se re-ejecutan (del
Historial).

### Repositorio

**Ninguno propio.** Lo que se conserva va al Historial, adoptando su forma (Conformist) y con la relación
reservada para el valor (`context-map.md` fila #3).

### Eventos de dominio

Ninguno.

### Lo que publica

| Hacia | Qué |
|---|---|
| **Ejecución** *(Customer–Supplier)* | las variables de un paso · interpolar · *¿cambiaron?* · registrar lo que produjo un comando |
| **Simulación** *(Customer–Supplier)* | interpolar en una petición sin intento |

