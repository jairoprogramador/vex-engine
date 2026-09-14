# Huella de la declaración de un step — regla `pipe-v1`

> **Estado:** congelada · **Prefijo:** `pipe-v1:` · **Implementación de
> referencia:** `declaration.go`, en este mismo directorio · **Vectores:** §6

Este documento es **normativo**, con la misma disciplina que `SPEC-v1.md` (la
huella del árbol). Responde a una sola pregunta:

> **¿Qué declara este step hacer?**

Tiene respuesta sin saber nada del proyecto que se despliega, sin saber a qué
ambiente va y sin saber dónde se guarda el resultado. Es la mitad estable de la
identidad de un step (`sf-v1`, `SPEC-STEP-v1.md`) y es lo que `content_id`
consume (spec 17).

Si el código y este documento discrepan, **discrepan los dos**: uno de los dos
tiene un bug y hay que decidir cuál. Lo que no se puede hacer es cambiar la regla
y dejar el prefijo en `pipe-v1:`.

---

## 1. Por qué existe esta regla

Sustituye a `inst-v1` (comandos declarados) y a `vars-v1` (variables del paso),
que la spec 27 retira. No es una regla más: donde había **dos** respuestas que
alguien tenía que acordarse de componer —y un tercio del material que no estaba
en ninguna de las dos— hay **una**.

Lo que las dos anteriores no miraban, y esta sí:

| Qué faltaba | Consecuencia que producía |
|---|---|
| el cuerpo de las **plantillas** (`templates:` aportaba las rutas, no el contenido) | editar `steps/02-supply/k8s/deployment.yaml` y volver a ejecutar hacía que el step **se saltara**: el despliegue no ocurría y el motor decía «sin cambios» (D14) |
| los **archivos auxiliares que nadie declaró** — un `.tf` bajo `steps/02-supply/terraform/` | lo mismo, sin nombre hasta la spec 27 |
| `config.yaml` — el `scope` (spec 13) y las `rules` (spec 15) | cambiar las reglas de re-ejecución de un step no movía su huella, y el step se saltaba **con las reglas viejas** |

Y lo que `vars-v1` miraba de más: el **mapa acumulado resuelto**, que incluye las
salidas del propio step. Como se cargan antes de decidir, todo step con un
`outputs` se re-ejecutaba exactamente una vez de más. Aquí el material es la
**declaración**, así que el valor producido en runtime no entra en absoluto.

## 2. Vocabulario

| Término | Significado |
|---|---|
| **step** | un directorio `steps/NN-<nombre>/` del pipelinecode |
| **archivos de declaración** | los tres archivos que el motor sabe interpretar: `commands.yaml`, `config.yaml` y `variables/<ambiente>/<paso>.yaml` |
| **directorio del step** | `steps/NN-<nombre>/`, la raíz del árbol de §3.4 |
| **comando** | una entrada de la lista de `commands.yaml` |
| **declaración de variable** | una entrada de `variables/<ambiente>/<paso>.yaml`, sin resolver |
| **entrada** | la línea con la que un elemento participa en el material |
| **campo** | uno de los valores que componen una entrada |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`): comillas dobles alrededor, `\` y `"` escapados, y **todo
carácter de control escapado** — en particular `\n` (U+000A) y `\x1e` (U+001E).

Sea `H(x)` el SHA-256 de `x` en hexadecimal minúsculo (64 caracteres).

## 3. La regla

El material son **líneas**, unidas por `\n` (U+000A) **sin salto final**, en este
orden exacto:

```
vex-step-declaration/pipe-v1
Q(scope)
Q(rules)
<n_comandos>
  <una línea por comando, en el orden declarado>          ← §3.2
<n_variables>
  <una línea por declaración de variable, ordenadas>      ← §3.3
Q(huella del árbol del directorio del step)               ← §3.4
```

y la huella es `H(material)`.

La cabecera existe para que esta huella no pueda coincidir con el hash de otra
cosa que se le parezca. Las cardinalidades van **delante** de cada bloque: no
hacen falta para la inyectividad —las comillas ya la garantizan— y sí para que un
tercero reimplemente la regla sin adivinar dónde termina un bloque.

### 3.1 `config.yaml` entra ENTERO

Los dos campos que hoy existen, y los que se añadan:

| Campo | Qué entra |
|---|---|
| `scope` | el token declarado tal cual: `project` o `environment` |
| `rules` | la forma canónica del **conjunto**, `RuleSet.Canonical()` |

Un step **sin `config.yaml`** produce `Q("")` en las dos líneas. No es material
incompleto: es un step que no declara ámbito, y de ahí se sigue que se ejecuta
siempre y no persiste registro (spec 13 §5.3).

**Lo que entra de `rules` es el VALOR DE DOMINIO, no el texto del archivo.**
`- state_changed` y `- state_changed: [pipeline, project]` son la misma
declaración y producen la misma cadena; `max_age: 60m` y `max_age: 1h` también,
porque la duración entra normalizada por `time.Duration`. Si entrara el texto,
reescribir la forma corta como larga re-ejecutaría el step sin haber cambiado
nada.

**El ORDEN de las reglas sí es significativo y entra.** Decide qué motivo se
emite cuando más de una se cumple —es la primera que dice que sí—, así que
normalizar ordenándolas sería perder información observable. Lo que se ordena son
las FUENTES de `state_changed`, no las reglas.

> **`scope` entra; la DIRECCIÓN que de él se deriva, NO.** Son dos cosas que se
> llaman igual y están en lados opuestos de la frontera entre contenido y
> dirección:
>
> | | Qué es | ¿Entra? |
> |---|---|---|
> | `scope: project` | un **valor declarado** en `config.yaml`, como `show` o `cmd` | **sí** |
> | `state/<subject>/project/<step_id>/` | la **dirección** resuelta donde vive el registro | **no** — es la clave (spec 11 §5.1) |
>
> Que el valor entre lo exige la regla general de §3.6: cambiar `scope:
> environment` por `scope: project` es una edición deliberada del pipelinecode.
> Que la dirección **no** entre es lo que hace que esta huella siga respondiendo
> «¿es el mismo trabajo?» y no «¿es el mismo trabajo guardado en el mismo
> sitio?». Si la ruta entrara, dos proyectos con el mismo pipelinecode tendrían
> huellas distintas por vivir en directorios distintos, y la pregunta que el
> índice de la spec 11 §5.5 responde —«¿este contenido ya corrió en algún
> sitio?»— dejaría de tener sentido.

### 3.2 La entrada de un comando

Los campos se unen con **U+001E** (RS) en este orden exacto:

```
Q(name) ␞ Q(cmd) ␞ Q(workdir) ␞ <show> ␞ <n_templates> [␞ Q(template_i)]* ␞ <n_outputs> [␞ Q(name_j) ␞ Q(probe_j)]*
```

- `<show>` es `true` o `false`, sin comillas.
- `<n_templates>` y `<n_outputs>` son la cardinalidad de cada lista en decimal.
  Van **delante** de la lista y no son decorativas: sin ellas, dos comandos con
  el mismo total de elementos repartido de otra forma entre las dos listas
  producirían la misma cadena.
- Las plantillas y los outputs conservan **el orden declarado**. Reordenar dos
  plantillas cambia el orden de interpolación, así que cambia la huella.
- Un `workdir` ausente es la cadena vacía, y `Q("")` es `""`, no ausencia.
- Las rutas de plantilla y el workdir se normalizan a `/` **antes** de
  entrecomillarse: la misma declaración en Windows y en Linux produce la misma
  huella.

Los comandos van **en el orden declarado en `commands.yaml`**, sin ordenar. Es la
diferencia deliberada con el bloque de variables: en un conjunto de variables el
orden no significa nada; en una lista de comandos es la secuencia de ejecución, y
ejecutar `build` y luego `deploy` no es lo mismo que al revés.

Esta parte del material es la `inst-v1` retirada, sin un byte de diferencia en la
línea de cada comando.

### 3.3 La entrada de una declaración de variable

Dos campos unidos con **U+001E** (RS):

```
Q(nombre) ␞ Q(declaración)
```

**`declaración` es la forma canónica de lo que el pipelinecode DICE de la
variable, nunca su valor resuelto** (`step.VariableDeclaration.Canonical()`):

| Caso | Qué entra |
|---|---|
| literal | `Q(value)` — el valor entrecomillado |
| `resolve: step-output` | `step-output ␞ Q(from) ␞ Q(key)` |
| `resolve: state` | `state ␞ Q(scope) ␞ Q(key)` |

Las entradas se ordenan **ascendentemente por comparación byte a byte de la
entrada completa**, no del nombre — la misma disciplina que la huella del árbol
(`SPEC-v1.md` §3.3), y da un orden total aunque el llamador entregue dos
declaraciones con el mismo nombre, cosa que un mapa no puede pero un slice sí.

Un bloque de **cero variables es legítimo**, no material incompleto: el archivo
`variables/<ambiente>/<paso>.yaml` ausente es válido desde siempre, y significa
que el step no declara variables (spec 24 §5.3).

> **Ésta es la mitad de la regla que cierra dos defectos de signo contrario.**
> Con el material en la declaración:
>
> - las salidas de una corrida dejan de ser entradas de la siguiente, así que un
>   step con `outputs` deja de re-ejecutarse una vez de más;
> - editar un literal vuelve a mover la huella, porque el `value` declarado entra;
> - y **un valor producido en runtime deja de entrar en absoluto**, lo que cierra
>   la fuga que la spec 20 §8 dejó anotada: `step_fingerprint` viaja en los
>   hechos (spec 19) y se empuja al destino (spec 21), y un paso con una sola
>   variable no volátil dejaba ahí un digest sin sal de su valor.
>
> **Los literales SÍ siguen entrando, y eso no reabre la fuga.** La declaración de
> un literal es su valor entrecomillado, igual que en `content.parameters`; la
> spec 20 §9.4 ya decidió que eso no cuenta como «valor» —vive en un repositorio
> versionado en git y quitarlo rompería la identidad—. La contrapartida operativa
> es la misma: **un secreto no se declara en `variables/`**, llega por el entorno
> del shell (`$VAR`), la única vía que no pasa por el motor.

**Las variables VOLÁTILES no necesitan filtro aquí.** Las seis que el motor
deriva de la ejecución en curso —`project_version`, `project_revision`,
`project_revision_full`, `tool_name`, `project_workdir`, `step_workdir`— no se
declaran nunca en un `variables/<ambiente>/<paso>.yaml`: las inyecta el motor. No
hay nada que filtrar porque no hay nada que pueda entrar. La lista sobrevive en
`command.VolatileVarNames` con su otra razón —el filtro de lo que se persiste en
el registro— y **deja de ser normativa de ninguna regla de huella**.

Y por lo mismo, **`environment` deja de entrar en la huella**. Era una variable
del mapa acumulado y entraba en `vars-v1` por derecho propio; no se declara
nunca, así que con el material en la declaración desaparece sola. Es la segunda
de las dos vías por las que el ambiente entraba en la huella de un step —la
primera era la dimensión `Scope` de `cache.Material`— y las dos se cierran con la
spec 27. Cerrar sólo una no habría cambiado nada.

### 3.4 El resto del directorio del step entra como árbol, CRUDO

La última línea es la huella `v1:` del árbol del directorio del step, por su
**forma canónica completa** (con prefijo), calculada con la regla de `SPEC-v1.md`
**sin ninguna modificación** — la misma función, el mismo puerto `TreeSource`,
otra raíz.

Componer sobre la forma completa y no sobre el hash pelado es lo que hace que un
salto a `v2` de la regla del árbol invalide todas las `pipe-v1` sin una línea de
código extra.

**Se excluyen del árbol los archivos de declaración que ya entraron
normalizados**, y sólo si están en la RAÍZ del directorio del step:

```
commands.yaml
config.yaml
```

`variables/<ambiente>/<paso>.yaml` no está en la lista porque no vive bajo el
directorio del step: no hay nada de lo que excluirlo. Un
`terraform/commands.yaml` **sí** entra en el árbol: es un archivo arbitrario del
step como cualquier otro, y el motor no lo interpreta.

Sin esta exclusión, los dos archivos entrarían dos veces —normalizados y crudos—
y añadir un comentario a `commands.yaml` re-ejecutaría el step por la puerta de
atrás, con lo que el reparto de esta sección dejaría de ser cierto.

> **Crudo y no normalizado, por una razón que no admite excepción.** Son archivos
> arbitrarios —`Dockerfile`, `.tf`, `deployment.yaml`, un script— y el motor no
> sabe interpretarlos. Normalizar exigiría un parser por formato; hashear crudo
> exige cero, y un comentario dentro de un `.tf` **sí** cambia lo que terraform
> lee en algunos casos.
>
> El coste de la asimetría es que un comentario en un `Dockerfile` re-ejecuta y
> uno en `commands.yaml` no. Es el reparto correcto, porque el segundo el motor lo
> entiende y el primero no.

> **Entra TODO el directorio, no sólo las plantillas declaradas.** `templates:`
> declara qué se **interpola**, no qué **importa**. Un `.tf` que nadie interpola
> sigue siendo lo que decide qué se provisiona. La contrapartida está medida: en
> los templates `commercial` y `critical`, `02-acr/terraform/bootstrap/` —el
> terraform que se aplica a mano una vez— queda bajo el directorio del step, así
> que editarlo re-ejecuta el ACR aunque no cambie nada de lo que sus comandos
> hacen (spec 24 §9.1). Es una consecuencia conocida, no un descubrimiento: la
> alternativa era una lista declarada de qué importa, que es exactamente lo que
> se descartó.

### 3.5 La representación externa

```
pipe-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo. El token es
**distinto** del `v1:` del árbol a propósito: las dos huellas son del mismo tipo y
una compone sobre la otra, pero son reglas distintas y sus versiones tienen que
poder moverse por separado.

### 3.6 La regla general de los campos declarables, y la única exclusión

> **Todo campo declarable en `commands.yaml` o en `config.yaml` entra en la
> huella. Excluir uno exige justificarlo POR CAMPO, escrito aquí.**

Existe porque `show` se quedó fuera por olvido durante toda la vida del motor, y
una huella que ignora una edición deliberada del pipelinecode es indistinguible
de una huella rota.

**`show` entra, aunque no cambie qué se ejecuta.** El criterio de la spec 08 —«se
corrige lo que cambia el despliegue sin cambiar la huella»— lo dejaría fuera:
sólo decide si la salida del comando se imprime. Entra igualmente porque aquel
criterio está formulado para la huella del **árbol**, donde el consumidor es la
máquina. Aquí el consumidor es una persona editando `commands.yaml`, y excluirlo
produce esta secuencia:

1. un comando falla o hace algo raro;
2. el autor añade `show: true` para ver qué pasa;
3. la huella no cambia ⇒ el step se **salta** ⇒ no se imprime nada;
4. el autor concluye que `show` no funciona.

**`description` NO entra, y ésta es su justificación por campo** — heredada de
`inst-v1` §3.5 y no reescrita. No llega al modelo de ejecución: el mapeo
`PipelineCommandDTO → command.Command` la descarta, así que no existe para nada
de lo que ocurre después. No cambia qué se ejecuta, ni dónde, ni qué se imprime,
ni qué se extrae: editarla no tiene **ningún** efecto observable que una huella
pudiera suprimir, que es justo lo contrario del caso de `show`. Aplica igual a la
`description` de un `outputs[]` y a la de una declaración de variable
(`pipeline_vars_dto.go`).

Condición de reapertura escrita: si algún día `description` llega al dominio —a
un log, al registro de despliegue—, esta exclusión deja de estar justificada y
hay que reabrirla en una `pipe-v2`.

**Sigue habiendo una sola exclusión en toda la huella.**

## 4. Lo que la regla NO mira

- **La dirección.** Ni el sujeto (la url del proyecto), ni el `step_id`, ni la
  ruta del ámbito. Todo eso es la clave de estado (spec 11 §5.1), y mezclarlo con
  el contenido era el defecto (e) de la spec 27: el mismo trabajo calculado sobre
  el mismo material producía identidades distintas según dónde se guardara.
- **La url del pipelinecode.** Su CONTENIDO ya está aquí —los tres archivos y el
  árbol—, así que dos clones del mismo contenido desde remotos distintos son el
  mismo trabajo. Misma regla que «la huella identifica, el commit documenta» (P9).
- **El ambiente.** Ver §3.3. El aislamiento entre ambientes lo garantiza la CLAVE
  de estado, no el hash — que es la propiedad que la spec 11 compró.
- **El código del proyecto.** Es el término condicional de `sf-v1`
  (`SPEC-STEP-v1.md` §3.2), no material de esta regla. Un step que crea un
  registro de contenedores no depende del código de la aplicación, y meterlo aquí
  lo re-ejecutaría en cada commit.
- **El resultado de interpolar.** El material es la declaración, con sus
  `${var....}` sin sustituir.
- **El origen de una variable** (`command.Origin`). Identifica el par
  `(nombre, valor)` resultante, no su procedencia — y aquí ni siquiera hay valor.

## 5. Límites conocidos

- **`.git` como archivo entra en el árbol.** En un *worktree* o un submódulo,
  `.git` es un archivo regular con una ruta ABSOLUTA dentro, así que dos máquinas
  con el mismo árbol producirían huellas distintas y el destino compartido de la
  spec 16 no serviría de nada, sin que falle nada. El defecto lo hereda esta regla
  de `SPEC-v1.md` §5 (spec 08 §9.10) sobre una raíz más —el directorio del step—.
  **Anotado, no corregido**: con clon limpio no ocurre, y hay test de que no
  ocurre. Se cierra donde se cierre el de la raíz del proyecto, en un salto de
  versión de la regla del árbol.
- **La renumeración de un step no la ve esta regla, pero pierde su historia
  igual.** `NN-` es parte del `step_id`, que es la CLAVE. Renumerar `02-supply` a
  `03-supply` no mueve esta huella y sí mueve el sitio donde se recuerda: el
  efecto es una re-ejecución, nunca un despliegue omitido.
- **`.gitignore` se aplica con la raíz en el directorio del step.** Un
  `.gitignore` de la raíz del pipelinecode no se ve desde aquí; uno dentro del
  directorio del step sí, con la semántica congelada de `SPEC-v1.md` §5.4.
- **`resolve: state` no tiene pipelinecode real que lo ejercite** (spec 25 §5.3).
  Su rama de la forma canónica llega a esta regla probada sólo por vectores.

## 6. Vectores

Estos vectores son la tabla que una implementación independiente debe
reproducir. Están ejecutados en `declaration_test.go`, en memoria sobre
`MemTreeSource`.

Convención: cada comando se escribe como `name | cmd`, y los campos que no se
nombran valen su cero — `workdir` vacío, `show: false`, sin plantillas y sin
outputs. «árbol vacío» es un directorio de step que sólo contenía sus archivos de
declaración.

| # | Material | Huella |
|---|---|---|
| 1 | *(todo vacío, árbol vacío)* | `pipe-v1:63b4d5fc8088564bf120ecd3457bbc7164aa8886e6ec9676dda4434db3d0f89f` |
| 2 | el 1 con `scope: environment` | `pipe-v1:6d4d2125cf6cec9c16de0610c0d85aec3f37ea5d36983a9d5aeeb469a999d7ab` |
| 3 | el 2 con `rules: state_changed␞"pipeline,project"` | `pipe-v1:24f69822a34b20a25227f3cb4cb093b24b227619711dd6b24a963c1b013d2c3f` |
| 4 | el 3 con el comando `build \| mvn package` | `pipe-v1:fcf95580a27707004141c20d3969c24063d350d85cb6909f87d75ca289b64598` |
| 5 | el 4 con la declaración `region` = literal `us-east-1` | `pipe-v1:5e8c0b8fa33766e01d3e23e9e81612bc30538d6c74ff31225757537e965aed2e` |
| 6 | el 5 con un `k8s/deployment.yaml` en el árbol | `pipe-v1:4e39b400046b872123c7ce01c93ab2c69155c11348b93d8ff8b55b248d1a998b` |

**Si un vector se pone en rojo sin que nadie haya cambiado este documento a
propósito, hay una regresión**; si la regla cambia a propósito, cambia de
versión: `pipe-v1:` deja de ser `pipe-v1:`.

Los vectores 1 → 6 añaden una fuente de material cada uno, así que las seis
huellas tienen que ser distintas entre sí: es la comprobación de que ninguna de
las cinco fuentes se cayó del material.

## 7. Cómo validar una implementación independiente

1. Reproducir los vectores de §6.
2. Comprobar las sensibilidades de `commands.yaml`: cambiar `name`, `cmd`,
   `workdir`, `show`, cualquier plantilla, cualquier `outputs[].name` o
   `outputs[].probe`, el orden de las plantillas, el orden de los outputs o el
   orden de los comandos cambia la huella.
3. Comprobar las de `config.yaml`: cambiar `scope`, añadir o quitar una regla,
   cambiar la parametrización de una regla o cambiar el ORDEN de las reglas
   cambia la huella.
4. Comprobar las del árbol: añadir un archivo, cambiar su contenido, ponerle el
   bit de ejecución o cambiar el destino de un enlace cambia la huella.
5. Comprobar las de las declaraciones: cambiar el `value` de un literal, cambiar
   el `from` de un `step-output`, o añadir una declarada y vacía cambia la
   huella; el orden de entrega **no** la cambia.
6. Comprobar las insensibilidades: cambiar `description`, reescribir
   `- state_changed` como `- state_changed: [pipeline, project]`, reescribir
   `max_age: 60m` como `max_age: 1h`, o añadir un comentario a `commands.yaml` o
   a `config.yaml` **no** la cambian.
7. Comprobar que el valor RESUELTO de una variable no la cambia: dos ejecuciones
   que resuelven valores distintos para la misma declaración producen la misma
   `pipe-v1`.
8. Comprobar que la dirección no la cambia: el mismo pipelinecode servido desde
   dos raíces distintas, o para dos sujetos distintos, produce la misma
   `pipe-v1`.

## 8. Historia

| Versión | Cambio |
|---|---|
| `pipe-v1` | Primera regla congelada (spec 27). Absorbe `inst-v1` —sin cambiar un byte de la línea de cada comando— y sustituye `vars-v1`, cuyo material pasa de «mapa acumulado resuelto» a «declaración». Añade `config.yaml` entero y el árbol crudo del directorio del step. Retira del material las cuatro dimensiones de dirección que `ck-v1` mezclaba, y con ellas las dos vías por las que el ambiente entraba en la huella de un step |
