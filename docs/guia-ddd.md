# Guía de DDD aplicada al motor de Vex

> Compañera de `plan-ddd.md`. Aquí se explica **qué dice el libro**; en `iteraciones/` se
> **decide**. Cada iteración indica qué sección leer antes.
>
> Fuente: Vaughn Vernon, *Implementing Domain-Driven Design* (2013). Se cita el capítulo para
> que puedas contrastar; los ejemplos son siempre de este motor, nunca genéricos.
>
> Crece por bloques: el bloque estratégico (§1–§5) y el táctico (§6–§13), escrito al abrir E6.

---

## §0. Las dos mitades de DDD, y por qué el orden importa

DDD tiene una mitad **estratégica** (qué modelos existen, dónde acaba cada uno, cómo se hablan)
y una **táctica** (agregados, entidades, value objects, eventos, repositorios).

La táctica es la parte famosa y la que más se copia. También es la que no sirve de nada si la
estratégica está mal: un agregado perfecto dentro de un contexto mal trazado es un agregado en
el sitio equivocado. Vernon insiste en esto porque es el error más común — equipos que aplican
«patrones DDD» sobre una única bola de barro y concluyen que DDD no funciona.

**Cómo se manifiesta esto en Vex hoy.** El motor ya tiene táctica de buen nivel: dos agregados
declarados como tales (`command.Execution` y `deployment.Content`), un catálogo de value
objects real, puertos definidos en el paquete que los consume, eventos de dominio con
vocabulario cerrado. Y sin embargo `internal/domain/command/` contiene a la vez la ejecución,
el step, la variable y el contexto compartido, porque el grafo de imports de Go no dejaba
ponerlos en otro sitio.

Eso es exactamente el síntoma: táctica buena sobre una frontera que nadie trazó. Por eso este
plan gasta cinco etapas en lo estratégico antes de tocar un solo agregado.

---

## §1. Dominio, subdominios y destilación *(cap. 2)*

### Los términos

- **Dominio**: aquello de lo que trata el software. En Vex: desplegar software mediante
  pipelines declarativos y poder responder después qué pasó y por qué.
- **Subdominio**: una parcela del dominio con cohesión propia. El dominio se descompone en
  subdominios para poder decidir *dónde invertir*.

### Los tres tipos

**Core Domain** — donde está la ventaja competitiva. Es la razón por la que alguien elegiría
Vex y no otra herramienta. Aquí va el mejor diseño, el mejor modelado y el mejor programador.
Vernon es explícito: si no puedes nombrar tu core domain, no sabes qué estás construyendo.

**Supporting Subdomain** — necesario, con lógica de negocio propia y específica tuya, pero no
es lo que te diferencia. Merece un modelo cuidado, no el mejor.

**Generic Subdomain** — necesario y resuelto igual por todo el mundo. La pregunta correcta
aquí no es «cómo lo diseño» sino «puedo no escribirlo». Comprarlo, usar una librería o
copiarlo son respuestas legítimas y a menudo la mejor.

### La trampa que este motor tiene puesta

`modelo/dominio.md` declara **Diagnóstico** como core. En el código no existe: no hay
`internal/domain/diagnostic/`, no hay agregado, no hay puerto. Las 14.000 líneas de dominio
están repartidas así:

| Paquete | Líneas | Subdominio declarado |
|---|---:|---|
| `step` | 3.276 | Ejecución de pipeline *(supporting)* |
| `command` | 2.487 | mezcla *(supporting + resolución de variables)* |
| `pipeline` | 2.238 | Ejecución de pipeline *(supporting)* |
| `record` | 2.188 | Registro *(supporting)* |
| `deployment` | 1.505 | Registro *(supporting)* |
| `fingerprint` | 975 | sin dueño declarado |
| resto | ~1.400 | generic |
| **Diagnóstico (core)** | **0** | **core** |

Hay dos lecturas posibles y E1 tiene que elegir una:

1. **La clasificación es correcta y el core está sin construir.** Entonces el plan es justo el
   que dice el libro: construir el core y dejar de crecer los supporting.
2. **La clasificación está mal.** Quizá el core real es la re-ejecución por contenido (el
   sistema de huellas: decidir con exactitud qué hay que volver a ejecutar y qué no), y
   Diagnóstico es una vista de consulta sobre el Registro. Sería un core que sí está construido
   y sí tiene el mejor diseño del repositorio — las tres reglas de huella tienen especificación
   normativa propia.

Distinguirlas no es cosmética: cambia dónde se invierte todo el esfuerzo posterior.

### La prueba práctica para clasificar

Para cada subdominio, tres preguntas:

1. **Si esto lo hiciera igual que un competidor, ¿perdería clientes?** Sí ⇒ candidato a core.
2. **¿Existe ya, hecho por otro, algo que sirva?** Sí ⇒ candidato a generic.
3. **¿Tiene reglas que solo tienen sentido en mi negocio, pero que nadie elegiría el producto
   por ellas?** Sí ⇒ supporting.

---

## §2. Bounded Context *(cap. 2)*

### Qué es, exactamente

Un **bounded context** es una frontera explícita dentro de la cual un modelo de dominio tiene
un significado único y consistente. Dentro de ella, cada término significa una cosa y solo una.

Un subdominio es una parcela del **problema**. Un bounded context es una parcela de la
**solución**. Lo ideal es que se correspondan uno a uno, pero no siempre pasa — y cuando no
pasa, hay que saberlo.

### El lenguaje ubicuo vive DENTRO de un contexto

Este es el punto que más cuesta y el que gobierna la etapa E2.

No existe *un* lenguaje ubicuo del sistema. Existe uno **por contexto**. El mismo término puede
significar cosas distintas en dos contextos y eso no es un error que arreglar: es información
sobre dónde está la frontera. Lo que sí es un error es que signifique dos cosas *dentro* del
mismo contexto.

**Vex ya tiene esto documentado sin haberlo llamado así.**. Un `step` es a la vez:

- el directorio `NN-nombre/` con su `commands.yaml` — eso es Definición de Pipeline;
- la unidad que se ejecuta o revive en un intento — eso es Ejecución;
- el `step_id` que forma parte de la clave de estado — eso es Registro.

Tres contextos, tres significados. Que el código los tenga en el mismo paquete `command` es el
problema; que existan tres significados no lo es.

Lo mismo con «variable» (declarada en `variables/`, resuelta en tiempo de ejecución, extraída
de stdout), con «estado» (el `state/` de un step frente al ciclo de vida de la `Execution`), y
con «registro» (el contexto Registro frente a `state.StepRecord`).

> **Cerrado en E2 (IT-02).** Los tres se resolvieron de tres formas distintas, y la diferencia
> enseña más que el resultado:
>
> - **«paso» y «variable» sobreviven como homónimos declarados**, cada uno con la frontera que
>   señala escrita al lado. Es lo que el libro autoriza, y es lo que E3 consume.
> - **«estado» se descargó**: lo que el `state/` guarda pasó a llamarse **historial**, y «estado»
>   quedó solo para el desenlace de un intento. Un homónimo menos, sin que nadie cediera nada.
> - **«evidencia» no llegó a ser homónimo**: los dos sentidos iban a encontrarse *dentro* del core
>   —porque el core lee el historial— y ése es el caso que sí es un error. **Cedió el core**, que
>   es quien ve la colisión, y dice **sustento**.
>
> La lección: *«homónimo entre contextos, sí; dentro de uno, no»* no se aplica mirando el mapa, se
> aplica mirando **quién lee a quién**.

### Cuándo un contexto es demasiado pequeño

Vernon advierte de los dos extremos, y el segundo es el que amenaza a Vex.

Un contexto **demasiado grande** es una bola de barro: términos ambiguos, un modelo que
nadie entiende entero.

Un contexto **demasiado pequeño** cuesta traducción. Cada frontera es un coste permanente:
puertos, adaptadores, mapeo, y la disciplina de no dejar que los modelos se filtren. Si dos
supuestos contextos comparten el mismo lenguaje sin traducir nada, no son dos contextos: son
uno partido en dos por comodidad de directorios.

**Contraste que E3 tiene que hacer.** `modelo/bounded-contexts.md` declara **once** contextos
para un binario one-shot de 14.000 líneas mantenido por una persona. Es plausible, pero la
carga de la prueba está del lado de mantenerlos separados, no de fusionarlos. La prueba que
pide E3 —«si lo fusionara con X, se rompería esto»— es exactamente eso.

Dato relevante para esa discusión: la heurística clásica «un equipo por contexto» no aplica
aquí. Un equipo puede llevar varios contextos; lo que no puede es que un contexto lo lleven dos
equipos. Con un solo desarrollador, esa heurística no descarta nada, así que el criterio tiene
que ser puramente el del lenguaje: **dos contextos son dos si al cruzar la frontera hay que
traducir**.

---

## §3. Context Map y sus patrones de relación *(cap. 3)*

### Qué es

El **Context Map** dice cómo se relacionan los contextos. No es un diagrama bonito: por cada
par que se integra declara la **dirección** (quién es upstream, quién downstream) y el
**patrón**, que define quién absorbe el coste del cambio.

Es la pieza que falta por completo en `docs/`. Y no porque las relaciones no existan —
existen y hay indicios dentro de `internal/interfaces/cli/factory.go`, no se va ha seguir el codigo
para este diseño, pero nos dara una idea.

### Los nueve patrones

**De organización** (quién manda):

- **Partnership** — dos contextos que se hunden o flotan juntos. Se coordinan y sus
  planificaciones están atadas. Es caro; se usa cuando no queda otra.
- **Shared Kernel** — comparten *código de modelo*. Cualquier cambio en la parte compartida
  hay que negociarlo. Vernon lo trata como un mal necesario: minimizarlo siempre.
- **Customer–Supplier** — el upstream provee y el downstream consume, pero el downstream tiene
  voz: sus necesidades entran en la planificación del upstream.
- **Conformist** — el downstream se traga el modelo del upstream tal cual, sin traducir. Barato
  y contaminante: el modelo ajeno entra en tu dominio.
- **Separate Ways** — se decide no integrar. Duplicar sale más barato que acoplar.

**De técnica** (cómo se cruza la frontera):

- **Anticorruption Layer (ACL)** — el downstream se defiende: una capa traduce el modelo ajeno
  al propio, y ningún concepto de fuera entra sin pasar por ella. Es el patrón por defecto
  cuando el upstream no lo controlas.
- **Open Host Service (OHS)** — el upstream publica un protocolo de servicio pensado para ser
  consumido por varios, en vez de un acuerdo distinto por cada cliente.
- **Published Language** — el vocabulario compartido y versionado con el que se habla en la
  frontera. Suele acompañar al OHS.
- **Big Ball of Mud** — no es un patrón que se elija: es lo que hay cuando no hay frontera. Se
  dibuja en el mapa para saber dónde no meterse.

### Los casos concretos que E4 tiene que resolver

Todos están en el código y ninguno tiene nombre todavía:

**1. Los tres `FactSink`.** Tres contextos (`command`, `step`, `sync`) declaran cada uno su
propio puerto `FactSink`, y un único adaptador `record.Facts` los implementa los tres. Es la
solución correcta al ciclo `record → deployment → step`, y su nombre en el mapa es lo que hay
que decidir: ¿tres relaciones Customer–Supplier independientes con el mismo proveedor, o un
Published Language que resulta que tiene tres puertas?

**2. La aserción del factory.** `var _ stepDom.AnchorLookup = deploymentDom.RollbackAnchor{}`
en `internal/interfaces/cli/factory.go:177`. El comentario dice que vive ahí «por ser el único
punto que ve ambos lados sin cerrar un ciclo de imports». Es una relación real entre Registro y
Ejecución de Step, verificada en el composition root porque el modelo no tiene dónde ponerla.

**3. Las cinco `SPEC-*.md` junto al código.** `fingerprint/SPEC-v1.md`, `SPEC-PIPELINE-v1.md`,
`SPEC-STEP-v1.md`, `deployment/SPEC-CONTENT-v1.md`, `record/SPEC-FOLD-v1.md`. Son reglas
normativas, versionadas con token propio (`v1:`, `pipe-v1:`, `sf-v1:`, `cnt-v1:`), y la regla
del repositorio es que cambiar lo que entra en un hash obliga a subir la versión. Eso es
**Published Language** de libro, y llamarlo así tiene consecuencias: un published language no
se toca sin versionar, y ese contrato ya se está cumpliendo por disciplina sin tener el nombre.

**4. El contrato con el CLI.** `RequestInput` con su `CurrentSchemaVersion = 2`, sincronizado
**a mano** entre dos repositorios. Es un contrato entre sistemas separados y hoy se mantiene
por memoria humana. En el mapa: ¿OHS + Published Language, o Conformist?

**5. `fingerprint`, el paquete sin dueño.** 975 líneas de las que dependen tres contextos. O es
un contexto propio, o es un Shared Kernel declarado, o pertenece a uno y los demás lo consumen
por un puerto. Las tres respuestas son defendibles y hay que elegir una.

---

## §4. Arquitectura *(cap. 4)*

### Lo que el libro dice de entrada

DDD no impone arquitectura. Impone una condición: **el dominio no depende de nada de fuera**.
Cualquier arquitectura que garantice eso vale.

### Los estilos que importan aquí

**Capas / Hexagonal (Ports & Adapters).** El dominio define puertos (interfaces) y la
infraestructura los implementa. **Esto ya está hecho y bien hecho en Vex**: la regla
`interfaces → application → domain ← infrastructure` está escrita, los puertos se declaran en
el paquete que los consume, y hay verificaciones de contrato en tiempo de compilación
(`var _ domPipeline.Port = (*infra.Impl)(nil)`). E5 no tiene que decidir esto.

**CQRS.** Separar el modelo de escritura del de lectura. Un agregado bien diseñado es malísimo
para consultar, y forzarlo a servir consultas lo deforma.

Vex ya hace CQRS **sin llamarlo así**: el lado de escritura es `record.EventSink` y el de
lectura es `record.ResultProjection`, implementado por `ScanProjection`. Hay incluso la regla
explícita de que «un lector no debe poder escribir». Lo que E5 tiene que decidir es si eso se
declara como la arquitectura del contexto Registro —con las consecuencias que tiene: proyección
materializada, modelo de lectura propio, `vexd record` como cliente de primera clase— o se
queda como una coincidencia afortunada.

**Event Sourcing.** El estado se deriva de la secuencia de eventos, no se guarda. `record` es
event sourcing puro: hechos en JSONL con `seq` monótono, y `Fold` para derivar el resultado.

Aquí hay una pregunta de modelado, no de implementación, y es de las buenas: **¿el event
sourcing es el modelo del contexto Registro, o es su mecanismo de persistencia?** Si es el
modelo, el lenguaje ubicuo de Registro habla de hechos y de plegado, y los diez tipos de evento
son conceptos de dominio. Si es persistencia, el lenguaje habla de intentos y despliegues, y
los eventos son un detalle que podría cambiarse por una tabla sin tocar el modelo. Las dos
respuestas producen diseños distintos y ninguna es obviamente correcta.

**Arquitectura dirigida por eventos.** Cómo se comunican los contextos. Dentro de un mismo
proceso caben tres respuestas: llamada directa a través de un puerto, evento de dominio
publicado, o un híbrido según el par. Vex es un binario **one-shot**, lo que cambia el cálculo:
no hay proceso de larga vida, no hay consistencia eventual entre servicios, y un bus de eventos
en memoria puede ser complejidad sin contrapartida — o justo lo que desacople los contextos sin
crear ciclos de import.

### El módulo *(cap. 9)*, que en Go es el paquete

Un módulo es un contenedor con nombre dentro de un contexto, y su nombre es parte del lenguaje
ubicuo. Vernon avisa contra los módulos organizados por *tipo técnico* (`entities/`,
`services/`, `repositories/`): agrupan por lo que las cosas son en vez de por lo que tratan.

En Vex el problema es una variante de ese: los paquetes de dominio están organizados por
**cadena de proceso** (`pipeline`, `step`, `command`, siguiendo los tres Chain of
Responsibility), no por contexto. Es agrupar por el mecanismo, no por el significado. De ahí
sale el `command` de 2.487 líneas que contiene la ejecución, el step, la variable y el contexto
compartido.

E5 decide el árbol objetivo. Y la decisión de fondo es si **un paquete Go de primer nivel =
un bounded context**, con las capas dentro de cada uno, en vez de las capas arriba y los
contextos mezclados dentro.

---

## §5. Cómo se leen las decisiones que ya están tomadas

Este motor no parte de cero: hay 29 specs implementadas, cada una con su §8 «Decisiones» y su
§9 «Hallazgos al implementar». Esa §9 —lo que se descubrió *después* de escribir el diseño— es
material de primera calidad para el modelado, porque es donde el dominio contradijo al plan.

Dos reglas al usarlas en este proceso:

1. **Una spec implementada es un hecho, no una opinión — pero no es una restricción del
   modelado.** Enmendado en IT-01 (`Q-01.15`): el modelo se diseña siguiendo el libro, y la
   cuenta de qué specs quedan derogadas se hace **una sola vez, en E11**. Dejar que cada decisión
   se reconcilie con lo implementado hace que lo construido dirija lo que se modela.
2. **Una spec no es el modelo.** Que algo esté implementado de una forma no significa que esa
   forma sea la correcta; significa que es la que hay. Los §A-1..A-8 de `vex-plan-revision.md`
   ya listan problemas de diseño conocidos (el God Object `ExecutionContext`, la regla que
   decide-hashea-y-escribe en el mismo eslabón, la falta de red de seguridad) que este proceso
   hereda como entrada.

---

## Bloque táctico *(§6–§13)*

> Se escribió al abrir E6. Aquí se explica qué dice el libro; en las iteraciones se decide. Los
> ejemplos salen del modelo vigente (`modelo/`), nunca del código.

**El orden importa otra vez.** El libro no empieza un modelo táctico por las entidades: empieza por lo
que el modelo tiene que **responder**. Por eso E6 abre con escenarios. Los patrones de abajo son las
piezas con las que después se escriben esas respuestas, y no al revés.

---

## §6. Entidades *(cap. 5)*

Una **entidad** es algo que conserva su **identidad** aunque cambie todo lo demás. Dos entidades son
la misma si tienen la misma identidad, aunque sus atributos sean distintos.

**La prueba**: *si cambian todos sus atributos, ¿sigue siendo lo mismo?* Si la respuesta es sí, es una
entidad.

**En este modelo.** IT-03 pasó por aquí sin nombrarlo. `Q-03.16` preguntó si un paso *es* sus comandos
o *sigue siendo el mismo* aunque sus comandos cambien. La respuesta (el nombre dice cuál es y los
comandos dicen si cambió) convierte al paso de Definición en una entidad cuya identidad es su nombre.
El **intento** del Historial también es una entidad: tiene identificador propio y sabe cuál fue el
anterior.

**La trampa.** Modelar algo como entidad porque «se guarda con un identificador». Tener una fila en un
almacén no le da identidad de dominio.

---

## §7. Value Objects *(cap. 6)*

Un **value object** mide, cuantifica o describe algo. El libro le da estas características:

- es **inmutable**: no se modifica, se sustituye por otro;
- es un **todo conceptual**: sus atributos solo tienen sentido juntos;
- se compara **por valor**: dos iguales son intercambiables;
- su comportamiento **no tiene efectos laterales**.

El libro recomienda **preferirlos**: todo lo que no necesite identidad se modela como value object.

**En este modelo.** Casi todo lo que maneja el core tiene esa forma. Un **hash**: dos hashes iguales
son el mismo hash. El **estado de un eje** entre dos despliegues: cambió o no cambió. La
**atribución**, que es un subconjunto de ejes. Ninguno necesita identidad.

---

## §8. Servicios de dominio *(cap. 7)*

Un **servicio de dominio** es una operación del dominio que no pertenece de forma natural a ninguna
entidad ni a ningún value object. No tiene estado, se nombra con el lenguaje ubicuo y **no** es un
servicio de aplicación, porque contiene reglas de negocio.

**En este modelo.** La **eliminación** es la candidata obvia: recibe el estado de los tres ejes y
devuelve los candidatos que sobreviven. No pertenece a un eje ni a un despliegue; es un procedimiento,
y el core ya le puso nombre.

**La trampa.** Sacar todo el comportamiento a servicios deja las entidades vacías, que es el llamado
modelo anémico. Un servicio solo se justifica cuando meter la operación dentro de un objeto lo
deformaría.

---

## §9. Eventos de dominio *(cap. 8)*

Un **evento de dominio** es algo que pasó y que le importa al experto. Se nombra en pasado, es
inmutable y lleva lo que necesitan quienes lo escuchan. Lo publica el modelo que lo produce, y se
puede escuchar dentro del mismo contexto o desde otros.

**En este modelo.** *Despliegue registrado* (IT-04 `DEC-04.8`) es el primero: lo publica el Historial y
lo escucha Lanzamiento.

Conviene ver un parentesco: un **registro** del Historial ya es un hecho escrito, inmutable y en
pasado. Por eso `DEC-05.5` dice que los registros *son* el modelo del Historial. Ese contexto está
hecho de lo que en otros contextos serían eventos.

---

## §10. Agregados *(cap. 10)*

Un **agregado** es una frontera de consistencia: un grupo de entidades y value objects, con una raíz,
que se modifica **entero o nada** en una misma transacción. Existe para proteger **invariantes**,
reglas que tienen que cumplirse siempre y no solo «al final».

Las cuatro reglas del libro:

1. **Modela invariantes verdaderas dentro de la frontera.** Si nada obliga a que dos cosas cambien
   juntas, no van en el mismo agregado.
2. **Diseña agregados pequeños.** Uno grande bloquea, cuesta cargarlo y casi siempre protege
   invariantes que no existen.
3. **Referencia a otros agregados por su identidad**, no por el objeto.
4. **Fuera de la frontera, consistencia eventual**: lo que tiene que pasarle a otro agregado ocurre
   después, normalmente a través de un evento.

El libro admite cuatro razones para romperlas: la comodidad de la interfaz de usuario, la falta de
mecanismos técnicos, las transacciones globales impuestas y el rendimiento de las consultas. Si se
rompe una regla, se escribe con su razón.

**Dos lecturas previas para este modelo:**

- **Aquí la transacción tiene una forma concreta.** Con `DEC-05.9`, cada registro se escribe y se lleva
  al almacén en cuanto ocurre. Lo que tiene que ser verdad a la vez es lo que cabe en un registro. Y
  en un proceso de un solo uso, *después* significa más tarde dentro de la misma invocación
  (`DEC-05.3`), no en otro proceso.
- **Un contexto que solo lee puede no tener agregados.** Los agregados protegen invariantes cuando algo
  **cambia de estado**, y Diagnóstico no escribe nada: compara lo que el Historial ya guardó. Es posible
  que su modelo sean value objects y un servicio de dominio, y que eso sea lo correcto, no un modelo a
  medias.

---

## §11. Factorías *(cap. 11)*

Una **factoría** encapsula la creación de algo cuya construcción es compleja o que tiene que nacer ya
válido. Puede ser un método en la raíz de un agregado o un servicio. El libro destaca un caso: el
**servicio que traduce el modelo de otro contexto** a objetos del propio también es una factoría.

**En este modelo.** El ACL de Diagnóstico hacia el Historial (IT-04, fila #1) es exactamente eso: a
partir de registros fabrica el estado de los tres ejes, que es lo único con lo que razona el core.

---

## §12. Repositorios *(cap. 12)*

Un **repositorio** da acceso a los agregados como si fueran una colección en memoria. Hay **uno por
agregado**, no uno por tabla ni uno por value object. Su interfaz vive en el dominio y su
implementación, en la infraestructura. El libro distingue dos estilos:

- **orientado a colección**: se añade y se quita, y lo que se modifica queda guardado sin más;
- **orientado a persistencia**: hay que guardar explícitamente.

**En este modelo.** En el Historial los registros solo se añaden, nunca se cambian (`DEC-05.5`). Su
repositorio solo necesita **añadir** y **recorrer**. Si algún día apareciera *actualizar* o *borrar*,
sería la señal de que algo dejó de ser un hecho.

---

## §13. Servicios de aplicación e integración *(cap. 13, cap. 14)*

Un **servicio de aplicación** coordina un caso de uso: recibe la petición, carga lo que hace falta,
llama al dominio y deja constancia. Controla la transacción y la seguridad, y **no contiene reglas de
negocio**. Si una regla aparece ahí, al dominio le está faltando algo.

**Integrar dos contextos** tiene dos piezas en el libro: quien llama al otro contexto (un adaptador) y
quien traduce su modelo al propio (un traductor). Juntas forman el ACL.

**En este modelo.** Los contextos se llaman directamente dentro del proceso (`DEC-05.3`), y cada
relación del context map ya dice dónde vive su traducción (`arquitectura.md`). Lo que falta, contexto
por contexto, es escribir qué operaciones publica cada uno, y eso forma parte del cierre de cada
iteración de este bloque.
