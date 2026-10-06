# IT-02 — Lenguaje ubicuo por contexto

> Etapa: E2 · Estado: **cerrada**
> Abierta: 2026-08-27 · Cerrada: 2026-08-27
> Lectura previa: `guia-ddd.md` §2 *(«El lenguaje ubicuo vive DENTRO de un contexto»)*
>
> **37 preguntas · 18 decisiones · 4 rondas de validación.** Tres decisiones enmiendan a IT-01:
> `DEC-02.4` y `DEC-02.16` tocan a `DEC-01.2`; `DEC-02.18` retira una pieza de `DEC-01.4`.
>
> **Archivo cerrado: no se vuelve a editar.** Lo que cambia a partir de aquí es `modelo/`.

---

## 1. Objetivo — qué tiene que ser verdad al cerrar

`modelo/lenguaje.md` hoy es **un glosario único**, marcado obsoleto desde el cierre de IT-01. El
libro dice que eso no existe: no hay *un* lenguaje ubicuo del sistema, hay **uno por contexto**, y
que un término signifique dos cosas en dos contextos no es un error — es información sobre dónde
está la frontera. El error es que signifique dos cosas **dentro** del mismo.

Al cerrar tiene que ser verdad:

1. El lenguaje está **partido por ámbito**, no en una lista plana, y arranca por el del core.
2. Existe una **tabla de homónimos** con su resolución: por cada término que nombra más de una
   cosa, qué significa en cada sitio y si sobrevive con el mismo nombre.
3. Existe una **tabla de sinónimos**: por cada concepto nombrado de dos formas, cuál se queda.
4. Ningún término de `modelo/dominio.md` queda sin definir, y ninguno definido queda sin usar.
5. Los términos que IT-01 **retiró o creó** están aplicados: fuera «Plan automático»; dentro
   *desplegar* vs. *lanzar*, *deducción* vs. *inferencia*, los tres ejes, y el lenguaje del core
   —**causa · evidencia · reincidencia**.
6. Está corregida la línea falsa que IT-01 dejó señalada: Simulación **no** es «contexto aislado
   que solo depende de Definición» (`Q-01.16`).
7. Cada homónimo que sobrevive lleva escrito **qué frontera está señalando**, porque eso es
   exactamente la entrada de E3.

**Lo que esta iteración NO hace.** No decide fronteras (E3), no elige patrones de relación (E4),
no toca código. Decide **palabras**, y deja el material con el que E3 traza la línea.

---

## 2. Punto de partida

### 2.1 Lo que dice hoy el modelo

`modelo/dominio.md` está **vigente** desde el cierre de IT-01 y manda. Aporta a esta iteración:

- **Nueve subdominios** con actor nombrado: Diagnóstico *(core)*, Definición de Pipeline,
  Registro de Despliegue, Lanzamiento, Ejecución de Pipeline, Resolución de Variables, Simulación
  de Pipeline *(supporting)*, Suministro de Fuentes y Sincronización de Estado *(generic)*.
- **Tres actores**: programador, DevOps, dueño del negocio.
- **Lenguaje del core ya fijado** (`DEC-01.2`): causa · evidencia · reincidencia.
- **Los tres ejes** (`DEC-01.12`): código del producto · instrucciones del pipeline · variables.
- **Dos actos distintos** (`DEC-01.13`): desplegar y lanzar.
- **Una distinción de método** (`DEC-01.11`): deducción, nunca inferencia.

`modelo/lenguaje.md` está **marcado obsoleto** y es lo que esta iteración reescribe.
`modelo/bounded-contexts.md` está obsoleto y **no se toca aquí**: es IT-03.

**El encargo explícito de IT-01 §6** para esta iteración: reorganizar por contexto arrancando del
core; retirar «Plan automático»; desambiguar «el último resultado exitoso», que tras `Q-01.21`
nombra dos cosas; y escribir *desplegar* vs. *lanzar* y *deducción* vs. *inferencia*.

**Duda diferida que vence aquí**: la **#14** de IT-01 §9 — «el último resultado exitoso» nombra
el último **despliegue** y el último **lanzamiento**, y el core usa uno u otro según quién
pregunte. Es `Q-02.4`.

### 2.2 Lo que dice hoy el código

El vocabulario real, contado por tipos exportados de `internal/domain/`. No es una auditoría de
código: es **la lista de palabras que el motor ya usa**, que es contra lo que hay que contrastar
el lenguaje que se escriba.

| Paquete | Líneas | Palabras que aporta |
|---|---:|---|
| `step` | 6.062 | `Scope` · `Rule`/`RuleSet`/`RuleKind`/`RuleCategory` · `SkipReason` · `VariableDeclaration`/`VariableSource` · `EvidenceFact` · `LoadedStep` · `StepConfig` · `RecordProvider` · `FactSink` |
| `record` | 3.547 | `Event`/`EventType` (diez tipos) · `AttemptStarted`/`AttemptFinished` · `ParameterResolved` · `EvidenceRef` · `ErrorClass` · `Fault` · `Seq` · `Facts` · `ResultProjection` |
| `command` | 3.277 | `Execution`/`ExecutionStatus`/`ExecutionContext` · `Step`/`StepName`/`StepStatus`/`StepReason` · `Variable`/`Origin`/`ExecutionVariableMap` · `Command`/`CommandStatus` · `FactSink` |
| `pipeline` | 2.936 | `Manifest` · `Pipelinecode` · `StepEntry` · `ProjectVersion` · `ContentFingerprint` · `PipelineSource` · nueve `*Rule` de validación estructural |
| `deployment` | 2.942 | `Content`/`ContentID` · `DeploymentID` · `Attempt` · `Lineage` · `StepContent` · `Source` · `Subject` · `Operation` · `Destination` · `RollbackAnchor` |
| `fingerprint` | 2.183 | `Fingerprint` · `TreeSource` · `*Material` |
| `state` | 902 | `Key` · `Scope` · `StepRecord` · `RecordID` · `Provenance` · `Records` |
| `sync` | 836 | `Sink` · `Batch` · `AckStore` · `Synchronizer` · `FactSink` |
| `cache` | 162 | `Entry`/`Entries` |
| `syncconfig`, `notify`, `shared` | 425 | `Config`/`Type` · `Vocabulary` · `Clock` |

**Cinco hechos de este inventario, que son los que arman las preguntas:**

1. **Tres paquetes declaran `FactSink`** (`command`, `step`, `sync`) y uno solo lo implementa
   (`record.Facts`). El *patrón* es de E4; la *palabra* es de aquí.
2. **Dos paquetes declaran `Scope`** —`state/scope.go:39` y `step/scope.go:38`— y el modelo usa
   «ámbito» para una tercera cosa.
3. **`deployment.Attempt` y `deployment.DeploymentID` no significan lo que el modelo llama
   intento y despliegue.** Está en el propio doc del código: `attempt.go:21` define el intento
   como *«el número de intento de un mismo despliegue»*, y `deployment_id.go:23` define el
   despliegue como *«en qué posición de la historia de este ambiente cae este contenido»* —
   derivado **antes** de ejecutar y con independencia del resultado.
4. **El Registro llama *parámetro* a lo que todos los demás llaman *variable***
   (`record/payloads.go:279`, `parameter_resolved`).
5. **`evidencia` ya está ocupada**: `record/evidence.go:35` y `step/facts.go:86` la usan para
   *qué registro justificó que un step reviviera*. El core la reclamó en `DEC-01.2` para otra
   cosa.

---

## 3. Inventario de preguntas

> Doce preguntas. Las dos primeras son de **método** —sobre qué eje se escribe el lenguaje, y qué
> autoridad tiene sobre el código— y condicionan la forma de todas las demás. Las nueve
> siguientes son **homónimos y sinónimos concretos**, ordenadas de mayor a menor consecuencia. La
> última recoge lo que sale.

---

### Q-02.1 — Si los contextos se deciden en E3, ¿sobre qué eje se escribe el lenguaje en E2?

**Contexto.** El libro es tajante: el lenguaje ubicuo vive **dentro de un bounded context**
(`guia-ddd.md` §2). Pero los bounded contexts son espacio de la **solución** y se deciden en E3;
hoy solo están cerrados los **nueve subdominios** del espacio del problema (IT-01). El plan puso
E2 antes que E3 a propósito, y hay que decidir qué significa eso operativamente.

**Por qué importa.** No es una cuestión de formato. Si el lenguaje se escribe por subdominio, E3
recibe una **tabla de homónimos que es evidencia de dónde está la frontera** — que es el orden
que el libro defiende: el lenguaje *revela* el límite, no al revés. Si se escribe por contexto, E2
está decidiendo E3 de tapadillo y llamándolo glosario.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | Un apartado por **subdominio** de IT-01, declarando que son *contextos candidatos* y que E3 puede fusionarlos o partirlos | Es el orden del libro: el lenguaje se descubre y la frontera se deduce de él · No adelanta ninguna decisión de E3 · Nueve apartados con actor ya asignado | Al fusionar dos subdominios en E3, sus dos apartados se funden y algún homónimo pasa a ser un error dentro de un contexto — hay que rehacer esa parte |
| **B** | Un apartado por **actor** (programador · DevOps · dueño del negocio) | El lenguaje es de quien lo habla, y los tres actores están nombrados | Diagnóstico tiene dos clientes y quedaría partido en dos; los genéricos no tienen actor propio y quedarían huérfanos |
| **C** | Esperar: escribir solo el lenguaje del core y aplazar el resto a después de E3 | Nada se escribe dos veces | Deja a E3 sin la única evidencia que hace que su prueba se pueda aplicar: qué concepto cambia de significado al cruzar |

**Lo que arrastra.** Si sale **A**, cada homónimo del inventario tiene que salir con una frase
escrita —«esta ambigüedad señala la frontera entre X e Y»— porque ése es el entregable que E3
consume. Si sale **C**, la duda diferida #9 de IT-01 (la prueba de frontera de E3) se queda sin
material y hay que decir de dónde saldrá.

---

### Q-02.2 — ¿Qué autoridad tiene este lenguaje sobre el código, y en qué idioma se nombra?

**Contexto.** `DEC-01.10` frente 1.7 exige que *«un nombre del core se lea en voz alta a un DevOps
y signifique lo mismo que en `lenguaje.md`»*. Hoy hay dos idiomas conviviendo: los identificadores
son ingleses (`StepRecord`, `EvidenceRef`, `attempt_started`), los comentarios son mixtos, y todo
el modelo está en español. Y hay una capa más: los **hechos serializados** (`attempt_started`,
`parameter_resolved`) y los **tokens de regla** (`v1:`, `pipe-v1:`, `sf-v1:`, `cnt-v1:`) son
vocabulario que ya salió del binario y está escrito en disco.

**Por qué importa.** «Ubicuo» quiere decir *en todas partes*: si el término vive solo en el
documento, no es lenguaje ubicuo, es un glosario. Pero renombrar identificadores tiene coste, y
renombrar **lo serializado** tiene un coste distinto —invalida registros ya escritos— que no se
puede pagar por gusto lingüístico.

**Por qué es de esta iteración y no de E11.** Porque la respuesta cambia **cómo se escribe la
tabla**: si el lenguaje es español con traducción declarada, cada término lleva dos columnas
—término y nombre en código— y el desajuste se ve; si es inglés, la tabla se escribe en inglés y
`dominio.md` queda hablando otro idioma que su propio lenguaje.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Español es el lenguaje; el código traduce**, y cada término lleva su nombre en código en una columna | El actor habla español; el modelo entero está en español · La columna de traducción hace visible cada desajuste, que es lo que 1.7 pide | Convive con `record` y sus payloads en inglés para siempre · Una traducción declarada se puede incumplir sin que nada falle |
| **B** | **El lenguaje se escribe en el idioma en que se va a nombrar en código** (inglés), con el español como glosa | Elimina la capa de traducción: lo que se lee es lo que se escribe | Rompe con `dominio.md` y con los tres actores · Traducir *reincidencia* o *dar por bueno* al inglés es exactamente donde se pierden los matices que IT-01 tardó siete rondas en fijar |
| **C** | **Español en dominio y aplicación; inglés congelado en lo serializado**, declarado como frontera de traducción | Reconoce que el vocabulario de disco es un contrato con coste de cambio distinto | Introduce una frontera de idioma que hay que sostener a mano |

**Lo que arrastra.** Sea cual sea, hay que decir qué pasa cuando el nombre del código contradice
al del modelo: ¿es un defecto que se anota para E11, o el modelo cede? IT-01 ya fijó la dirección
—el modelo manda— pero no el procedimiento.

---

### Q-02.3 — «Intento», «despliegue» y «ejecución»: tres palabras, y el código usa dos de ellas al revés

**Contexto.** Es el homónimo más grave del inventario, porque el modelo y el código no discrepan
en el matiz: dicen **cosas opuestas**.

| Palabra | Qué dice el modelo (`lenguaje.md` obsoleto + `dominio.md`) | Qué dice el código |
|---|---|---|
| **intento** | ejecutar uno o varios pasos de un pipeline en un ambiente | `deployment/attempt.go:21` — **el número de reintento** de un mismo despliegue: la 1ª vez es 1, un reintento es 2 |
| **despliegue** | intento **exitoso de todos** los pasos; tiene padre; sin padre-último es rollback | `deployment/deployment_id.go:23` — **la posición de un contenido** en la historia de un ambiente, derivada **antes** de ejecutar y **con independencia del resultado** |
| **ejecución** | *(no está en el modelo)* | `command/execution.go:19` — el ciclo de vida de lo que el modelo llama intento: `queued → running → succeeded/failed/canceled` |

Hay además una cuarta pieza: el **`execution_id`** es lo que nombra el archivo de hechos de un
intento, precisamente porque `attempt` no servía para nombrarlo (`attempt.go`, decisión I-2).

**Por qué importa.** El core compara *«contra el último despliegue exitoso»* (`DEC-01.2`). Si
«despliegue» nombra a la vez *un resultado exitoso* y *una posición derivada antes de ejecutar*,
la frase del core admite dos lecturas y una de ellas es falsa. Y no se puede tapar: `DEC-01.12`
dice que el core **nunca habla con la fuente, lee el registro de lo que se usó** — o sea, lee
justo la pieza cuyo nombre está en disputa.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Manda el modelo**: intento = una ejecución; despliegue = intento exitoso de todos los pasos. Lo que el código llama `deployment_id` recibe **otro nombre** (p. ej. *posición* o *contenido situado*) y `attempt` pasa a ser *número de intento* | El lenguaje del negocio gana, que es la regla · Deja «despliegue» limpio para el core y para Lanzamiento | Renombrar `deployment_id` toca lo serializado, la CLI (`vexd record show <deployment_id>`) y tres specs |
| **B** | **Manda el código**: se acepta que despliegue es la posición y que intento es el reintento, y el modelo adopta otra palabra para «intento exitoso de todos los pasos» | Cero renombrados · La distinción del código es real y está bien argumentada | Un despliegue que **falló** seguiría llamándose despliegue; el DevOps no lo va a decir así, y 1.7 falla |
| **C** | **Tres palabras, tres cosas**: *ejecución* (el ciclo de vida técnico), *intento* (lo que se pidió hacer) y *despliegue* (lo que salió bien y completo), y la posición del linaje se llama de otra forma | Máxima precisión; cada una tiene dueño distinto | Cuatro términos donde el DevOps usa uno; riesgo de vocabulario que solo existe en el documento |

**Lo que arrastra.** La respuesta decide si **Registro de Despliegue** conserva su nombre: si
despliegue es solo lo exitoso y completo, el Registro guarda sobre todo cosas que **no** son
despliegues.

---

### Q-02.4 — «El último resultado exitoso» nombra dos cosas *(duda diferida #14 de IT-01)*

**Contexto.** `Q-01.21` estableció que el core tiene **dos puntos de referencia**: el programador
compara contra el último **despliegue** exitoso en ese ambiente; el dueño del negocio, contra el
último **lanzamiento**. La expresión *«el último resultado exitoso»* —que aparece en el
`lenguaje.md` de hoy y en varios sitios del código— nombra las dos, y IT-01 la declaró **no
válida** en este modelo.

**Por qué importa.** Es la expresión que sostiene la comparación del core. Si se queda ambigua, la
misma frase del producto significa una cosa para el programador y otra para el dueño del negocio,
que es exactamente el fallo que `DEC-01.13` separó.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | Se **prohíbe** la expresión y solo se admiten las dos concretas: *último despliegue exitoso en `<ambiente>`* y *último lanzamiento* | Elimina la ambigüedad de raíz · Obliga a que cada uso diga contra qué compara | Frases más largas; hay que revisar cada aparición |
| **B** | Se conserva como **término paraguas** con la regla «se resuelve según quién pregunta» | Una sola palabra para lo que el core hace en los dos casos | Un paraguas cuya resolución depende del lector es la definición de término ambiguo |
| **C** | Se sustituye por un término nuevo —*referencia*, *punto de comparación*— que se **parametriza** con el cliente | Nombra lo que de verdad hay: una función de quién pregunta | Vocabulario nuevo que ningún actor usa hoy |

**Lo que arrastra.** Si sale **A**, hay que decidir qué pasa cuando en un ambiente hay
despliegues pero ningún lanzamiento, y a la inversa: el core tiene que poder decir *«no hay contra
qué comparar»* sin que suene a fallo.

---

### Q-02.5 — «Paso»: tres o cuatro significados, y todos legítimos

**Contexto.** Es el caso que `guia-ddd.md` §2 usa de ejemplo, y la spec 17 §5.6 ya se titulaba
«Dos cosas llamadas *step*». Hoy son al menos cuatro:

| Sentido | Dónde vive | Palabra en código |
|---|---|---|
| el **directorio declarado** `NN-nombre/` con sus instrucciones | Definición de Pipeline | `pipeline.StepEntry`, `step.LoadedStep`, `step.StepConfig` |
| lo que **se ejecuta o se da por bueno** en un intento | Ejecución de Pipeline | `command.Step`, `step.StepExecutable` |
| el **`step_id`** que forma parte de la clave del estado | Registro / Sincronización | `state.Key` (`key.go:41`) |
| la **declaración congelada** dentro del objeto de despliegue | Registro | `deployment.StepContent` (`step_content.go:49`) |

**Por qué importa.** El modelo dice *«un paso no puede acceder a otro ámbito ni a otro espacio de
trabajo que no sea el suyo propio»*. ¿Cuál de los cuatro pasos es ése? Y `DEC-01.12` dice que las
instrucciones están *«agrupadas por paso»*: ése es el primero. La ambigüedad no es teórica —
decide qué eje del core está agrupado por qué cosa.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Un solo nombre, cuatro contextos**: «paso» significa lo suyo en cada apartado, y la tabla de homónimos lo declara | Es lo que el libro autoriza explícitamente · El DevOps dice «paso» y no va a decir otra cosa | Dentro del código de hoy los cuatro conviven en dos paquetes: la ambigüedad *sí* es un error mientras la frontera no exista |
| **B** | **Nombres distintos por sentido**: *paso declarado* · *paso ejecutado* · *posición de estado* · *paso registrado* | Cada frase del modelo queda sin ambigüedad | Vocabulario que nadie habla; cuatro términos para lo que el actor llama paso |
| **C** | **Dos nombres**: se separa la **declaración** (paso) de la **ejecución del paso** (otro término), y los otros dos sentidos se declaran derivados | Corta por donde `DEC-01.12` corta: instrucciones frente a lo que se hizo con ellas | Hay que elegir la segunda palabra, y las candidatas obvias ya están ocupadas |

---

### Q-02.6 — «Variable»: cuatro sentidos, un sinónimo del Registro y tres palabras para la procedencia

**Contexto.** Dos problemas encadenados.

*Los sentidos:*

| Sentido | Dónde | Código |
|---|---|---|
| **variable de pipeline declarada** (por ambiente y por paso) | Definición de Pipeline | `step.VariableDeclaration` |
| **valor efectivo** con el que se despliega aquí | Resolución de Variables | `command.Variable` (`variable.go:79`), `ExecutionVariableMap` |
| **variable de ejecución** extraída de la salida de un comando | Ejecución de Step | `05_vars_extractor_handler` → `Origin` runtime |
| **la marca del valor** que se conserva para comparar sin exponer (`DEC-01.6`) | Resolución / Registro | `record.ParameterResolved.Digest` |

*El sinónimo:* el Registro la llama **parámetro** (`parameter_resolved`, `ParameterDigester`).
Todos los demás la llaman variable.

*La procedencia:* hay **tres palabras** para «de dónde salió esto»: `command.Origin`
(`origin.go:15` — la precedencia: declarada < estado < inyectada < runtime), `state.Provenance`
(`record.go:20` — qué ejecución y cuándo escribió el registro) y `step.VariableSource`
(`variable_declaration.go:24` — de dónde se resuelve el valor declarado). Tres cosas distintas o
tres nombres de lo mismo: hay que decirlo.

**Por qué importa.** «Variable» es **uno de los tres ejes del core** (`DEC-01.12`) y el único
cuya promesa —comparar sin exponer— sostiene una categoría entera de la respuesta. Un eje cuyo
nombre significa cuatro cosas no es un eje.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | «Variable» se queda como término general; los sentidos se distinguen con **adjetivo obligatorio** (*declarada*, *efectiva*, *de ejecución*); **«parámetro» se retira** como sinónimo | El actor sigue diciendo variable · El adjetivo obliga a precisar donde hace falta | Retirar «parámetro» toca un tipo de hecho ya serializado (`parameter_resolved`) |
| **B** | Se reservan **palabras distintas**: *declaración* · *variable* (la efectiva) · *salida* (la de ejecución) | Cada concepto un nombre; la frase «una declaración produce una variable» se lee sola | «Salida» ya se usa para la salida del comando; hay que comprobar que no se pisa |
| **C** | Se acepta que son cuatro sentidos en **cuatro ámbitos distintos** y no se toca nada, declarándolos en la tabla | Coste cero, y es lo que el libro autoriza entre contextos | Deja al eje del core sin nombre propio, y `DEC-01.6` depende de distinguir *valor* de *marca del valor* |

**Y en los tres casos hay que responder aparte**: `Origin`, `Provenance` y `VariableSource`
—¿tres conceptos o tres nombres de uno?

---

### Q-02.7 — «Ámbito»: dos tipos en código, un tercer sentido en el modelo

**Contexto.** Existen dos `Scope` (`state/scope.go:39` y `step/scope.go:38`), y el modelo usa
«ámbito» para una tercera cosa:

| Sentido | Qué es |
|---|---|
| **el ámbito del estado** | `project` o `environment/<nombre>`: dos tramos de la ruta donde vive el registro de un paso; determina **qué registro se lee y se escribe** |
| **el ámbito que un paso declara** | lo que el paso dice en su `config.yaml`; el propio código lo llama *«dirección»* y avisa de que tiene **dos lecturas del mismo dato** (`step/scope.go:32`) |
| **el ámbito del modelo** | *«espacio lógico y físico donde existen las variables»* (`lenguaje.md`), y la frase *«un paso no puede acceder a otro ámbito»* |

**Por qué importa.** El primero decide qué se compara contra qué; el tercero es una promesa de
aislamiento. `DEC-01.8` (retirada) mandó el aislamiento a Ejecución de Pipeline como *cómo cumple
lo suyo* — pero si el aislamiento se nombra con la misma palabra que la clave del estado, no se
puede decir cuál de los dos se rompió.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | «Ámbito» = **solo** el eje `proyecto \| ambiente` que decide qué registro y qué variables ve un paso. El aislamiento se nombra **aislamiento**, no ámbito | Una palabra, un sentido, y el más cargado de los tres | Hay que revisar la frase del modelo y el comentario de `step/scope.go` |
| **B** | Se conservan los dos sentidos con adjetivo: *ámbito de estado* y *ámbito declarado* | Refleja lo que hay | Dos ámbitos siguen siendo un ámbito ambiguo en cuanto alguien omite el adjetivo |
| **C** | Se separa por función: **ámbito** (dónde vive el dato) frente a **alcance** (hasta dónde llega lo que un paso ve) | Dos palabras españolas distintas para dos ideas distintas | «Alcance» y «ámbito» son casi sinónimos en el habla; puede no sostenerse |

---

### Q-02.8 — «Estado» y «registro»: dos palabras, cinco cosas

**Contexto.** Se tratan juntas porque el desambiguar una empuja a la otra.

*Estado:*

| Sentido | Dónde |
|---|---|
| el `state/` — los registros de paso: **la verdad** del despliegue, append-only | Registro / Sincronización |
| el **ciclo de vida** de la ejecución: `queued → running → succeeded/failed/canceled` | Ejecución |
| el subdominio **Sincronización de Estado**, que transporta y custodia lo primero | Generic |

*Registro:*

| Sentido | Dónde |
|---|---|
| el subdominio **Registro de Despliegue** — la memoria del negocio | Supporting |
| el **`StepRecord`** — lo que una ejecución real de un paso dejó escrito | `state/record.go:25` |
| en español, «registro» es además *log*: los `Event` en JSONL | `record` |

**Por qué importa.** El subdominio genérico se llama por **la carpeta que mueve**, no por lo que
promete —*«¿está disponible aquí lo que dejó dicho el último despliegue?»*—, y eso es nombrar una
capacidad de negocio por su implementación. Y «registro» nombra a la vez al contexto que es la
memoria del negocio y a la entrada individual que lo compone.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Renombrar el genérico** por lo que promete (p. ej. *Disponibilidad de la memoria*, *Custodia*), y reservar «estado» para el ciclo de vida | Cada nombre dice qué se promete, no qué carpeta se copia | Toca el nombre de un subdominio que IT-01 cerró; hay que justificar que es lenguaje y no reclasificación |
| **B** | Conservar los nombres y **desambiguar por adjetivo**: *estado del despliegue* vs. *estado de la ejecución* | Coste cero sobre lo cerrado en IT-01 | Los adjetivos se caen al hablar, que es donde 1.7 se comprueba |
| **C** | Retirar «estado» del lenguaje de negocio: lo que el `state/` guarda son **hechos de lo que un paso dejó**, y eso ya tiene nombre | Elimina la palabra más sobrecargada del sistema | «Sincronización de Estado» se queda sin nombre y arrastra a **A** |

**Y para «registro»**, la sub-pregunta: ¿el contexto y la entrada pueden llamarse igual, o la
entrada pasa a llamarse de otra forma —*constancia*, *asiento*, *anotación*—?

---

### Q-02.9 — «Regla»: dos familias sin parentesco, y una tercera que IT-01 creó

**Contexto.** Tres cosas se llaman regla y no se parecen en nada:

| Familia | Qué es | Dónde |
|---|---|---|
| **reglas de re-ejecución** | lo que un paso declara para decidir si hay que rehacerlo (`step/rule.go:122`, `RuleSet`, `MaxAgeRule`, `StateChangedRule`) | Definición → la evalúa Ejecución |
| **reglas de validación estructural** | nueve `*Rule` que comprueban que un pipelinecode está bien formado (`UniqueStepOrderRule`, `StepEntryFormatRule`, `VariableGraphRule`…) | Definición |
| **la regla escrita y versionada** | `DEC-01.4`: qué cuenta como cambio, y que cambiarla obliga a declararlo. *«Lo único con carga de negocio»* del sistema de huellas | sin dueño asignado |

**Por qué importa.** La tercera es una **promesa de negocio de la que depende la comparabilidad
entera** —si cambia sin avisar, todo lo comparado antes deja de ser comparable— y hoy comparte
nombre con dos mecanismos internos. Que la palabra más cargada del modelo sea la misma que la de
un validador de formato es exactamente lo que hace que nadie la trate como contrato.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | Tres nombres distintos: *regla de re-ejecución* · *validación* · **contrato de comparación** (la tercera) | Separa lo que es contrato de lo que es mecanismo · Prepara E4, que tiene que asignarle patrón (duda #10) | Tres términos donde hoy hay uno |
| **B** | «Regla» se queda para lo que un paso **declara**, y lo estructural pasa a llamarse **validación**; la tercera se nombra aparte | Cambio mínimo y alineado con quién la escribe: la regla la escribe el DevOps, la validación la aplica el motor | No resuelve si la tercera es «regla» o algo más fuerte |
| **C** | No tocar: son tres contextos distintos y el homónimo es legítimo | Coste cero | La tercera no está en ningún contexto todavía: es una promesa sin dueño, y sin nombre propio se pierde |

---

### Q-02.10 — «Causa» y «evidencia»: el core las reclamó y el Registro ya las usaba

**Contexto.** `DEC-01.2` fijó tres términos como propios del core: **causa**, **evidencia**,
**reincidencia**. Dos de ellos ya están ocupados en el código, con otro significado:

| Término | Lo que el core quiere que signifique | Lo que ya significa |
|---|---|---|
| **evidencia** | lo que sostiene la atribución: qué ejes cambiaron, qué variables, quién y cuándo | `record.EvidenceRef` (`evidence.go:35`) y `step.EvidenceFact` (`facts.go:86`): **qué registro justificó que un paso reviviera** |
| **causa** | dónde está el origen: código, instrucciones o variables | `record.ErrorClass` (`error_class.go:32`): `command_failed`, `invalid_pipelinecode`, `source_unavailable`, `state_unavailable`, `cancelled`, `unknown` — más `record.Fault` |
| **reincidencia** | cuántas veces se ha intentado. Un hecho, no una lectura | *(libre)* |

**Por qué importa.** Son **dos taxonomías de fallo en dos ámbitos**: la del Registro clasifica la
**circunstancia** (qué salió mal técnicamente); la del core emite una **atribución** (de quién es).
Si comparten nombre, la salida del core acabará leyendo la del Registro como si fuera la suya —
que es precisamente la inferencia que `DEC-01.11` prohíbe.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | El core se queda con **causa** y **evidencia**; el Registro renombra las suyas: `ErrorClass` → *clase de fallo* (circunstancia), `EvidenceRef` → *justificación* | El core es donde va el mejor diseño y el mejor vocabulario; sus tres términos sobrevivieron a siete rondas | Renombra dos tipos y un campo serializado |
| **B** | Homónimo declarado: *evidencia* significa una cosa en Diagnóstico y otra en Registro, y la tabla lo dice | Es lo que el libro autoriza · Cero renombrados | Estos dos contextos se hablan **directamente** —el core lee el Registro—, que es donde el homónimo sí duele |
| **C** | El core cede: usa *prueba* / *sustento* en vez de evidencia, y *atribución* en vez de causa | El código no se toca | Se cambia el vocabulario del core para no molestar a un supporting, que es al revés de la regla del libro |

**Y una sub-pregunta**: ¿*reincidencia* es la palabra? IT-01 la marcó *(nombre propuesto)* y nunca
se validó con un actor.

---

### Q-02.11 — «Huella», «identidad» y «versión»: qué se dice y qué se deja de decir

**Contexto.** `DEC-01.4` decidió tres cosas a la vez: el **algoritmo** de huella no es dominio; la
**regla escrita** sí (→ `Q-02.9`); y las **identidades** que produce *«son conceptos de quien las
usa»* — la del código y la del pipeline en Suministro de Fuentes, la del paso en Ejecución.

La consecuencia lingüística no se escribió: si la identidad pertenece a quien la usa, **«huella»
no es término de ningún ámbito** — cada uno nombra *su* identidad. Hoy en cambio hay `Fingerprint`,
`ContentFingerprint`, `*Material`, y cuatro tokens visibles en disco (`v1:`, `pipe-v1:`, `sf-v1:`,
`cnt-v1:`).

Encima se cruza con **versión**, que nombra tres cosas:

| Sentido | Dónde |
|---|---|
| **versión de producto** — la etiqueta de negocio con la que sale al cliente | Lanzamiento (`DEC-01.13`) |
| **`ProjectVersion`** — lo que el motor calcula hoy del proyecto (`project_version.go:8`) | Ejecución |
| **versión de una regla** — `v1`, `pipe-v1`, `schema_version` | contrato escrito |

**Por qué importa.** Con `DEC-01.13`, la etiqueta de negocio tiene dueño y actor. Lo que el motor
calcula hoy y llama versión **no es** esa etiqueta, y mientras compartan nombre el dueño del
negocio va a creer que ya existe algo que no existe.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | «Huella» **sale** del lenguaje de negocio; cada ámbito nombra su identidad (*identidad del código*, *identidad de la definición del paso*, *identidad de lo desplegado*). «Versión» queda **reservada a Lanzamiento** y lo demás se renombra | Aplica `DEC-01.4` hasta el final · Deja «versión» limpia para el actor que la decide | Renombrado ancho; y los tokens en disco no se pueden renombrar sin invalidar lo escrito |
| **B** | «Huella» se conserva como término **técnico compartido**, declarado explícitamente como *no de negocio* | Coste bajo; la palabra ya está en boca de todos | Un término que aparece en todos los ámbitos y no es de ninguno es la definición de shared kernel accidental — el problema que este plan vino a resolver |
| **C** | «Huella» se conserva **solo dentro de Ejecución** (que es quien decide con ella) y los demás dicen identidad | Compromiso: una palabra, un ámbito | Hay que comprobar que Registro puede decir lo suyo sin usarla |

---

### Q-02.12 — Qué sale del lenguaje: retirados, sinónimos y una línea falsa

**Contexto.** Cierre de limpieza. Cinco asuntos pequeños que, sin decisión, se quedan en el
documento por inercia.

| # | Asunto | Estado |
|---|---|---|
| 1 | **«Plan automático»** | `DEC-01.5` lo retiró. Su verificación es literalmente *«tras IT-02, `lenguaje.md` no lo contiene»*. Falta decidir **qué lo sustituye** en las frases donde estaba: ¿*validación* (Definición), *simulación*, o nada? |
| 2 | **«Espacio de Trabajo»** | Salió del mapa (`DEC-01.8` retirada). ¿Desaparece la palabra, o sobrevive como término interno de Ejecución? |
| 3 | **«Caché»** | El `lenguaje.md` de hoy dice *«no hay caché real que limpiar en ningún otro contexto»*; el código tiene `cache/`, que es un **índice derivable que no participa en ninguna decisión**, y un `gc --cache`. ¿La palabra sale y se llama **índice**? |
| 4 | **Tres verbos para lo mismo** | Un paso que no se rehace se dice hoy de tres formas: **revivir** (código), **omitir/`skipped`** (`SkipReason`) y **«dar por bueno sin rehacer»** (`dominio.md`). Y no son lo mismo: `skipped{no_commands}` es *no había nada que hacer* y revivir es *ya estaba hecho*. Hay que elegir, y separar los dos casos |
| 5 | **Una línea falsa** | *«Simulación — contexto aislado: solo depende de Definición de Pipeline»*. `Q-01.16` la declaró falsa: también depende de Resolución de Variables y de Suministro. Se corrige — la pregunta es si al corregirla hay que nombrar **qué finge** con una palabra propia |

**Por qué importa.** El punto 4 es el que más pesa: `DEC-01.12` dice que *«un paso que no se
re-ejecutó es un agujero en la comparación»*, y el core necesita distinguir **no hacía falta
rehacerlo** de **no había nada que hacer**. Con un solo verbo para los dos, esa distinción no se
puede decir.

---

## 3.bis Rondas de validación

### Ronda 2 — Qué encontró la validación

Doce respuestas en bloque. Ocho cierran limpio y **cuatro mueven el modelo**, una de ellas fuera
de esta iteración:

1. **`Q-02.3` y `Q-02.4` no respondieron a lo que se preguntaba: reformularon el problema.** No
   eligieron entre «manda el modelo» y «manda el código» y ya está — introdujeron **tres cadenas
   encadenadas** (intento → despliegue → lanzamiento), cada eslabón con identificador propio y con
   memoria de su anterior. Es la misma clase de hallazgo que `Q-01.27` en IT-01: la pregunta era
   más pobre que la respuesta.
2. **`Q-02.4` simplifica `DEC-01.2`.** IT-01 le dio al core **dos puntos de referencia**; con las
   cadenas hay **uno solo** —el despliegue— y **dos puertas de entrada**, porque un lanzamiento
   sabe cuál es su despliegue. `dominio.md` tiene que cambiar, y esta iteración nombra la enmienda.
3. **`Q-02.11` no eligió B: eligió B y le cambió el contenido.** «Instantánea» no es un sinónimo
   más cómodo de «huella»: llega con una tríada —instantánea · identificador · versión— que le da
   a la palabra una carga que la opción B negaba explícitamente.
4. **Cuatro sub-preguntas quedaron sin respuesta** porque estaban dentro de opciones que se
   eligieron por otra cosa: el sinónimo «parámetro», las tres palabras de procedencia, el nombre
   del genérico que se queda sin «estado», y si «registro» puede nombrar a la vez al contexto y a
   la entrada.

Once preguntas nuevas, en la misma serie.

---

### Q-02.13 — Español en el código: ¿dónde acaba la frontera?

**Contexto.** `Q-02.2` decidió **todo en español, el lenguaje y el código**. Dentro del motor eso
es una decisión sin ambigüedad. En el borde hay tres capas donde el español deja de ser gratis:

| Capa | Qué es hoy | Quién lo lee |
|---|---|---|
| **el vocabulario serializado** | `attempt_started`, `parameter_resolved`, `step_finished`… escritos en disco | el propio motor, y el portal |
| **los tokens de regla** | `v1:`, `pipe-v1:`, `sf-v1:`, `cnt-v1:` — prefijos que viajan **dentro** de cada identificador | el motor, al recomputar |
| **el contrato con el CLI** | `RequestInput`, `schema_version` — un contrato entre **dos repositorios** | `vex`, que es un sistema externo del mapa |

**Por qué importa.** Los dos primeros son internos y el refactor los reescribe. El tercero **no
es del motor**: es una frontera con un sistema externo, y traducirla unilateralmente no es una
decisión de lenguaje, es un cambio de contrato. El libro tiene nombre para esto —*Published
Language*— y su elección es de E4, pero el **idioma** de esa frontera es de aquí.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | El español llega **hasta el borde del motor**; la frontera con el CLI y el portal se declara **zona de traducción**, y su idioma se decide en E4 con el patrón | Reconoce que un contrato con dos dueños no lo renombra uno solo · No adelanta E4 | Deja una costura de idioma visible |
| **B** | El español llega **también al contrato**: se traduce todo y el CLI se adapta | Un solo idioma en todo el ecosistema | El alcance del plan es *solo* `vex-engine`; obliga a otro repositorio |
| **C** | El español es **de dominio y aplicación**; lo serializado se conserva en inglés como formato | Cero riesgo sobre lo escrito | Reintroduce la traducción que `Q-02.2` quitó, y justo en lo que el usuario ve en `vexd registro …` |

---

### Q-02.14 — «Parámetro» no es un homónimo: es un sinónimo. Y quedan tres palabras para la procedencia

**Contexto.** `Q-02.6` eligió **C** —cuatro sentidos en cuatro ámbitos, declarados y sin
renombrar—. Pero C respondía a los **sentidos**, y en la pregunta había dos cosas más que C no
cubre:

1. **«Parámetro»**: el Registro llama parámetro a lo que todos llaman variable. Eso no es un
   término que signifique dos cosas: son **dos términos para una sola cosa**, y el criterio de
   cierre de esta iteración prohíbe exactamente eso («ni dos términos para lo mismo»).
2. **Tres palabras para la procedencia**: `Origin` (la precedencia: declarada < estado <
   inyectada < runtime), `Provenance` (qué intento la escribió y cuándo) y `VariableSource` (de
   dónde se resuelve una declarada). ¿Tres conceptos, o tres nombres del mismo?

**Por qué importa.** El eje «variables» es uno de los tres del core, y `DEC-01.6` depende de
distinguir el **valor** de la **marca del valor**. Con C, el eje se queda sin nombre propio: hay
que comprobar que el core puede decir *«esta variable cambió»* sin ambigüedad sobre cuál de los
cuatro sentidos cambió.

**Opciones.** Para el sinónimo: **A** «parámetro» se retira y todo es variable · **B** se
conserva y se declara que el Registro habla así · **C** al revés: el Registro tiene razón y lo que
se registra son parámetros. Para la procedencia: **A** tres conceptos, tres nombres · **B** uno
solo con tres facetas · **C** dos: *de dónde vino* y *quién la escribió*.

---

### Q-02.15 — Si «estado» es el Registro, ¿cómo se llama el genérico que lo lleva y lo trae?

**Contexto.** `Q-02.8` respondió que **estado está representado por el subdominio Registro de
Despliegue**. La consecuencia es inmediata: el genérico llamado **Sincronización de Estado** pasa
a nombrarse por una palabra que ya no le pertenece. Y su promesa nunca fue el estado: era
*«¿está disponible aquí lo que dejó dicho el último despliegue?»* — transporte y custodia.

**Por qué importa.** Es el caso que `dominio.md` ya señala en otro sitio: nombrar una capacidad
por la carpeta que copia. Mientras se llame así, cualquiera creerá que ese genérico *sabe* algo
del despliegue, y `DEC-01.7` dice explícitamente que no sabe nada.

**Opciones.** **A** *Sincronización del Registro* — cambio mínimo, hereda el verbo · **B**
*Custodia del Registro* — nombra la promesa (que esté aquí y siga íntegro), no el mecanismo ·
**C** *Disponibilidad del Registro* — nombra el efecto que el core necesita · **D** otro.

**Y una segunda mitad**: `Q-02.3` dice que un intento *«tiene un estado exitoso o fallido»*. Si
«estado» queda absorbido por el Registro, ¿ese estado del intento se sigue llamando estado, o el
intento tiene **resultado** / **desenlace** y «estado» desaparece del lenguaje?

---

### Q-02.16 — «Registro»: ¿el contexto y la entrada pueden llamarse igual?

**Contexto.** Sub-pregunta de `Q-02.8` que quedó sin responder, y que `Q-02.8` agrava: si además
«estado» pasa a decirse «registro», la palabra carga ahora **tres** sentidos —el subdominio
(*Registro de Despliegue*, la memoria del negocio), la **entrada individual** (lo que una
ejecución real de un paso dejó escrito) y, en español, el **diario de hechos** (*log*).

**Por qué importa.** Es el único homónimo del inventario que ocurre **dentro de un mismo ámbito**,
que es lo que el libro sí llama error. Las frases del modelo lo demuestran: *«el registro guarda
un registro por cada paso ejecutado»* es una frase legítima y no se entiende.

**Opciones.** **A** el contexto se queda con «registro» y la entrada se llama **constancia** /
**asiento** / **anotación** · **B** la entrada se queda con «registro» y el contexto se llama
**Memoria del Despliegue** · **C** se aceptan los tres sentidos con adjetivo obligatorio.

---

### Q-02.17 — «Evidencia»: el homónimo declarado se cruza *dentro* de Diagnóstico

**Contexto.** `Q-02.10` eligió **B**: homónimo declarado, *evidencia* significa una cosa en
Diagnóstico y otra en Registro, sin renombrar nada.

**La objeción que la validación tiene que hacer.** El libro autoriza el homónimo **entre**
contextos precisamente porque en la frontera hay traducción. Aquí no la hay en un sentido
concreto: `DEC-01.12` dice que el core **lee el registro de lo que se usó**, así que la
«evidencia» del Registro —qué registro justificó que un paso no se re-ejecutara— **entra en
Diagnóstico como dato**. Y Diagnóstico llama evidencia a otra cosa: lo que sostiene la atribución.
En el momento en que las dos conviven dentro del core, deja de ser un homónimo entre contextos y
pasa a ser el caso que el libro prohíbe.

Nótese que no es hipotético: la evidencia del Registro **es una de las evidencias del core**. Un
paso que no se re-ejecutó es, según `DEC-01.12`, *«un agujero en la comparación»* — o sea, dato de
la atribución.

**Opciones.** **A** se mantiene B y se declara que al cruzar se traduce, nombrando la traducción
· **B** el Registro renombra la suya (*justificación*, *respaldo*) y evidencia queda solo del core
· **C** el core la renombra (*sustento*), contra la regla de que el mejor vocabulario va al core.

**Sub-pregunta pendiente desde `Q-01.14`**: ¿es **reincidencia** la palabra? Nació marcada
*(nombre propuesto)* y nunca se contrastó con un actor. Alternativas: *repetición*, *insistencia*,
*número de intentos*.

---

### Q-02.18 — «Instantánea»: ¿término técnico compartido, o concepto de negocio?

**Contexto.** `Q-02.11` eligió **B** —conservar un término técnico compartido, declarado como *no
de negocio*— pero cambió la palabra y, al hacerlo, le añadió una tríada:

> la **instantánea** es lo capturado; el **identificador** es algo técnico sobre esa instantánea; la
> **versión** es una etiqueta de negocio sobre la misma instantánea.

**La objeción.** Si la versión —que es de negocio, la pone el dueño del negocio y sale al cliente—
es *«una etiqueta sobre la instantánea»*, entonces la instantánea es **la cosa que el negocio
etiqueta**. Un término que el negocio etiqueta no se puede declarar «no de negocio», que es
justamente lo que la opción B afirmaba. La respuesta parece más cercana a un concepto **con dueño**
que a una técnica compartida.

Y hay una segunda consecuencia, sobre una decisión de IT-01: `DEC-01.4` separó tres cosas y dijo
que **las identidades pertenecen a quien las usa** —la del código y la del pipeline a Suministro,
la del paso a Ejecución—. Con «instantánea» como palabra común, la identidad vuelve a tener un
nombre único para todos, que es el punto que `DEC-01.4` evitó. O `DEC-01.4` se enmienda, o la
instantánea es de alguien.

**Opciones.** **A** técnica compartida, declarada, sin dueño (B literal) · **B** concepto con
dueño: la instantánea es de **Suministro de Fuentes** y los demás la consumen · **C** la palabra
es común y lo que tiene dueño es **cada instantánea concreta** (*instantánea del código*,
*instantánea de las instrucciones del paso*), que es lo que `DEC-01.4` decía con otras palabras.

**Y la que quedó suelta de `Q-02.9`**: la tercera regla —*qué cuenta como cambio, y que cambiarla
obliga a declararlo*, la única con carga de negocio del sistema de huellas— sigue sin nombre. Con
la tríada, el candidato natural es **la regla de la instantánea**: qué entra en ella y qué no.
¿Ése es el nombre, o es algo más fuerte —*contrato de comparación*—, dado que si cambia sin avisar
todo lo comparado antes deja de ser comparable?

---

### Q-02.19 — «No se re-ejecuta»: ¿un caso o dos?

**Contexto.** `Q-02.12` fijó el verbo: **re-ejecución de un paso**, y la forma de decir el
negativo: *«no había nada diferente para este paso, por lo tanto no se re-ejecuta»*. La comparación
es **por paso**.

**Lo que queda sin decidir.** Había dos situaciones distintas y la respuesta resuelve una:

| Situación | Hoy se dice | Con la respuesta |
|---|---|---|
| el paso **no cambió** desde la última vez | *revive* | **no se re-ejecuta** ✔ |
| el paso **no tiene nada que hacer** (ninguna tarea declarada) | `skipped{no_commands}` | *(sin decidir)* |

**Por qué importa.** Para el core no son lo mismo: *«no hacía falta rehacerlo»* significa que hay
un resultado anterior válido contra el que comparar; *«no había nada que hacer»* significa que ese
paso **nunca aporta** nada a la comparación. Si se dicen igual, un agujero en la comparación se
confunde con un paso vacío.

**Opciones.** **A** dos expresiones distintas, y la segunda se nombra (*paso sin tareas*) · **B**
una sola: si no hay nada que hacer, tampoco hay nada diferente, y se dice igual · **C** el segundo
caso deja de existir en el lenguaje: un paso sin tareas es un **error de definición**, no un
resultado.

---

### Q-02.20 — El estado de un intento: ¿bastan exitoso y fallido?

**Contexto.** `Q-02.3` dice que un intento *«tiene un estado exitoso o fallido»*. Dos huecos:

1. **El cancelado.** Hoy existe y es terminal (el motor atiende la señal, deshace y deja constancia
   de que se canceló). Con dos valores, una cancelación tiene que declararse fallo — y un fallo es
   dato de la atribución del core, mientras que una cancelación **no es un fallo de nadie**.
2. **El intento exitoso que no es despliegue.** Un intento puede ejecutar *uno o varios* pasos.
   Si ejecuta tres de cinco y los tres salen bien, es **exitoso** y **no** es un despliegue. Esa
   figura no tiene nombre y es el caso más frecuente del día a día del programador.

**Por qué importa.** El punto 2 decide si «exitoso» significa *todos los pasos que se pidieron* o
*todos los pasos del pipeline*. Si es lo segundo, la mayoría de intentos serían fallidos sin haber
fallado nada.

**Opciones.** Para el estado: **A** exitoso · fallido · cancelado · **B** dos, y la cancelación es
un fallo con causa declarada · **C** dos, y la cancelación **no cierra** el intento (queda sin
desenlace). Para el nombre: **A** «exitoso» = *todos los pasos pedidos*, y el despliegue añade
*todos los del pipeline* · **B** un término propio para el intento completo.

---

### Q-02.21 — «Rollback»: ¿nombre español, y dónde cae en las tres cadenas?

**Contexto.** `Q-02.3` dice que un despliegue *«representa un punto de retorno en caso las cosas
salgan mal»*. El `lenguaje.md` viejo definía el rollback por la forma de la cadena: *«un despliegue
que se crea a partir de un despliegue anterior al último»*. Con las cadenas encadenadas, esa
definición ya no se sostiene sola: si cada despliegue sabe cuál fue el anterior, volver atrás
**no rompe la cadena**, la continúa — el despliegue nuevo tiene como anterior el último, y lo que
viene de atrás es su **contenido**, no su posición.

**Por qué importa.** Es la diferencia entre *deshacer* y *avanzar hacia atrás*. Si la cadena se
bifurcara, «el último despliegue» dejaría de ser único y el core perdería su punto de referencia
—el único que le queda tras `Q-02.4`—.

**Opciones.** **A** *volver a un despliegue*: es un intento nuevo cuyo contenido es el de un
despliegue anterior; la cadena no se bifurca y «rollback» sale del lenguaje · **B** se conserva
«rollback» como término propio con esa definición · **C** el término es **punto de retorno** (el
despliegue) y la acción no tiene sustantivo: se dice *volver a*.

---

### Q-02.22 — «Ejecución»: ¿se retira el sustantivo?

**Contexto.** `Q-02.3` define el intento como *«una ejecución de uno o varios pasos»*: ahí
«ejecución» es un acto, no una cosa con identidad. Pero la palabra nombra además un **subdominio**
—Ejecución de Pipeline— y aparece en *variables de ejecución* y *recursos de ejecución*.

**Por qué importa.** Si «ejecución» puede nombrar tanto el acto como la cosa, vuelve por la puerta
de atrás el homónimo que `Q-02.3` acaba de cerrar: alguien dirá «la ejecución» queriendo decir el
intento.

**Opciones.** **A** «ejecución» es **solo acto**, nunca entidad identificable; la entidad es
siempre el intento; el subdominio conserva el nombre porque nombra la actividad · **B** se retira
también del nombre del subdominio, que pasa a llamarse por lo que produce · **C** se conserva como
sinónimo admitido de intento, en contra del criterio de cierre.

---

### Q-02.23 — «Validación» nombra ahora dos momentos: ¿son el mismo?

**Contexto.** `Q-02.9` eligió **B**: lo estructural pasa a llamarse **validación**. Y `Q-02.12`
dice que lo que era «Plan automático» **ahora es validación**. Son dos cosas que ocurren en
momentos distintos:

| | Cuándo | Qué comprueba |
|---|---|---|
| la estructural | al cargar un pipelinecode | que está bien formado: orden de pasos único, formato, grafo de variables |
| la que sustituye al plan automático | antes de cada intento | que la definición y las variables de **este** ambiente están completas y resuelven |

**Por qué importa.** `DEC-01.5` las puso a las dos en Definición de Pipeline, así que no hay
conflicto de ámbito. Pero si son un solo término y dos momentos, hay que poder decir cuál falló;
y si son dos términos, hay que nombrar el segundo.

**Opciones.** **A** una sola validación con dos momentos declarados · **B** dos términos:
*validación* (la forma) y *comprobación previa* (lo de este ambiente) · **C** dos términos con el
segundo nombrado por lo que evita.

---

### Ronda 3 — Qué encontró la validación

Once respuestas. Ocho cierran; tres cambian algo que ya estaba escrito, y una de ellas corrige un
**hecho** del modelo, no un matiz:

1. **`Q-02.21` corrige la forma del objeto.** Lo que se escribió como *cadena* es un **árbol**, y
   el rollback **sí** bifurca. La frase con la que `DEC-02.3` justificaba que «el último despliegue»
   fuera único —*«la cadena no se bifurca»*— era falsa: la unicidad viene de que el **tiempo** es un
   orden total, no de la forma del árbol. Corregido en `DEC-02.3` y `DEC-02.4`. Y al corregirlo
   aparece algo que no se veía: **hay dos relaciones distintas** donde se leía una.
2. **`Q-02.13` no respondió una pregunta: enmendó el proceso.** El código sale del espacio de
   trabajo de las iteraciones estratégicas — no como criterio de peso, sino como **material que no
   se mira**. `DEC-02.10`.
3. **`Q-02.16` y `Q-02.23` renombran, y las dos arrastran una consecuencia que no se escribió**: el
   nombre del subdominio, y el hueco que deja sacar las variables de la comprobación.

Seis preguntas.

---

### Q-02.24 — El árbol se bifurca: son **dos** relaciones, no una. ¿Cuál usa el core?

**Contexto.** `Q-02.3` y `Q-02.4` se leyeron como una secuencia. `Q-02.21` corrige: es un árbol, y
un rollback *«se crea a partir de un despliegue anterior»* que **no** es el último. Entonces
«cuál fue el anterior» nombra dos cosas que en el caso normal coinciden y que un rollback separa:

| Relación | Qué dice | ¿Se bifurca? |
|---|---|---|
| **anterioridad** | cuál ocurrió antes en ese ambiente | **no** — el tiempo es un orden total |
| **procedencia** | de qué despliegue se creó éste | **sí** — en un rollback apunta a uno que no es el último |

Y esto ilumina la definición vieja de rollback —*«un despliegue cuyo padre no es el último»*—: no
era una definición, era **la señal de que había dos relaciones** leídas como una.

**Por qué importa.** El core compara contra «el último despliegue», y tras un rollback las dos
lecturas dan respuestas distintas:

> Producción tiene D1 → D2 → D3, y D3 falla. Se vuelve a **D2**: nace **D4**, cuya *procedencia* es
> D2 y cuya *anterioridad* es D3. Ahora algo falla en D4.
>
> - Contra **el último en el tiempo (D3)**: dice qué cambió respecto de lo que estaba puesto justo
>   antes — y como se volvió atrás, cambió casi todo.
> - Contra **su procedencia (D2)**: dice qué cambió respecto de lo que se quiso restaurar — y si el
>   contenido es el mismo, **no cambió nada en los tres ejes**, que es información valiosísima: la
>   causa no está en ninguno de ellos.

**Opciones.** **A** siempre el último en el tiempo · **B** siempre la procedencia · **C** depende
de la pregunta, y el core **dice contra cuál compara**, que es lo coherente con `DEC-01.11`.

**Y una segunda mitad, para no crear un homónimo nuevo hoy mismo.** «Procedencia» acaba de
usarse en `Q-02.14` para la variable —*de dónde sale su valor*—. Hacen falta dos palabras
distintas: para el despliegue (*procedencia* · *origen* · *de dónde viene*) y para la variable
(*origen* · *fuente* · *procedencia*).

---

### Q-02.25 — Si «registro» es un hecho y «historial» es el conjunto, ¿cómo se llama el subdominio?

**Contexto.** `Q-02.16` fija el corte: **registro** = un hecho concreto; **historial** = todos los
hechos, un conjunto de registros. El subdominio nombra el conjunto y hoy se llama **Registro de
Despliegue** — o sea, con la palabra de la parte.

**Por qué importa.** Es el único homónimo del inventario que ocurría **dentro** de un mismo
ámbito, y la respuesta lo resuelve por dentro pero deja el nombre de fuera sin tocar. Con el corte
escrito, *«el registro guarda un registro por cada hecho»* se dice ahora *«el historial guarda un
registro por cada hecho»*, que sí se entiende.

**Opciones.** **A** **Historial de Despliegues** · **B** se queda *Registro de Despliegue* y se
acepta el desajuste · **C** otro nombre que no use ninguna de las dos palabras.

**Lo que arrastra.** Renombra un subdominio que IT-01 cerró, así que `dominio.md` cambia y la
iteración lo nombra. Y hay que comprobar que el nombre cubre las **tres** cadenas: el historial no
guarda solo despliegues, guarda intentos, despliegues y lanzamientos.

---

### Q-02.26 — La tercera regla: ya sabemos qué es. ¿Cómo se llama?

**Contexto.** La duda de `Q-02.18`, respondida arriba y recogida aquí para que quede escrita.
`DEC-01.4` separó tres cosas: el **algoritmo** (técnica), las **identidades** (de quien las usa) y
en medio una tercera —**qué cuenta como cambio**: qué entra en la instantánea y qué no—. Un
comentario en un fichero, ¿cuenta? ¿El orden en que están escritas las variables? ¿La descripción
de un paso? Cada respuesta es una decisión, y el conjunto **es** la regla.

**No es una validación**, y ésa es la distinción que importa: una validación mira **un** pipeline y
dice si está bien formado; esto mira **dos instantáneas** y decide si son la misma.

**Por qué importa.** `DEC-01.4` la señala como *lo único con carga de negocio* de todo el sistema
de instantáneas, y la razón es dura: si cambia sin avisar, **todo lo comparado antes deja de ser
comparable**, el core sigue diciendo «no cambió nada» y nadie puede notarlo. Por eso lleva versión:
cambiarla obliga a declararlo.

**Opciones.** **A** *regla de la instantánea* — qué entra y qué no · **B** *contrato de
comparación* — nombra lo que promete, no lo que hace, y prepara E4, que tiene que asignarle patrón
de relación (duda diferida **#10** de IT-01) · **C** no es una regla aparte: es **parte de la
definición de instantánea**, y lo único que hay que nombrar es su **versión**.

---

### Q-02.27 — ¿Un paso tiene **tareas** o **comandos**?

**Contexto.** El glosario viejo tenía los dos: *tarea* = «acción a realizar, típicamente un
comando»; *comando* = «comando de Linux o herramienta de terceros». `Q-02.23` acaba de escribir
*«pasos con comandos»*. Dos términos para algo muy próximo es exactamente lo que el criterio de
cierre prohíbe.

**Por qué importa.** `Q-02.19` decidió que un paso **sin nada que hacer** es un error de
definición que la comprobación tiene que atrapar. Para decir ese error hay que poder nombrar lo que
falta, y hoy hay dos nombres.

**Opciones.** **A** dos niveles: un paso tiene **tareas**, y una tarea se realiza **con** un
comando — deja sitio a una tarea que algún día no sea un comando · **B** solo **comandos**: es lo
que el DevOps escribe y lo que ve · **C** solo **tareas**, y «comando» es detalle de cómo se
realiza.

---

### Q-02.28 — Si la comprobación no mira las variables, ¿quién lo hace y cuándo?

**Contexto.** `DEC-01.5` puso en validación *«comprobar definición **y variables** sin ejecutar»*.
`Q-02.23` la reduce: la comprobación es **sobre el pipeline, no sobre el acto de ejecutar**, y por
eso *«variables resueltas y completas no son parte de esta comprobación»*. El grafo de variables sí
—que las referencias apunten a algo declarado—, pero que cada una tenga valor efectivo aquí, no.

**Por qué importa.** Queda un hueco con nombre: *«faltan variables en este ambiente»* es el fallo
más común del DevOps, y ahora mismo no tiene dueño ni momento.

**Opciones.** **A** **nadie antes**: se descubre al intentar, y es un fallo de Resolución de
Variables durante el intento — coherente con que la comprobación sea sobre el pipeline y no sobre
el acto · **B** **Simulación**: interpola de verdad (`DEC-01.5`) y es el sitio donde el DevOps lo
prueba antes de publicar · **C** una comprobación **distinta**, ligada al ambiente, con su propio
nombre y su propio momento.

**Segunda mitad.** ¿Sobrevive la palabra **validación**? `Q-02.12` la puso en el sitio del «plan
automático» y `Q-02.23` la sustituye por *comprobación de la estructura*. O se retira, o queda como
verbo general y entonces hay que decir qué valida.

---

### Q-02.29 — ¿Es «reincidencia» la palabra? Y ¿necesita palabra?

**Contexto.** Pendiente desde `Q-01.14`, donde nació marcada *(nombre propuesto)* y nunca se
contrastó. Es uno de los tres términos propios del core, y con `Q-02.17` los otros dos quedaron en
**causa** y **sustento**.

**Lo que ha cambiado desde entonces.** `DEC-02.3` dice que *«cuántos intentos van desde el
despliegue anterior»* se responde **recorriendo**, y eso es exactamente lo que la reincidencia
mide: cuántas veces se ha intentado sin llegar a despliegue. O sea que el número ya está, y la
pregunta es si el core necesita **una palabra propia** para él o le basta con nombrar el recorrido.

**Por qué importa.** `DEC-01.11` la entrega como hecho y prohíbe la lectura. Un nombre con carga
—*reincidencia* la tiene, suena a juicio— empuja precisamente a la lectura que está prohibida.

**Opciones.** **A** *reincidencia* · **B** *repetición* · **C** *intentos desde el último
despliegue*: no es un término, es el hecho dicho entero, y es imposible de leer como juicio · **D**
otro.

---

### Ronda 4 — Qué encontró la validación

Seis respuestas. Cuatro cierran y **dos corrigen cosas escritas**:

1. **`Q-02.26` deshace la tríada de `DEC-02.8`.** La versión **no** es una etiqueta sobre la
   instantánea: es una etiqueta sobre el **lanzamiento**, y no responde a «¿cambió algo?». Las dos
   cosas se habían fundido porque el glosario viejo decía *«etiqueta asignada a un fingerprint
   cuando ese fingerprint se convierte en lanzamiento»* — una frase que ataba la etiqueta a la
   firma sin motivo. Corregido.
2. **`Q-02.24` disuelve la pregunta en vez de elegir opción.** Anterioridad y procedencia no eran
   dos formas de comparar: **comparar** y **volver atrás** son dos operaciones distintas, y solo la
   primera compara. *«En este caso ya no estamos comparando, estamos creando un despliegue a partir
   de otro.»*
3. **`Q-02.29` deja al core con menos lenguaje propio del que tenía**, y eso toca a la prueba con
   la que IT-01 confirmó que el core existe.

Seis preguntas.

---

### Q-02.30 — ¿*Instantánea* o *firma*? ¿Y hay un identificador aparte?

**Contexto.** `Q-02.26` describe la cosa así: *«técnicamente es un conjunto de caracteres que
representan el contenido de un repositorio, casi como una firma; si el contenido cambia, la firma
también»* — y añade *«tal vez el nombre no es correcto»*.

**Por qué importa.** Las dos palabras no nombran lo mismo. Una **instantánea** es *lo capturado*:
el repositorio tal como estaba. Una **firma** es *la cadena que lo representa*. La descripción de
la respuesta es la de una firma, no la de una instantánea — y de paso deja sin sitio al
«identificador» que `DEC-02.8` puso encima: si la firma **ya es** la cadena con la que se compara,
no hay dos cosas, hay una.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Firma** — un término, y es el que describe lo que hay: la cadena con la que se compara | Dice exactamente lo que es · Elimina el identificador sobrante | «Firma» arrastra connotación de autoría/criptografía en otros contextos |
| **B** | **Instantánea** — se conserva, y la cadena es su forma | Ya está escrito en `DEC-02.8` · «Instantánea del código» se entiende sin explicar | Nombra lo capturado, y lo que se maneja es la cadena |
| **C** | **Dos términos**: la *instantánea* es lo capturado y la *firma* es lo que la representa | Máxima precisión | Dos términos donde solo uno se usa: nadie va a hablar nunca de lo capturado |

---

### Q-02.31 — La regla: qué es, con ejemplos. ¿Es dominio o es técnica?

**Contexto.** Mi explicación anterior era demasiado abstracta. Va concreta.

Tomas una firma del repositorio del pipeline hoy y otra mañana y las comparas para saber si
cambió. Para poder calcularlas, alguien tuvo que decidir **qué se mira**. Estas preguntas tienen
dos respuestas posibles cada una, y ninguna es obvia:

| La decisión | Si es «sí» | Si es «no» |
|---|---|---|
| ¿entra el **nombre** del fichero, o solo su contenido? | renombrar `deploy.sh` a `desplegar.sh` cambia la firma | no la cambia |
| ¿entran los **permisos de ejecución**? | dar `+x` a un script cambia la firma | no |
| ¿entran los ficheros que **git ignora**? | un `.env` local cambia la firma | no |
| ¿entra un cambio de **fin de línea** CRLF→LF? | abrir el repo en Windows cambia la firma | no |
| ¿entra un **comentario** dentro de un fichero de instrucciones? | sí | no |

**El conjunto de esas respuestas es lo que yo llamaba «la tercera regla».** No calcula nada: dice
**qué cuenta como contenido**. Y por eso no es una comprobación: una comprobación mira **un**
pipeline y dice si está bien formado; esto mira **dos firmas** y decide si son la misma cosa.

**Por qué podría ser de negocio.** Dos firmas solo son comparables si se calcularon con las mismas
respuestas. El día que alguien decida que los permisos ahora cuentan, el mismo repositorio sin
tocar da otra firma: el core dirá *«cambió»* sobre algo que nadie tocó. Y peor — todas las
comparaciones anteriores dejan de significar lo mismo y **nada avisa**. La mitad de la promesa que
`DEC-01.1` dice que *se cumple siempre* —*«¿tocó alguien el pipeline?»*— depende de que esas
respuestas no cambien en silencio.

**Por qué podría ser técnica.** Es, literalmente, cómo se calcula una firma. Nadie del negocio va
a hablar jamás de fines de línea.

`DEC-01.4` eligió la primera lectura y la llamó *«lo único con carga de negocio»* de todo el
sistema. La pregunta, ahora con el lenguaje claro:

**Opciones.** **A** es de negocio y necesita nombre —*qué cuenta como cambio*, *contrato de
comparación*— · **B** es técnica, y lo único de negocio es la obligación de **declarar cuándo
cambia**: entonces lo que se nombra no es la regla, es su **versión** · **C** es técnica y punto:
`DEC-01.4` se enmienda y su tercera pieza desaparece del mapa.

---

### Q-02.32 — El core se ha quedado con dos términos. ¿Sigue teniendo lenguaje propio?

**Contexto.** IT-01 confirmó que Diagnóstico es el core con una prueba concreta (`Q-01.14`):
*«nombra tres términos que sean solo de Diagnóstico»*. Salieron **causa**, **evidencia** y
**reincidencia**. Dos de los tres se han ido en esta iteración:

| Término | Qué le pasó |
|---|---|
| **causa** | sigue siendo del core |
| **evidencia** | **cedida** al historial; el core dice ahora **sustento** (`Q-02.17`) |
| **reincidencia** | **retirada**: es la **cantidad de intentos**, y se cuenta recorriendo el historial (`Q-02.29`) |

**Por qué importa.** No es contabilidad: es la prueba con la que se confirmó el core. Si el
lenguaje propio se encoge hasta dos términos y uno de ellos nació hoy para evitar un choque, hay
que mirarlo — IT-01 dejó escrito que si el core sale vacío se vuelve a E1.

**Y hay una lectura que dice que no hay problema**: la prueba pedía tres términos, no un
inventario. El core tiene más lenguaje del que se listó entonces, y está en `DEC-01.11` y
`DEC-01.12` sin haberse recogido como vocabulario:

| Candidato | Qué nombra | ¿Es solo del core? |
|---|---|---|
| **eje** | cada una de las tres cosas que se comparan: código, instrucciones, variables | nadie más los llama así |
| **eliminación** | el procedimiento: descartar ejes hasta que quedan uno o dos | sí |
| **atribución** | lo que el core emite: a qué eje se asigna la causa | sí |
| **deducción** / **inferencia** | la distinción que separa un hecho de una probabilidad | sí, y `DEC-01.11` la hace suya |

**Opciones.** **A** el inventario del core es el de arriba y la prueba de `Q-01.14` se da por
pasada con holgura · **B** hace falta rehacer la prueba con los términos nuevos antes de escribir
el apartado del core · **C** el encogimiento es una señal real y hay que revisar `DEC-01.2` en E6.

---

### Q-02.33 — «Historial de Despliegues»: ¿nombra los tres árboles o solo uno?

**Contexto.** `Q-02.25` acepta renombrar el subdominio a **Historial de Despliegues**. Pero
después de `DEC-02.3`, «despliegue» ya no es una palabra vaga: es **un intento exitoso de todos los
pasos**. Y el historial no guarda solo eso — guarda intentos (la mayoría sin despliegue),
despliegues y lanzamientos.

**Por qué importa.** Es el riesgo que esta iteración existe para evitar: un nombre que, leído con
el vocabulario preciso que acabamos de fijar, dice **menos** de lo que la cosa es. *«Consulta el
historial de despliegues»* sonaría a que los intentos están en otro sitio.

**Opciones.** **A** *Historial de Despliegues*, entendiendo «despliegue» como el **acto** —el
dominio entero es desplegar— y no como el eslabón · **B** **Historial** a secas: guarda todo lo que
pasó y no hace falta calificarlo · **C** un nombre que nombre los tres —*Historial de
Despliegues y Lanzamientos*— · **D** otro.

---

### Q-02.34 — ¿El rollback es solo del último ambiente?

**Contexto.** `Q-02.24` sitúa el rollback ahí: *«cuando hablamos de despliegues en el último
ambiente hay un concepto llamado rollback»*, y en los ambientes anteriores *«ocurre normalmente en
secuencia y funciona casi como una cadena, aunque no siempre»*.

**Por qué importa.** Hay dos cosas que pueden estar pasando y significan modelos distintos:

- **Es exclusivo del último ambiente.** Entonces rollback es un concepto de negocio ligado a que
  ahí hay clientes delante, y en dev o staging elegir otro padre sencillamente no existe.
- **Puede pasar en cualquiera, pero solo tiene nombre ahí.** Entonces la estructura —elegir padre—
  es general, y «rollback» es el nombre que el negocio le da cuando importa.

**Opciones.** **A** exclusivo del último ambiente · **B** la estructura es general y el nombre es
del último ambiente · **C** general, nombre incluido.

**Y una segunda mitad**: si en los ambientes anteriores el padre es siempre el último, entonces
**el padre de un despliegue solo es una elección en el último ambiente** — y eso hay que decirlo,
porque `DEC-02.3` lo escribió como propiedad general.

---

### Q-02.35 — Las dos etiquetas del lanzamiento: ¿dos términos, o uno con dos lecturas?

**Contexto.** `Q-02.26` describe dos etiquetas sobre **el mismo lanzamiento**: una **técnica**, que
llamamos *versión*, y una **de negocio**, que el dueño del negocio llama *nombre*. Y una regla: si
el dueño del negocio no escribe nombre, la herramienta hace que el de negocio sea el técnico.

**Por qué importa.** Es exactamente el caso que `DEC-01.13` describió como *actuar en nombre del
actor ausente*, y el modelo tiene que poder decirlo sin confundir las dos etiquetas: que coincidan
por defecto no las hace la misma cosa, igual que el lanzamiento automático no hace que lanzar sea
parte de desplegar.

**Lo que no se decide aquí.** **Cómo** se deriva la etiqueta técnica es duda diferida **#12** de
IT-01, aparcada en IT-10 por decisión explícita. Aquí solo se decide **cómo se llaman**.

**Opciones.** **A** dos términos: **versión** (técnica) y **nombre** (de negocio) · **B** un
término —*etiqueta*— con dos lecturas según quién la pone · **C** dos términos, pero otros: *nombre
técnico* y *nombre de negocio*.

---

### Ronda 5 — Qué encontró la validación

Seis respuestas, cuatro de ellas confirmaciones. La lupa ya no encuentra problemas: encuentra
**consecuencias mecánicas** de lo decidido, que es la señal que IT-01 describió como el final de la
validación. Dos, y ninguna abre nada nuevo:

1. **`Q-02.33` deja mal el nombre del genérico.** Si el subdominio es **Historial** y un *registro*
   es un hecho suelto, entonces *Sincronización del Registro* dice que se sincroniza **un hecho**.
   Lo que se sincroniza es el historial.
2. **Tres términos del modelo no están en español**, y uno acaba de entrar hoy: *pipeline*,
   *rollback* y *hash*. `DEC-02.2` dice «todo en español» sin haber previsto el caso.

Dos preguntas. Si cierran limpio, la iteración cierra.

---

### Q-02.36 — ¿*Sincronización del Registro* o *Sincronización del Historial*?

**Contexto.** `DEC-02.12` nombró el genérico cuando «registro» todavía estaba en discusión.
Después, `DEC-02.11` fijó que un **registro** es un hecho concreto y el **historial** es el
conjunto, y `Q-02.33` dejó el subdominio en **Historial** a secas.

**Por qué importa.** El genérico no lleva y trae un hecho: lleva y trae **lo que el historial
necesita conservar**, que es justo la promesa que `dominio.md` le atribuye — *«lo que dejó dicho el
último despliegue, ¿está disponible aquí?»*. Con el nombre actual, la frase *«sincroniza el
registro»* se lee como que mueve una entrada.

**Opciones.** **A** *Sincronización del Historial* — coherente con `DEC-02.11`, y dice lo que hace
· **B** se queda *Sincronización del Registro*, entendiendo «registro» como el acto de registrar y
no como la entrada · **C** otro nombre.

---

### Q-02.37 — «Pipeline», «rollback» y «hash»: ¿qué hace `DEC-02.2` con los préstamos?

**Contexto.** `DEC-02.2` decidió **todo en español**, sin capa de traducción. Y el modelo usa tres
palabras que no lo son, una de ellas elegida hoy mismo:

| Término | De dónde viene | ¿Tiene traducción usable? |
|---|---|---|
| **pipeline** | está en el modelo desde el principio, y en el nombre de tres subdominios | *tubería*, *canalización* — ninguna se usa |
| **rollback** | `DEC-02.14`, y es concepto **de negocio**: volver a un estado anterior | *reversión*, *vuelta atrás* — se usan, con menos precisión |
| **hash** | `Q-02.30`, elegido sobre *firma* e *instantánea* | *firma*, *resumen*, *huella* — las tres se descartaron |

**Por qué importa.** No es purismo: es que `DEC-02.2` es **verificable** —«un término de
`lenguaje.md` se busca en el código y aparece con ese nombre»— y una regla verificable con
excepciones no escritas deja de serlo. Además el criterio 1.7 de `DEC-01.10` se comprueba
*hablando con un DevOps*, y un DevOps hispanohablante dice «pipeline» y «rollback» sin traducir:
la excepción probablemente es correcta, pero tiene que estar declarada.

**Opciones.**

| | Qué haría | A favor | En contra |
|---|---|---|---|
| **A** | **Préstamo consolidado**: se conserva la palabra extranjera cuando es la que el actor usa de verdad, y se declara la lista cerrada — *pipeline*, *rollback*, *hash* | Respeta el criterio real: el lenguaje es el que se habla, no el que se traduce · Lista cerrada = sigue siendo verificable | Hay que mantener la lista, y cada término nuevo obliga a decidir |
| **B** | **Se traducen los tres**, y el modelo asume el coste | Regla sin excepciones | *Canalización* no la va a decir nadie; el lenguaje dejaría de ser el que se habla, que es lo único que `DEC-01.10` mide |
| **C** | **No hay regla**: se decide caso por caso | Flexible | `DEC-02.2` deja de ser verificable |

---

## 4. Respuestas y análisis

> Ronda 1 completa: las doce preguntas de apertura. El análisis dice, por cada una, qué gana el
> modelo, qué descarta y qué abre — y lo que abre está en §3.bis.

---

### Q-02.1 — El eje del lenguaje · **A**

**Respuesta.** Un apartado por **subdominio** de IT-01, declarados *contextos candidatos*.

**Análisis.** Es el orden del libro: el lenguaje se descubre y la frontera se deduce de él, no al
revés. La consecuencia operativa es la que hay que sostener: cada homónimo que sobreviva sale con
una frase escrita —*«esta ambigüedad señala la frontera entre X e Y»*— porque ése es el entregable
que E3 consume, y sin él la prueba de frontera de E3 (duda diferida **#9** de IT-01) se queda sin
material.

**Qué descarta.** Organizar por actor, que habría partido Diagnóstico en dos y dejado a los
genéricos sin apartado. Y esperar a E3, que era la opción cómoda y la que rompía el orden.

**Coste aceptado.** Si E3 fusiona dos subdominios, sus dos apartados se funden y algún homónimo
pasa a ser un error dentro de un contexto. Eso **no es un fallo del método**: es exactamente la
señal que E3 busca.

---

### Q-02.2 — Idioma y autoridad · **todo en español, lenguaje y código**

**Respuesta.** El español no es el idioma del documento: es el idioma del sistema. Sin capa de
traducción.

**Análisis.** Es más fuerte que la opción A que se ofrecía —«el modelo en español, el código
traduce»— y por una razón buena: una traducción declarada es una traducción que se puede
incumplir sin que nada falle, y `DEC-01.10` frente 1.7 exige precisamente que el nombre del
modelo y el del código sean el mismo. Con un solo idioma, el criterio 1.7 deja de necesitar
vigilancia: se comprueba leyendo.

**Qué descarta.** El inglés como idioma de nombres, y con él la posibilidad de conservar
`StepRecord`, `EvidenceRef` o `attempt_started` por inercia. Descarta también la opción C
—español arriba, inglés en lo serializado—, que reintroducía la costura justo en lo que el usuario
ve.

**Qué abre.** Dónde acaba el español: lo serializado y los tokens de regla son internos y el
refactor los reescribe, pero el contrato con el CLI tiene **dos dueños** y está fuera del alcance
del plan. → `Q-02.13`.

---

### Q-02.3 — Intento, despliegue y ejecución · **A, y con un modelo de cadenas**

**Respuesta.** Manda el modelo; el código no dirige este diseño y las specs tampoco. Y la
respuesta no se queda en elegir: define la estructura.

| Eslabón | Qué es | Identificador | Sabe | Lo suyo |
|---|---|---|---|---|
| **intento** | una ejecución de uno o varios pasos de un pipeline **en un ambiente** | propio y único | cuál fue **el intento anterior** en ese ambiente | su **estado**: exitoso o fallido |
| **despliegue** | un intento **exitoso de todos los pasos** de un pipeline en un ambiente | propio, **distinto** del identificador del intento | **su intento**, y **el despliegue anterior** | es un **punto de retorno** |

Dos hechos que se derivan de la forma, no se guardan: **cada despliegue tiene un intento, pero no
todo intento tiene despliegue**; y *«cuántos intentos van desde el despliegue anterior»* se
responde **recorriendo la cadena**, no con un contador.

**Análisis — por qué esto es mejor que lo que se preguntaba.** La pregunta ofrecía elegir quién
manda. La respuesta cambia la naturaleza del objeto: el despliegue deja de ser *una etiqueta sobre
un intento* y pasa a ser **un eslabón propio con su propia cadena**. Eso resuelve de golpe tres
cosas que estaban sueltas:

1. **Deja «despliegue» limpio para el core.** *«El último despliegue exitoso»* deja de ser una
   frase con dos lecturas: un despliegue es exitoso por definición, y «el último» es único porque
   la cadena no se bifurca.
2. **Da nombre a lo que el código tenía sin él.** Lo que hoy se deriva antes de ejecutar —una
   posición de contenido en la historia de un ambiente— no es un despliegue: es, como mucho, la
   posición que ese contenido *ocuparía*. Al no ser el mismo objeto, no compiten por el nombre.
3. **Hace derivable lo que hoy es un campo.** El número de reintento era un dato calculado por
   conteo, sin atomicidad, y con la cadena se recorre.

**Qué descarta.** Que el código mande (opción B), y con ella la figura de un «despliegue» que
pudo fallar. Descarta también la opción C —tres términos, uno de ellos *ejecución* como entidad—:
la ejecución queda como **acto**, y la entidad es el intento.

**Qué abre.** Si el intento solo tiene dos estados, ¿dónde cae el cancelado, y cómo se llama el
intento exitoso que ejecutó tres pasos de cinco? → `Q-02.20`. Y «ejecución» sigue nombrando un
subdominio → `Q-02.22`. Y el rollback ya no se puede definir por la forma de la cadena →
`Q-02.21`.

---

### Q-02.4 — «El último resultado exitoso» · **el core compara siempre contra el despliegue**

**Respuesta.** Hay una **tercera cadena** y el core no la usa para comparar.

| Eslabón | Qué es | Sabe | Lo suyo |
|---|---|---|---|
| **lanzamiento** | lo que ocurre **después de un despliegue en el último ambiente** | **su despliegue**, y **el lanzamiento anterior** | su **fecha de publicación** |

Y la regla de comparación queda en una sola:

> **El core compara siempre contra el despliegue.** El lanzamiento es **una consulta más**.

Los actores no se reparten permisos, se reparten **interés**: cualquiera puede consultar
cualquiera de las tres cadenas; al programador y al DevOps les importan los intentos y los
despliegues, y al dueño del negocio los lanzamientos. Mientras el producto no está terminado, el
lanzamiento sencillamente **no ha aparecido todavía**.

**Análisis — esto enmienda `DEC-01.2`.** IT-01 le dio al core **dos puntos de referencia** y lo
escribió como una consecuencia directa de que desplegar y lanzar sean actos distintos. Con las
cadenas, hay **uno solo y dos puertas**: un lanzamiento sabe cuál es su despliegue, así que
preguntar desde el cliente **resuelve** a un despliegue y compara igual que la otra puerta.

No es una contradicción con `DEC-01.13` —desplegar y lanzar siguen siendo dos actos, con dos
dueños—: es que la **separación de actos** no obligaba a **duplicar el mecanismo de comparación**,
y IT-01 lo dio por hecho. Se gana lo que siempre se gana al quitar un caso: el core tiene un solo
procedimiento, y la cadena hace el trabajo que iba a hacer una segunda regla.

**Y desaparece la ambigüedad que motivaba la pregunta.** *«El último resultado exitoso»* nombraba
dos cosas porque había dos referencias. Con una sola, la expresión se sustituye por **el último
despliegue en ese ambiente** y no hace falta prohibir nada: no queda a qué otra cosa referirse.
La duda diferida **#14** de IT-01 se cierra aquí.

**Qué descarta.** El término paraguas (opción B) y el término nuevo parametrizado (opción C):
ambos existían para tapar una duplicidad que la respuesta elimina en vez de tapar.

**Lo que hay que escribir con cuidado.** *«El lanzamiento solo ocurre después de un despliegue en
el último ambiente»* ata Lanzamiento al **orden declarado de los ambientes**, que `Q-01.25` puso
en Definición de Pipeline. Es coherente y hay que decirlo: sin orden declarado, «el último
ambiente» no significa nada.

---

### Q-02.5 — «Paso» · **A**

**Respuesta.** Un solo nombre, cuatro contextos, homónimo declarado en la tabla.

**Análisis.** Es lo que el libro autoriza explícitamente, y es lo que el actor hace: el DevOps
dice «paso» y no va a decir otra cosa. La condición que lo hace legítimo —y que hay que cumplir—
es la de `Q-02.1`: cada uno de los cuatro sentidos sale con **qué frontera señala**, porque un
homónimo entre contextos es información y un homónimo sin frontera declarada es solo ambigüedad.

**Qué descarta.** Los cuatro nombres distintos (opción B), que era vocabulario que solo existiría
en el documento. Y el corte en dos (opción C), que era el compromiso: se descarta porque obligaba
a inventar una segunda palabra teniendo las candidatas ocupadas.

**El riesgo asumido, dicho.** Mientras la frontera no exista en el código, los cuatro sentidos
conviven en dos paquetes y ahí la ambigüedad **sí** es un error. No se resuelve en E2: se resuelve
cuando E3 traza la línea. Queda anotado, no tapado.

---

### Q-02.6 — «Variable» · **C**

**Respuesta.** Cuatro sentidos en cuatro ámbitos distintos, declarados en la tabla. No se
renombra nada.

**Análisis.** Consistente con `Q-02.5`: si el homónimo entre contextos es información, lo es
también aquí, y «variable» es la palabra que usan los tres actores. La coherencia entre las dos
respuestas es lo que evita un lenguaje que trate un caso como legítimo y otro idéntico como
defecto.

**Qué descarta.** El adjetivo obligatorio (opción A) y las palabras distintas por sentido
(opción B). Con ellas se va también la posibilidad de nombrar por separado *el valor* y *la marca
del valor*, que es de lo que depende `DEC-01.6` — y por eso queda comprobación pendiente.

**Qué abre.** C respondía a los **sentidos** y en la pregunta había dos cosas más: «parámetro»,
que es un **sinónimo** y no un homónimo, y las tres palabras de la procedencia. → `Q-02.14`.

---

### Q-02.7 — «Ámbito» · **A**, y con la historia de por qué existe

**Respuesta.** Se conservan **dos** palabras y no se pisan:

| Término | Qué nombra |
|---|---|
| **ambiente** | la separación: dev, staging, producción. Es lo que el DevOps declara y lo que ordena Definición |
| **ámbito** | **qué variables ve un paso** |

Y viene con su origen: las variables se guardaban **por ambiente**, y al ver que en realidad se
separaban por otra cosa nació el ámbito. Ambiente siguió valiendo para lo que siempre valió.

**Análisis.** Es el mejor tipo de respuesta que puede dar un experto de dominio: no elige entre
opciones, **cuenta cómo se descubrió el concepto**. Y eso zanja la ambigüedad mejor que una
definición, porque explica por qué las dos palabras no son intercambiables aunque los valores de
una se nombren con la otra: un ámbito puede *ser* el de un ambiente, y aun así ámbito y ambiente
no son lo mismo — uno es la separación, el otro es lo que un paso alcanza a ver.

**Qué descarta.** Los dos ámbitos con adjetivo (opción B) y el par ámbito/alcance (opción C), que
partía en dos una palabra que el experto usa como una.

**Y cierra la tercera acepción.** El «ámbito» que aparecía en *«un paso no puede acceder a otro
ámbito ni a otro espacio de trabajo»* era **aislamiento**, no ámbito. Con `DEC-01.8` retirada, el
aislamiento es *cómo* Ejecución cumple lo suyo, y se dice con su palabra.

---

### Q-02.8 — «Estado» · **lo representa el Registro de Despliegue**

**Respuesta.** El «estado» —la verdad persistida de lo que un despliegue dejó dicho— **es el
Registro de Despliegue**. No es un concepto aparte.

**Análisis.** Elimina de un golpe el sentido más cargado de la palabra: lo que se guardaba en
`state/` no era «el estado», era **el registro**, y llamarlo estado venía de la carpeta. Que la
verdad tenga un solo nombre es lo que permite que la cadena de garantías de `dominio.md` se lea
sin traducir: *«existe un último despliegue exitoso y se sabe con qué se hizo»* es una frase sobre
el Registro y sobre nada más.

**Qué descarta.** Desambiguar por adjetivo (opción B) — *estado del despliegue* frente a *estado
de la ejecución*—, que dependía de que nadie omitiera el adjetivo al hablar.

**Qué abre, y es lo grande.** El genérico se llama **Sincronización de Estado** con una palabra
que ya no le pertenece, y su promesa nunca fue el estado. → `Q-02.15`. Y queda la sub-pregunta que
no se respondió: si además «registro» nombra al contexto y a la entrada, la palabra carga tres
sentidos **dentro del mismo ámbito**. → `Q-02.16`.

---

### Q-02.9 — «Regla» · **B**

**Respuesta.** «Regla» se queda para lo que un paso **declara** —las reglas de re-ejecución—; lo
estructural pasa a llamarse **validación**.

**Análisis.** El corte es por **quién la escribe**, que es el criterio bueno: la regla la escribe
el DevOps en su pipelinecode; la validación la aplica el motor sobre lo que el DevOps escribió.
Dos actividades con dueño distinto dejan de compartir nombre, y ninguna de las dos pierde nada.

**Qué descarta.** Los tres nombres distintos (opción A) y no tocar nada (opción C).

**Qué queda suelto.** La tercera —*qué cuenta como cambio, y que cambiarla obliga a declararlo*,
que `DEC-01.4` señala como lo único con carga de negocio del sistema de huellas— sigue sin nombre.
Se resuelve junto a la instantánea, porque es la regla **de** la instantánea. → `Q-02.18`.

**Y aparece un solape nuevo**: «validación» nombra ahora también lo que era el plan automático
(`Q-02.12`), que ocurre en otro momento. → `Q-02.23`.

---

### Q-02.10 — «Causa» y «evidencia» · **B**

**Respuesta.** Homónimo declarado: *evidencia* significa una cosa en Diagnóstico y otra en
Registro, y la tabla lo dice. Nada se renombra.

**Análisis.** Coherente con `Q-02.5` y `Q-02.6`: el homónimo entre contextos es información sobre
la frontera. Y aquí la frontera que señala es nítida y valiosa para E3 — la evidencia del Registro
es **circunstancia** (qué justificó no re-ejecutar un paso); la del core es **sustento de una
atribución**. Que la misma palabra sirva para las dos dice exactamente dónde acaba uno y empieza
el otro.

**Qué descarta.** Renombrar en el Registro (opción A) y que ceda el core (opción C).

**La objeción que la validación tiene que hacer, y que no es cosmética.** Estos dos ámbitos no se
hablan a través de una traducción: el core **lee** el registro. En cuanto la evidencia del
Registro entra en Diagnóstico como dato, los dos sentidos conviven dentro del mismo ámbito, que es
el caso que el libro sí llama error. → `Q-02.17`, junto con la sub-pregunta pendiente desde
`Q-01.14`: si **reincidencia** es la palabra.

---

### Q-02.11 — «Huella», identidad y versión · **B, con la palabra cambiada y una tríada**

**Respuesta.** «Huella» se sustituye por **instantánea**, y con ella llegan tres términos que
antes eran uno:

| Término | Qué es | Naturaleza |
|---|---|---|
| **instantánea** | lo que se capturó de algo en un momento | la cosa |
| **identificador** | lo derivado de esa instantánea, con lo que se compara | **técnico** |
| **versión** | una **etiqueta** sobre esa misma instantánea | **de negocio** |

*«El identificador es algo técnico y la versión es algo más de negocio sobre lo mismo.»*

**Análisis.** Es la respuesta que más orden pone. «Huella» describía **el mecanismo** —una marca
que se calcula— y por eso arrastraba el algoritmo a todas las conversaciones. «Instantánea»
describe **lo que se tiene**: el código tal como estaba, las instrucciones tal como estaban. Con
eso, el identificador y la versión dejan de competir: no son dos cosas distintas, son **dos
nombres encima de la misma cosa**, uno para comparar y otro para hablar con el cliente.

Y encaja con lo que ya estaba escrito sin encajar: la *versión de producto* se definía como una
etiqueta asignada *solo cuando algo se convierte en lanzamiento*. Con la tríada eso deja de ser
una regla suelta y pasa a ser lo que es — la versión es la etiqueta de negocio de una instantánea,
y quien la pone es el dueño del negocio, en el lanzamiento.

**Qué descarta.** Que «huella» salga del lenguaje y cada ámbito nombre su identidad sin palabra
común (opción A), y el compromiso de reservarla a Ejecución (opción C).

**Qué abre, y es una objeción de peso.** La opción elegida decía *«término técnico compartido,
declarado como no de negocio»*, y la tríada dice que el negocio **etiqueta** la instantánea. Las
dos cosas no se sostienen a la vez. Además, una palabra común para la identidad roza lo que
`DEC-01.4` evitó al decir que *las identidades pertenecen a quien las usa*. → `Q-02.18`.

---

### Q-02.12 — Lo que sale del lenguaje · **punto por punto**

*(La pregunta no llevaba opciones numeradas; se responde por asunto.)*

| Asunto | Resolución |
|---|---|
| **Plan automático** | Se retira. Lo que hacía **ahora es validación** — y por tanto es de Definición de Pipeline, como fijó `DEC-01.5` |
| **Espacio de Trabajo** | Deja de ser término del mapa y sobrevive como **término interno de Ejecución** |
| **Caché** | **Sale del lenguaje.** Lo que hay es un índice derivable que no participa en ninguna decisión, y el término venía del código — que no dirige este diseño |
| **Los tres verbos** | El verbo es **re-ejecución de un paso**, y la comparación es **por paso**. El negativo se dice por su razón: *«no había nada diferente para este paso, por lo tanto no se re-ejecuta»* |
| **La línea falsa** | **Simulación** conserva su nombre; se corrige la línea que la declaraba «contexto aislado que solo depende de Definición» |

**Análisis.** Los dos primeros y el último son ejecución de decisiones ya tomadas en IT-01. Los
otros dos son decisiones nuevas y las dos van en la misma dirección: **quitar del lenguaje las
palabras que vienen del mecanismo**. «Caché» nombraba una carpeta; «revivir» nombraba lo que le
pasa a un objeto en memoria. Ninguna de las dos era una palabra que un actor use, y las dos
desaparecen sin que el modelo pierda nada que supiera decir.

Y el negativo dicho **por su razón** —*no había nada diferente*— es más fuerte que un verbo: no
nombra un estado del paso, nombra **el hecho que lo justifica**, que es lo único que el core puede
usar. Es la misma disciplina de `DEC-01.11`: hechos, no conclusiones.

**Qué abre.** El caso del paso sin tareas queda sin decidir: *no había nada diferente* y *no había
nada que hacer* no son lo mismo para el core. → `Q-02.19`.

---

### Ronda 2 — Respuestas *(Q-02.13 … Q-02.23)*

---

### Q-02.13 — La frontera del español · **B, y una enmienda al proceso**

**Respuesta.** El español llega **también al contrato**. El alcance es solo `vex-engine` y se
acepta el riesgo de romper hacia fuera. Y con ello, una regla que va más allá de la pregunta:

> El código actual **no dirige este diseño**. Los términos que están en el código **no se toman
> en cuenta**. El código es espacio de la solución y en esta etapa no puede seguir interrumpiendo
> el diseño; hay que enfocarse en el espacio del problema.

**Análisis.** La primera mitad es una decisión de alcance y es coherente con lo ya declarado: el
plan dice desde `plan-ddd.md` §3 que CLI y portal son **sistemas externos**, y un sistema externo
no tiene voto sobre el lenguaje interno de este motor. Lo que cambia es quién paga la traducción:
la paga el borde, no el dominio.

La segunda mitad es más importante que la pregunta que la trajo, y es del mismo tipo que la
enmienda de `Q-01.15`. Aquella sacó a las **specs** del análisis; ésta saca al **código**. Y el
argumento es el mismo, aplicado un nivel más abajo: dejar que lo construido esté presente mientras
se modela hace que lo construido dirija lo que se modela — aunque solo esté ahí «como referencia»,
porque una palabra que ya existe siempre pesa más que una que hay que inventar.

Es además autocrítica del propio archivo: §2.2 de esta iteración se titula *«Lo que dice hoy el
código»* y buena parte del inventario de preguntas se armó con nombres de tipos de Go. Eso ayudó a
**encontrar** los homónimos —y ahí fue legítimo, porque un homónimo se detecta donde se manifiesta—
pero no puede seguir estando presente cuando se **decide** cómo se llaman las cosas.

**Qué descarta.** La zona de traducción en el borde (opción A) y el inglés conservado en lo
serializado (opción C).

**Consecuencia sobre este documento y los siguientes.** → `DEC-02.10`.

---

### Q-02.14 — «Parámetro» y las tres palabras de la procedencia · **A**

**Respuesta.** «Parámetro» **se retira**: todo es **variable**. Y la procedencia son **tres
conceptos, tres nombres**.

**Análisis.** El sinónimo se va sin discusión: dos palabras para una sola cosa es lo único que el
criterio de cierre prohíbe sin matices, y «parámetro» no aportaba ningún matiz — era el nombre que
una parte del sistema le daba a la variable.

Lo de los tres conceptos merece una comprobación, porque al mirarlos **en el espacio del problema**
—como manda `Q-02.13`— uno de los tres se disuelve:

| Concepto | Qué pregunta responde | ¿Sobrevive? |
|---|---|---|
| **de dónde sale el valor** de una variable | ¿es un literal, o se resuelve de algo? | **sí**, es propio de Resolución de Variables |
| **cuál gana** cuando dos sitios dan valor al mismo nombre | la **precedencia** | **sí**, y es el que más consecuencias tiene |
| **quién escribió este registro y cuándo** | qué intento lo dejó | **no hace falta**: `DEC-02.3` ya dice que un registro conoce su intento, y el intento conoce su momento |

O sea: son tres conceptos distintos —la respuesta se sostiene— pero el tercero **ya está nombrado
por el árbol**, y darle término propio sería nombrar dos veces la misma relación.

**Qué descarta.** Conservar «parámetro» declarando que el historial habla así; y fundir los tres
en uno solo con facetas.

**Qué abre.** La palabra para el primero choca con la que `Q-02.21` necesita para el despliegue.
→ `Q-02.24`, segunda mitad.

---

### Q-02.15 — El genérico y el estado del intento · **A, y los dos estados están en contextos distintos**

**Respuesta.** El genérico pasa a llamarse **Sincronización del Registro**. Y sobre el estado del
intento: *«esa definición está en otro contexto, están en contextos separados»*.

**Análisis.** El renombrado quita la última palabra que venía de la carpeta. Lo que ese genérico
promete —que lo que dejó dicho el último despliegue esté disponible aquí— no cambia; lo que cambia
es que ahora el nombre lo dice y no sugiere que sepa algo del despliegue, que es lo que
`DEC-01.7` niega expresamente.

La segunda mitad es la aplicación limpia de `DEC-02.6`: «estado» **sí** puede nombrar dos cosas
—la verdad persistida y el desenlace de un intento— porque están en **ámbitos distintos**, y eso es
un homónimo legítimo que la tabla declara. Lo que no podía era nombrarlas dos veces dentro del
mismo ámbito, y eso ya lo resolvió `Q-02.8` al llevarse la primera al historial.

**Qué descarta.** *Custodia* y *Disponibilidad* como nombres, que nombraban la promesa pero
perdían el verbo que describe lo que hace; y retirar «estado» del todo, que habría dejado al
intento sin palabra para su desenlace.

---

### Q-02.16 — «Registro» · **registro es un hecho; historial es el conjunto**

**Respuesta.**

> El historial total guarda la historia, y **cada item de la historia es un registro de un hecho en
> particular**. Registro es sobre **un hecho**; historial es sobre **todos** los hechos — un
> conjunto de registros.

**Análisis.** Es el corte que ninguna de las tres opciones ofrecía, y es mejor que las tres. Las
opciones peleaban por **quién se queda la palabra** —el contexto o la entrada—; la respuesta dice
que la palabra ya era correcta para la entrada y que **al conjunto le faltaba la suya**. Con
«historial» aparecida, «registro» deja de estar sobrecargada sin que nadie la ceda.

Y encaja con `DEC-02.3` mejor de lo que la pregunta preveía: si un registro es *un hecho*, el
historial es donde viven los tres árboles —intentos, despliegues, lanzamientos— y consultarlos es
recorrer el historial. Las «acciones de consulta» que el glosario viejo agrupaba a mano dejan de
ser una lista y pasan a ser lo que el historial **es**.

**Qué descarta.** Renombrar la entrada a *constancia* o *asiento*; renombrar el contexto a
*Memoria*; y los tres sentidos con adjetivo.

**Qué abre.** El subdominio se llama con la palabra de **la parte**, no la del conjunto. →
`Q-02.25`.

---

### Q-02.17 — «Evidencia» · **C: el core cede la palabra**

**Respuesta.** El core renombra lo suyo: lo que sostiene la atribución se llama **sustento**.
«Evidencia» se queda donde estaba: lo que justificó que un paso no se re-ejecutara.

**Análisis.** Es la opción que la pregunta presentaba como la más débil —*«contra la regla de que
el mejor vocabulario va al core»*— y hay un argumento por el que no lo es: el core es quien **lee**
al historial, así que es el core quien se encuentra las dos palabras juntas. Quien puede evitar la
colisión sin coste es quien la ve, y el historial no la ve nunca.

Y hay una lectura del castellano que lo sostiene: *evidencia* es lo que consta —encaja con un
historial que guarda hechos—, mientras que *sustento* es lo que **sostiene una afirmación**, que es
exactamente lo que el core hace con ella. La palabra que cede no es la mejor palabra: es la que
describía peor lo suyo.

**Enmienda a `DEC-01.2`.** El trío del core pasa a ser **causa · sustento · reincidencia**, y con
ello desaparece el único homónimo del inventario que iba a cruzarse dentro de un mismo ámbito.

**Qué descarta.** Mantener el homónimo declarando la traducción (opción A) —que era declarar una
traducción que nadie iba a hacer— y renombrar en el historial (opción B).

**Qué queda.** Si *reincidencia* es la palabra sigue sin responderse. → `Q-02.29`.

---

### Q-02.18 — «Instantánea» · **C**

**Respuesta.** La palabra es común y **lo que tiene dueño es cada instantánea concreta** — la del
código, la de las instrucciones de un paso, la de lo desplegado.

**Análisis.** Disuelve la objeción sin ceder ninguna de las dos cosas que la provocaban. «No de
negocio» y «el negocio la etiqueta» no eran contradictorias: lo que no es de negocio es **la
técnica de tomarla**; lo que sí lo es, es **cada instantánea concreta**, que pertenece a quien la
usa — que es literalmente lo que `DEC-01.4` dijo con otras palabras al decir que *las identidades
pertenecen a quien las usa*.

Así que `DEC-01.4` **no se enmienda**: se confirma, y ahora con vocabulario. Lo que aporta la
tríada es que la identidad ya no se nombra con la palabra del mecanismo —«huella»— sino con la de
la cosa, y por eso el dueño se ve: *la instantánea del código* dice de quién es; *la huella del
código* no decía nada.

**Qué descarta.** La técnica compartida sin dueño (opción A), que era el shared kernel accidental;
y meter la instantánea entera dentro de Suministro (opción B), que habría hecho que Ejecución
pidiera prestada la identidad de su propio paso.

---

### Q-02.19 — El paso sin tareas · **C: es un error de definición**

**Respuesta.** No es un resultado: es un **error de definición**. El validador del pipeline tiene
que darse cuenta de que hay un paso sin tareas y **no dejar avanzar**.

**Análisis.** Cierra el caso por donde había que cerrarlo: no dándole nombre, sino quitándolo del
conjunto de cosas que pueden pasar. Un paso sin nada que hacer no es una situación que el core
tenga que interpretar — es un pipeline mal escrito, y se detecta antes de que exista un intento.

Y el efecto sobre el core es exactamente el que se buscaba: si el caso no existe, **«no se
re-ejecuta» tiene un solo significado** —*no había nada diferente*— y deja de poder confundirse un
agujero en la comparación con un paso vacío.

**Qué descarta.** Dos expresiones distintas (opción A) y decirlo igual (opción B) — las dos
asumían que el caso era legítimo.

**Qué arrastra.** La comprobación tiene que poder **nombrar lo que falta**, y hoy hay dos palabras
para ello. → `Q-02.27`.

---

### Q-02.20 — El estado del intento · **A, y el intento incompleto no necesita nombre**

**Respuesta.** Tres estados: **exitoso · fallido · cancelado**. Y:

> Un intento exitoso no es un despliegue, es correcto. Si hay 3 intentos de 5 no hace falta ponerle
> nombre: todos son simplemente intentos y nada más. Cuando se ejecutan todos con éxito, ahí sí
> tiene nombre y se llama despliegue. Puedes hacer muchos intentos sin ser despliegue, y los puedes
> consultar como intentos.

**Análisis.** La segunda mitad es la respuesta más económica posible y merece decirse en voz alta:
la pregunta daba por hecho que a la figura frecuente le faltaba nombre, y la respuesta es que **el
nombre ya lo tiene: intento**. Lo que el modelo nombra no es un grado de completitud, es **el
momento en que algo se vuelve un punto de retorno**. Antes de eso, todo es lo mismo.

Eso además protege al despliegue de convertirse en una escala: no hay despliegues parciales, medio
despliegues ni despliegues incompletos. Hay intentos, y hay despliegues.

Y el **cancelado** entra donde tenía que entrar. Un fallo es dato de la atribución del core; una
cancelación **no es un fallo de nadie**, y meterla en «fallido» habría metido ruido justo en la
entrada del core.

**Qué descarta.** Los dos estados con la cancelación declarada fallo; el intento sin desenlace; y
un término propio para el intento completo pero parcial.

---

### Q-02.21 — El rollback · **es un árbol, y sí se bifurca**

**Respuesta.**

> Cuando dije cadena era para representar una secuencia, pero siendo más técnico se comporta más
> como un **árbol**, y en el rollback no necesariamente se sigue la secuencia: se crea a partir de
> un despliegue anterior y **sí se bifurca**.

**Análisis — esto corrige un hecho, no un matiz.** `DEC-02.3` justificaba que «el último
despliegue» fuera único diciendo que *la cadena no se bifurca*. Es falso. Lo que hace único al
último despliegue es que **el tiempo es un orden total**: en un ambiente, dos despliegues nunca
ocurren a la vez, así que siempre hay uno que es el más reciente, se ramifique el árbol como se
ramifique. La conclusión de `DEC-02.4` —un punto de referencia y dos puertas— **se mantiene**; lo
que se cae es el argumento con el que se sostuvo, y se sustituye por el correcto.

**Y aparece lo que la corrección deja ver.** Si el árbol se bifurca, «cuál fue el anterior» nombra
**dos relaciones distintas** que en el caso normal coinciden: la **anterioridad** (cuál ocurrió
antes) y la **procedencia** (de cuál se creó éste). El rollback es el único sitio donde divergen —
y por eso la definición vieja de rollback hablaba del padre: no era una definición, era el síntoma.

**Qué descarta.** Que «rollback» salga del lenguaje por poder decirse como «volver a»: si la
bifurcación es real, el rollback es una **forma del árbol** y no solo una manera de hablar.

**Qué abre.** Con dos relaciones, hay que decir contra cuál compara el core — y las dos dan
respuestas distintas justo después de un rollback. → `Q-02.24`.

---

### Q-02.22 — «Ejecución» · **A**

**Respuesta.** «Ejecución» es **solo acto**, nunca entidad identificable. La entidad es siempre el
intento. El subdominio conserva su nombre porque nombra la actividad.

**Análisis.** Cierra por donde `Q-02.3` había dejado la puerta entreabierta: sin esta respuesta,
alguien diría «la ejecución» queriendo decir el intento y el homónimo volvería por donde acababa de
salir. Y la regla es fácil de comprobar al hablar: si la frase admite «una» o «la», es intento; si
admite «la ejecución **de** algo», es acto.

**Qué descarta.** Renombrar el subdominio, y admitir «ejecución» como sinónimo de intento.

---

### Q-02.23 — «Validación» · **comprobación de la estructura, y solo del pipeline**

**Respuesta.**

> **Comprobación de la estructura antes de un intento**: que esté bien formado — orden de pasos
> único, formato, grafo de variables, pasos con comandos (`Q-02.19`). Es una validación **sobre el
> pipeline, no sobre el acto de ejecutar**; por eso las variables resueltas y completas **no** son
> parte de esta comprobación.

**Análisis.** El criterio que separa es más limpio que el que la pregunta ofrecía. La pregunta
proponía separar por **momento** —al cargar, o antes de cada intento—; la respuesta separa por
**objeto**: lo que se comprueba es el pipeline, y el pipeline es el mismo en todos los ambientes.
Por eso da igual cuándo se haga, y por eso las variables de *este* ambiente caen fuera: no son
propiedades del pipeline, son propiedades de un ambiente concreto.

Eso hace además que la comprobación herede la invariante del core: **las instrucciones no varían
por ambiente**. Una comprobación que mirara las variables de un ambiente dejaría de tener una
respuesta única para el pipeline, y sería una comprobación distinta por ambiente sin decirlo.

**Qué descarta.** Una sola validación con dos momentos, y dos términos donde el segundo se nombra
por el momento en vez de por el objeto.

**Qué abre.** Sacar las variables deja un hueco con nombre —*«faltan variables en este ambiente»*—
sin dueño ni momento. → `Q-02.28`.

---

### Ronda 3 — Respuestas *(Q-02.24 … Q-02.29)*

---

### Q-02.24 — Las dos relaciones · **comparar y volver atrás son dos operaciones, y solo una compara**

**Respuesta.** El árbol es ya lenguaje técnico. Lo que el modelo tiene que saber es otra cosa:

| Operación | Por defecto | ¿Se puede elegir? | Qué es |
|---|---|---|---|
| **comparar** | el **último** despliegue | sí, cualquier otro | una **consulta**: casi siempre se analizan diferencias contra el último, pero nada impide elegir otro |
| **volver atrás** *(rollback)* | el que está **antes del último** | sí, cualquiera anterior al último | una **elección de hacia dónde regresar** — no es una comparación |

Y el rollback, dicho entero: es *«un mecanismo o proceso para volver atrás, entendiéndose como
regresar a un estado anterior desde el punto de vista de negocio»*. Técnicamente crea un despliegue
nuevo cuyo **padre** es uno anterior al último. Con D1 → D2 → D3 y D3 fallando, se crea D4 eligiendo
como padre a D2: entonces **D3 y D4 comparten padre** y deja de haber secuencia.

> *«Pero en este caso ya no estamos comparando, estamos creando un despliegue a partir de otro.»*

**Análisis.** La pregunta ofrecía elegir cuál de las dos relaciones usa el core para comparar, y la
respuesta dice que **la pregunta estaba mal planteada**: la procedencia no es una forma de comparar
que compita con la anterioridad. Es parte de **crear** un despliegue. Las dos relaciones existen
—eso se confirma— pero pertenecen a operaciones distintas, y por eso nunca compiten.

Y con eso se disuelve el escenario que la pregunta pintaba como difícil. Tras el rollback, «el
último despliegue» es D4 y comparar contra él es lo normal; querer comparar contra D2 es
**posible**, pero como una elección del usuario, no como una regla que el core aplique solo.

**Lo que gana el modelo.** La comparación pasa a tener **una forma sola**: *comparar dos
despliegues*, con el último como valor por defecto. Eso es más simple que lo que `DEC-02.4` decía
—un punto de referencia— y más potente, porque el punto de referencia se convierte en un
**parámetro con valor por defecto**. Y encaja con `DEC-01.11` sin esfuerzo: la herramienta muestra
las diferencias entre dos despliegues; cuáles mirar lo decide la persona.

**Qué descarta.** Que el core tenga una regla fija sobre contra qué comparar tras un rollback; y
que «rollback» salga del lenguaje, porque es un concepto **de negocio** —volver a un estado
anterior— y no solo una forma del árbol.

**Qué abre.** Si en los ambientes anteriores al último no hay rollback, el padre elegible es
propiedad del último ambiente y no general. → `Q-02.34`.

---

### Q-02.25 — El nombre del subdominio · **A**

**Respuesta.** *«El historial guarda un registro por cada hecho»*: se cambia el nombre para que
esté alineado. **Historial de Despliegues**.

**Análisis.** Consecuencia directa de `DEC-02.11`: si la palabra de la parte es *registro* y la del
conjunto es *historial*, un subdominio que **es** el conjunto no puede llamarse con la de la parte.
El renombrado no cambia nada de lo que IT-01 decidió sobre él —sigue siendo Supporting, sigue
siendo la memoria del negocio, sigue prometiéndole al core un despliegue con el que comparar—:
cambia la palabra para que diga lo que la cosa es.

**Qué descarta.** Conservar *Registro de Despliegue* aceptando el desajuste.

**Qué abre.** Con «despliegue» ya siendo un término preciso, el nombre podría estar diciendo menos
de lo que el historial guarda. → `Q-02.33`.

---

### Q-02.26 — Instantánea y versión · **son dos cosas distintas, y no estaban relacionadas**

**Respuesta.** Dos aclaraciones separadas.

**La instantánea** está relacionada con cambios en un repositorio —el del proyecto o el del
pipeline—: *«técnicamente es un conjunto de caracteres que representan el contenido de un
repositorio, casi como una firma; si el contenido cambia, la firma también»*. Sirve para **saber si
hubo cambio**.

**La versión** no tiene que ver con eso y **no responde a si algo cambió**. Es una **etiqueta de un
lanzamiento**. Y hay dos, sobre el mismo lanzamiento: la **técnica**, que llamamos *versión*, y la
**de negocio**, que el dueño del negocio llama *nombre*. Cuando el dueño del negocio no escribe un
nombre, la herramienta hace que el de negocio sea el técnico.

**Análisis — esto corrige `DEC-02.8`.** La tríada *instantánea → identificador → versión* era
falsa en su tercer término: la versión no cuelga de la instantánea, cuelga del **lanzamiento**. El
error venía de arrastrar una frase del glosario viejo —*«etiqueta asignada a un fingerprint solo
cuando ese fingerprint se convierte en lanzamiento»*— que ataba las dos cosas sin motivo. Al
separarlas, cada una queda en su sitio y con su dueño: la firma responde *¿cambió?* y sirve al
core; la etiqueta responde *¿cómo se llama esto que sale?* y sirve al dueño del negocio.

Y aparece algo que la tríada tapaba: las **dos etiquetas** de un lanzamiento son dos, no una con
dos formas. Que por defecto coincidan es exactamente lo que `DEC-01.13` llama *actuar en nombre del
actor ausente* — el mismo patrón que el lanzamiento automático, y por la misma razón: un valor por
defecto sobre **quién decide** no funde las dos cosas.

**Qué descarta.** La tríada de `DEC-02.8`, y con ella el «identificador» como término intermedio.

**Qué abre.** Si la cosa es *«casi como una firma»*, ¿el término es *firma* o *instantánea*? →
`Q-02.30`. Y las dos etiquetas necesitan nombre → `Q-02.35`.

---

### Q-02.27 — Tareas o comandos · **B**

**Respuesta.** Solo **comandos**. Un paso tiene comandos.

**Análisis.** Es lo que el DevOps escribe y lo que ve, y «tarea» era un envoltorio que no aportaba
nada: una tarea era *«una acción a realizar, típicamente un comando»*, o sea un comando con una
capa de abstracción por si algún día no lo fuera. El lenguaje no paga por adelantado una
generalidad que nadie usa todavía.

**Y hace decible el error de `Q-02.19`**: un paso sin comandos. Con dos palabras había que elegir
cuál usar en el mensaje; con una, no.

**Qué descarta.** Los dos niveles, y «tarea» como término único.

---

### Q-02.28 — La comprobación · **no sobrevive «validación», y las variables sí se comprueban**

**Respuesta.** *«Validación en este contexto no sobrevive; ahora lo llamamos comprobación.»* Y su
alcance queda escrito, que es lo que faltaba:

| Comprueba | No comprueba |
|---|---|
| el **formato** de lo declarado en el pipeline | nada del **acto de ejecutar** |
| que una variable que aparece en un comando o en un fichero de configuración de un paso **esté declarada** — como salida de un comando de un paso anterior, o en las variables del ambiente | la **interpolación completa**: el valor de algunas variables solo aparece en ejecución |
| que las **expresiones regulares** de las variables de salida de un paso sean correctas | |

Y con ello queda dicha la forma del pipeline: **dos partes** — la declaración de variables (por paso
y por ambiente) y las instrucciones de cada paso (una lista de comandos).

**Análisis.** El hueco que la pregunta señalaba no existía: la comprobación **sí** mira las
variables. Lo que no mira son sus **valores**. Y esa línea es mejor que la que yo había escrito
—«las variables quedan fuera»— porque distingue dos cosas que se parecen: *que una variable esté
disponible* es una propiedad del pipeline, comprobable sin ejecutar nada; *qué vale* es una
propiedad de una ejecución concreta.

La regla completa es fuerte: **toda variable usada tiene que estar declarada, o bien como salida de
un paso anterior, o bien en las variables del ambiente**. Si no, es un error conocible antes de
ejecutar. Eso convierte el pipeline en algo verificable de principio a fin sin tocar la nube, que
es lo que el DevOps quiere antes de publicar.

Y aparece vocabulario nuevo que hay que recoger: **variables de salida** de un paso, declaradas con
una expresión regular — lo que el glosario viejo llamaba *forma de salida esperada*.

**Qué descarta.** Las tres opciones de la pregunta, que repartían el hueco entre Resolución de
Variables, Simulación y una comprobación nueva: no hacía falta repartir nada. Y descarta la palabra
**validación**, que sale del lenguaje.

---

### Q-02.29 — «Reincidencia» · **es la cantidad de intentos**

**Respuesta.** Se llama **cantidad de intentos**, y se obtiene contando cuántos hay entre dos
despliegues.

**Análisis.** Es la opción C de la pregunta y es la que respeta `DEC-01.11` sin esfuerzo: un
número contado entre dos puntos es imposible de leer como juicio, mientras que *reincidencia*
—que suena a expediente— empujaba justo a la lectura prohibida. El core no pierde nada: sigue
entregando el mismo hecho.

**Lo que sí cambia, y hay que mirarlo.** El término deja de ser **del core**: contar intentos entre
dos despliegues es recorrer el **historial**. Sumado a que *evidencia* se cedió en `Q-02.17`, el
trío con el que `Q-01.14` confirmó que Diagnóstico existe se ha quedado en uno de los tres
originales. → `Q-02.32`.

**Qué descarta.** *Reincidencia*, *repetición* e *insistencia*, las tres por lo mismo: nombran una
lectura, no un hecho.

---

### Ronda 4 — Respuestas *(Q-02.30 … Q-02.35)*

---

### Q-02.30 — El término · **hash**

**Respuesta.** Se llama **hash**.

**Análisis.** Descarta las tres opciones de la pregunta y elige una cuarta, y es la que menos
promete: *instantánea* sugiere que se guardó el contenido, *firma* sugiere autoría, y **hash** no
sugiere nada — nombra exactamente lo que hay, una cadena derivada del contenido que cambia si el
contenido cambia. Para un término cuyo único trabajo es responder *¿cambió?*, no prometer de más es
la virtud.

Y confirma lo que `Q-02.26` había dejado implícito: **no hay un identificador aparte**. El hash ya
es la cadena con la que se compara, así que el término intermedio de la tríada desaparece del todo.

**Qué descarta.** *Firma*, *instantánea*, y los dos términos separados para lo capturado y su
representación.

**Qué abre.** No es una palabra española, y `DEC-02.2` no previó el caso. → `Q-02.37`.

---

### Q-02.31 — La regla · **C: es técnica, y `DEC-01.4` se enmienda**

**Respuesta.** Qué entra en el hash y qué no es **técnica**. La tercera pieza de `DEC-01.4`
desaparece del mapa.

**Análisis.** Es la respuesta que menos conserva y hay un argumento fuerte a su favor: el
contenido de esa regla —nombres de fichero, permisos, fines de línea, comentarios— **no es
enunciable en lenguaje de negocio**. Un subdominio se justifica por experto, y aquí no hay experto:
no existe la persona que sepa de despliegues y opine sobre CRLF. `DEC-01.4` la había separado del
algoritmo por su **consecuencia**, no por su contenido — y una consecuencia grave no convierte en
dominio a lo que no lo es. Es el mismo criterio con el que `DEC-01.8` sacó del mapa a Espacio de
Trabajo.

**Pérdida aceptada y nombrada.** El riesgo que motivaba la decisión es real y no desaparece: si esa
regla cambia en silencio, el mismo repositorio sin tocar da otro hash, el core dice *«cambió»* sobre
algo que nadie tocó, y todas las comparaciones anteriores dejan de significar lo mismo **sin que
nada avise**. Lo que cambia es de quién es el problema: pasa a ser responsabilidad de quien
implementa, y el modelo no tiene nada que decir al respecto. Se escribe aquí para que la elección
sea consciente y no un olvido.

**Dos consecuencias de contabilidad.** La duda diferida **#10** de IT-01 —*«si la regla de huella es
contrato versionado, ¿qué patrón de relación le corresponde y quién es su dueño?»*, aparcada en
IT-04— **queda sin objeto**: no hay contrato al que asignar patrón. Y `dominio.md` §*Lo que no es
subdominio* pierde su segunda mitad, la que decía que *«lo que sí es de negocio es que la regla esté
escrita y versionada»*: ya no lo es.

**Qué descarta.** Que sea de negocio con nombre propio; y la lectura intermedia —técnica, pero con
la obligación de declarar cuándo cambia—, que habría dejado media promesa flotando sin dueño.

---

### Q-02.32 — El lenguaje del core · **A**

**Respuesta.** El inventario del core es el completo, y la prueba de `Q-01.14` se da por pasada con
holgura.

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen: en el código, en las instrucciones o en las variables |
| **sustento** | lo que sostiene la atribución |
| **eje** | cada una de las tres cosas que se comparan |
| **eliminación** | el procedimiento: descartar ejes hasta que quedan uno o dos |
| **atribución** | lo que el core emite |
| **deducción** / **inferencia** | la distinción que separa un hecho de una probabilidad |

**Análisis.** Lo que parecía un encogimiento era un **error de conteo**: `Q-01.14` pidió tres
términos como *prueba*, y desde entonces se leyó esa lista de tres como si fuera el inventario. El
core tenía más vocabulario del que se había recogido — estaba escrito en `DEC-01.11` y `DEC-01.12`,
sin recopilar.

Y el inventario real es más fuerte que el original en el punto que importa: **cuatro de los seis
nombran el método**, no el objeto. *Eje*, *eliminación*, *deducción* y *atribución* son palabras
sobre **cómo se razona**, y eso es lo que distingue un ámbito propio de una consulta sobre el ámbito
de otro. Un contexto que solo tuviera nombres de cosas sería una vista; uno que nombra su propio
procedimiento, no.

**Qué descarta.** Rehacer la prueba antes de escribir el apartado; y revisar `DEC-01.2` en E6, que
habría reabierto la clasificación del core sobre un recuento mal hecho.

---

### Q-02.33 — El nombre del subdominio · **B**

**Respuesta.** **Historial**, a secas. Guarda todo lo que pasó y no hace falta calificarlo.

**Análisis.** Evita el problema que la pregunta señalaba sin inventar una enumeración: con
«despliegue» ya siendo un término preciso —*intento exitoso de todos los pasos*—, cualquier nombre
compuesto habría dicho de menos o habría tenido que listar los tres eslabones. *Historial* no dice
de menos: dice exactamente su alcance, que es **todo**.

Y encaja con la forma de los otros nombres del mapa: *Diagnóstico* y *Lanzamiento* también son una
palabra, y también nombran la actividad entera y no su unidad.

**Qué descarta.** *Historial de Despliegues* —que después de `DEC-02.3` se leería como que los
intentos están en otro sitio— y la enumeración de los tres.

**Qué abre.** El genérico se llama *Sincronización del Registro*, y «registro» ya no es el conjunto.
→ `Q-02.36`.

---

### Q-02.34 — El rollback · **C: general, nombre incluido**

**Respuesta.** Volver atrás existe en cualquier ambiente, y se llama **rollback** en todos.

**Análisis.** Es la respuesta que menos casos especiales crea. La alternativa —estructura general,
nombre solo en el último ambiente— habría obligado a que el modelo dijera dos veces lo mismo con
palabras distintas según dónde ocurriera, y a que alguien decidiera dónde acaba una y empieza la
otra.

Y tiene sentido de negocio: en dev y staging también se vuelve a un estado anterior, y también por
la misma razón —lo último no sirve—. Lo que cambia entre ambientes no es el acto sino **quién lo
sufre**: en producción hay clientes delante. Eso es un dato del ambiente, no una diferencia del
concepto.

**Consecuencia sobre `DEC-02.3`**: elegir el padre de un despliegue es una propiedad **general**, no
del último ambiente. La decisión ya estaba escrita así y se confirma.

**Qué descarta.** El rollback exclusivo del último ambiente; y la estructura general con el nombre
reservado.

---

### Q-02.35 — Las dos etiquetas del lanzamiento · **C, con la aclaración**

**Respuesta.** Son dos, sobre el mismo lanzamiento:

| | Qué es | De quién |
|---|---|---|
| **versión** | la **etiqueta técnica**, clave-valor | de la herramienta |
| **nombre del lanzamiento** | el nombre que le quieran poner | del **dueño del negocio** |

Si el actor está ausente, el **nombre del lanzamiento** toma el **valor** de la etiqueta *versión*.

**Análisis.** La aclaración es la que evita el error que la pregunta buscaba: no son «dos nombres
del mismo tipo», son una **etiqueta** y un **nombre**, y solo una de las dos es de negocio. Que por
defecto coincidan en valor no las funde — es el mismo patrón que `DEC-01.13` llamó *actuar en nombre
del actor ausente*, y por la misma razón: un valor por defecto sobre **quién decide** no convierte
dos cosas en una.

Y el detalle *clave-valor* dice algo que no estaba: la versión es **una** etiqueta de un conjunto
posible, no un campo único del lanzamiento. Lo que quede de ahí —qué otras etiquetas hay, cómo se
deriva el valor— es duda diferida **#12** de IT-01, aparcada en IT-10 por decisión explícita, y aquí
no se toca.

**Qué descarta.** Un único término *etiqueta* con dos lecturas; y el par *nombre técnico* / *nombre
de negocio*, que habría hecho de la versión un nombre — que no lo es.

---

### Ronda 5 — Respuestas *(Q-02.36, Q-02.37)*

---

### Q-02.36 — El nombre del genérico · **A**

**Respuesta.** **Sincronización del Historial**.

**Análisis.** Cierra la última palabra desalineada del mapa. El genérico no mueve un hecho suelto:
mueve **lo que el historial necesita conservar**, que es exactamente la promesa que `dominio.md`
le atribuye. Con `DEC-02.11` fijando que un registro es la parte y el historial el conjunto, era la
única lectura que quedaba en pie.

**Qué descarta.** Conservar *Registro* leyéndolo como el acto de registrar — que habría dejado
convivir dos lecturas de la misma palabra en el mismo mapa, justo lo que esta iteración vino a
quitar.

---

### Q-02.37 — Los préstamos · **A**

**Respuesta.** **Préstamo consolidado**, con lista cerrada: **pipeline**, **rollback**, **hash**.

**Análisis.** Es la respuesta que salva `DEC-02.2` en vez de debilitarla, y por el motivo correcto:
la regla nunca fue *traducir*, fue que **el término del modelo y el del código sean el mismo, y sea
el que el actor habla**. Un DevOps hispanohablante dice «pipeline» y «rollback» sin traducir, así
que traducirlos habría roto el criterio 1.7 —el que se comprueba hablando— en nombre de la letra de
`DEC-02.2`.

Y la **lista cerrada** es lo que mantiene la regla verificable: no es «se admiten anglicismos», es
«se admiten estos tres, y cualquier otro exige una decisión».

**Qué descarta.** Traducir los tres —*canalización* no la va a decir nadie— y decidir caso por caso,
que dejaba `DEC-02.2` sin forma de comprobarse.

---

## 5. Decisiones

> **Dieciocho decisiones firmes**, tras cinco rondas y treinta y siete preguntas. Se conservan los
> números originales aunque el contenido haya cambiado, para que las referencias de §3.bis y §4
> sigan siendo legibles.
>
> Tres **enmiendan decisiones de IT-01**: `DEC-02.4` y `DEC-02.16` tocan a `DEC-01.2`, y `DEC-02.18`
> retira una pieza de `DEC-01.4`. Ninguna deroga una decisión entera.

---

### DEC-02.1 — El lenguaje se escribe por subdominio, y cada homónimo declara la frontera que señala

**Contexto.** El lenguaje vive dentro de un bounded context, y los contextos son de E3.

**Decisión.** `modelo/lenguaje.md` se organiza en un apartado por **subdominio** de IT-01,
declarados *contextos candidatos*. Cada término que sobreviva con más de un sentido sale con una
frase que dice **entre qué y qué está la frontera** que ese homónimo señala.

**Consecuencias.** La tabla de homónimos es el **entregable principal** de esta iteración y la
entrada de la prueba de E3 (duda diferida #9). Si E3 fusiona dos subdominios, algún homónimo pasa
a ser un error dentro de un contexto: eso es la señal, no un fallo.

**Alternativas descartadas.** Por actor; y esperar a E3.

**Verificación.** Ninguna entrada de la tabla de homónimos sin su frase de frontera.

---

### DEC-02.2 — El idioma del sistema es el español, sin capa de traducción y hasta el borde

**Contexto.** `DEC-01.10` frente 1.7 exige que el nombre del modelo y el del código sean el mismo.

**Decisión.** Español en el lenguaje **y en el código**, y **también en el contrato** con los
sistemas externos. No hay tabla de traducción: el término del modelo **es** el identificador. El
alcance sigue siendo solo `vex-engine`, y se acepta el riesgo de romper hacia fuera.

**Consecuencias.** El criterio 1.7 deja de necesitar vigilancia: se comprueba leyendo. El coste de
la traducción lo paga el borde, no el dominio — coherente con que CLI y portal sean **sistemas
externos** desde `plan-ddd.md` §3. El patrón de esa frontera sigue siendo materia de E4.

**Alternativas descartadas.** La zona de traducción en el borde; el inglés conservado en lo
serializado; y el español con traducción declarada, que es una regla que se puede incumplir sin
que nada falle.

**Préstamos consolidados, lista cerrada**: **pipeline**, **rollback** y **hash** se conservan sin
traducir, porque son las palabras que el actor habla — y el criterio 1.7 de `DEC-01.10` se comprueba
hablando. La lista es cerrada: cualquier otro préstamo exige una decisión, que es lo que mantiene
la regla verificable.

**Verificación.** Un término de `lenguaje.md` elegido al azar se busca en el código y aparece con
ese nombre.---

### DEC-02.3 — Tres eslabones: intento, despliegue y lanzamiento *(revisada en rondas 2 y 3)*

**Contexto.** «Intento» y «despliegue» significaban cosas opuestas en el modelo y en el código, y
«el último resultado exitoso» nombraba dos cosas.

**Decisión.**

| | Qué es | Sabe | Lo suyo |
|---|---|---|---|
| **intento** | una ejecución de uno o varios pasos de un pipeline en un ambiente | cuál fue el **anterior** | su **estado**: exitoso · fallido · cancelado |
| **despliegue** | un intento **exitoso de todos** los pasos en un ambiente | su intento, y su **padre** | es un **punto de retorno** |
| **lanzamiento** | lo que ocurre tras un despliegue **en el último ambiente** | su despliegue, y el anterior | sus **etiquetas** y su fecha de publicación |

Cada despliegue tiene un intento; **no todo intento tiene despliegue**. Un intento exitoso que no
ejecutó todos los pasos **no tiene nombre propio**: es un intento, y se consulta como tal. *«La
cantidad de intentos»* entre dos despliegues se **cuenta**, no se guarda.

**El padre de un despliegue es normalmente el último, pero puede elegirse uno anterior** — y ahí la
secuencia se rompe: dos despliegues pueden compartir padre. Lo que hace único a «el último
despliegue» **no** es la forma sino que **el tiempo es un orden total**: en un ambiente no hay dos
despliegues a la vez.

**Consecuencias.** «Despliegue» queda limpio para el core y no admite grados: no hay medio
despliegue. «Ejecución» deja de ser entidad (`DEC-02.13`). Los actores se reparten **interés**, no
permiso: cualquiera consulta cualquiera de los tres.

**Alternativas descartadas.** Que mandara el código; *ejecución* como entidad; dos estados con la
cancelación declarada fallo; y un término propio para el intento parcial.

**Elegir el padre es general**, no del último ambiente: volver atrás existe en cualquiera
(`Q-02.34`).

**Verificación.** Ninguna frase del modelo necesita decir «despliegue exitoso»: es redundante.

---

### DEC-02.4 — Comparar es comparar dos despliegues, y el último es el valor por defecto *(revisada)*

**Contexto.** `DEC-01.2` le dio al core **dos puntos de referencia**, uno por cliente. Después, la
bifurcación abrió la duda de contra cuál comparar.

**Decisión.** La comparación tiene **una forma sola**: *comparar dos despliegues del mismo
ambiente*. El **último** es el valor por defecto —es contra lo que casi siempre se analiza un
error— y **el usuario puede elegir otro**. Preguntar desde el cliente no cambia el mecanismo:
un lanzamiento sabe cuál es su despliegue, así que resuelve a la misma operación.

**Consecuencias.** Se **enmienda `DEC-01.2`**: no hay dos puntos de referencia, hay **un parámetro
con valor por defecto**. `DEC-01.13` no se toca — desplegar y lanzar siguen siendo dos actos con
dos dueños; lo que cae es la suposición de que dos actos obligaban a dos mecanismos. Encaja con
`DEC-01.11` sin esfuerzo: la herramienta muestra las diferencias entre dos despliegues, y cuáles
mirar lo elige la persona. Y la duda diferida **#14** de IT-01 se cierra sin prohibir nada.

**Alternativas descartadas.** El término paraguas y el término parametrizado; una regla fija sobre
contra qué comparar tras un rollback.

**Verificación.** Todo escenario de E6 nombra dos despliegues, nunca un lanzamiento.

---

### DEC-02.5 — «Ambiente» y «ámbito» son dos cosas, y ninguna es el aislamiento

**Contexto.** Dos sentidos técnicos y un tercero en el modelo.

**Decisión.** **Ambiente** es la separación —dev, staging, producción— que declara y ordena
Definición de Pipeline. **Ámbito** es **qué variables ve un paso**. El *aislamiento* se dice con su
palabra y no es ninguna de las dos.

**Consecuencias.** Un ámbito puede ser el de un ambiente sin que ámbito y ambiente sean lo mismo.
La frase *«un paso no puede acceder a otro ámbito»* se reescribe como aislamiento, que es lo que
decía.

**Alternativas descartadas.** Dos ámbitos con adjetivo; y partir en ámbito/alcance.

**Verificación.** Ninguna frase del modelo usa «ámbito» para hablar de dev/staging/producción.

---

### DEC-02.6 — «Paso» y «variable» son homónimos declarados; «evidencia» no llegó a serlo *(revisada)*

**Contexto.** Términos con varios sentidos en ámbitos distintos.

**Decisión.** **Paso** y **variable** conservan un solo nombre y sus sentidos se declaran en la
tabla de homónimos con la frontera que señalan. **Estado** también: la verdad persistida (hoy
historial) y el desenlace de un intento viven en ámbitos separados.

**«Evidencia» es la excepción, y por una razón asimétrica**: los dos sentidos iban a encontrarse
**dentro** del core, porque el core lee el historial. Quien ve la colisión es quien puede evitarla,
así que **el core cede la palabra**: lo que sostiene una atribución se llama **sustento**;
*evidencia* se queda con lo que justificó que un paso no se re-ejecutara.

**Consecuencias.** El lenguaje propio del core queda en **causa** y **sustento**: *reincidencia* se
retiró en favor de **cantidad de intentos**, que es un hecho del historial (`DEC-02.9`). Eso
**enmienda la tabla de términos de `DEC-01.2`** y toca a la prueba con la que IT-01 confirmó el
core.

**Alternativas descartadas.** Adjetivo obligatorio; nombres por sentido; declarar una traducción en
la frontera que nadie iba a hacer; y renombrar en el historial.

**El core no se queda corto de lenguaje**: la lista de tres de `Q-01.14` era una *prueba*, no un
inventario, y el inventario completo está en `DEC-02.16`.

**Verificación.** Cada sentido de «paso», «variable» y «estado» aparece en la tabla con su ámbito
y su frontera; «evidencia» aparece una sola vez.

---

### DEC-02.7 — «Regla» es lo que un paso declara; la estructura se **comprueba** *(revisada)*

**Contexto.** Tres familias sin parentesco compartían la palabra «regla», y «validación» quedó
nombrando dos momentos.

**Decisión.** **Regla** = lo que un paso declara para decidir si hay que re-ejecutarlo, y la
escribe el DevOps. **Comprobación** = lo que el motor verifica sobre el pipeline antes de un
intento. La palabra **validación sale del lenguaje**.

Su alcance, escrito:

| Comprueba | No comprueba |
|---|---|
| el **formato** de lo declarado | nada del **acto de ejecutar** |
| que toda variable usada en un comando o en un fichero de configuración de un paso **esté declarada**: como **variable de salida** de un paso anterior, o en las variables del ambiente | la **interpolación completa** — el valor de algunas variables solo aparece en ejecución |
| que las **expresiones regulares** de las variables de salida sean correctas | |
| que ningún paso esté **sin comandos** | |

**Consecuencias.** La línea no es «las variables quedan fuera» sino **los valores quedan fuera**:
que una variable esté *disponible* es una propiedad del pipeline, comprobable sin ejecutar; qué
*vale* es una propiedad de una ejecución. Con eso un pipeline es verificable de principio a fin sin
tocar la nube, que es lo que el DevOps quiere antes de publicar. Y la comprobación hereda la
invariante del core: las instrucciones no varían por ambiente, luego hay una sola respuesta para
todos.

Queda fijada la **forma del pipeline**: dos partes — la declaración de variables (por paso y por
ambiente) y las instrucciones de cada paso (una lista de comandos).

**Alternativas descartadas.** Tres nombres para «regla»; una sola validación con dos momentos;
nombrar el segundo momento por el momento en vez de por el objeto; y repartir la comprobación de
variables entre Resolución de Variables, Simulación o una comprobación nueva.

**La regla que dice qué entra en el hash es técnica** y sale del mapa (`DEC-02.18`).

**Verificación.** Ninguna regla del pipelinecode y ninguna comprobación del motor comparten nombre.

---

### DEC-02.8 — El **hash** responde «¿cambió?», y no tiene nada que ver con la versión *(corregida)*

**Contexto.** «Huella» nombraba el mecanismo y arrastraba el algoritmo a toda conversación; y
«versión» nombraba tres cosas. La primera versión de esta decisión las ató en una tríada
—*instantánea → identificador → versión*— y eso era **falso**: la versión no cuelga de lo que se
compara, cuelga del **lanzamiento**.

**Decisión.** El **hash** es un conjunto de caracteres que representa el contenido de un
repositorio —el del proyecto o el del pipeline—. Si el contenido cambia, cambia. Responde a una
sola pregunta: **¿cambió algo?**

**No hay identificador aparte**: el hash ya es la cadena con la que se compara. Y la palabra es
común, pero **lo que tiene dueño es cada hash concreto** — el del código, el de las instrucciones —
que es lo que `DEC-01.4` decía al escribir que las identidades pertenecen a quien las usa.

**Consecuencias.** «Huella» sale del lenguaje, y con ella el término intermedio de la tríada. La
*versión de producto* del glosario viejo se va entera a `DEC-02.17`, donde le corresponde.

**Alternativas descartadas.** La tríada; *firma* —sugiere autoría— e *instantánea* —sugiere que se
guardó el contenido—; dos términos separados para lo capturado y su representación; y una técnica
compartida sin dueño.

**Verificación.** «Hash» aparece siempre calificado por lo que resume: *el hash del código*, *el
hash del pipeline*.---

### DEC-02.9 — Lo que sale del lenguaje *(revisada)*

**Contexto.** Palabras que venían del mecanismo, sinónimos, y términos que nombraban una lectura.

**Decisión.**

| Sale | Entra | Por qué |
|---|---|---|
| «plan automático» · «validación» | **comprobación** | una comprobación es sobre el pipeline, no sobre el acto |
| «caché» | **índice** | derivable, y no participa en ninguna decisión |
| «revivir» · «omitir» | **re-ejecución de un paso**; el negativo se dice por su razón: *«no había nada diferente para este paso, por lo tanto no se re-ejecuta»* | nombra el hecho que lo justifica, no un estado del paso |
| «parámetro» | **variable** | era un sinónimo, no un matiz |
| «huella» | **firma** *(nombre pendiente)* | nombraba el mecanismo |
| «tarea» | **comando** | envoltorio de una generalidad que nadie usa |
| «reincidencia» | **cantidad de intentos**, contada entre dos despliegues | nombraba una lectura; un número contado no se puede leer como juicio |
| «Espacio de Trabajo» como término del mapa | término **interno de Ejecución** | `DEC-01.8` retirada |

**Simulación** conserva su nombre; se corrige la línea que la declaraba «contexto aislado que solo
depende de Definición»: también depende de Resolución de Variables y de Suministro (`Q-01.16`).

**Consecuencias.** Todas van en la misma dirección: **fuera las palabras que vienen del mecanismo o
que nombran una lectura**. Con `DEC-02.7`, el caso del paso sin comandos desaparece como resultado
posible, así que *«no se re-ejecuta»* tiene un solo significado.

**Alternativas descartadas.** Conservar «caché» y «parámetro» por ser lo que dice el código — que
no dirige este diseño (`DEC-02.10`).

**Verificación.** `lenguaje.md` no contiene ninguna de las palabras de la columna izquierda.

---

### DEC-02.10 — El código sale del espacio de trabajo de las iteraciones estratégicas *(enmienda de proceso)*

**Contexto.** `Q-01.15` sacó del análisis a las **specs**, con el argumento de que dejar que lo
construido se reconcilie con cada decisión hace que lo construido dirija lo que se modela. Esta
iteración demostró que el argumento se aplica un nivel más abajo: §2.2 se titula *«Lo que dice hoy
el código»* y buena parte del inventario de preguntas se armó con nombres de tipos.

**Decisión.** El código es **espacio de la solución** y no es material de las iteraciones
estratégicas (E2–E5). Los términos que existen en el código **no se toman en cuenta** al decidir
cómo se llaman las cosas.

Una sola excepción, acotada: el código puede usarse para **detectar** una ambigüedad —un homónimo
se manifiesta donde se usa— pero no para **resolverla**.

**Consecuencias.** Se enmienda la plantilla de `plan-ddd.md` §6: en las iteraciones estratégicas,
§2 «Punto de partida» dice **solo lo que dice el modelo**, y §7 deja de ser un inventario. La
reconciliación con lo construido —código y specs— es entregable **único de E11**, que es la regla
que `Q-01.15` ya fijó para las specs.

**Alternativas descartadas.** Conservar el código «como referencia», que es donde una palabra que
ya existe siempre pesa más que una que hay que inventar.

**Verificación.** Ninguna decisión de IT-03 a IT-05 cita un identificador de Go como argumento.

---

### DEC-02.11 — «Registro» es un hecho, «historial» es el conjunto, y el subdominio es **Historial** *(revisada)*

**Contexto.** «Registro» nombraba el subdominio, la entrada y el diario — tres sentidos dentro del
mismo ámbito, que es el único caso que el libro llama error.

**Decisión.** Un **registro** es un hecho concreto. El **historial** es el conjunto de todos los
registros; consultar es recorrerlo. Y el subdominio pasa a llamarse **Historial**, a secas:
*Registro de Despliegue* → **Historial**.

**Consecuencias.** «Registro» deja de estar sobrecargada sin que nadie ceda su palabra: lo que
faltaba era el nombre del conjunto. Los tres eslabones —intentos, despliegues, lanzamientos— viven
en el historial, y las «acciones de consulta» dejan de ser una lista escrita a mano para ser lo que
el historial **es**. Nada de lo que IT-01 decidió sobre el subdominio cambia: sigue siendo
Supporting y sigue prometiéndole al core un despliegue con el que comparar.

El nombre sin calificar es deliberado: con «despliegue» ya siendo un término preciso, *Historial de
Despliegues* habría dicho **menos** de lo que el historial guarda.

**Alternativas descartadas.** Renombrar la entrada; *Memoria*; los tres sentidos con adjetivo;
conservar *Registro de Despliegue*; y enumerar los tres eslabones en el nombre.

**Verificación.** La frase *«el historial guarda un registro por cada hecho»* se lee sin ambigüedad.---

### DEC-02.12 — El genérico se llama **Sincronización del Historial**

**Contexto.** Se llamaba *Sincronización de Estado*, con una palabra que ya no le pertenece y que
sugería que sabe algo del despliegue.

**Decisión.** **Sincronización del Historial**: lleva y trae lo que el historial necesita
conservar. No sabe qué hay en producción, no decide y no efectúa nada (`DEC-01.7`).

**Consecuencias.** Desaparece la última palabra del mapa que venía de una carpeta. La promesa no
cambia; el nombre pasa a decirla.

**Alternativas descartadas.** *Custodia* y *Disponibilidad*, que nombraban la promesa y perdían el
verbo; y retirar «estado» del todo, que dejaba al intento sin palabra para su desenlace.

**Por qué no *del Registro***: el genérico no mueve un hecho suelto, mueve el conjunto. Con
`DEC-02.11`, «registro» es la parte.

**Verificación.** Ningún subdominio del mapa se llama por el sitio donde guarda algo.---

### DEC-02.13 — «Ejecución» es acto, nunca entidad

**Contexto.** `DEC-02.3` cerró la entidad en «intento», pero «ejecución» seguía disponible para
nombrarla por la puerta de atrás.

**Decisión.** «Ejecución» nombra **el acto** y nunca una cosa identificable. La entidad es siempre
el **intento**. El subdominio conserva el nombre porque nombra la actividad.

**Consecuencias.** Regla comprobable al hablar: si la frase admite «una» o «la», es un intento; si
admite «la ejecución **de** algo», es un acto. *Variables de ejecución* y *recursos de ejecución*
siguen siendo legítimos: nombran el acto.

**Alternativas descartadas.** Renombrar el subdominio; admitir «ejecución» como sinónimo de intento.

**Verificación.** Ninguna frase del modelo dice «la ejecución» refiriéndose a un intento.

---

### DEC-02.14 — Comparar y volver atrás son dos operaciones distintas *(nueva)*

**Contexto.** Al bifurcarse el árbol de despliegues aparecían dos relaciones —anterioridad y
procedencia— y la duda de cuál usaba el core para comparar.

**Decisión.** No compiten, porque pertenecen a operaciones distintas:

| Operación | Por defecto | Elegible | Qué es |
|---|---|---|---|
| **comparar** | el **último** despliegue | cualquier otro | una consulta: qué difiere entre dos despliegues |
| **volver atrás** *(rollback)* | el que está **antes del último** | cualquiera anterior al último | una **elección de hacia dónde regresar** |

**Rollback** es un concepto **de negocio**: volver a un estado anterior. Técnicamente crea un
despliegue nuevo cuyo padre no es el último, y por eso dos despliegues pueden compartir padre —
*ahí ya no se está comparando, se está creando un despliegue a partir de otro*.

**Consecuencias.** La comparación tiene una forma sola y su referencia es un **parámetro con valor
por defecto**, no una regla. «Rollback» se queda en el lenguaje: no es una forma del árbol, es un
acto con intención.

**Alternativas descartadas.** Que el core aplique una regla fija tras un rollback; y retirar
«rollback» por poder decirse como «volver a».

**No es exclusivo del último ambiente**: volver atrás existe en cualquiera, y se llama rollback en
todos. Lo que cambia entre ambientes no es el acto sino **quién lo sufre** — en producción hay
clientes delante, y eso es un dato del ambiente, no una diferencia del concepto.

**Verificación.** Ninguna frase del modelo usa el padre de un despliegue como referencia de
comparación.

---

### DEC-02.15 — Un paso tiene **comandos** *(nueva)*

**Contexto.** El glosario tenía *tarea* («acción a realizar, típicamente un comando») y *comando*.

**Decisión.** Un paso tiene **comandos**. «Tarea» sale del lenguaje.

**Consecuencias.** Es lo que el DevOps escribe y lo que ve. Y hace decible el error de `DEC-02.7`:
*un paso sin comandos*, sin tener que elegir entre dos palabras para el mensaje.

**Alternativas descartadas.** Los dos niveles —una tarea se realiza con un comando—, que pagaba por
adelantado una generalidad que nadie usa; y «tarea» como término único.

**Verificación.** «Tarea» no aparece en `lenguaje.md`.

---

### DEC-02.16 — El inventario de lenguaje del core *(nueva)*

**Contexto.** `Q-01.14` pidió **tres términos** como prueba de que Diagnóstico es un ámbito y no
una consulta sobre el de otro. Desde entonces esa lista de tres se leyó como si fuera el
inventario, y cuando dos de los tres se movieron en esta iteración pareció que el core se encogía.

**Decisión.** El lenguaje propio del core es:

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen: en el código, en las instrucciones o en las variables |
| **sustento** | lo que sostiene la atribución |
| **eje** | cada una de las tres cosas que se comparan |
| **eliminación** | el procedimiento: descartar ejes hasta que quedan uno o dos |
| **atribución** | lo que el core emite |
| **deducción** / **inferencia** | la distinción que separa un hecho de una probabilidad |

**Consecuencias.** La prueba de `Q-01.14` se da por pasada con holgura, y **`DEC-01.2` no se
reabre**. El inventario es más fuerte que la lista original en lo que importa: **cuatro de los seis
nombran el método**, no el objeto — un contexto que solo tuviera nombres de cosas sería una vista;
uno que nombra su propio procedimiento, no. *Cantidad de intentos* queda fuera a propósito: es un
hecho del historial que el core consume.

**Alternativas descartadas.** Rehacer la prueba antes de escribir el apartado; y revisar `DEC-01.2`
en E6, que habría reabierto la clasificación del core sobre un recuento mal hecho.

**Verificación.** Ningún término de esta tabla aparece en el apartado de otro subdominio.

---

### DEC-02.17 — Un lanzamiento lleva una **versión** y un **nombre** *(nueva)*

**Contexto.** El glosario viejo tenía *versión de producto* como «etiqueta asignada a un fingerprint
cuando se convierte en lanzamiento», una frase que ataba la etiqueta al hash sin motivo.

**Decisión.** Son dos, sobre el mismo lanzamiento:

| | Qué es | De quién |
|---|---|---|
| **versión** | la **etiqueta técnica**, clave-valor | de la herramienta |
| **nombre del lanzamiento** | el nombre que le quieran poner | del **dueño del negocio** |

Si el dueño del negocio está ausente, el **nombre** toma el **valor** de la etiqueta *versión*.

**Consecuencias.** No son dos nombres del mismo tipo: son una **etiqueta** y un **nombre**, y solo
uno de los dos es de negocio. Que por defecto coincidan en valor no los funde — mismo patrón que
`DEC-01.13` llamó *actuar en nombre del actor ausente*. Y *clave-valor* dice que la versión es
**una** etiqueta de un conjunto posible, no un campo único.

**Fuera de alcance aquí.** Qué otras etiquetas hay y cómo se deriva el valor de la versión es duda
diferida **#12** de IT-01, aparcada en IT-10 por decisión explícita.

**Alternativas descartadas.** Un único término *etiqueta* con dos lecturas; y el par *nombre
técnico* / *nombre de negocio*, que habría hecho de la versión un nombre — que no lo es.

**Verificación.** Ninguna frase del modelo usa «versión» para algo que no sea la etiqueta técnica
de un lanzamiento.

---

### DEC-02.18 — Enmienda a `DEC-01.4`: qué entra en el hash es **técnica** *(nueva)*

**Contexto.** `DEC-01.4` separó tres cosas —el algoritmo (técnica), la **regla escrita y
versionada** (*«lo único con carga de negocio»*) y las identidades (de quien las usa)— y dejó la
segunda como promesa sin dueño.

**Decisión.** **Qué entra en el hash y qué no es técnica.** La segunda pieza de `DEC-01.4`
desaparece del mapa; quedan dos: la técnica y las identidades.

**Por qué.** El contenido de esa regla —nombres de fichero, permisos, fines de línea, comentarios—
**no es enunciable en lenguaje de negocio**, y no existe la persona que sepa de despliegues y opine
sobre CRLF. `DEC-01.4` la había separado del algoritmo por su **consecuencia**, no por su
contenido, y una consecuencia grave no convierte en dominio a lo que no lo es — mismo criterio con
el que `DEC-01.8` sacó del mapa a Espacio de Trabajo.

**Pérdida aceptada y nombrada.** El riesgo sigue siendo real: si esa regla cambia en silencio, el
mismo repositorio sin tocar da otro hash, el core dice *«cambió»* sobre algo que nadie tocó, y todas
las comparaciones anteriores dejan de significar lo mismo **sin que nada avise**. Lo que cambia es
de quién es el problema: pasa a ser responsabilidad de quien implementa, y el modelo no tiene nada
que decir. Se escribe para que sea una elección consciente y no un olvido.

**Consecuencias de contabilidad.** La duda diferida **#10** de IT-01 —qué patrón de relación le
corresponde a esa regla y quién es su dueño, aparcada en IT-04— **queda sin objeto**. Y
`dominio.md` §*Lo que no es subdominio* pierde su segunda mitad, la que decía que *«lo que sí es de
negocio es que la regla esté escrita y versionada»*.

**Alternativas descartadas.** Que sea de negocio con nombre propio; y la lectura intermedia
—técnica, con la obligación de declarar cuándo cambia—, que habría dejado media promesa flotando
sin dueño.

**Verificación.** Ninguna fila de la cadena de garantías de `dominio.md` menciona la regla del hash.

---

## 6. Impacto en el modelo

| Documento | Qué cambia | Cuándo |
|---|---|---|
| `modelo/lenguaje.md` | **Reescrito**: un apartado por subdominio arrancando por el core; las reglas del lenguaje; la tabla de **homónimos** con la frontera que señala cada uno; la de **sinónimos**; los **términos retirados**; y los pares que se confunden | **IT-02 — hecho** |
| `modelo/dominio.md` | **Enmiendas a `DEC-01.2`**: la comparación pasa a *dos despliegues, el último por defecto* (`DEC-02.4`) y el lenguaje del core se recoge entero (`DEC-02.16`). **Enmienda a `DEC-01.4`**: se reescribe §*Lo que no es subdominio* (`DEC-02.18`). **Renombrados**: *Registro de Despliegue* → **Historial**, *Sincronización de Estado* → **Sincronización del Historial**. Entran los tres eslabones, el rollback y las dos etiquetas del lanzamiento | **IT-02 — hecho** |
| `plan-ddd.md` | §6: la enmienda de proceso de `DEC-02.10` — el código sale del material de las iteraciones estratégicas, y la plantilla cambia en §2 y §7 | **IT-02 — hecho** |
| `modelo/bounded-contexts.md` | **No se toca** — es IT-03. La tabla de homónimos es su entrada | IT-03 |

**Lo que `lenguaje.md` dice hoy y no decía al abrir la iteración:**

1. **Está partido por ámbito**, no es una lista plana, y arranca por el core — que es donde el
   libro dice que vive el lenguaje.
2. **Los homónimos están declarados con la frontera que señalan.** No son defectos pendientes de
   arreglar: son el material con el que E3 traza la línea.
3. **Hay reglas de idioma escritas y verificables**: español, sin traducción, con tres préstamos en
   lista cerrada.
4. **Nueve términos se han retirado** y cada uno dice por qué. Todos por la misma razón: venían del
   mecanismo o nombraban una lectura.
5. **El modelo tiene los tres eslabones** —intento, despliegue, lanzamiento— que antes eran una
   sola palabra usada de tres formas.

## 7. Impacto en el código

Esta iteración **no toca código**, y por `DEC-02.10` **no lleva inventario**: la reconciliación con
lo construido —código y specs— es entregable **único de E11**, misma regla que `Q-01.15` fijó para
las specs. Lo que hay que saber para hacerla ya está en §5: el lenguaje entero cambia de idioma, y
una docena de términos cambian de nombre o desaparecen.

## 8. Criterio de cierre

- [x] Las **37** preguntas están respondidas o diferidas con razón escrita (12 de apertura + 25 en
      cuatro rondas de validación).
- [x] `modelo/lenguaje.md` está partido por ámbito y arranca por el del core.
- [x] Existe **tabla de homónimos** con, por cada entrada, los significados y **qué frontera
      señala** — que es lo que E3 consume.
- [x] Existe **tabla de sinónimos** con el término que se queda y los que se retiran.
- [x] No queda ningún término con dos significados **dentro** de un mismo ámbito.
- [x] «Plan automático» no aparece (verificación de `DEC-01.5`).
- [x] «El último resultado exitoso» está desambiguado (duda diferida **#14** de IT-01).
- [x] Están escritos *desplegar* vs. *lanzar* y *deducción* vs. *inferencia*.
- [x] Está corregida la línea de Simulación como «contexto aislado».
- [x] Todo término usado en `modelo/dominio.md` está definido, y todo término definido se usa.
- [x] Está decidido el idioma, su alcance y sus excepciones (`DEC-02.2`).
- [x] Las **dos enmiendas a `DEC-01.2`** y la **enmienda a `DEC-01.4`** están escritas en
      `dominio.md`.
- [x] Los **dos renombrados de subdominio** están escritos en `dominio.md`.
- [x] La **enmienda de proceso** (`DEC-02.10`) está escrita en `plan-ddd.md`.
- [x] Tablero de `plan-ddd.md` actualizado.

## 9. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | ¿Cuántos de los cuatro sentidos de **paso** sobreviven como contextos distintos? La tabla de homónimos los declara; decidir si la frontera está ahí es la prueba de E3 | **IT-03** | E2 declara la ambigüedad, E3 traza la línea. Es el reparto que `DEC-02.1` fija |
| 2 | Lo mismo para **variable** (cuatro sentidos) y para **estado** (dos) | **IT-03** | Igual que la anterior |
| 3 | **Rollback** es de Ejecución de Pipeline por ser un acto, pero el *padre* que elige vive en el Historial. ¿Dónde cae la frontera de esa operación? | **IT-03** | Es una relación entre dos contextos: materia de E3 y E4 |
| 4 | El **idioma de la frontera** con CLI y portal: `DEC-02.2` decide que el español llega hasta el contrato, pero el **patrón** de esa relación —OHS, Published Language, Conformist— es otra cosa | **IT-04** | El idioma es de E2; el patrón es de E4 |
| 5 | **Ámbito** se declara en Definición de Pipeline y gobierna en Resolución de Variables. ¿De quién es el término? | **IT-03** | Un término con dos ámbitos plausibles es exactamente lo que E3 resuelve |
| 6 | Qué otras **etiquetas** lleva un lanzamiento y cómo se deriva el valor de la *versión* | **IT-10** | Es la duda **#12** de IT-01, aparcada por decisión explícita; `DEC-02.17` solo fija los nombres |
| 7 | Las cinco `SPEC-*.md` que viven junto al código se leyeron como **Published Language** en `guia-ddd.md` §3. Con `DEC-02.18`, la regla que documentan es técnica: ¿siguen siendo published language de algo? | **IT-04** | La lectura era de E4 y la premisa ha cambiado; conviene que E4 la reevalúe con esto escrito |

### Duda de IT-01 que queda **sin objeto**

La **#10** de IT-01 —*«si la regla de huella es contrato versionado, ¿qué patrón de relación le
corresponde y quién es su dueño?»*, aparcada en IT-04— **desaparece**: `DEC-02.18` decide que no
hay contrato al que asignar patrón. Se anota aquí y no en IT-01, que no se edita.

### Nota de proceso

Cinco rondas, y las cuatro de validación no encontraron lo mismo.

**La ronda 2 encontró un problema de forma**: las respuestas a `Q-02.3` y `Q-02.4` no eligieron
entre las opciones, reformularon el objeto — los tres eslabones no estaban en ninguna de las tres
alternativas que la pregunta ofrecía. Es la tercera vez en este plan que pasa (`Q-01.27` fue la
primera) y ya se puede decir como regla: **cuando la respuesta no cabe en las opciones, la pregunta
estaba peor planteada que el dominio**.

**La ronda 3 encontró un error mío**, no una imprecisión del modelo. `DEC-02.3` justificaba que «el
último despliegue» fuera único diciendo que la cadena no se bifurca, y era falso: lo hace único que
el tiempo sea un orden total. La conclusión sobrevivió al argumento que la sostenía, lo que solo se
descubre mirando el argumento y no la conclusión.

**La ronda 4 encontró un error de lectura acumulado.** `Q-01.14` pidió tres términos como *prueba*
de que el core es un ámbito, y durante cuatro rondas esa lista de tres se manejó como si fuera el
inventario del core — hasta el punto de que dos movimientos legítimos parecieron un encogimiento
que obligaba a revisar `DEC-01.2`. No había tal: el inventario estaba escrito en `DEC-01.11` y
`DEC-01.12` sin recopilar.

**La ronda 5 no encontró problemas: encontró dos consecuencias mecánicas** —un nombre desalineado y
tres préstamos sin regla—. Las dos cerraron en una vuelta. Es el mismo umbral que IT-01 describió:
cuando la validación deja de encontrar problemas y empieza a encontrar confirmaciones, ha
terminado.

**Y una observación sobre la enmienda de `Q-02.13`.** La iteración empezó con un apartado titulado
*«Lo que dice hoy el código»* y con la mitad de las preguntas armadas sobre nombres de tipos de Go.
Eso sirvió para **encontrar** los homónimos —un homónimo se manifiesta donde se usa— pero estaba a
un paso de decidir el lenguaje con el vocabulario que se pretendía sustituir. La enmienda llegó a
tiempo y por iniciativa del experto, no del proceso: el proceso no tenía nada que la impidiera.
Ahora lo tiene (`DEC-02.10`).
