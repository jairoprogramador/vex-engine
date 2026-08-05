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
│   │   └── commands.yaml
│   ├── 02-supply/
│   │   ├── commands.yaml
│   │   └── terraform/...        # archivos auxiliares del step
│   └── NN-<nombre>/
│       └── commands.yaml
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
- `value` **no puede ser `shared`**: es el nombre del ámbito del almacén compartido y ocupa
  esa misma posición en la ruta, así que un ambiente así declarado lo pisaría. Declararlo
  hace fallar la ejecución (spec 04 §5.4).
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
- El orden de ejecución es el orden **numérico** del prefijo, ordenado explícitamente por el
  dominio y no heredado del orden en que el sistema de archivos lista el directorio.

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
| `show` | si es `true`, vuelca el stdout del comando al log de la ejecución. Por defecto `false` |
| `templates` | archivos a interpolar antes de ejecutar `cmd`                                         |
| `outputs[]` | `name` + `description` + `probe`. Ver abajo                                           |

### workdir

Ruta **relativa al directorio del step** dentro de la copia del pipeline (ver sección 6).
Si el step copiado vive en `.../workdirs/<pipeline>/<entorno>/steps/02-supply`, entonces
`workdir: "./terraform/shared"` ejecuta en `.../steps/02-supply/terraform/shared`.

**Si `workdir` está vacío o es `"."`, el comando se ejecuta en el directorio del proyecto**,
no en el del step. Es lo que permite que `01-test` corra `mvn clean verify` sobre el código
del proyecto sin declarar nada.

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

Por cada step, el mapa se alimenta en este orden, y **el último en escribir gana**:

```
1. variables del almacén, ámbito shared      (de ejecuciones anteriores)
2. variables del almacén, ámbito del entorno (de ejecuciones anteriores)
3. variables declaradas del step (pipeline)
4. variables runtime extraídas durante ese step
```

Consecuencia hoy: una variable declarada en `variables/prod/deploy.yaml` **pisa** un valor
que produjo el step `supply` en la misma corrida. Solo lo extraído dentro del propio step
la sobrescribe a su vez. *(Esto está decidido que cambie — ver P3 en la sección 9.)*

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
   `.gitignore`, sha256 por archivo y sha256 del conjunto.
9. **Ejecutar los steps** en orden, entrando en la cadena de step por cada uno.

### 5.2 Cadena de step — una vez por step

1. **Cargar del almacén las variables del ámbito `shared`** de ese step.
2. **Cargar del almacén las variables del ámbito del ambiente** de ese step.
3. **Cargar y resolver las variables declaradas** del step para ese ambiente.
4. **Decidir si el step se re-ejecuta** (sección 7) y, si procede, recorrer sus comandos
   entrando en la cadena de comando por cada uno.

Al terminar:

- **Si el step tuvo éxito** → se persiste el mapa acumulado, partido en dos: lo marcado
  como compartido va al ámbito `shared`, el resto al ámbito del ambiente. Quedan excluidas
  las seis variables volátiles: `project_version`, `project_revision`,
  `project_revision_full`, `tool_name`, `project_workdir` y `step_workdir`.
- **Si el step falló** → se borran las huellas de estado de ese step, para que la siguiente
  ejecución lo vuelva a correr.

El almacén guarda el mapa acumulado **completo** bajo el nombre de cada step, no solo lo
que ese step produjo. El archivo de `deploy` contiene también lo que produjo `supply`.

### 5.3 Cadena de comando — una vez por comando

1. **Interpolar los `templates`**, guardando copia del original.
2. **Interpolar el `cmd`.**
3. **Ejecutar** en el workdir resuelto y capturar la salida. Un exit code distinto de cero
   aborta.
4. **Verificar los `probe`** contra el stdout normalizado.
5. **Extraer las variables runtime** e incorporarlas al mapa acumulado.

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

## 7. Re-ejecución: cuándo un step se salta

Antes de ejecutar los comandos de un step, el motor evalúa un conjunto de comprobaciones.
**Si cualquiera detecta un cambio, el step se ejecuta.** Solo se salta si todas coinciden
en que nada cambió.

### 7.1 Las cuatro comprobaciones

| Comprobación | Material de la huella |
|---|---|
| **instrucciones** | el `commands.yaml` del step canonicalizado: por comando, su `name`, `cmd`, `workdir`, lista de `templates` y lista de outputs (`name` + `probe`). **No** entran `description` ni `show` |
| **variables** | el mapa acumulado ordenado por nombre (nombre + valor + si es compartida), menos las seis volátiles de la sección 5.2 |
| **código** | la huella del árbol del proyecto calculada en el paso 8 de la cadena de pipeline |
| **tiempo** | TTL fijo de **30 días** desde la última ejecución del step |

### 7.2 Qué comprobaciones aplica cada step

Hoy están cableadas en el motor por nombre de step:

| Step | Comprobaciones |
|---|---|
| `test` | instrucciones + variables + código + tiempo |
| `supply` | instrucciones + variables |
| `package` | instrucciones + variables + código |
| `deploy` | instrucciones + variables + código |

**Un step con cualquier otro nombre hace fallar la ejecución, nombrándolo y enumerando los
conocidos** (spec 05 §5.2). Hasta entonces el motor lo interpretaba como «nada cambió» y lo
saltaba siempre, sin avisar y con exit code 0.

Es una **medida de transición**, no el destino: el vocabulario de steps sigue siendo abierto
para cargarlos y cerrado para decidir si se ejecutan. La brecha se cierra en dos entregas:
la spec 10 borra esta tabla entera al sustituir las cuatro reglas por una comparación de
`cache_key` —y ahí un step desconocido pasa a **ejecutarse**—, y la 15 devuelve la
granularidad, ya declarada por el pipelinecode *(ver P1 en la sección 9)*.

Debajo del cableado hay además una corrección de semántica que vale para cualquier policy,
la construya quien la construya: **cero comprobaciones evalúa a «ejecutar», no a «nada
cambió»**. Un conjunto vacío de evidencia no concluye que el step esté al día.

### 7.3 Cómo se guarda el estado

Cada comprobación guarda su huella con una clave distinta:

| Comprobación | Clave |
|---|---|
| instrucciones | `(proyecto, pipeline, step)` |
| código | `(proyecto, pipeline, step)` |
| variables | `(proyecto, pipeline, ambiente, step)` |
| tiempo | `(proyecto, ambiente, step)` |

Dos consecuencias de que las claves no sean uniformes:

- **El aislamiento entre ambientes depende de una sola comprobación.** Como instrucciones y
  código no llevan ambiente, correr `supply` en `sand` deja escrito «sin cambios» para
  `prod`. Que `prod` sí se ejecute lo garantiza únicamente que sus variables declaradas
  difieran. *(Ver P4 en la sección 9.)*
- La comprobación de tiempo no lleva pipeline: dos pipelines sobre el mismo proyecto
  comparten su TTL.

La huella nueva **se escribe durante la evaluación**, antes de que el step corra. Si el step
falla después, el borrado de estado de la sección 5.2 lo compensa.

### 7.4 Granularidad

La decisión es **todo o nada por step**: o corren todos sus comandos, o no corre ninguno.
No existe granularidad menor.

---

## 8. Ámbito `shared`

`shared` no es un ambiente: es una marca sobre las variables que un comando produce. Las
marcadas como compartidas se guardan en un ámbito propio y quedan visibles **desde todos
los ambientes**, en vez de asociarse solo a aquel en el que se ejecutó el comando.

Es lo que permite que un recurso común a todos los ambientes —un registro de contenedores,
por ejemplo— se cree una sola vez y que los tres ambientes lean su nombre.

**Cómo se marca hoy:** el motor mira el **primer segmento** de la ruta de `workdir`. Si es
exactamente `shared`, las variables de ese comando se marcan como compartidas.

```yaml
workdir: "shared/terraform"     # ✅ primer segmento = "shared"  → compartida
workdir: "./terraform/shared"   # ❌ primer segmento = "."       → NO compartida
```

> **Estado real.** Los tres pipelines de `Vex/pipelines` escriben `./terraform/shared`, así
> que hoy **el mecanismo no se activa en ninguno** y las variables del bloque compartido se
> guardan duplicadas, una copia por ambiente. Que eso no cause un recurso duplicado se debe
> a que el backend de terraform del bloque compartido omite el ambiente en su clave de
> estado — es una convención del pipelinecode, no una garantía del motor. La marca va a
> cambiar a un campo explícito *(ver P2 en la sección 9)*.

Hoy `shared` no es más que eso: una marca sobre variables y un ámbito de almacén. No tiene
ejecución propia ni decisión de re-ejecución propia — sus comandos son comandos del step
como cualquier otro. Que pase a ser una fase con ciclo propio es lo que describe P5.

---

## 9. Diseño pendiente

Decisiones tomadas que el motor todavía no implementa, y defectos conocidos. Nada de esto
está en el cuerpo del documento porque hoy no ocurre.

Las entradas conservan su número aunque el orden temático no coincida: son anclas estables
a las que los planes ya referencian.

| Tema | Entradas |
|---|---|
| Steps y re-ejecución | P1, P4, P10 |
| Variables | P3, P7, P8 |
| Ámbito `shared` | P2, P5 |
| Identidad y procedencia | P9, P6 |
| Registro de despliegue | P11, P12 |

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

**El primer tiempo está implementado** (spec 05): `PolicyBuilder.Build` devuelve error ante
un step que no conoce, y `Policy.Evaluate` con cero reglas manda ejecutar en vez de saltar.

El segundo tiempo llega repartido, y conviene no confundir las dos mitades. El **vocabulario
se abre en la spec 10**, por eliminación: al desaparecer `PolicyBuilder` no queda ningún
nombre que reconocer, y un step desconocido simplemente no tiene entrada de caché, luego se
ejecuta. La **declaración** —`checks` por step, que es lo que devuelve la granularidad que
la 10 sacrifica— es la **spec 15**. La medida de transición de la 05 vive solo entre la 05 y
la 10.

**P2 — La marca `shared` pasa a ser explícita.**
Un campo `scope: shared` en el comando, junto al `workdir`:

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

**P3 — Sobre un valor literal, la variable runtime siempre gana.**
Un `value` literal en `variables/<ambiente>/<step>.yaml` es un valor **por defecto**: en
cuanto un comando produce una variable con ese nombre, el valor runtime manda para el resto
de la ejecución. Requiere invertir el orden actual de la sección 3.4, moviendo las
declaradas al principio:

```
1. variables declaradas del step (pipeline)   ← el default, primero
2. variables del almacén, ámbito shared
3. variables del almacén, ámbito del entorno
4. variables runtime extraídas durante ese step
```

**Alcance, y su frontera con P7.** P3 es una regla de **precedencia**, y solo tiene sentido
donde puede haber choque: dos fuentes que aportan un nombre y hay que decidir cuál vale. Eso
ocurre con los valores literales, y ahí P3 rige. **No** ocurre con las variables de origen
declarado (P7): esas nombran su fuente, así que tienen un único proveedor por construcción y
no hay nada que resolver.

Son dos mecanismos con dos reglas, y ambos quedan vigentes. Conviene no leer P7 como una
derogación de P3: P7 no cambia quién gana, **quita la pregunta** en los casos que cubre.

**P4 — El ambiente es parte de la identidad de re-ejecución.**
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

**P5 — El bloque `shared` es un step previo, con su propio ciclo.**
Consecuencia de P2 y P4, y es más que partir una decisión en dos: `shared` deja de ser una
marca sobre variables (sección 8) y pasa a ser **una fase que corre antes del step real, con
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
es todo o nada por step (sección 7.4)—. Toca la clave de re-ejecución, la construcción de
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
el archivo de `deploy` contiene lo que produjo `test`. Si el pipeline forma parte de la
clave está sin decidir (sección 9.3).

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

**P10 — El estado de re-ejecución pasa a estar direccionado por contenido.**
Las cuatro huellas sueltas con cuatro claves distintas (sección 7.3) se unifican en **una
entrada bajo una clave derivada del contenido**: instrucciones + variables + código +
ambiente (P4) + ámbito (P5). Tres cambios de comportamiento:

- **Volver atrás acierta.** Hoy, alternando entre dos estados A y B, *cada* cambio
  re-ejecuta, porque lo guardado es siempre lo último escrito. Con la clave derivada del
  contenido, volver a A encuentra la entrada de A.
- **La entrada se escribe después del éxito, no antes.** Hoy la huella se escribe durante la
  evaluación y un borrado compensatorio la revierte si el step falla (sección 7.3). Si el
  proceso muere duro —Ctrl-C, timeout, OOM— ese borrado nunca corre y **la siguiente
  ejecución salta un step que jamás terminó**. Escribiendo después, el compensador
  desaparece y la ventana con él.
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

**P12 — El destino del estado es configuración explícita, no un modo.**
Hoy una bandera `--mode local|remote` decide dónde se escribe, y `remote` asume una
plataforma concreta. Eso ata el motor a un proveedor y contradice el resto del diseño, que
es agnóstico de dónde corre.

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

**La transición rompe a propósito.** Se retiran `--mode` y las siete banderas de endpoints
de una vez, y la versión del contrato de invocación sube: el motor rechaza clientes viejos
**por contrato, no por memoria de nadie**. `type: http` queda congelado hasta que exista un
cliente real esperándolo; hasta entonces el modo remoto corre sin caché, que es degradación
de rendimiento, no de correctitud.

### 9.2 Defectos conocidos

| # | Qué pasa |
|---|---|
| D1 | ~~Si un step falla, **las plantillas interpoladas no se restauran**: quedan con los valores sustituidos dentro de la copia de trabajo~~ **Corregido (spec 06 §5.1): la limpieza del Template Method está en `defer`, así que corre falle o no la ejecución. El error de `exec` sigue siendo el que manda —el de la restauración se acompaña con `errors.Join` en vez de descartarse— y `step_workdir` deja de quedar huérfano en el mapa acumulado tras un step fallido. No cubre la muerte dura del proceso (`Ctrl-C`, D10): eso es la opción D de la spec, diferida** |
| D2 | ~~Un step sin `commands.yaml` —o con el archivo vacío— se registra como éxito. «Step vacío» y «step ejecutado» son indistinguibles~~ **Corregido (spec 04 §5.3, D-A12): es `skipped{reason: no_commands}`, y además deja de persistir estado de re-ejecución** |
| D3 | ~~Un directorio de step con prefijo de un solo dígito (`2-supply`) hace que el motor busque `02-supply/commands.yaml`, no lo encuentre, y el step pase sin ejecutar nada (efecto de D2)~~ **Corregido (spec 04 §5.1): el prefijo se valida a exactamente dos dígitos en el validador de estructura y en `NewStepName`, así que ese directorio ya no puede existir sin que la ejecución falle nombrándolo** |
| D4 | Las escrituras del almacén y del estado no son atómicas. Una interrupción a mitad deja el archivo truncado, y el motor lee un archivo truncado como «no hay nada»: el valor se pierde en silencio |
| D5 | Si una comprobación no consigue guardar su huella, el step se marca para ejecutar y se ejecuta, pero la huella nunca se escribe — y la corrida siguiente vuelve a encontrar la vieja. Se re-ejecuta indefinidamente, sin señal |
| D6 | La variable `shared_workdir` está declarada en el motor y no se asigna nunca |
| D7 | ~~**El orden de ejecución de los steps es lexicográfico, no numérico:** `os.ReadDir` ordena por nombre y `StepName.Order()` no se usa jamás para ordenar. Con prefijos de **dos** dígitos ambos órdenes coinciden y no se manifiesta —doce steps `01…12` salen en orden, verificado con test—. Se manifiesta en cuanto entra un prefijo de **un** dígito (D2/D3): `1-test, 10-promote, …, 2-supply`, y basta un directorio mal nombrado entre otros correctos~~ **Corregido (spec 04 §5.2): `NewStepNames` ordena por `Order()`. Con el prefijo validado a dos dígitos no había un orden roto que arreglar; lo que se gana es que el invariante esté enunciado en una línea en vez de deducido de que `os.ReadDir` ordena por nombre y de que `%02d` es de ancho fijo** |
| D8 | Un campo vacío en los datos del proyecto produce una variable bajo la clave `""`: el error de construcción se ignora y la variable vacía se inserta igual. Esa entrada **entra en la huella de variables**, así que no es un log feo — es material de identidad contaminado. **Corregido al implementar (spec 03 §9.1): el mecanismo era real, el disparador no.** Ningún campo vacío del proyecto llega al handler —`create_execution.go:74-100` los valida antes—, así que la entrada anónima solo la producía un defecto del propio motor. El daño cotidiano era el inverso: un `variables/<env>/<step>.yaml` con `value: ""` **abortaba la ejecución entera** |
| D9 | ~~Un ambiente llamado `shared` en `environments.yaml` pisa el almacén compartido. El ambiente se valida contra la lista, pero no contra nombres reservados~~ **Corregido (spec 04 §5.4): el handler 03 rechaza el `environments.yaml` que declare `value: "shared"`. La spec 15 extiende la reserva al vocabulario de ámbitos cuando `shared` pase a ser una fase** |
| D10 | Un `Ctrl-C` mata la ejecución sin dejar rastro de cancelación: no hay manejador de señales y el mecanismo de cancelación que el motor declara no se invoca nunca. «Cancelado» e «interrumpido» son indistinguibles |
| D11 | Si el identificador de ejecución tiene menos de cuatro caracteres, la línea de log que lo abrevia provoca un panic |

### 9.3 Decisiones abiertas

No están decididas, y por eso no aparecen arriba. Se listan para no darlas por supuestas al
leer las entradas que dependen de ellas. El detalle y el estado vivo están en
`vex-plan-revision.md` §6.

| Pregunta | Afecta a |
|---|---|
| ¿La clave del estado incluye el pipeline? Conceptualmente el recurso compartido pertenece al proyecto, no al pipeline — pero quitarlo hace que dos pipelines del mismo proyecto pasen a compartir estado | P8 |
| Forma concreta y versión de esquema de `variables/<ambiente>/<step>.yaml`: qué vocabulario admite `resolve`, y cómo se negocia la versión | P7 |
| ¿Son legítimos los valores de variable vacíos? Hoy se prohíben, lo que hace inexpresable un parámetro opcional | P7 |
| Un hash de un parámetro de baja entropía es reversible por fuerza bruta (`REPLICAS=3`), lo que contradice la promesa de «solo hashes, nunca valores». ¿Sal por proyecto? Cambiarlo después invalida las comparaciones previas | P11 |
| Reglas del enmascarado de valores en los extractos: longitud mínima y orden de sustitución. Sin ellas, una variable con valor `"1"` o `"prod"` destroza cualquier extracto | P11 |
| Duración de la ventana del clon viejo, y si es configurable por proyecto | P6 |

---

## Resumen del flujo end-to-end

1. El motor recibe un **ambiente** y un **step**.
2. Clona el proyecto y el pipeline, y copia el pipeline a un directorio de trabajo propio de
   la terna (proyecto, pipeline, ambiente).
3. Calcula la versión del proyecto, inyecta las variables iniciales y computa la huella del
   código.
4. Recorre los steps desde el primero hasta el pedido.
5. Por cada step: carga las variables del almacén (compartidas y del ambiente), carga y
   resuelve las declaradas, y evalúa si algo cambió.
6. Si se ejecuta: por cada comando interpola plantillas y `cmd`, resuelve el workdir,
   ejecuta, verifica los `probe` y extrae las nuevas variables runtime.
7. Al terminar el step con éxito, persiste el acumulado partido en ámbito compartido y
   ámbito del ambiente. Termine como termine, restaura las plantillas y retira
   `step_workdir` del acumulado.
