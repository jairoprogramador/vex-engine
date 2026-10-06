# IT-01 — Destilación del dominio

> Etapa: **E1** · Estado: **cerrada**
> Abierta: 2026-08-25 · Cerrada: 2026-08-26
> Lectura previa: `guia-ddd.md` §0, §1, §2
> Preguntas: **30** en 7 rondas (10 · 9 · 5 · 2 · 2 · 1 · 1) · Respondidas: **30** · Decisiones: **14**
>
> **Responder no fue cerrar.** Las diez primeras respuestas abrieron seis rondas de validación
> contra el libro: 9 → 5 → 2 → 2 → 1 → 1 preguntas nuevas, hasta que la última **confirmó sin
> cambiar nada**. Ésa fue la señal de cierre, no que se acabaran las preguntas.

---

## 1. Objetivo — qué tiene que ser verdad al cerrar

1. Cada subdominio tiene: propósito en una frase, clasificación (core / supporting / generic),
   la **razón** de esa clasificación, y qué se pierde si se clasifica mal.
2. Hay **un solo core domain** identificado y defendido con un argumento económico, no estético.
3. Está escrito qué subdominios genéricos son candidatos a **no escribirse**.
4. Están fijados los **criterios de éxito** de todo el ejercicio DDD: cómo sabremos, dentro de
   seis meses, si valió la pena.
5. `modelo/dominio.md` reescrito con todo lo anterior.

Lo que **no** se decide aquí: fronteras de contexto (E3), relaciones entre contextos (E4),
paquetes (E5), agregados (bloque B). Si una pregunta empieza a resolverse por ahí, se difiere y
se anota en §9.

---

## 2. Punto de partida

### 2.1 Lo que dice hoy el modelo

`modelo/dominio.md` clasifica así:

- **Core**: Diagnóstico.
- **Supporting**: Registro de despliegue · Ejecución de pipeline · Definición de pipeline ·
  Resolución de variables · Espacio de trabajo · Simulación de pipeline.
- **Generic**: Sincronización de estado · Gestión del proyecto a desplegar · Gestión del
  repositorio de pipeline.

`modelo/lenguaje.md` describe Diagnóstico con dos capacidades:

- **Plan automático** — se ejecuta antes de cada intento, valida pipeline y variables, compara
  contra el último despliegue exitoso. *«La información debe ser siempre exacta, nunca
  inferencial.»*
- **Diagnóstico de errores** — clasifica si un fallo es del producto (código fuente) o del
  ambiente (pipeline, variables). Un error en instrucciones afecta a todos los ambientes; uno
  en variables afecta a un ambiente y a un solo paso.

### 2.2 Lo que dice hoy el código

| Subdominio declarado | Paquetes | Líneas de dominio |
|---|---|---:|
| **Diagnóstico *(core)*** | — **ninguno** | **0** |
| Ejecución de pipeline | `pipeline`, `step`, `command` | ~8.000 |
| Registro de despliegue | `deployment`, `record`, `state`, `cache` | ~4.300 |
| Resolución de variables | disperso en `command` y `step` | *no separable* |
| Definición de pipeline | disperso en `pipeline`, `step`, `infrastructure/step` | *no separable* |
| Sincronización de estado | `sync`, `syncconfig` | 649 |
| Suministro del proyecto | `pipeline` (clonadores) | *no separable* |
| Suministro del pipeline | `pipeline` (clonadores) | *no separable* |
| Espacio de trabajo | `pipeline` (handler 05, workdir repo) | *no separable* |
| **Simulación de pipeline** | — **ninguno** | **0** |
| *(sin subdominio declarado)* | `fingerprint` | 975 |
| *(sin subdominio declarado)* | `notify` | 48 + infra |

Tres hechos que la tabla deja ver y que gobiernan esta iteración:

- **El core no existe.** Cero líneas. Todo el esfuerzo histórico está en supporting y generic.
- **Dos subdominios no tienen código y uno de ellos es el core.** El otro es Simulación.
- **`fingerprint` no tiene subdominio asignado**, y son 975 líneas con **tres
  especificaciones normativas propias** (`SPEC-v1.md`, `SPEC-PIPELINE-v1.md`,
  `SPEC-STEP-v1.md`), versionadas con token (`v1:`, `pipe-v1:`, `sf-v1:`) y con la regla de que
  cambiar lo que entra en un hash obliga a subir la versión. Es el código más cuidado del
  repositorio y está huérfano en el modelo.

---

## 3. Inventario de preguntas

Léelas todas antes de responder ninguna. Están ordenadas por dependencia: la 1 condiciona a la
2, y la 2 condiciona a casi todo lo demás.

---

### Q-01.1 — ¿Por qué elegiría alguien Vex y no otra herramienta?

**Contexto.** Es la pregunta que el libro pone antes que ninguna otra, porque el core domain se
define por valor de negocio, no por complejidad técnica. La parte más difícil de construir y la
parte que te diferencia no tienen por qué ser la misma — y cuando no lo son, invertir en la
difícil es el error clásico.

**Por qué importa.** Sin esta respuesta, la clasificación de Q-01.2 es una opinión. Con ella,
es una deducción.

**Opciones (no excluyentes; puedes ordenarlas por peso):**

- **A. Saber exactamente qué se va a ejecutar y por qué, antes de ejecutarlo.** El valor está
  en la certeza previa: nadie más te dice «esto va a re-ejecutar el paso 3 porque cambió esta
  variable, y va a saltarse el 1 y el 2 porque su contenido es idéntico al del despliegue del
  martes». *Consecuencia si es esto: el core es el Plan automático, y depende del sistema de
  huellas y del Registro para existir.*
- **B. No repetir trabajo que no hace falta.** El valor está en la velocidad: el motor decide
  con exactitud qué reviva y qué no, por contenido y no por convención. *Consecuencia: el core
  es el sistema de huellas y la decisión de re-ejecución.*
- **C. Poder responder después qué pasó, con precisión y sin logs.** El valor está en la
  trazabilidad consultable: intentos, despliegues, lanzamientos, versiones de producto,
  rollback anclado. *Consecuencia: el core es el Registro, que hoy es supporting y tiene 4.300
  líneas — la clasificación cambiaría por completo.*
- **D. Separar limpiamente el pipeline del proyecto.** El valor está en el modelo de
  pipelinecode reutilizable: un pipeline, N proyectos, variables por ambiente. *Consecuencia:
  el core es Definición de Pipeline.*
- **E. Otra cosa.** Dilo con tus palabras.

**Ventaja/desventaja de cada una como core**, en corto:

| | Ventaja | Desventaja |
|---|---|---|
| A | Es lo único que hoy nadie hace bien; encaja con «nunca inferencial» | No existe; hay que construirlo entero |
| B | Ya está construido y con el mejor diseño del repo | Es difícil de defender como *diferenciador*: suena a caché, y hay quien lo hace |
| C | Ya está construido y es lo que pide el portal | Trazabilidad la venden muchos; el diferenciador estaría en el *contenido* del registro, no en tenerlo |
| D | Es la forma del producto, lo que el usuario ve | Es un formato, y los formatos se copian en una tarde |

---

### Q-01.2 — ¿Cuál es el core domain?

**Contexto.** Hoy es *Diagnóstico*. La duda no es si Diagnóstico es valioso, es si es **el
core** o si es una **vista de consulta** construida sobre otros dos subdominios (Registro y
Definición) que serían los que de verdad llevan el peso.

Un dato que empuja en las dos direcciones a la vez: `lenguaje.md` dice que Diagnóstico *«explota
Registro + Definición»*. Un core domain que solo explota a otros dos es sospechoso — podría ser
una capa de presentación. Pero también podría ser exactamente lo que Vernon llama core: la parte
donde el conocimiento del negocio se convierte en una respuesta que nadie más sabe dar.

**Por qué importa.** Determina dónde va el mejor diseño, qué se modela primero en el bloque B, y
qué se congela. También determina si E6 (que ya está planificada como «Diagnóstico, en terreno
limpio») es la etapa correcta o hay que reordenar el plan entero.

**Opciones:**

- **A. Diagnóstico, tal como está declarado.** Se mantiene el plan: E6 modela el core en
  greenfield y sus necesidades dictan lo que Registro y Definición deben publicar.
  *Ventaja:* es la lectura que hace el libro cuando el core está sin construir — señala
  precisamente el trabajo pendiente más valioso. *Desventaja:* es el subdominio del que menos
  se sabe; modelarlo primero puede producir un modelo bonito y vacío.
- **B. La decisión de re-ejecución por contenido (el sistema de huellas).** El core sería la
  respuesta exacta a «¿qué hay que volver a ejecutar?». Diagnóstico pasaría a supporting: una
  vista sobre el core y el Registro.
  *Ventaja:* es lo más cuidado del repositorio, tiene tres specs normativas, y es una decisión
  de negocio real (no re-desplegar cuesta dinero). Explica por qué se ha invertido tanto ahí.
  *Desventaja:* si esto es el core, entonces el core **ya está construido**, y el rediseño pasa
  de «construir lo que falta» a «reorganizar lo que hay» — un plan distinto.
- **C. El Registro de Despliegue.** El core sería la trazabilidad: el objeto de despliegue con
  su `content_id`, los hechos, el linaje, el rollback anclado.
  *Ventaja:* 4.300 líneas y ocho specs dicen que ahí está el esfuerzo; es lo que consume el
  portal. *Desventaja:* «fuente de verdad» es una función de infraestructura de negocio; que
  algo sea imprescindible no lo hace diferenciador.
- **D. Un core con dos mitades: la decisión y la explicación.** Decidir qué re-ejecutar
  (huellas) y saber decir por qué (Diagnóstico) serían la misma competencia vista antes y
  después del intento.
  *Ventaja:* es la lectura más honesta con el código y con `lenguaje.md`; «Plan automático» y
  «re-ejecución por contenido» son literalmente la misma comparación de huellas, una antes de
  ejecutar y otra durante. *Desventaja:* Vernon desconfía de los cores dobles — suelen ser una
  forma de no elegir, y el objetivo de la destilación es precisamente elegir.

---

### Q-01.3 — «Ejecución de pipeline» mezcla decidir y ejecutar. ¿Se parte en dos subdominios?

**Contexto.** `dominio.md` define el subdominio así: *«Ejecución de pipeline: incluye el
concepto de intento; **decide qué re-ejecutar** apoyándose en Registro»*. Son dos capacidades
muy distintas metidas en la misma frase:

- **Decidir** qué se ejecuta — comparar huellas, aplicar reglas de re-ejecución, resolver el
  ancla de un rollback. Es razonamiento sobre el estado del mundo.
- **Ejecutar** — clonar, interpolar, lanzar comandos, capturar stdout, extraer variables. Es
  efecto sobre el mundo.

En el código están fundidas: `03_step_runner_handler.go` decide si el step revive y, si no,
entra en la cadena de comandos. Es el mismo eslabón. `vex-plan-revision.md` §A-2 ya lo señala
como problema («la misma pieza decide, hashea y escribe»).

**Por qué importa.** Si Q-01.2 responde B o D, la mitad «decidir» es **core** y la mitad
«ejecutar» es supporting — y tenerlas en el mismo subdominio garantiza que el core nunca reciba
tratamiento de core. Es el caso de libro donde la destilación consiste en *sacar* el core de
donde está enterrado.

**Opciones:**

- **A. Se parte en dos subdominios**, con nombres propios (p. ej. *Decisión de re-ejecución* y
  *Ejecución de pipeline*). *Ventaja:* hace visible el core y permite protegerlo; alinea el
  modelo con §A-2. *Desventaja:* dos subdominios donde el usuario ve una sola cosa; hay que
  defender la frontera en E3.
- **B. Sigue siendo uno**, y la separación se resuelve en el nivel táctico (agregados y
  servicios distintos dentro del mismo contexto). *Ventaja:* menos fronteras, menos traducción;
  el problema de §A-2 se puede arreglar sin partir nada. *Desventaja:* el core queda dentro de
  un subdominio supporting, que es justo lo que la destilación intenta evitar.
- **C. Depende de Q-01.2** — si el core acaba siendo Diagnóstico (opción A), la decisión de
  re-ejecución no es core y no hay razón para partir. Es una respuesta legítima: dilo y se
  resuelve por dependencia.

---

### Q-01.4 — ¿A qué subdominio pertenece el sistema de huellas (`fingerprint`)?

**Contexto.** 975 líneas de dominio, 1.208 de tests, tres reglas versionadas (`v1:` para el
árbol de archivos, `pipe-v1:` para lo que un step declara, `sf-v1:` para la identidad de un
step) y **tres especificaciones normativas** que viven junto al código. De él dependen al menos
tres subdominios: Ejecución (para decidir si un step revive), Registro (el `content_id` del
objeto de despliegue se deriva de ahí) y Suministro (proyecto y pipeline calculan su propia
huella).

En `dominio.md` no aparece. Es el activo más trabajado del repositorio y no tiene dueño.

**Por qué importa.** Un componente del que dependen tres subdominios, sin dueño, es el candidato
número uno a convertirse en un shared kernel involuntario — el mismo mecanismo que produjo el
paquete `command`. Y si Q-01.2 responde B, esto **es** el core y no puede seguir sin nombre.

**Opciones:**

- **A. Es el core, o el corazón del core.** Se le da nombre de subdominio propio (*Identidad de
  contenido*, o similar). *Ventaja:* explica la inversión histórica y protege el activo.
  *Desventaja:* solo coherente si Q-01.2 = B o D.
- **B. Es un subdominio genérico.** «Hashear un árbol de archivos de forma reproducible» lo
  hace git. *Ventaja:* honesto sobre el mecanismo. *Desventaja:* falso sobre las reglas — las
  tres divergencias deliberadas respecto a git (precedencia de reglas, prefijos anclados,
  `path.Match` en vez de `filepath.Match`) son decisiones de negocio, no de infraestructura, y
  `pipe-v1` hashea *declaraciones*, que git no sabe qué son.
- **C. Es parte de Definición de Pipeline.** La huella de un step es, al fin y al cabo, «qué
  declara este step». *Ventaja:* `pipe-v1` encaja perfecto ahí. *Desventaja:* `v1` (árbol de
  archivos del proyecto) no encaja nada — el proyecto no es el pipeline.
- **D. Se parte según la regla:** `v1` a Suministro, `pipe-v1` a Definición, `sf-v1` a quien
  decida la re-ejecución. *Ventaja:* cada regla acaba donde se usa. *Desventaja:* las tres
  reglas **componen** (`sf-v1` sobre `pipe-v1` sobre `v1`) y partirlas crea tres dependencias
  cruzadas donde hoy hay un paquete cohesionado. El repositorio tiene una regla explícita —
  «una regla, tres raíces, para que nadie introduzca una variante» — que esto rompería.

---

### Q-01.5 — ¿Qué es Simulación de Pipeline, y no será el mecanismo del Plan automático?

**Contexto.** Está declarada como supporting y no tiene código. `lenguaje.md` la describe como
*«mismo mecanismo de orquestar+ejecutar, pero sin invocar comandos reales; genera valores de
salida simulados usando la forma de salida esperada (regex); nunca persiste»*.

Y el Plan automático (dentro de Diagnóstico, el core) se describe como *«se ejecuta antes de
cada intento, valida pipeline + variables, compara contra el último despliegue exitoso»*.

Las dos descripciones se solapan mucho: las dos recorren el pipeline sin ejecutarlo para
decirte qué pasaría.

**Por qué importa.** Si son lo mismo, hay un subdominio de más y el core tiene un mecanismo que
no sabíamos que tenía. Si no lo son, la diferencia hay que escribirla, porque no se ve.

Hay además una tensión declarada que conviene resolver aquí: el Plan automático debe ser
*«siempre exacto, nunca inferencial»*, y la Simulación *«genera valores de salida simulados»*.
Un valor simulado es una inferencia. O el Plan automático no usa Simulación, o «exacto» significa
otra cosa (p. ej.: exacto sobre *qué se va a ejecutar*, inferencial sobre *qué va a salir*).

**Opciones:**

- **A. Son lo mismo.** Simulación desaparece como subdominio y pasa a ser el mecanismo del Plan
  automático, dentro del core.
- **B. Son distintos y hay que separarlos por su propósito.** Simulación sirve al *autor del
  pipeline* (probar un pipelinecode sin desplegar); el Plan automático sirve al *que despliega*
  (saber qué va a pasar ahora). Mismo mecanismo, dos usuarios, dos subdominios.
- **C. Simulación se aparca.** No existe, no está pedida por nadie hoy, y modelar lo que no
  existe es la vía rápida a un modelo especulativo. Se saca del modelo y se anota como
  candidata futura.
- **D. Simulación es genérica.** «Ejecutar en seco» es una función que tiene todo motor de
  pipelines.

**Y una subpregunta que hay que responder en cualquiera de los cuatro casos:** ¿«exacto, nunca
inferencial» aplica a *qué pasos se ejecutarán* (comprobable con huellas, exacto de verdad) o
también a *qué van a producir* (imposible sin ejecutar)?

---

### Q-01.6 — ¿Falta un subdominio de protección de valores sensibles?

**Contexto.** La spec 20 estableció un conjunto de reglas que hoy no pertenecen a ningún
subdominio declarado, y no son plomería:

- Ningún valor resuelto entra jamás en un hecho del registro; lo que entra es un digest
  `hmac-sha256`, con **clave por proyecto** — porque un `sha256` pelado de `REPLICAS=3` es una
  búsqueda en tabla, no un secreto.
- Ese `content_id`, en cambio, se mantiene **sin sal** a propósito: la comparabilidad entre
  organizaciones es su razón de existir. Las dos propiedades tienen que cumplirse a la vez.
- El secreto vive en el destino (`keys/digest-v1.key`), 0600, creado con `O_EXCL`, **nunca
  sobreescrito** — renombrarlo invalidaría en silencio todos los digests emitidos.
- La redacción del log ocurre en el borde de salida, con dos límites documentados: solo
  coincidencias literales de ≥ 8 bytes, y nunca la línea que revela el valor por primera vez.
- El estado (`state/`) se guarda en claro **a propósito**, para que un diagnóstico pueda decir
  «se re-ejecutó porque `DB_POOL_SIZE` pasó de 10 a 50».

Eso es un cuerpo de reglas coherente, con decisiones difíciles y compromisos explícitos, y hoy
está repartido entre `record`, `notify` e `infrastructure/record`.

**Por qué importa.** Si es un subdominio, tiene lenguaje y modelo propios y merece frontera. Si
no lo es, hay que decir a quién pertenecen esas reglas, porque son transversales y las
transversales sin dueño se erosionan.

**Opciones:**

- **A. Sí: un subdominio propio** (*Protección de valores*, o similar), supporting.
  *Ventaja:* las reglas tienen dueño y se pueden verificar en un sitio. *Desventaja:* es un
  concern transversal, y los subdominios transversales tienden a convertirse en contextos que
  todos los demás tienen que consultar.
- **B. No: pertenece al Registro**, que es quien decide qué se escribe y qué no.
  *Ventaja:* el registro ya es el dueño de los hechos, y la regla es sobre los hechos.
  *Desventaja:* la redacción del log no pasa por el registro; ocurre en `notify`.
- **C. No es dominio: es una política de infraestructura.** *Ventaja:* simple.
  *Desventaja:* difícil de sostener — decidir que el `content_id` va sin sal y el digest con
  clave por proyecto es una decisión de negocio sobre qué información puede salir de la
  organización.

---

### Q-01.7 — ¿Los tres subdominios genéricos son genéricos de verdad? ¿Alguno no debería escribirse?

**Contexto.** La pregunta correcta ante un genérico no es «cómo lo diseño» sino «puedo no
escribirlo». Los tres declarados:

**Sincronización de Estado** (`sync` + `syncconfig`, 649 líneas). Empuja `objects/` y `events/`
al destino, con `ack` como optimización de ancho de banda que *nunca* es checkpoint, reenvío
idempotente porque los objetos son write-once por `content_id` y el `seq` es único y monótono
dentro de una tira, y un fallo que produce un hecho `sync_failed` sin tumbar el pipeline.
*Eso no es plomería:* es un protocolo de replicación con reglas de negocio propias.

**Suministro del Pipeline.** Clona el repo del pipelinecode con una **ventana de reutilización**
(`clone_window`, 24 h por defecto) y emite `stale_clone_used` cuando reusa. Eso es una política
de frescura, con un compromiso declarado (dentro de la ventana, un push al pipelinecode no se
recoge).

**Suministro del Proyecto.** Clona o enlaza el repo del proyecto y calcula su huella. Tiene dos
implementaciones (clonado y local) que se eligen por configuración.

**Por qué importa.** Clasificar como genérico algo que tiene reglas propias garantiza que esas
reglas se implementen mal, en el sitio equivocado, y sin tests. Y al revés: clasificar como
supporting algo que es genérico consume el presupuesto de diseño.

**Opciones (responde una por cada uno de los tres):**

- **A. Genérico de verdad**, y se busca activamente cómo no escribirlo.
- **B. Genérico en el mecanismo, supporting en la política.** El clon es genérico; la ventana de
  frescura no. El transporte es genérico; la idempotencia y el `ack` no. Se parte.
- **C. Supporting**: tiene reglas propias suficientes para merecer modelo.

---

### Q-01.8 — ¿Espacio de Trabajo es supporting o genérico?

**Contexto.** Está declarado supporting. `lenguaje.md` lo describe como *«copia física mutable
del pipeline, por proyecto, donde se interpolan templates durante la ejecución»*, con ciclo de
vida propio (crear, usar, limpiar) y con la nota de que *«es lo único que realmente se puede
limpiar»*.

En el código es el handler 05 (`CopyWorkdirHandler`), un repositorio de workdir, y las
`fileSessions` del `ExecutionContext` que guardan copias de seguridad de las plantillas para
restaurarlas si el paso falla (la corrección del defecto D1).

**Por qué importa.** Es el subdominio más pequeño y el más fácil de reclasificar. Pero la
restauración de plantillas tras un fallo **sí** es una regla de negocio: fue un defecto real
(D1) y arreglarlo costó una spec.

**Opciones:**

- **A. Genérico.** Copiar un directorio y borrarlo es plomería. La restauración tras fallo
  pertenece a quien ejecuta, no a quien copia.
- **B. Supporting.** El ciclo de vida (crear / usar / restaurar / limpiar) y la garantía de
  aislamiento entre pasos son reglas propias.
- **C. No es un subdominio: es un detalle de Ejecución.** Se absorbe y desaparece del modelo.

---

### Q-01.9 — ¿Qué va a ser el motor dentro de doce meses?

**Contexto.** Hoy `vexd` es un binario **one-shot**: lee un `RequestInput`, ejecuta un pipeline,
escribe, sale. No hay servidor HTTP en el repositorio, no hay demonio. En remoto, el portal
levanta una Fly Machine efímera con `auto_destroy=true`.

**Por qué importa aquí y no en E5.** Cambia la clasificación de los subdominios, no solo la
arquitectura. Si el motor pasa a ser un servicio de larga vida, Sincronización de Estado deja de
ser plomería y se vuelve central. Si va a ejecutar varios pipelines en paralelo, la concurrencia
entra en el dominio (hoy `Execution` ya tiene un mutex y `ErrTransicionIlegal`, pero para un
único hilo de ejecución). Y si va a seguir siendo one-shot para siempre, hay complejidad que se
puede *retirar* del modelo, no solo reorganizar.

**Opciones:**

- **A. Sigue siendo one-shot, indefinidamente.** El proceso muere con el intento.
- **B. One-shot, pero ejecutando varios pipelines o ambientes en el mismo proceso.**
- **C. Va hacia un servicio de larga vida** (cola de intentos, estado en memoria entre
  ejecuciones).
- **D. No está decidido** — y entonces el modelo debe diseñarse para que la decisión no lo
  invalide, que es un requisito de diseño y hay que escribirlo como tal.

---

### Q-01.10 — ¿Cómo sabremos, dentro de seis meses, que este ejercicio valió la pena?

**Contexto.** Un rediseño profundo sin criterio de éxito se juzga por gusto estético, y el gusto
estético siempre dice que el código nuevo es mejor. El plan necesita una vara concreta.

**Por qué importa.** Es lo que permite cerrar E12 diciendo «esto está terminado» en vez de «esto
ya no se me ocurre cómo mejorarlo». También es lo que permite **rechazar** una propuesta de
rediseño durante el bloque B: si no mueve ninguno de estos criterios, no entra.

**Opciones (elige las que apliquen y ordénalas):**

- **A. El core se puede construir.** Diagnóstico (o lo que resulte ser el core) pasa de cero
  líneas a existir, y el modelo dice exactamente qué necesita de los demás.
- **B. Baja el coste de cambio.** Una funcionalidad nueva toca un contexto, no cuatro paquetes.
  Medible: hoy añadir un tipo de hecho toca `record`, `command`, `step`, `sync` y el factory.
- **C. Desaparece una clase entera de defecto.** Los §A-1..A-8 de `vex-plan-revision.md` (God
  Object, decide-hashea-escribe en el mismo sitio, reglas no extensibles) dejan de ser posibles
  por construcción, no por disciplina.
- **D. El modelo se puede explicar.** Una persona nueva entiende el motor leyendo `modelo/`, sin
  leer código. Hoy el documento que cumple esa función es un `CLAUDE.md` de 750 líneas cuyo
  contenido es, en buena parte, advertencias sobre trampas.
- **E. El lenguaje deja de tener homónimos.** «Step», «variable», «estado» y «registro»
  significan una sola cosa en cada contexto.
- **F. Otra cosa.**

---

## 3.bis Rondas de validación — contra Vernon

> **Ronda 2** abierta el 2026-08-26, contrastando §4 y §5 contra *Implementing DDD* cap. 2
> (dominios, subdominios, bounded contexts) y cap. 3 (context map). **Ronda 3** abierta el mismo
> día, a partir de lo que las respuestas de la ronda 2 dejaron al descubierto.
>
> **Responder no es cerrar.** Todo lo respondido queda provisional hasta que una ronda de
> validación no encuentre nada. Ocurrió en la **séptima**.

### Ronda 2 — Qué encontró la validación

Ocho hallazgos, aceptados. Cinco abren pregunta nueva; tres se resuelven escribiéndolos.

| # | Hallazgo | Estado |
|---|---|---|
| **H1** | «Capacidad Generic sin subdominio propio» (`DEC-01.4`) es una categoría que Vernon no tiene, y es la que deja abierta la contradicción con la regla del repositorio. El libro ya separa las tres piezas: **el algoritmo no es dominio**, la regla escrita y versionada **sí** lo es, y las huellas son conceptos distintos de cada subdominio que las usa | → `Q-01.11` |
| **H2** | Espacio de Trabajo falla la prueba práctica de Generic. Un Generic sigue siendo capacidad **de negocio**, indiferenciada y **comprable** — y de esto no hay proveedor. Su propia razón (*«es mecánica de infraestructura, no modelado de negocio»*) lo saca del espacio del problema. La opción C de Q-01.8 era la conforme | → `Q-01.12` |
| **H3** | Al degradar `clone_window` a configuración, Suministro del Proyecto y del Pipeline se quedaron con **la misma capacidad literal**. El actor distinto justifica dos **fuentes**, no dos subdominios | → `Q-01.13` |
| **H4** | Todos los términos de la definición del core son prestados. Un ámbito con vocabulario enteramente ajeno es una **consulta sobre el ámbito de otro**. El riesgo estaba registrado; el test para falsarlo, no | → `Q-01.14` |
| **H6** | `DEC-01.6` es tácticamente correcta —convierte una política transversal en invariante del valor en su origen— pero probablemente **amplía** la spec 20 en vez de derogarla: «comparar sin ver» ya existe. Y le falta decir **quién custodia** la invariante | → `Q-01.15` |
| **H7** | Simulación es el **único Supporting del que el core no depende**, y arrastra una contradicción heredada: *«mismo mecanismo de orquestar+ejecutar»* y *«contexto aislado»* no pueden ser verdad a la vez | → `Q-01.16` |
| **H5** | De los tres criterios que `DEC-01.3` deja apuntados para partir Ejecución en dos contextos, **solo uno es criterio de Vernon**: que haya que traducir al cruzar la frontera. Volatilidad y aislamiento en pruebas son razones de ingeniería, no de contexto | anotado para **IT-03** |
| **H8** | Los criterios de éxito miden que el lenguaje sobreviva **hablado**, no que llegue **al código** — que es la tesis central del libro | → `Q-01.19` |

Y dos preguntas que nacen al reescribir `dominio.md` como espacio del problema: falta nombrar
un actor (`Q-01.17`) y falta una regla de clasificación deducible (`Q-01.18`).

---

### Q-01.11 — Si el algoritmo de huella no es dominio, ¿qué parte de «saber si algo cambió» sí lo es?

**Contexto.** `DEC-01.4` inventó una categoría para no dejar la huella sin dueño. Con la
taxonomía del libro no hace falta: el **algoritmo** es técnica compartida y no entra en el mapa;
las **huellas** son conceptos propios de cada subdominio que las usa; y la **regla escrita y
versionada** —qué entra en el cálculo, y que cambiarla obliga a declarar una versión nueva— es
lo único con carga de negocio, porque si cambia sin avisar todo lo comparado antes deja de ser
comparable.

**Por qué importa.** Con el reencuadre desaparecen a la vez la contradicción con la regla del
repositorio («una regla, tres raíces») y la necesidad de que E4 elija patrón de relación para una
función de hash. Y se evita el riesgo real del texto actual: *«duplicada a propósito»* se puede
leer como permiso para tres implementaciones que diverjan.

**Opciones.**

- **A. Se adopta el reencuadre y la huella sale del mapa** como capacidad. Lo que queda en el
  modelo es la **regla escrita**, tratada como contrato versionado, y su dueño se decide en E4.
- **B. Se adopta el reencuadre, pero la regla escrita se declara desde ya propiedad de un
  subdominio concreto** — el candidato natural es Definición de Pipeline, que es quien declara
  qué cuenta como «un paso» y qué cuenta como «cambio».
- **C. Se mantiene `DEC-01.4` tal cual**, aceptando la categoría fuera de taxonomía, y se
  resuelve la contradicción con la regla del repositorio de otra manera.

---

### Q-01.12 — Si Espacio de Trabajo sale del mapa, ¿quién absorbe «cada ambiente trabaja aislado»?

**Contexto.** `DEC-01.8` lo clasificó Generic con el argumento de que es infraestructura. Por la
taxonomía del libro esa frase lo saca del espacio del problema entero: un Generic es capacidad de
negocio, solo que indiferenciada — y la prueba práctica es si tiene proveedor. Git, sí. Un
almacén de estado, sí. «Gestión de espacios de trabajo para un motor de pipelines», nadie.

**Por qué importa.** El mapa baja de diez subdominios a nueve y una expectativa real del negocio
—que un ambiente no pise a otro, y que un intento fallido no deje el material a medio tocar— se
queda sin dueño si no se dice a quién pasa.

**Opciones.**

- **A. La absorbe Ejecución de Pipeline**, junto con la restauración tras fallo que `DEC-01.8` ya
  le había mandado. Es *cómo* Ejecución cumple lo suyo.
- **B. La absorbe Sincronización de Estado**, por ser quien ya custodia lo que un intento deja.
- **C. Se queda como subdominio Generic**, aceptando que no pasa la prueba del proveedor.

---

### Q-01.13 — ¿Un «Suministro de Fuentes» o dos subdominios de suministro?

**Contexto.** Se mantuvieron separados por actor de negocio. Pero en la misma decisión se degradó
la ventana de reutilización del clon a *«configuración de frecuencia»*, y ésa era la única
capacidad que uno tenía y el otro no. Lo que queda son dos subdominios con la misma capacidad
literal, la misma prueba de Generic y los mismos candidatos a no escribirse.

**Por qué importa.** El espacio del problema se parte por lenguaje y por experto. Aquí el
lenguaje es idéntico y el experto solo cambia de qué repo es dueño. Y hay coste práctico: duplica
en E10 el trabajo de evaluar si se compra o se escribe.

**Opciones.**

- **A. Uno solo — «Suministro de Fuentes» — con dos fuentes declaradas** (el producto, del
  programador; el pipeline, del DevOps). Una evaluación en E10 en vez de dos.
- **B. Siguen siendo dos**, y entonces hay que nombrar qué capacidad tiene uno que el otro no,
  ahora que la ventana ya no cuenta.

---

### Q-01.14 — Nombra tres términos que sean solo de Diagnóstico

**Contexto.** Es la pregunta que decide si el core existe. Hoy **todos** los términos de su
definición son prestados: *fallo*, *ejecución*, *despliegue exitoso*, *pipeline*, *variables*,
*ambiente* — cada uno tiene dueño en otro subdominio. Un ámbito cuyo vocabulario es enteramente
ajeno no es un ámbito: es una consulta sobre el ámbito de otro, y el libro es explícito en que
eso **no** es un core.

**Por qué importa.** `DEC-01.2` tiene la válvula escrita —*«si sale vacío, se vuelve a E1»*— pero
la difiere entera a E6, cuatro iteraciones más tarde. Esta pregunta la convierte en comprobable
**ahora**, y es el puente natural con IT-02, que va justo de lenguaje.

**Qué se pide.** Al menos **tres** términos que pertenezcan a Diagnóstico y a nadie más, y que no
se puedan decir sin pérdida en el lenguaje del Registro. Candidatos que el propio dominio sugiere,
para arrancar y no para copiar: *causa* · *atribución* · *evidencia* · *descarte* · *ventana de
comparación* · *sospechoso*.

**Cómo se lee el resultado.**

- **Salen tres o más, con definición propia** ⇒ el core existe y E6 tiene de dónde tirar.
- **Salen menos de tres** ⇒ Diagnóstico es una vista de consulta, `DEC-01.2` cae, y se vuelve a
  Q-01.2 con el candidato siguiente. Está permitido y es barato ahora; en E6 no.

---

### Q-01.15 — La protección de valores, ¿deroga la spec 20 o la amplía? ¿Y quién la custodia?

**Contexto.** `DEC-01.6` declaró que toda variable se ofusca por defecto y que eso deroga la
spec 11 §5.3 y la tabla de la spec 20. Pero «comparar sin ver» ya existe hoy: los hechos guardan
una marca del valor, con clave por proyecto, precisamente para poder comparar sin exponer. Si la
comparación del core se hace sobre esas marcas, lo único que `DEC-01.6` añade de verdad es
**extender ese mismo trato a lo que hoy se guarda en claro**.

**Por qué importa.** Cambia el tamaño de la decisión: derogar dos specs y reescribir el criterio,
o derogar una y ampliar la otra. La segunda es más barata, más coherente y no rompe nada que hoy
funcione. Y queda un hueco aparte: una invariante sin nadie que la custodie es una convención —
hay que decir **qué concepto es dueño del valor** y **cuál es la única operación** que lo entrega
en claro.

**Opciones.**

- **A. Amplía.** Se deroga solo la spec 11 §5.3 (lo que se guarda en claro), y la regla de
  comparar por marca se extiende a ese sitio. La spec 20 se conserva entera.
- **B. Deroga ambas**, como está escrito hoy, y se reescribe también cómo se compara.
- **C. Se reabre la pérdida aceptada**: puede que decir *«pasó de 10 a 50»* valga más de lo que
  se supuso, y que la ofuscación por defecto deba tener una excepción declarada por el DevOps.

**En cualquiera de las tres**: nombrar el custodio del valor y la operación que lo expone.

---

### Q-01.16 — Simulación: ¿sobrevive, y con qué relación con Ejecución?

**Contexto.** Es el único Supporting del que **el core no depende**, el único sin usuario real hoy
y el único cuya descripción se contradice: *«mismo mecanismo de orquestar y ejecutar»* y
*«contexto aislado, solo depende de Definición»* no pueden ser verdad a la vez. Si comparte el
mecanismo, comparte modelo — y compartir modelo es lo que el libro trata como mal necesario a
minimizar, nunca como algo que se hereda sin decidirlo.

**Por qué importa.** `DEC-01.5` lo sostiene por actor, momento y propósito, y eso está bien
argumentado. Lo que no está decidido es qué es respecto de Ejecución, y de eso depende que en E10
haya un contexto nuevo o un modo de uno existente.

**Opciones.**

- **A. Subdominio propio, y en la solución adopta el modelo de Ejecución sin traducirlo.** Barato,
  y el modelo ajeno entra tal cual.
- **B. No es subdominio: es un modo de Ejecución de Pipeline** — intentar sin efectos. El actor
  distinto no basta si el lenguaje es el mismo.
- **C. Sobrevive en el mapa pero no se modela** hasta que haya un escenario real. El libro
  desconfía de modelar lo que no existe.

---

### Q-01.17 — ¿Hay un tercer actor?

**Contexto.** El modelo habla de **lanzamiento**, **estrategia de lanzamiento** y **versión de
producto**. Son conceptos de negocio puros —no técnicos— y **no son del programador ni del
DevOps**: responden a *«¿qué hay hoy en producción y de dónde salió?»*, que es una pregunta de
quien decide qué sale.

**Por qué importa.** El espacio del problema se parte por experto. Si ese actor existe, el
Registro tiene dos clientes con lenguajes distintos y eso condiciona E3. Si no existe —si hoy es
el mismo DevOps con otro sombrero— hay que escribirlo, porque un concepto de negocio sin actor es
un concepto que nadie va a defender cuando estorbe.

**Opciones.**

- **A. Existe y hay que nombrarlo** (responsable de release, o como se llame en tu contexto).
- **B. No existe todavía**: hoy es el DevOps, y lanzamiento y versión de producto son capacidad
  suya. Se anota como actor futuro.
- **C. Lanzamiento y versión de producto no son de este dominio** y salen del modelo.

---

### Q-01.18 — ¿Se adopta la regla de clasificación por garantías?

**Contexto.** Las diez clasificaciones de la primera ronda se defendieron una a una, con
argumentos distintos cada vez. Al reescribir `dominio.md` apareció un criterio que las explica
todas a la vez:

> **Core** — la respuesta por la que te eligen.
> **Supporting** — o el core depende de una **garantía** suya, o es una capacidad propia del
> negocio que **ningún proveedor cubre**.
> **Generic** — presta un **servicio** que cualquiera presta igual y **se puede comprar**.

**Por qué importa.** Con la regla escrita, clasificar deja de ser un juicio por subdominio y pasa
a ser una deducción — y un subdominio nuevo se clasifica solo. Es además lo que hace visible el
caso raro: Simulación es el único que entra por la segunda pata, y por eso es el más frágil.

**Opciones.**

- **A. Se adopta**, y `dominio.md` la encabeza.
- **B. Se adopta con una tercera pata** que hoy falta y que habría que nombrar.
- **C. No se adopta**: se prefiere clasificar caso por caso.

---

### Q-01.19 — ¿Se enmienda `DEC-01.10` para medir el lenguaje **en el código**?

**Contexto.** El frente 1.3 mide que el lenguaje sobreviva **hablado** con alguien externo. La
tesis central del libro es más dura: el lenguaje ubicuo tiene que estar **en el código** —en los
nombres de lo que se programa—, no solo en `modelo/`. Un modelo que se habla bien y se lee mal es
el fallo típico del DDD documental, y es el que este proceso tiene más cerca: hay 14.000 líneas ya
escritas con otro vocabulario.

**Por qué importa.** Sin este criterio, IT-01 puede darse por buena con un `modelo/` impecable
sobre un código que sigue diciendo otra cosa.

**Opciones.**

- **A. Se añade al frente 1**: *un nombre del core se lee en voz alta a un DevOps y significa lo
  mismo que en `lenguaje.md`*.
- **B. Se añade al frente 2** como quinto criterio de aceptación de una propuesta: *acerca el
  código al lenguaje*.
- **C. No se añade**: se da por cubierto con el frente 1.3.

---


---

### Ronda 3 — Lo que abren las respuestas de la ronda 2

> Cinco preguntas. Cuatro nacen de `Q-01.14` y `Q-01.17`, que resultaron ser las dos que más
> movían el modelo; la quinta cierra el corte de `Q-01.15`.

---

### Q-01.20 — ¿Lanzamiento es un subdominio propio, separado de Registro de Despliegue?

**Contexto.** `Q-01.17` dice, con estas palabras, que un lanzamiento *«parece parte del mismo
despliegue, pero no lo es»*: hay alguien —el dueño del negocio— que decide **cuándo** el producto
llega al cliente y **con qué nombre**, y cuando no está, se asume que acepta enviarlo en cuanto
esté listo.

Con la prueba del libro, la separación pasa las tres patas: experto distinto (dueño del negocio
vs. DevOps), lenguaje sin solapamiento (*lanzamiento*, *listo para el cliente*, *etiqueta de
negocio* frente a *intento*, *despliegue*, *ambiente*) y capacidad que existe fuera del software
— decidir cuándo un producto sale al mercado es un oficio entero.

**Por qué importa.** Registro de Despliegue es hoy el subdominio con más peso del mapa después
del core. Si lleva dentro una capacidad con otro dueño y otro lenguaje, es un candidato claro a
que esa capacidad no se modele nunca bien: la del actor que no está en la sala gana siempre la
que sí.

**Opciones.**

- **A. Subdominio propio — *Lanzamiento*.** Supporting, con el dueño del negocio como experto.
  Registro se queda con la memoria de lo desplegado; Lanzamiento con la decisión de entregar.
- **B. Sigue dentro de Registro de Despliegue**, y se nombra al dueño del negocio como segundo
  cliente del mismo subdominio.
- **C. Sigue dentro, pero se ancla la razón**: lanzar es *un tipo de anotación* sobre un
  despliegue, y no una decisión con vida propia.

**Si sale A**, hay que decir además si entra en la cadena de garantías del core o si —como
Simulación— entra por la segunda pata.

---

### Q-01.21 — ¿Contra qué compara el core: el último despliegue exitoso, o lo que el cliente está usando?

**Contexto.** `DEC-01.2` fija el punto de referencia del core: *«el último despliegue exitoso
conocido»*. Si desplegar y lanzar son cosas distintas —`Q-01.17`— entonces en producción pueden
convivir dos verdades: lo último que se desplegó bien, y lo que de verdad está delante del
cliente porque es lo último **lanzado**.

**Por qué importa.** Es el punto de referencia del core, no un detalle. Si un fallo lo reporta un
cliente, la comparación correcta es contra lo que ese cliente usa. Si lo reporta el programador
al desplegar, es contra el último despliegue. Elegir mal hace que la atribución sea exacta
respecto de la cosa equivocada, que es peor que ser inexacta.

**Opciones.**

- **A. Siempre el último despliegue exitoso.** El lanzamiento no cambia el punto de referencia,
  solo etiqueta. Simple, y puede mentirle a quien pregunta desde producción.
- **B. Depende de quién pregunta**: el programador compara contra el último despliegue; el dueño
  del negocio, contra el último lanzamiento. Dos referencias declaradas, un core que sabe cuál
  usar.
- **C. Siempre lo que está vivo ante el cliente**, y el despliegue no lanzado se trata como un
  estado intermedio del que el core no responde todavía.

---

### Q-01.22 — ¿El core admite dos clases de salida: el hecho y la lectura?

**Contexto.** `DEC-01.2` promete *«datos exactos, no inferencia ni intuición humana»*. Y
`Q-01.14` introduce la **reincidencia** con la frase *«nos puede dar indicios si es algo
momentáneo o perenne»*. Un indicio es una inferencia. Las dos cosas no caben en la misma promesa
sin decir cómo.

**Por qué importa.** Es el contrato del core con quien le pregunta. La promesa de exactitud es lo
que separa a Vex de una herramienta que «sugiere»; abrirla sin declararla la disuelve. Pero
negarse a leer la reincidencia tira una de las señales más útiles que tienes.

**Opciones.**

- **A. Una sola clase, y la reincidencia es un hecho.** El core entrega *«este fallo se ha
  repetido 4 veces con el mismo pipeline»* y la lectura la hace la persona. La promesa de
  exactitud queda intacta y el core no opina nunca.
- **B. Dos clases declaradas y distinguibles**: **hecho** (exacto, comprobable) y **lectura**
  (probable, derivada de hechos). El core puede decir *«probablemente sea del ambiente»* siempre
  que quede marcado como lectura y nunca se confunda con un hecho.
- **C. Una sola clase, y la reincidencia entra en la evidencia** de la causa que se atribuye, sin
  ser una salida por sí misma.

---

### Q-01.23 — ¿La atribución nombra también a un responsable? Y si sí, ¿quién promete la autoría?

**Contexto.** `Q-01.14` incluye *autor del cambio (correo)* y *fecha del último resultado
exitoso* entre lo que Diagnóstico necesita. Eso hace dos cosas: amplía el espacio de respuestas
del core —de *qué* a *qué y quién*— y crea una promesa que **hoy nadie hace en el mapa**.
Suministro de Fuentes promete que el material está delante y que sabe si cambió; no promete
**quién lo cambió ni cuándo**.

**Por qué importa.** Es la primera vez que el mecanismo de `DEC-01.2` funciona: el core pide, y
otro subdominio tiene que prometer. Pero también toca algo delicado — atribuir a una persona no
es lo mismo que atribuir a un origen, y conviene que sea una decisión y no un efecto lateral.

**Opciones sobre el alcance.**

- **A. El core atribuye origen y responsable.** *«Es del pipeline, lo cambió Fulano el martes.»*
- **B. El core atribuye origen; la autoría es evidencia** que acompaña, no parte de la respuesta.
- **C. La autoría queda fuera** del modelo por ahora.

**Y, si sale A o B, quién lo promete.**

- **Suministro de Fuentes** — es quien tiene delante el material y su historia. Encaja, pero le
  añade una promesa a un Generic, y los Generic no prometen: prestan servicio.
- **Registro de Despliegue** — anota quién pidió cada intento. Encaja con «quién desplegó», no
  necesariamente con «quién escribió el cambio».

---

### Q-01.24 — ¿Se acepta partir la protección de valores en dos, y sale la mitad de abajo del modelo?

**Contexto.** `Q-01.15` aclaró que ofuscar **no es cifrar**: se ofusca para guardar, y eso es de
quien persiste. `DEC-01.6` tenía las dos cosas juntas, y por eso parecía transversal.

El corte que se propone:

| | Promete | Dónde |
|---|---|---|
| **Comparar sin exponer** | se sabe si una variable cambió respecto del último resultado exitoso, sin que el valor circule | **Resolución de Variables** — dominio, y garantía de la que depende el core |
| **Ofuscar al guardar** | lo que queda en reposo no es legible de un vistazo | **fuera del modelo** — política de quien persiste |

**Por qué importa.** Con el corte, la mitad de dominio es una promesa concreta que sostiene una
categoría entera de la respuesta del core, y la otra mitad deja de arrastrar al mapa una decisión
que no es de negocio. Sin el corte, la protección vuelve a ser un asunto de todos.

**Opciones.**

- **A. Se acepta el corte** tal como está.
- **B. Se acepta, pero la mitad de abajo se queda anotada en el modelo** como restricción
  declarada, aunque no sea subdominio.
- **C. No se parte**: la protección entera es de Resolución de Variables, incluida la forma de
  guardarla.


---

### Ronda 4 — Dos preguntas

> La ronda 2 tuvo nueve, la 3 cinco, esta tiene dos. Las dos salen de `Q-01.21` y `Q-01.22`, que
> son las respuestas que más apretaron el modelo.

---

### Q-01.25 — ¿De quién es el **orden** de los ambientes?

**Contexto.** `Q-01.21` define desplegar como colocar el código **sucesivamente** en dev →
staging → producción, *«cada uno una preparación»*. Esa palabra —sucesivamente— introduce algo
que hoy no está en el mapa: **los ambientes tienen un orden**, y una cosa *avanza* por ellos.

Hoy el modelo trata cada ambiente como independiente: cada uno con sus valores, su historia y su
estado. Nadie es dueño de *«esto ya pasó por staging, ahora va a producción»*, ni de *«esto está
en staging y todavía no ha llegado a producción»*.

**Por qué importa.** Es la condición de la escena diagnóstica canónica: *«funcionó en staging y
falla en producción»*. Sin orden declarado, esa frase compara dos ambientes cualesquiera; con
orden, compara una cosa consigo misma en dos momentos de su avance — que es una pregunta
distinta y mucho más fuerte. Y sin dueño, la progresión se acabará implementando por convención
en el sitio que toque.

**Opciones.**

- **A. De Definición de Pipeline.** Declara los ambientes, así que declara también su orden. La
  progresión es parte de *cómo se despliega este producto*.
- **B. Repartido: Definición declara el orden, Registro sabe por dónde va cada cosa.** Dos
  promesas distintas sobre el mismo hecho.
- **C. Es un subdominio propio** — *Promoción*, o como se llame: hacer avanzar algo por una
  cadena de preparaciones hasta que esté listo para lanzarse. Tiene actor (el programador, o el
  DevOps) y existe fuera del software.
- **D. No hay orden en el modelo.** Los ambientes son independientes y la sucesión es una
  costumbre del usuario, no una regla. Es una respuesta legítima, pero entonces hay que quitar
  «sucesivamente» del lenguaje.

---

### Q-01.26 — Si el core solo entrega hechos, ¿**nombra** la causa o **presenta** la evidencia?

**Contexto.** `Q-01.22` cerró que el core solo muestra hechos exactos y no opina nunca. Pero
`DEC-01.1` vende exactamente lo contrario de no opinar: *«saber con certeza si es tu código o el
pipeline»*. Nombrar una causa a partir de evidencia parece una conclusión, no un hecho.

**Por qué importa.** Es el contrato del core dicho con precisión, y de él depende que la promesa
de exactitud y la promesa de atribución puedan convivir. También decide qué se modela en E6: un
ámbito que **concluye** tiene reglas de deducción; uno que **presenta** tiene un modelo de
evidencia y nada más.

**La distinción que hay que resolver.**

- **Inferencia** — *«esto se ha repetido cuatro veces, probablemente sea del ambiente»*. Va de
  probabilidad. Ya está descartada por `Q-01.22`.
- **Deducción** — *«el pipeline es idéntico en los dos ambientes; funcionó allí y falla aquí; lo
  único distinto es esta variable; por tanto la causa está en las variables»*. No va de
  probabilidad: **es una conclusión forzosa** dada la invariante de Definición. Es exacta.

**Opciones.**

- **A. El core nombra la causa, y es una deducción.** Las conclusiones son forzosas porque los
  pasos son idénticos entre ambientes — y eso convierte la invariante de Definición de
  *restricción habilitante* en la **premisa** del razonamiento del core. La promesa de exactitud
  se mantiene y la de atribución también.
- **B. El core presenta la evidencia ordenada y la persona nombra la causa.** Máxima pureza en la
  promesa de exactitud, y `DEC-01.1` hay que reescribirla: lo que se vende ya no es «saber», es
  «tener delante todo lo que hace falta para saber».
- **C. Depende del caso**: nombra la causa cuando la deducción es forzosa y presenta evidencia
  cuando no lo es. Honesto, pero exige declarar **cuándo** es forzosa, y eso es una regla del
  modelo, no una decisión caso por caso.

**Si sale A**, hay una consecuencia que conviene ver ahora: la invariante de Definición deja de
ser una condición externa y pasa a ser parte del razonamiento del core. Debilitarla no
empeoraría la respuesta — **la invalidaría**.


---

### Ronda 5 — Dos preguntas, ambas consecuencia de elegir «deducción»

> No es que aparezcan cosas nuevas: es que elegir A en `Q-01.26` obliga a declarar dos cosas que
> antes se podían dejar implícitas. Son las últimas piezas estructurales que faltan.

---

### Q-01.27 — ¿El core compara una cosa o dos?

**Contexto.** Hay dos escenas de diagnóstico, y el modelo solo ha declarado el punto de
referencia de una:

| Escena | Compara | Qué necesita |
|---|---|---|
| **(a)** *«Ayer desplegué a producción y funcionó; hoy falla»* | el mismo ambiente, dos momentos | el último despliegue exitoso **en ese ambiente** — declarado en `Q-01.21` |
| **(b)** *«Funcionó en staging y falla en producción»* | dos ambientes, en el orden declarado | el **orden** de `Q-01.25`, y la invariante de pasos idénticos |

`Q-01.21` respondió la (a). Pero la ventaja competitiva de `DEC-01.1` se apoya en la invariante
de pasos idénticos, **que solo hace falta para la (b)** — y `Q-01.26` acaba de convertir esa
invariante en la premisa del razonamiento. La escena que sostiene la promesa no es la que tiene
punto de referencia declarado.

**Por qué importa.** Son dos deducciones distintas, con premisas distintas y con evidencia
distinta. Si el core hace solo la (a), la invariante de pasos idénticos no es su premisa y
`Q-01.26` queda a medias. Si hace las dos, hay que decir cuándo usa cada una — y esa regla es del
modelo, no del que programa.

**Opciones.**

- **A. Las dos, y la regla es explícita**: si hay un despliegue exitoso previo del mismo contenido
  en el ambiente anterior, se compara contra él (escena b); si no, contra el último éxito del
  mismo ambiente (escena a). Dos deducciones declaradas.
- **B. Solo la (a).** El core compara siempre dentro del mismo ambiente. Más simple, y hay que
  reescribir por qué la invariante de pasos idénticos es la premisa — porque entonces no lo es.
- **C. Solo la (b).** El core es esencialmente comparador entre ambientes, y la escena (a) es un
  caso particular donde el ambiente anterior es él mismo en otro momento.

---

### Q-01.28 — ¿Qué dice el core cuando **no puede** deducir?

**Contexto.** `Q-01.26` = A dice que el core concluye porque la conclusión es forzosa. Pero
forzosa no siempre lo es: el pipeline pudo cambiar entre los dos despliegues que se comparan, la
evidencia puede faltar, pueden haber cambiado el código **y** una variable a la vez. En esos casos
la deducción no está disponible.

**Por qué importa.** Es la mitad silenciosa del contrato. Un core que promete exactitud y no
declara qué hace ante la duda acaba **suponiendo**, y suponiendo sin marcarlo — que es exactamente
lo que `Q-01.22` y `Q-01.26` prohibieron. Y en términos del libro: si «no puedo concluir» es una
respuesta legítima del core, entonces es un **concepto del dominio** con su propio nombre, no un
error ni un hueco.

**Opciones.**

- **A. Es una respuesta más, con nombre propio** —*indeterminado*, *no concluyente*— y viene
  acompañada de la evidencia y de qué faltó para poder concluir. La promesa de exactitud se
  mantiene entera: el core nunca supone, y decirlo también es un hecho.
- **B. El core no responde**: si no puede deducir, calla y muestra la evidencia. La persona
  concluye. Diferencia sutil con A y consecuencias distintas — A modela la indeterminación, B la
  trata como ausencia de respuesta.
- **C. Se acota el espacio de causas para que siempre haya una.** Cualquier caso no deducible cae
  en una categoría *«mixto»* o *«varias causas»*. Evita el hueco, y a cambio inventa respuestas
  que no son forzosas — que es lo que se acaba de descartar.

**En cualquiera de las tres**, hay que decir si el core distingue *«no puedo concluir porque falta
evidencia»* de *«no puedo concluir porque cambiaron dos cosas a la vez»*. Son diagnósticos
distintos para quien recibe la respuesta: en el primero hay algo que arreglar en el registro; en
el segundo, algo que arreglar en cómo se trabaja.


---

### Ronda 6 — Una pregunta

> 9 → 5 → 2 → 2 → 1. La única que queda no cuestiona nada del modelo: pone por escrito qué forma
> tiene la respuesta del core cuando la eliminación no deja un solo candidato.

---

### Q-01.29 — Cuando quedan dos candidatos, ¿qué entrega el core?

**Contexto.** `DEC-01.2` dice que el core *«determina y comunica la causa»* —en singular— dentro
de un espacio cerrado de tres categorías. `Q-01.27` muestra un caso, y no es raro, en el que la
eliminación deja **dos**: mismo ambiente, dos momentos, las instrucciones no cambiaron, y quedan
el código y las variables.

En ese caso el core sabe con exactitud **qué cambió** —los dos ejes— y no puede reducirlo a uno.
No le falta evidencia (`Q-01.28`) ni tiene que inferir: sencillamente la eliminación no llega a
uno solo.

**Por qué importa.** Decide qué se modela en E6. Un core que devuelve **una categoría** y otro que
devuelve **el conjunto de candidatos que sobreviven** son dos modelos distintos, con dos formas de
respuesta distintas. Y no es un caso marginal: es el caso normal de la escena más frecuente
—«ayer funcionó, hoy falla»—, que es con la que empieza cualquiera.

**Lo que ya está decidido y acota las opciones**: el core no infiere (`Q-01.22`, `Q-01.26`), así
que «elige el más probable de los dos» está descartado de antemano.

**Opciones.**

- **A. El conjunto que sobrevive.** La respuesta es *«la causa está entre el código y las
  variables; las instrucciones no cambiaron»*, y sigue siendo un hecho exacto. Con un candidato se
  lee como una atribución; con dos, como una eliminación parcial. **Sigue encaminando**: descarta
  al DevOps del pipeline aunque no distinga entre el programador y la configuración del ambiente.
  Obliga a reescribir el espacio de respuestas de `DEC-01.2`: de tres categorías, al subconjunto
  que queda.
- **B. Una categoría, o ninguna.** Si la eliminación no llega a un solo eje, el core no atribuye:
  entrega la evidencia —qué cambió y qué no— y no nombra causa. Mantiene `DEC-01.2` intacta y
  renuncia a encaminar en el caso más frecuente.
- **C. El conjunto, y además qué haría falta para reducirlo.** Como A, más una indicación
  accionable: *«compara contra staging y sabrás cuál de los dos es»*. Es lo que un DevOps con
  experiencia diría, y es exacto — pero convierte al core en algo que además **aconseja**, que es
  una capacidad distinta de atribuir y habría que decir si entra.

**En cualquiera de las tres**, hay que decidir si esto cambia la frase de `DEC-01.1`. Hoy dice
*«saber si es tu código o si cambió algo del pipeline/ambiente»*, que promete distinguir siempre
entre las dos cosas — y en la escena de mismo ambiente no siempre se puede.


---

### Ronda 7 — Una pregunta, y no es sobre el core

> Sale de aplicar la regla de `Q-01.18` a un caso que el modelo de tres ejes acaba de volver
> load-bearing. Es exactamente para lo que sirve validar: la regla se adoptó en abstracto y ahora
> se prueba contra un caso concreto que antes no existía.

---

### Q-01.30 — ¿De quién consume el core la identidad del código y del pipeline?

**Contexto.** El modelo de tres ejes (`Q-01.27`) dice que el core deduce comparando si cambió el
código, si cambiaron las instrucciones y si cambiaron las variables. Las dos primeras son
**identidades de una fuente**, y hay dos candidatos a prometerlas:

- **Suministro de Fuentes**, que es quien las **calcula** —tiene el material delante y sabe si
  cambió—, y está clasificado **Generic**;
- **Registro de Despliegue**, que es quien las **conserva** —«se sabe con qué se hizo» cada
  despliegue—, y está clasificado Supporting con fila propia en la cadena de garantías.

**Por qué importa, y por qué no es una sutileza.** `dominio.md` afirma hoy que *«los dos Generic
no prometen nada al core: prestan un servicio a los demás»*. Si el core consume la identidad
directamente de Suministro, esa frase es **falsa** y la regla de clasificación se rompe en su
primer caso difícil: Suministro sería Supporting por la primera pata (el core depende de una
garantía suya) y Generic por la tercera (se puede comprar) **a la vez**.

Y hay una respuesta que lo resuelve sin tocar nada: el core no lee la fuente, lee el **registro**
de lo que se usó. Suministro calcula la identidad en el momento de ejecutar; Registro la conserva;
el core compara dos registros. Encaja además con lo que ya se decidió en `Q-01.27` —la comparación
es contra *el pipeline que se usó*, no contra el de hoy—, que solo Registro puede sostener.

**Opciones.**

- **A. De Registro de Despliegue.** Suministro calcula la identidad y se la entrega a quien
  ejecuta; Registro la conserva; el core compara registros y nunca habla con la fuente. La regla
  aguanta, Suministro sigue siendo un servicio puro, y la cadena de garantías no cambia.
- **B. De Suministro de Fuentes.** El core pregunta directamente si cambió. Entonces hay que
  **ascender Suministro a Supporting** o **arreglar la regla**, porque como está permite las dos
  clasificaciones a la vez.
- **C. La regla necesita una tercera pata, se elija A o B.** Que el core dependa de algo no lo
  hace Supporting —el libro insiste en que un Generic puede ser crítico—; lo que lo hace
  Supporting es que la garantía sea **propia del negocio y no la venda nadie**. Ordenar las patas
  en vez de ofrecerlas en paralelo: *«¿se puede comprar? → Generic, por mucho que el core se
  apoye en ello»*.

**Nota sobre A y C**: no son excluyentes. A resuelve el caso; C evita que el siguiente vuelva a
abrirlo.

## 4. Respuestas y análisis

> Se conservan **tal como se escribieron**, incluidas las que la validación después modificó o
> retiró: son el registro de con qué información se decidió cada cosa. Lo que vale hoy está en §5
> y en `modelo/dominio.md`.

### Q-01.1 — ¿Por qué elegiría alguien Vex y no otra herramienta?

**Respuesta: E (otra cosa).** A es la más cercana de las cuatro y aun así se queda corta.

> Porque cuando algo falla, sabes con certeza si es tu código o si cambió algo del
> pipeline/ambiente — y eso solo es posible porque los pasos son idénticos entre ambientes.
> Si algo funcionó el martes y hoy falla, sabes en segundos si el problema es tuyo o si alguien
> tocó el pipeline, porque no hay forma de que el pipeline sea distinto entre ambientes sin que
> tú te enteres.

**Análisis.** El valor no es la certeza *previa* (A), ni la trazabilidad *consultable* (C), ni
la velocidad (B): es la **atribución de causa**. Las tres opciones del inventario describen
insumos de esa atribución, no la atribución misma. Eso reordena el mapa entero:

- La frase *«no hay forma de que el pipeline sea distinto entre ambientes sin que tú te
  enteres»* no es una capacidad de Diagnóstico: es una **garantía estructural** que ya está
  escrita en `lenguaje.md` («las instrucciones de despliegue son iguales en todos los
  ambientes; solo las variables de pipeline cambian por ambiente») y pertenece a **Definición
  de Pipeline**. Definición no es el core, pero es la **restricción que lo hace posible**: sin
  esa invariante, la atribución de causa deja de ser exacta y pasa a ser conjetura.
- El **Registro** es la base de evidencia («el martes funcionó»). Sin historial no hay contra
  qué comparar, pero tener historial no es la respuesta — es el material de la respuesta.
- El **sistema de huellas** es el instrumento de medición del cambio. También material.

Por tanto el diferenciador está en la pieza que convierte garantía + evidencia + medición en
una **respuesta atribuida**, y esa pieza es Diagnóstico.

**Qué descarta.** B como diferenciador: no re-ejecutar de más pasa de «la razón por la que
alguien elige Vex» a «una consecuencia agradable del mismo mecanismo que permite atribuir».
D como core: el formato pipelinecode deja de competir por el puesto y queda reclasificado como
la restricción habilitante del core, que es un papel más fuerte que el que tenía.

---

### Q-01.2 — ¿Cuál es el core domain?

**Respuesta: A — Diagnóstico**, con esta definición operativa:

> Diagnóstico es el subdominio que, dado un fallo o un resultado de ejecución, determina y
> comunica la causa: si el origen está en el código del proyecto, en el pipeline (definición o
> variables) o en el ambiente — comparando la ejecución actual contra el último despliegue
> exitoso conocido, usando datos exactos y no inferencia ni intuición humana.

**Análisis.** La respuesta es una deducción de Q-01.1, no una opinión: si el valor es la
atribución de causa, el core es quien atribuye. La definición añade tres cosas que `dominio.md`
no tenía y que son las que permiten construirlo en E6:

1. **Una entrada declarada**: un fallo o un resultado de ejecución. No «el estado del mundo».
2. **Un espacio de respuestas cerrado**: código del proyecto · pipeline (definición o
   variables) · ambiente. Tres categorías, no una narración.
3. **Un punto de referencia declarado**: el último despliegue exitoso conocido. Eso convierte
   a Diagnóstico en un consumidor con requisitos concretos sobre el Registro, que es
   exactamente el efecto lateral que E6 busca.

La objeción del inventario («un core que solo explota a otros dos podría ser una capa de
presentación») queda respondida: Diagnóstico no presenta el Registro, **decide una atribución**
que ninguno de sus dos proveedores puede emitir por sí solo — Registro sabe qué pasó, Definición
sabe qué se declaró, y ninguno de los dos sabe *de quién es la culpa*.

El argumento económico: cero líneas hoy, y es lo único del mapa que un competidor no tiene
resuelto. Las 14.000 líneas existentes están en subdominios que, según Q-01.7, son en buena
parte comprables.

**Qué descarta.** B y D quedan descartadas de forma explícita: el sistema de huellas es
**mecanismo**, no core (ver Q-01.4). C queda descartada: el Registro es fuente de verdad, y ser
imprescindible no es ser diferenciador. Se descarta también el **core doble** de D — que era la
salida de no elegir.

**Qué confirma del plan.** E6 sigue siendo la primera iteración del bloque táctico y el plan no
se reordena. El riesgo declarado en `plan-ddd.md` («si sale vacío, la clasificación de E1
estaba mal y se vuelve a E1») sigue vigente, pero ahora tiene un asidero: el espacio de
respuestas de tres categorías es comprobable contra escenarios reales antes de modelar nada.

---

### Q-01.3 — «Ejecución de pipeline» mezcla decidir y ejecutar. ¿Se parte en dos subdominios?

**Respuesta: B — sigue siendo un solo subdominio.** Y no por dependencia (C), sino con
argumento propio.

> No hay un experto de negocio distinto para cada lado, ni un vocabulario reconocible fuera del
> contexto técnico que los separe: ambas son facetas de la misma capacidad, «ejecutar el
> pipeline de forma inteligente». Un DevOps haciéndolo a mano no las separa mentalmente como
> dos actividades de negocio distintas.
>
> Internamente sí se podría modelar como (al menos) dos bounded contexts dentro de ese
> subdominio: vocabulario sin solapamiento (huella/diff/regla vs. clonar/interpolar/stdout),
> ejes de volatilidad distintos (cambia el algoritmo de fingerprint vs. cambia el tipo de
> executor) y aislamiento práctico en pruebas (tests de dominio sin necesitar ejecución real).

**Análisis.** La respuesta aplica la prueba correcta y la aplica en el nivel correcto. Un
subdominio es una parcela del **espacio del problema** y se justifica con experto y lenguaje de
negocio; un bounded context es una decisión del **espacio de la solución** y se justifica con
vocabulario, volatilidad y aislamiento. Los tres criterios que se citan para partir son
criterios de contexto, no de subdominio — así que partir aquí habría sido cometer en E1 una
decisión que pertenece a E3.

La preocupación del inventario («el core queda dentro de un subdominio supporting») **no se
materializa**, porque Q-01.2 respondió A: el core es Diagnóstico, no la decisión de
re-ejecución. La mitad «decidir» de Ejecución de pipeline es supporting, y que conviva con
«ejecutar» ya no compromete a ningún core. Ese es el motivo por el que la opción C existía y
por el que su premisa se ha desactivado.

Queda nombrada una pieza nueva del espacio de la solución: **Decidir / Planificación del
Intento**, que es donde Q-01.4 aloja la huella granular. Existe como candidato a bounded
context, no como subdominio.

**Qué descarta.** A, y con ella la lectura de que la destilación tenía que *sacar* el core de
donde estaba enterrado — no estaba enterrado ahí. Lo que §A-2 de `vex-plan-revision.md`
denuncia («la misma pieza decide, hashea y escribe») sigue siendo un problema real, pero pasa a
ser un problema **táctico**, resoluble en E9 sin mover ninguna frontera de subdominio.

**Qué difiere.** La partición en dos bounded contexts, con su prueba de separación → **IT-03**.

---

### Q-01.4 — ¿A qué subdominio pertenece el sistema de huellas (`fingerprint`)?

**Respuesta: B, con corrección de fondo.** No es un subdominio propio ni un bounded context
propio, pero tampoco es «un subdominio genérico»: es una **capacidad Generic sin subdominio
propio**, que vive **duplicada a propósito** en dos subdominios ya existentes.

> Suministro del Proyecto calcula el fingerprint del árbol de contenido del proyecto, y
> Suministro del Pipeline calcula el fingerprint del árbol de contenido del pipeline. No hay un
> tercer lugar donde «el fingerprint» resida de forma centralizada — son dos capacidades
> gemelas, cada una dueña de su propio cálculo sobre su propia fuente de contenido.
>
> No confundir con el fingerprint granular: existe un segundo cálculo, más fino (por step,
> combinando el subconjunto de archivos del pipeline que pertenece a ese step + variables del
> step/ambiente), que decide si un step específico se re-ejecuta. Ese fingerprint #2 no vive en
> Suministro — vive en Decidir / Planificación del Intento, dentro de Ejecución de pipeline,
> consumiendo a Definición de Pipeline. Es el mismo algoritmo de hash aplicado con distinto
> alcance, no un subdominio nuevo ni una responsabilidad adicional de Suministro.

**Análisis.** La respuesta separa dos cosas que el paquete `fingerprint` tiene fundidas y que el
inventario preguntaba como una sola: **el algoritmo** (genérico, sin dueño, replicable) y **el
alcance sobre el que se aplica** (que sí tiene dueño, y son tres dueños distintos). Con esa
separación, la opción D del inventario acierta en el reparto —`v1` a Suministro, la huella de
declaración a Definición, la de identidad de step a quien decide la re-ejecución— pero se
equivoca en el verbo: no se «parte» un activo con dueño, porque nunca lo tuvo.

El riesgo declarado en el inventario (shared kernel involuntario) queda **abierto, no
resuelto**: hoy `internal/domain/fingerprint` es un paquete único y el repositorio tiene una
regla explícita — *«una regla, tres raíces, para que nadie introduzca una variante»* — que
existe justamente para impedir la duplicación que esta decisión declara. Ambas cosas pueden ser
ciertas a la vez si lo duplicado es la **propiedad de la capacidad** y no la **implementación
del algoritmo**, pero eso es un patrón de context mapping (shared kernel, published language o
librería), y su elección es materia de E4.

**Una tensión que hay que registrar.** La descripción de la huella granular incluye *«variables
resueltas del step/ambiente»*, y la **spec 27** hashea explícitamente **declaraciones, nunca
valores resueltos** (`pipe-v1`): es lo que hace la huella conocible *antes* de ejecutar y lo que
impide que un valor de runtime salga como digest sin sal. La contradicción es real y toca
directamente a Q-01.6. Se difiere a **IT-08**, nombrada.

**Qué descarta.** A (es el core o su corazón): incoherente con Q-01.2 = A, y ya descartada allí.
C (es parte de Definición): la huella del árbol del proyecto no encaja, como el propio
inventario anticipaba.

---

### Q-01.5 — ¿Qué es Simulación de Pipeline, y no será el mecanismo del Plan automático?

**Respuesta: B, con una precisión que cambia la forma de la respuesta.** No son lo mismo, pero
tampoco son independientes.

> Validación (lo que informalmente se llamaba «Plan») es un subconjunto que Simulación reutiliza
> como su fase inicial. Validación es la responsabilidad ya existente de **Definición de
> Pipeline** —comprobar que la definición y las variables cumplen lo que el motor espera, sin
> ejecutar nada— y no es un subdominio nuevo. Simulación es un subdominio propio que, además de
> correr esa validación, ejecuta un flujo completo en memoria (sin comandos reales, sin
> persistencia) para dar confianza sobre el comportamiento total del pipeline.
>
> Se diferencian por **actor** (programador ejecutando vs. devops diseñando el pipeline),
> **momento** (antes de cada intento vs. antes de publicar el pipeline) y **propósito** (evitar
> fallos evidentes vs. validar el diseño completo).

**Análisis.** El inventario planteaba una disyuntiva binaria (o son lo mismo, o hay que escribir
la diferencia) y la respuesta rompe la disyuntiva: la relación es de **contención**, no de
identidad ni de independencia. Con eso, **«Plan automático» deja de ser un término del modelo**
y se descompone en dos piezas que ya tienen dueño:

| Lo que era «Plan automático» | Dueño después de IT-01 |
|---|---|
| Comprobar que la definición y las variables son válidas, sin ejecutar | **Definición de Pipeline** *(Validación)* |
| Comparar contra el último despliegue exitoso y atribuir causa | **Diagnóstico** *(el core)* |

Que Validación no sea un subdominio nuevo es lo que impide que el mapa crezca por cada
capacidad nombrada. Que Simulación sí lo sea se sostiene por la prueba de experto: el DevOps que
diseña un pipelinecode y quiere probarlo antes de publicarlo es un actor distinto del que
despliega, con un job-to-be-done distinto.

**La subpregunta obligatoria** — ¿«exacto, nunca inferencial» aplica a *qué pasos se ejecutarán*
o también a *qué van a producir*? — queda resuelta por construcción, no por elección: la
tensión que el inventario señalaba nacía de suponer que el Plan automático usaba Simulación.
No la usa. Diagnóstico se alimenta de Validación (estructural, exacta) y del Registro (hechos
ocurridos, exactos); **ningún valor simulado entra jamás en una respuesta de Diagnóstico**. Los
valores de salida simulados existen solo dentro de Simulación, cuyo propósito declarado es dar
*confianza*, no *exactitud*. Así, «exacto, nunca inferencial» aplica sin recortes a todo lo que
el core afirma, porque el core no afirma nada sobre lo que un comando va a producir.

**Qué descarta.** A (son lo mismo): había un solapamiento aparente, no real. C (aparcarla): se
mantiene en el mapa, con actor y momento propios. D (es genérica): el «ejecutar en seco» de
otros motores no valida un diseño de pipelinecode contra las reglas que este motor espera.

---

### Q-01.6 — ¿Falta un subdominio de protección de valores sensibles?

**Respuesta: ninguna de las tres del inventario.** No es A (subdominio propio), no es B
(pertenece al Registro) y no es C (política de infraestructura).

> Se resuelve dentro de **Resolución de Variables** —Supporting, confirmado como subdominio real
> por experto de dominio, existencia sin software y job-to-be-done reconocible fuera de lo
> técnico— en el punto exacto donde se determina el valor efectivo de cada variable, sea
> declarada o extraída por probe desde el output de un comando.

**Y una decisión táctica anticipada desde el modelado estratégico:**

> Cada variable se ofuscará antes de persistirse o mostrarse — no como excepción para casos
> «sensibles» detectados, sino como comportamiento por defecto para toda variable, desde el día
> uno. Esto reemplaza la postura anterior de «riesgo aceptado» (asumir que las variables no son
> sensibles) por una postura de mitigación activa desde el diseño inicial.

**Análisis.** La respuesta corrige la premisa de la pregunta. El inventario buscaba dueño para un
*conjunto de reglas de salida* (digest, sal, redacción, secreto) repartido entre `record`,
`notify` e `infrastructure/record`, y la respuesta señala que ese reparto es un síntoma: las
cuatro reglas se aplican en cuatro sitios porque el valor viaja sin protección hasta llegar a
cada uno de ellos. Poner la ofuscación en el punto donde el valor **nace** —la resolución—
elimina el concern transversal en vez de darle dueño, que es la única forma de evitar la
desventaja que el propio inventario le achacaba a la opción A.

**La restricción sobre el core, y su resolución.** Ofuscar no puede impedir que Diagnóstico
cumpla su función:

> La comparación / detección de cambio (fingerprint, diff) se calcula sobre el valor real en
> memoria, **antes** de descartarlo. Solo la representación que se persiste en Registro o se
> muestra al usuario queda enmascarada. Aunque el valor nunca se vea en texto plano, Diagnóstico
> sigue pudiendo decir «esta variable cambió desde el último despliegue exitoso», porque la
> comparación no depende de leer el valor enmascarado sino de comparar su huella antes de
> ocultarlo.

**Pérdida aceptada y nombrada.** Diagnóstico podrá decir *«`DB_POOL_SIZE` cambió»*, y **no**
*«pasó de 10 a 50»*. Ese segundo mensaje es exactamente el ejemplo que la **spec 11 §5.3** usa
para justificar que `state/` se guarde en claro **a propósito**, y que la tabla de la **spec 20**
repite. Esta decisión contradice a ambas y lo dice: la protección por defecto se antepone a la
riqueza del mensaje diagnóstico. Es una pérdida de *detalle*, no de *función*: el espacio de
respuestas de Q-01.2 (código · pipeline · ambiente) se sigue pudiendo resolver con «cambió» y
sin «de cuánto a cuánto».

**Qué descarta.** B: el Registro decide qué se escribe, pero la redacción del log no pasa por él
y el valor ya habría viajado hasta `notify` para entonces. C: decidir que el `content_id` va sin
sal y el digest con clave por proyecto es una decisión sobre qué información puede salir de la
organización, y eso es negocio.

**Qué difiere.** El diseño concreto de la ofuscación —qué se conserva para comparar, dónde vive
el secreto, qué pasa con `keys/digest-v1.key` y con la sal del `content_id`— a **IT-08**.

---

### Q-01.7 — ¿Los tres subdominios genéricos son genéricos de verdad? ¿Alguno no debería escribirse?

**Respuesta: A para los tres.** Ninguno se elimina del mapa; ninguno se construye desde cero si
la industria ya lo resolvió.

> Ninguno pasa la prueba de experto de dominio distintivo, ni de existencia como práctica de
> negocio propia, ni de job-to-be-done reconocible fuera de contexto técnico **como algo
> específico de Vex**.

**Suministro del Proyecto y Suministro del Pipeline.** Responsabilidad acotada con precisión:
disponibilizar el repo (clonado) y calcular un **fingerprint global** sobre el árbol completo de
esa fuente — una identidad de contenido **gruesa**, no una comparación por partes. Que tengan
actores de negocio distintos (el programador es dueño del proyecto, el DevOps es dueño del
pipeline) confirma por qué están separados **entre sí**, pero no los asciende de Generic:
*«tener mi repo listo y saber si cambió en general»* es higiene básica de control de versiones,
ya resuelta por Git.

| Pieza | Candidato a no escribir |
|---|---|
| Clonado | librerías Git existentes (`go-git` en el ecosistema Go) |
| Fingerprint global | `golang.org/x/mod/sumdb/dirhash`, del propio toolchain de Go — evaluarla antes de mantener las ~380 líneas propias |

Y el criterio que zanja la duda de fondo: **construirlo a mano no lo saca de Generic**. La prueba
de clasificación es si es tu ventaja competitiva —no lo es—, no quién lo programa.

**Sincronización de Estado.** Responsabilidad acotada: persistir físicamente los datos de
ejecución por step y hacerlos disponibles entre máquinas y ubicaciones, para que otro dev en otro
sitio esté sincronizado. **No** incluye saber qué hay en producción o en ambientes previos —eso
es Registro de Despliegue— ni decidir o efectuar el cambio —eso es Orquestación / Ejecución de
Step—. Que en la práctica manual un mismo DevOps haga las tres cosas no las funde: son roles de
negocio distintos con dueño en otros subdominios. Y ya se resuelve hoy con el patrón correcto
para un Generic: **un puerto con dos implementaciones** (archivo local, Supabase remoto) en vez
de un motor de persistencia propio. Para generalizar a más backends sin escribir cada adaptador,
existe `gokv`.

**Análisis.** La respuesta desarma los tres argumentos que el inventario ofrecía a favor de
ascenderlos, y lo hace por el mismo camino en los tres casos: **reasignando la regla, no
negándola.**

| Regla que el inventario invocaba | Dónde queda |
|---|---|
| `ack` idempotente, `seq` monótono, reenvío sin duplicar | mecánica del **puerto**, no del subdominio: es cómo se cumple el contrato «persistir y estar disponible» |
| `sync_failed` como hecho | lo emite el adaptador, pero es un hecho del **Registro** — quien lo modela y lo lee no es Sincronización |
| `clone_window` / `stale_clone_used` como «política de frescura» | `lenguaje.md` ya la califica de *«intervalo de refresco (es una configuración de frecuencia)»*: es un parámetro, no una regla de negocio |

**Qué descarta.** B para los tres (genérico en el mecanismo, supporting en la política): no hay
política que quede huérfana al clasificarlos como genéricos, porque cada una tiene dueño en otro
sitio. C para los tres.

**Qué difiere.** Evaluar `dirhash` frente a la implementación propia, y `gokv` frente al puerto
actual, a **IT-10**. La evaluación puede salir negativa: *seguir escribiéndolo* es una respuesta
válida para un genérico siempre que la razón esté escrita.

---

### Q-01.8 — ¿Espacio de Trabajo es supporting o genérico?

**Respuesta: A — Genérico.**

> Su aislamiento por ambiente —cada ambiente con su propio espacio de trabajo, con continuidad de
> archivos entre ejecuciones— es mecánica de infraestructura, no modelado de negocio.

**Análisis.** Es la única reclasificación de esta iteración: el subdominio baja de Supporting a
Generic. La respuesta introduce además dos cambios de contenido respecto de lo que `lenguaje.md`
y el código dicen hoy, y conviene dejarlos escritos porque no son consecuencias automáticas de
la reclasificación:

| Hoy | Después de IT-01 |
|---|---|
| «copia física mutable del pipeline, **por proyecto**» | por **ambiente** |
| ciclo crear / usar / limpiar, copia por ejecución (handler 05) | **continuidad de archivos entre ejecuciones** |

Al elegir A se adopta también su premisa: **la restauración de plantillas tras un fallo pertenece
a quien ejecuta, no a quien copia.** Las `fileSessions` del `ExecutionContext` y la corrección
del defecto D1 quedan, por tanto, en **Ejecución de Step** — que es donde la regla se puede
enunciar en términos de intento fallido, y no en términos de directorio.

Nota de coherencia con Q-01.7: se aplica el mismo criterio en los dos sitios. Allí el
`clone_window` no ascendía a Suministro porque tenía dueño en otro sitio; aquí la restauración
no sostiene a Espacio de Trabajo como supporting por la misma razón. Lo que queda tras
reasignarla es copiar, aislar y limpiar directorios.

**Qué descarta.** B (supporting). C (absorberlo en Ejecución y borrarlo del modelo): sobrevive
como subdominio genérico porque el aislamiento por ambiente y la continuidad entre ejecuciones
son una responsabilidad nombrable, aunque no sea negocio.

---

### Q-01.9 — ¿Qué va a ser el motor dentro de doce meses?

**Respuesta: A — sigue siendo one-shot, indefinidamente. El proceso muere con el intento.**

**Análisis.** Es la respuesta que **habilita retirar** complejidad, no solo reorganizarla, y es la
única de las cuatro que lo hace: D («no está decidido») habría obligado a diseñar el modelo para
que la decisión no lo invalide, lo que en la práctica significa conservar toda la maquinaria de
concurrencia por si acaso.

Consecuencias inmediatas sobre el mapa:

- **Sincronización de Estado se queda en Generic** — su ascenso a central dependía de un motor
  de larga vida. Coherente con Q-01.7.
- **El mutex de `Execution` y `ErrTransicionIlegal` quedan marcados como retirables.** Con un
  único hilo de ejecución, el mutex no protege nada. `ErrTransicionIlegal` es un caso aparte y
  **no se retira sin más**: hoy sostiene que una cancelación por señal sobreviva al fallo que
  ella misma provoca, y eso no es concurrencia entre intentos sino una carrera entre el manejador
  de señal y la cadena. Q-01.10 lo recoge con la formulación correcta: *retirado **o**
  justificado*, no retirado a la fuerza.
- **La ejecución en paralelo de varios pipelines no entra en el dominio.** Cualquier propuesta
  del bloque B que la asuma se rechaza contra esta decisión.

**Qué descarta.** B y C, y con ellas la cola de intentos, el estado en memoria entre ejecuciones
y la concurrencia como concepto de dominio.

---

### Q-01.10 — ¿Cómo sabremos, dentro de seis meses, que este ejercicio valió la pena?

**Respuesta: F**, que absorbe A, B, C y E del inventario y las organiza en **tres frentes con
evidencia concreta**, no en la sensación de que «quedó más claro».

**Frente 1 — El modelo funcionó si:**

1. Los límites del código no se rompen: no se puede cambiar un contexto sin que otro se entere.
2. Los requisitos nuevos caen dentro de un contexto **sin reabrir el debate de fronteras**.
3. El lenguaje —«intento», «despliegue», «ámbito»— sobrevive hablando con alguien externo.
4. **Diagnóstico recibió más inversión real que los demás.**
5. La complejidad marcada como retirable (mutex + `ErrTransicionIlegal`) de hecho se **retiró o
   se justificó**.
6. Hubo **al menos un error concreto** que la separación evitó antes de llegar a producción.

**Frente 2 — La vara para el Bloque B.** Una propuesta de rediseño solo entra si cumple **al
menos una** de estas cuatro:

| | Criterio |
|---|---|
| **A** | Baja acoplamiento **medible** |
| **B** | Fuerza una regla **por construcción** en vez de por convención |
| **C** | **Simplifica un test existente** |
| **D** | Se puede **revertir tocando un solo contexto** |

Si falla las cuatro, se rechaza con la fórmula estándar, **sin juicio estético**. Y el criterio de
terminación: **E12 se cierra cuando la siguiente propuesta falla las cuatro** — no cuando ya no
se te ocurra nada más.

**Frente 3 — El aprendizaje personal de DDD se valida si:**

1. Puedes defender la frontera de un agregado real (**Despliegue**) nombrando su invariante
   concreta.
2. Puedes nombrar el **patrón de context mapping exacto** (Open Host Service, Anticorruption
   Layer, …) para cada relación real entre motor, CLI y portal.

**Análisis.** Los tres frentes son comprobables y ninguno se puede satisfacer con una opinión.
Dos merecen destacarse porque cambian cómo se cierra el plan:

- **El frente 2 es un filtro de entrada, no un informe de salida.** Convierte el bloque B en un
  proceso con condición de parada explícita. Sin él, «rediseño profundo» no tiene final.
- **El frente 1.6 es el único criterio que no se puede fabricar.** «Un error concreto que la
  separación evitó» solo se puede comprobar mirando hacia atrás, y es el que separa un modelo
  que funciona de un modelo que se lee bien.

Correspondencia con el inventario: **A** → frente 1.4 · **B** → frente 2.A · **C** → frente 2.B ·
**E** → frente 1.3. **D** («el modelo se puede explicar») queda absorbida parcialmente en el
frente 1.3, con un listón más exigente: no basta con que el documento se entienda, tiene que
sobrevivir dicho en voz alta a alguien de fuera.

---


---

### Ronda 2 — Respuestas *(Q-01.11 … Q-01.19)*

> Respondidas el 2026-08-26. Cuatro cierran; tres modifican el modelo; **dos abren ronda 3**.

### Q-01.11 — ¿Qué parte de «saber si algo cambió» es de negocio? · **A**

**Respuesta.** Se adopta el reencuadre. La huella sale del mapa como capacidad; lo que queda en
el modelo es la **regla escrita** —qué cuenta como cambio y que cambiarla obliga a declararlo—,
tratada como contrato versionado. Su dueño se decide en E4.

**Qué cierra.** `DEC-01.4` queda sustituida. Desaparece la categoría fuera de taxonomía y con
ella la contradicción con la regla del repositorio: no se duplica nada, hay tres conceptos
distintos que se calculan con la misma técnica, y la técnica no es dominio.

---

### Q-01.12 — ¿Quién absorbe «cada ambiente trabaja aislado»? · **A**

**Respuesta.** Ejecución de Pipeline, junto con la restauración del material tras un intento
fallido.

**Qué cierra.** Espacio de Trabajo sale del espacio del problema. La expectativa de negocio no se
pierde: pasa a ser **cómo** Ejecución cumple su promesa, y por tanto se defiende dentro de ella.

---

### Q-01.13 — ¿Un «Suministro de Fuentes» o dos? · **A**

**Respuesta.** Uno solo, con dos fuentes declaradas: el producto (del programador) y el pipeline
(del DevOps).

**Qué cierra.** `DEC-01.7` se reduce de tres genéricos a dos. Y `Q-01.14` le acaba de añadir una
promesa que no tenía — ver abajo.

---

### Q-01.14 — Tres términos que sean solo de Diagnóstico · **RESPONDIDA, y el core sobrevive**

**Respuesta.**

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen: en el código o en el pipeline |
| **evidencia** | lo que sostiene la atribución: diferencias en variables, en pasos del pipeline, en el cambio de código |
| **reincidencia** *(nombre propuesto)* | cuántas veces se ha intentado: distingue lo **momentáneo** de lo **perenne**. Una causa de código suele mostrarse en un intento; una del pipeline o del ambiente se repite |
| **responsable** *(candidato)* | quién hizo el cambio — el autor, con su correo, y la fecha del último resultado exitoso |

**Veredicto: pasa.** *Causa*, *evidencia* y *reincidencia* son de Diagnóstico y de nadie más, y
ninguno se puede decir sin pérdida en el lenguaje del Registro. `DEC-01.2` se sostiene: **hay
core y hay lenguaje propio**. El plan no se reordena y E6 tiene de dónde tirar.

**Y abre dos cosas que no estaban en el modelo:**

1. **«Reincidencia» es una lectura, no un hecho.** «Nos puede dar indicios» es exactamente lo que
   `DEC-01.2` prohibió al escribir *«datos exactos, no inferencia»*. O el core admite dos clases
   de salida distintas, o la reincidencia se entrega como hecho y la lectura la hace la persona.
   → `Q-01.22`
2. **«Responsable» amplía el espacio de respuestas.** `DEC-01.2` cerró la respuesta en tres
   categorías; nombrar a un autor añade una segunda dimensión —*qué* y *quién*— y exige una
   promesa que hoy **nadie hace en el mapa**: Suministro de Fuentes promete que el material está
   delante y que sabe si cambió, no quién lo cambió ni cuándo. → `Q-01.23`

Es la primera vez que el mecanismo previsto en `DEC-01.2` funciona de verdad: **lo que el core
necesita se convierte en requisito de otro subdominio**, en vez de adivinarse.

---

### Q-01.15 — La protección de valores · **respuesta con dos partes**

**Parte de proceso.** Las specs antiguas salen del análisis. El modelo se diseña siguiendo el
libro, no reconciliándose con lo que hay: muchas quedarán derogadas, y averiguar cuáles es
trabajo de la migración, no de cada decisión.

> **Enmienda al proceso.** `plan-ddd.md` §6 decía que toda decisión que contradiga una spec la
> nombra. Se cambia: **el modelo se diseña sin las specs como restricción**, y la reconciliación
> —qué queda derogado— es un entregable **único de E11**. Se preserva el motivo original (que
> nada se olvide) sin dejar que lo implementado dirija lo que se modela.

**Parte de modelo.** La ofuscación **no es cifrado**: se ofusca **para guardar**, y por tanto es
responsabilidad de **quien persiste**.

**Análisis — hay que partir en dos lo que `DEC-01.6` tenía junto**, porque una mitad es dominio y
la otra no:

| | Qué promete | Dónde vive |
|---|---|---|
| **Comparar sin exponer** | se sabe si una variable cambió respecto del último resultado exitoso, sin que su valor circule | **Resolución de Variables** — es garantía de negocio, y es de la que depende el core |
| **Ofuscar al guardar** | lo que queda en reposo no es legible de un vistazo | **No es dominio.** Es política de quien persiste, exactamente como dijiste |

Con ese corte, `DEC-01.6` se sostiene en su mitad de dominio y suelta la otra — que es lo que
hacía que pareciera un asunto transversal. → `Q-01.24`

---

### Q-01.16 — Simulación · **subdominio propio, y la contradicción queda resuelta**

**Respuesta.** Es un subdominio propio. Su usuario es el **DevOps**, no el programador. No tiene
efectos: no guarda nada. Y **lo único que se simula es la ejecución de los comandos** — la
interpolación de variables y las validaciones se hacen de verdad.

**Qué cierra.** La contradicción heredada (*«mismo mecanismo»* vs. *«contexto aislado»*) se
resuelve a favor del mecanismo: Simulación **no** depende solo de Definición. Depende también de
Resolución de Variables (interpola de verdad) y de Suministro de Fuentes (necesita el material).
La línea *«contexto aislado: solo depende de Definición de Pipeline»* es falsa y se corrige en
IT-02.

**Qué queda para E3.** Sustituir un solo eslabón —los comandos— y dejar el resto intacto es la
definición de compartir modelo, y el libro trata eso como mal necesario a minimizar. La pregunta
del espacio de la solución sigue abierta: **¿contexto propio, o modo de Ejecución?** Que sea
subdominio propio no la responde, y no hace falta responderla aquí.

**Lo que sigue siendo verdad.** Es el único Supporting del que el core no depende. Entra por la
segunda pata de la regla. Está anotado en `dominio.md` y no se oculta.

---

### Q-01.17 — ¿Hay un tercer actor? · **A, y es el hallazgo mayor de la ronda**

**Respuesta.** Sí: el **dueño del negocio**.

> El programador, siguiendo el pipeline del DevOps, puede dejarlo todo listo en producción. Pero
> **quién decide en qué momento eso llega al cliente final es el dueño del negocio**, y esa
> separación debe existir. Cuando ese actor no está presente, se asume que acepta enviar en
> cuanto esté listo — por eso, sin una definición, al quedar listo en producción se crea un
> lanzamiento automáticamente. **Parece parte del mismo despliegue, pero no lo es.**
>
> Y decide también **el nombre**: la etiqueta de negocio con la que el producto sale. Si no crea
> una, se asume la que la herramienta puso por defecto.

**Análisis.** Esto no añade un actor a un subdominio existente: **parte uno en dos**.

*Desplegar* y *lanzar* venían juntos en Registro de Despliegue, y tu frase los separa
explícitamente. Con la prueba del libro, la separación se sostiene en las tres patas a la vez:

| | Registro de Despliegue | Lanzamiento |
|---|---|---|
| **Experto** | DevOps / programador | **dueño del negocio** |
| **Lenguaje** | intento, despliegue, ambiente, linaje | lanzamiento, *listo para el cliente*, etiqueta de negocio, estrategia |
| **Pregunta** | ¿qué se desplegó, cuándo y con qué? | ¿cuándo sale al cliente, y cómo se llama? |
| **Existe sin software** | como registro de operaciones | como decisión de negocio — **decidir cuándo un producto sale al mercado**, que es un oficio entero |

Y hay una regla de negocio de verdad, no plomería: **el valor por defecto cuando el actor no
está** —se lanza en cuanto está listo— es una decisión sobre quién decide, no una comodidad de
implementación.

**Lo que queda dentro y lo que queda fuera.** A nivel de dominio entra: hay alguien que decide
**cuándo** se entrega y **con qué nombre**, y hay un comportamiento declarado para su ausencia.
Queda fuera por ahora, por decisión explícita: las **estrategias de lanzamiento**, y el detalle
de que existan dos etiquetas —una técnica por defecto y una de negocio— que es forma, no
problema.

**Lo que abre.** Si desplegar y lanzar son cosas distintas, **lo último desplegado con éxito y lo
que el cliente está usando pueden no ser lo mismo**. Y el core compara contra «el último
despliegue exitoso conocido». → `Q-01.21`, y `Q-01.20` para la frontera.

---

### Q-01.18 — La regla de clasificación por garantías · **A**

**Respuesta.** Se adopta y encabeza `dominio.md`.

**Qué cierra.** Clasificar deja de ser un juicio por subdominio. Y lo que la regla deja a la
vista se mantiene a la vista: Simulación —y ahora Lanzamiento— entran por la segunda pata, que
es la débil, y eso es información, no un defecto que tapar.

---

### Q-01.19 — El lenguaje en el código · **A**

**Respuesta.** Se añade al frente 1 de `DEC-01.10`:

> **1.7 —** Un nombre del core se lee en voz alta a un DevOps y significa lo mismo que en
> `lenguaje.md`.

**Qué cierra.** IT-01 ya no se puede dar por buena con un `modelo/` impecable sobre un código que
sigue diciendo otra cosa — que es el riesgo más cercano, con 14.000 líneas ya escritas con otro
vocabulario.


---

### Ronda 3 — Respuestas *(Q-01.20 … Q-01.24)*

> Respondidas el 2026-08-26. Cuatro cierran; una —`Q-01.22`— obliga a afinar el contrato del
> core y abre la ronda 4, que es de **dos** preguntas. La validación converge.

### Q-01.20 — ¿Lanzamiento es subdominio propio? · **A**

**Respuesta.** Sí, subdominio propio.

**Qué cierra.** Registro de Despliegue se queda con la memoria de lo desplegado; Lanzamiento, con
la decisión de entregar. Y `Q-01.21` lo asciende un escalón más: deja de entrar por la pata débil
de la regla y **entra en la cadena de garantías del core**.

---

### Q-01.21 — ¿Contra qué compara el core? · **B**, y con vocabulario nuevo

**Respuesta.** Depende de quién pregunta, porque **en sentido estricto son dos actores**.

Y con esto, dos términos del lenguaje quedan definidos por primera vez:

| | Qué es |
|---|---|
| **Despliegue** *(deploy)* | colocar el código **sucesivamente** en dev → staging → producción. Son pasos técnicos de validación y posicionamiento: cada uno es una **preparación** |
| **Lanzamiento** *(release)* | *«ahora sí: actívalo, anúncialo, hazlo visible».* Una decisión **separada y posterior**, que puede tomarse el mismo minuto del despliegue a producción o semanas después |

**La regla que hay que leer con cuidado.** Llegar al último despliegue —producción— **no dispara
el lanzamiento**. Que hoy, por defecto, un despliegue exitoso en producción cree un lanzamiento
inmediato **no es un efecto del despliegue**: es la herramienta actuando **en nombre del actor
ausente**. La diferencia parece sutil y no lo es — si el automatismo fuera parte del despliegue,
la separación no se podría introducir después sin romper el modelo. Por eso la separación existe
**desde ahora**, aunque las estrategias de lanzamiento estén fuera de alcance.

**Qué cambia en el modelo.** El core tiene **dos puntos de referencia declarados**, y sabe cuál
usar según quién pregunta:

- el **programador** pregunta desde el despliegue → compara contra el último despliegue exitoso
  en ese ambiente;
- el **dueño del negocio** pregunta desde el cliente → compara contra el último **lanzamiento**.

Consecuencia directa: *«el último resultado exitoso»* deja de ser una expresión válida en el
modelo, porque nombra dos cosas. IT-02 tiene que desambiguarla.

**Y consecuencia sobre la clasificación**: Lanzamiento le promete algo al core —*se sabe qué está
vivo ante el cliente, y desde cuándo*— así que entra en la cadena de garantías. Simulación se
queda como el único Supporting que no promete nada al core.

---

### Q-01.22 — ¿Dos clases de salida? · **A**

**Respuesta.** Una sola. **Solo hechos exactos.** El core no opina nunca.

**Qué cierra.** La promesa de `DEC-01.2` queda intacta y sin excepciones. **Reincidencia** se
reformula: no es «un indicio de si algo es momentáneo o perenne», es el **hecho** de cuántas
veces se ha intentado. La lectura de ese hecho la hace la persona, y el core no la escribe. La
palabra *indicio* sale del modelo.

**Y lo que obliga a afinar.** Si el core solo entrega hechos, hay que decir si **nombrar la
causa** es un hecho o una conclusión — porque atribuir es justamente lo que `DEC-01.1` vende. La
respuesta probable es que sea una **deducción**, no una inferencia: dado que los pasos son
idénticos entre ambientes, si funcionó allí y falla aquí y lo único distinto es una variable, la
causa se **deduce**, no se adivina. Pero eso hay que escribirlo, porque es la formulación más
precisa del contrato del core y explica por qué la invariante de Definición es la condición
habilitante. → `Q-01.26`

---

### Q-01.23 — ¿La atribución nombra a un responsable? · **B**, y la promesa se mantiene sin dueño

**Respuesta.** El core atribuye **origen**; la autoría es **evidencia** que acompaña, no parte de
la respuesta. Y la promesa de saber quién cambió qué y cuándo se mantiene **anotada, sin dueño
asignado**, por ahora.

**Qué cierra.** El espacio de respuestas del core no se amplía: sigue siendo *dónde está la
causa*, no *de quién es la culpa*. Y el test de lenguaje **sigue pasando con tres términos
propios** —*causa*, *evidencia*, *reincidencia*— incluso después de mover *responsable* a
evidencia, que era la comprobación importante.

**Qué queda anotado.** La fila sin dueño de la cadena de garantías se queda a la vista. Es
deliberado: una promesa que nadie hace, escrita donde se ve, es más segura que una promesa
repartida entre dos candidatos por comodidad.

---

### Q-01.24 — El corte de la protección de valores · **B**

**Respuesta.** Se acepta el corte, y la mitad de abajo queda **anotada como restricción
declarada**, sin ser subdominio.

**Qué cierra.** *Comparar sin exponer* es promesa de dominio, de Resolución de Variables, y de
ella depende el core. *Ofuscar lo que se guarda* —que no es cifrar— es de quien persiste, sale
del mapa y se escribe junto a la restricción de plataforma, que es el sitio que `dominio.md` ya
tenía para lo que no es subdominio y no se puede perder.


---

### Ronda 4 — Respuestas *(Q-01.25, Q-01.26)* y confirmaciones de la ronda 3

> Respondidas el 2026-08-26. `Q-01.21` = **B** y `Q-01.23` = **B**, como se había leído. Las dos
> respuestas nuevas cierran el contrato del core y abren la **ronda 5**, de dos preguntas: las dos
> son consecuencias directas de haber elegido «deducción», y las dos son las últimas piezas
> estructurales que faltan.

### Q-01.23 — quién necesita la evidencia · **B**, y con el destinatario nombrado

**Respuesta.** El core atribuye **origen**; la autoría es evidencia. Y el dueño de la pregunta es,
principalmente, **el programador**:

> Si algo falla debe saber cómo corregirlo: si pedir ayuda al equipo de DevOps, o modificar él
> mismo su código. Por eso hay que manifestar el hecho y no inferir ni suponer.

**Análisis — esto explica el conjunto de categorías, que hasta ahora estaba sin justificar.** Las
tres respuestas posibles del core no son una taxonomía elegida por gusto: **encaminan una
acción**.

| Si la causa es… | Quién lo arregla |
|---|---|
| el código del proyecto | el **programador**, él mismo |
| el pipeline | el **DevOps** — hay que pedir ayuda |
| las variables del ambiente | el **DevOps** |

Y de ahí sale el argumento más fuerte que ha aparecido para la promesa de exactitud: **una
atribución adivinada hace escalar mal**. No es que quede feo — es que el programador pierde el día
o interrumpe a otro equipo sin motivo. Eso es lo que `Q-01.26` protege.

**Lo que sigue sin decidir.** Quién *promete* la autoría —Suministro de Fuentes o Registro de
Despliegue— no se responde aquí: lo respondido es **quién la necesita**. La fila sigue en la
cadena de garantías sin dueño, y la decisión está diferida a IT-03 (§9, #15).

---

### Q-01.25 — ¿De quién es el orden de los ambientes? · **A**

**Respuesta.** De **Definición de Pipeline**. Declara los ambientes, así que declara también su
orden: la progresión es parte de *cómo se despliega este producto*.

**Qué cierra.** La palabra *sucesivamente* deja de estar en el lenguaje sin dueño. Y Definición
pasa a hacerle al core **dos** promesas en vez de una: que los pasos son idénticos entre
ambientes, y que los ambientes tienen un orden declarado.

**Qué abre.** El orden es lo que hace que una comparación entre ambientes sea significativa —lo
que hay en producción vino de staging, no de cualquier sitio—. Pero `Q-01.21` fijó el punto de
referencia **dentro del mismo ambiente**. Las dos cosas no se contradicen, pero tampoco están
reconciliadas. → `Q-01.27`

---

### Q-01.26 — ¿Nombra la causa o presenta la evidencia? · **A**

**Respuesta.** **Nombra la causa, y eso es un hecho, no una inferencia.**

> Lo importante es manifestar un hecho, no inferir.

**Análisis.** La distinción queda cerrada y es la formulación más precisa del contrato del core:

- **Inferencia** — *«se ha repetido cuatro veces, probablemente sea del ambiente»*. Va de
  probabilidad. **Nunca.**
- **Deducción** — *«el pipeline es idéntico en los dos ambientes; funcionó allí y falla aquí; lo
  único distinto es esta variable; luego la causa está en las variables»*. No va de probabilidad:
  la conclusión es **forzosa**. Es un hecho.

**La consecuencia grande, y hay que verla ahora.** La invariante de Definición —*los pasos son
idénticos en todos los ambientes*— deja de ser una **condición habilitante** y pasa a ser la
**premisa** del razonamiento del core. Debilitarla ya no empeoraría la respuesta: **la
invalidaría**. Es la relación más fuerte del mapa, y por eso `dominio.md` la escribe así.

**Y la consecuencia incómoda.** Si el core solo concluye cuando la conclusión es forzosa, hay que
decir qué hace **cuando no lo es** — porque si no se declara, el primer caso difícil se convierte
en una suposición sin que nadie lo note. → `Q-01.28`


---

### Ronda 5 — Respuestas *(Q-01.27, Q-01.28)*

> Respondidas el 2026-08-26. `Q-01.27` **reformula la pregunta** en vez de elegir opción, y la
> reformulación es mejor que las tres que había. `Q-01.28` desactiva una mitad del problema y deja
> la otra a la vista. Ronda 6: **una** pregunta.

### Q-01.27 — ¿El core compara una cosa o dos? · **ninguna de las tres: compara tres ejes**

**Respuesta.**

> El core compara el **código**, que siempre es uno porque el proyecto tiene identidad propia; las
> **instrucciones del pipeline**, que también son uno; y las **variables** definidas en el
> pipeline, que sí son diferentes por paso en cada ambiente.
>
> «Ayer funcionó, hoy falla» → mismo ambiente: en ese tiempo otro programador pudo cambiar el
> código o el equipo de DevOps el pipeline. Si las instrucciones no han cambiado, quedan **dos**
> motivos: el código o las variables.
>
> «Funcionó en staging y falla en producción» → dos ambientes: sabemos que el código no ha
> cambiado, tampoco las instrucciones, entonces **quedan las variables**.
>
> Sobre la invariante de pasos idénticos: me refiero a las **instrucciones** del pipeline, que
> están agrupadas por paso **pero no por ambiente**. Lo que sí varía son las variables, declaradas
> por paso y por ambiente. Si las instrucciones cambian, afectan a **todos los despliegues en
> todos los ambientes**.

**Análisis — la pregunta estaba mal planteada, y así es como estaba mal.** Yo la formulé como una
elección entre dos *escenas* con dos puntos de referencia distintos. No hay dos escenas: hay **un
solo procedimiento —eliminación sobre tres ejes— con distinto punto de partida**.

| Eje | Cuántos hay | ¿Varía por ambiente? | Si cambia, ¿a qué afecta? |
|---|---|---|---|
| **Código del producto** | uno — el proyecto tiene identidad propia | no | a lo que se despliegue desde ahí |
| **Instrucciones del pipeline** | uno, agrupadas por paso | **no** | a **todos** los despliegues de **todos** los ambientes |
| **Variables** | muchas: por paso **y** por ambiente | **sí** | a **un** ambiente, y a un paso |

Y las dos escenas son el mismo procedimiento resuelto con distinta información de partida:

| Escena | Qué fija la situación por sí sola | Qué queda |
|---|---|---|
| mismo ambiente, dos momentos | nada: en ese tiempo pudieron cambiar el código **y** el pipeline | si las instrucciones no cambiaron, **dos** candidatos |
| dos ambientes | el código es el mismo por identidad; las instrucciones, porque no varían por ambiente | **uno**: las variables |

**Por qué comparar entre ambientes es lo potente.** Elimina **dos ejes de golpe**, así que la
deducción cae en un solo candidato. Es literalmente de dónde sale el «en segundos» de `DEC-01.1`.

**Y esto conecta algo que llevaba suelto desde el principio.** `lenguaje.md` ya decía que *«un
error en las instrucciones afecta a todos los ambientes; uno en variables afecta a un ambiente y a
un solo paso»*. Estaba escrito como una **observación**. Ahora se ve que es el **mecanismo de la
deducción**: es esa asimetría —un eje que no varía por ambiente y otro que sí— la que permite
eliminar. No era una curiosidad del formato; era el motor del core.

**Precisión sobre la premisa.** La invariante no dice «los pasos son idénticos»: dice que **las
instrucciones** del pipeline no varían por ambiente. Las variables sí, y ésa es justamente la
razón de ser de la premisa — si las instrucciones también variaran, no quedaría ningún eje fijo
que eliminar y no habría deducción posible.

**Una consecuencia sobre Registro.** El DevOps puede cambiar el pipeline **mientras hay
ejecuciones en curso**. La comparación no es contra «el pipeline de hoy», sino contra **el que se
usó en aquel despliegue** — que es exactamente por qué la promesa de Registro está redactada como
*«se sabe con qué se hizo»* y no como *«se sabe qué pipeline hay»*.

---

### Q-01.28 — ¿Qué dice el core cuando no puede deducir? · **la mitad del problema no existe**

**Respuesta.**

> El core siempre va a poder saber qué cambió: si el código, si las variables o si las
> instrucciones. Para eso están las huellas y el registro de lo que sucede en cada despliegue y en
> cada intento.

**Análisis.** Correcto, y elimina la rama que más preocupaba: **nunca falta evidencia**. Saber qué
cambió es siempre un hecho disponible, y por tanto el core nunca se queda mudo ni tiene que
suponer para poder hablar.

**Pero queda la otra rama, y está en tu propia respuesta a `Q-01.27`:** *«si las instrucciones no
han cambiado, nos quedan dos posibles motivos, el código o las variables»*. Ahí el core **sabe
perfectamente qué cambió** —los dos ejes— y aun así **no puede reducirlo a una sola causa**.

Saber *qué cambió* y saber *cuál de los cambios causó el fallo* no son lo mismo, y la primera no
siempre da la segunda.

Lo que hay que declarar no es entonces «qué hace cuando le falta evidencia» —nunca le falta— sino
**qué entrega cuando la eliminación deja más de un candidato**. Y `DEC-01.2` hoy dice que el core
*«determina y comunica la causa»*, en singular, dentro de tres categorías. → `Q-01.29`


---

### Ronda 6 — Respuesta *(Q-01.29)*

> Respondida el 2026-08-26. Cierra el contrato del core. Ronda 7: **una** pregunta, y no sale del
> core — sale de aplicar la regla de clasificación a un caso que el modelo de tres ejes acaba de
> volver load-bearing.

### Q-01.29 — Cuando quedan dos candidatos · **A**

**Respuesta.**

> Si hay dos candidatos se muestran **siempre los dos**. Si las instrucciones no cambiaron pero
> las variables y el código sí, eso es lo que se indica. Será el **programador** quien decida cuál
> es más probable. A nivel de herramienta, **siempre muestro todo**.

**Análisis.** Cierra el contrato del core y lo hace consistente con las dos respuestas anteriores,
que resultan ser **la misma regla dicha tres veces**:

| Dónde apareció | La regla |
|---|---|
| `Q-01.22` — reincidencia | el core da el **hecho** (cuántos intentos); la lectura la hace la persona |
| `Q-01.26` — la causa | el core **deduce**, nunca infiere; lo forzoso lo dice, lo probable no |
| `Q-01.29` — dos candidatos | el core **muestra los dos**; cuál es más probable lo elige la persona |

Enunciada una sola vez: **la herramienta muestra hechos, la persona juzga probabilidades.** Es el
contrato del core entero, y explica por qué «exacto, nunca inferencial» no era una aspiración sino
una división del trabajo.

**Qué cambia en `DEC-01.2`.** El espacio de respuestas deja de ser *«una de tres categorías»* y
pasa a ser **el subconjunto de ejes que sobrevive a la eliminación**. Con un superviviente se lee
como atribución; con dos, como eliminación parcial — y sigue siendo un hecho exacto en ambos
casos.

**Qué precisa `DEC-01.1`, sin reescribirla.** La promesa tiene dos mitades y solo una se cumple
siempre:

- *«¿tocó alguien el pipeline?»* — **siempre** tiene respuesta exacta. Las instrucciones son un
  solo eje, con una sola identidad, que no varía por ambiente: o cambiaron o no.
- *«¿es mío o del ambiente?»* — se resuelve cuando la eliminación deja un candidato. Cuando deja
  dos, se muestran los dos.

La mitad que siempre se cumple es, además, la que más vale: es la que evita interrumpir al equipo
de DevOps sin motivo, o buscar durante horas en el propio código algo que estaba en el pipeline.

**Y una precisión que conviene dejar escrita**, porque «siempre muestro todo» y «el valor no
circula» se pueden leer como contradictorios y no lo son: **todo** es todo lo que se sabe —qué
ejes cambiaron, qué variables cambiaron, quién y cuándo—, **no** el valor de las variables. Se
muestra que `DB_POOL_SIZE` cambió, no de cuánto a cuánto.


---

### Ronda 7 — Respuesta *(Q-01.30)*

> Respondida el 2026-08-26. **Confirma sin cambiar nada**, que es la señal de que la validación
> terminó: la séptima ronda no encontró un problema, encontró una comprobación que pasa.

### Q-01.30 — ¿De quién consume el core la identidad? · **A**

**Respuesta.** De **Registro de Despliegue**.

> Sigue siendo Generic, porque incluso puedo usar el propio repositorio para saber si algo cambió.
> El proyecto es un repositorio y el pipeline también: puedo mirar sus diferencias y saber si
> cambiaron, sin tener que calcular nada propio.

**Análisis.** Cierra el caso y de paso confirma dos decisiones anteriores por un camino distinto:

- **`DEC-01.4`** —el algoritmo de huella no es dominio— queda confirmada por sustituibilidad: si
  mirar las diferencias de un repositorio responde lo mismo, el mecanismo era técnica, no dominio.
- **`DEC-01.7`** —Suministro de Fuentes es Generic— queda confirmada por la prueba directa del
  libro: existe la herramienta que lo hace.

**Y aparece la comprobación más limpia del mapa entero.** De los tres ejes de `DEC-01.12`:

| Eje | ¿Lo responde algo que se compra? |
|---|---|
| Código del producto | **sí** — es un repositorio |
| Instrucciones del pipeline | **sí** — es un repositorio |
| Variables | **no** — su valor efectivo también nace en ejecución, y eso no lo resuelve nada de fuera |

**Ésa es exactamente la línea entre los dos genéricos y Resolución de Variables**, y no se
dibujó: salió de aplicar la regla. Cuando la clasificación y el mecanismo coinciden sin que nadie
los haya forzado a coincidir, el modelo está diciendo algo verdadero sobre el dominio.

**Lo que no cambia.** El core no habla con la fuente: lee el registro de lo que se usó. Es lo
único compatible con que el DevOps pueda cambiar el pipeline mientras hay ejecuciones en curso —
comparar contra la fuente de hoy daría una respuesta exacta sobre otra cosa.

**Sobre la tercera pata.** La opción C no se eligió, pero **el razonamiento de la respuesta es
C**: se resuelve preguntando si se puede comprar, y esa respuesta decide por encima de cuánto se
apoye el core. La regla queda escrita **ordenada** en `DEC-01.14`.

## 5. Decisiones

> Catorce decisiones firmes, tras seis rondas de validación y treinta preguntas. Se conservan los
> números originales aunque el contenido haya cambiado, para que las referencias de §3.bis y §4
> sigan siendo legibles; `DEC-01.11`–`DEC-01.14` nacieron en la validación. **`DEC-01.8` está
> retirada** y su hueco se deja a propósito.
>
> Sin contabilidad de specs derogadas: por `Q-01.15`, esa cuenta es entregable único de E11.

---

### DEC-01.1 — La ventaja competitiva es la atribución de causa, y tiene dos mitades

**Contexto.** El core se define por valor de negocio, no por complejidad. Ninguna de las cuatro
hipótesis de partida (certeza previa, velocidad, trazabilidad, formato) era el diferenciador:
las cuatro eran insumos suyos.

**Decisión.** Alguien elige Vex para saber, ante un fallo, **si la causa está en su código o en
el pipeline/ambiente**, con certeza y en segundos. La promesa tiene dos mitades y solo una se
cumple siempre:

| | Se cumple |
|---|---|
| *«¿tocó alguien el pipeline?»* | **siempre**. Las instrucciones son un eje con una identidad que no varía por ambiente: cambiaron o no |
| *«¿es mío o del ambiente?»* | cuando la eliminación deja un candidato; si deja dos, se muestran los dos |

**Consecuencias.** La mitad que siempre se cumple es la que más vale: evita interrumpir al DevOps
sin motivo y evita buscar horas en el propio código algo que no estaba ahí. Definición de
Pipeline queda declarada **premisa** del core, no característica del formato (ver `DEC-01.12`).

**Alternativas descartadas.** Las cuatro del inventario sobreviven como insumos, ninguna como
diferenciador.

**Verificación.** Un usuario externo, preguntado por qué usaría Vex, reproduce esta frase sin
ayuda.

---

### DEC-01.2 — El core es Diagnóstico, con lenguaje propio y respuesta por eliminación

**Contexto.** Ya se declaraba core, sin definición operativa, sin lenguaje y con cero líneas —lo
que hacía la clasificación indefendible.

**Decisión.** Diagnóstico es el core. Dado un fallo o un resultado, determina y comunica la causa
comparando contra un despliegue exitoso conocido, con datos exactos.

**Su lenguaje propio** —lo que lo hace un ámbito y no una consulta sobre el de otro:

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen |
| **evidencia** | lo que la sostiene: qué cambió, quién lo cambió y cuándo |
| **reincidencia** | cuántas veces se ha intentado. Un hecho, no una lectura |

**Su espacio de respuestas** no es «una de tres categorías»: es **el subconjunto de ejes que
sobrevive a la eliminación**. Con un superviviente se lee como atribución; con dos, como
eliminación parcial. Ambas son hechos exactos y ambas encaminan.

**Sus dos puntos de referencia**, porque tiene dos clientes:

| Pregunta | Compara contra |
|---|---|
| el **programador**, desde el despliegue | el último despliegue exitoso en ese ambiente |
| el **dueño del negocio**, desde el cliente | el último **lanzamiento** |

**Consecuencias.** E6 sigue siendo la primera iteración táctica, en terreno limpio. Lo que
Diagnóstico necesita se convierte en requisito de otros — y ya ocurrió una vez: la autoría del
cambio es una promesa que hoy nadie hace y que está anotada sin dueño.

**Alternativas descartadas.** El sistema de huellas y el Registro como core: son mecanismo y
evidencia. El core doble: era la forma de no elegir.

**Verificación.** Al cerrar E6, cada escenario termina en un subconjunto de los tres ejes, y para
ninguno hace falta un cuarto.

---

### DEC-01.3 — «Ejecución de pipeline» es un solo subdominio

**Contexto.** Funde decidir y ejecutar, y el análisis de código lo señala como problema.

**Decisión.** Un solo subdominio: no hay experto de negocio distinto por mitad ni vocabulario de
negocio que las separe. Son dos facetas de *ejecutar el pipeline de forma inteligente*.

**Consecuencias.** «La misma pieza decide, hashea y escribe» pasa de problema estratégico a
**táctico**, resoluble en E9 sin mover fronteras. Queda nombrado *Decidir / Planificación del
Intento* como candidato a contexto interno.

**Alternativas descartadas.** Partirlo en dos: sus argumentos —vocabulario, volatilidad,
aislamiento en pruebas— son criterios de **contexto**, y aplicarlos aquí habría adelantado a E1
una decisión de E3.

**Verificación.** IT-03 llega a la partición interna sin reabrir la lista de subdominios.

---

### DEC-01.4 — El sistema de huellas no es dominio; la regla escrita sí

**Contexto.** 975 líneas, tres reglas versionadas, tres consumidores y ningún dueño. La primera
versión de esta decisión inventó una categoría fuera de la taxonomía del libro; la validación la
retiró.

**Decisión.** Se separan tres cosas que estaban fundidas:

| | Qué es |
|---|---|
| El **algoritmo** | técnica compartida. **No entra en el mapa de subdominios** |
| La **regla escrita y versionada** —qué cuenta como cambio, y que cambiarla obliga a declararlo— | lo único con carga de negocio: si cambia sin avisar, todo lo comparado antes deja de ser comparable |
| Las **identidades** que produce | conceptos de quien las usa: la del código y la del pipeline en Suministro de Fuentes; la del paso, en Ejecución |

**Consecuencias.** Desaparece la contradicción con la regla del repositorio: no se duplica nada,
hay conceptos distintos calculados con la misma técnica. Y el mecanismo es sustituible — el
proyecto y el pipeline son repositorios, y mirar sus diferencias responde lo mismo.

**Alternativas descartadas.** Que sea el core; que pertenezca a Definición de Pipeline; y la
categoría inventada de la primera ronda.

**Verificación.** E4 asigna patrón de relación a la regla escrita. E8 decide si el paquete único
sobrevive.

---

### DEC-01.5 — Validación es de Definición; Simulación sobrevive; «Plan automático» se retira

**Contexto.** «Plan automático» y «Simulación» se solapaban, y la exigencia de exactitud chocaba
con generar valores simulados.

**Decisión.**

1. **Validación** —comprobar definición y variables sin ejecutar— es de **Definición de
   Pipeline**. No es subdominio nuevo.
2. **Simulación** es subdominio propio: recorre el flujo entero sin efectos y sin guardar nada, y
   **lo único que finge es la ejecución de los comandos** — la interpolación y las validaciones se
   hacen de verdad.
3. Se distinguen por actor (DevOps que diseña vs. programador que despliega), momento (antes de
   publicar vs. antes de cada intento) y propósito.
4. **«Plan automático» se retira del lenguaje.**

**Consecuencias.** La tensión «exacto vs. simulado» se disuelve: Diagnóstico no consume
Simulación, luego ningún valor simulado entra en una afirmación del core. Y Simulación **no**
depende solo de Definición: también de Resolución de Variables y de Suministro.

**Alternativas descartadas.** Que sean lo mismo; aparcar Simulación; que sea genérica.

**Verificación.** Tras IT-02, `lenguaje.md` no contiene «Plan automático» y ninguna capacidad
queda huérfana.

---

### DEC-01.6 — La protección de valores se parte en dos, y solo una mitad es dominio

**Contexto.** Las reglas de protección no tenían dueño y estaban repartidas. La primera versión
las puso todas en Resolución de Variables; la validación mostró que había dos cosas distintas.

**Decisión.**

| | Promete | Dónde |
|---|---|---|
| **Comparar sin exponer** | se sabe si una variable cambió, sin que el valor circule | **Resolución de Variables** — dominio, y el core depende de ello |
| **Ofuscar al guardar** | lo que queda en reposo no es legible de un vistazo | **fuera del modelo** — no es cifrar, y es de quien persiste |

**Consecuencias.** La mitad de dominio sostiene una categoría entera de la respuesta del core; la
otra deja de arrastrar al mapa una decisión que no es de negocio, y queda escrita como restricción
declarada. Pérdida aceptada: el core dirá *«`DB_POOL_SIZE` cambió»* y no *«pasó de 10 a 50»*.

**Alternativas descartadas.** Un subdominio propio de protección (habría creado un contexto que
todos consultan); que pertenezca al Registro; que sea infraestructura entera.

**Verificación.** Ningún consumidor obtiene el valor en claro salvo el ejecutor del comando, y
Diagnóstico responde sus escenarios de E6 sin leerlo.

---

### DEC-01.7 — Los dos genéricos son genéricos, y ninguno se escribe desde cero

**Contexto.** Había tres genéricos declarados y argumentos para ascender los tres.

**Decisión.** Son **dos**: **Suministro de Fuentes** —unificado, con dos fuentes: el producto, del
programador, y el pipeline, del DevOps— y **Sincronización de Estado**. Ninguno sale del mapa; en
ninguno se invierte presupuesto de diseño.

Candidatos declarados a no escribir: librerías de repositorio para el clonado y las diferencias,
una librería de hash de árbol para la identidad, y una abstracción de almacén para los backends de
estado.

**Consecuencias.** Las reglas que parecían ascenderlos tienen dueño en otro sitio: la idempotencia
y el acuse son mecánica del puerto; el fallo de sincronización es un hecho del **Registro**; la
ventana de reutilización del clon es una configuración de frecuencia. Y las fronteras quedan
fijadas: Sincronización **no** sabe qué hay en producción ni decide ni efectúa nada.

**Alternativas descartadas.** Mantenerlos como tres; partirlos en mecanismo genérico y política
supporting.

**Verificación.** IT-10 escribe, por cada uno, o la sustitución o la razón de seguir
manteniéndolo.

---

### DEC-01.8 — *(retirada)*

Clasificaba Espacio de Trabajo como Generic. La validación mostró que su propia razón —*«es
mecánica de infraestructura, no modelado de negocio»*— lo saca del espacio del problema entero: un
Generic sigue siendo capacidad de negocio, y de ésta no hay proveedor.

**Espacio de Trabajo sale del mapa.** El aislamiento por ambiente y la restauración del material
tras un intento fallido son **cómo** Ejecución de Pipeline cumple su promesa. El hueco de
numeración se deja a propósito.

---

### DEC-01.9 — El motor es one-shot, indefinidamente

**Contexto.** La respuesta cambia la clasificación de subdominios, no solo la arquitectura.

**Decisión.** Lee la petición, ejecuta un pipeline, escribe y sale. Sin servicio de larga vida,
sin cola, sin estado en memoria entre ejecuciones, sin varios pipelines en el mismo proceso.

**Consecuencias.** Sincronización de Estado se queda en Generic. **La concurrencia no entra en el
dominio**: el mutex de la ejecución queda marcado como retirable, y el error de transición ilegal
como retirable **o justificado** por la carrera entre la señal y la cadena. Toda propuesta que
asuma ejecución paralela se rechaza contra esta decisión.

**Alternativas descartadas.** Varios pipelines en un proceso; servicio de larga vida; y «no está
decidido», que habría obligado a conservar la maquinaria de concurrencia por si acaso.

**Verificación.** Criterio 1.5 del frente 1 de `DEC-01.10`.

---

### DEC-01.10 — Criterios de éxito y vara de aceptación del Bloque B

**Contexto.** Un rediseño sin criterio se juzga por gusto estético, y el gusto siempre prefiere el
código nuevo.

**Decisión — tres frentes.**

**Frente 1, el modelo funcionó si:** los límites no se rompen sin que otro contexto se entere ·
los requisitos nuevos caen sin reabrir el debate de fronteras · el lenguaje sobrevive hablado con
alguien externo · **Diagnóstico recibió más inversión que los demás** · la complejidad marcada
como retirable se retiró o se justificó · hubo **al menos un error concreto** que la separación
evitó antes de producción · **un nombre del core se lee en voz alta a un DevOps y significa lo
mismo que en `lenguaje.md`**.

**Frente 2, la vara del Bloque B.** Una propuesta entra si cumple **al menos uno**: baja
acoplamiento medible · fuerza una regla por construcción en vez de por convención · simplifica un
test existente · se puede revertir tocando un solo contexto. Si falla los cuatro se rechaza **sin
juicio estético**. **E12 cierra cuando la siguiente propuesta falla los cuatro.**

**Frente 3, el aprendizaje.** Defender la invariante concreta de un agregado real, y nombrar el
patrón de context mapping exacto de cada relación entre motor, CLI y portal.

**Consecuencias.** El bloque B adquiere condición de parada explícita. El frente 1.7 impide dar
IT-01 por buena con un `modelo/` impecable sobre un código que dice otra cosa, que es el riesgo
más cercano. El frente 1.6 es el único que no se puede fabricar.

**Verificación.** Se cita el frente 2 al abrir cada iteración del bloque B; se recorren los tres
al cerrar el plan.

---

### DEC-01.11 — El contrato del core: la herramienta muestra hechos, la persona juzga

**Contexto.** Tres preguntas distintas de la validación —la reincidencia, la naturaleza de la
causa, y qué hacer con dos candidatos— resultaron ser la misma pregunta.

**Decisión.**

> **La herramienta muestra hechos. La persona juzga probabilidades.**

Aplicada: la reincidencia se entrega como número, no como lectura · la causa se **deduce** y nunca
se infiere · cuando quedan dos candidatos **se muestran los dos**, y cuál es más probable lo elige
el programador.

**La distinción que lo sostiene:**

- **Inferencia** — *«se repitió cuatro veces, probablemente sea del ambiente»*. Va de
  probabilidad. **Nunca.**
- **Deducción** — *«las instrucciones son idénticas; funcionó allí y falla aquí; lo único distinto
  es esta variable; luego la causa está en las variables»*. Es **forzosa**. Es un hecho.

**Consecuencias.** «Exacto, nunca inferencial» deja de ser una aspiración y pasa a ser una
**división del trabajo**. Y *«mostrar todo»* significa todo lo que se sabe —qué ejes cambiaron,
qué variables, quién y cuándo—, **no** el valor: compatible con `DEC-01.6`.

**Alternativas descartadas.** Dos clases de salida marcadas (hecho y lectura); que el core calle
cuando no puede reducir a un candidato; inventar una categoría «mixto» para que siempre haya una
respuesta única.

**Verificación.** Ningún escenario de E6 produce una salida con la palabra «probablemente».

---

### DEC-01.12 — El mecanismo: tres ejes, eliminación, y una premisa asimétrica

**Contexto.** El modelo hablaba de «comparar contra el último despliegue exitoso» sin decir
comparar **qué**.

**Decisión.** El core no compara ejecuciones: compara **tres ejes** y atribuye por **eliminación**.

| Eje | Cuántos | ¿Varía por ambiente? | Si cambia, afecta a |
|---|---|---|---|
| Código del producto | uno — el proyecto tiene identidad propia | no | lo que se despliegue desde ahí |
| Instrucciones del pipeline | uno, agrupadas por paso | **no** | **todos** los despliegues de **todos** los ambientes |
| Variables | muchas: por paso **y** por ambiente | **sí** | **un** ambiente, y un paso |

**La premisa** no es «los pasos son idénticos»: es que **las instrucciones no varían por ambiente
y las variables sí**. Esa asimetría **es** el mecanismo — sin un eje fijo que eliminar no hay
deducción posible. Por eso Definición de Pipeline no es una condición externa del core: es la
premisa de su razonamiento, y debilitarla no empeora la respuesta, la **invalida**.

**Consecuencias.** Las dos escenas de diagnóstico son el mismo procedimiento con distinto punto de
partida: entre ambientes se eliminan dos ejes de golpe y queda uno —de ahí el «en segundos»—;
dentro del mismo ambiente, si las instrucciones no cambiaron, quedan dos y se muestran los dos.
El core **nunca habla con la fuente**: lee el registro de lo que se usó, que es lo único
compatible con que el pipeline pueda cambiar durante las ejecuciones.

**Consecuencia sobre el mapa.** Dos ejes los responde algo que se compra —el proyecto y el
pipeline son repositorios— y el tercero no, porque el valor efectivo de una variable también nace
en ejecución. **Ésa es la línea entre los dos genéricos y Resolución de Variables.**

**Verificación.** Todo escenario de E6 se resuelve nombrando qué ejes cambiaron y cuáles no.

---

### DEC-01.13 — Desplegar y lanzar son actos distintos: Lanzamiento es subdominio propio

**Contexto.** *Despliegue* y *lanzamiento* venían juntos en Registro, y el actor del segundo no
estaba nombrado.

**Decisión.** Son dos actos y dos subdominios.

- **Desplegar** es colocar el código **sucesivamente** en dev → staging → producción: pasos de
  validación y posicionamiento, cada uno una **preparación**. Ese orden lo declara **Definición de
  Pipeline**.
- **Lanzar** es *«ahora sí: actívalo, anúncialo, hazlo visible»*. Decisión **separada y
  posterior**, del **dueño del negocio**, que decide el momento y el nombre.
- **Llegar al último despliegue no dispara el lanzamiento.** Que hoy, sin nadie que decida, lo que
  queda listo se lance solo es **actuar en nombre del actor ausente**, nunca un efecto del
  despliegue.

**Consecuencias.** Aparece el tercer actor del dominio. Lanzamiento entra en la cadena de
garantías: le promete al core saber qué está vivo ante el cliente, que es el segundo punto de
referencia de `DEC-01.2`. Y la separación existe **desde ahora** aunque las estrategias de
lanzamiento estén fuera de alcance: si el automatismo fuera parte del despliegue, no se podría
introducir después sin romper el modelo.

**Alternativas descartadas.** Que siga dentro de Registro con el dueño del negocio como segundo
cliente; que lanzar sea un tipo de anotación sobre un despliegue.

**Verificación.** IT-03 traza la frontera sin que ningún término de Lanzamiento aparezca en el
lenguaje de Registro.

---

### DEC-01.14 — La regla de clasificación es ordenada, no disyuntiva

**Contexto.** La regla se adoptó con sus tres patas en paralelo, y el primer caso difícil
—Suministro de Fuentes, al volverse load-bearing con `DEC-01.12`— daba **dos respuestas a la
vez**: Supporting porque el core depende de ello, Generic porque se puede comprar.

**Decisión.** Las preguntas están **ordenadas** y la primera que responde «sí» decide:

1. **¿Es la respuesta por la que te eligen?** → **Core**.
2. **¿Se puede comprar?** → **Generic**, por mucho que el core se apoye en ello.
3. **¿El core depende de una garantía suya, o es capacidad propia que nadie vende?** →
   **Supporting**.

**Consecuencias.** Que algo sea imprescindible no lo hace diferenciador: **un genérico puede ser
crítico**, y el libro insiste en ello. Clasificar deja de ser un juicio y un subdominio nuevo se
clasifica solo. Y la regla deja a la vista el caso frágil: **Simulación** es el único Supporting
que entra por la tercera pata sin que el core dependa de él.

**Verificación.** Ningún subdominio del mapa admite dos clasificaciones aplicando la regla en
orden.

## 6. Impacto en el modelo

| Documento | Qué cambia | Cuándo |
|---|---|---|
| `modelo/dominio.md` | **Reescrito como espacio del problema.** Nueve subdominios con actor, propósito, razón y coste; la cadena de garantías; la regla ordenada; el mecanismo de tres ejes; y las restricciones declaradas | **IT-01 — hecho** |
| `modelo/lenguaje.md` | Reorganizarlo **por contexto**, arrancando del lenguaje del core (`DEC-01.2`); retirar «Plan automático»; desambiguar «el último resultado exitoso», que ahora nombra dos cosas; escribir *desplegar* vs. *lanzar*, y *deducción* vs. *inferencia* | **IT-02** |
| `modelo/bounded-contexts.md` | Realinear con los nueve subdominios; *Decidir / Planificación del Intento* como candidato; decidir si Simulación es contexto propio o modo de Ejecución; asignar dueño a la promesa de autoría | **IT-03** |

**Lo que `dominio.md` dice hoy y no decía al abrir la iteración:**

1. **Nueve subdominios, no diez.** Espacio de Trabajo salió del espacio del problema; los dos
   Suministro se unificaron; Lanzamiento entró al separarlo de Registro.
2. **Los actores están en el documento** — programador, DevOps, dueño del negocio — y cada
   subdominio dice a quién sirve. Un subdominio se justifica por experto, y los expertos no
   estaban.
3. **La relación entre subdominios es una cadena de garantías**, no un flujo de datos: cada
   Supporting existe porque el core no puede responder sin algo que solo él promete, y cada fila
   dice qué se rompe si esa promesa cae.
4. **La clasificación es deducible** de una regla ordenada, probada contra el caso que la habría
   roto.
5. **El core tiene lenguaje propio y mecanismo escrito**: tres ejes, eliminación, y una premisa
   asimétrica de la que depende que haya respuesta.
6. **No queda nada técnico dentro.** Paquetes, ficheros y specs viven solo en §7.

## 7. Impacto en el código

Esta iteración **no toca código**. Lo que queda anotado para las que sí lo tocarán:

| Paquete / ruta | Qué queda afectado | Por | Se resuelve en |
|---|---|---|---|
| `internal/domain/diagnostic/` *(no existe)* | Nace con el core: tres ejes, eliminación, y una salida que puede traer más de un candidato | `DEC-01.2` · `DEC-01.11` · `DEC-01.12` | E6 |
| `internal/domain/fingerprint/` | Deja de ser «paquete sin dueño»: el algoritmo es técnica, la regla escrita es contrato, las identidades pertenecen a quien las usa. Sustituible por las diferencias del propio repositorio | `DEC-01.4` | E4 · E8 |
| `internal/domain/command/` (`Variable`, `Origin`, `ExecutionVariableMap`) | La protección se aplica donde nace el valor efectivo; ofuscar al guardar sale del dominio | `DEC-01.6` | E8 |
| `record.Facts`, `notify.RedactingObserver`, `infrastructure/record` | Dejan de ser los cuatro puntos donde se protege el valor | `DEC-01.6` | E8 |
| `deployment` / `record` | Hay que separar **despliegue** de **lanzamiento**: hoy no existe el segundo como concepto | `DEC-01.13` | E7 |
| `pipeline` (environments) | El **orden** de los ambientes es declarado, no una costumbre | `DEC-01.13` | E8 |
| `pipeline` handler 05 (`CopyWorkdirHandler`), repositorio de workdir | Espacio de Trabajo deja de ser subdominio: pasa a ser cómo Ejecución cumple lo suyo, por ambiente | `DEC-01.8` *(retirada)* | E9 · E10 |
| `ExecutionContext.fileSessions` | La restauración del material tras fallo se reubica en Ejecución | `DEC-01.8` *(retirada)* | E9 |
| `Execution` (mutex) | Retirable: no protege nada con un único hilo | `DEC-01.9` | E9 |
| `ErrTransicionIlegal` | Retirable **o** justificado por la carrera señal/cadena | `DEC-01.9` | E9 |
| `pipeline` (clonadores), `sync`, `syncconfig` | Candidatos a sustitución por librerías de terceros | `DEC-01.7` | E10 |
| `03_step_runner_handler.go` | «Decide, hashea y escribe» sigue siendo problema, pero táctico | `DEC-01.3` | E9 |

## 8. Criterio de cierre

- [x] Las **30** preguntas están respondidas o diferidas con razón escrita.
- [x] Hay exactamente un core domain nombrado y defendido con argumento económico — y ha **pasado
      el test de lenguaje** (`Q-01.14`), con tres términos propios que sobrevivieron a mover
      *responsable* a evidencia.
- [x] Cada subdominio tiene propósito, clasificación, razón y coste de clasificarlo mal, **en
      lenguaje de negocio**.
- [x] La clasificación es **deducible de una regla escrita** (`DEC-01.14`), y esa regla **no
      clasifica ningún subdominio de dos formas a la vez** — probado contra el caso que la habría
      roto (`Q-01.30`).
- [x] Está escrito **cómo se relacionan** los subdominios entre sí, y qué se rompe si una relación
      cae.
- [x] Está escrito qué genéricos son candidatos a no escribirse.
- [x] Los criterios de éxito están fijados, son comprobables, e incluyen que el lenguaje llegue
      **al código** (frente 1.7).
- [x] Ninguna categoría del modelo queda fuera de la taxonomía del libro.
- [x] Cada subdominio tiene **actor** nombrado; el único con dos clientes —Diagnóstico— lo dice.
- [x] El **punto de referencia del core** está declarado: dos, uno por cliente.
- [x] El **contrato de salida del core** está declarado: solo hechos, y nombra la causa por
      deducción forzosa.
- [x] Está declarada la **forma de la respuesta**: el subconjunto de ejes que sobrevive.
- [x] Ningún término de `dominio.md` nombra dos cosas; el que lo hacía está identificado y su
      corrección es trabajo de IT-02.
- [x] `modelo/dominio.md` reescrito y sin espacio de la solución dentro.
- [x] Tablero de `plan-ddd.md` actualizado.

## 9. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Partir Ejecución de pipeline en dos bounded contexts (*Decidir / Planificación del Intento* vs. ejecución real), con su prueba de separación | **IT-03** | Es una decisión del espacio de la solución; DEC-01.3 la deja nombrada y sin resolver a propósito |
| 2 | Cómo conviven la duplicación declarada de la capacidad de huella y la regla del repositorio «una regla, tres raíces» — ¿shared kernel, published language o librería? | **IT-04** | Es la elección de un patrón de context mapping, materia de E4 |
| 3 | **Contradicción con la spec 27**: la huella granular se describió con «variables resueltas del step/ambiente», y `pipe-v1` hashea **declaraciones, nunca valores resueltos** — precisamente para ser conocible antes de ejecutar y para no emitir valores de runtime como digest | **IT-08** | Toca a la vez a la regla de huella y a DEC-01.6; resolverla aquí habría sido decidir táctica sin el contexto modelado |
| 4 | Diseño de la ofuscación por defecto: qué se conserva para comparar, dónde vive el secreto, qué pasa con `keys/digest-v1.key` y con el `content_id` sin sal | **IT-08** | DEC-01.6 fija la postura; el mecanismo es táctico |
| 5 | Espacio de Trabajo «por ambiente» y «con continuidad entre ejecuciones» frente a la copia por ejecución del handler 05 | **IT-10** | Cambio de contenido, no de clasificación; se modela con el contexto |
| 6 | Reubicar la restauración de plantillas tras fallo (defecto D1) en Ejecución de Step | **IT-09** | Depende de qué agregado protege la invariante del intento |
| 7 | ¿Se retira el mutex de `Execution` y `ErrTransicionIlegal`, o se justifica este último por la carrera señal/cadena? | **IT-09** | DEC-01.9 lo habilita; la comprobación es táctica |
| 8 | Evaluar `dirhash` frente a las ~380 líneas propias, `go-git` frente a los clonadores y `gokv` frente al puerto de estado | **IT-10** | «Seguir escribiéndolo» es respuesta válida si la razón está escrita |
| 9 | La prueba de frontera de E3 tiene que ser **una**: qué concepto cambia de significado al cruzar de «decidir» a «ejecutar». Volatilidad y aislamiento en pruebas no son criterios de contexto (**H5**) | **IT-03** | El criterio se fija aquí; aplicarlo es trabajo de E3 |
| 10 | Si la regla de huella es contrato versionado, ¿qué patrón de relación le corresponde y quién es su dueño? (**H1**, resto de `Q-01.11`) | **IT-04** | Es elección de patrón de context map |

| 11 | ¿Simulación es un **contexto propio o un modo** de Ejecución? Comparte casi todo el modelo: solo sustituye la ejecución de los comandos (`Q-01.16`) | **IT-03** | Es espacio de la solución; el subdominio ya está decidido |
| 12 | **Estrategias de lanzamiento**, y la etiqueta técnica por defecto frente al nombre que elige el dueño del negocio (`Q-01.17`) | **IT-10** | Fuera de alcance por decisión explícita; es forma, no problema |
| 13 | **Reconciliación con las specs `00`–`28`**: cuáles quedan derogadas por el modelo | **IT-11** | Regla del proceso enmendada en `Q-01.15`: la cuenta se hace una vez, al final, no por decisión |

| 14 | Desambiguar «el último resultado exitoso»: tras `Q-01.21` nombra dos cosas —último despliegue y último lanzamiento— y el core usa una u otra según quién pregunte | **IT-02** | Es trabajo de lenguaje, y E2 va justo de eso |
| 15 | Quién promete **quién** cambió qué y **cuándo**. La fila está en la cadena de garantías sin dueño, a propósito (`Q-01.23`) | **IT-03** | Dos candidatos, y asignarlo por comodidad es peor que dejarlo a la vista |

### Nota de proceso

La primera ronda cumplió su función —hay respuesta a las diez preguntas y un modelo que se
sostiene— pero se validó contra sí misma. La segunda ronda existe porque **responder no es
cerrar**: una respuesta es material que hay que someter a la lupa del libro, y ocho de los diez
puntos no la habían pasado todavía. La iteración se cierra cuando la pase, no cuando se acaben
las preguntas — y aquí hicieron falta **seis rondas más** para que la pasara.

La ronda 2 lo confirmó: cuatro respuestas cerraron limpias, pero `Q-01.14` y `Q-01.17` —las dos
que obligaban a escribir lenguaje y actores en vez de clasificaciones— **movieron el mapa**. Una
confirmó el core y le dio vocabulario; la otra partió un subdominio en dos. Ninguna de las dos
habría aparecido revisando la lista de subdominios: aparecieron al preguntar *quién habla* y
*con qué palabras*, que es por donde el libro dice que se empieza.

Las cinco rondas siguientes lo repitieron a menor escala. `Q-01.27` no eligió ninguna de las tres
opciones: **reformuló la pregunta**, y la reformulación —tres ejes y eliminación— resultó ser el
mecanismo del core, que hasta entonces se describía como «comparar» sin decir comparar qué.
`Q-01.29` reveló que tres respuestas dispersas eran **una sola regla**. Y `Q-01.30` no encontró
nada: comprobó que la línea entre los genéricos y el supporting coincide con la línea entre lo que
un repositorio puede responder y lo que no — una coincidencia que nadie forzó.

**Cuando la validación deja de encontrar problemas y empieza a encontrar confirmaciones, ha
terminado.**
