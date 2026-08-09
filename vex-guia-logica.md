# Guía del motor de pipelines de Vex

Este documento describe cómo está estructurado un pipeline (repositorio Git) y qué hace
el motor de Vex al ejecutarlo.

> **Cómo leer este documento.** Las secciones 1–8 describen **lo que el motor hace hoy**,
> verificado contra el código. Lo que aún no existe —aunque esté decidido— vive en la
> sección 9. Si el comportamiento cambia, se actualiza el cuerpo; si se decide algo nuevo,
> entra en la 9 hasta que se implemente. El cuerpo nunca describe intenciones.
>
> La sección 9 recoge **qué** se decidió y **por qué**, en términos de comportamiento
> observable. El **cómo** y el **cuándo** —etapas, refactorizaciones, puertos, layout en
> disco— viven en `vex-plan-registro-local.md` y en su revisión `vex-plan-revision.md`.
> Esta guía no los repite: los referencia.

---

## 1. Estructura de un repositorio pipeline

```
pipeline-repo/
├── environments.yaml
├── steps/
│   ├── 01-test/
│   │   ├── commands.yaml
│   │   └── config.yaml          # lo que el step declara de sí mismo (sección 8)
│   ├── 02-supply/
│   │   ├── commands.yaml
│   │   ├── config.yaml
│   │   └── terraform/...        # archivos auxiliares del step
│   └── NN-<nombre>/
│       └── commands.yaml        # sin config.yaml: se ejecuta siempre, no persiste
└── variables/
    ├── sand/
    │   ├── supply.yaml
    │   └── deploy.yaml
    ├── stag/
    └── prod/
```

### environments.yaml

Lista de ambientes.

```yaml
- name: "sandbox"
  description: "ephemeral environments for feature development and testing"
  value: "sand"

- name: "staging"
  description: "stable pre-production environment for validation"
  value: "stag"

- name: "production"
  description: "live environment with real users"
  value: "prod"
```

- `value` — **identificador único** del ambiente. Es lo único que el motor lee: es la
  clave real que usa el resto del pipeline (nombre de carpeta en `variables/`, clave del
  estado de re-ejecución, valor de la variable `${var.environment}`).
- `value` **no tiene ninguna palabra reservada**. Hasta la spec 13 no podía ser `shared`,
  porque el ámbito del almacén compartido ocupaba esa misma posición en la ruta y un
  ambiente así declarado lo pisaba (spec 04 §5.4). El ámbito de ambiente viaja ahora
  **siempre prefijado** en la clave de estado —`environment:<nombre>`— así que `shared` da
  `environment:shared` y `project` da `environment:project`, y ninguno colisiona con el
  ámbito de proyecto. **D9 queda derogada.** Lo que sí se sigue validando es el nombre:
  `/`, `\`, `:`, `.` y `..` lo hacen fallar, porque el ámbito es un tramo de ruta del
  almacén.
- `name` y `description` — documentación. El motor no los usa.

**Sobre el orden.** El motor usa el orden para una sola cosa: si la invocación no indica
ambiente, se toma **el primero de la lista**. No hay noción de promoción ni de progresión
entre ambientes — el motor no impide desplegar a `prod` sin haber pasado por `sand`.

### steps/

Cada subdirectorio es un paso del pipeline. El motor **no impone qué pasos deben existir
ni cuántos**: lee lo que encuentra.

Reglas que sí impone:

- El nombre del directorio debe seguir el formato `NN-<nombre>`, con **exactamente dos
  dígitos**. Un directorio que no encaje —`2-supply`, `002-supply`, `4_test`— hace **fallar
  la ejecución antes del primer step**, con un mensaje que lo nombra. Hasta la spec 04 se
  ignoraba en silencio, o pasaba sin ejecutar nada.
- Dos steps no pueden declarar el mismo orden (`02-a` y `02-b`): también es error de
  validación.
- Si el step tiene `config.yaml`, tiene que declarar un `scope` del vocabulario cerrado
  (sección 8). No tenerlo es legítimo; tenerlo y no declarar nada válido es error de
  validación, con el archivo nombrado en el mensaje.
- El orden de ejecución es el orden **numérico** del prefijo, ordenado explícitamente por el
  dominio y no heredado del orden en que el sistema de archivos lista el directorio.

Las tres reglas se comprueban **enteras y antes del primer step**: descubrir un typo a mitad
del despliegue llega tarde, porque los steps anteriores ya tuvieron efectos reales. Y no
cortan en el primer fallo — se reportan todos los problemas de una vez.

### variables/

Un subdirectorio por ambiente, nombrado con el `value` de `environments.yaml` (no con
`name`). Dentro, un archivo por paso: `<nombre-step>.yaml`, **sin** el prefijo numérico.

```yaml
- name: "instance_count"
  value: "3"
- name: "ingress_host"
  value: "${var.project_name}.${var.azure_kubernetes_cluster_fqdn}"
```

Solo se leen `name` y `value`. Un valor puede referenciar otras variables con
`${var.<nombre>}`; el motor las resuelve iterativamente y falla si detecta una dependencia
circular o una variable que no existe.

Que el archivo no exista es válido: se interpreta como «este step no declara variables en
este ambiente». No hay error.

---

## 2. commands.yaml

Cada paso declara una lista de comandos que se ejecutan **en el orden en que están
escritos**. Si un comando falla, el step falla y la ejecución se aborta ahí: no se
ejecutan los comandos siguientes ni los steps posteriores.

```yaml
- name: "Apply terraform shared"
  description: "execute the changes defined in the plan to provision the shared resources"
  cmd: "terraform apply plan.out"
  workdir: "./terraform/shared"
  show: false
  templates:
    - "terraform.tfvars"
  outputs:
    - name: "azure_container_registry_name"
      description: "Return the name of the Azure Container Registry (ACR)"
      probe: azure_container_registry_name\s*=\s*"([^"]+)"
```

| Campo | Descripción                                                                           |
|---|---------------------------------------------------------------------------------------|
| `name` | identificación del comando. Aparece en los logs                                       |
| `description` | documentación. El motor no la usa y **no entra en la huella de instrucciones**        |
| `cmd` | el comando shell a ejecutar. Se lanza con `sh -c` (`cmd /C` en Windows)               |
| `workdir` | ver abajo                                                                             |
| `show` | si es `true`, vuelca el stdout del comando al log de la ejecución. Por defecto `false`. **Sí entra en la huella de instrucciones** (sección 7.2) |
| `templates` | archivos a interpolar antes de ejecutar `cmd`                                         |
| `outputs[]` | `name` + `description` + `probe`. Ver abajo                                           |

### workdir

Ruta **relativa al directorio del step** dentro de la copia del pipeline (ver sección 6).
Si el step copiado vive en `.../workdirs/<pipeline>/<entorno>/steps/02-supply`, entonces
`workdir: "./terraform/shared"` ejecuta en `.../steps/02-supply/terraform/shared`.

**Si `workdir` está vacío o es `"."`, el comando se ejecuta en el directorio del proyecto**,
no en el del step. Es lo que permite que `01-test` corra `mvn clean verify` sobre el código
del proyecto sin declarar nada.

`workdir` significa **dos cosas y sólo dos**: el directorio de ejecución y la base contra la
que se resuelven los `templates`. Hasta la spec 13 significaba una tercera —el ámbito del
almacén, deducido de su primer segmento— y ése es exactamente el modo de fallo que la
retirada cierra: un mismo string decidiendo tres cosas independientes es cómo se llega a que
cambiar una rompa otra en silencio. Un directorio llamado `shared` ya no le dice nada al
motor.

Los `templates`, en cambio, se resuelven **siempre** contra el directorio del step más el
`workdir` — nunca contra el del proyecto. Un comando sin `workdir` que declare
`templates: ["docker/Dockerfile"]` interpola `<dir-del-step>/docker/Dockerfile` y ejecuta
en el directorio del proyecto. Es deliberado: los archivos auxiliares pertenecen al
pipeline, la ejecución ocurre sobre el proyecto.

### templates

Se interpolan **en el sitio**, sobreescribiendo el archivo dentro de la copia del pipeline.
El motor guarda el contenido original y lo restaura al terminar el step, **haya terminado
bien o mal** (spec 06 §5.1). El repositorio original del pipeline nunca se toca.

### outputs y probe

`probe` es una expresión regular que se aplica sobre el **stdout normalizado** del comando
—sin secuencias ANSI, con saltos de línea unificados y sin espacios al inicio o al final—.
Nunca sobre stderr.

Hay dos usos, y el motor los distingue por si el output tiene `name`:

- **Con `name`** → extracción. El grupo 1 de la regex se guarda como variable runtime con
  ese nombre. Si la regex no encuentra coincidencia, o el grupo 1 sale vacío, **el comando
  falla**.
- **Sin `name`, solo `probe`** → aserción. Solo se exige que haya coincidencia; no se
  guarda nada. Es como `01-test` verifica que aparezca `BUILD SUCCESS`.

Un mismo comando puede declarar varios outputs, todos evaluados sobre la misma salida.

---

## 3. Variables

El motor mantiene **un único mapa acumulado** durante toda la ejecución, compartido por
todos los steps. Tres fuentes lo alimentan.

### 3.1 Inyectadas por el motor

Disponibles siempre, sin declarar nada:

| Variable | Contenido |
|---|---|
| `project_id` | identificador del proyecto, del request |
| `project_name` | nombre del proyecto |
| `project_organization` | organización |
| `project_team` | equipo |
| `project_workdir` | ruta absoluta del proyecto en disco |
| `project_version` | ver abajo |
| `project_revision` | primeros 8 caracteres del hash de HEAD |
| `project_revision_full` | hash completo de HEAD |
| `environment` | el `value` del ambiente en ejecución |
| `tool_name` | siempre `vex` |
| `step_workdir` | ruta absoluta del directorio del step actual dentro de la copia |

`step_workdir` es la única que cambia entre steps: se añade al empezar cada step y se
retira al terminarlo.

**Cómo se calcula `project_version`.** Depende del step **pedido en la invocación**, y el
valor resultante rige para toda la corrida:

- Si el step pedido es `deploy` → versión semántica derivada del historial git: se parte
  del **tag semver más alto** del repositorio —no del más reciente— y se recorren los
  commits desde HEAD hasta ese tag (máximo 200). `BREAKING CHANGE` o `!` tras el tipo →
  mayor; `feat` → menor; `fix` → parche. Si no hay ningún commit relevante, se usa un sello
  de fecha. Si no hay ningún tag semver, `v0.1.0`.
- Para cualquier otro step → sello de fecha UTC con formato `vAAMMDDHHmm`.

### 3.2 Declaradas

Las de `variables/<ambiente>/<step>.yaml`. Se cargan al empezar cada step, ya resueltas
entre sí (ver sección 1).

### 3.3 Runtime

Las extraídas del stdout de un comando vía `outputs[].probe`. Se incorporan al mapa
acumulado en cuanto el comando termina, así que están disponibles para los comandos
siguientes del mismo step y para todos los steps posteriores.

### 3.4 Precedencia

Cada variable lleva su **origen** (`command.Origin`), y **el orden del enum ES la
precedencia** (spec 12). `ExecutionVariableMap.Add` inserta si la variable no existe, o si su
origen tiene precedencia **mayor o igual** que la de la ya presente:

```
1. OriginDeclared   variables declaradas del step (pipeline)   ← el default
2. OriginState      último registro del ámbito de proyecto, luego el del ambiente
3. OriginInjected   las diez que el motor deriva de ESTA ejecución (handler 07)
4. OriginRuntime    extraídas del stdout durante ese step       ← siempre gana
```

Tres consecuencias:

- Un literal de `variables/prod/deploy.yaml` es un **valor por defecto**: en cuanto algo
  produce ese nombre, el producido manda para el resto de la ejecución. Hasta la spec 12 era
  al revés y el pipeline no podía reaccionar a lo que él mismo producía.
- Un hecho de la ejecución actual (`project_revision`, `project_name`) **no** puede ser
  pisado por un valor de una corrida anterior. Antes sí podía, y lo único que lo evitaba era
  que las volátiles no se persisten — la protección era la lista, no el orden.
- La **igualdad** permite actualizar: dos comandos del mismo step que producen la misma
  variable, gana el segundo; y el ámbito del ambiente, que carga después, gana sobre el de
  proyecto.

**El orden en que la cadena alimenta el mapa dejó de decidir quién gana**, y lo fijan
`TestVarsChain_ElOrdenDeLosHandlersYaNoDecideQuienGana` —las permutaciones con los handlers
reales— y las 24 de `TestExecutionVariableMap_Add_ElOrdenDeLlegadaNoCambiaElResultado`.

Pero el orden **no es indiferente**, y la spec 12 §5.3 —que mandaba adelantar el handler 03—
se retira por eso (§9.1 de esa spec). El handler 03 no solo añade las variables declaradas:
las **resuelve**, interpolando `${var.…}` contra el mapa acumulado tal como esté en ese
instante. Cargarlo antes que el almacén le quita de la vista el registro del propio step, y
un literal que dependa de él tumba la ejecución entera con «variable faltante». La cadena
queda `01 → 02 → 03 → 04`: **el orden significa completitud del mapa, y quién gana lo dice
`Origin`.** Lo fija `TestRunCommand_UnLiteralPuedeInterpolarElRegistroDelPropioStep`.

> **Pérdida aceptada y viva.** El registro persiste hoy el mapa acumulado entero, no solo lo
> que el step produjo, así que un literal declarado entra en el almacén en la primera corrida
> y vuelve como `OriginState` en la segunda. Editarlo en el pipelinecode deja entonces de
> surtir efecto —y el step ni se re-ejecuta, porque la huella de variables tampoco cambia—.
> Es la misma raíz que el «se re-ejecuta una vez de más» de la sección 7, y lo corrige P7
> (spec 14) al distinguir lo que un paso **consume** de lo que **produce**. Lo fija
> `TestRunCommand_ElAlmacenPisaAlLiteralDeclarado`, que se pondrá en rojo cuando llegue.

### 3.5 Interpolación

La sintaxis del motor es `${var.<nombre>}`, tanto en `cmd` como en los archivos de
`templates` y en los valores de `variables/`. Si el nombre no existe en el mapa acumulado,
la interpolación falla y con ella el comando.

`$VARIABLE` y `${VARIABLE}` **no** son del motor: llegan intactas a la shell, que las
resuelve con el entorno del proceso. Es como los pipelines pasan secretos
(`$ARM_CLIENT_SECRET`) sin que el motor los vea nunca.

---

## 4. Entrada de la invocación

El motor **no** ejecuta todos los ambientes en una corrida. Recibe:

- un **ambiente** (uno solo, opcional — si no se indica, el primero de `environments.yaml`), y
- un **step** (obligatorio).

Se ejecutan los steps desde el primero hasta el indicado, en ese único ambiente. Si el
ambiente no está en `environments.yaml`, o el step no existe en `steps/`, la ejecución
falla antes de empezar.

> Ejemplo: «ejecuta en `prod` el step `supply`» corre `01-test`, `02-supply` en `prod`.

**Y recibe además, obligatoriamente, dónde vive el estado.** El motor no sabe de modos: no
hay un `--mode local|remote` con un default que asuma una plataforma. La configuración de
destino llega por `--state-config <archivo>` o por la env var `VEX_STATE_CONFIG` (YAML o
JSON, crudo o base64), y **si falta, el motor no arranca**:

```yaml
type: local          # vocabulario cerrado: local | http
local:
  path: /mnt/vex-state
```

`type: local` **no es un no-op**: si la ruta no existe o no es escribible —el volumen no
montado, el caso cotidiano— falla al instante y con causa, porque escribir a ciegas en el
filesystem efímero de un contenedor perdería los identificadores de recursos que ya existen
en la nube. `type: http` está en el vocabulario y **todavía no está implementado**; el error
lo dice con esas palabras, para que no se confunda con una configuración equivocada.

Aparte del destino, el motor es dueño de su **área de trabajo**, que sí tiene default y no
depende de configuración ninguna: `--staging-dir` → `$XDG_STATE_HOME/vex/staging` →
`$HOME/.local/state/vex/staging` → `os.TempDir()`. Nunca cae bajo el volumen del destino.

---

## 5. Ciclo de ejecución

El motor está construido como tres cadenas anidadas: una de pipeline, una de step y una de
comando.

### 5.1 Cadena de pipeline — una vez por ejecución

1. **Clonar el proyecto** a desplegar.
2. **Clonar el pipeline.**
3. **Resolver el ambiente** — validarlo contra `environments.yaml`, o tomar el primero.
4. **Cargar los steps** — leer `steps/` y quedarse con los que van del primero al pedido.
5. **Copiar el pipeline** al directorio de trabajo (sección 6).
6. **Calcular la versión** del proyecto y el hash de HEAD.
7. **Inyectar las variables iniciales** (sección 3.1).
8. **Calcular la huella del código** del proyecto — recorrido del árbol respetando
   `.gitignore`, sha256 por archivo y sha256 del conjunto. La regla está congelada y
   especificada desde la spec 08: `internal/domain/fingerprint/SPEC-v1.md`. Entran el
   bit de ejecución de cada archivo y el destino de cada enlace simbólico; la huella
   lleva el prefijo `v1:` en todo lo que persiste y compara.
9. **Ejecutar los steps** en orden, entrando en la cadena de step por cada uno.

### 5.2 Cadena de step — una vez por step

1. **Cargar el almacén**: el último registro del ámbito de **proyecto** de ese step y,
   después, el del ámbito del **ambiente**, añadiendo las variables de los dos.
2. **Cargar y resolver las variables declaradas** del step para ese ambiente.
3. **Leer el `config.yaml` del step** (sección 8) y **decidir si se re-ejecuta**
   (sección 7); si procede, recorrer sus comandos entrando en la cadena de comando por cada
   uno.

**Se leen los dos ámbitos y se escribe en uno.** El ámbito que el step declara decide
**dónde vive su registro**, no qué puede ver: un step de ambiente ve lo que dejó un step de
ámbito de proyecto, y por eso dos despliegues a ambientes distintos comparten el ACR.

Al terminar:

- **Si el step ejecutó y tuvo éxito** → se añade **un registro nuevo** (sección 7.3), bajo
  el ámbito que el step declara, con el mapa acumulado menos las seis variables volátiles
  —`project_version`, `project_revision`, `project_revision_full`, `tool_name`,
  `project_workdir` y `step_workdir`—, que son las mismas que la huella de variables
  excluye, de una sola lista. Después se indexa ese registro.
- **Si el step no declara ámbito** (no tiene `config.yaml`) → no se escribe nada, y el step
  se ejecuta en todas las corridas.
- **Si el step revivió** → no se escribe nada. Revivir no es un hecho nuevo del step, es la
  constatación de uno viejo.
- **Si el step falló** → no se escribe nada. No hay registro que borrar porque no llegó a
  escribirse: consultar no modifica nada, y el único momento de escritura es el primer punto.

El registro guarda el mapa acumulado **completo**, no solo lo que ese step produjo. El
registro de `deploy` contiene también lo que produjo `supply`.

### 5.3 Cadena de comando — una vez por comando

1. **Interpolar los `templates`**, guardando copia del original.
2. **Interpolar el `cmd`.**
3. **Ejecutar** en el workdir resuelto y capturar la salida. Un exit code distinto de cero
   aborta.
4. **Verificar los `probe`** contra el stdout normalizado.
5. **Extraer las variables runtime** e incorporarlas al mapa acumulado.

### 5.4 Estado de la ejecución

La ejecución **conoce y publica su propio estado**. Nace `queued`, pasa a `running` al
empezar el trabajo y termina en uno de tres estados terminales: `succeeded`, `failed` o
`canceled`. Cada estado terminal lleva el instante en que se alcanzó —de ahí sale la
duración— y los dos primeros, además, un exit code: el del comando que falló, o el genérico
si lo que falló no fue un comando (un clone, la validación del pipelinecode).

Quien registra el resultado es **la capa que lo ve**, y solo ella: el use case, que es la
única que observa tanto el éxito como el fallo de la cadena. Las transiciones ilegales
—terminar sin haber empezado, dar por fallida una ejecución ya cancelada— se rechazan en vez
de pisar el estado en silencio.

Todos los instantes salen de **una sola fuente inyectable**, no de `time.Now()` disperso por
el dominio. Es lo que hace que la versión por fecha del proyecto sea reproducible y que el
registro que se construya encima pueda probarse.

El exit code del proceso:

| Código | Significado |
|---|---|
| `0` | ejecución exitosa |
| `1` | la pipeline falló |
| `2` | input inválido (JSON malformado, `schema_version` no soportado, fuente vacía, destino del estado ausente o inalcanzable) |
| `130` | ejecución cancelada por señal (`SIGINT`/`SIGTERM`) |

**La cancelación es best-effort.** `SIGINT` y `SIGTERM` cancelan el contexto de la ejecución:
el comando en curso muere, la cadena se desenreda —restaurando plantillas y revirtiendo el
estado del step— y la ejecución queda registrada como `canceled`, que es una decisión, no
una desgracia. Hay un plazo breve para ese desenredo; pasado el plazo el proceso sale igual.
Un `SIGKILL` o un OOM siguen sin dejar rastro, y eso es correcto: son interrupciones, no
cancelaciones.

---

## 6. El pipeline como plantilla reutilizable

Un mismo pipeline puede ser usado por muchos proyectos distintos. Por eso el motor **no
ejecuta sobre el clon del pipeline**: lo copia en un directorio de trabajo propio, y es
sobre esa copia que interpola variables y archivos. El clon original nunca se modifica.

- La copia se resuelve **una vez por ejecución** y se reutiliza para todos los steps.
- El destino es propio de la terna **(proyecto, pipeline, ambiente)**: dos ambientes del
  mismo proyecto no comparten directorio de trabajo.
- Se copia el árbol completo del pipeline **excepto `.git`**, sobreescribiendo lo que
  hubiera de corridas anteriores.

El clon del pipeline se rehace **en cada ejecución**: se borra el anterior y se clona de
nuevo con profundidad 1. No hay ventana de reutilización *(ver P6 en la sección 9)*.

---

## 7. Re-ejecución: cuándo un step revive

Antes de ejecutar los comandos de un step, el motor lee el **último registro** de su clave
de posición —`(proyecto, ámbito, step)`— y compara su huella con la que acaba de calcular a
partir de todo lo que determina el resultado de ese step.

```
no hay registro                     →  el step se ejecuta
la huella del último NO coincide    →  el step se ejecuta
el último tiene más de 30 días      →  el step se ejecuta
coincide y está vigente             →  el step REVIVE: no ejecuta nada y no escribe nada
```

No hay reglas, ni una lista de comprobaciones, ni una tabla por nombre de step. La decisión
es una comparación de igualdad contra **un** registro, el más reciente.

**El índice de `cache/` no participa en esta decisión** (sección 7.3): borrarlo entero no
cambia una sola. Y la comparación es contra el último registro y no contra «algún registro
con esta huella», así que `A → B → A` re-ejecuta —regresión declarada y segura respecto de
lo que el motor hacía entre las specs 10 y 11: ejecutar de más nunca omite un despliegue—.

### 7.1 El material de la huella

Siete dimensiones, **todas obligatorias**:

| # | Dimensión | Qué es |
|---|---|---|
| 1 | proyecto | la url del repositorio del proyecto |
| 2 | pipeline | la url del pipelinecode |
| 3 | **ámbito** | el ambiente. *(Está decidido que el ámbito salga de la huella y pase a la clave de posición — ver P16 en la sección 9)* |
| 4 | step | el nombre del step, sin el prefijo `NN-` |
| 5 | instrucciones | la huella `inst-v1` de los comandos declarados |
| 6 | variables | la huella `vars-v1` de las variables del step |
| 7 | código | la huella `v1` del árbol del proyecto |

El **ámbito es obligatorio y no anulable**, y ese es el punto: la clave no responde «¿son
iguales las entradas?» sino **«¿esto ya se ejecutó *aquí*?»**. Un step tiene efectos sobre un
ambiente real, y que dos ambientes tengan entradas idénticas no significa que ejecutar en uno
haya dejado algo hecho en el otro. Hasta que la clave existió, el aislamiento entre `sand` y
`prod` dependía de que la variable `environment` estuviera en el mapa acumulado y nadie la
declarara volátil.

Si **falta cualquiera de las siete**, no se compone clave: el step se ejecuta y no se escribe
entrada. Como *ausencia de entrada ⇒ ejecutar*, «no se pudo averiguar» y «no consta» llevan al
mismo sitio, que es el seguro.

**El tiempo no entra.** El TTL —30 días— es metadato de expiración de la entrada, no material
de la clave: una entrada que caduca se sustituye **bajo la misma clave**. Se aplica a todos los
steps por igual.

### 7.2 Las tres huellas

Las tres son SHA-256 sobre un material canónico, cada una con su **regla congelada, su token
de versión y su especificación normativa** en `internal/domain/fingerprint/`:

| Huella | Token | Material |
|---|---|---|
| instrucciones | `inst-v1:` | por comando y **en el orden declarado**: `name`, `cmd`, `workdir`, `show`, la lista de `templates` y la de outputs (`name` + `probe`). **No** entra `description` |
| variables | `vars-v1:` | el mapa acumulado, ordenado, con nombre + valor + un tercer campo constante `false`, **menos las seis volátiles** de la sección 5.2 |
| código | `v1:` | el árbol del proyecto: una entrada por archivo visible, con su contenido, su bit de ejecución y el destino de los enlaces |

Tres cosas que conviene no perder de vista:

- **`show` entra**, aunque no cambie qué se ejecuta. Si no entrara, añadir `show: true` para
  depurar un comando no invalidaría el caché: el step se saltaría y no se imprimiría nada, y
  un caché que ignora una edición deliberada del pipelinecode es indistinguible de uno roto.
  La regla general es que **todo campo declarable en `commands.yaml` entra**; excluir uno
  exige justificarlo por campo, y hoy la única exclusión justificada es `description`, que ni
  siquiera llega al modelo de ejecución.
- **`description` no entra**, ni el del comando ni el de un output.
- **El cuerpo de las plantillas no entra en ninguna de las tres.** `templates:` aporta las
  *rutas*; el contenido de esos archivos no está en el material de instrucciones, y la huella
  del árbol es la del **proyecto**, no la del pipelinecode. Consecuencia viva: **editar
  `steps/02-supply/k8s/deployment.yaml` y volver a ejecutar hace que el step se salte.** Es un
  defecto conocido, anotado como D14 en la sección 9.2, y está decidido cómo se corrige
  (P15).

Cada huella lleva su token en la forma externa, y la clave se compone sobre esas cadenas
**completas**, nunca sobre el hash pelado. De ahí sale que subir de versión cualquiera de las
tres reglas invalide todas las claves emitidas sin código extra.

### 7.3 Cómo se guarda el estado

**Dos tiendas con reglas de vida opuestas**, y la separación se ve desde `ls`.

**La verdad — `state/`, append-only.** Un archivo JSON por ejecución real de un step, bajo
su clave de posición. **Nunca se sobrescribe ninguno**, y el nombre del archivo es un ULID,
cuyo orden lexicográfico es el temporal:

```
<destino>/state/<proyecto>/<ámbito>/<step_id>/<record_id>.json
```

`<destino>` es el `local.path` de la configuración de la sección 4 — el volumen montado, no
el `$HOME` del proceso. Es lo que hace que el estado **no sea de una máquina**: dos máquinas
que apuntan al mismo destino se reviven entre sí, porque la huella no depende de dónde estén
los archivos.

`<ámbito>` es `project` o `environment/<nombre>` — **dos segmentos**, no
`environment:<nombre>`: el valor lógico lleva los dos puntos y la ruta no, porque `:` es
ilegal en rutas de Windows. `<step_id>` es el nombre del directorio del step **con su
prefijo** (`02-supply`), así que **renumerar un step pierde su historia** — el efecto es una
re-ejecución, nunca un despliegue omitido.

```json
{
  "schema_version": 2,
  "record_id": "01KZBF3MG0000G40R40M30E209",
  "step_fingerprint": "ck-v1:6d12da1d…",
  "variables": [{ "name": "acr_name", "value": "acmeregistry.azurecr.io" }],
  "produced_by": { "execution_id": "…", "at": "2026-08-06T12:00:00Z" }
}
```

Esta tienda **no se borra nunca**: aquí viven los identificadores de recursos que existen de
verdad en la nube, y borrarlos implicaría perderlos de vista o crear duplicados. La clave
**no lleva el pipeline** a propósito: el ACR pertenece al proyecto, y un proyecto que cambia
de plantilla no debe perder de vista lo que ya creó. Que dos pipelines no se reviven entre
sí lo garantiza la huella, que incluye el pipelinecode entero.

**El índice — `cache/`, desechable.** Mismo layout direccionado por contenido de siempre,
otra carga: responde «¿este contenido exacto ya corrió alguna vez, y cuál fue el registro?».

```
<destino>/cache/<versión de la clave>/<2 primeros del hash>/<resto>.json
   →  { cache_key, state_key: {subject, scope, step_id}, record_id }
```

**No participa en ninguna decisión del motor**, y esa es su definición: es derivable
—se reconstruye recorriendo los registros— y borrarlo entero no cambia una sola decisión.
Sus consumidores son de consulta.

El registro **se escribe después de que el step termina bien**, desde un único sitio y una
sola vez, bajo el **ámbito que el step declara** (sección 8), y sólo si el step **ejecutó**:
un step que revive no escribe nada. Un step que empieza y no termina —incluidos `SIGKILL`,
un OOM o un corte de luz— tampoco, y la corrida siguiente lo vuelve a ejecutar. Un step que
no declara ámbito tampoco escribe, y por eso se ejecuta en todas las corridas.

**Los registros con `"schema_version": 1` se siguen leyendo.** Lo que la versión 2 quitó es
el campo `shared` de cada variable, y el lector ya no tiene dónde ponerlo: el ámbito lo dice
la clave bajo la que está el registro. Rechazarlos habría dejado huérfanos los
identificadores de recursos que existen de verdad en la nube. Una versión **desconocida**
sigue siendo un error, que es la asimetría de la sección 7.4.

### 7.4 Cuando el motor no puede averiguarlo

Se conserva el **fail-open** —ante la duda, ejecutar, porque ejecutar de más nunca produce un
despliegue que no ocurrió— y se conserva que **no sea silencioso**. La duda que ejecutar
resuelve es una: **no se pudo componer la huella** (falta material). Entonces el step se
ejecuta con advertencia explícita y su registro se escribe **sin huella**, así que no revivirá
nunca; lo que produjo se conserva igual, porque perder de vista un ARN es peor que
re-ejecutar. No se disfraza de «el contenido cambió», que es lo que ocurría cuando un fallo
de I/O producía la misma decisión y la misma forma de razón que un cambio real.

**Lo que NO es fail-open es un registro ilegible.** Ahí la duda no se puede resolver
ejecutando sin arriesgar un recurso duplicado, así que la ejecución **falla**. Es la
asimetría deliberada entre las dos tiendas: en el índice, ilegible ⇒ ausente; en el
registro, ilegible ⇒ error.

Si la escritura del registro falla **después** de un step exitoso, el step no falla —el
despliegue ocurrió— pero se emite una advertencia: se acaba de perder la pista de lo que ese
step produjo, y el usuario tiene que poder saberlo.

### 7.5 Granularidad

La decisión es **todo o nada por step**: o corren todos sus comandos, o no corre ninguno.
No existe granularidad menor.

Y **todo entra en la clave de todos los steps**. No hay comprobaciones seleccionadas por
nombre: un cambio de código re-ejecuta también `supply`, al que no le afecta. Es una pérdida
de eficiencia aceptada a cambio de correctitud —la selección estaba cableada en el motor por
nombre de step, y por eso un step llamado de cualquier otra forma no se ejecutaba jamás— y se
recupera cuando el pipelinecode pueda declararla *(ver P1 y P14 en la sección 9)*.

**Cualquier nombre de step vale.** El motor deriva la decisión del contenido, no de una lista
de nombres conocidos, así que un `05-notify` se ejecuta como cualquier otro y la corrida
siguiente lo salta.

---

## 8. El step declara su ámbito

Un step declara en su propia configuración a qué ámbito pertenece. Exactamente uno, nunca
dos, nunca ninguno de forma implícita.

```yaml
# steps/01-acr/config.yaml
scope: project
```

`scope` admite **exactamente dos valores**, y el vocabulario es cerrado: un ámbito
inventado no tendría dónde persistirse, así que cualquier otro valor es un error de
pipelinecode.

| Valor | Significado | Dónde vive el registro |
|---|---|---|
| `project` | el trabajo es común a todos los ambientes del proyecto | `project/` |
| `environment` | el trabajo es propio del ambiente en ejecución | `environment/<nombre>/` |

Es lo que permite que un recurso común a todos los ambientes —un registro de contenedores,
por ejemplo— se cree una sola vez y que los tres ambientes lean su nombre.

**Un step, un ámbito.** Si un step necesita los dos, **se parte en dos steps**, secuenciados
por su prefijo numérico:

```
steps/
  01-acr/          scope: project        (crea el registro de contenedores)
  02-provision/    scope: environment    (crea recursos propios del ambiente)
```

La regla no necesita nada más: el orden numérico ya garantiza que `01-acr` corra —o reviva—
antes de que `02-provision` lo necesite, el step se sigue identificando por su ruta y no hay
dos bloques dentro de un step que ordenar.

**Sin `config.yaml` el step no declara ámbito**, y de ahí se sigue todo: **se ejecuta
siempre, y no escribe registro de estado**. Ejecutar siempre es el default seguro —ejecutar
de más nunca produce un despliegue que no ocurrió— y no escribir es lo que impide que el
motor le invente un ámbito. Sus variables siguen viajando en el mapa acumulado de la
corrida; lo que no ocurre es que crucen de una ejecución a la siguiente. Es el mismo
criterio que un step sin comandos, que es `skipped{no_commands}` y tampoco persiste estado.

**La validación corre antes del primer step.** Un `config.yaml` presente cuyo `scope` falta
o está fuera del vocabulario aborta la ejecución con un mensaje que nombra el directorio,
como cualquier otra regla de estructura del pipelinecode (sección 2).

> **De dónde viene esto, porque es un mecanismo que se sustituyó entero.** Hasta la spec 13
> el ámbito era una marca sobre cada *variable*, deducida del **primer segmento** de la ruta
> de `workdir`: si era exactamente `shared`, las variables de ese comando se guardaban en un
> ámbito común. Los tres pipelines de `Vex/pipelines` escriben `./terraform/shared`, cuyo
> primer segmento es `.`, así que **el mecanismo nunca llegó a activarse en ninguno**. Que
> eso no causara un recurso duplicado se debía a que el backend de terraform del bloque
> compartido omite el ambiente en su clave de estado — una convención del pipelinecode, no
> una garantía del motor.
>
> Se retira sin dejar azúcar de compatibilidad: `workdir: shared/x` ya no comparte nada, y
> `shared` deja de ser una palabra con significado para el motor, tanto como directorio como
> nombre de ambiente. Dos mecanismos para lo mismo es exactamente cómo se llega a que uno
> esté muerto y nadie lo note, que es lo que pasó.

> **Estado de los templates.** Los tres de `Vex/pipelines` **todavía no declaran
> `config.yaml`**, así que sus steps se ejecutan siempre y no persisten registro: más lento
> que antes, nunca incorrecto. Lo cierra la spec 24, que parte `02-supply` en dos.

> **Límite vivo.** La lectura cruzada entre ambientes funciona —un step de ambiente ve lo que
> produjo uno de ámbito de proyecto—, pero un step `scope: project` **no revive todavía** el
> registro que escribió desplegando a otro ambiente: su huella lleva el ambiente, tanto en la
> dimensión `ámbito` de la clave (sección 7.2) como en la variable `environment`. Se ejecuta
> una vez por ambiente y escribe en el mismo sitio. Lo cierra la spec 27, al sacar de la
> huella las dimensiones de dirección *(ver P16 en la sección 9)*.

---

## 9. Diseño pendiente

Decisiones tomadas que el motor todavía no implementa, y defectos conocidos. Nada de esto
está en el cuerpo del documento porque hoy no ocurre.

Las entradas conservan su número aunque el orden temático no coincida: son anclas estables
a las que los planes ya referencian.

| Tema | Entradas |
|---|---|
| Steps y re-ejecución | P1, P4, P10, **P14** |
| Variables | P3, P7, P8 |
| Ámbito del step | ~~P2~~, ~~P5~~, ~~**P13**~~ |
| Identidad y procedencia | P9, P6, **P15** |
| Dónde vive el estado | **P16**, **P17** |
| Registro de despliegue | P11, P12 |

> **Un rediseño posterior sustituyó cinco de estas entradas.** El documento
> *«Ámbito, ejecución y re-ejecución de steps»* llegó con las specs 00–10 ya
> implementadas y cambió el mecanismo —no el objetivo— de la parte de ámbito y
> re-ejecución. **P2 queda superada y P5 derogada**; en su lugar entran P13–P17.
> Las entradas viejas se conservan tachadas y con su motivo, porque los planes y
> las specs las referencian por número y porque el argumento por el que se
> descartaron es parte de la decisión.

### 9.1 Decidido, pendiente de implementar

**P1 — Vocabulario de steps abierto, con re-ejecución declarada por el pipeline.**
Cualquier `NN-<nombre>` debe ser un step válido. Lo que hoy está cableado en el motor —qué
comprobar para decidir si un step se re-ejecuta— pasa a declararse en el propio
pipelinecode, por step. **Un step sin declaración se ejecuta siempre.** El nombre del
concepto («policy») está por confirmar; lo que importa es su finalidad: declarar qué
comprobar.

Hay dos tiempos y conviene no confundirlos. **Mientras la declaración no exista**, un step
sin comprobaciones cableadas debe **fallar ruidosamente**: es el arreglo inmediato del
silencio, y no exige formato nuevo. **Una vez exista**, un step sin declaración **se
ejecuta siempre** — que es el destino, y es seguro porque ejecutar de más nunca produce un
despliegue que no ocurrió, mientras que saltar de menos sí.

**El primer tiempo se implementó** (spec 05) y **ya no existe**: `PolicyBuilder.Build`
devolvía error ante un step que no conocía, y `Policy.Evaluate` con cero reglas mandaba
ejecutar en vez de saltar. Los dos tipos se fueron con la spec 10. Lo que sobrevive es la
semántica —**un conjunto vacío de evidencia no concluye que el step esté al día**—, hoy en la
forma «ausencia de entrada de caché ⇒ ejecutar», y sobrevive **por construcción**: una clave
que nadie escribió no puede afirmar que nada cambió.

**El vocabulario ya está abierto (spec 10).** Al desaparecer `PolicyBuilder` no quedó ningún
nombre que reconocer: un step desconocido no tiene entrada de caché, luego se ejecuta, y al
terminar bien escribe la suya. La medida de transición de la 05 vivió exactamente entre la 05
y la 10, y el cuerpo de este documento (sección 7.5) ya lo describe así.

**Lo que queda pendiente es la declaración** —`checks` por step, que es lo que devuelve la
granularidad que la 10 sacrificó al meter todo en la clave de todos los steps—, y es la
**spec 15**.

**P2 — ~~La marca `shared` pasa a ser explícita.~~ SUPERADA por P13.**

> **Por qué se descartó, que es lo que hay que conservar.** P2 resolvía dos de los
> tres problemas —la marca posicional y el nombre reservado— y dejaba el tercero
> intacto: **un step seguía pudiendo tener una parte compartida y otra de
> ambiente**. Eso obligaba a P5, o sea a que el motor aprendiera a partir la
> unidad de decisión en `step × ámbito`, con dos claves, dos huellas, dos ciclos y
> un orden entre fases que antes garantizaba el autor. P13 mueve la declaración un
> nivel arriba —al step— y esa capacidad deja de hacer falta: el pipelinecode
> expresa dos unidades con dos directorios, y el orden numérico ya las secuencia.
>
> El argumento **contra la ruta posicional** se conserva entero y es de P2: la
> ruta `workdir` ya significa tres cosas, y un mismo string decidiendo tres cosas
> independientes es cómo se llega a que cambiar una rompa otra en silencio.

Lo que P2 proponía, para referencia: un campo `scope: shared` en el comando, junto
al `workdir`:

```yaml
- name: "Apply terraform shared"
  scope: shared
  workdir: "./terraform/shared"
```

Se elige explícito y no posicional porque la regla posicional ya falló en silencio una vez
(sección 8) y porque no obliga a mover un solo archivo de los templates existentes. La
convención de directorio puede quedar como azúcar que implique `scope: shared`, pero no
como único mecanismo. Trabajo asociado en `Vex/pipelines`: marcar los comandos del bloque
compartido en los tres templates.

**El trabajo en el motor es menor de lo que parece:** la tubería de almacenamiento
compartido ya existe y funciona de punta a punta (sección 8). Lo único que cambia es de
dónde sale la marca. Lo que sí es nuevo es la **validación**: si un step declara outputs
bajo un directorio llamado `shared` pero ningún comando resulta con ámbito compartido, el
motor debe avisar. Es exactamente la comprobación que habría evitado que la regla
posicional muriera en silencio durante toda la vida de los tres templates.

**Lo que se gana no es comodidad, es una garantía.** Hoy que el recurso común no se
duplique lo asegura que el autor del pipeline recordara omitir `${var.environment}` de la
clave del estado remoto de terraform. Es una convención no verificada dentro de un string
de configuración: si alguien la añade, se crean tres recursos y nada lo señala. P2 mueve
esa garantía del pipelinecode al motor.

**P3 — Sobre un valor literal, la variable runtime siempre gana. — HECHO (spec 12).**
Un `value` literal en `variables/<ambiente>/<step>.yaml` es un valor **por defecto**: en
cuanto un comando produce una variable con ese nombre, el valor runtime manda para el resto
de la ejecución. La precedencia dejó de emerger del orden de los handlers y es una invariante
de `ExecutionVariableMap.Add` sobre el enum ordenado `command.Origin`; el orden vigente y sus
consecuencias están en la sección 3.4. Lo que queda de esta entrada es la frontera con P7,
que sigue valiendo para quien llegue después.

**Alcance, y su frontera con P7.** P3 es una regla de **precedencia**, y solo tiene sentido
donde puede haber choque: dos fuentes que aportan un nombre y hay que decidir cuál vale. Eso
ocurre con los valores literales, y ahí P3 rige. **No** ocurre con las variables de origen
declarado (P7): esas nombran su fuente, así que tienen un único proveedor por construcción y
no hay nada que resolver.

Son dos mecanismos con dos reglas, y ambos quedan vigentes. Conviene no leer P7 como una
derogación de P3: P7 no cambia quién gana, **quita la pregunta** en los casos que cubre.

**P4 — El ambiente es parte de la identidad de re-ejecución. — HECHO (spec 10).**
El ámbito es una de las siete dimensiones de la clave, obligatorio y no anulable, y el cuerpo
de este documento lo describe en la sección 7.1. Lo que queda de esta entrada es el porqué,
que sigue valiendo para quien llegue después.

Las cuatro comprobaciones pasan a la clave `(proyecto, pipeline, ambiente, step)`. La huella
no responde «¿son iguales las entradas?» sino «¿esto ya se ejecutó *aquí*?»: un step tiene
efectos sobre un ambiente real, y que dos ambientes tengan entradas idénticas no significa
que ejecutar en uno haya dejado algo hecho en el otro.

**Única excepción: el ámbito `shared`**, que por definición es común a todos los ambientes y
lleva `shared` en el lugar del ambiente. Es la única, y hay que mantenerla explícita: si
alguien mete el ambiente también ahí, el recurso compartido se duplica por ambiente. Es un palabra
reservada para el motor y no se puede declarar un ambiente con value `shared`

Incluye añadir el pipeline a la clave de la comprobación de tiempo, y validar que ningún
ambiente pueda llamarse `shared` (hoy colisionaría con el almacén compartido sin aviso).

**Hoy el aislamiento no solo es parcial: es accidental.** Que cambiar de ambiente no
provoque un salto indebido depende únicamente de que la variable `environment` esté en el
mapa acumulado y no figure en la lista de exclusión de la huella de variables. Nadie lo
escribió con esa intención y nada lo protege: añadir `environment` a esa lista —una
decisión que parecería razonable, porque es una variable volátil— rompería el aislamiento
entre `prod` y `sand` sin una sola señal. P4 convierte esa coincidencia en una regla, y
exige un test que corra el mismo árbol contra dos ambientes y verifique que el segundo
**no** se salta.

**P5 — ~~El bloque `shared` es un step previo, con su propio ciclo.~~ DEROGADA por P13.**

> **Por qué se derogó.** P5 era la consecuencia inevitable de P2: si la marca es
> del comando, un step tiene dos ámbitos, y para que cada uno pueda saltarse por
> separado el motor tiene que partir la unidad de decisión en `step × ámbito`. Eso
> es **capacidad nueva** —partición de la huella de variables, orden garantizado
> entre fases, identificador compuesto `<NN-nombre>/<ámbito>` (S-9), y una
> composición de decisiones que reintroduce la pregunta del tercer estado—.
>
> P13 elimina la premisa. Con **un step, un ámbito**, no hay fases que componer:
> lo que P5 quería expresar se expresa con dos directorios, y todo lo que P5
> tenía que construir ya existía. El caso de uso no se pierde ni un poco; lo que
> desaparece es el mecanismo.
>
> **S-9 queda sin objeto**: no hay identificador de fase porque no hay fases. El
> step se identifica por su ruta, como siempre.

Lo que P5 proponía, para referencia: `shared` dejaba de ser una marca sobre
variables (sección 8) y pasaba a ser **una fase que corre antes del step real, con
el mismo ciclo que un step** —carga de variables, decisión de re-ejecución, ejecución de
comandos, persistencia del resultado— solo que en su propio ámbito, que llamamos `shared`.

El modelo mental: un step tiene dos fases, `shared` y ambiente. Misma mecánica, ámbitos
distintos.

| | fase `shared` | fase de ambiente |
|---|---|---|
| Cuándo corre | siempre primero | después, y solo si `shared` no falló |
| Clave de estado | `(proyecto, pipeline, shared, step)` | `(proyecto, pipeline, ambiente, step)` |
| Huella de variables | solo lo genuinamente común | el resto del acumulado |
| Dónde persiste | ámbito `shared` | ámbito del ambiente |
| Visibilidad de lo que produce | todos los ambientes | solo ese ambiente |

De ahí se sigue lo que hay que construir:

- **Se salta por separado.** Si el bloque compartido no cambió, no se ejecuta, aunque la
  parte de ambiente sí se ejecute. Hoy es al revés: cualquier cambio en la parte de ambiente
  re-ejecuta el step entero, bloque compartido incluido — y con P4 eso significaría
  re-crear el recurso común una vez por ambiente.
- **El orden lo garantiza el motor**, no el autor. Hoy que el bloque compartido corra primero
  depende únicamente del orden en que se escribieron los comandos en `commands.yaml`; la
  parte de ambiente necesita las variables que produce, así que la precedencia tiene que ser
  una regla del motor.
- **La huella se parte.** La del bloque compartido se calcula solo sobre lo genuinamente
  común —las instrucciones de sus comandos y las variables compartidas— y nunca sobre
  parámetros de un ambiente. Si no, un cambio en `variables/prod/deploy.yaml` invalidaría el
  bloque compartido, que no depende de él.
- **Sus fallos se comportan como los de un step.** Si el bloque compartido falla, se borran
  sus huellas y el step no continúa a la fase de ambiente.
- **No es un directorio en `steps/`.** No se declara aparte: son los comandos del propio step
  marcados con `scope: shared` (P2). Un step puede no tener fase compartida en absoluto —el
  caso normal—, y entonces todo lo de esta entrada no aplica.
- **Lo que produce la fase `shared` queda en el mapa acumulado** para la fase de ambiente del
  mismo step y para todos los steps posteriores, exactamente como hoy.
- **Cada fase se nombra `<NN-nombre>/<ámbito>`** — `02-supply/shared`, `02-supply/prod`.
  Conserva el orden legible y coincide con lo que hay en disco.

**Dimensionarlo bien.** P5 no formaliza algo que el motor ya haga a medias: parte la unidad
de decisión de `step` a `step × ámbito`, y hoy esa granularidad **no existe** —la decisión
es todo o nada por step (sección 7.5)—. Toca la clave de re-ejecución, la construcción de
la decisión, el orden de ejecución y la partición de la huella de variables, que hoy se
calcula sobre un único mapa acumulado indivisible. Es capacidad nueva sobre una base
existente, no un ajuste.

**P6 — Ventana de reutilización del clon del pipeline.**
Clonar la primera vez y considerar el clon válido durante un tiempo configurable (24h por
defecto) antes de volver a clonar. Incluye el caso contrario: si el remoto no responde,
usar el clon viejo y avisar, en vez de fallar.

Reutilizar un clon viejo cambia qué se ejecutó, así que arrastra dos condiciones:

- **La identidad se calcula sobre la fuente realmente usada**, nunca sobre la que se pidió.
  Se resuelve primero qué clon se va a usar, y se identifica después.
- **El hecho queda registrado** (P11), con la antigüedad del clon. Que la explicación de
  por qué un despliegue corrió contra otra versión del pipeline no dependa de la memoria de
  nadie.

**P7 — El consumidor declara de dónde viene cada variable.**
Hoy una variable producida por un step aparece de la nada: `02-supply` la extrae del stdout
y la inyecta en un espacio global plano; dos steps más tarde `03-package` la interpola por
nombre. El consumidor **no declara su origen** — solo la nombra. El acoplamiento es por
nombre, en un espacio plano, resuelto por orden de ejecución.

Pasa a declararse en el propio `variables/<ambiente>/<step>.yaml`:

```yaml
- name: "DB_HOST"
  resolve: env-state
  key: "lb-arn"
```

Por qué importa más allá de la higiene: es lo que permite conocer la identidad de una
ejecución **antes** de ejecutarla. Un valor producido en runtime no se puede saber por
adelantado, pero **la declaración de cómo se obtiene sí**. De ahí la regla: en la identidad
entra la declaración, nunca el valor; el valor resuelto se registra aparte, como hecho
(P11). Y la declaración tiene que ser **específica** —qué fuente, qué clave— nunca un
genérico «esto se resuelve luego», que no discrimina nada y volvería la identidad inútil.

**Convive con los valores literales**, que siguen existiendo y siguen rigiéndose por P3.
`resolve` no los reemplaza: cubre lo que un literal no puede expresar — un valor que no se
conoce hasta que algo lo produce.

Consecuencias, en orden de tamaño: `variables/<ambiente>/<step>.yaml` necesita gramática
nueva y, por tanto, **versión de esquema propia** —hoy no tiene ninguna—; se invierte el
modelo de variables, que es el cambio más grande que trae la decisión; y hay que migrar los
tres templates, donde cada `${var.X}` que hoy consume el output de otro step pasa a
necesitar declaración.

**Es precondición, no acompañamiento.** Mientras el consumidor no declare su origen, no hay
forma de distinguir «parámetro con valor conocido» de «parámetro que se resolverá», y la
identidad quedaría incompleta **en silencio** — que es la peor manera de quedar incompleta.

**P8 — Las variables con efecto real no son caché.**
La prueba para clasificar una variable: si se borra el almacén y el step vuelve a correr
desde cero, ¿el valor sale idéntico o puede salir distinto?

- **Determinista** —un hash de artefacto, un cálculo sobre las entradas del step— →
  pertenece al **caché**, y es desechable.
- **Depende de un efecto real** —un ARN, una IP asignada, un recurso creado en la nube— →
  **no**. Borrar el caché no puede implicar perder de vista un recurso que sigue existiendo,
  ni crear uno duplicado la próxima vez.

Hoy el motor no distingue: persiste el acumulado completo no volátil en un mismo almacén
(sección 5.2), incluidas las variables extraídas del stdout — que es exactamente de donde
sale un ARN. Se separan en dos tiendas con reglas opuestas: el caché se borra entero sin
consecuencias, el estado no se borra nunca.

La clave del estado es **`(proyecto, ámbito, nombre)`**, sin el step — lo cual no lo quita,
lo *reconoce*: hoy ya se guarda el acumulado completo bajo el nombre de cada step, así que
el archivo de `deploy` contiene lo que produjo `test`.

> **P16 fija la clave, y no es esta.** Es `(subject, scope, step_id)`: **con** el step y
> **sin** el nombre de la variable, que pasa a vivir dentro del registro. El step vuelve a la
> clave porque el hecho que se guarda es «este step corrió», y eso es de un step concreto.
> Y el pipeline queda **fuera**, con la pregunta de 9.3 cerrada por el argumento de P16.
>
> Lo que P8 aporta y P16 no deroga es **el criterio de clasificación** —determinista vs.
> efecto real— y la conclusión que se sigue de él: mientras no haya información para
> clasificar, todo se guarda en la tienda que no se borra. Ante la duda, no desechable.

**P9 — La identidad del código es la huella de su contenido; el commit es metadato.**

```yaml
source:
  vcs: git
  fingerprint: "sha256:9b1c…"   # identifica
  commit: "a3f21c8e…"           # documenta
```

**Por qué no `commit` + «sucio».** Dos árboles con cambios sin comitear completamente
distintos producirían la **misma** identidad: «sucio» dice que algo cambió, no dice qué. Es
una primitiva de identidad que no identifica, justo en el caso cotidiano —en local el
proyecto *es* el árbol de trabajo real, sucio la mayor parte del tiempo—. Y como la
identidad y la clave de re-ejecución comparten material (P10), el defecto no produce un
historial impreciso: produce **un caché que sirve resultados de un árbol distinto al que
está en disco**.

**Por qué no ambos.** La huella ya determina completamente si dos árboles son iguales. El
commit no añade discriminación, añade procedencia — es un puntero a una *posición en la
historia*, la misma categoría que el resto de lo que ya se excluye. Dos árboles idénticos
con distinto commit de origen (rebase, cherry-pick, dos clones de remotos distintos)
tendrían identidades distintas, rompiendo la comparabilidad que es lo que le da valor al
identificador.

**Se aplica igual al repo de pipeline**, sin excepción. Hoy siempre se clona limpio, pero
el día que alguien itere localmente sobre un pipeline antes de comitear —flujo deseable, no
rareza— reaparece el mismo problema. Una sola regla evita una asimetría que solo existe
porque hoy nadie desarrolla pipelines en local.

**La regla se congela y se publica.** El riesgo real no es «huella contra commit»: es que un
algoritmo a medida es difícil de reproducir bit a bit por un tercero. Semántica de
`.gitignore`, orden de los archivos, separador de concatenación, normalización de rutas —
cualquier divergencia silenciosa produce huellas distintas para lo que un humano llamaría
«el mismo código». La solución no es sacrificar precisión por reproducibilidad: es
especificar. **Regla versión 1, congelada, con vectores de prueba** que permitan a una
implementación independiente validarse.

> **Hecho (spec 08).** La regla vive en `internal/domain/fingerprint/SPEC-v1.md`, con 16
> vectores que se ejecutan en memoria. La regla canónica se separó del recorrido de disco
> —`Compute` sobre el puerto `TreeSource`—, lo que es exactamente lo que permite que los
> vectores sean verificables sin replicar un árbol de archivos. Se corrigieron tres
> defectos que producían falsos negativos del caché (permisos, enlaces, escapes del
> parser) y se congelaron dos divergencias respecto de git que no los producen
> (precedencia de reglas y anclado por prefijo), documentadas. Lo que queda de P9 es
> aplicar la misma huella al repo de pipeline (spec 18) y sustituir el commit por ella en
> el registro.

**P10 — El estado de re-ejecución pasa a estar direccionado por contenido. — HECHO (spec 10),
y luego SUPERADO por P16.**
Implementado tal cual, incluidas las entradas de sola presencia. El cuerpo de este documento
lo describe en la sección 7; lo que queda aquí es el registro de lo que se decidió y por qué.

> **Qué de P10 sobrevive a P16, que es más de lo que parece.** Se retira el
> **direccionamiento por contenido de la clave** —la clave vuelve a ser posición— y con él
> el beneficio de «volver atrás acierta». Sobreviven las tres cosas que eran el fondo: que el
> **ambiente sea parte de la identidad por derecho propio** (ahora en la clave, donde no
> puede caerse de un hash), que el **tiempo no sea identidad** (ahora ni siquiera metadato:
> se declara, P14), y que la escritura ocurra **una sola vez desde el camino de éxito**.
>
> Y sobrevive entero §5.3bis, que es lo que P1 necesitaba: el vocabulario de steps lo abrió
> la spec 10 al borrar el `switch`, y nada posterior lo vuelve a cerrar.

Las cuatro huellas sueltas con cuatro claves distintas (sección 7.3) se unifican en **una
entrada bajo una clave derivada del contenido**: instrucciones + variables + código +
ambiente (P4) + ámbito (P5). Tres cambios de comportamiento:

- **Volver atrás acierta.** Hoy, alternando entre dos estados A y B, *cada* cambio
  re-ejecuta, porque lo guardado es siempre lo último escrito. Con la clave derivada del
  contenido, volver a A encuentra la entrada de A.
- ~~**La entrada se escribe después del éxito, no antes.**~~ **Hecho (spec 09).** La huella
  se escribía durante la evaluación y un borrado compensatorio la revertía si el step
  fallaba; si el proceso moría duro —`SIGKILL`, OOM; ya no un `Ctrl-C`, que desde la spec 07
  cancela el contexto y sí pasaba por el compensador— ese borrado nunca corría y **la
  siguiente ejecución saltaba un step que jamás terminó**. Ahora `Evaluate` es una consulta
  que devuelve lo observado, y la escritura ocurre una sola vez desde el camino de éxito del
  step: el compensador desapareció y la ventana con él (sección 7.3). Lo que la 10 hereda es
  la clave, no el momento.
- **El tiempo deja de ser identidad.** El TTL no es una propiedad del contenido, así que no
  entra en la clave: pasa a ser metadato de expiración de la entrada.

Alcance honesto de la primera versión: las entradas son **de sola presencia**. El contenido
reutilizable se queda donde está hasta que P8 se diseñe por completo; el beneficio de los
tres puntos anteriores se mantiene igual.

**P11 — El motor deja rastro consultable de lo que hace.**
Hoy, al terminar una ejecución, lo único que persiste son valores de variables y huellas de
re-ejecución, ambos sobrescritos. Lo que el usuario ve son líneas de log en texto libre que
además son **descartables por diseño**. No hay contra qué comparar nada, y por eso ninguna
pregunta comparativa —«¿esto ya corrió?», «¿qué cambió desde la última vez?»— tiene hoy
respuesta posible.

Se añaden dos piezas:

- **Un objeto por operación pedida.** Pedir `deploy` es **una** intención y produce **un**
  objeto, aunque internamente corran cuatro steps. Los steps **no tienen identidad propia en
  el registro**: son estructura interna, visible a través de los eventos. Su identidad
  deriva del contenido (P9, P7), así que se conoce **antes** de ejecutar — que es
  precisamente lo que permite preguntar «esta misma configuración, ¿ya corrió?» sin ejecutar
  nada.
- **Eventos append-only como verdad.** El resultado no se guarda: se **pliega** desde los
  eventos. Es lo que hace que una ejecución interrumpida deje datos útiles en vez de nada, y
  no es hipotético: en remoto la máquina es efímera, en local el usuario puede matar el
  contenedor.

Dos reglas de disciplina que gobiernan todo el vocabulario: **guardar hechos, nunca
conclusiones** —«no se sabe» es una respuesta legítima y preferible a forzar una
clasificación—, y **los valores nunca entran al registro**: solo nombres y hashes, para que
el registro pueda salir de la organización.

Las tiendas resultantes no son intercambiables, y la diferencia es el punto: objetos y
eventos son **permanentes**; el caché (P10) es **desechable** —borrarlo entero no pierde un
solo hecho histórico—; el estado (P8) **no se borra nunca**.

**P12 — El destino del estado es configuración explícita, no un modo. — HECHO (spec 16),
salvo la sincronización.**
Hasta la spec 16 una bandera `--mode local|remote` decidía dónde se escribe, y `remote`
asumía una plataforma concreta. Eso ataba el motor a un proveedor y contradecía el resto del
diseño, que es agnóstico de dónde corre.

> **Lo implementado y lo que queda.** Se retiraron `--mode` y las seis banderas de endpoints,
> el contrato de invocación subió a `schema_version: 2`, el adaptador de Supabase del almacén
> desapareció y `linkVexHome` —que borraba el `~/.vex` real de quien ejecutara el binario
> fuera del contenedor— dejó de existir. Las tres primeras reglas de abajo están en el
> código, incluida la del área de trabajo. **La cuarta —sincronizar por step con reintento
> acumulativo— no**: hoy el destino se escribe directamente y `staging/` se resuelve y se
> crea sin que nadie escriba en él todavía. Es lo que hace la spec 21, y `type: http` sigue
> congelado hasta la 26. Del lado de fuera, el CLI `vex` (spec 23) y las edge functions
> (spec 26) están **rotos a propósito** hasta que pasen la configuración.

El motor deja de saber de modos. Recibe siempre una configuración de destino, la invoque
quien la invoque:

```yaml
type: local | http
local:
  path: /mnt/vex-state
```

Cuatro reglas, y ninguna es un detalle de implementación:

- **El motor nunca tiene un default propio.** Un default implícito haría que el motor se
  comportara distinto según quién lo invocó, sin que quede escrito en ningún lado. Si falta
  la configuración, falla. El default vive **en quien invoca**.
- **El motor sí es dueño de su área de trabajo.** Escribe siempre en un directorio propio,
  sin depender de que haya volumen montado. Eso no es una opción de `type`: **registrar es
  incondicional**. La configuración decide únicamente si, *además*, hay un destino al que
  empujar lo ya escrito.
- **`local` no es un no-op.** Si el volumen no está montado, falla ruidosamente. Un no-op
  dejaría al motor escribiendo en un filesystem efímero sin ninguna señal — pérdida de datos
  invisible. `local` y `http` son el mismo mecanismo con el mismo tratamiento de error;
  ninguno es «el normal».
- **Se sincroniza por step, con reintento acumulativo.** Por evento penaliza la ejecución con
  red constante; solo al final concentra todo el riesgo en un punto, justo en el escenario de
  fallo que más interesa conservar. La correctitud **no depende de ningún punto de control**:
  cada hecho tiene identidad estable, así que reenviar lo mismo dos veces no duplica nada.
  Si un envío falla, el siguiente step recoge lo pendiente sin lógica extra — y ese fallo
  queda registrado como hecho, para que un hueco en el destino se explique en vez de
  descubrirse por casualidad.

**La transición rompe a propósito.** Se retiraron `--mode` y las **seis** banderas de
endpoints de una vez —`--status-endpoint` y `--log-endpoint` sobreviven: el primero
transporta el estado terminal y es la única señal de que el contenedor terminó—, y la versión
del contrato de invocación subió a 2: el motor rechaza clientes viejos **por contrato, no por
memoria de nadie**. `type: http` queda congelado hasta que exista un cliente real
esperándolo.

> **Con P16 eso deja de ser solo rendimiento.** Lo que cruza al destino pasa a ser
> estado que **no se borra nunca**, así que un modo remoto sin destino configurado
> no pierde velocidad: pierde de vista un recurso creado en la nube. Es
> exactamente la razón de la tercera regla de arriba —`local` no puede ser un
> no-op—, y con P16 gana peso.

**P13 — El ámbito lo declara el step, y un step tiene exactamente uno. — HECHO (spec 13).**
El cuerpo de este documento lo describe en la sección 8; lo que queda aquí es el registro de
lo que se decidió y por qué.

> **Qué quedó fuera, y es lo único.** La lectura mira los dos ámbitos y la escritura sólo el
> declarado, como se decidió. Lo que **no** ocurre todavía es que un step `scope: project`
> revive entre ambientes: su huella lleva el ambiente por dos vías —la dimensión `ámbito` de
> la clave de caché y la variable `environment` del mapa acumulado—, así que se ejecuta una
> vez por ambiente aunque escriba y lea en el mismo sitio. La lectura cruzada, que es lo que
> el caso del ACR necesita, sí funciona. Lo cierra P16 (spec 27).
>
> Y en producción el cambio observable es **cero** hasta la spec 24: ningún template declara
> `config.yaml` todavía, así que sus steps se ejecutan siempre y no persisten registro.

Cada step declara en un `config.yaml` propio a qué **ámbito** pertenece: `project` —el
trabajo es común a todos los ambientes del proyecto— o `environment` —es propio del
ambiente en ejecución—. Vocabulario cerrado; no hay tercer valor.

```yaml
# steps/01-acr/config.yaml
scope: project
```

**La regla dura: nunca dos.** Si un step, tal como se pensó, necesita ambos ámbitos, se
parte obligatoriamente en dos steps secuenciados:

```
steps/01-acr/          scope: project        (crea el registro de contenedores)
steps/02-provision/    scope: environment    (crea recursos del ambiente)
```

Tres cosas que esta regla **no** necesita, y que P2+P5 sí necesitaban: ninguna declaración
de dependencia —el orden numérico ya secuencia—, ningún identificador compuesto, y ninguna
regla de precedencia dentro del step.

Tres consecuencias, y la tercera es la que nadie pedía y sale gratis:

- **Un step vuelve a tener una identidad**, porque vuelve a tener un solo criterio de
  cambio. Es lo que hace posible P15.
- **La palabra `shared` deja de existir en el motor**, y con ella la reserva de
  `environments.yaml` (D9). El ámbito de ambiente viaja siempre prefijado
  —`environment:<nombre>`—, así que un ambiente llamado `project` da `environment:project`
  y no colisiona con nada. **No queda ninguna palabra reservada.**
- **`isShared` desaparece de `Variable`.** El ámbito es del step, así que la marca por
  variable era información duplicada — y fue la fuente del defecto que la spec 02 tuvo que
  corregir.

**Un step sin `config.yaml` se ejecuta siempre y no persiste nada.** Ejecutar es el default
seguro; no persistir es lo que impide inventarle un ámbito, que sería la deducción implícita
que P13 retira, solo que en otro archivo. Mismo criterio que un step sin comandos (D2).

**P14 — Las reglas de re-ejecución las declara el step.**
El mismo `config.yaml` declara **bajo qué reglas** el step debe volver a ejecutarse. Es la
mitad de P1 que la spec 10 dejó abierta: aquella abrió el vocabulario de steps borrando el
`switch` cableado, y a cambio dejó una sola política —«todo importa siempre»— que es segura
y cara.

```yaml
rules:
  - state_changed: [pipeline]   # invalidación: ¿cambió la huella del step?
  - max_age: 24h                # expiración: ¿lleva demasiado sin revisarse?
```

Cinco reglas de semántica, y ninguna es obvia:

1. **Dos categorías, no una lista.** *Invalidación* afirma que lo guardado ya no es
   correcto; *expiración* fuerza una revisión periódica sin importar si algo cambió. Toda
   regla futura se clasifica en una de las dos —o en una tercera, *anulación explícita*, si
   se añade forzar a mano—. Mezclarlas fue lo que produjo el TTL global.
2. **Se combinan con OR.** Cada regla es una razón independiente para desconfiar de lo
   guardado.
3. **`state_changed` no es implícito.** Un step con `config.yaml` que no lo declare no lo
   evalúa: puede revivir aunque su huella haya cambiado. Es válido y **se avisa**, no se
   prohíbe.
4. **Sin reglas —o con `rules: []`— se ejecuta, nunca revive.** El OR de un conjunto vacío
   es falso, pero «no hay nada que comprobar» no es «esto está al día». Es exactamente el
   defecto que la spec 05 corrigió, y la forma nueva de volver a caer en él.
5. **El TTL global se retira.** Un step sin `max_age` **no caduca**. Los 30 días que la
   spec 10 extendió a todos los pasos vuelven a declararse donde se sabe: en el
   pipelinecode.

**P15 — La huella de un step incluye todo lo que declara y nada de lo que produce.**
Hay **tres identificadores** y son independientes entre sí:

| Identificador | Responde | Cambia cuando |
|---|---|---|
| `content_id` | ¿Qué se pretende ejecutar, en conjunto? | cambia cualquier campo del contenido |
| `deployment_id` | ¿En qué posición de la historia de este ambiente cae? | **en cada ejecución nueva**, aunque nada haya cambiado |
| `step_fingerprint` | ¿Cambió el trabajo de este step? | solo cuando su declaración cambia |

**Regla de independencia:** ninguno se calcula a partir de otro. En particular, la decisión
de saltar o ejecutar un step **nunca** consulta `deployment_id` ni `content_id` — y no
podría, porque el primero cambia siempre.

```
step_fingerprint(step) = sha256( huella_pipeline(step) [, huella_proyecto] )

huella_pipeline(step)  = sha256( commands.yaml, config.yaml, variables declaradas,   ← normalizados
                                 el resto del árbol del directorio del step )        ← crudo
```

Cuatro decisiones dentro de la fórmula:

- **El cuerpo de las plantillas entra**, y con él todo lo demás que hay en el directorio del
  step. Es lo que cierra D14: hoy editar un `deployment.yaml` hace que el step se salte. Y
  entra el **directorio entero**, no solo lo declarado en `templates:` — esa lista dice qué
  se *interpola*, no qué *importa*: un `.tf` que nadie declara decide qué se provisiona.
- **`config.yaml` entra entero**, `scope` incluido. Si no entrara, cambiar las reglas de un
  step no movería su huella y el step se saltaría **con las reglas viejas**. En toda la
  huella hay **una sola exclusión justificada**: `description`, que ni siquiera llega al
  modelo de ejecución.
- **La `dirección` que el ámbito determina nunca entra.** Y aquí hay que separar dos cosas
  que se llaman igual: `scope: project` es un **valor declarado** y se hashea como cualquier
  campo; la **ruta** `state/<subject>/project/<step_id>/` es la clave, y no entra en ningún
  hash. Si la ruta entrara, dos proyectos con el mismo pipelinecode tendrían huellas
  distintas por vivir en directorios distintos, y la pregunta «¿este contenido ya corrió en
  algún sitio?» dejaría de tener sentido.
- **La huella del proyecto es condicional.** Entra solo si el step lo declara. Un step que
  crea un registro de contenedores no depende del código de la aplicación, y re-ejecutarlo
  en cada commit vaciaría de sentido haberlo separado. Es la granularidad que la spec 10
  sacrificó, ahora declarada.

**Y el material de variables pasa de resuelto a declarado**, que es lo que cierra el rebote
de más: hoy la huella mezcla lo que el step consume con lo que produce, así que **todo step
con un output se re-ejecuta exactamente una vez de más**. En la identidad entra la
declaración, nunca el valor — la misma regla de P7, aplicada aquí.

**Se normaliza lo que el motor entiende y se hashea crudo lo que no.** Los tres archivos de
declaración se parsean a estructura canónica —claves ordenadas, sin comentarios— para que un
comentario no dispare una re-ejecución; el resto del árbol son archivos arbitrarios que el
motor no sabe interpretar, y normalizarlos exigiría un parser por formato.

**P16 — El estado de un step es append-only, y la dirección sale del hash.**
La clave deja de derivarse del contenido y pasa a ser **posición**:

```
clave_estado = (subject, scope, step_id)
```

donde `scope` es `project` o `environment:<nombre>`. Bajo esa clave se **añade** un registro
por cada ejecución real del step —nunca cuando revive— y **nunca se sobrescribe ninguno**:

```yaml
record_id: 01J9X3QK8F7…        # ULID: circunstancial, no de contenido, y ordenable
step_fingerprint: sf-v1:…      # lo que se compara en la próxima evaluación
variables:                     # lo que este step produjo
  ACR_NAME: acmeregistry.azurecr.io
produced_by: deployment_id/attempt   # trazabilidad — nunca decide nada
```

Cinco consecuencias:

- **Append-only es la precondición de P17**, y esa es toda su justificación. Guardar historia
  cuesta espacio; lo que compra es poder anclar un rollback a un punto en el tiempo.
- **El aislamiento entre ambientes se conserva por otra vía.** P4 lo compró metiendo el
  ambiente en la huella; ahora viaja en la clave. La garantía es la misma —«¿esto ya se
  ejecutó *aquí*?»— y ya no puede caerse de un hash por descuido, porque no está en ninguno.
- **El pipeline sale de la clave, y D-A14 se cierra al revés.** El modo de fallo que motivó
  meterlo —leer un valor ajeno en silencio— lo cierra la huella: revivir exige igualdad de
  `step_fingerprint`, que incluye el pipelinecode entero. Manda entonces el argumento
  conceptual, que siempre estuvo del otro lado: el ACR pertenece al proyecto.
- **El caché no desaparece: cambia de papel.** Pasa a ser un **índice** desechable y
  derivable que responde «¿este contenido ya corrió alguna vez?». **No participa en ninguna
  decisión**, y esa es su definición: borrarlo entero no cambia una sola decisión del motor.
- **Se pierde «volver atrás acierta».** La comparación es contra el **último** registro, así
  que `A → B → A` re-ejecuta, donde la spec 10 acertaba. Es una regresión declarada y segura
  —ejecutar de más nunca omite un despliegue—, y el índice es lo que la haría barata de
  revertir.

**P17 — Un rollback ancla en una ejecución pasada, no en el estado de hoy.**
Un rollback es una ejecución **nueva** —`content_id` idéntico, `parent` distinto y por tanto
`deployment_id` distinto— que consulta los registros de estado correspondientes a una
ejecución exitosa anterior **específica**, no los más recientes.

Que `content_id` coincida es lo que garantiza que es idéntica a la original en todo lo que
importa, y es la mejor prueba de que separar las dos identidades era necesario.

- **Destino válido = todos los steps terminaron correctamente.** Un intento con steps
  fallidos o no alcanzados no lo es, y se rechaza antes de empezar. **Un step revivido
  cuenta como correcto**: revivir afirma que el trabajo estaba hecho, y lo contrario haría
  que una ejecución fuera peor destino cuanto mejor funcionó el motor.
- **El ancla se lee, no se deriva.** Los eventos de la ejecución destino dicen qué registro
  estuvo vigente en cada step. Derivarlo por fechas falla en cuanto dos ejecuciones se
  solapan o un reloj va mal, y sería guardar una conclusión en vez de un hecho.
- **No relaja ninguna comprobación.** Mismo bucle, mismas reglas; solo cambia con qué
  registro se compara. Si el trabajo cambió, el step se ejecuta.
- **Límite conocido y no resuelto:** si algún recurso cambió por fuera del control de Vex, el
  rollback no lo detecta ni lo revierte — opera sobre el estado que Vex conoce, no sobre el
  mundo. Es la tensión de cualquier herramienta de infraestructura como código, y atribuirle
  esa garantía al rollback sería la peor forma de documentarlo.

### 9.1' El bucle de decisión, con todo lo anterior

Por step, en cada ejecución. Es el resumen ejecutable del rediseño:

```
1. Leer config.yaml.  No existe → EJECUTAR, ir a 5
2. Calcular step_fingerprint (P15)
3. Leer el ÚLTIMO registro de clave_estado (P16)
      no hay registro   → EJECUTAR
      no se pudo leer   → EJECUTAR, con advertencia
4. Evaluar las reglas declaradas, combinadas con OR (P14)
      alguna verdadera  → EJECUTAR
      ninguna verdadera → REVIVIR: cargar las variables del registro,
                          emitir el hecho, no ejecutar nada
5. EJECUTAR los comandos del step
6. Al terminar con éxito, añadir un registro NUEVO
```

Ningún paso consulta `deployment_id`, `content_id` ni `parent`.

### 9.2 Defectos conocidos

| # | Qué pasa |
|---|---|
| D1 | ~~Si un step falla, **las plantillas interpoladas no se restauran**: quedan con los valores sustituidos dentro de la copia de trabajo~~ **Corregido (spec 06 §5.1): la limpieza del Template Method está en `defer`, así que corre falle o no la ejecución. El error de `exec` sigue siendo el que manda —el de la restauración se acompaña con `errors.Join` en vez de descartarse— y `step_workdir` deja de quedar huérfano en el mapa acumulado tras un step fallido. No cubría la muerte dura del proceso; desde la spec 07 un `Ctrl-C` ya no lo es —cancela el contexto y sale por ese mismo `defer`—, así que lo que queda fuera es `SIGKILL` y OOM (opción D de la spec 06, diferida)** |
| D2 | ~~Un step sin `commands.yaml` —o con el archivo vacío— se registra como éxito. «Step vacío» y «step ejecutado» son indistinguibles~~ **Corregido (spec 04 §5.3, D-A12): es `skipped{reason: no_commands}`, y además deja de persistir estado de re-ejecución** |
| D3 | ~~Un directorio de step con prefijo de un solo dígito (`2-supply`) hace que el motor busque `02-supply/commands.yaml`, no lo encuentre, y el step pase sin ejecutar nada (efecto de D2)~~ **Corregido (spec 04 §5.1): el prefijo se valida a exactamente dos dígitos en el validador de estructura y en `NewStepName`, así que ese directorio ya no puede existir sin que la ejecución falle nombrándolo** |
| D4 | ~~Las escrituras del almacén y del estado no son atómicas. Una interrupción a mitad deja el archivo truncado, y el motor lee un archivo truncado como «no hay nada»: el valor se pierde en silencio~~ **Corregido en dos tiempos. La spec 02 hizo atómicas las escrituras (temporal + `fsync` + rename) y convirtió el archivo truncado en un error reportado. La spec 11 elimina la clase entera del almacén de estado: append-only significa que cada archivo se escribe UNA vez y no se toca nunca más, así que no puede haber uno a medio reescribir. Queda una lectura tolerante y es deliberada: en el índice de `cache/`, ilegible ⇒ ausente, porque no decide nada y se reconstruye solo** |
| D5 | ~~Si una comprobación no consigue guardar su huella, el step se marca para ejecutar y se ejecuta, pero la huella nunca se escribe — y la corrida siguiente vuelve a encontrar la vieja. Se re-ejecuta indefinidamente, sin señal~~ **Corregido (spec 09 §5.1–5.3, R-23/R-20): la comprobación ya no escribe, así que su respuesta no puede quedar contaminada por el fallo de una escritura. Un fallo de infraestructura se responde `undetermined` —«no pude averiguarlo»— en vez de disfrazarse de «cambió», y si falla la escritura posterior al éxito del step se emite una advertencia explícita. La re-ejecución de más se conserva a propósito (fail-open); lo que se elimina es que sea silenciosa** |
| D6 | ~~La variable `shared_workdir` está declarada en el motor y no se asigna nunca~~ **Corregido (spec 11 §5.6): se elimina. Una variable que no existe no puede ser volátil ni no volátil, y conservarla era invitar a que alguien la asignara sin saber qué significaba** |
| D7 | ~~**El orden de ejecución de los steps es lexicográfico, no numérico:** `os.ReadDir` ordena por nombre y `StepName.Order()` no se usa jamás para ordenar. Con prefijos de **dos** dígitos ambos órdenes coinciden y no se manifiesta —doce steps `01…12` salen en orden, verificado con test—. Se manifiesta en cuanto entra un prefijo de **un** dígito (D2/D3): `1-test, 10-promote, …, 2-supply`, y basta un directorio mal nombrado entre otros correctos~~ **Corregido (spec 04 §5.2): `NewStepNames` ordena por `Order()`. Con el prefijo validado a dos dígitos no había un orden roto que arreglar; lo que se gana es que el invariante esté enunciado en una línea en vez de deducido de que `os.ReadDir` ordena por nombre y de que `%02d` es de ancho fijo** |
| D8 | Un campo vacío en los datos del proyecto produce una variable bajo la clave `""`: el error de construcción se ignora y la variable vacía se inserta igual. Esa entrada **entra en la huella de variables**, así que no es un log feo — es material de identidad contaminado. **Corregido al implementar (spec 03 §9.1): el mecanismo era real, el disparador no.** Ningún campo vacío del proyecto llega al handler —`create_execution.go:74-100` los valida antes—, así que la entrada anónima solo la producía un defecto del propio motor. El daño cotidiano era el inverso: un `variables/<env>/<step>.yaml` con `value: ""` **abortaba la ejecución entera** |
| D9 | ~~Un ambiente llamado `shared` en `environments.yaml` pisa el almacén compartido. El ambiente se valida contra la lista, pero no contra nombres reservados~~ **Corregido (spec 04 §5.4): el handler 03 rechaza el `environments.yaml` que declare `value: "shared"`. Y luego DEROGADO (spec 13 §5.5): el ámbito de ambiente viaja prefijado (`environment:<nombre>`), así que la colisión dejó de existir y la comprobación del handler 03 se retiró. **No queda ninguna palabra reservada**, ni siquiera `project`. La corrección no fue inútil —prohibió un nombre mientras la colisión existió—; lo que la deroga es haber eliminado la colisión** |
| D10 | ~~Un `Ctrl-C` mata la ejecución sin dejar rastro de cancelación: no hay manejador de señales y el mecanismo de cancelación que el motor declara no se invoca nunca. «Cancelado» e «interrumpido» son indistinguibles~~ **Corregido (spec 07 §5.4): `cmd/vexd` maneja `SIGINT`/`SIGTERM` cancelando el contexto de la ejecución, que queda registrada como `canceled` y sale con exit code 130. Es best-effort y así se documenta (sección 5.4): `SIGKILL` y OOM siguen plegando a «interrumpido», que es la respuesta honesta. De paso, el `cancelFn` que se guardaba sin llamar nunca obtuvo su `defer`** |
| D11 | ~~Si el identificador de ejecución tiene menos de cuatro caracteres, la línea de log que lo abrevia provoca un panic~~ **Corregido (spec 07 §5.5): se abrevia solo a partir de ocho caracteres. El plan lo describía como «menos de 8»; el código panicaba con menos de 4 y solapaba las dos mitades entre 4 y 7** |
| D12 | ~~El estado de la ejecución no se usa: `status` se queda en `queued` de principio a fin, `finishedAt` y `exitCode` son siempre `nil`, y el estado terminal lo deduce la CLI a partir del error devuelto~~ **Corregido (spec 07 §5.2): el use case invoca las transiciones y el agregado publica su estado terminal con sus instantes. Ver sección 5.4** |
| D14 | **El cuerpo de las plantillas no participa en ninguna huella.** `templates:` aporta las *rutas* al material de instrucciones; el contenido de esos archivos no entra ahí, y la huella del árbol es la del **proyecto**, no la del pipelinecode. Consecuencia reproducida en el harness: **editar `steps/02-supply/k8s/deployment.yaml` y volver a ejecutar hace que el step se salte** — el despliegue no ocurre y el motor dice «sin cambios», sobre el archivo que más se edita a mano de todo el pipelinecode. Es la forma exacta del criterio de corrección de la spec 08 («lo que cambia el despliegue sin cambiar la huella») y la misma clase que el `chmod` que aquella arregló. **No lo introduce el direccionamiento por contenido**; lo hace visible haber escrito por primera vez qué entra en cada huella. **Arreglo decidido (P15):** entra el árbol del **directorio del step** — ni el pipelinecode entero, que haría que tocar `09-notify` moviera la identidad de `01-test`, ni solo las plantillas declaradas, que dejaría fuera un `.tf` que nadie declara. Cambia todas las huellas emitidas una vez |
| D13 | ~~Dos relojes en la misma función: el cálculo de versión tomaba el instante de `time.Now()` en una rama y del `startedAt` de la ejecución en la otra, para la misma decisión~~ **Corregido (spec 07 §5.1, R-25): el puerto `Clock` es la única fuente de instantes del dominio, y las dos ramas la comparten** |

### 9.3 Decisiones abiertas

No están decididas, y por eso no aparecen arriba. Se listan para no darlas por supuestas al
leer las entradas que dependen de ellas. El detalle y el estado vivo están en
`vex-plan-revision.md` §6.

| Pregunta | Afecta a |
|---|---|
| ~~¿La clave del estado incluye el pipeline?~~ **Cerrada por P16: no.** El modo de fallo que lo justificaba —leer un valor ajeno en silencio— lo cierra la huella, no la clave | P8, P16 |
| Forma concreta y versión de esquema de `variables/<ambiente>/<step>.yaml`: qué vocabulario admite `resolve`, y cómo se negocia la versión | P7 |
| ¿Son legítimos los valores de variable vacíos? Hoy se prohíben, lo que hace inexpresable un parámetro opcional | P7 |
| Un hash de un parámetro de baja entropía es reversible por fuerza bruta (`REPLICAS=3`), lo que contradice la promesa de «solo hashes, nunca valores». ¿Sal por proyecto? **P15 reduce la superficie** —lo hasheado son literales del pipelinecode, no valores producidos en runtime— pero no la elimina | P11, P15 |
| Reglas del enmascarado de valores en los extractos: longitud mínima y orden de sustitución. **Se aplican también al campo `variables` del registro de step**, que puede llevar el valor en claro y puede sincronizarse a un destino remoto | P11, P16 |
| Duración de la ventana del clon viejo, y si es configurable por proyecto | P6 |
| **Retención de los registros de step.** Crecen sin límite: uno por ejecución real de cada step, para siempre. Aceptable durante mucho tiempo —son JSON pequeños—, pero cualquier política tiene que responder antes **hasta dónde debe alcanzar un rollback**, que es decisión de producto y no de almacenamiento | P16, P17 |
| **`.git` como archivo en un worktree** rompe la comparabilidad de la huella entre máquinas. Con P15 la regla de árbol se aplica ahora a **dos raíces** —el proyecto y el directorio del step—, así que la superficie del defecto crece. Corregirlo es una `v2` de la regla, y debe decidirse antes de emitir el primer `content_id` — **ahora con dueño y fecha escritos**: `internal/domain/deployment/SPEC-CONTENT-v1.md` §9 lo declara defecto heredado de la regla `cnt-v1` y lo sitúa antes de la spec 18, que es la que emite | P9, P15 |
| ~~**¿`record_id` es ULID o UUIDv7?** Misma pregunta que `event_id`~~ **Cerrada por P11 y P16, y NO con la misma respuesta.** `record_id` es un **ULID** (spec 11, implementada): su orden lexicográfico es el temporal y de ahí sale «el último registro se obtiene sin leer ninguno», porque los nombres de archivo se ordenan solos. `event_id` es un **UUIDv7** (spec 17, implementada): `google/uuid` ya era dependencia, así que ULID añadía una sin aportar nada, y el orden de los eventos no lo da el identificador sino `seq`. Dos identificadores circunstanciales con dos formatos, cada uno por su razón | P11, P16 |

---

## Resumen del flujo end-to-end

1. El motor recibe un **ambiente** y un **step**.
2. Clona el proyecto y el pipeline, y copia el pipeline a un directorio de trabajo propio de
   la terna (proyecto, pipeline, ambiente).
3. Calcula la versión del proyecto, inyecta las variables iniciales y computa la huella del
   código.
4. Recorre los steps desde el primero hasta el pedido.
5. Por cada step: carga las variables del último registro de cada ámbito (proyecto y
   ambiente), carga y resuelve las declaradas, y evalúa si algo cambió.
6. Si se ejecuta: por cada comando interpola plantillas y `cmd`, resuelve el workdir,
   ejecuta, verifica los `probe` y extrae las nuevas variables runtime.
7. Al terminar el step con éxito —y sólo si ejecutó—, añade un registro nuevo por ámbito
   con el acumulado partido, y lo indexa. Termine como termine, restaura las plantillas y retira
   `step_workdir` del acumulado.

---

## Resumen del flujo, tal como queda decidido

Los pasos 1–4 y 6 no cambian. Los que sí, y en qué se convierten:

| Hoy | Decidido |
|---|---|
| 3. Computa la huella del código, que entra en la clave de todos los steps | Computa la huella del código; **entra solo en los steps que la declaran** (P15) |
| 5. Por cada step: carga variables del almacén (compartidas y del ambiente) y evalúa si algo cambió | Por cada step: lee su `config.yaml` (P13, P14), calcula su huella (P15), lee el **último registro** de su clave de posición (P16) y evalúa **las reglas que declara** |
| 7. Persiste el acumulado partido en dos ámbitos, sobrescribiendo | **Añade un registro nuevo** bajo `(subject, scope, step_id)`, en el ámbito que el step declara, sin sobrescribir ninguno (P16) |
| — | Si la invocación pide un rollback, el paso 5 lee el registro **anclado** en vez del último (P17) |
