# Identidad de un despliegue — reglas `cnt-v1` y `dep-v1`

> **Estado:** congelada · **Prefijos:** `cnt-v1:` y `dep-v1:` · **Implementación
> de referencia:** `content.go`, `step_content.go` y `deployment_id.go`, en este
> mismo directorio · **Vectores:** §7

Este documento es **normativo**. Define **dos** identidades y la relación entre
ellas:

| Identidad | Pregunta | Cuándo existe | ¿Borrable? |
|---|---|---|---|
| `content_id` | ¿**Qué** se pretende ejecutar? | **antes** de ejecutar | nunca |
| `deployment_id` | ¿En qué **posición** de la historia de este ambiente cae? | **antes** de ejecutar | nunca |

Hay una **tercera** identidad y no vive aquí: `step_fingerprint` responde «¿cambió
el trabajo de este step concreto?» y vive en `cache`/`state`.

> **Regla de independencia.** Ninguna de las tres se calcula a partir de otra de
> una fila distinta. En particular, **la decisión de saltar o ejecutar un step
> nunca consulta `deployment_id` ni `content_id`** — y no podría, porque
> `deployment_id` cambia en cada ejecución nueva al mismo ambiente aunque el step
> no haya cambiado nada. Lo que una regla de re-ejecución ve es
> `step.RuleSubject`, que tiene tres campos y ninguno es un identificador.

Las dos identidades son **permanentes**, y de ahí sale el nivel de rigor de este
documento: `record verify` (spec 22) las recomputa, y el backend (spec 26) tiene
que poder reproducirlas bit a bit.

---

## 1. Vocabulario

| Término | Significado |
|---|---|
| **objeto** | `Content`: la intención congelada de una operación |
| **material** | la forma canónica del objeto, sobre la que se hashea |
| **operación** | el step *pedido* (`deploy`), no los steps internos |
| **linaje** | la cadena de despliegues de un proyecto sobre un ambiente |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`), `H(x)` el SHA-256 de `x` en hexadecimal minúsculo, `LF` el
byte `U+000A` y `FS` el byte `U+001E`.

## 2. Las capas de versión

| Capa | Token | Documento |
|---|---|---|
| huella del árbol (proyecto y pipelinecode) | `v1:` | `fingerprint/SPEC-v1.md` |
| huella de la declaración de un step | `pipe-v1:` | `fingerprint/SPEC-PIPELINE-v1.md` |
| composición del objeto | `cnt-v1:` | **este**, §5 |
| derivación de la posición | `dep-v1:` | **este**, §6 |

Los cuatro versionados son **independientes**. Un `content_id` puede saltar a v2
sin que ninguna huella se mueva, y una huella puede saltar a v2 sin que la regla
de composición cambie ni un byte. Leerlos como si fueran el mismo es el error que
la spec 17 §8 cierra por escrito, y por eso **`ContentID` no reutiliza el value
object `fingerprint.Fingerprint`**: la disciplina se copia, el tipo no.

**Se compone siempre sobre las formas canónicas COMPLETAS —con prefijo—, nunca
sobre los hashes pelados.** De ahí sale la propiedad que hace barato el
versionado: un salto a v2 de cualquier regla invalida todo lo derivado sin una
línea de código extra. Aquí el argumento es más fuerte que en la huella de un step,
porque el resultado es permanente: si se compusiera sobre el hash pelado, el
mismo árbol identificado con la regla v1 y con la v2 daría el **mismo**
`content_id` — dos objetos distintos con la misma identidad, que es el único
fallo que un registro direccionado por contenido no puede permitirse.

## 3. El material del objeto

### 3.1 Las seis dimensiones

| # | Campo | Qué es | Obligatorio |
|---|---|---|---|
| 1 | `subject` | url del proyecto | sí |
| 2 | `operation` | el step **pedido** | sí |
| 3 | `destination` | el ambiente | sí |
| 4 | `source.project` | huella `v1` del árbol del código | sí |
| 5 | `source.pipeline` | huella `v1` del árbol del pipelinecode | sí |
| 6 | `format` | versión del formato del pipelinecode, y si se declaró | sí |
| 7 | `steps` | material de los steps de la operación, ≥ 1 | sí |

**Ninguno es anulable por omisión.** Un material incompleto no produce un
identificador degradado: produce un **error**, y la validación está en el
constructor para que `ID()` pueda ser total. La razón es la misma que en
`cache.Material` y aquí es más grave: una identidad con un hueco es perfectamente
válida y **colisiona** con la de cualquier otro material al que le falte lo mismo.
En el caché esa colisión cuesta un paso saltado; aquí cuesta dos despliegues
distintos con la misma identidad, para siempre.

### 3.2 Lo que NO entra, y es la mitad del valor

```
timestamp    actor    runner    parent
```

Sin esa exclusión el direccionamiento por contenido no existe: dos ejecuciones
idénticas darían identidades distintas y toda comparación se caería. La exclusión
la sostiene el **tipo** —`Content` no tiene dónde ponerlos—: el actor y el runner
viajan en `attempt_started` y el padre en `deployment_id`.

**Tampoco entra el commit.** El HEAD de cada clon es metadato del objeto: dos
árboles idénticos con distinto commit de origen —rebase, cherry-pick, dos clones
de remotos distintos— deben tener el mismo `content_id`, porque si no la
comparabilidad entre organizaciones desaparece. La huella identifica; el commit
documenta.

**Y no entra ningún valor resuelto.** En la identidad entra la **declaración** de
cómo se resuelve un parámetro, nunca su valor. Es la precondición dura de todo el
registro: un valor de runtime no se puede saber por adelantado, y la declaración
de cómo obtenerlo sí — que es lo que permite calcular la identidad **antes** de
ejecutar.

### 3.3 El material de un step

| # | Campo | Qué es | Obligatorio |
|---|---|---|---|
| 1 | `step_id` | nombre del **directorio**, CON su prefijo de orden | sí |
| 2 | `scope` | el `scope` del `config.yaml`, o `""` si no hay archivo | condicional |
| 3 | `rules` | forma canónica de las reglas declaradas, o `""` | condicional |
| 4 | `declaration` | huella `pipe-v1` de lo que el step declara | sí |
| 5 | `parameters` | las declaraciones de variables, ordenadas por nombre | sí (puede ser 0) |

`scope` y `rules` van vacíos **juntos** y sólo en un caso: el step no tiene
`config.yaml`. Un `config.yaml` presente siempre declara ámbito —lo exige
`step.NewStepConfig`—, así que `scope = ""` con `rules ≠ ""` no es producible. Es
lo que hace distinguibles «no hay `config.yaml`» y «hay uno que no declara
reglas», que son dos hechos con consecuencias distintas.

**`step_id` lleva el prefijo de orden**, así que renumerar un step cambia el
`content_id`. Es consecuencia directa de que la identidad de un step sea su ruta
—la misma decisión que en `state.Key`—, y el precio es una re-ejecución, nunca un
despliegue omitido.

**`parameters` lleva TODAS las declaraciones, no sólo las que declaran fuente.**
Es la decisión que la spec 17 dejó abierta y se cierra por el lado conservador. Un
literal que no entrara no entraría por ninguna otra vía, y dos pipelinecode que
sólo difieren en el valor de un literal declaran ejecutar cosas distintas.

> **Desde la spec 27, `scope`, `rules` y `parameters` están DOS veces en el
> material: como campos de esta regla y dentro de `pipe-v1`.** Es inofensivo para
> el hash y se conserva a propósito: quitarlos sería un `cnt-v2` —otra generación
> de identidades permanentes huérfanas— y son lo que `record show` imprime, así
> que el objeto tiene que poder responder «¿qué declaraba este step?» sin
> recomputar ninguna regla ni rehidratar nada.

Un parámetro **declarado dos veces** es un error y no «gana el último»: la forma
canónica ordena por nombre, así que cuál gana dependería del orden de lectura del
archivo y la identidad dejaría de ser reproducible.

### 3.4 El sub-bloque independiente del destino

Los steps que declaran `scope: project` (spec 13) tienen un material que **no
depende de `Destination`**. Es la primera vez que `Content` tiene una parte así, y
queda declarado para que nadie meta el ambiente ahí más adelante: son steps
completos —no fases de otro—, y su contenido es el mismo se despliegue a `sand` o
a `prod`. `Content.ProjectScopedSteps()` existe para que la propiedad sea
comprobable desde fuera.

El **objeto entero** sí depende del destino, y debe: `destination` es la
dimensión 3.

### 3.5 Los órdenes: cuál entra y cuál no

| Secuencia | ¿El orden entra? | Por qué |
|---|---|---|
| `steps` | **sí**, el de ejecución | ejecutar `01-test` antes que `02-supply` es parte de lo que se pretende hacer |
| `rules` | **sí**, el declarado | es el orden de evaluación, y decide qué motivo se emite |
| `parameters` | **no**, se ordena por nombre | son un mapa en el archivo: el orden de lectura es circunstancia del parser |
| fuentes de `state_changed` | **no**, se normaliza | `[pipeline, project]` y `[project, pipeline]` declaran lo mismo |

## 4. Lo que el objeto declara sobre sí mismo: `format`

Un pipelinecode sin `vexpipeline.yaml` no declara orígenes, así que una variable
producida por otro step aparece en su material como si no viniera de ninguna
parte. El objeto se compone igual y se emite **marcado** (`IsComplete() == false`,
spec 18 §5.5): «este material está incompleto» es un hecho; omitir el objeto sería
perder la historia, y emitirlo como completo sería mentir.

`format` entra en el hash, y no es opcional que lo haga: si no entrara, un
pipelinecode que sube de la v1 a la v2 sin tocar ninguna otra línea produciría el
mismo `content_id` con distinta marca de completitud — dos objetos con la misma
identidad y afirmaciones distintas sobre ella.

## 5. La regla `cnt-v1`

### 5.1 El material canónico

Líneas unidas con `LF`, **sin salto final**:

```
vex-content/cnt-v1
Q(subject)
Q(operation)
Q(destination)
Q(source.project)      ← la forma canónica COMPLETA, "v1:<hash>"
Q(source.pipeline)     ← "v1:<hash>"
Q(format.schema_version)   ← el entero en decimal, entrecomillado
Q(format.declared)         ← "true" o "false", entrecomillado
<número de steps>          ← decimal, SIN comillas
<bloque del step 1>
<bloque del step 2>
…
```

- La primera línea es una **cabecera fija**. Existe para que un `content_id` no
  pueda coincidir con el hash de otra cosa que se le parezca.
- El orden de las ocho líneas de cabecera es **fijo**. Permutar dos campos es
  cambiar la regla.
- El **número de steps** no hace falta para la inyectividad —las comillas ya la
  garantizan— y va porque permite reimplementar la regla sin adivinar dónde
  termina el bloque. Es el único valor sin comillas, y lo produce el motor: no
  puede contener un separador.
- `Q` escapa **todo** carácter de control, incluidos `LF` y `FS`: ningún valor
  puede simular un salto de línea ni un separador de campo, así que la regla es
  inyectiva por construcción.

### 5.2 El bloque de un step

Una línea de cabecera y una línea por parámetro:

```
Q(step_id) FS Q(scope) FS Q(rules) FS Q(declaration) FS <número de parámetros>
Q(nombre del parámetro 1) FS Q(declaración canónica del parámetro 1)
Q(nombre del parámetro 2) FS Q(declaración canónica del parámetro 2)
…
```

`declaration` va por su forma canónica completa (`"pipe-v1:<hash>"`).

La **declaración canónica** de un parámetro es `VariableDeclaration.Canonical()`
(spec 14 §5.3), que se calcula una vez y se comparte con `pipe-v1`:

| Origen | Forma |
|---|---|
| literal | `Q(value)` |
| `resolve: step-output` | `step-output` FS `Q(from)` FS `Q(key)` |
| `resolve: state` | `state` FS `Q(scope)` FS `Q(key)` |

No colisionan: la forma del literal empieza por comilla y las otras dos por su
token.

La forma canónica de `rules` es `RuleSet.Canonical()`: las reglas **en su orden
declarado**, unidas por `U+001F`, cada una como `<kind>` FS `<parametrización
entrecomillada>`.

| Regla | Parametrización |
|---|---|
| `state_changed` | `"pipeline"` o `"pipeline,project"` |
| `max_age` | la duración normalizada por `time.Duration` (`"1h0m0s"`) |

Los dos niveles usan separadores distintos a propósito: con el mismo, un conjunto
de dos reglas y una sola regla con dos campos podrían producir la misma cadena.

### 5.3 La composición y la forma externa

`content_id = H(material canónico)`, y su representación externa:

```
cnt-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo. Dos identidades de
versiones distintas **nunca** son iguales, aunque el hash coincida.

## 6. La regla `dep-v1`

Tres líneas unidas con `LF`, sin salto final:

```
vex-deployment/dep-v1
Q(content_id)      ← la forma canónica COMPLETA, "cnt-v1:<hash>"
Q(parent)          ← la forma canónica COMPLETA, o "" en la raíz del linaje
```

`deployment_id = H(ese material)`, con forma externa `dep-v1:<64 hex>`.

- **La raíz no es un hueco**: un linaje sin cabeza aporta la cadena vacía, que
  ninguna identidad real puede producir, así que el primer despliegue de un
  ambiente no puede colisionar con ninguno derivado.
- **El mismo contenido dos veces produce dos posiciones distintas**, porque la
  segunda cuelga de la primera. Es la idea de git robada sin su motor (spec 17 §4,
  A7), y es exactamente la diferencia entre las dos identidades.
- **Dos ambientes son dos linajes** sin necesidad de meter el ambiente aquí: ya
  viaja dentro del `content_id`.
- Un `content_id` cero **no deriva nada**: devuelve error. El boceto de la spec
  escribe la función como total; se implementa con error por la regla de §3.1.

## 7. Vectores

Están ejecutados en `content_test.go` (`TestContentID_VectoresCongelados`) y
escritos como literales en `vectors_test.go`. Las huellas del material base son
literales fijos —no calculados— para que este documento se pueda validar sin
reimplementar antes las reglas de huella.

```
subject             = "https://vex.test/acme/demo-app"
operation           = "deploy"
destination         = "sand"
source.project      = "v1:" + "3" repetido 64 veces
source.pipeline     = "v1:" + "4" repetido 64 veces
format              = schema_version 2, declarado

step 1: step_id "01-test",   sin config.yaml,
        declaration "pipe-v1:" + "1" repetido 64 veces, sin parámetros
step 2: step_id "02-supply", scope "project", rules [state_changed: [pipeline]],
        declaration "pipe-v1:" + "2" repetido 64 veces,
        parámetros: acr_name = literal "vexacr"
                    image    = resolve step-output, from "01-test", key "image"
```

| # | Material | Identidad |
|---|---|---|
| 1 | el base | `cnt-v1:c1d0dd8880053c2c7829dd84bd9a24351c7f476aa718c77850180b7aede249e1` |
| 2 | el base con `destination = "prod"` | `cnt-v1:e11db527e332d2b191d95d1610d03463a95209382b31548e247406f7f9d7303d` |
| 3 | el objeto 1 sin padre | `dep-v1:8cfd173977dceb8ac0708882cc7069eacd412f6cea266cc3e67dfd59b9892c00` |
| 4 | el objeto 1 colgando de 3 | `dep-v1:15305711e5c548f5d875362e6fca527d9ca39396813e94d592db5b4393d8bc28` |

La forma canónica completa del vector 1 está escrita línea a línea en
`vectors_test.go`, con sus separadores explícitos: una regla que sólo se puede
comprobar por su hash no se puede depurar cuando dos implementaciones discrepan.

> El test es la autoridad; si esta tabla y el test discrepan, discrepan los dos.

## 8. Cómo validar una implementación independiente

1. Reproducir los cuatro vectores de §7, y la forma canónica literal del primero.
2. Comprobar que cambiar **cualquiera** de las seis dimensiones cambia el
   `content_id`. Son seis comprobaciones, no una.
3. Comprobar que un material al que le falte cualquiera produce un **error**, no
   una identidad.
4. Comprobar que cambiar sólo el **prefijo de versión** de una huella, dejando su
   hash igual, cambia el `content_id` (§2).
5. Comprobar las exclusiones: dos objetos que difieren sólo en el instante, el
   actor, el runner o el padre producen el **mismo** `content_id`. En esta
   implementación la comprobación es estructural —no hay dónde ponerlos—; en otra
   puede no serlo.
6. Comprobar que dos ejecuciones que resuelven valores distintos para la misma
   declaración producen el **mismo** `content_id`, y que dos declaraciones que
   difieren en su `from` producen identidades distintas.
7. Comprobar que dos objetos que sólo difieren en `destination` tienen el mismo
   material en su sub-bloque de ámbito de proyecto (§3.4).
8. Comprobar que reordenar los parámetros no mueve la identidad y que reordenar
   los steps o las reglas sí (§3.5).
9. Comprobar que componer la misma identidad dos veces da el mismo valor, y que
   componerla no escribe nada en ninguna parte.

## 9. Defecto heredado, no resuelto aquí

**El `content_id` no es del todo reproducible entre máquinas, y no por culpa de
esta regla.** Si el proyecto es un *worktree* de git o un submódulo, `.git` es un
**archivo** cuyo contenido es `gitdir: <ruta absoluta de la máquina>`, y la huella
del árbol lo incluye porque sólo excluye `.git` cuando es un directorio
(`fingerprint/SPEC-v1.md` §6, 08 §9.10).

En el caché eso cuesta una re-ejecución de más. Aquí duele más: `content_id` es
**permanente** y `record verify` (spec 22) lo recomputa. La corrección es una
**v2 de la regla del árbol**, y debería estar decidida antes de emitir el primer
`content_id` — o sea, antes de implementar la spec 18.

## 10. Historia

| Versión | Cambio |
|---|---|
| `cnt-v1` / `dep-v1` | Primeras reglas (spec 17). No sustituyen a nada: hasta aquí no había ninguna identidad de despliegue, sólo entradas de caché sobrescribibles y líneas de log descartables |

> **La spec 27 no cambia estas reglas y aun así cambia todos los `content_id`.**
> El campo 4 del material de un step pasa de transportar una `inst-v1` a
> transportar una `pipe-v1`, y como entra por su forma canónica COMPLETA, la
> identidad se mueve sin que este documento cambie un byte de §5. Es exactamente
> la propiedad que §2 compra, ejercida por primera vez — y su precio, anunciado
> por la spec 18 §9.12 y ya contraído: los objetos emitidos antes son permanentes
> e incomparables con los de después. Los vectores de §9 cambian de valor por lo
> mismo: cambian sus ENTRADAS, no la regla.
>
> Un lector nuevo sigue pudiendo VERIFICAR un objeto viejo: `verify` no recomputa
> las huellas, las hashea como dato, así que sólo necesita reconocer `inst-v1`
> como token legítimo. La lista de tokens que un binario sabe leer dejó de ser la
> de los que sabe calcular.
