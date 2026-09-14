# IT-03 — Frontera de los contextos

> Etapa: E3 · Estado: **cerrada**
> Abierta: 2026-09-13 · Cerrada: 2026-09-13
> Lectura previa: `guia-ddd.md` §2 *(«Cuándo un contexto es demasiado pequeño»)* y la tabla de
> homónimos de `modelo/lenguaje.md`
>
> **27 preguntas · 18 decisiones · 3 rondas.** Cerró con una enmienda de proceso: desde aquí se
> itera con propuestas y preguntas solo sobre incongruencias (`DEC-03.15`).
>
> **Archivo cerrado: no se vuelve a editar.** Lo que cambia a partir de aquí es `modelo/`.
>
> **Material: solo `modelo/`** (`DEC-02.10`). Ningún argumento de este archivo cita código ni specs.

---

## 1. Objetivo — qué tiene que ser verdad al cerrar

Hasta aquí el modelo habla del **problema**: nueve subdominios (IT-01) y un lenguaje partido por
subdominio, con los homónimos declarados y la frontera que señala cada uno (IT-02). E3 es la primera
iteración que habla de la **solución**: cuántos **bounded contexts** hay, dónde acaba cada uno y por
qué.

Al cerrar tiene que ser verdad:

1. Hay **una sola prueba de separación**, escrita, y está dicho qué cuenta como pasarla (duda **#9**
   de IT-01, hallazgo **H5**).
2. Cada contexto que sobrevive tiene **propósito** en una frase, el **subdominio o subdominios** que
   realiza, sus **términos propios** y su **prueba de separación** contra cada contexto con el que
   se habla, escrita como *«si lo fusionara con X, se rompería esto»*.
3. Cada fusión o partición respecto de los nueve subdominios está **justificada con esa misma
   prueba**. Nunca con volatilidad, facilidad de prueba o tamaño.
4. Ningún término significa dos cosas **dentro** de un contexto. Cada homónimo de `lenguaje.md` que
   sobrevive queda asignado a una frontera concreta; los que no sobreviven, se resuelven.
5. Las dudas diferidas a E3 tienen respuesta: la partición de Ejecución (IT-01 **#1**), la prueba
   (**#9**), Simulación como contexto o modo (**#11**), la autoría (**#15**); los sentidos de *paso*
   (IT-02 **#1**), de *variable* y *estado* (**#2**), el rollback (**#3**) y el *ámbito* (**#5**).
6. Se han comprobado las dos verificaciones que decisiones anteriores dejaron para aquí:
   - `DEC-01.3` — *«IT-03 llega a la partición interna sin reabrir la lista de subdominios»*.
   - `DEC-01.13` — *«IT-03 traza la frontera sin que ningún término de Lanzamiento aparezca en el
     lenguaje de Registro»* (hoy: del Historial).
7. `modelo/bounded-contexts.md` está reescrito.

**Lo que esta iteración NO hace.** No elige **patrones de relación** (Conformist, ACL, Shared
Kernel…): eso es E4. Aquí se decide *si hay frontera* y *qué cambia al cruzarla*; E4 decide *quién
paga esa traducción y cómo*. Tampoco decide paquetes ni módulos (E5), ni agregados (bloque B). Si una
respuesta empieza a resolverse por ahí, se difiere y se anota en §9.

---

## 2. Punto de partida

### 2.1 Lo que dice hoy el modelo

`dominio.md` y `lenguaje.md` están **vigentes** desde el cierre de IT-02 y mandan.
`bounded-contexts.md` está **obsoleto**: lista once contextos sobre un mapa que ya no existe. No es
punto de partida; solo deja constancia de lo que había.

**Los nueve subdominios, con el lenguaje que `lenguaje.md` les asigna:**

| Subdominio | Tipo | Actor | Términos propios | Nº |
|---|---|---|---|---:|
| **Diagnóstico** | Core | programador · dueño del negocio | causa · sustento · eje · eliminación · atribución · deducción · inferencia | 7 |
| Definición de Pipeline | Supporting | DevOps | pipeline · paso · comando · instrucciones · variable declarada · variable de salida · ambiente · orden de los ambientes · último ambiente · regla · comprobación | 11 |
| Historial | Supporting | DevOps · programador | registro · historial · intento · estado · despliegue · padre · punto de retorno · cantidad de intentos · evidencia | 9 |
| Lanzamiento | Supporting | dueño del negocio | lanzar · lanzamiento · fecha de publicación · versión · nombre del lanzamiento | 5 |
| Ejecución de Pipeline | Supporting | programador | intentar · ejecución · paso · re-ejecución de un paso · no se re-ejecuta · recursos de un paso · rollback · aislamiento · espacio de trabajo | 9 |
| Resolución de Variables | Supporting | DevOps | variable · ámbito · precedencia · origen · interpolación | 5 |
| Simulación de Pipeline | Supporting | DevOps | simulación · simular un comando | 2 |
| Suministro de Fuentes | Generic | — | fuente · hash | 2 |
| Sincronización del Historial | Generic | — | sincronizar | 1 |

Y un candidato que no es subdominio: **Decidir / Planificación del Intento**, nombrado dentro de
Ejecución de Pipeline por `DEC-01.3`. Hoy no tiene apartado propio en `lenguaje.md`.

**Los homónimos que IT-02 dejó como entrada de esta iteración:**

| Término | Sentidos | Frontera que señala, según `lenguaje.md` |
|---|---|---|
| **paso** | unidad declarada *(Definición)* · unidad que se ejecuta o no se re-ejecuta *(Ejecución)* | entre **declarar** y **hacer** |
| **paso** | *(los dos de arriba)* · posición bajo la que se guardan sus registros *(Historial)* | entre **hacer** y **recordar** |
| **variable** | declarada *(Definición)* · valor efectivo *(Resolución)* | entre lo escrito y lo vigente |
| **variable** | *(los dos de arriba)* · la que un paso produce *(Ejecución)* | entre lo que se declara y lo que nace ejecutando |
| **estado** | desenlace de un intento *(Historial)* | ninguna: no queda segundo sentido vivo |
| **evidencia** | lo que justificó no re-ejecutar un paso *(Historial)* | ninguna: el core cedió la palabra |

### 2.2 Quién lee a quién, según el modelo

La lección de IT-02 (`guia-ddd.md` §2) es que la prueba *«homónimo entre contextos, sí; dentro de
uno, no»* no se aplica mirando el mapa: se aplica mirando **quién lee a quién**. Esta tabla recoge
las lecturas que el modelo afirma. **No son patrones**: eso es E4.

| Quién usa | Qué | De quién | Dónde lo dice |
|---|---|---|---|
| Diagnóstico | despliegue · intento · cantidad de intentos · evidencia | Historial | `lenguaje.md` §Diagnóstico; `dominio.md`: *«el core lee el registro de lo que se usó»* |
| Diagnóstico | que las instrucciones no varían por ambiente · el orden de los ambientes · ambiente · paso | Definición | cadena de garantías; §Diagnóstico |
| Diagnóstico | qué está vivo ante el cliente, que resuelve a su despliegue | Lanzamiento | cadena de garantías; *«dos puertas de entrada»* |
| Diagnóstico | qué se hizo, qué no se re-ejecutó y por qué | Ejecución | cadena de garantías |
| Diagnóstico | si una variable cambió, sin su valor | Resolución | cadena de garantías |
| Diagnóstico | hash | Suministro | §Diagnóstico, *«términos que consume»*; pero *«nunca habla con la fuente»* |
| Ejecución | el material y su hash | Suministro | `dominio.md` §Cómo se relacionan |
| Ejecución | pasos · comandos · reglas · comprobación | Definición | `lenguaje.md` |
| Ejecución | valores efectivos · interpolación | Resolución | *«recursos de un paso»* |
| Ejecución | lo que no cambió desde la última vez · el padre al volver atrás | Historial | *«la historia, en una lectura»*; *rollback* |
| Historial | lo que el intento hizo y dejó dicho | Ejecución | `dominio.md` |
| Sincronización | lo que el historial necesita conservar | Historial | `dominio.md` |
| Lanzamiento | su despliegue | Historial | §Lanzamiento |
| Lanzamiento | el último ambiente | Definición | §Definición; **contradicho** en §Lanzamiento (ver 2.4) |
| Resolución | variables declaradas · variables de salida | Definición | `lenguaje.md` |
| Resolución | lo que un paso produce | Ejecución | *«origen»* |
| Simulación | comprobación · material · interpolación real | Definición · Suministro · Resolución | §Simulación |
| Simulación | *¿nada?* | Historial | *«sin guardar nada»*; el modelo no dice si **lee** |

### 2.3 Dudas diferidas que vencen aquí

| Origen | Duda | Pregunta |
|---|---|---|
| IT-01 **#9** | La prueba de frontera es **una**: qué concepto cambia de significado al cruzar. Volatilidad y aislamiento en pruebas no cuentan (**H5**) | `Q-03.1` |
| IT-01 **#1** | Partir Ejecución de Pipeline en *Decidir / Planificación del Intento* y ejecución real | `Q-03.4` |
| IT-01 **#11** | ¿Simulación es contexto propio o modo de Ejecución? | `Q-03.10` |
| IT-01 **#15** | Quién promete **quién** cambió qué y **cuándo** | `Q-03.12` |
| IT-02 **#1** | ¿Cuántos sentidos de **paso** sobreviven como contextos distintos? | `Q-03.7` |
| IT-02 **#2** | Lo mismo para **variable** y **estado** | `Q-03.5` · `Q-03.13` |
| IT-02 **#3** | **Rollback** es un acto de Ejecución, pero el *padre* es del Historial | `Q-03.9` |
| IT-02 **#5** | **Ámbito** se declara en Definición y gobierna en Resolución: ¿de quién es? | `Q-03.6` |

### 2.4 Lo que el modelo dice mal, o no dice, detectado al abrir

Seis cosas que aparecieron al releer `modelo/` con la pregunta de E3 en la mano. Ninguna es de
lenguaje por gusto: **las seis tocan una frontera**.

1. **¿Dónde ocurre un lanzamiento?** `lenguaje.md` §Lanzamiento dice *«ocurre en todos los
   ambientes»*. §Definición, en el mismo documento, dice que el último ambiente *«es donde ocurren
   los lanzamientos»*, y `DEC-02.3` dice *«tras un despliegue en el último ambiente»*. → `Q-03.8`.
2. **El core dice consumir el *paso* de Definición**, pero *«nunca habla con la fuente»*: lee lo que
   se usó. El paso que puede leer es el del Historial. → `Q-03.7`.
3. **«Pipeline» es homónimo y no está en la tabla.** En Definición es *la declaración*; en Suministro
   es *una fuente*, un repositorio con su hash. → `Q-03.11`.
4. **«Quién» nombra dos cosas en el sustento**: quién hizo un cambio y quién pidió un intento. →
   `Q-03.12`.
5. **«Recursos de un paso» y «eje» agrupan el mismo material** —código, instrucciones, variables— en
   perpendicular: uno por paso, el otro por eje. Entre dos ámbitos no es un error, pero el core lee
   lo que Ejecución deja dicho. → `Q-03.3`.
6. **Dos subdominios casi sin lenguaje**: Simulación tiene dos términos y Sincronización uno. Un
   bounded context es la frontera de un **modelo**, y hay que decir si ahí hay modelo que acotar. →
   `Q-03.10` · `Q-03.11`.

### 2.5 El encuadre de tamaño

`plan-ddd.md` enmarca E3 así: *«un desarrollador, un binario one-shot — once contextos es candidato
claro a sobre-descomposición»*. Hoy hay nueve subdominios y un candidato interno. Sobre ese
encuadre, tres observaciones:

- **Un desarrollador** no descarta nada. Un equipo puede llevar varios contextos; lo que no puede
  pasar es que un contexto lo lleven dos equipos (`guia-ddd.md` §2).
- **One-shot** es una restricción declarada del modelo. Quita la concurrencia del dominio, pero no
  quita fronteras.
- **El tamaño del código no es argumento** (`DEC-02.10`).

Queda un solo criterio, el del lenguaje: **dos contextos son dos si al cruzar hay que traducir**.
Cada frontera tiene además un coste permanente: traducción, disciplina y la vigilancia de que un
modelo no se filtre en el otro.

---

## 3. Inventario de preguntas

> Trece. Las dos primeras son de **método**: qué cuenta como pasar la prueba y desde qué mapa se
> aplica. Condicionan a todas las demás. De la tercera a la duodécima, cada pregunta aplica la prueba
> a **una frontera concreta**, de mayor a menor consecuencia: primero las que tocan al core, luego
> las que parten o funden un supporting, y al final los genéricos y la autoría. La decimotercera
> recoge lo que sale.
>
> Son una más que el umbral de ~12 de `plan-ddd.md` §5. No se parte la iteración: la decimotercera es
> de limpieza y no abre ninguna frontera.

---

### Q-03.1 — ¿Qué cuenta exactamente como «cambiar de significado al cruzar»?

**Contexto.** IT-01 dejó una sola prueba (**H5**, duda **#9**): dos contextos son dos si al cruzar
la frontera hay que traducir. Volatilidad, facilidad de prueba y tamaño quedaron fuera. IT-02 aportó
el material (la tabla de homónimos) y una lección: la prueba se aplica mirando quién lee a quién.

Pero *cambiar de significado* admite al menos tres lecturas, y cada una traza un mapa distinto:

| Lectura | Ejemplo en este modelo | Dónde daría frontera |
|---|---|---|
| **la palabra** significa otra cosa | *paso*: la unidad declarada en Definición; la que se ejecuta o no se re-ejecuta en Ejecución | solo donde hay homónimo en la tabla |
| la palabra significa lo mismo pero **obedece otras reglas** | *intento*: en Ejecución se está llevando a cabo; en el Historial es un hecho con estado que ya no cambia | también donde un concepto cambia de invariantes |
| el concepto **pierde o gana información** al cruzar | *variable*: tiene valor en Resolución; en lo que lee el core, solo su marca (`DEC-01.6`) | también donde hay una proyección |

**Por qué importa.** La primera lectura es estricta y barata, y deja pocos contextos. La tercera es
la más generosa: casi cualquier par la pasa, y los nueve subdominios sobreviven como nueve contextos,
que es justo la sobre-descomposición contra la que avisa el plan. La prueba de todo lo que sigue
depende de cuál se elija.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | Solo **homónimo**: hay frontera si un término significa otra cosa al otro lado | Se verifica contra `lenguaje.md` sin juicio · Es la lectura literal de H5 | No ve el caso *intento*: misma palabra, mismo significado, y reglas opuestas a cada lado |
| **B** | **Homónimo o reglas distintas**: también hay frontera si el concepto conserva el significado y cambia lo que se puede hacer con él | Recoge *intento* · Sigue siendo lenguaje, no ingeniería | No basta con señalar la palabra: hay que escribir qué regla cambia, y eso pide más juicio |
| **C** | **B más proyección**: también hay frontera si al cruzar se pierde o se transforma información | Recoge *variable → marca*, que sostiene una promesa al core | Casi todo par pasa, y la carga de la prueba deja de pesar del lado de separar, al revés de lo que pide `plan-ddd.md` E3 |

**Y una segunda mitad, obligatoria: ¿qué pasa con los pares que no se hablan?** Si Simulación nunca
lee el Historial, entre los dos no hay cruce y la prueba no dice nada. **i** no hay cruce, luego son
distintos · **ii** no hay cruce, luego no hay evidencia, y se funden si su lenguaje se solapa.

**Lo que arrastra.** Sea cual sea la respuesta, las pruebas de separación de `bounded-contexts.md`
se escriben todas con la misma forma. Propuesta:

> *Al cruzar de X a Y, «T» deja de significar S₁ y pasa a significar S₂. Si X e Y se fusionaran,
> «T» significaría S₁ y S₂ dentro del mismo contexto.*

---

### Q-03.2 — ¿Desde qué mapa se aplica la prueba, y hacia qué lado pesa?

**Contexto.** El libro pone como objetivo que subdominios y contextos se correspondan **uno a uno**,
y admite que no siempre ocurre (`guia-ddd.md` §2). `plan-ddd.md` E3 añade que *la carga de la prueba
está del lado de mantenerlos separados*. Las dos cosas empujan en sentidos opuestos, y el punto de
partida decide cuál gana cuando la prueba no es concluyente.

**Por qué importa.** Una frontera dudosa sobrevive o cae según desde dónde se mire. Con nueve
candidatos de partida, sobrevive por inercia; con uno, cae por inercia.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Nueve candidatos**, uno por subdominio, y cada frontera se defiende o cae | Es el uno a uno que el libro pone como objetivo · El lenguaje ya está partido así (`DEC-02.1`) | La carga pesa del lado de fusionar: una frontera sin argumento claro para caer se queda |
| **B** | **Un solo contexto**, el motor entero, y cada partición se gana con la prueba | Lectura literal de *«la carga está del lado de separar»* · Nada se sobre-descompone por inercia | Obliga a re-probar fronteras que IT-01 ya sostuvo por experto y lenguaje (el core, Lanzamiento) · Un contexto con los nueve lenguajes nace con los homónimos dentro, así que la partición es inmediata |
| **C** | **Los homónimos trazan el mapa**: se parte por cada homónimo vivo y se funde lo que ninguno separa | Usa justo el entregable que IT-02 hizo para esto | Es responder A en `Q-03.1` por la puerta de atrás · Deja sin evidencia, a favor o en contra, a Simulación y a los genéricos |

**Y una segunda mitad: ¿se admiten las desviaciones del uno a uno?** ¿Un contexto puede realizar dos
subdominios, y un subdominio repartirse en dos contextos? **i** Sí, y cada desviación se escribe con
su razón. **ii** No. Con **ii**, algunas respuestas quedan descartadas de antemano: *Decidir* como
contexto (`Q-03.4` B), Simulación como modo (`Q-03.10` B) y Resolución fundida con otro (`Q-03.5` B
y C).

---

### Q-03.3 — Diagnóstico y el Historial: el core lee el historial. ¿Qué se traduce al cruzar?

**Contexto.** El core es el único contexto que el libro exige proteger por encima de todo. El modelo
dice que *«nunca habla con la fuente: lee el registro de lo que se usó»*, e IT-02 le hizo ceder la
palabra *evidencia* precisamente porque el lenguaje del Historial **entra** en el core. La 2.2 deja
dos hallazgos.

**1. Seis prometen y el core lee a pocos.** La cadena de garantías lista seis promesas: dos de
Definición, y una del Historial, de Lanzamiento, de Ejecución y de Resolución. Pero si el core solo
lee registros, lo que le prometen Ejecución y Resolución le **llega a través del Historial**. Una
garantía es del espacio del problema; una lectura, del de la solución. Visto así, la frontera que el
core cruza de verdad podría ser una sola, la del Historial, más la premisa de Definición y la puerta
de Lanzamiento.

**2. El mismo material, cortado en perpendicular.** Ejecución habla de los *recursos de un paso*
(instrucciones, variables y código de ese paso). El core habla de *ejes* (código, instrucciones y
variables de todo el despliegue). No son un sinónimo: es la misma materia agrupada de dos formas.

Candidatos a traducción al cruzar del Historial a Diagnóstico:

| En el Historial | En Diagnóstico |
|---|---|
| **despliegue**: un punto de retorno con padre | uno de los dos lados de una comparación |
| **evidencia**: por qué un paso no se re-ejecutó | con qué recursos se hizo de verdad ese paso: los de un intento anterior |
| registros **por paso** | el estado de cada **eje**: cambió o no cambió |
| **hash** del código, hash del pipeline | eje código, eje instrucciones |

**Por qué importa.** Si nada cambia de significado, el core es una consulta sobre el Historial, que
es justo lo que `Q-01.14` temía. El vocabulario de método (*eje*, *eliminación*, *deducción*) prueba
que hay **dominio**, pero no que haya **frontera**. Y si algo cambia, ésa es la traducción más
importante del mapa, y E4 tendrá que darle el patrón más fuerte.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Contexto propio**, y la prueba pasa por las filas 2 y 3: el core no razona sobre registros ni sobre pasos, razona sobre **ejes**, y convertir lo uno en lo otro es traducir | Protege el core, como manda el libro · La traducción queda nombrada: registros por paso → estado de tres ejes | La conversión es trabajo real, y alguien la paga (E4) |
| **B** | **Módulo de lectura dentro del Historial**: el core como consulta sofisticada | Sin traducción ni frontera | Contradice `DEC-01.2` —*«no presenta lo que otros saben: decide una atribución»*— y deja el core dentro de un supporting |
| **C** | **Contexto propio, con otra prueba**: el lenguaje de método basta para separarlo, sin buscar traducción | Se apoya en lo ya probado en IT-01 e IT-02 | Mezcla dos pruebas, y `Q-03.1` pide una sola |

**Lo que arrastra.** Si sale A, ¿el core lee **solo** del Historial (más la premisa y la puerta), o
también de Ejecución y de Resolución directamente? La respuesta cambia cuántas fronteras tiene el
core y cuánto trabajo le deja a E4. Y en cualquier caso hay que confirmar que *recursos de un paso* y
*eje* son dos conceptos, no un sinónimo entre dos contextos que se leen.

---

### Q-03.4 — Ejecución de Pipeline: ¿uno o dos contextos? *(dudas #1 y #9 de IT-01)*

**Contexto.** `DEC-01.3` dejó Ejecución como un solo subdominio, porque no hay un experto distinto
para decidir y otro para ejecutar. Nombró *Decidir / Planificación del Intento* como candidato a
contexto y difirió la partición hasta tener la prueba. **H5** descartó dos de los tres argumentos que
se dieron entonces: la volatilidad y el aislamiento en pruebas no son criterio. Hoy `lenguaje.md`
pone todos los términos de Ejecución en un solo apartado, sin ningún homónimo entre sus dos mitades.

La pregunta, con la prueba en la mano, es qué concepto cambia de significado al pasar de **decidir**
a **hacer**:

| | Decidir | Hacer |
|---|---|---|
| **paso** | sus **recursos** y su **regla**, comparados con la última vez | sus **comandos**, en un espacio de trabajo aislado |
| **variable** | si **cambió**; por `DEC-01.6`, sin ver su valor | su **valor en claro**, lo único que un comando entiende |
| **resultado** | *se re-ejecuta* · *no se re-ejecuta, porque…* | exitoso · fallido · y el material restaurado si falla |

**El nudo está en la fila 2.** La verificación de `DEC-01.6` dice: *«ningún consumidor obtiene el
valor en claro salvo el ejecutor del comando»*. Si decidir compara variables, compara **marcas**; si
hacer las usa, usa **valores**. Dentro de un solo contexto, *variable* significaría marca y valor a
la vez, que es justo el error. **Salvo** que el paso de valor a marca ocurra **antes**, en Resolución
de Variables, y ésta entregue a cada uno la cara que necesita.

Así que la pregunta de fondo es otra: **¿dónde ocurre el paso de valor a marca?**

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Un contexto.** Resolución entrega las dos caras (marca para decidir, valor para hacer), y dentro de Ejecución *variable* es lo que llega en cada momento. La partición es táctica (E9) | Coherente con `DEC-01.3` y con que no hay experto distinto · Una frontera menos | Ejecución maneja a la vez la marca y el valor de la misma variable, y hay que escribir por qué eso no es un homónimo interno |
| **B** | **Dos contextos**: *Decidir* nunca ve un valor y *Hacer* nunca ve una marca. La frontera es exactamente la de `DEC-01.6` | La promesa de no exponer se cumple por frontera y no por disciplina, que es el frente 2.B de `DEC-01.10` | *Decidir* necesita nombre y lenguaje propios, que hoy no tiene · Un subdominio en dos contextos: desviación que `Q-03.2` tiene que admitir |
| **C** | **Dos contextos cortados por el momento**: decidir antes del primer comando, hacer después | Parece la línea natural | **El modelo la hace imposible**: los recursos de un paso incluyen variables que produce un paso anterior del mismo intento, así que decidir y hacer se intercalan paso a paso · Además, `Q-02.23` ya separó por **objeto** y no por momento |

**Lo que arrastra.** A y B cumplen la verificación de `DEC-01.3`: ninguna reabre la lista de
subdominios. Si sale B, *Decidir / Planificación del Intento* necesita un nombre en español
(`DEC-02.2`) y su propio apartado en `lenguaje.md`. Y la respuesta queda atada a `Q-03.5`: A solo se
sostiene si Resolución es quien produce la marca.

---

### Q-03.5 — Resolución de Variables: ¿frontera propia o parte de otro? *(duda #2 de IT-02, «variable»)*

**Contexto.** *Variable* tiene cuatro sentidos declarados, y el modelo usa un quinto sin haberlo
recogido como término:

| Sentido | Ámbito |
|---|---|
| variable declarada | Definición |
| variable de salida, declarada con una expresión regular | Definición |
| variable: el **valor efectivo** | Resolución |
| la que un paso **produce** | Ejecución |
| la **marca** del valor: lo que se conserva y se compara | *sin término* — `dominio.md` §Resolución |

Al cruzar cada frontera, esto es lo que cambia:

| Cruce | Qué cambia |
|---|---|
| Definición → Resolución | lo escrito, por paso y por ambiente → lo vigente, tras aplicar precedencia y ámbito |
| Ejecución → Resolución | lo que produjo un comando → un valor efectivo de origen *«producido por un paso»* |
| Resolución → Ejecución | valor efectivo → valor en claro dentro de un comando (interpolación) |
| Resolución → lo que lee el core | valor efectivo → marca |

**Por qué importa.** Resolución es Supporting y le promete algo al core. Pero en la solución es la
frontera **más transitada** del mapa: cada comando de cada paso le pide interpolar, y cada paso puede
darle valores nuevos. Una frontera que se cruza en los dos sentidos en cada paso es donde el libro
dice que un contexto es demasiado pequeño, porque el coste de traducir es permanente. Y, a la vez,
lo que promete (comparar sin exponer) es exactamente lo que una frontera haría cumplir.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Contexto propio**: aquí viven el valor efectivo, la precedencia, el ámbito y la marca, y es el único sitio donde un valor se vuelve marca | La promesa al core tiene dueño con frontera · Es lo que `Q-03.4` A necesita | Frontera cruzada en ambos sentidos en cada paso: el mayor coste de traducción del mapa |
| **B** | **Dentro de Ejecución**: resolver es parte de ejecutar, y la marca se produce al dejar dicho lo que se hizo | Elimina la frontera más transitada | Ejecución maneja valores en claro **y** fabrica marcas, así que `DEC-01.6` pasa de frontera a disciplina · Un contexto con dos subdominios |
| **C** | **Dentro de Definición**: el DevOps declara y resuelve, es el mismo actor | Mismo experto en los dos lados | Mete dentro de un contexto el homónimo que la tabla declara como frontera, *«entre lo escrito y lo vigente»* |

**Lo que arrastra.** Sea cual sea la respuesta: dónde **nace** la marca y quién puede **tener** el
valor en claro. Y la marca necesita un término, porque hoy la usan tres apartados de `dominio.md` sin
que esté en `lenguaje.md`.

---

### Q-03.6 — Definición declara y otro aplica: ¿el término cruza igual? *(duda #5 de IT-02, «ámbito»)*

**Contexto.** La duda de *ámbito* no es un caso aislado. El modelo repite cuatro veces el mismo
patrón: Definición declara algo cuyo significado se ejerce en otro sitio.

| Término | Lo declara | Lo aplica | Dónde lo pone hoy `lenguaje.md` |
|---|---|---|---|
| **ámbito** | el DevOps, en el pipeline | Resolución: qué variables ve un paso | solo en Resolución |
| **regla** | un paso, en Definición | Ejecución: si hay que re-ejecutarlo | solo en Definición |
| **variable de salida** | Definición, con su expresión regular | Ejecución la produce; Resolución le da valor | en Definición |
| **orden de los ambientes** · **último ambiente** | Definición | Diagnóstico, para comparar · Lanzamiento, para saber dónde se lanza | en Definición |

**Por qué importa.** Si lo declarado significa lo mismo donde se aplica, el que aplica se amolda a
Definición y la frontera es barata. Si cambia (una regla declarada es texto; aplicada, es una
decisión con resultado), cada caso es una traducción, y Definición tiene frontera con casi todos.
Responder los cuatro por separado arriesga cuatro respuestas distintas a una misma forma, que es el
error que IT-02 evitó al tratar juntos *paso* y *variable*.

Y hay una restricción ya escrita: `DEC-02.5` descartó *«dos ámbitos con adjetivo»*.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **El término es de quien lo declara**, y quien lo aplica lo recibe con el mismo significado | Una sola regla para los cuatro · Definición es la fuente del vocabulario del DevOps | *Ámbito* se va de Resolución, y *último ambiente* ata el lenguaje de Lanzamiento al de Definición |
| **B** | **El término es de quien lo aplica**, y Definición solo transporta lo que el DevOps escribió | Cada término vive donde tiene reglas · Coherente con que `lenguaje.md` ya pone *ámbito* en Resolución | Las comprobaciones de Definición (que la expresión regular sea correcta, que toda variable usada esté declarada) necesitan **entender** lo que declaran, no solo transportarlo |
| **C** | **Homónimo declarado** en los cuatro: *lo declarado* frente a *lo aplicado* | Refleja que declarar y aplicar son dos actos | Enmienda `DEC-02.5` en el caso de *ámbito* y multiplica por cuatro la tabla de homónimos |

---

### Q-03.7 — «Paso» en el historial: ¿qué es, y cuántos pasos hay dentro del Historial? *(duda #1 de IT-02)*

**Contexto.** La tabla de homónimos da tres sentidos de *paso*. El del Historial es la **posición**
bajo la que se guardan los registros, y *«el historial identifica pasos que quizá ya no existen en el
pipeline de hoy»*.

IT-02 §9 hablaba de **cuatro** sentidos. El cuarto no está en la tabla, pero sí en la cadena de
garantías: el Historial promete que *«se sabe **con qué se hizo** — no con qué se haría hoy»*. O sea
que conserva, de cada paso, la declaración con la que se hizo, que es el *paso* de Definición
congelado.

Dentro del Historial pueden llamarse *paso*, por tanto, dos cosas: la **posición**, un nombre bajo
el que se acumulan registros a lo largo del tiempo, y la **declaración** tal como era. Dos sentidos
bajo una sola palabra y dentro de un mismo contexto es el error que el libro sí prohíbe.

**Y un tercer hallazgo** (2.4 #2): §Diagnóstico dice que el core consume *paso* **de Definición**,
pero el core nunca habla con la fuente. El paso que el core lee solo puede ser el del Historial.

**Opciones: qué es un paso dentro del Historial.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Solo la posición.** De *con qué se hizo* se conservan hashes, no pasos | Sin homónimo interno · El Historial no copia el lenguaje de Definición | *Con qué se hizo* deja de poder **mostrarse**: solo se sabe si coincide |
| **B** | **Posición y declaración, con dos palabras**: la posición es *paso* y la declaración congelada se llama de otra forma | Conserva todo lo que el core podría mostrar · Quita el homónimo | Término nuevo que ningún actor usa hoy |
| **C** | **Solo la declaración congelada**: el paso del Historial es el de Definición tal como era, y la posición se deriva de su nombre | *Paso* cruza sin traducir | Contradice la tabla, que ve una frontera justo ahí: el Historial recuerda pasos que ya no existen |

**Segunda mitad, obligatoria: ¿qué identifica a un paso a lo largo del tiempo?** **i** Su nombre:
renombrarlo crea otro paso y su historia se corta. **ii** Su lugar en el orden: reordenarlo crea
otro paso. **iii** Algo que asigna el Historial y que sobrevive a un renombrado; entonces alguien
tiene que decidir cuándo dos pasos con distinto nombre son el mismo, y esa regla hoy no tiene actor.

**Lo que arrastra.** Con la respuesta se corrige la línea de §Diagnóstico sobre de quién consume el
core el *paso*.

---

### Q-03.8 — Lanzamiento y el Historial: ¿quién guarda un lanzamiento, y dónde ocurre?

**Contexto.** `DEC-01.13` separó Lanzamiento del Historial con las tres patas del libro (experto,
lenguaje y oficio) y dejó una verificación para esta iteración: **ningún término de Lanzamiento
aparece en el lenguaje del Historial**. Pero el modelo vigente ya dice, en `dominio.md` §Historial:

> Guarda tres eslabones […]: intento, despliegue y **lanzamiento**. […] El historial sí guarda los
> lanzamientos, como guarda todo lo que pasó.

Tal como está escrita, la verificación **falla**, salvo que la frontera se trace de otra forma.

**Y hay una contradicción dentro de `lenguaje.md`** (2.4 #1): §Lanzamiento dice que un lanzamiento
*«ocurre en todos los ambientes»*, y §Definición que el último ambiente *«es donde ocurren los
lanzamientos»*. No es un matiz. Si se lanza en cualquier ambiente, *último ambiente* pierde su única
razón de estar en Definición, y Lanzamiento deja de depender del orden.

**Por qué importa.** Lanzamiento es el subdominio del actor que no está en la sala, y `dominio.md`
avisa de que su capacidad *«pierde siempre contra la del que sí»*. Si sus hechos viven en el
lenguaje del Historial, la separación que IT-01 ganó en el problema se deshace en la solución, que
es justo el riesgo que `DEC-01.13` nombró.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Lanzamiento guarda los suyos**: tiene memoria propia de lanzamientos, y el Historial solo sabe de intentos y despliegues | La verificación de `DEC-01.13` pasa al pie de la letra · El dueño del negocio tiene un contexto entero | Hay dos memorias, *«el historial guarda todo lo que pasó»* deja de ser verdad y `dominio.md` se enmienda · Quien entra por la puerta de Lanzamiento tiene que cruzar a otro contexto para llegar al despliegue |
| **B** | **El Historial guarda el hecho y Lanzamiento es dueño de la decisión.** *Lanzamiento* cruza cambiando de significado (en Lanzamiento, una decisión con nombre y etiquetas; en el Historial, un registro) y la verificación se reescribe: *ninguna **regla** de Lanzamiento aparece en el Historial* | Una sola memoria, como dice `dominio.md` · La traducción es real y tiene nombre | Enmienda la verificación de `DEC-01.13`, y hay que nombrarlo · El registro necesita las etiquetas, y *versión* y *nombre del lanzamiento* son términos de Lanzamiento |
| **C** | **Un contexto para los dos subdominios** | Sin traducción | Es lo que `DEC-01.13` descartó: dentro de un mismo contexto, lo del actor ausente pierde |

**Segunda mitad, obligatoria: ¿dónde ocurre un lanzamiento?** **i** Solo tras un despliegue en el
último ambiente. **ii** En cualquier ambiente. Con cualquiera de las dos se corrige una línea de
`lenguaje.md`, y la respuesta decide si Lanzamiento lee a Definición.

---

### Q-03.9 — Rollback: ¿de quién es la operación, y de dónde salen los recursos? *(duda #3 de IT-02)*

**Contexto.** `lenguaje.md` pone *rollback* en Ejecución: *«volver a un estado anterior. Crea un
despliegue nuevo cuyo padre no es el último»*. Pero *padre* y *punto de retorno* son términos del
Historial. Y `DEC-02.14` dice que comparar y volver atrás son dos operaciones: un rollback *«elige
hacia dónde regresar»*.

Lo que decide la frontera el modelo no lo dice: **¿con qué** se hace el despliegue nuevo? *Volver a
un estado anterior* admite dos lecturas.

| Lectura | Los recursos del nuevo despliegue son… | Quién los pone delante |
|---|---|---|
| volver a **lo que se hizo** | los del despliegue elegido: su código, sus instrucciones y sus variables, tal como eran | el **Historial**, que *«sabe con qué se hizo»* |
| volver a **desde dónde se parte** | los de hoy, con el despliegue elegido como padre | Suministro, como en cualquier intento |

**Por qué importa.** Con la primera lectura, en un rollback Ejecución toma el material del Historial
en vez de Suministro. Es un cruce que no existe en ningún otro intento, y *recursos de un paso*
tendría dos procedencias con significado distinto: obtenidos o recuperados. Con la segunda lectura,
un rollback es solo elegir padre, y pertenece casi entero al Historial.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Es de Ejecución**: un rollback es un intento con padre elegido, y el Historial solo lo anota | Coherente con donde lo pone `lenguaje.md` · Un rollback es un acto, y los actos son de Ejecución | Ejecución tiene que entender *padre* y *punto de retorno*, dos términos del Historial |
| **B** | **Es del Historial**: elegir un punto de retorno es recorrer el árbol, y Ejecución recibe un intento más sin saber que es un rollback | *Rollback* y *padre* viven juntos · Ejecución no aprende nada nuevo | Un acto con intención, *volver a un estado anterior*, queda en el contexto de la memoria, que no efectúa nada |
| **C** | **Partido**: el Historial convierte el punto de retorno en unos recursos y Ejecución los ejecuta. *Rollback* es la palabra del actor y no es de ningún contexto | Cada mitad queda donde tiene sus reglas | Un término sin contexto es lo que `DEC-02.1` no admite: todo término vive en un ámbito |

**Segunda mitad, obligatoria: ¿con qué recursos?** **i** Los del despliegue elegido. **ii** Los de
hoy, con el padre elegido.

---

### Q-03.10 — Simulación: ¿contexto propio o modo de Ejecución? *(duda #11 de IT-01)*

**Contexto.** Es subdominio propio desde `DEC-01.5`, por actor, momento y propósito. Finge una sola
cosa, los comandos; la interpolación y la comprobación las hace de verdad. No tiene efectos y no
guarda nada. `Q-01.16` dejó abierta la pregunta de la solución. Su lenguaje tiene dos términos.

Así queda la prueba sobre los términos que Simulación comparte con Ejecución:

| Término | En Ejecución | En Simulación |
|---|---|---|
| **intento** | tiene identificador y estado, y sabe cuál fue el anterior; queda en el historial | *sin decidir*: si no se guarda nada, no tiene anterior ni queda en ningún sitio |
| **no se re-ejecuta** | se decide contra la última vez, leyendo el historial | *sin decidir*: ¿recorre todos los pasos, o lee el historial para saber cuáles se re-ejecutarían? |
| **variable que un paso produce** | sale de la salida real de un comando | *sin decidir*: si el comando es fingido, su salida también, y el paso siguiente interpola un valor que nadie produjo |
| **paso** | la unidad que se ejecuta o no se re-ejecuta | la unidad que se recorre |

**La fila 3 es la que decide.** El modelo dice que la interpolación es real, pero la variable que
produce un comando fingido tiene que **fabricarse**, seguramente a partir de la expresión regular de
su variable de salida. Es un valor que no existe en ningún otro sitio, y `DEC-01.5` garantiza que
ningún valor simulado entra en el core.

**Por qué importa.** Si es un modo, *intento* y *variable producida* tienen dos significados dentro
de Ejecución: el homónimo interno. Si es un contexto, duplica casi todo el modelo de Ejecución, y el
libro trata compartir modelo como un mal necesario (cómo compartirlo es de E4).

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Contexto propio**, con palabra propia para lo que recorre y para la variable fingida | Ningún homónimo interno en Ejecución · La garantía de `DEC-01.5` queda en una frontera | Duplica casi todo el mecanismo, y E4 tendrá que decir cómo se comparte |
| **B** | **Modo de Ejecución**: un intento sin efectos | Un solo mecanismo | *Intento* y *variable producida* significan dos cosas dentro de Ejecución, y hay que escribir por qué eso no es un error |
| **C** | **Contexto declarado y sin frontera trazada** hasta que tenga usuario real | El libro desconfía de modelar lo que no existe, y `dominio.md` dice que no tiene usuario todavía | Una duda diferida más, y E4 recibe una arista sin extremo |

**Segunda mitad, independiente de la opción: ¿Simulación lee el historial?** Si lo lee, tiene con el
Historial una arista que el modelo no declara. Si no, recorre siempre todos los pasos, y *«¿va a
funcionar?»* no dice nada de lo que se re-ejecutaría.

---

### Q-03.11 — Los dos genéricos: ¿son contextos?

**Contexto.** Suministro de Fuentes tiene dos términos (*fuente*, *hash*); Sincronización del
Historial, uno (*sincronizar*). `lenguaje.md` lo dice abiertamente: Sincronización *«no tiene casi
vocabulario propio: es la señal de que es genérico»*.

Un bounded context es la frontera de un **modelo**. Con un verbo de lenguaje, cabe preguntar si hay
modelo que acotar, y el mismo hecho admite dos lecturas contrarias:

- **Sin lenguaje no hay contexto**: lo que no tiene modelo es mecanismo de quien lo usa.
- **Sin lenguaje, la traducción es forzosa**: justo porque no entiende de registros, todo lo que le
  llega tiene que convertirse en algo que sí entienda, una carga opaca que llevar y traer. Con la
  lectura C de `Q-03.1`, eso es cambiar de significado.

**Y Suministro tiene un homónimo que la tabla no recoge** (2.4 #3):

| Término | En Definición | En Suministro |
|---|---|---|
| **pipeline** | la declaración: variables, instrucciones de cada paso y ambientes | una **fuente**: un repositorio con su hash, cuyo contenido no se entiende |
| **código del producto** | *(en Diagnóstico)* un **eje** | una **fuente** |

Al cruzar de Suministro a Definición, un repositorio se convierte en una declaración, y eso es una
traducción.

**Por qué importa.** Clasificarlos como Generic no decide si son contextos: el subdominio sigue
existiendo en el problema, lo que se pregunta es si merece frontera en la solución. Cada frontera
tiene coste permanente, y pagarlo por algo que se puede comprar va contra el sentido de un genérico.
Pero no pagarlo mete su vocabulario, por pequeño que sea, dentro de otro contexto.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Los dos son contextos**, finos y genéricos | Aplica la prueba sin excepciones · E10 los evalúa como piezas separadas | Un contexto con un verbo de lenguaje; se paga frontera por algo que se compra |
| **B** | **Ninguno es contexto**: son mecanismo detrás de quien los usa, Suministro detrás de Ejecución y Sincronización detrás del Historial | Menos fronteras · Coherente con que *«no prometen nada al core»* | *Pipeline* significaría declaración y repositorio dentro de Ejecución · El Historial tendría que hablar de transporte y destinos |
| **C** | **Asimétrico**: Suministro es contexto, porque tiene lenguaje y el homónimo *pipeline* lo prueba; Sincronización no | Cada uno según su propia evidencia | Hay que decir por qué la carga opaca no cuenta como traducción en un caso y el repositorio sí en el otro |

**Lo que arrastra.** *Pipeline* entra en la tabla de homónimos con la frontera que señala o, con B,
pasa a ser un error interno que hay que resolver.

---

### Q-03.12 — La autoría: ¿quién promete *quién* cambió qué y *cuándo*? *(duda #15 de IT-01)*

**Contexto.** Es la única fila de la cadena de garantías sin dueño, y está así a propósito:
*«se sabe quién hizo cada cambio y cuándo — pendiente»*. `Q-01.23` dejó dos candidatos: Suministro,
que tiene delante el material y su historia, y el Historial, que anota quién pidió cada intento.

**Al abrir apareció que «quién» nombra dos cosas** (2.4 #4):

| Sentido | Ejemplo | Quién lo sabe |
|---|---|---|
| quién **hizo el cambio** en una fuente | *«`DB_POOL_SIZE` la cambió X el martes, en el pipeline»* | la historia del repositorio: Suministro |
| quién **pidió el intento** | *«ese despliegue lo pidió Y»* | el Historial |

Y hay un tercer caso: una variable **producida por un paso** no tiene autor. Su *quién* es el intento
que la produjo.

`Q-01.30` ya resolvió un caso con esta misma forma: el core no habla con la fuente; Suministro
calcula el hash, el Historial lo conserva y el core compara registros. El mismo patrón serviría para
la autoría.

**Por qué importa.** Por `DEC-01.14`, un Generic no promete nada. Si Suministro prometiera la autoría
directamente, la regla de clasificación se rompería, igual que se habría roto en `Q-01.30`. Y si la
promete el Historial, el registro de un intento tiene que llevar autores de cambios que ese intento
no hizo.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Promete el Historial, con el patrón del hash**: Suministro obtiene el autor al ejecutar, el Historial lo conserva junto a *con qué se hizo* y el core lo lee | Reutiliza lo resuelto en `Q-01.30` · La regla de clasificación aguanta | Obliga a partir *quién* en dos términos, porque el Historial va a guardar los dos |
| **B** | **Promete Suministro**: la autoría es historia del repositorio | Es quien lo sabe de primera mano | Rompe *«los genéricos no prometen»* y reabre `DEC-01.14` · El core hablaría con la fuente, contra `DEC-01.12` |
| **C** | **Sigue sin dueño** y se difiere a E6, donde los escenarios dirán si el core la necesita | No se asigna por comodidad | Sería la segunda iteración que la difiere, y E4 recibiría una promesa sin contexto que la cumpla |

**En las tres hay que nombrar los dos sentidos de *quién*.** Candidatos, para arrancar y no para
copiar: *autor* (del cambio) y *solicitante* (del intento).

---

### Q-03.13 — Lo que queda: nombres, «estado», un término que falta y la forma del documento

**Contexto.** Cierre de limpieza. Cinco asuntos pequeños que, sin decisión, se quedarían en el
documento por inercia.

| # | Asunto | Estado |
|---|---|---|
| 1 | **Nombre de los contextos.** ¿Un contexto se llama como el subdominio que realiza? Si realiza dos, o un subdominio queda en dos, hace falta nombre nuevo, y en español (`DEC-02.2`) | sin decidir |
| 2 | **«Estado».** IT-02 §9 **#2** lo difirió con *dos* sentidos, pero la tabla vigente de `lenguaje.md` dice que *no queda segundo sentido vivo*. ¿Se confirma que no hay nada que decidir? | probablemente cerrado |
| 3 | **El hash de un paso.** `DEC-01.4` dejó la identidad del paso *en Ejecución*, y `DEC-02.8` dice que la palabra es común y cada hash concreto tiene dueño. Pero `lenguaje.md` solo define *hash* en Suministro, sobre un repositorio, mientras Ejecución compara *por paso*. ¿Ejecución incorpora *hash de los recursos de un paso*? | falta en el lenguaje |
| 4 | **La frase de apertura de §Diagnóstico** dice que compara *«datos del historial (intento, despliegue, lanzamiento)»*; `DEC-02.4` dice que todo escenario nombra dos despliegues y nunca un lanzamiento | se corrige |
| 5 | **La forma de `bounded-contexts.md`.** Propuesta, por contexto: propósito · subdominios que realiza · términos propios · términos que consume y de quién · prueba de separación contra cada vecino. ¿Falta o sobra algo? En particular: ¿se dibujan las aristas sin patrón, sabiendo que las completa E4? | propuesta |

---

## 3.bis Rondas de validación

### Ronda 2 — Qué encontró la validación

Doce respuestas; `Q-03.13` quedó sin responder. Casi todas cierran lo que preguntaban. La lupa
encontró **seis cosas**, y una de ellas cambia la naturaleza de un contexto:

1. **`Q-03.8` cambia qué es el Historial.** *«Entendiendo historial como una BD»*: una base de datos
   es **cómo se guarda** algo, no un modelo. En el libro, dónde y cómo se guarda un modelo es parte
   de la solución *de un contexto* (sus repositorios); nunca es un contexto. Leída al pie de la
   letra, deja al subdominio Historial sin contexto y a sus reglas (padre, anterior, cantidad de
   intentos, *«el despliegue no admite grados»*) sin dueño. Y `plan-ddd.md` E5 ya tiene escrita esa
   misma pregunta para el Historial: si su secuencia de hechos es *el modelo* o *el mecanismo de
   persistencia*. → `Q-03.14`.
2. **`Q-03.3` y `Q-03.8`, juntas, convierten al Historial en la única puerta del core.** Todo lo que
   cinco contextos le prometen al core tiene que quedar conservado ahí, y hay que decir con el
   significado de quién. → `Q-03.14`, segunda cara.
3. **La explicación de `Q-03.7` es mejor que su opción, y choca con ella en un punto.** La identidad
   por comandos disuelve el problema de *quién decide que dos pasos son el mismo*. Pero *«el paso
   cambió»* necesita algo que siga siendo el mismo a través del cambio. → `Q-03.16`.
4. **Dos respuestas se sostienen en lecturas que `Q-03.1` no admite.** `Q-03.11` A se argumentó con
   *«los dos traducen»*, y en Sincronización eso es proyección a carga opaca: la lectura C,
   descartada. Y `Q-03.4` A necesita que Ejecución maneje la marca y el valor de una misma variable,
   que con la lectura B —reglas distintas— es un homónimo interno. → `Q-03.19` · `Q-03.15`.
5. **`Q-03.6` B deja a la comprobación sin significado que comprobar.** La comprobación vive en
   Definición y verifica términos que ahora son de otros. → `Q-03.17`.
6. **Mitades sin responder**: los pares que no se hablan (`Q-03.1`), dónde ocurre un lanzamiento
   (`Q-03.8`), con qué recursos se vuelve atrás (`Q-03.9`), si Simulación lee el historial y sus dos
   palabras (`Q-03.10`), el nombre de la marca (`Q-03.5`), el de la declaración congelada
   (`Q-03.7`) y los dos *quién* (`Q-03.12`). Y `Q-03.13` entera.

**Y una constatación que no es pregunta.** De nueve subdominios salen **nueve contextos**, sin una
sola fusión. Es consecuencia directa de `Q-03.2` A, que puso la carga de la prueba del lado de
**fusionar**, justo al revés del encuadre de `plan-ddd.md` E3. Es legítimo, y el argumento del
proyecto que empieza de cero lo sostiene, pero es una enmienda al encuadre y se nombra (`DEC-03.2`).
Lo que exige a cambio es que **cada prueba de separación esté escrita**. La más débil todavía no lo
está: `Q-03.19`.

Ocho preguntas nuevas, en la misma serie.

---

### Q-03.14 — «El historial como una BD»: ¿contexto con modelo, o sitio donde se guarda?

**Contexto.** `Q-03.8` responde que el lanzamiento se guarda en el Historial, *«entendiendo historial
como una BD»*, y que el lanzamiento es *«un concepto lógico similar a un despliegue»*. La segunda
mitad encaja con `DEC-02.3`: el lanzamiento es un eslabón como los otros dos. La primera admite dos
lecturas que dan dos mapas distintos:

| Lectura | Qué es el Historial | Sus reglas (padre, anterior, cantidad de intentos) | Qué lee el core |
|---|---|---|---|
| **metáfora**: *la* memoria, una sola, donde queda todo lo que pasó | un contexto cuyo modelo son **registros**: intentos, despliegues y lanzamientos como hechos | se quedan en el Historial | el modelo del Historial, y lo traduce a ejes (`Q-03.3`) |
| **literal**: una base de datos | **no es contexto**: es donde otros contextos guardan lo suyo | cada eslabón es de quien lo produce: intento y despliegue de Ejecución, lanzamiento de Lanzamiento | lo guardado por cinco contextos, sin modelo en medio |

**Y la segunda cara.** Con `Q-03.3`, el core lee **solo** el Historial. Todo lo que otros le
prometen tiene que estar conservado ahí:

| Tiene que estar en el Historial | Lo promete | Porque el core lo necesita para |
|---|---|---|
| por qué un paso no se re-ejecutó (*evidencia*) | Ejecución | saber con qué recursos se hizo de verdad cada paso |
| la marca de cada variable | Resolución | saber qué variables cambiaron |
| el hash del código y el del pipeline · el autor | Suministro | los ejes código e instrucciones, y el sustento |
| el lanzamiento, con su versión y su nombre | Lanzamiento | la puerta de entrada desde el cliente |
| el orden de los ambientes con el que se desplegó | Definición | saber que producción viene de staging |

**Por qué importa.** Con la lectura literal, el core deja de leer *un contexto* y pasa a leer
**los datos de cinco**. Se queda sin modelo en medio que traduzca, y la tabla de traducción de
`Q-03.3` pasa a tener cinco orígenes en vez de uno. Es el acoplamiento más fuerte que hay: modelos
atados por cómo se guarda cada uno. Además, E3 cerraría la pregunta que `plan-ddd.md` reserva para
E5. Con la metáfora, en cambio, hay que decir qué hace el Historial con contenido que no es suyo.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Metáfora: contexto de registros.** Un registro es un hecho con su tipo, y el Historial es dueño de la **forma** (a qué intento pertenece, cuál fue el anterior, el padre). Lo que el hecho **dice** sigue siendo de quien lo produjo | El subdominio conserva contexto y reglas · El core traduce desde un solo modelo · E5 sigue abierto | El Historial guarda contenido que no interpreta, y hay que escribir por qué eso no mete cinco lenguajes dentro |
| **B** | **Literal: no es contexto**, sino persistencia compartida, y cada eslabón es de quien lo produce | Refleja cómo se piensa la memoria en la práctica | El subdominio Historial se queda sin contexto y sus reglas se mudan a Ejecución · El core lee datos de cinco contextos · Decide E5 desde E3 |
| **C** | **Metáfora, y la pregunta modelo o persistencia se difiere a E5**, como la tiene escrita el plan | No adelanta E5 | Deja sin decir qué es un registro, que es justo lo que el core traduce |

**La verificación de `DEC-01.13` depende de esta respuesta.** Con A o C, se reescribe como *«ninguna
**regla** de Lanzamiento aparece en el Historial»*: el lanzamiento es un tipo de registro, y la
decisión, el significado de sus etiquetas y el actor ausente son de Lanzamiento. Con B pasa al pie
de la letra, pero porque el Historial se queda sin lenguaje.

**Y la mitad pendiente de `Q-03.8`: ¿dónde ocurre un lanzamiento?** *«Similar a un despliegue»*
admite las dos respuestas. **i** Solo tras un despliegue en el último ambiente. **ii** En cualquier
ambiente. Una de las dos líneas de `lenguaje.md` es falsa hoy.

---

### Q-03.15 — ¿Qué ve Ejecución de una variable? *(`Q-03.1` B × `Q-03.4` A × `Q-03.5` A)*

**Contexto.** Tres respuestas correctas una a una, a las que les falta la frase que las hace encajar:

- `Q-03.1` B: la misma palabra con reglas distintas **es** frontera.
- `Q-03.5` A: el valor se convierte en marca **solo** en Resolución.
- `Q-03.4` A: Ejecución es **un** contexto, y Resolución le entrega *las dos caras*.

Dentro de Ejecución, la marca y el valor de una misma variable obedecen reglas **opuestas**. El valor
puede entrar en un comando y no puede conservarse; la marca puede conservarse y compararse y no puede
entrar en un comando. Si las dos se llaman *variable* dentro de Ejecución, por `Q-03.1` B eso es un
homónimo interno.

Y además, con `Q-03.3`, las marcas tienen que **llegar al Historial**. ¿Quién las lleva?

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Ejecución solo ve valores.** La marca nunca entra en Ejecución: Resolución la deja directamente en el Historial, y a Ejecución le da, para decidir, un solo dato por paso: si las variables de ese paso cambiaron | *Variable* tiene un solo sentido en Ejecución · No exponer y comparar viven en el mismo contexto | Aparece una arista nueva, Resolución → Historial · Resolución tiene que saber de pasos e intentos para dejar la marca en su sitio |
| **B** | **Ejecución lleva las marcas sin leerlas**: las recibe como carga opaca junto a los valores y las entrega al Historial al dejar dicho lo que se hizo | Sin arista nueva: todo lo del intento sale por Ejecución | Una carga opaca que no se entiende es la proyección que `Q-03.1` no aceptó como prueba, y hay que decir por qué dentro de un contexto sí vale |
| **C** | **Dos palabras en Ejecución**: *variable* para el valor y otra para la marca | Sin homónimo, porque son dos términos · Ejecución decide variable a variable | La palabra de la marca pasa a ser de dos contextos, Resolución y Ejecución, y hay que declararlo |

**Y la palabra, que `Q-03.5` dejó pendiente.** `dominio.md` usa la marca del valor en tres sitios y
`lenguaje.md` no la recoge. **i** *marca*. **ii** *hash de la variable*: `DEC-02.8` definió el hash
para repositorios, pero su trabajo es el mismo, responder *«¿cambió?»*, y la palabra es común con
dueño en cada instancia. **iii** Otra.

---

### Q-03.16 — La identidad de un paso: ¿lo que hace, o cómo se llama? *(`Q-03.7`)*

**Contexto.** `Q-03.7` respondió con una cuarta vía que no estaba entre las opciones:

> Un paso se identifica por lo que hace, sus comandos en la definición. Para facilidad, ese conjunto
> de comandos tiene un nombre. Si esos comandos cambian, significa que el paso cambió.

Lo que gana es real: nadie tiene que decidir cuándo dos pasos son el mismo, porque lo decide el
contenido. Renombrar o reordenar no corta ninguna historia, y el nombre queda como etiqueta.

**Pero la frase tiene dos mitades que tiran en sentidos distintos.** *«Se identifica por sus
comandos»* dice que un paso **es** su contenido. *«Si los comandos cambian, el paso cambió»* dice que
un paso **sigue siendo el mismo** aunque su contenido cambie. Son las dos formas de existir que el
libro separa: algo con identidad que persiste a través de sus cambios, o algo que *es* sus atributos
y que, si cambia, es otra cosa. Dan modelos distintos, y los casos donde se ve son estos:

| Caso | Identidad por nombre | Identidad por comandos |
|---|---|---|
| se renombra, con los mismos comandos | otro paso: la historia se corta | el mismo paso |
| mismo nombre, otros comandos | el mismo paso, que **cambió** | **otro** paso: el anterior dejó de existir |
| dos pasos con los mismos comandos y distintas variables (dos piezas desplegadas con el mismo comando) | dos pasos | **uno solo** |

La tercera fila no es rara: lo normal es desplegar cosas distintas con el mismo comando y variables
distintas por paso.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **El nombre dice cuál es; los comandos dicen si cambió** | Se puede decir *«el paso cambió»* · Dos pasos con los mismos comandos siguen siendo dos | Renombrar corta la historia, y hay que asumirlo o volverlo un error de la comprobación |
| **B** | **Los comandos son la identidad; el nombre es etiqueta** (la respuesta literal) | Renombrar y reordenar no rompen nada · Nadie decide equivalencias | La tercera fila funde dos pasos en uno · *«Cambió»* solo se puede decir a través de la etiqueta, que no es identidad |
| **C** | **Los comandos y las variables del paso son la identidad** | Resuelve la tercera fila | Las variables varían por ambiente: el mismo paso sería otro en staging y en producción, y la premisa del core (las instrucciones no varían por ambiente) deja de poder decirse paso a paso |

**Consecuencia sobre `Q-03.7`.** Con B, la *posición* bajo la que se guardan los registros **es** la
declaración: posición y declaración coinciden, y la respuesta a `Q-03.7` pasa a ser su opción C.
Con A, `Q-03.7` B se sostiene y falta la palabra para la declaración congelada, que no se dio.

---

### Q-03.17 — Si el término es de quien lo aplica, ¿qué comprueba Definición? *(`Q-03.6`)*

**Contexto.** `Q-03.6` B mueve cuatro términos:

| Término | Queda en |
|---|---|
| **ámbito** | Resolución. Cierra la duda **#5** de IT-02, y `lenguaje.md` no cambia |
| **regla** | Ejecución; sale de Definición |
| **variable de salida** | **dos** aplicadores: Ejecución la produce y Resolución le da valor |
| **orden de los ambientes** · **último ambiente** | **dos** aplicadores: Diagnóstico, a través del Historial, y Lanzamiento |

La objeción que B llevaba escrita no se respondió, y es seria. La **comprobación** (`DEC-02.7`) es
de Definición y verifica que toda variable usada esté declarada *como variable de salida de un paso
anterior* o en las del ambiente, y que las expresiones regulares sean correctas. Para eso tiene que
saber qué es una variable de salida, qué es estar disponible y en qué orden van los pasos. Si esos
términos son de otros, Definición comprueba cosas cuyo significado no es suyo.

**Por qué importa.** La comprobación es lo que hace a un pipeline *verificable de principio a fin
sin tocar la nube*. Es lo que el DevOps quiere antes de publicar, y es el primer paso de Simulación.
Si se parte, se parte esa promesa.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Definición comprueba la forma; el significado es de quien aplica.** La comprobación mira que lo escrito esté bien formado y bien referenciado, sin saber qué hará cada cosa. Un término con dos aplicadores es un homónimo declarado entre ellos | B se mantiene sin excepciones · La comprobación sigue sin ejecutar nada | *«Bien referenciado»* ya es significado: que una variable de salida de un paso anterior esté disponible es una regla de Resolución |
| **B** | **Excepción escrita**: *variable de salida* y *orden de los ambientes* son de Definición, porque su comprobación los necesita; *ámbito* y *regla* son de quien los aplica | Cada término queda donde se usa de verdad | `Q-03.6` deja de ser una regla y se vuelve una lista, que es lo que tratarlos juntos quería evitar |
| **C** | **La parte de la comprobación que necesita significado se va con el significado**: que las variables estén disponibles lo comprueba Resolución antes del intento; Definición se queda con formato, expresiones regulares y pasos sin comandos | B sin excepciones y sin homónimos | Enmienda `DEC-02.7`: la comprobación deja de ser una sola, y el DevOps necesita dos contextos para saber si su pipeline está bien |

---

### Q-03.18 — Rollback: ¿con qué recursos, y qué tiene que guardar el Historial para poder volver? *(`Q-03.9`)*

**Contexto.** `Q-03.9` A: el rollback es de Ejecución, y el Historial solo lo anota. Quedó sin
responder con qué recursos se hace el despliegue nuevo, y esa mitad choca ahora con `DEC-02.8`: el
hash *«responde a una sola pregunta: ¿cambió algo?»*. Un hash no devuelve contenido.

| Si los recursos son… | Qué tiene que existir |
|---|---|
| **i** los del despliegue elegido | que el Historial guarde, de cada despliegue, algo con lo que **recuperar** el código y las instrucciones de entonces. El hash no sirve: solo dice si coincide. Las variables tampoco se recuperan de su marca (`DEC-01.6`): las declaradas se resolverían de nuevo desde el pipeline de entonces, y las producidas se volverían a producir |
| **ii** los de hoy, con el padre elegido | nada nuevo. Pero entonces *volver a un estado anterior* no vuelve a nada: si el fallo está en el código de hoy, el rollback lo despliega otra vez |

**Por qué importa.** Con i nace una promesa nueva del Historial (se sabe con qué se hizo **y se puede
volver a tenerlo delante**) y, por `DEC-02.8`, un término nuevo; y Suministro tiene que saber traer
versiones anteriores, no solo la de hoy. Con ii, *punto de retorno* deja de nombrar un sitio al que
volver.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **i**: el Historial guarda, además del hash, **una referencia recuperable** de cada fuente, y Suministro sabe traer esa versión | El rollback vuelve de verdad · *Punto de retorno* conserva su sentido | Promesa y término nuevos en el Historial · Suministro deja de trabajar solo con el presente |
| **B** | **ii**: los recursos de hoy, con el padre elegido | Nada nuevo | *Punto de retorno* y `DEC-02.14` se quedan sin contenido: se elige un padre y no se vuelve a nada |
| **C** | **El actor elige qué ejes vuelven atrás**: el código, las instrucciones o los dos | Refleja que un rollback de pipeline y uno de código son decisiones distintas | El rollback deja de ser un solo acto, y un despliegue mixto tiene padre para un eje y no para otro |

**Segunda mitad: ¿con qué palabra dice Ejecución a qué despliegue vuelve?** *Padre* es del
Historial, donde nombra una relación del árbol; en Ejecución sería un parámetro del intento. Con
`Q-03.1` B, si la regla cambia, es un homónimo que hay que declarar; si no, Ejecución usa palabra
propia.

---

### Q-03.19 — Sincronización del Historial: ¿cuál es su prueba con la lectura B? *(`Q-03.11`)*

**Contexto.** `Q-03.11` A se argumentó con *«los dos traducen»*. Suministro pasa la prueba B por su
propio pie: *pipeline* cambia de significado entre declaración y repositorio. Sincronización, en
cambio, **no comparte ninguna palabra** con el Historial, así que no hay homónimo. Su caso era *«lo
que le llega se convierte en carga opaca»*, que es proyección, la lectura C que `Q-03.1` descartó.

Con B le queda una vía: que **lo mismo** obedezca **reglas distintas** a cada lado. Pero para decir
«lo mismo» hace falta nombrarlo del lado de Sincronización, y su lenguaje tiene un solo verbo.

**Por qué importa.** Es la única de las nueve fronteras sin prueba escrita. Con la carga de la
prueba del lado de fusionar (`Q-03.2` A), sobrevive por defecto, que es exactamente la inercia de la
que avisaba el encuadre del plan. Y con la lectura literal de `Q-03.14`, Sincronización sería la
réplica de una base de datos: técnica pura.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Contexto, con prueba por reglas**: lo que el Historial conserva cruza como otra cosa con nombre propio en Sincronización, con reglas propias (estar disponible aquí, llegar íntegro) que el Historial no tiene | La prueba se escribe con B sin excepciones · E10 lo evalúa como pieza aparte | Hay que darle vocabulario a un genérico que `lenguaje.md` presenta como genérico precisamente por no tenerlo |
| **B** | **La prueba se amplía para los genéricos**: un genérico pasa si **no entiende** lo que recibe, y la proyección a carga opaca cuenta solo aquí | Refleja lo que es un genérico | Una prueba con excepción deja de ser una sola (**H5**) |
| **C** | **No es contexto**: es cómo el Historial está disponible en otra máquina, y `Q-03.11` pasa a ser su opción C | Honesto con la prueba elegida · Una frontera menos | Enmienda `Q-03.11` · El Historial tiene que hablar de transporte |

---

### Q-03.20 — Simulación: ¿lee el historial, y cómo se llama lo que recorre? *(`Q-03.10`)*

**Contexto.** `Q-03.10` A: contexto propio, con palabra propia para lo que recorre y para la variable
que produce un comando fingido. Quedaron sin responder las dos palabras y la segunda mitad.

| Si Simulación lee el historial | Si no lo lee |
|---|---|
| puede decir qué pasos se re-ejecutarían, pero para eso necesita las reglas de re-ejecución de Ejecución, y duplica también su mitad de decidir | recorre siempre todos los pasos y responde *«¿funciona el pipeline?»*, que es la pregunta que `dominio.md` le asigna |

Y un dato que empuja hacia la segunda columna: Simulación sirve al DevOps **antes de publicar**. De
un pipeline todavía sin publicar no hay *«última vez»* en ningún ambiente.

**Opciones.** **A** No lee el historial: recorre siempre todo. **B** Lo lee y dice qué se
re-ejecutaría. **C** Lo lee solo si se le pide un ambiente concreto.

**Las dos palabras.** Para lo que recorre, `lenguaje.md` ya tiene un sustantivo, **simulación**: *una
simulación* puede ser la entidad, igual que *un intento* lo es en Ejecución, sin inventar nada. Para
lo que devuelve un comando fingido: **i** *salida simulada* · **ii** *valor simulado* · **iii** otra.

---

### Q-03.21 — Los dos *quién* y los pares que no se hablan

**Contexto.** Dos asuntos que quedaron sin responder y no abren frontera.

| # | Asunto | De |
|---|---|---|
| 1 | **Los dos sentidos de *quién***: quien hizo el cambio en una fuente y quien pidió el intento. Con `Q-03.12` A, el Historial guarda los dos, así que dentro de él tienen que llamarse distinto. Candidatos: *autor* y *solicitante* | `Q-03.12` |
| 2 | **Los pares que no se hablan.** Con `Q-03.3`, el core ya no habla con cinco contextos. **i** Sin cruce, son distintos. **ii** Sin cruce no hay evidencia, y se funden si su lenguaje se solapa. Para el core da igual: su lenguaje no se solapa con ninguno. Importa en Simulación–Historial si `Q-03.20` = A | `Q-03.1` |

**Recordatorio.** `Q-03.13` sigue sin respuesta y entra en el mismo bloque.

---

### Ronda 3 — Qué encontró la validación

Ocho respuestas: siete de la ronda 2 más `Q-03.13`. `Q-03.21` sigue sin responder. La lupa encontró
menos que en la ronda anterior, pero no poco: **la primera fusión del mapa**, **tres choques**
entre respuestas que son correctas por separado, y **un hueco del modelo** que solo se ve con las
tres respuestas sobre variables delante.

1. **La primera fusión.** `Q-03.19` C: Sincronización del Historial no es contexto. El mapa baja a
   **ocho contextos**, y la desviación que admitió `Q-03.2` tiene su primer caso: el Historial
   realiza dos subdominios, uno supporting y uno generic. No es pregunta, se escribe (`DEC-03.11`).
   Y demuestra que la constancia de la ronda 2 servía: la única frontera sin prueba escrita era la
   que no la tenía.
2. **`Q-03.13` #3 choca con `Q-03.15` A.** Los recursos de un paso incluyen sus variables, y un hash
   calculado sobre valores **es** una marca, nacida fuera de Resolución. → `Q-03.22`.
3. **`Q-03.14` A parte el registro en forma y contenido, y *evidencia* cae justo en el corte.** →
   `Q-03.23`.
4. **`Q-03.17` A tropieza con el ámbito.** Una variable declarada puede no ser visible para el paso
   que la usa. → `Q-03.24`.
5. **El hueco.** Si el Historial solo conserva marcas y el motor no recuerda nada entre intentos,
   ¿de dónde sale el valor que produjo la última vez un paso que ahora no se re-ejecuta? →
   `Q-03.25`.
6. **`Q-03.20` A vuelve urgente la mitad de `Q-03.1` que nadie respondió.** Simulación y Ejecución
   no se hablan y comparten casi todo el lenguaje: según qué lectura, Simulación se funde. →
   `Q-03.26`.
7. **Una consecuencia mecánica de `Q-03.16` A, sin pregunta.** Renombrar un paso corta su historia, y
   no se puede convertir en error: una comprobación mira **un** pipeline (`Q-02.31`), y un
   renombrado solo existe entre **dos** versiones. Pérdida aceptada, escrita en `DEC-03.7`.
8. **Mitades y palabras pendientes.** → `Q-03.27`. Y `Q-03.21` #1 (los dos *quién*) sigue abierta; su
   #2 se replantea en `Q-03.26`, ahora con consecuencias.

Seis preguntas.

---

### Q-03.22 — «Hash de los recursos de un paso» y una Ejecución que no ve marcas *(`Q-03.13` #3 × `Q-03.15` A)*

**Contexto.** `Q-03.13` #3 incorpora *hash de los recursos de un paso* al lenguaje de Ejecución. Por
`lenguaje.md`, los recursos de un paso son sus **instrucciones**, sus **variables** y el **código del
producto**. Y `Q-03.15` A dice que Ejecución **solo ve valores**: la marca nace únicamente en
Resolución (`DEC-03.5`), y a Ejecución le llega un solo dato por paso, si sus variables cambiaron.

Un hash sobre los recursos de un paso incluye sus variables. Si lo calcula Ejecución, lo calcula
sobre **valores**, y un hash de un valor es exactamente una marca: algo que dice si cambió sin
mostrarlo. Nacería una segunda fábrica de marcas fuera de Resolución, que es lo que `DEC-03.5`
prohíbe y lo que `Q-03.15` A existía para evitar.

**Y algo que `Q-03.15` A no decía.** Para saber si las variables de un paso cambiaron, Resolución
compara contra las marcas de la última vez, que están en el Historial. Resolución no solo **escribe**
en el Historial: también **lee** de él.

Así queda quién sabe, de cada recurso, si cambió:

| Recurso del paso | Quién sabe si cambió | Con qué |
|---|---|---|
| sus instrucciones | Ejecución | un hash sobre sus comandos |
| el código del producto | Suministro lo calcula, el Historial lo conserva | el hash del código |
| sus variables | Resolución | sus marcas contra las de la última vez |

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **El hash de Ejecución cubre solo las instrucciones del paso**, y se llama *hash de las instrucciones de un paso*. *«¿Cambiaron los recursos?»* se responde juntando tres respuestas con tres dueños | `DEC-03.5` intacta · Cada recurso con un dueño, igual que cada eje en el core · Es lo que `Q-03.16` A usa para decir *«el paso cambió»* | El término que `Q-03.13` #3 incorporó se reescribe |
| **B** | **Resolución entrega una marca por paso**, de todas sus variables, y Ejecución la combina con instrucciones y código en un solo hash | Un solo hash por paso, que es lo que `Q-03.13` #3 pedía | Ejecución recibe una marca, aunque sea de un paso entero, y `Q-03.15` A se debilita |
| **C** | **Los recursos de un paso dejan de incluir las variables**: son instrucciones y código, y las variables son una condición aparte | El hash de los recursos se sostiene sin tocar marcas | *Recursos de un paso* cambia de definición, y lo que un paso necesita para ejecutarse sí incluye variables |

---

### Q-03.23 — «Evidencia»: ¿forma del Historial o contenido de Ejecución? *(`Q-03.14` A × `DEC-02.6`)*

**Contexto.** `Q-03.14` A partió el registro en dos: la **forma** (a qué intento pertenece, cuál fue
el anterior, el padre) es del Historial; lo que el registro **dice** es de quien lo produjo.
`DEC-02.6` le dio *evidencia* al Historial: *«lo que justificó que un paso no se re-ejecutara»*. Esa
frase admite dos lecturas, y cada una cae a un lado del corte:

| Lectura | Qué es | De quién, con `Q-03.14` A |
|---|---|---|
| **un enlace** | el registro con el que se hizo de verdad este paso: *«no se re-ejecutó; vale lo que dejó el intento X»* | **forma**, una relación entre registros como el padre → Historial |
| **una razón** | *«no había nada diferente en los recursos del paso»*, dicho por quien decidió | **contenido**, lo produjo quien decidió → Ejecución |

`lenguaje.md` ya tiene la segunda en el apartado de Ejecución, sin llamarla evidencia: *«no se
re-ejecuta: dicho siempre por su razón»*.

**Por qué importa.** El core necesita la evidencia (`Q-03.3`, fila 2) para saber con qué recursos se
hizo **de verdad** cada paso de un despliegue, y para eso sirve el enlace, no la razón. Si la
evidencia fuera contenido de Ejecución, el core leería con significado de Ejecución justo la palabra
que cedió para que no chocara dentro de él.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Enlace, del Historial.** La evidencia es la relación con el registro que vale; la razón es contenido de Ejecución y se sigue diciendo entera, como ya hace `lenguaje.md` | `DEC-02.6` intacta · Es lo que el core usa · No hace falta palabra nueva | Dos cosas que antes compartían frase quedan en dos contextos, y hay que decirlo en los dos apartados |
| **B** | **Razón, de Ejecución.** La evidencia se muda a Ejecución y el Historial la guarda como contenido | Quien decide, justifica | Enmienda `DEC-02.6` · El core necesita el enlace y así no lo recibe |
| **C** | **Las dos, en el Historial**, como una sola evidencia | Una sola palabra | Un término con forma y contenido, dentro del contexto que `Q-03.14` A partió justo por ahí |

---

### Q-03.24 — La comprobación y el ámbito *(`Q-03.17` A × `Q-03.6` B)*

**Contexto.** `Q-03.17` A: Definición comprueba la **forma**, que lo escrito esté bien formado y bien
referenciado, sin saber qué hará cada cosa. La objeción de la opción era que *bien referenciado* ya
es significado. Se puede sostener: que un nombre usado esté declarado **antes** en el orden de pasos
es integridad de lo escrito, sin valores y sin precedencia.

Pero un caso no se sostiene así. Una variable puede estar **declarada** y aun así **no ser visible**
para el paso que la usa. Eso lo decide el **ámbito** (*qué variables ve un paso*), que por `Q-03.6` B
es de Resolución.

| Si la comprobación mira… | Un pipeline que usa una variable declarada fuera del ámbito del paso |
|---|---|
| solo que esté declarada | **pasa** la comprobación y falla al resolver, ya dentro de un intento |
| también el ámbito | la comprobación necesita el significado de *ámbito*, que es de Resolución |

**Por qué importa.** `DEC-02.7` promete un pipeline *verificable de principio a fin sin tocar la
nube*. Con la primera fila, la promesa tiene un agujero con nombre.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **La comprobación no mira el ámbito.** Los errores de ámbito los encuentra **Simulación**, que interpola de verdad y es lo que el DevOps usa antes de publicar | `Q-03.17` A sin excepción · El agujero tiene quien lo cubra, en el momento en que importa | La promesa se reescribe: *verificable sin tocar la nube* pasa a necesitar comprobación **y** simulación · Antes de cada intento, sin simulación, el agujero sigue abierto |
| **B** | **El ámbito tiene una forma declarada**: lo que el DevOps escribe es forma de Definición y lo que significa es de Resolución, con homónimo declarado entre los dos | La comprobación vuelve a ser completa | *Ámbito* entra en la tabla de homónimos, y hay que explicar por qué eso no contradice lo que `DEC-02.5` descartó (dos ámbitos con adjetivo) |
| **C** | **La comprobación le pregunta a Resolución** por la visibilidad | Completa y sin homónimo | La comprobación deja de ser de un solo contexto: Definición depende de Resolución para saber si un pipeline está bien |

**Y los dos homónimos que `Q-03.17` A dejó declarados** necesitan su frase de frontera: *variable de
salida* (Ejecución la produce, Resolución le da valor) y *orden de los ambientes* (Diagnóstico y
Lanzamiento). El segundo depende de `Q-03.27` #1: si un lanzamiento ocurre en cualquier ambiente,
Lanzamiento no aplica el orden y el homónimo desaparece.

---

### Q-03.25 — ¿Dónde queda el valor que produjo un paso, entre un intento y el siguiente?

**Contexto.** Tres hechos del modelo, ciertos cada uno por separado:

1. **El motor no recuerda nada entre intentos** (restricción declarada): toda continuidad está
   escrita en algún sitio.
2. **Un paso que no se re-ejecuta no produce nada nuevo**, y los recursos de un paso incluyen
   variables producidas por un paso anterior (`Q-03.4`).
3. **Lo que el Historial conserva de una variable es su marca** (`DEC-01.6`, `Q-03.15` A), y una
   marca no devuelve el valor.

Juntos: el paso 2 no se re-ejecuta, el paso 3 sí, y necesita el valor que el paso 2 produjo la
última vez. Ese valor no está en el proceso (1), no se produce ahora (2) y no se saca de la marca
(3). La restricción de `dominio.md` —*«lo que se guarda no queda legible de un vistazo; ofuscar no es
cifrar»*— dice que **sí** se guardan valores, pero no dice quién los guarda ni dónde.

`Q-03.18` A lo agrava: en un rollback, las variables producidas *«se vuelven a producir»*, y eso solo
es verdad si los pasos que las producen se re-ejecutan.

**Por qué importa.** Decide quién tiene valores **en reposo**, justo lo que `DEC-03.5` quiso encerrar
en una frontera (*«solo tienen el valor en claro Resolución y quien ejecuta el comando»*). Y decide
si *una sola memoria* (`DEC-03.8`) sigue siendo verdad.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **En el Historial, ofuscado, como contenido de Resolución**, y solo Resolución lo pide de vuelta | Una sola memoria · Coherente con `Q-03.14` A: el Historial guarda lo que no interpreta · Sincronización lo lleva a otra máquina sin hacer nada nuevo | El core lee el Historial y el valor está ahí: *sin que el valor circule* pasa de frontera a disciplina de quien lee |
| **B** | **Resolución tiene memoria propia de valores** | La frontera de `DEC-03.5` queda entera: los valores nunca salen de Resolución | Dos memorias, contra `DEC-03.8` · Sincronización, que ahora vive dentro del Historial, no la llevaría a otra máquina |
| **C** | **No se guarda**: un paso cuyas variables necesita otro posterior se re-ejecuta cada vez que ese otro se ejecute | Ningún valor en reposo, en ningún sitio | *Hacer solo el trabajo que hace falta* se debilita justo en los pasos que crean recursos, que suelen ser los caros |

---

### Q-03.26 — Simulación y Ejecución no se hablan y comparten lenguaje: ¿qué lectura se aplica? *(`Q-03.21` #2 × `Q-03.20` A)*

**Contexto.** La segunda mitad de `Q-03.1` lleva dos rondas sin respuesta y parecía no importar. Con
`Q-03.20` A importa: Simulación no lee el historial y, según el modelo, depende de Definición,
Resolución y Suministro, no de Ejecución. Simulación y Ejecución **no se hablan**, y su vocabulario
se solapa casi entero: paso, comando, interpolación, variable de salida.

| Lectura | Qué pasa con Simulación |
|---|---|
| **i** sin cruce, son distintos | sobrevive como contexto (`Q-03.10` A) |
| **ii** sin cruce no hay evidencia, y se funden si su lenguaje se solapa | **se funde** con Ejecución: `Q-03.10` pasa a su opción B, con el homónimo interno que la A evitaba |

**Un matiz que puede disolverla.** Lo que comparten, ¿es lenguaje **suyo** o de terceros que las dos
consumen? *Paso* y *comando* son de Definición; *interpolación*, de Resolución. Si todo lo que se
solapa es vocabulario prestado, no hay solapamiento **propio** y las dos lecturas dan lo mismo.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **i** | Simple · `Q-03.10` A intacta | Dos contextos que nunca se hablan quedan separados sin que la prueba de `DEC-03.1` se les aplique |
| **B** | **ii, contando solo el lenguaje propio**: se funden si comparten términos que son suyos, no los que consumen de otros | La prueba también alcanza a los pares sin cruce · Simulación sobrevive si sus términos (simulación, simular un comando, salida simulada) no son de Ejecución | Hay que distinguir, par por par, lenguaje propio de lenguaje consumido |
| **C** | **ii, contando todo** | Máxima exigencia | Simulación se funde y `Q-03.10` se enmienda |

---

### Q-03.27 — Lo que falta decir: dónde ocurre un lanzamiento, y cinco palabras

**Contexto.** Mitades que las respuestas dejaron abiertas. Ninguna abre frontera, pero sin ellas
`lenguaje.md` no se puede escribir.

| # | Asunto | De | Candidatos |
|---|---|---|---|
| 1 | **Dónde ocurre un lanzamiento**: **i** solo tras un despliegue en el último ambiente · **ii** en cualquier ambiente. Decide cuál de las dos líneas de `lenguaje.md` se corrige, y si *orden de los ambientes* tiene uno o dos aplicadores (`Q-03.24`) | `Q-03.8` · `Q-03.14` | — |
| 2 | La **marca** del valor | `Q-03.5` · `Q-03.15` | *marca* · *hash de la variable* |
| 3 | La **declaración congelada** de un paso dentro del Historial: con `Q-03.16` A, la posición es *paso* y esto necesita otra palabra | `Q-03.7` · `Q-03.16` | *declaración usada* · *declaración del paso* |
| 4 | La **referencia recuperable** de una fuente. *Versión* está reservada al lanzamiento (`DEC-02.17`), y *commit* sería un préstamo nuevo que exige decisión (`DEC-02.2`) | `Q-03.18` | *revisión* · *referencia* · *commit* |
| 5 | Cómo dice Ejecución **a qué despliegue vuelve**: *padre*, como homónimo declarado con el Historial, o palabra propia | `Q-03.18` | *padre* · *destino* |
| 6 | Lo que devuelve un **comando simulado**, y confirmar *simulación* como la entidad que se recorre | `Q-03.20` | *salida simulada* · *valor simulado* |

---

## 4. Respuestas y análisis

> Ronda 1: doce de las trece preguntas de apertura. Se conservan tal como se escribieron; lo que la
> validación cambie queda en §3.bis y en §5.

---

### Q-03.1 — La prueba · **B**

**Respuesta.** Hay frontera si un término significa otra cosa al otro lado, **o** si conserva el
significado y obedece otras reglas. La proyección, por sí sola, no basta.

**Análisis.** Recoge el caso que la A no veía —*intento*: en Ejecución se está llevando a cabo; en el
Historial es un hecho que ya no cambia— sin abrir la puerta a que cualquier par pase. Y deja fuera lo
que había que dejar fuera: que al cruzar se pierda información (valor → marca) no separa por sí
mismo; separa si con ello cambian las reglas de lo que cruza.

El precio es el que la opción anunciaba: la prueba ya no se hace solo mirando la tabla de homónimos.
Cada prueba de separación tiene que **nombrar la regla que cambia**, y su forma queda así:

> *Al cruzar de X a Y, «T» deja de significar S₁ (o de obedecer R₁) y pasa a significar S₂ (u
> obedecer R₂). Si X e Y se fusionaran, las dos convivirían dentro del mismo contexto.*

**Qué descarta.** La prueba solo por homónimo, y la prueba por proyección, que llevaba a la
sobre-descomposición contra la que avisa el plan.

**Qué abre.** Dos respuestas de esta misma ronda se sostienen en lecturas que B no admite:
`Q-03.11` → `Q-03.19`, y `Q-03.4` → `Q-03.15`. Y la segunda mitad, la de los pares que no se
hablan, quedó sin responder → `Q-03.21`.

---

### Q-03.2 — El mapa de partida · **A**, con una desviación admitida

**Respuesta.** Nueve candidatos, uno por subdominio. *«Un contexto acotado puede abarcar más de un
subdominio; lo ideal es uno a uno en un proyecto como el nuestro, que se inicia de cero.»*

**Análisis.** El argumento del proyecto que empieza de cero es el que hace defendible el uno a uno.
Los desajustes que el libro tolera vienen de sistemas ya construidos, donde un contexto creció antes
de que nadie nombrara los subdominios. Aquí los subdominios se nombraron primero (IT-01), y la
solución puede seguirlos. La desviación se admite en un solo sentido, un contexto con varios
subdominios. El otro, un subdominio en varios contextos, no tiene ningún caso después de `Q-03.4` A,
así que no hace falta decidirlo.

**La consecuencia que hay que escribir.** A pone la carga de la prueba del lado de **fusionar**, y el
encuadre de `plan-ddd.md` E3 (y `guia-ddd.md` §2) la pone del lado de **separar**. No es un descuido
sino una enmienda al encuadre, y se nombra (`DEC-03.2`). Y el efecto ya se ve: en las doce respuestas
**no se propone ninguna fusión**. Nueve subdominios, nueve contextos. Puede ser el ideal alcanzado o
la inercia de la que avisaba el encuadre; lo que los distingue es que **cada prueba esté escrita**.

**Qué descarta.** Partir de un solo contexto, y dejar que los homónimos tracen el mapa.

---

### Q-03.3 — Diagnóstico y el Historial · **A, y lee solo el Historial**

**Respuesta.** Contexto propio. La prueba pasa por la traducción de registros por paso a estado de
tres ejes. Y el core lee **solo** del Historial.

**Análisis.** Resuelve el *«lo que arrastra»* en su forma más fuerte: en la solución, el core tiene
**una frontera**, aunque seis subdominios le prometan algo en el problema. Una garantía no es una
lectura. La cadena de garantías de `dominio.md` sigue intacta, con cada promesa y su dueño, pero en
la solución todo lo que el core necesita le **llega ya conservado**. Es el patrón de `Q-01.30` para
el hash, generalizado: el core nunca habla con quien produce, solo con quien recuerda.

Con `Q-03.1` B, la prueba se escribe con reglas y no solo con palabras. En el Historial, un registro
pertenece a un paso y a un intento, y no cambia. En Diagnóstico, lo que se compara es el estado de un
eje entre dos despliegues, que se deriva y solo existe mientras dura una comparación.

**Consecuencias.** La lista *«términos que consume»* de §Diagnóstico se corrige: todos vienen del
Historial, incluidos *paso* (2.4 #2) y *hash*. Y el Historial tiene que conservar, de forma que el
core pueda leerlo, lo que los otros cinco prometieron.

**Qué descarta.** El core como vista dentro del Historial, y la prueba de método.

**Qué abre.** Con qué significado conserva el Historial lo que no es suyo → `Q-03.14`.

---

### Q-03.4 — Ejecución de Pipeline · **A**

**Respuesta.** Un contexto. Resolución entrega las dos caras, y la partición entre decidir y hacer
es táctica (E9).

**Análisis.** Pasa la verificación de `DEC-01.3`: no reabre la lista de subdominios. *Decidir /
Planificación del Intento* deja de ser candidato a contexto y pasa a ser una pregunta de diseño de
E9, que es adonde `DEC-01.3` mandó el problema de *«la misma pieza decide y ejecuta»*. Es coherente
con que no haya un experto por mitad. Y la opción C, aunque descartada, deja un hecho escrito:
decidir y hacer **se intercalan paso a paso**, porque los recursos de un paso incluyen variables que
produce el anterior. No existe un corte temporal.

**Qué descarta.** Los dos contextos, y el corte por momento.

**Qué abre.** A traía una condición: escribir por qué Ejecución, manejando marca y valor de la misma
variable, no tiene un homónimo interno. Con `Q-03.1` B cuesta más, porque las dos obedecen reglas
opuestas → `Q-03.15`.

---

### Q-03.5 — Resolución de Variables · **A**

**Respuesta.** Contexto propio. Aquí viven el valor efectivo, la precedencia, el ámbito y la marca,
y es el único sitio donde un valor se convierte en marca.

**Análisis.** La promesa al core, comparar sin exponer, pasa a tener frontera: es el frente 2.B de
`DEC-01.10`, una regla que se cumple por construcción. **Coste aceptado, y se escribe como tal**: es
la frontera más transitada del mapa, porque cada comando pide interpolar y cada paso puede aportar
valores. Es el caso que `guia-ddd.md` §2 describe como contexto demasiado pequeño, y se acepta
sabiéndolo: lo que la frontera protege (quién puede tener un valor en claro) vale más que lo que
cuesta. Tienen el valor en claro Resolución y quien ejecuta el comando dentro de Ejecución. Nadie
más.

**Qué descarta.** Resolución dentro de Ejecución, y dentro de Definición.

**Qué abre.** El nombre de la marca, y cómo llega al Historial si el core solo lee de ahí →
`Q-03.15`.

---

### Q-03.6 — Declarar y aplicar · **B**

**Respuesta.** El término es de quien lo aplica; Definición transporta lo que el DevOps escribió.

**Análisis.** Una sola regla para los cuatro casos, y la que pone cada término donde tiene reglas.
*Ámbito* se queda en Resolución y cierra la duda **#5** de IT-02 sin tocar `lenguaje.md`. *Regla*
pasa a Ejecución. `DEC-02.5` no se enmienda.

**Qué descarta.** Que el término sea de quien lo declara, y el homónimo declarado en los cuatro.

**Qué abre.** El *«en contra»* de B no se respondió: la comprobación (`DEC-02.7`) está en Definición
y verifica términos cuyo significado ya es de otros. Y dos de los cuatro términos tienen **dos**
aplicadores → `Q-03.17`.

---

### Q-03.7 — El paso en el Historial · **B, con identidad por comandos**

**Respuesta.** Posición y declaración, con dos palabras. Y: *«un paso se identifica por lo que hace,
sus comandos en la definición; para facilidad, ese conjunto de comandos tiene un nombre; si esos
comandos cambian, significa que el paso cambió»*.

**Análisis.** La segunda mitad es mejor que las tres opciones que se ofrecían. Las tres buscaban la
identidad en algo **además** del contenido (el nombre, el orden, un identificador asignado), y la
tercera tropezaba con quién decide que dos pasos son el mismo. La respuesta quita el problema: no lo
decide nadie, lo decide el contenido. Renombrar o reordenar no corta ninguna historia, y el nombre
queda como etiqueta de conveniencia.

Tiene que aguantar en dos sitios. Primero, *«el paso cambió»* trata al paso como algo que persiste a
través del cambio, y si su identidad **son** sus comandos, cambiarlos no hace que cambie: hace que sea
otro. Segundo, dos pasos con los mismos comandos y variables distintas serían uno solo → `Q-03.16`.

Y una consecuencia mecánica: con identidad por comandos, la *posición* bajo la que se acumulan los
registros **es** la declaración, así que la opción B converge con la C. Tampoco se dio la palabra
para la declaración congelada.

**Qué descarta.** Que el Historial solo conozca la posición, y la identidad por nombre, por orden o
asignada.

---

### Q-03.8 — Lanzamiento · **en el Historial, entendido como una BD**

**Respuesta.** El lanzamiento se guarda en el Historial, *«entendiendo historial como una BD»*. El
lanzamiento es *«un concepto lógico similar a un despliegue»*.

**Análisis.** *Similar a un despliegue* es correcto y encaja con `DEC-02.3`: el lanzamiento es un
eslabón como los otros dos, con identificador propio y memoria del anterior. Eso descarta la opción A
(dos memorias): hay **una sola**, como dice `dominio.md`. Lo más cercano es la opción B.

*Entendiendo historial como una BD* cambia algo que no se preguntaba, la **naturaleza** del
Historial. Una base de datos es cómo se guarda algo, y leída al pie de la letra no es un contexto →
`Q-03.14`. Y la verificación de `DEC-01.13` depende de cuál de las dos lecturas valga.

**Qué descarta.** Dos memorias, y el lanzamiento con memoria propia.

**Qué abre.** La segunda mitad, dónde ocurre un lanzamiento, quedó sin responder, y `lenguaje.md`
sigue teniendo dos líneas que se contradicen → `Q-03.14`.

---

### Q-03.9 — Rollback · **A**

**Respuesta.** Es de Ejecución: un intento con padre elegido, que el Historial anota.

**Análisis.** Un acto con intención queda en el contexto de los actos, que es donde `lenguaje.md` ya
lo pone. El coste es el que la opción anunciaba: Ejecución tiene que entender *padre* y *punto de
retorno*, dos términos del Historial.

**Qué descarta.** El rollback como recorrido del árbol del Historial, y el rollback partido.

**Qué abre.** La segunda mitad, con qué recursos, quedó sin responder y ahora choca con `DEC-02.8`:
un hash no devuelve contenido. Y con `Q-03.1` B hay que comprobar si *padre* obedece las mismas
reglas a los dos lados → `Q-03.18`.

---

### Q-03.10 — Simulación · **A**

**Respuesta.** Contexto propio, con palabra propia para lo que recorre y para la variable fingida.

**Análisis.** Evita el homónimo interno que el modo habría metido en Ejecución (*intento* y *variable
producida* con dos significados), y deja la garantía de `DEC-01.5` en una frontera: ningún valor
simulado entra en el core porque no hay por dónde. La duplicación del mecanismo se acepta, y cómo se
comparte es de E4. Cierra la duda **#11** de IT-01.

**Qué descarta.** Simulación como modo de Ejecución, y dejarla sin frontera trazada.

**Qué abre.** Las dos palabras, y si lee el historial → `Q-03.20`.

---

### Q-03.11 — Los genéricos · **A**

**Respuesta.** Los dos son contextos, finos y genéricos.

**Análisis.** En Suministro, la prueba B se sostiene por su cuenta: *pipeline* es declaración en
Definición y repositorio en Suministro, y entra en la tabla de homónimos. En Sincronización, la razón
de la opción (*«los dos traducen»*) era proyección a carga opaca, la lectura C.

**Qué descarta.** Los genéricos como mecanismo sin contexto, y el trato asimétrico.

**Qué abre.** La prueba de Sincronización con la lectura elegida → `Q-03.19`.

---

### Q-03.12 — La autoría · **A**

**Respuesta.** La promete el Historial, con el patrón del hash: Suministro obtiene el autor al
ejecutar, el Historial lo conserva y el core lo lee.

**Análisis.** Cierra la duda **#15** de IT-01 y da dueño a la única fila sin él de la cadena de
garantías. `DEC-01.14` aguanta, porque el genérico no promete nada: sirve a quien promete. Y es
coherente con `Q-03.3`: el core no habla con la fuente. El tercer caso queda resuelto de paso: una
variable producida por un paso no tiene autor, y su *quién* es el intento que la produjo.

**Qué descarta.** Que la prometa Suministro, y seguir sin dueño.

**Qué abre.** Los dos sentidos de *quién* tienen que llamarse distinto dentro del Historial →
`Q-03.21`.

---

### Q-03.13 — Lo que queda · **punto por punto** *(respondida junto con la ronda 2)*

| # | Asunto | Respuesta |
|---|---|---|
| 1 | Nombre de los contextos | **Cada contexto se llama como su subdominio** |
| 2 | «Estado» | **Correcto**: queda un solo sentido vivo, y la mitad *estado* de la duda **#2** de IT-02 se cierra |
| 3 | El hash de un paso | **Se incorpora** *hash de los recursos de un paso* al lenguaje de Ejecución |
| 4 | Frase de apertura de §Diagnóstico | **Correcto**: se corrige; el core compara dos despliegues y el lanzamiento es una puerta de entrada |
| 5 | Forma de `bounded-contexts.md` | **Se acepta la propuesta** |

**Análisis.** El #1 es la consecuencia natural del uno a uno y no cuesta nada mientras se cumpla. Su
primera prueba llega en esta misma ronda: con `Q-03.19` C, un contexto realiza dos subdominios, y
la regla se completa sola, porque se llama como el subdominio **cuyo modelo es**: Historial.
Sincronización aporta mecanismo, no modelo. El #5 deja escrito que `bounded-contexts.md` dibuja
aristas **sin patrón**, y que el patrón lo pone E4.

**Qué abre.** El #3 choca con `Q-03.15` A: un hash sobre recursos que incluyen variables convierte
valores en marca fuera de Resolución → `Q-03.22`.

---

### Ronda 2 — Respuestas *(Q-03.14 … Q-03.20)*

---

### Q-03.14 — El Historial · **A: contexto de registros**

**Respuesta.** Metáfora, no base de datos. El Historial es un contexto cuyo modelo son los
registros: es dueño de la **forma** (a qué intento pertenece un registro, cuál fue el anterior, el
padre) y lo que el registro **dice** sigue siendo de quien lo produjo.

**Análisis.** El subdominio conserva su contexto y sus reglas, el core traduce desde un solo modelo
y la pregunta de E5 —si la secuencia de hechos es el modelo o su persistencia— sigue abierta.

El corte entre forma y contenido es lo que responde la objeción de la opción, *¿por qué no mete
cinco lenguajes dentro?*: porque el Historial **no aplica ninguna regla** del contenido que guarda.
Solo aplica las suyas. Y es simétrico con `Q-03.19` C: llevar contenido que no se interpreta **no
crea frontera y tampoco contamina**. Las dos respuestas salen de la misma lectura de `Q-03.1`, en la
que lo que cuenta son las reglas que se aplican.

**Enmienda nombrada.** La verificación de `DEC-01.13` se reescribe: *ninguna **regla** de Lanzamiento
aparece en el Historial*. El lanzamiento es un tipo de registro; la decisión, el significado de sus
etiquetas y el actor ausente son de Lanzamiento.

**Qué descarta.** El Historial como persistencia compartida sin contexto, y diferirlo a E5 sin decir
qué es un registro.

**Qué abre.** *Evidencia* cae justo en el corte → `Q-03.23`. Y la segunda mitad, dónde ocurre un
lanzamiento, sigue sin respuesta → `Q-03.27` #1.

---

### Q-03.15 — Lo que Ejecución ve de una variable · **A: solo valores**

**Respuesta.** La marca nunca entra en Ejecución. Resolución la deja directamente en el Historial, y
a Ejecución le da un solo dato por paso: si sus variables cambiaron.

**Análisis.** *Variable* tiene un solo sentido dentro de Ejecución, y comparar sin exponer vive en un
solo contexto. El coste aceptado es una arista nueva, Resolución → Historial, y hay una segunda que
la opción no decía: para saber si las variables de un paso cambiaron, Resolución compara contra las
marcas de la última vez, que están en el Historial. **Resolución lee y escribe en el Historial**, y
para dejar cada marca en su sitio usa términos de la forma del Historial (paso, intento).

**Qué descarta.** Que Ejecución lleve marcas sin leerlas, y que tenga dos palabras.

**Qué abre.** Choca con `Q-03.13` #3 → `Q-03.22`. Deja a la vista el hueco de los valores entre
intentos → `Q-03.25`. Y la palabra de la marca sigue sin elegir → `Q-03.27` #2.

---

### Q-03.16 — La identidad de un paso · **A: el nombre dice cuál es; los comandos, si cambió**

**Respuesta.** Identidad por nombre, cambio por comandos.

**Análisis.** Corrige la segunda mitad de `Q-03.7` (*«se identifica por lo que hace»*) y mantiene su
opción B: posición y declaración, con dos palabras. *«El paso cambió»* vuelve a tener un sujeto que
sigue siendo el mismo, y dos pasos con los mismos comandos siguen siendo dos. Lo que *«cambió»*
compara es un hash sobre los comandos del paso, y eso enlaza con `Q-03.22`.

**Pérdida aceptada, sin pregunta.** Renombrar un paso corta su historia. No se puede convertir en
error de la comprobación: una comprobación mira **un** pipeline (`Q-02.31`), y un renombrado solo
existe entre **dos** versiones. Queda como lo que es, un coste consciente.

**Qué descarta.** La identidad por comandos (funde dos pasos en uno), y por comandos y variables
(deja de poder decirse la premisa del core paso a paso).

**Qué abre.** La palabra para la declaración congelada → `Q-03.27` #3.

---

### Q-03.17 — Qué comprueba Definición · **A: la forma**

**Respuesta.** Definición comprueba que lo escrito esté bien formado y bien referenciado; el
significado es de quien lo aplica, y un término con dos aplicadores es un homónimo declarado entre
ellos.

**Análisis.** *Bien referenciado* se sostiene como forma: que un nombre usado esté declarado antes en
el orden de los pasos es integridad de lo escrito, sin valores ni precedencia. La comprobación sigue
siendo una sola y sigue sin ejecutar nada. Entran dos homónimos en la tabla: *variable de salida*
(Ejecución la produce, Resolución le da valor) y *orden de los ambientes* (Diagnóstico y
Lanzamiento).

**Qué descarta.** Las excepciones por lista, y partir la comprobación entre dos contextos.

**Qué abre.** El ámbito decide si una variable declarada es **visible**, y eso no es forma →
`Q-03.24`.

---

### Q-03.18 — Rollback · **A: vuelve a los recursos del despliegue elegido**

**Respuesta.** El Historial guarda, además del hash, **una referencia recuperable** de cada fuente,
y Suministro sabe traer esa versión.

**Análisis.** El rollback vuelve de verdad y *punto de retorno* conserva su sentido.

**El hash no sobra**, y conviene dejar escrito por qué, porque parece que la referencia lo hace
redundante. Si alguien revierte un cambio, la referencia es nueva y el contenido es el de antes: el
hash es igual y dice *«no cambió»*, que es verdad, mientras que comparar referencias diría
*«cambió»*. El hash responde *¿cambió?*; la referencia, *¿cómo vuelvo a tenerlo delante?*. Son dos
preguntas y dos términos.

La promesa de Suministro crece (*tener delante el material tal como era*) y sigue siendo algo que se
compra: cualquier repositorio lo hace. Por `DEC-01.14` sigue siendo Generic. Las variables declaradas
se resuelven de nuevo desde el pipeline de entonces; las producidas se vuelven a producir.

**Qué descarta.** Los recursos de hoy con el padre elegido, y elegir qué ejes vuelven atrás.

**Qué abre.** *«Las producidas se vuelven a producir»* solo es verdad si su paso se re-ejecuta →
`Q-03.25`. Y dos palabras → `Q-03.27` #4 y #5.

---

### Q-03.19 — Sincronización del Historial · **C: no es contexto**

**Respuesta.** Es cómo el Historial está disponible en otra máquina. `Q-03.11` pasa a su opción C:
Suministro es contexto y Sincronización no.

**Análisis.** **Primera fusión del mapa: ocho contextos.** El subdominio no cambia en el problema:
`dominio.md` lo sigue teniendo como Generic, que se puede comprar. Lo que cambia es que en la
solución se realiza **dentro del Historial**, que es el primer caso de la desviación que admitió
`DEC-03.2`. Es simétrico con `Q-03.14` A: llevar contenido sin interpretarlo no es cruzar a otro
modelo. Y la restricción de `dominio.md` (*ofuscar lo que se guarda es de quien persiste*) encuentra
dueño: quien persiste es el Historial.

**Coste aceptado.** El Historial habla de transporte, y su lenguaje gana *sincronizar*. E10 evalúa la
compra como mecanismo del Historial, no como pieza aparte.

**Qué descarta.** Crearle vocabulario a un genérico para que pase la prueba, y abrir una excepción en
la prueba para los genéricos.

---

### Q-03.20 — Simulación · **A: no lee el historial**

**Respuesta.** Recorre siempre todos los pasos.

**Análisis.** Responde *¿funciona el pipeline?*, que es la pregunta de su actor, y es la única
lectura coherente con su momento: de un pipeline sin publicar no hay *última vez*. Además reduce la
duplicación que `Q-03.10` aceptó, porque Simulación no necesita la mitad de decidir de Ejecución.

**Qué descarta.** Decir qué se re-ejecutaría, con o sin un ambiente dado.

**Qué abre.** Simulación y Ejecución no se hablan y comparten lenguaje → `Q-03.26`. Y sus dos
palabras → `Q-03.27` #6.

---

---

### Ronda 3 — Respuestas *(Q-03.22 … Q-03.27)* y cierre

> Respondidas el 2026-09-13, junto con la enmienda de proceso `DEC-03.15`. A partir de aquí el
> análisis es corto: qué decide cada respuesta y qué arrastra.

| Q | Respuesta | Qué decide | Qué arrastra |
|---|---|---|---|
| `Q-03.22` | **A** | El hash que calcula Ejecución cubre solo los comandos: **hash de las instrucciones de un paso**. *¿Cambiaron los recursos?* se responde juntando tres respuestas con tres dueños | Se reescribe el término de `DEC-03.14`; `DEC-03.5` queda intacta |
| `Q-03.23` | **A** | **Evidencia** es el enlace al registro con el que se hizo de verdad un paso: es forma, y es del Historial. La razón es contenido de Ejecución y se dice entera | `DEC-02.6` queda intacta |
| `Q-03.24` | **B** | **Ámbito** tiene forma declarada en Definición y significado en Resolución: es un homónimo **entre** contextos. No contradice `DEC-02.5`, que descartó dos ámbitos **dentro** de un mismo lenguaje. La comprobación vuelve a estar completa | La regla de `DEC-03.6` se generaliza: los cuatro términos declarados tienen forma en Definición |
| `Q-03.25` | **A** | El valor de una variable producida queda **en el Historial, ofuscado**, como contenido de Resolución, y solo Resolución lo pide de vuelta. Una sola memoria | **Pérdida aceptada**: con el valor en reposo, *sin que el valor circule* depende de la disciplina de quien lee el historial; no lo garantiza una frontera |
| `Q-03.26` | **A** | Dos contextos que no se hablan son distintos. Simulación sobrevive como contexto | La prueba de `DEC-03.1` solo se aplica a pares que se cruzan |
| `Q-03.27` | por punto | **#1** Un lanzamiento ocurre **en cualquier ambiente** · **#2** la marca se llama **hash de variable** · **#3** **declaración congelada** · **#4** **commit**, préstamo nuevo · **#5** **padre** en el Historial y **destino** en Ejecución · **#6** **salida simulada**, con *simulación* como la entidad | El #1 retira *último ambiente*, deja *orden de los ambientes* con un solo aplicador (Diagnóstico) y abre una incoherencia con la definición de Lanzamiento → §9 #1. El #4 amplía la lista cerrada de `DEC-02.2` |
| `Q-03.21` | **por defecto** | #1: **autor**, quien hizo un cambio en una fuente, y **solicitante**, quien pidió un intento. El #2 lo resolvió `Q-03.26` | Decidido por defecto bajo `DEC-03.15`; se revierte si no sirve |

**Cómo se interpretaron dos respuestas escuetas.** *«Sí, la declaración congelada»* se lee como
aceptar ese término. *«Padre destino»* se lee como un reparto: *padre* se queda en el Historial y
*destino* es la palabra de Ejecución.

## 5. Decisiones

> **Firmes al cierre.** Se revisaron tras la ronda 2, y lo que resolvió cada *Pendiente de* está en
> *Cierre*, al final de esta sección. Los números se conservan aunque cambie el contenido.
> `DEC-03.13` a `DEC-03.18` nacieron en la validación.

---

### DEC-03.1 — La prueba de separación: homónimo o reglas distintas

**Contexto.** IT-01 dejó una sola prueba (**H5**) sin decir qué cuenta como *cambiar de significado*.

**Decisión.** Hay frontera entre X e Y si, al cruzar, un término **significa otra cosa** u **obedece
otras reglas**. La proyección (que al cruzar se pierda información) no basta por sí sola. Toda prueba
se escribe con la forma de `Q-03.1` §4 y **nombra la regla que cambia**.

**Consecuencias.** Las pruebas de `bounded-contexts.md` no se verifican solo contra la tabla de
homónimos: cada una nombra su regla. Y llevar contenido **sin interpretarlo** no crea frontera
(`Q-03.19`) ni contamina a quien lo lleva (`Q-03.14`): lo que cuenta son las reglas que se aplican.

**Alternativas descartadas.** Solo homónimo, porque no ve el caso *intento*. Homónimo, reglas o
proyección, porque casi todo par pasaría. Y una excepción para los genéricos, porque la prueba
dejaría de ser una sola.

**Pendiente de.** `Q-03.26`: los pares que no se hablan.

**Verificación.** Ninguna prueba de separación se apoya solo en que se pierda información.

---

### DEC-03.2 — Un contexto por subdominio como ideal, y la carga del lado de fusionar

**Contexto.** El libro pone como objetivo el uno a uno; el encuadre de `plan-ddd.md` E3 ponía la
carga de la prueba del lado de separar.

**Decisión.** Se parte de **un contexto por subdominio**. En un proyecto que empieza de cero, el uno
a uno es el ideal y se sigue salvo razón escrita. Se admite que un contexto realice más de un
subdominio.

**Consecuencias.** **Enmienda el encuadre de `plan-ddd.md` E3**: la carga de la prueba pesa del lado
de **fusionar**. A cambio, cada frontera que sobrevive tiene su prueba escrita. La desviación
admitida ya tiene un caso: el Historial realiza Historial y Sincronización del Historial
(`DEC-03.11`). **Ocho contextos.**

**Alternativas descartadas.** Partir de un solo contexto; que los homónimos tracen el mapa.

**Verificación.** `bounded-contexts.md` no tiene ninguna frontera sin prueba escrita.

---

### DEC-03.3 — Diagnóstico es contexto propio y lee solo el Historial

**Contexto.** El core lee lo que se usó y nunca habla con la fuente; seis subdominios le prometen
algo.

**Decisión.** Diagnóstico es contexto propio. En la solución tiene **una sola frontera**, con el
Historial. La prueba: en el Historial, un registro pertenece a un paso y a un intento y no cambia; en
Diagnóstico, lo comparado es el estado de un eje entre dos despliegues, derivado y solo durante la
comparación.

**Consecuencias.** Una garantía no es una lectura: la cadena de `dominio.md` no cambia, pero todo lo
prometido llega al core **ya conservado** en el Historial, con el corte de `DEC-03.13`. La lista de
*«términos que consume»* de §Diagnóstico se corrige, y todos vienen del Historial.

**Alternativas descartadas.** El core como vista dentro del Historial; separar por el lenguaje de
método en vez de por la prueba.

**Pendiente de.** `Q-03.23`: si la evidencia que el core lee es forma o contenido.

**Verificación.** Ningún escenario de E6 lee algo que no esté en el Historial.

---

### DEC-03.4 — Ejecución de Pipeline es un contexto, y solo ve valores

**Contexto.** `DEC-01.3` dejó a *Decidir / Planificación del Intento* como candidato a contexto.

**Decisión.** Un solo contexto. Decidir y hacer se intercalan paso a paso, y su separación es
táctica (E9). Dentro de Ejecución, *variable* es siempre el **valor**: la marca nunca entra
(`Q-03.15`).

**Consecuencias.** Se cumple la verificación de `DEC-01.3`. *Decidir / Planificación del Intento* sale
del mapa de contextos.

**Alternativas descartadas.** Dos contextos; el corte por momento, imposible porque los recursos de
un paso incluyen lo que produjo el anterior; que Ejecución lleve marcas sin leerlas, o con otra
palabra.

**Pendiente de.** `Q-03.22`: el hash de los recursos de un paso.

**Verificación.** `lenguaje.md` no tiene apartado de *Decidir*, y *variable* tiene un solo sentido
dentro de Ejecución.

---

### DEC-03.5 — Resolución de Variables es contexto propio, y es donde nace la marca

**Contexto.** Es el Supporting cuya promesa (comparar sin exponer) sostiene una categoría entera de
la respuesta del core.

**Decisión.** Contexto propio: valor efectivo, precedencia, ámbito y marca. **Único** sitio donde un
valor se convierte en marca. La marca va **directamente al Historial**, y a Ejecución le llega, por
paso, si sus variables cambiaron. Resolución **lee y escribe** en el Historial: compara contra las
marcas de la última vez.

**Consecuencias.** Coste aceptado y escrito: es la frontera más transitada del mapa, y tiene dos
aristas con el Historial. Solo tienen el valor en claro Resolución y quien ejecuta el comando.

**Alternativas descartadas.** Dentro de Ejecución (la promesa pasaría a ser disciplina); dentro de
Definición (metería dentro el homónimo *lo escrito / lo vigente*).

**Pendiente de.** `Q-03.22` (otra posible fábrica de marcas), `Q-03.25` (valores en reposo) y
`Q-03.27` #2 (la palabra).

**Verificación.** Ningún contexto distinto de Resolución produce una marca.

---

### DEC-03.6 — Lo que Definición declara es de quien lo aplica; Definición comprueba la forma

**Contexto.** Cuatro términos que Definición declara y otros ejercen, y una comprobación que los
verifica.

**Decisión.** El término es **de quien lo aplica**: *ámbito* es de Resolución (cierra IT-02 **#5**) y
*regla*, de Ejecución. Definición **comprueba la forma**: que lo escrito esté bien formado y bien
referenciado, sin saber qué hará cada cosa. Un término con dos aplicadores es un **homónimo
declarado** entre ellos: *variable de salida* (Ejecución y Resolución) y *orden de los ambientes*
(Diagnóstico y Lanzamiento).

**Consecuencias.** *Regla* sale de §Definición en `lenguaje.md`. `DEC-02.5` no se enmienda. La
comprobación sigue siendo una y sigue sin ejecutar nada.

**Alternativas descartadas.** De quien lo declara; homónimo declarado en los cuatro; excepciones por
lista; partir la comprobación.

**Pendiente de.** `Q-03.24` (el ámbito y la visibilidad) y `Q-03.27` #1 (si Lanzamiento aplica el
orden).

**Verificación.** Ningún término de §Definición tiene sus reglas en otro contexto.

---

### DEC-03.7 — El nombre dice cuál es un paso; sus comandos, si cambió

**Contexto.** *Paso* tenía tres sentidos declarados y un cuarto implícito dentro del Historial.
`Q-03.7` lo identificó por sus comandos, y `Q-03.16` lo corrigió.

**Decisión.** Un paso se identifica **por su nombre**, y sus **comandos** dicen si cambió. En el
Historial, la posición es *paso* y la declaración congelada lleva otra palabra.

**Consecuencias.** Dos pasos con los mismos comandos son dos pasos. **Pérdida aceptada**: renombrar un
paso corta su historia, y no se puede convertir en error, porque una comprobación mira un pipeline y
un renombrado existe entre dos.

**Alternativas descartadas.** Identidad por comandos: funde dos pasos en uno y deja *«cambió»* sin
sujeto. Por comandos y variables: el mismo paso sería otro en cada ambiente.

**Pendiente de.** `Q-03.27` #3: la palabra de la declaración congelada.

**Verificación.** La frase *«el paso cambió»* tiene un sujeto que sigue siendo el mismo.

---

### DEC-03.8 — El lanzamiento es un registro del Historial

**Contexto.** `DEC-01.13` pedía que ningún término de Lanzamiento apareciera en el Historial, y
`dominio.md` dice que el Historial guarda los lanzamientos.

**Decisión.** **Una sola memoria.** El lanzamiento es un eslabón, como el intento y el despliegue, y
queda en el Historial como tipo de registro. La decisión, el significado de sus etiquetas y el actor
ausente son de Lanzamiento.

**Consecuencias.** **Enmienda nombrada a la verificación de `DEC-01.13`**, que pasa a ser: *ninguna
**regla** de Lanzamiento aparece en el Historial*.

**Alternativas descartadas.** Lanzamiento con memoria propia; un solo contexto para los dos
subdominios.

**Pendiente de.** `Q-03.27` #1: dónde ocurre un lanzamiento.

**Verificación.** `lenguaje.md` tiene una sola línea sobre dónde ocurre un lanzamiento, y §Historial
no enuncia ninguna regla de Lanzamiento.

---

### DEC-03.9 — El rollback es de Ejecución, y vuelve a lo que se hizo

**Contexto.** *Rollback* estaba en Ejecución; *padre*, en el Historial; y un hash no devuelve
contenido.

**Decisión.** El rollback es un **intento con padre elegido**, y es de Ejecución. Se hace con **los
recursos del despliegue elegido**: el Historial guarda, además del hash, **una referencia
recuperable** de cada fuente, y Suministro sabe traer esa versión. Las variables declaradas se
resuelven de nuevo desde el pipeline de entonces.

**Consecuencias.** Promesa nueva del Historial y término nuevo. La promesa de Suministro crece y sigue
siendo Generic. **El hash no se retira**: después de revertir un cambio, la referencia es nueva y el
contenido el mismo, y solo el hash dice la verdad (*no cambió*).

**Alternativas descartadas.** Los recursos de hoy con el padre elegido (*punto de retorno* sin
contenido); elegir qué ejes vuelven atrás.

**Pendiente de.** `Q-03.25` (las variables producidas) y `Q-03.27` #4 y #5 (dos palabras).

**Verificación.** *Rollback* aparece en un solo apartado de `lenguaje.md`, y *hash* y la referencia
recuperable son dos términos distintos.

---

### DEC-03.10 — Simulación es contexto propio y no lee el historial

**Contexto.** Duda **#11** de IT-01.

**Decisión.** Contexto propio, con palabras propias para lo que recorre y para lo que devuelve un
comando fingido. **No lee el historial**: recorre siempre todos los pasos.

**Consecuencias.** Ni *intento* ni *variable producida* tienen segundo sentido en Ejecución. La
garantía de `DEC-01.5` queda en una frontera. Simulación no duplica la mitad de decidir de Ejecución.
Cómo se comparte el resto del mecanismo es de E4.

**Alternativas descartadas.** Modo de Ejecución; contexto sin frontera trazada; leer el historial.

**Pendiente de.** `Q-03.26` (si sobrevive sin cruce con Ejecución) y `Q-03.27` #6 (las palabras).

**Verificación.** Ningún término propio de Ejecución aparece en §Simulación.

---

### DEC-03.11 — Suministro de Fuentes es contexto; Sincronización del Historial no *(revisada)*

**Contexto.** Dos genéricos. `Q-03.11` los hizo contextos a los dos; `Q-03.19` mostró que Sincronización
no tenía prueba con la lectura elegida.

**Decisión.** **Suministro de Fuentes es contexto**: *pipeline* es declaración en Definición y fuente
en Suministro, y entra en la tabla de homónimos. **Sincronización del Historial no es contexto**: es
cómo el Historial está disponible en otra máquina, y se realiza **dentro del Historial**.

**Consecuencias.** `Q-03.11` pasa a su opción C. **Ocho contextos.** El subdominio no cambia en
`dominio.md`. El Historial gana *sincronizar* y es quien ofusca lo que guarda. E10 evalúa la compra
como mecanismo del Historial.

**Alternativas descartadas.** Sincronización como contexto con vocabulario inventado; una excepción
en la prueba para los genéricos.

**Verificación.** `bounded-contexts.md` tiene ocho contextos, y Sincronización del Historial aparece
como subdominio realizado por el Historial.

---

### DEC-03.12 — La autoría la promete el Historial

**Contexto.** Duda **#15** de IT-01: la única fila sin dueño de la cadena de garantías.

**Decisión.** La promete el **Historial**, con el patrón del hash: Suministro obtiene el autor al
ejecutar, el Historial lo conserva y el core lo lee. Una variable producida por un paso no tiene
autor: su *quién* es el intento que la produjo.

**Consecuencias.** La fila de `dominio.md` tiene dueño. `DEC-01.14` aguanta.

**Alternativas descartadas.** Que la prometa Suministro; seguir sin dueño.

**Pendiente de.** `Q-03.21` #1: los dos nombres de *quién*.

**Verificación.** Ninguna fila de la cadena de garantías queda sin dueño.

---

### DEC-03.13 — El Historial es un contexto de registros: la forma es suya, lo que dicen no *(nueva)*

**Contexto.** `Q-03.8` lo describió como *«una BD»*; leído al pie de la letra, no sería contexto.

**Decisión.** El Historial es un contexto cuyo modelo son los **registros**. Es dueño de su
**forma**: a qué intento pertenece un registro, cuál fue el anterior y el padre. Lo que un registro
**dice** sigue siendo de quien lo produjo, y el Historial lo guarda sin aplicar sus reglas.

**Consecuencias.** El subdominio conserva contexto y reglas, y el core traduce desde un solo modelo.
El Historial no tiene cinco lenguajes dentro porque no aplica ninguna regla ajena. Si la secuencia
de registros es el modelo o su persistencia sigue siendo pregunta de **E5**.

**Alternativas descartadas.** Persistencia compartida sin contexto: dejaba sin dueño las reglas del
árbol y al core leyendo datos de cinco contextos. Diferirlo entero a E5 sin decir qué es un
registro.

**Pendiente de.** `Q-03.23`: si *evidencia* es forma o contenido.

**Verificación.** Ninguna regla de §Historial en `lenguaje.md` usa el significado de un término de
otro contexto.

---

### DEC-03.14 — Nombres, «estado» y la forma de `bounded-contexts.md` *(nueva)*

**Contexto.** `Q-03.13`.

**Decisión.**
- Cada contexto **se llama como su subdominio**. Si realiza dos, como el del subdominio **cuyo modelo
  es**: Historial.
- **«Estado»** queda con un solo sentido vivo, el desenlace de un intento, y cierra la mitad *estado*
  de la duda **#2** de IT-02.
- Ejecución incorpora **hash de los recursos de un paso**.
- La frase de apertura de §Diagnóstico se corrige: el core compara **dos despliegues**, y el
  lanzamiento es una puerta de entrada.
- `bounded-contexts.md` lleva, por contexto: **propósito · subdominios que realiza · términos propios
  · términos que consume y de quién · prueba de separación contra cada vecino**. Las aristas se
  dibujan **sin patrón**; lo pone E4.

**Pendiente de.** `Q-03.22`: el hash de los recursos de un paso choca con `DEC-03.5`.

**Verificación.** Cada contexto de `bounded-contexts.md` tiene las cinco partes.

---

### Cierre — cómo quedan los *Pendiente de*

| Decisión | Pendiente | Queda |
|---|---|---|
| `DEC-03.1` | `Q-03.26` | La prueba se aplica a los pares que se cruzan; dos contextos sin cruce son distintos |
| `DEC-03.3` | `Q-03.23` | El core lee la evidencia como enlace, que es forma del Historial |
| `DEC-03.4` | `Q-03.22` | Ejecución calcula el hash de las instrucciones de un paso, nunca sobre variables |
| `DEC-03.5` | `Q-03.22` · `Q-03.25` · `Q-03.27` #2 | La marca se llama **hash de variable** y solo nace en Resolución; el valor en reposo queda ofuscado en el Historial y solo Resolución lo pide de vuelta |
| `DEC-03.6` | `Q-03.24` · `Q-03.27` #1 | Los cuatro términos declarados tienen forma en Definición y son homónimos entre contextos; *orden de los ambientes* tiene un solo aplicador, Diagnóstico |
| `DEC-03.7` | `Q-03.27` #3 | La palabra es **declaración congelada** |
| `DEC-03.8` | `Q-03.27` #1 | Un lanzamiento ocurre en cualquier ambiente (`DEC-03.17`) |
| `DEC-03.9` | `Q-03.25` · `Q-03.27` #4 y #5 | La referencia recuperable es el **commit** y Ejecución dice **destino**. Las variables producidas se vuelven a producir si su paso se re-ejecuta y, si no, se piden al Historial |
| `DEC-03.10` | `Q-03.26` · `Q-03.27` #6 | Simulación sobrevive; *simulación* es la entidad y **salida simulada**, lo que devuelve un comando simulado |
| `DEC-03.12` | `Q-03.21` #1 | **Autor** y **solicitante**, por defecto |
| `DEC-03.13` | `Q-03.23` | *Evidencia* es forma |
| `DEC-03.14` | `Q-03.22` | *Hash de los recursos de un paso* pasa a ser **hash de las instrucciones de un paso** |

---

### DEC-03.15 — Se itera rápido: el modelo se propone, y solo se pregunta donde algo no encaja *(enmienda de proceso)*

**Contexto.** E3 necesitó tres rondas y 27 preguntas. El experto lo dijo así: *«siento que no estoy
avanzando; necesito analizar rápido esta solución para implementarla rápido; eso no significa diseñar
mal»*. Pidió definir el modelo **a partir de lo que quiere** y recibir preguntas **enfocadas en lo
que se espera** solo cuando haya incongruencias. El formato de inventario lo obligaba a elegir entre
tablas de opciones en vez de decir qué quiere. Y lo precisó: *«me planteas las preguntas una por una,
esperas que yo responda antes de ir a la siguiente, siempre orientando las preguntas sobre lo que
queremos lograr; el plan es el mismo, solo el proceso cambia»*.

**Decisión.** `plan-ddd.md` §6 cambia:

1. **Se parte de lo que se quiere, no de un inventario.** El experto dice qué espera, y Claude lo
   convierte en modelo contrastándolo con el libro y **propone** las decisiones con su porqué.
2. **Solo se pregunta donde hay una incongruencia**: dos cosas del modelo que no pueden ser verdad a
   la vez, o una decisión que solo el experto puede tomar. Cada pregunta explica la incongruencia en
   dos líneas y trae una **propuesta recomendada**. **Una a la vez**: se espera la respuesta antes de
   plantear la siguiente.
3. **Lo que no es incongruencia se decide por defecto** y se anota como tal. El experto lo revierte si
   no le sirve.
4. **Vueltas cortas.** Cada respuesta se aplica al modelo en la misma vuelta. La iteración se cierra
   cuando una vuelta ya no deja incongruencias que muevan lo que decide la etapa; lo que quede sin
   moverlo se difiere con nombre.
5. **El archivo de iteración registra las decisiones y su porqué**, no las opciones que nadie eligió.

**Consecuencias.** Se retiran la puerta, las fases de apertura, respuesta y validación como rondas
de cuestionario, y el umbral de ~12 preguntas. La plantilla se aligera. **Se conservan**: la
inmutabilidad de las iteraciones cerradas, `modelo/` como presente, el código fuera de E2–E5, la
reconciliación con las specs en E11 y la numeración `Q-`/`DEC-`. **Riesgo nombrado**: decidir por
defecto puede meter en el modelo algo que el experto no habría elegido. Se mitiga escribiendo cada
decisión por defecto como tal.

**Alternativas descartadas.** Seguir con el inventario. Quitar el contraste con el libro: la velocidad
tiene que venir de no convertir ese contraste en un cuestionario, no de saltárselo.

**Verificación.** De IT-04 en adelante, ninguna pregunta existe sin una incongruencia que la motive,
y todas llevan propuesta.

---

### DEC-03.16 — Valores y hashes: quién calcula y quién guarda *(nueva)*

**Contexto.** `Q-03.22`, `Q-03.25` y `Q-03.27` #2.

**Decisión.** *Hash* es una **palabra común con dueño en cada caso**, y siempre responde *¿cambió?*:

- el del código y el del pipeline, de Suministro;
- el de las instrucciones de un paso, de Ejecución;
- el de variable, de Resolución, que es el **único** que se calcula sobre valores.

El valor de una variable producida queda **en el Historial, ofuscado**, y solo Resolución lo pide de
vuelta.

**Consecuencias.** `DEC-02.8` se amplía: el hash deja de ser solo de repositorios. **Hash y commit son
dos términos**: después de revertir un cambio, el commit es nuevo y el hash no cambia. **Pérdida
aceptada**: con el valor en reposo en el Historial, *sin que el valor circule* depende de la
disciplina de quien lee; ninguna frontera lo garantiza.

**Alternativas descartadas.** Un hash de recursos que incluya variables (una segunda fábrica de
marcas); que Resolución tenga memoria propia (dos memorias); no guardar el valor (el paso que lo
produjo se re-ejecutaría siempre).

**Verificación.** Ningún hash sobre valores se calcula fuera de Resolución.

---

### DEC-03.17 — Un lanzamiento ocurre en cualquier ambiente *(nueva)*

**Contexto.** `lenguaje.md` tenía dos líneas que se contradecían (2.4 #1).

**Decisión.** Un lanzamiento puede ocurrir **en cualquier ambiente**. *Último ambiente* sale del
lenguaje, y *orden de los ambientes* se queda con un solo aplicador, Diagnóstico.

**Consecuencias.** **Enmienda `DEC-02.3`**: en la fila del lanzamiento, *«tras un despliegue en el
último ambiente»* pasa a *«tras un despliegue, en cualquier ambiente»*. Lanzamiento deja de leer a
Definición.

**Pendiente** (§9 #1): qué es *el cliente* fuera de producción, y quién decide ahí.

**Verificación.** `lenguaje.md` no contiene *«último ambiente»*.

---

### DEC-03.18 — Palabras nuevas y un préstamo *(nueva)*

**Decisión.**

| Término | Contexto | Qué nombra |
|---|---|---|
| **hash de variable** | Resolución | dice si una variable cambió sin mostrar el valor; sustituye a *marca* |
| **hash de las instrucciones de un paso** | Ejecución | dice si los comandos de un paso cambiaron |
| **declaración congelada** | Historial | un paso tal como estaba declarado cuando se hizo |
| **commit** | Suministro | un punto de la historia de una fuente con el que se vuelve a tener delante el material |
| **destino** | Ejecución | el despliegue al que vuelve un rollback |
| **salida simulada** | Simulación | lo que devuelve un comando simulado |
| **simulación** | Simulación | la entidad que se recorre |
| **autor** | Suministro | quien hizo un cambio en una fuente |
| **solicitante** | Historial | quien pidió un intento |

**Consecuencias.** **Enmienda `DEC-02.2`**: la lista cerrada de préstamos pasa a cuatro (*pipeline*,
*rollback*, *hash* y *commit*). *Marca* y *último ambiente* salen del lenguaje.

**Verificación.** Todos los términos aparecen en `lenguaje.md`, cada uno bajo un contexto.

---

## 6. Impacto en el modelo

| Documento | Qué cambió | Cuándo |
|---|---|---|
| `modelo/bounded-contexts.md` | **Reescrito**: la prueba, ocho contextos, lo que no es contexto, quién habla con quién (sin patrón) y, por contexto, propósito, términos, consumo y pruebas de separación | **IT-03 — hecho** |
| `modelo/lenguaje.md` | **Partido por contexto**: regla 5 (forma declarada), regla 6 (palabra común con dueño), cuatro préstamos, la tabla de homónimos rehecha por frontera, términos nuevos (`DEC-03.18`), la contradicción del lanzamiento corregida, y el core consumiendo solo del Historial | **IT-03 — hecho** |
| `modelo/dominio.md` | La autoría con dueño; Simulación como contexto propio; Suministro con autor y commit; rollback con los recursos de entonces; Sincronización realizada dentro del Historial; la restricción de ofuscación con dueño; y el lanzamiento en cualquier ambiente | **IT-03 — hecho** |
| `plan-ddd.md` | §6, el proceso nuevo (`DEC-03.15`); el encuadre de E3 enmendado (`DEC-03.2`); la plantilla aligerada; el tablero | **IT-03 — hecho** |

## 7. Impacto en el código

Esta iteración **no toca código** y, por `DEC-02.10`, **no lleva inventario**: la reconciliación con
lo construido es entregable único de E11.

## 8. Criterio de cierre

- [x] Todas las preguntas tienen respuesta, decisión por defecto o diferimiento con razón
      (27 preguntas; `Q-03.21` #1 decidida por defecto).
- [x] Hay **una sola prueba de separación**, escrita y con su forma (`DEC-03.1`).
- [x] Está dicho desde qué mapa se aplicó y que se admite un contexto con varios subdominios
      (`DEC-03.2`).
- [x] Cada contexto tiene **propósito, subdominios, términos propios y prueba de separación**
      contra cada vecino con el que se cruza, en `bounded-contexts.md`.
- [x] La única fusión (Sincronización dentro del Historial) está justificada con la prueba.
- [x] Ningún término significa dos cosas **dentro** de un contexto.
- [x] Cada homónimo está asignado a una frontera, incluidos los detectados al abrir (*pipeline*, los
      dos *quién*, las formas declaradas).
- [x] Verificación de `DEC-01.3`: la partición de Ejecución no reabrió la lista de subdominios.
- [x] Verificación de `DEC-01.13`: reescrita con la enmienda nombrada (`DEC-03.8`).
- [x] Corregida la contradicción sobre dónde ocurre un lanzamiento (`DEC-03.17`).
- [x] La fila de autoría tiene dueño (`DEC-03.12`).
- [x] Ningún argumento de esta iteración cita código ni specs (`DEC-02.10`).
- [x] Tablero de `plan-ddd.md` actualizado.

## 9. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Si un lanzamiento ocurre en cualquier ambiente, **¿qué es *el cliente* fuera de producción, y quién decide ahí?** `dominio.md` define Lanzamiento por el cliente final y el dueño del negocio | **primera pregunta de la vuelta siguiente** | No mueve ninguna frontera: Lanzamiento es contexto con cualquier respuesta |
| 2 | El **patrón** de cada arista de `bounded-contexts.md`, incluido cómo comparten mecanismo Simulación y Ejecución | **IT-04** | Es de E4 |
| 3 | ¿La secuencia de registros del Historial es **su modelo** o **su persistencia**? | **IT-05** | Es la pregunta de E5 del plan; `DEC-03.13` la dejó abierta a propósito |
| 4 | Cómo se ofusca el valor en el Historial y cómo se asegura que solo Resolución lo pida de vuelta | **IT-08** | Es táctico; la pérdida aceptada está escrita en `DEC-03.16` |
| 5 | La separación táctica entre decidir y hacer dentro de Ejecución | **IT-09** | `DEC-03.4` |

Las dudas **#4** (idioma de la frontera con CLI y portal) y **#7** (las `SPEC-*.md` como Published
Language) de IT-02 siguen asignadas a IT-04.

### Nota de proceso

Tres rondas, y la tercera no la cerró la lupa: la cerró el experto al cambiar el método. Las rondas
sirvieron para lo que tenían que servir: la única frontera sin prueba escrita cayó (Sincronización),
y tres choques entre respuestas correctas por separado solo aparecieron al leerlas juntas. Pero el
formato hacía que el experto eligiera entre opciones en vez de decir qué quería, y a ese coste el
proceso no avanzaba. `DEC-03.15` conserva el contraste con el libro y se queda sin el cuestionario.
