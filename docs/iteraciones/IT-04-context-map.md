# IT-04 — Context Map

> Etapa: E4 · Estado: **cerrada**
> Abierta: 2026-09-13 · Cerrada: 2026-09-13
> Lectura previa: `guia-ddd.md` §3
>
> **Primera iteración con el proceso de `DEC-03.15`.** Claude define contextos, relaciones y
> lenguaje a partir de las respuestas del experto, contrasta con el libro y trae las incongruencias
> **una a una**. Las incongruencias se analizan juntos.
>
> **4 incongruencias · 10 decisiones.** Cerró cuando una vuelta ya no dejó incongruencias que
> movieran el mapa. **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Según el plan** (E4): que ningún par de contextos que se integra quede sin **dirección**, sin
**patrón** y sin **contrato**, y que todo ciclo tenga su resolución escrita.

**Según el experto**, sobre cómo se trabaja: *«según las respuestas tú vas definiendo los contextos y
el lenguaje, tú encuentras las incongruencias y las analizamos juntos»*.

**Lo que arrastra de antes:**

| Origen | Duda | Dónde queda |
|---|---|---|
| IT-03 §9 **#1** | Qué es *el cliente* de un lanzamiento fuera de producción, y quién decide ahí | `DEC-04.1` |
| IT-03 §9 **#2** | El patrón de cada arista, incluido cómo comparten mecanismo Simulación y Ejecución | `DEC-04.3` · `DEC-04.6` |
| IT-02 §9 **#4** | El idioma y el patrón de la frontera con el CLI y el portal | `DEC-04.4` |
| IT-02 §9 **#7** | Si las especificaciones de los hashes son Published Language | `DEC-04.5` |

---

## 2. Propuesta

El mapa completo está en `modelo/context-map.md`. Lo que lo sostiene, contrastado con el libro:

1. **El core se protege dos veces.** Es cliente del Historial (Customer–Supplier): lo que necesita
   decide qué se conserva, que es la regla del plan *«el core dicta lo que los supporting
   publican»*. Y traduce (ACL): nunca razona con registros, sino con ejes.
2. **El Historial manda sobre su forma y no sobre lo que guarda.** Quien escribe en él adopta la
   forma (Conformist) y deja su contenido opaco. Es la lectura directa de `DEC-03.13`.
3. **El pipeline es un Published Language.** El DevOps lo escribe, lo usan tres contextos y tiene su
   propia versión. Como las formas significan en quien las aplica (`DEC-03.6`), cada consumidor le
   pone su ACL.
4. **Lo genérico entra traducido.** Suministro llega a sus tres consumidores a través de un ACL, y
   lo que se compra (repositorios, almacén) queda detrás de otro ACL dentro de Suministro y dentro
   del Historial.
5. **No hay ciclos.** Las dos flechas que parecían ir en los dos sentidos eran lectura y escritura
   hechas por quien llama. El Historial guarda contenido ajeno sin depender de él.
6. **Hacia fuera, un solo lenguaje publicado.** El motor es un Open Host para el CLI y el portal, en
   español (`DEC-02.2`), y los dos se adaptan a él.

**Corrección de paso** en `bounded-contexts.md`: el diagrama dibujaba flechas en los dos sentidos
donde había una sola dirección de llamada. Queda con una sola dirección y remite aquí para los
patrones.

---

## 3. Incongruencias

### Q-04.1 — El valor ofuscado dentro de un Historial que publica sus registros · **resuelta**

**La incongruencia.** El Historial es upstream de cuatro contextos y, a través del Open Host del
motor, también del CLI y del portal: publica sus registros. Pero por `Q-03.25` guarda dentro el
**valor ofuscado** de las variables, y *«ofuscar no es cifrar»*. Si ese valor forma parte de lo que
el Historial publica, cualquiera que consulte el historial lo tiene, a un paso de leerlo. *«Sin que el
valor circule»* dejaría de ser disciplina y pasaría a ser falso.

**Propuesta.** El valor ofuscado **no forma parte del lenguaje publicado del Historial**. Se guarda
bajo su forma, pero solo se entrega a **Resolución**, por una relación propia que ningún otro
contexto ni sistema externo tiene. El core, Lanzamiento, Ejecución, el CLI y el portal leen
registros sin valores. Con eso, la pérdida aceptada en `DEC-03.16` se convierte en contrato: vuelve a
ser una frontera.

**La alternativa que cambiaría el resultado.** Aceptar que el Historial publique el valor ofuscado y
fiarlo a que nadie lo lea. Es más simple, pero la promesa al core queda en manos de cada lector,
también de los externos.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-04.7`).

---

### Q-04.2 — ¿Quién avisa a Lanzamiento de que hay un despliegue listo? · **resuelta**

**La incongruencia.** Por `DEC-04.1`, en los ambientes que el dueño del negocio no se reserva *se
lanza solo en cuanto el despliegue queda listo*. Pero en el mapa nadie llama a Lanzamiento: lee el
Historial solo cuando alguien de fuera se lo pide, y el motor no recuerda nada entre intentos. Tal
como está, el lanzamiento automático **no ocurre nunca**. Y la salida obvia, que Ejecución llame a
Lanzamiento al terminar un despliegue, contradice `dominio.md`: *«llegar al último despliegue no
dispara el lanzamiento»*, porque actuar en nombre del actor ausente **nunca es un efecto del
despliegue**.

**Propuesta.** El Historial **publica un evento de dominio**, *despliegue registrado*, que es el
propio registro del despliegue anunciado en su lenguaje publicado. Lanzamiento lo escucha y, si el
dueño del negocio no se ha reservado ese ambiente, lanza en su nombre dentro del mismo intento.
Ejecución no sabe que Lanzamiento existe, así que lanzar sigue sin ser un efecto del despliegue. La
dirección tampoco cambia: Lanzamiento sigue siendo downstream del Historial. Cómo viaja el evento
dentro del proceso es de E5; aquí solo se decide quién lo publica y quién lo escucha.

**La alternativa que cambiaría el resultado.** Que lance quien pidió el intento, el CLI o el portal,
cuando ve el despliegue terminado. El motor queda más simple, pero la regla del actor ausente sale
del dominio y cada cliente externo tiene que cumplirla por su cuenta.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-04.8`).

---

### Q-04.3 — ¿Dónde queda escrito qué ambientes se reserva el dueño del negocio? · **resuelta**

**La incongruencia.** Para lanzar en nombre del actor ausente, Lanzamiento tiene que saber si el
dueño del negocio se ha reservado ese ambiente. Pero el motor no recuerda nada entre intentos, y el
modelo no dice dónde queda escrita esa reserva. El sitio que parece natural, el pipeline, es del
DevOps: si ahí se escribiera qué ambientes se reserva el dueño del negocio, cambiar de opinión
cambiaría el hash del pipeline. El core vería entonces un cambio en el pipeline que no hizo el DevOps y
que no toca ninguna instrucción.

**Propuesta.** La reserva es **una decisión de Lanzamiento que queda como registro en el
Historial**. El dueño del negocio reserva o libera un ambiente a través del motor, con una operación
más del lenguaje publicado. Antes de lanzar en su nombre, Lanzamiento consulta la última reserva de ese
ambiente. Así hay una sola memoria, cada actor es dueño de su decisión y el pipeline no se toca.

**La alternativa que cambiaría el resultado.** Que quien pide cada intento, el CLI o el portal, diga
si ese ambiente está reservado. No habría nada que guardar, pero la regla volvería a depender de cada
cliente, y dos clientes podrían contradecirse.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-04.9`).

---

### Q-04.4 — Una salida simulada dentro del contexto donde ningún valor puede ser inventado · **resuelta**

**La incongruencia.** Simulación le pide a Resolución que interpole (fila #6 del mapa). Pero en una
simulación los comandos son fingidos, así que las variables de salida que usan los pasos siguientes
son **salidas simuladas**: valores que nadie produjo. Para interpolarlas, Resolución tendría que
tratarlas como *variables*, y en Resolución una variable es un **valor efectivo**. La prueba que separa
Simulación de Resolución dice justo lo contrario: *«fusionados, un valor efectivo podría ser
inventado»*. Tal como está el mapa, lo inventado entra en Resolución.

**Propuesta.** Para Resolución, una salida simulada es **un valor producido por un paso dentro de una
petición que no pertenece a ningún intento**. Lo resuelve y lo interpola como cualquier otro, pero no
le calcula hash, no deja nada en el Historial y no lo conserva. La palabra *simulada* solo existe en
Simulación: Resolución no sabe que el valor es inventado, sabe que no hay intento. La prueba de
separación se reescribe con esa regla: en Simulación el valor es inventado; en Resolución es un valor
sin intento, que no deja rastro.

**La alternativa que cambiaría el resultado.** Que Simulación interpole ella misma sus salidas
simuladas y solo le pida a Resolución los valores declarados. Resolución no vería nunca nada
inventado, pero Simulación duplicaría la interpolación, que hoy consumen las dos del mismo sitio
(`DEC-04.6`).

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-04.10`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* no vienen de una incongruencia: se decidieron con lo que ya estaba
> respondido y se revierten si no sirven (`DEC-03.15`).

### DEC-04.1 — Lanzar es hacer visible un despliegue a quien usa ese ambiente *(confirmada por el experto)*

**Decisión.** Un lanzamiento hace visible un despliegue a su **destinatario**, que es quien usa ese
ambiente: el equipo en dev, quien valida en staging, el cliente final en producción. El dueño del
negocio decide en los ambientes que él elija (normalmente producción). En los demás rige la regla del
actor ausente: se lanza solo en cuanto el despliegue queda listo.

**Por qué.** El experto respondió que un lanzamiento ocurre en cualquier ambiente (`DEC-03.17`), y
así Lanzamiento conserva su sentido fuera de producción sin inventar un segundo concepto. El experto
la confirmó: *«estoy de acuerdo, lanzar es hacer visible un despliegue a quien usa ese ambiente»*.

**Qué descarta.** Un lanzamiento reservado a producción; un dueño del negocio que decide en todos los
ambientes.

**Verificación.** `lenguaje.md` define *destinatario* y ya no tiene nada pendiente en §Lanzamiento.

### DEC-04.2 — Con un solo desarrollador, un patrón de organización dice qué modelo cede *(por defecto)*

**Decisión.** Customer–Supplier, Conformist y Partnership se leen como **qué modelo cambia cuando el
otro necesita algo**, no como relación entre equipos.

**Por qué.** El libro los formula para equipos, y aquí hay uno solo. Leídos al pie de la letra, no
distinguirían nada.

**Verificación.** Cada fila de `context-map.md` dice por qué ese modelo cede, o por qué no.

### DEC-04.3 — El mapa: doce relaciones con patrón y sin ciclos *(por defecto)*

**Decisión.** Las de `modelo/context-map.md`: core con Customer–Supplier y ACL; quien escribe en el
Historial, Conformist a su forma; Resolución como Supplier de Ejecución y de Simulación; el pipeline
como Published Language con ACL en cada consumidor; Suministro detrás de un ACL.

**Por qué.** Sección 2 de este archivo.

**Verificación.** Ninguna arista de `bounded-contexts.md` queda sin fila en `context-map.md`, y el
grafo de llamadas no tiene ciclos.

### DEC-04.4 — Hacia fuera, el motor es un Open Host con un Published Language en español *(por defecto)*

**Decisión.** El CLI y el portal hablan con el motor mediante un único lenguaje publicado, versionado
y en español, y se adaptan a él. Sus operaciones son intentar, hacer rollback, simular, lanzar,
preguntar la causa y consultar el historial.

**Por qué.** `DEC-02.2` ya llevó el español hasta el contrato y aceptó romper hacia fuera, y el plan
trata al CLI y al portal como sistemas externos. Cierra IT-02 §9 **#4**.

**Qué descarta.** Un contrato distinto por cliente; un ACL en el motor para cada cliente.

### DEC-04.5 — Las especificaciones de los hashes no están en el mapa *(por defecto)*

**Decisión.** No son Published Language de ninguna relación.

**Por qué.** Por `DEC-02.18`, lo que documentan (qué entra en un hash) es técnica y no dominio. Cierra
IT-02 §9 **#7**. Qué hacer con ellas es de E11.

### DEC-04.6 — Simulación y Ejecución siguen caminos separados *(por defecto)*

**Decisión.** **Separate Ways**: no se integran y no comparten núcleo.

**Por qué.** No se hablan (`Q-03.26`), y lo que comparten de verdad —interpolación y comprobación— ya
lo consumen las dos de otro contexto. Un Shared Kernel ataría dos contextos que no se necesitan, y el
libro lo trata como un mal a minimizar.

### DEC-04.7 — El valor ofuscado no forma parte de lo que el Historial publica *(respuesta a `Q-04.1`)*

**Decisión.** El Historial publica **registros sin valores**, tanto a los contextos como, a través del
motor, al CLI y al portal. El valor ofuscado de una variable se guarda bajo la forma del Historial y
solo vuelve a **Resolución de Variables**, por una relación que ningún otro contexto ni sistema
externo tiene.

**Por qué.** Ofuscar no es cifrar: con el valor dentro de lo publicado, *sin que el valor circule*
sería falso para cualquier lector.

**Consecuencias.** La pérdida aceptada de `DEC-03.16` (que la promesa dependiera de la disciplina de
quien lee) deja de serlo: vuelve a estar protegida por una frontera, que es el frente 2.B de
`DEC-01.10`.

**Qué descarta.** Publicar el valor ofuscado y confiar en que nadie lo lea.

**Verificación.** Ninguna fila de `context-map.md` lleva el valor de una variable salvo la #3.

### DEC-04.8 — El Historial anuncia cada despliegue registrado, y Lanzamiento lo escucha *(respuesta a `Q-04.2`)*

**Decisión.** Al registrar un despliegue, el Historial publica el evento de dominio **despliegue
registrado**. Lanzamiento lo escucha y, si el dueño del negocio no se ha reservado ese ambiente, lanza
en su nombre dentro del mismo intento.

**Por qué.** Sin ese aviso, el lanzamiento automático no ocurría nunca. Y que Ejecución llamara a
Lanzamiento habría convertido lanzar en un efecto del despliegue.

**Consecuencias.** Ejecución no conoce a Lanzamiento. Ninguna relación cambia de sentido: el evento
va del Historial a quien lo escuche, y el Historial no sabe quién es. Es el primer evento de dominio
del mapa. Cómo viaja dentro del proceso es de E5.

**Qué descarta.** Que Ejecución llame a Lanzamiento; que lance el CLI o el portal.

**Verificación.** Ninguna fila de `context-map.md` tiene a Ejecución llamando a Lanzamiento.

### DEC-04.9 — La reserva de un ambiente es una decisión de Lanzamiento que queda en el Historial *(respuesta a `Q-04.3`)*

**Decisión.** El dueño del negocio **reserva** o **libera** un ambiente a través del motor, con una
operación más del lenguaje publicado. Cada reserva queda como registro en el Historial. Antes de lanzar
en su nombre, Lanzamiento consulta la última reserva de ese ambiente.

**Por qué.** El motor no recuerda nada entre intentos. En el pipeline, la reserva habría cambiado el
hash del pipeline por una decisión que no es del DevOps ni toca ninguna instrucción.

**Consecuencias.** *Reserva* entra en el lenguaje en dos contextos: en Lanzamiento es la decisión y
en el Historial es el registro, el mismo reparto que tiene *lanzamiento*.

**Qué descarta.** La reserva en el pipeline; que la diga cada cliente en cada petición.

**Verificación.** Nada de lo que decide el dueño del negocio aparece en `lenguaje.md` §Definición.

### DEC-04.10 — Para Resolución, una simulación es una petición sin intento *(respuesta a `Q-04.4`)*

**Decisión.** Cuando lo que se le pide no pertenece a ningún intento, Resolución resuelve e interpola,
incluidos valores producidos por pasos, pero **no calcula hashes, no deja nada en el Historial y no
conserva nada**. No sabe por qué no hay intento. *Simulada* es una palabra que solo existe en
Simulación.

**Por qué.** Sin esta regla, un valor inventado entraba en Resolución como valor efectivo, justo lo que
la prueba de separación entre los dos contextos prohíbe.

**Consecuencias.** La prueba de separación se reescribe: en Simulación el valor es inventado; en
Resolución es un valor sin intento que no deja rastro. La interpolación sigue teniendo un solo dueño.

**Qué descarta.** Que Simulación interpole ella misma sus salidas simuladas, que duplicaría la
interpolación.

**Verificación.** Ningún hash de variable ni ningún registro nace de una petición sin intento.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/context-map.md` | **Nace** con la propuesta (provisional mientras IT-04 esté abierta) |
| `modelo/bounded-contexts.md` | Diagrama con una sola dirección de llamada; remite a `context-map.md` |
| `modelo/lenguaje.md` · `modelo/dominio.md` | Lanzamiento: *destinatario*, y quién decide en cada ambiente (`DEC-04.1`) |
| `plan-ddd.md` | Tablero |
| `modelo/context-map.md` · `lenguaje.md` · `dominio.md` · `bounded-contexts.md` | El valor ofuscado no se publica; solo vuelve a Resolución (`DEC-04.7`) |
| `modelo/context-map.md` · `lenguaje.md` · `dominio.md` · `bounded-contexts.md` | El evento *despliegue registrado* y Lanzamiento escuchándolo (`DEC-04.8`) |
| `modelo/context-map.md` · `lenguaje.md` · `dominio.md` · `bounded-contexts.md` | La reserva de un ambiente (`DEC-04.9`) |
| `modelo/context-map.md` · `lenguaje.md` · `bounded-contexts.md` | La petición sin intento (`DEC-04.10`) |
| `modelo/context-map.md` | Pasa a **vigente** |

## 6. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Cómo viaja un evento de dominio (*despliegue registrado*) dentro del proceso, y cómo se hablan en general los contextos en un motor one-shot | **IT-05** | Es la pregunta de E5 del plan |
| 2 | ¿La secuencia de registros del Historial es **su modelo** o **su persistencia**? (IT-03 §9 #3) | **IT-05** | Sigue siendo de E5 |
| 3 | La forma concreta del lenguaje publicado del motor: sus operaciones, cómo se versiona | **IT-05** | El patrón está decidido (`DEC-04.4`); la forma es de arquitectura |
| 4 | Qué almacén y qué acceso a repositorios se compran detrás de los dos ACL externos | **IT-10** | Es de E10 |
| 5 | Los cinco casos que `guia-ddd.md` §3 sacaba del código (los tres puertos de hechos, la aserción del factory, las especificaciones, el contrato con el CLI, el paquete de huellas) no fueron material de este mapa (`DEC-02.10`) | **IT-11** | Reconciliar lo construido con el mapa es de E11 |
