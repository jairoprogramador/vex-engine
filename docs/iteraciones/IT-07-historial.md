# IT-07 — Historial

> Etapa: E7 *(en el plan, «Registro de Despliegue»)* · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §10 (agregados) y §12 (repositorios)
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **3 incongruencias (una disuelta por el experto) · 9 decisiones.** **Archivo cerrado: no se vuelve a
> editar.**

---

## 1. Qué se quiere

**Cierre común del bloque B**: agregados con sus invariantes (o por qué no los tiene), entidades y value
objects, eventos, servicios de dominio, repositorios y factorías, servicios de aplicación y puertos.

**La vara del frente 2 de `DEC-01.10`**: una propuesta de diseño entra solo si cumple al menos una de
estas cuatro:

- baja acoplamiento medible;
- fuerza una regla por construcción;
- simplifica un test existente;
- se puede revertir tocando un solo contexto.

**Las preguntas que el plan le hace a E7.** Están escritas sobre el código, y el modelo ya las responde:

| Pregunta del plan | Respuesta del modelo |
|---|---|
| ¿Lo que un intento declara que va a hacer y lo que realmente pasó son dos modelos? | **Uno.** El Historial es un contexto de registros, y las dos cosas son registros del mismo modelo (`DEC-03.13`, `DEC-05.5`) |
| ¿El registro de un paso es de aquí o de Ejecución? | **De aquí**: es un registro, bajo la posición del paso en su ámbito (`DEC-06.12`). Lo que dice es contenido de Ejecución |
| ¿Dónde cae la frontera con Sincronización? | **Dentro**: Sincronización se realiza en el Historial (`DEC-03.11`) |
| ¿El índice es dominio o infraestructura? | **Infraestructura**: se puede derivar y no decide nada (`DEC-05.5`, `DEC-05.10`) |

**Lo que el core le pidió** está en `contextos/diagnostico.md` (Customer–Supplier, `context-map.md` fila
#1). Esta iteración tiene que poder darlo.

---

## 2. Propuesta

El modelo del contexto está en `modelo/contextos/historial.md`. Lo que lo sostiene:

1. **Se escribe desde sus clientes.** Quién escribe y quién lee, y qué, antes que las entidades: es el
   mismo orden que siguió Diagnóstico con sus escenarios.
2. **Cuatro agregados pequeños**: Intento, Despliegue, Lanzamiento y Reserva. Cada uno protege sus
   propias invariantes y referencia a los demás por identidad, que son las reglas 1 a 3 del libro. La
   cuarta regla, consistencia fuera de la frontera, aquí es *más tarde en la misma invocación*.
3. **Cada agregado son sus registros.** Una transacción es añadir **un** registro, después de comprobar
   las invariantes de su agregado. Encaja con `DEC-05.9`: cada registro se escribe en cuanto ocurre.
4. **El despliegue lo crea el Historial.** *Despliegue* es un concepto del Historial, y la regla que lo
   define (exitoso y con todos sus pasos) solo puede protegerla quien lo crea.
5. **El resto de invariantes se cumplen por construcción**: los repositorios no tienen *actualizar* ni
   *borrar*, y el valor ofuscado solo sale por la relación reservada.

---

## 3. Incongruencias

### Q-07.1 — Buscar por el hash del código, en un contexto que no interpreta el contenido · **resuelta**

**La incongruencia.** Diagnóstico necesita *el último despliegue de un ambiente con un hash del código
dado* (`DEC-06.8`). Pero el hash del código es **contenido de Suministro**, y por `DEC-03.13` el Historial
guarda el contenido **sin aplicarle sus reglas**: no sabe qué dice un registro, solo su forma (a qué
intento pertenece, qué paso, qué ambiente, cuándo). Para buscar por el hash del código, el Historial
tendría que mirar dentro de un contenido que no es suyo.

Pasa lo mismo, más suave, con cualquier otra búsqueda por algo que no sea forma. Pero el caso del hash del
código es el que necesita el core.

**Propuesta.** **La forma del Historial incluye las claves por las que sus clientes necesitan buscar**, y
esas claves entran porque el core las pide (Customer–Supplier). Hoy la única clave que no es forma es
**el hash del código de un intento**. El Historial la guarda como clave, sin saber qué significa: sabe
que dos intentos tienen la misma clave, no que tienen el mismo código. Todo lo demás sigue siendo
contenido opaco.

**La alternativa que cambiaría el resultado.** El Historial solo busca por forma, y el filtro por
contenido lo hace quien lee: Diagnóstico recorre los despliegues del ambiente anterior y compara los
hashes él mismo. El Historial queda puro, pero la búsqueda que necesita el core se hace fuera del
contexto que guarda los datos, y cada cliente que necesite algo parecido tendrá que hacer la suya.

**Respuesta.** *«Sí, correcto, si guarda el hash también; para el registro es una clave más.»* Se adopta
(`DEC-07.6`).

---

### Q-07.2 — Los commits entre dos puntos, ¿de dónde salen si el core solo lee el Historial? · **disuelta por el experto**

**La incongruencia.** `DEC-06.20` dice que el sustento da, por cada eje que cambió, *los commits de su
repositorio entre la referencia y el intento, con autor y fecha*. Pero el Historial solo tiene lo que
cada intento dejó escrito, y un intento solo conoce **su** commit. Si entre el despliegue de referencia y
el intento que falla hubo cinco commits y ningún intento usó los tres del medio, esos tres no están en el
Historial. Y el core solo lee el Historial (`DEC-03.3`).

**Propuesta.** **Al abrir un intento, Ejecución le pide a Suministro los commits que hubo desde el intento
anterior del mismo ambiente**, en el repositorio del producto y en el del pipeline, con su autor y su
fecha, y quedan como contenido de la apertura. Así, los commits entre la referencia y el intento que
falla son la suma de los que registró cada intento de ese tramo. Si es el primer intento del ambiente,
solo se guarda su propio commit.

**La alternativa que cambiaría el resultado.** Que Diagnóstico le pregunte directamente a Suministro por
los commits entre dos commits. Siempre estarían todos, pero el core dejaría de leer solo el Historial y
tendría una relación nueva con un genérico.

**Respuesta.** *«¿Para qué son importantes los commits? Pienso que no son importantes; lo importante es
saber si algo cambió o no. Los commits, el autor y más son parte de la herramienta git. En nuestro caso
debemos simplemente enfocarnos en esta herramienta de despliegue y lo necesario para ello.»* La pregunta
queda sin objeto (`DEC-07.7`).

---

### Q-07.3 — Dos intentos a la vez en el mismo ambiente · **resuelta**

**La incongruencia.** El modelo dice que *en un ambiente no hay dos despliegues a la vez* y que *la
concurrencia no es un concepto de este dominio*. De ahí salen *el último despliegue*, único por el orden
del tiempo, el padre por defecto y *el intento anterior*. Pero los intentos corren en máquinas efímeras que
pide cualquiera desde el CLI o el portal, y nada impide que dos personas lancen a la vez un intento al
mismo ambiente. Los dos escribirían en el mismo espacio de trabajo (el estado de terraform, por ejemplo) y
los dos tomarían el mismo padre.

**Propuesta.** **Un ambiente tiene como mucho un intento abierto.** Si se pide abrir un intento en un
ambiente donde hay otro sin desenlace, se rechaza y se dice cuál es. Como un intento sin desenlace puede
ser uno cuya máquina murió, quien pide puede **darlo por abandonado**: queda un registro que lo dice y el
ambiente se libera. No es un cuarto estado del intento: es otro hecho sobre él.

**La alternativa que cambiaría el resultado.** Que el motor no lo controle y lo evite quien lanza los
intentos, por ejemplo el portal. El modelo queda igual, pero dos intentos a la vez, uno desde el CLI y
otro desde el portal, escribirían en el mismo espacio de trabajo sin que nada lo impida.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-07.8`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-07.1 — Las preguntas del plan para E7 ya están respondidas por el modelo *(por defecto)*

**Decisión.** La tabla de §1: un solo contexto, el registro de un paso es de aquí, Sincronización está
dentro y el índice es infraestructura.

**Por qué.** Las preguntas estaban formuladas sobre paquetes de código. Las decisiones de IT-03 a IT-06
ya las responden en el lenguaje del modelo.

### DEC-07.2 — Cuatro agregados, y cada uno son sus registros *(por defecto)*

**Decisión.** **Intento**, **Despliegue**, **Lanzamiento** y **Reserva**, con las invariantes de
`contextos/historial.md`. Una transacción es añadir un registro, después de comprobar las invariantes de
su agregado.

**Por qué.** Son las cosas del Historial que tienen reglas propias que proteger, y ninguna regla obliga a
cambiar dos de ellas a la vez. Son las reglas 1 y 2 del libro.

**Qué descarta.** Un agregado por ambiente con todo dentro, que sería grande y protegería invariantes que
no existen.

### DEC-07.3 — El despliegue lo crea el Historial, a partir del intento *(por defecto)*

**Decisión.** Ejecución cierra un intento como exitoso. El Intento comprueba que tiene registro de todos
sus pasos previstos y crea el Despliegue, con el padre por defecto o con el destino del rollback.
Después se publica *despliegue registrado*.

**Por qué.** *Despliegue no admite grados* es una invariante del Historial, y solo la puede proteger
quien crea el despliegue. Si lo creara Ejecución, la regla viviría fuera del contexto al que pertenece.

**Consecuencias.** Al abrir un intento hay que declarar sus pasos previstos. Un paso que no hizo falta
re-ejecutar cuenta como hecho.

### DEC-07.4 — Lo anterior se deduce; el padre se guarda *(por defecto)*

**Decisión.** El intento anterior y el lanzamiento anterior salen del orden temporal del ambiente. El
padre de un despliegue se guarda.

**Por qué.** En un ambiente, el tiempo es un orden total (`DEC-02.3`), así que *anterior* se puede
deducir. El padre no, porque un rollback lo elige.

### DEC-07.5 — Los repositorios solo añaden y recorren *(por defecto)*

**Decisión.** Un repositorio por agregado, orientado a colección, sin *actualizar* ni *borrar*.

**Por qué.** Un registro es un hecho que no cambia. Que un repositorio no pueda cambiarlo hace que la
regla se cumpla por construcción (frente 2.B).

### DEC-07.6 — Las claves de búsqueda son forma, y las pide el cliente *(respuesta a `Q-07.1`)*

**Decisión.** La forma del Historial incluye las claves por las que sus clientes necesitan buscar. Hoy,
además de la forma propia (intento, paso, ámbito, ambiente y tiempo), hay una: **el hash del código de
cada intento**. Para el Historial es una clave más, y no sabe qué significa.

**Por qué.** Diagnóstico es cliente del Historial (Customer–Supplier), y lo que necesita entra en el
modelo del de arriba. Si la búsqueda se hiciera fuera, cada cliente tendría que hacerse la suya.

**Consecuencias.** Precisa `DEC-03.13`: la forma es del Historial, incluidas sus claves, y el significado
sigue siendo de quien produjo el contenido. Una clave nueva entra solo si un cliente la necesita para
buscar.

**Qué descarta.** Que los clientes filtren por contenido.

**Verificación.** Cada clave del Historial que no es forma propia nombra al cliente que la pidió.

### DEC-07.7 — Vex dice si algo cambió, no quién lo cambió *(respuesta a `Q-07.2`, que el experto disolvió)*

**Decisión.** El sustento no lleva autores ni commits: dice qué ejes cambiaron, en qué pasos y qué
variables, y entre qué dos momentos (la fecha de la referencia y la del intento). *Autor* sale del
lenguaje. El **commit** se queda, pero solo para lo que hace falta para desplegar: volver a tener delante
el material de un despliegue anterior en un rollback (`DEC-03.9`).

**Por qué.** Vex es una herramienta de despliegue, y para atribuir una causa lo que necesita saber es si
un eje cambió. Quién hizo cada cambio es asunto de la herramienta de repositorios.

**Consecuencias.** **Deroga `DEC-06.20` y `DEC-03.12`**, y retira de la cadena de garantías de
`dominio.md` la fila de la autoría, que IT-01 había dejado sin dueño. `Q-07.2` queda sin objeto. El
*solicitante* de un intento se queda, porque es un hecho de la herramienta de despliegue, no de git.

**Qué descarta.** Guardar los commits intermedios, y que el core pregunte a Suministro.

**Verificación.** Ningún documento de `modelo/` promete saber quién hizo un cambio.

### DEC-07.8 — Un ambiente tiene como mucho un intento en curso *(respuesta a `Q-07.3`)*

**Decisión.** En un ambiente no puede haber dos intentos sin desenlace a la vez. Si se pide abrir un
intento en un ambiente donde ya hay otro, se rechaza, diciendo cuál es. Un intento sin desenlace se
puede **dar por abandonado**: queda un registro que lo dice y el ambiente se libera. No es un cuarto
estado.

**Por qué.** Los intentos corren en máquinas efímeras que puede pedir cualquiera. Sin esta regla, dos
intentos escribirían a la vez en el mismo espacio de trabajo y tomarían el mismo padre.

**Consecuencias.**

- Precisa la restricción de `dominio.md`: la concurrencia no es un concepto del dominio **porque** un
  ambiente no la admite.
- **Un intento abandonado no acepta más registros.** Si su máquina seguía viva, no podrá escribir, y por
  `DEC-05.9` se detendrá antes del siguiente paso. Así, abandonar un intento también impide que siga.
- *Dar por abandonado un intento* es una operación más del lenguaje publicado del motor, y la atiende el
  Historial.

**Qué descarta.** Que lo controle quien lanza los intentos.

**Verificación.** Nunca hay en un mismo ambiente dos intentos sin desenlace y sin abandonar a la vez.

### DEC-07.9 — La ocupación de un ambiente es un agregado; la apertura declara los pasos del pipeline *(por defecto)*

**Decisión.**

- **Ocupación** es un agregado propio: qué intento tiene un ambiente en curso. Se ocupa al abrir el
  intento y se libera con su cierre o su abandono. Su invariante es que un ambiente tiene como mucho una
  ocupación vigente.
- La apertura de un intento declara **los pasos del pipeline y hasta cuál se pide**. El Despliegue solo
  nace si se pidieron y se registraron todos.

**Por qué.** Una regla que abarca varios intentos no cabe en el agregado Intento (regla 1 del libro), y
un agregado por ambiente con todo dentro sería grande (regla 2). La Ocupación es pequeña a propósito: es
la única regla que cruza intentos. Y un despliegue exige todos los pasos del pipeline, no solo los
pedidos.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/contextos/historial.md` | **Nace**: lo que le piden, y el modelo táctico propuesto |
| `plan-ddd.md` | Tablero |
| `modelo/contextos/historial.md` · `lenguaje.md` · `context-map.md` | Las claves de búsqueda, y el hash del código como clave (`DEC-07.6`) |
| `modelo/dominio.md` · `lenguaje.md` · `bounded-contexts.md` · `context-map.md` · `contextos/diagnostico.md` · `contextos/historial.md` | Fuera autores y commits como sustento; el commit solo sirve para volver atrás (`DEC-07.7`) |
| `modelo/dominio.md` · `lenguaje.md` · `arquitectura.md` · `context-map.md` · `contextos/historial.md` | Un intento en curso por ambiente, el abandono y la ocupación (`DEC-07.8`, `DEC-07.9`) |
| `modelo/contextos/historial.md` | Pasa a **vigente** |

## 6. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Cómo garantiza el almacén que dos máquinas no ocupen el mismo ambiente a la vez: la ocupación necesita una escritura que solo tenga éxito si el ambiente está libre | **IT-10** | Es infraestructura del Historial y del almacén que se compre |

**Duda anterior que queda sin objeto.** La **#1** de IT-05 (qué pasa con los registros escritos en la
máquina que no llegaron al almacén): con `DEC-06.18`, un registro no está escrito hasta que se puede leer
desde cualquier máquina. Se anota aquí porque IT-05 está cerrada.
