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
| **Definición** | las variables declaradas, cada una con su ámbito · el ámbito propio de cada paso (`config.yaml`, `steps.<paso>.scope`; RD-04 §9.19, §9.20) · las formas de ámbito y de variable de salida · los nombres de las **variables estándar**, cuyos valores da Resolución: los metadatos llegan con la solicitud y las generadas las crea el motor; las del paso solo valen mientras el paso se ejecuta (RD-04 §9, lo precisa RD-05) |
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
| ámbito | una variable pertenece a un ámbito, y un paso ve las de los ámbitos que ve **desde el suyo propio** (`DEC-06.12`, RD-04 §9.19): sin `scope` declarado, el que representa a su ambiente — su ambiente y el compartido, como siempre —; con `scope: shared`, solo el compartido. Lo compartido no ve lo del ambiente, y un paso de `scope: shared` tampoco |
| orden | una variable producida solo existe **desde el comando que la produjo**: la ven los comandos posteriores de su paso y los pasos posteriores de su ámbito (RD-04 §9) |
| precedencia | si dos orígenes dan valor al mismo nombre, **gana la producida**; entre producidas, **la del paso más reciente** (`DEC-08.4`) |
| paso que no se re-ejecuta | aporta las variables que produjo la última vez, con su valor pedido al Historial por la relación reservada |
| el valor no circula | solo sale hacia el comando que se ejecuta y, ofuscado, hacia el Historial |
| sin intento | no calcula hashes, no escribe nada y no conserva nada (`DEC-04.10`) |

### Value objects

| | Qué es |
|---|---|
| **variable efectiva** | nombre, valor, origen y ámbito. El valor es sensible y **no se muestra nunca** |
| **origen** | declarada · producida |
| **ámbito** | el de un ambiente · el compartido. Es de la variable, y también del paso (RD-04 §9.19): sin declarar el suyo, el del ambiente en que se ejecuta. Decide qué ve el paso y bajo cuál busca su última vez |
| **hash de variable** | dice si una variable cambió sin mostrar su valor. **Comparable entre ambientes de un mismo proyecto** (`DEC-08.6`). **Simple, sin clave**: no es una garantía de seguridad (`DEC-08.8`) |

### Servicios de dominio

| Servicio | Qué hace |
|---|---|
| **interpolación** | sustituye cada variable por su valor dentro de un comando o del material de un paso |
| **calcular el hash de una variable** | el único punto donde un valor se convierte en hash |
| **¿cambiaron las variables de un paso?** | compara los hashes de ahora con los de la última vez del paso en su ámbito. **Cuál es el ámbito de un paso lo declara Definición** (`config.yaml`, `steps.<paso>.scope`; RD-04 §9.19): Resolución lo usa, no lo decide |

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

