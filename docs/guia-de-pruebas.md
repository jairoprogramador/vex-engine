# Guía de pruebas de `vexd`

Cómo probar el motor a mano: compilarlo, armar un entorno, pedirle operaciones y leer lo que responde. Es la
guía práctica de `docs/modelo/lenguaje-publicado.md`, que es el contrato.

- [Arranque rápido](#arranque-rápido)
- [La invocación](#la-invocación)
- [Cómo se escribe una petición](#cómo-se-escribe-una-petición)
- [Las operaciones](#las-operaciones)
- [Armar tu propio pipeline de prueba](#armar-tu-propio-pipeline-de-prueba)
- [Recetas](#recetas)
- [Si algo falla](#si-algo-falla)

## Arranque rápido

```bash
scripts/demo.sh --limpio          # compila, arma /tmp/demo-vex y hace un primer intento
source /tmp/demo-vex/entorno.sh   # deja VEXD, VEX_ALMACEN, VEX_ESPACIO y VEX_MATERIAL en tu shell
vexd_demo despliegues '{"Version":"1","Ambiente":"sand"}'
```

`scripts/demo.sh` hace, en este orden:

1. **Compila** `./vexd` (está en `.gitignore`). Con `--sin-compilar` usa el que ya existe.
2. **Crea** `almacen/`, `espacio/` y `material/` en `$DEMO_DIR` (por defecto `/tmp/demo-vex`) y dos repos git:
   `proyecto/` (un README) y `pipeline/` (copia de `internal/ejecucion/testdata/ejemplo`). Si ya existen, los
   deja como están, así puedes editar el pipeline entre pruebas. Con `--limpio` empieza de cero.
3. **Intenta** en el ambiente `prod` (cambiable con `AMBIENTE=…`) y muestra la respuesta y el código de salida.
4. **Escribe** `$DEMO_DIR/entorno.sh` para que sigas con otras operaciones.

Con `source entorno.sh` tienes `VEX_ALMACEN`, `VEX_ESPACIO` y `VEX_MATERIAL` exportadas, que es de donde `vexd`
lee su configuración. `$VEXD` es la ruta al ejecutable, y `vexd_demo <operación> [params]` arma la petición del
protocolo (JSON-RPC, una línea), se la envía y muestra la respuesta. Con `jq` instalado, `vexd_demo … | jq .` la
muestra con sangría.

## La invocación

`vexd` no tiene subcomandos ni opciones. Habla **JSON-RPC 2.0 por la entrada y la salida estándar, un mensaje JSON
por línea**: lee una petición, escribe una respuesta y termina. No hay servidor (docs/rediseno/RD-13-protocolo.md).

```bash
echo '{"jsonrpc":"2.0","id":"1","method":"despliegues","params":{"Version":"1","Ambiente":"sand"}}' | $VEXD
```

### La petición

| Miembro | Qué es |
|---|---|
| `jsonrpc` | Siempre `"2.0"` |
| `id` | Una cadena o un número. La respuesta lo devuelve igual |
| `method` | La operación: `intentar`, `logs`, `describir`… (ver más abajo) |
| `params` | Los campos de la operación, con su `Version`. Es lo que las tablas de abajo llaman «petición» |
| `entorno` | Opcional. Variables de entorno (nombre → valor, todo cadenas) para los comandos. Solo `intentar` y `rollback` |

Una línea completa, sin saltos dentro. Pasa de 1 MiB y se rechaza. Una petición sin salto final también vale.

### Variables de entorno de `vexd`

| Variable | Qué es | Obligatoria |
|---|---|---|
| `VEX_ALMACEN` | El **historial** (en un sistema de ficheros local: no NFS ni SMB, ver `RD-13`, «El almacén compartido»): los registros de intentos, despliegues, lanzamientos y reservas. Es la memoria del motor. Si lo borras, olvida todo. El directorio **tiene que existir**: el motor no lo crea, para no escribir en el vacío si un volumen no se montó | Siempre, salvo `describir` |
| `VEX_ESPACIO` | El **espacio de trabajo**: donde se ejecutan los comandos de los pasos. Una subcarpeta por ambiente (`<espacio>/<ambiente>/motor/<paso>/`). Se rehace en cada intento | Solo `intentar` y `rollback` |
| `VEX_MATERIAL` | Donde el motor copia el código de las fuentes que lee de git. Es una copia desechable: borrarla no cambia ninguna decisión | No. Por defecto, `<temporal del sistema>/vex/material` |

No hay opciones de línea de comandos para esto: solo las variables. En un contenedor las fija la imagen.

### La respuesta

**stdout** es solo protocolo, una línea por mensaje. La **respuesta es la última línea**. Antes, en `intentar` y
`rollback`, van las notificaciones de progreso (ver «El avance»). Las demás secciones de esta guía llaman
«respuesta» a lo que va en `result`.

```json
{"jsonrpc":"2.0","id":"1","result":{"Intento":"…","Estado":"exitoso"}}
{"jsonrpc":"2.0","id":"1","error":{"code":-32004,"message":"…","data":{"tipo":"ambiente_ocupado","intento":"…"}}}
```

- **Un intento que termina `fallido` o `cancelado` es un `result` correcto** cuyo `Estado` lo dice. Solo es
  `error` lo que impide atender la petición.
- Las operaciones que no devuelven nada responden `{}`. Una lista sin elementos es `[]`.
- Lo que imprimen los comandos de los pasos no se muestra: se consulta con `logs`.
- **stderr** solo lleva la causa de un error interno (`-32000`), que no se cuenta a quien invoca.

### El avance de un intento

Mientras un intento corre, `vexd` cuenta cómo avanza con notificaciones, antes de la respuesta:

```
{"jsonrpc":"2.0","method":"progreso","params":{"evento":"intento_iniciado","intento":"01a1…"}}
{"jsonrpc":"2.0","method":"progreso","params":{"evento":"paso_iniciado","intento":"01a1…","paso":"test"}}
{"jsonrpc":"2.0","method":"progreso","params":{"evento":"comando_terminado","intento":"01a1…","paso":"test","comando":"comando-test-01","estado":"exitoso"}}
{"jsonrpc":"2.0","method":"progreso","params":{"evento":"paso_terminado","intento":"01a1…","paso":"test","estado":"ejecutado"}}
{"jsonrpc":"2.0","id":"1","result":{"Intento":"01a1…","Estado":"exitoso", …}}
```

`estado` de un paso: `ejecutado` o `precargado` (no se reejecutó: no hay `paso_iniciado`), o `fallido` o `cancelado`.
Nunca llevan lo que imprimió un comando: cuando llega `comando_terminado`, su salida ya está en el historial y se
lee con `logs`. Si quien invoca no lee la salida, el motor no se detiene: descarta eventos y lo dice en stderr. Para
ver solo la respuesta: `… | tail -n 1`.

### Variables de entorno para los comandos

`intentar` y `rollback` aceptan `entorno`, **fuera de `params`**, y el motor se lo añade al entorno de cada comando
del intento (las pedidas pisan las que `vexd` ya tenía):

```bash
echo '{"jsonrpc":"2.0","id":"1","method":"intentar","params":{ … },"entorno":{"REGISTRY_URL":"registry.local"}}' | $VEXD
```

- Los nombres son de variable de entorno: letras, números y `_`, sin empezar por un número. Si no, `-32602`, que dice
  el nombre y nunca el valor. Un valor no puede tener un carácter NUL.
- **No son variables del pipeline**: `${var.REGISTRY_URL}` no las ve; el comando las lee como `$REGISTRY_URL`.
- **El motor no las guarda ni gestiona secretos** (es otro producto): no entran en el historial, en un hash, en el
  avance ni en los errores. Si un comando las imprime, queda en su salida, que se lee con `logs`. Que una variable
  sea sensible es cosa de quien la pasa.
- Valen para todos los comandos del intento. Las demás operaciones no ejecutan comandos y rechazan `entorno`.

### Los errores

| `code` | `data.tipo` | Cuándo |
|---|---|---|
| `-32700` | `json_invalido` | La línea no es JSON |
| `-32600` | `peticion_invalida` | No es una petición JSON-RPC (falta `id` o `method`), no llegó ninguna, o la línea es demasiado larga |
| `-32601` | `operacion_desconocida` | El `method` no existe |
| `-32602` | `parametros_invalidos` | Un campo que la operación no tiene, un valor que no vale, `entorno` con valor |
| `-32001` | `version_no_soportada` | `Version` vacía o distinta de `"1"` |
| `-32002` | `rechazado` | Lo pedido rompe una regla del motor |
| `-32003` | `no_existe` | Lo consultado no está en el historial (incluido `logs` sin ningún intento) |
| `-32004` | `ambiente_ocupado` | Hay otro intento en el ambiente; `data.intento` dice cuál |
| `-32005` | `no_disponible` | El espacio de trabajo del ambiente no se alcanzó; el intento no empezó |
| `-32006` | `configuracion_invalida` | Falta una variable `VEX_*` obligatoria, o su directorio no existe; el mensaje nombra la variable |
| `-32007` | `cancelado` | Se canceló antes de abrir el intento |
| `-32008` | `escritura_concurrente` | Otros procesos escribieron a la vez, una y otra vez, y no se pudo escribir. No se escribió nada: volver a pedirlo es seguro |
| `-32000` | `interno` | Cualquier otro; la causa va a stderr |

Los códigos y los `tipo` son estables.

### Códigos de salida

| Código | Significa |
|---|---|
| `0` | Bien. Si era un intento o un rollback, terminó `exitoso` |
| `1` | La operación falló (por ejemplo, ambiente ocupado o una fuente que no existe), o el intento terminó `fallido` |
| `2` | La petición o la configuración son inválidas: JSON mal formado, operación desconocida, campo desconocido, versión no soportada, falta `VEX_ALMACEN`, almacén inexistente |
| `130` | Cancelado: `SIGTERM`, Ctrl-C o la notificación `cancelar`, o el intento terminó `cancelado` |

Un intento `fallido` **sí imprime su respuesta** en stdout, además de salir con `1`.

### Cancelar

Con `SIGTERM` o Ctrl-C, o enviando, en la misma entrada, la notificación `{"jsonrpc":"2.0","method":"cancelar"}`.
**Cerrar la entrada no cancela**: un pipe la cierra nada más escribir, y solo quiere decir «no envío más».

Cancelar le pide a cada comando que termine (`SIGTERM`) y acaba con él, y con todo lo que lanzó, a los 5 s si no lo
hace. Pulsar Ctrl-C más veces no acelera nada: `vexd` no muere de golpe, para no dejar procesos huérfanos ni el
ambiente ocupado.

## Cómo se escribe una petición

Los campos de cada operación van en `params`. Reglas:

- **Los nombres de campo son los del lenguaje publicado**, en español y `PascalCase`: `FuenteDelProyecto`,
  no `fuente_del_proyecto`. No distingue mayúsculas (`ambiente` vale), pero escríbelos como en esta guía.
- **`Version` va siempre** y hoy es `"1"`. Vacía o distinta da `-32001` (salida 2) antes de tocar nada.
- **Un campo que la operación no tiene se rechaza** (`-32602`, salida 2). Es a propósito: un error de escritura no debe
  hacer que el motor haga otra cosa de la que pediste.
- **Los campos opcionales se omiten** o se dejan vacíos.
- **Las fuentes son directorios locales de repos git.** Los remotos vendrán con RD-11.
- **Los identificadores** (`Intento`, `Despliegue`) los da el motor en sus respuestas: cópialos de ahí.

## Las operaciones

Doce operaciones. En cada una: los `params`, el `result` y un ejemplo con `vexd_demo` (después de `source entorno.sh`), con `$D` = `$DEMO_DIR`.

### `intentar` — ejecutar un pipeline hasta un paso en un ambiente

Necesita `VEX_ALMACEN` y `VEX_ESPACIO`.

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Ambiente` | sí | El ambiente donde se intenta: el **`value`** de `environments.yaml` (`prod`), no su `name` |
| `Solicitante` | sí | Quién lo pide. Queda en el historial |
| `FuenteDelProyecto` | sí | Directorio del repo git con el código del proyecto |
| `FuenteDelPipeline` | sí | Directorio del repo git con el pipeline (pasos, comandos, variables) |
| `CommitDelProyecto` | no | Commit del proyecto (identificador completo). Vacío: la cabeza del repo |
| `CommitDelPipeline` | no | Ídem para el pipeline |
| `CopiaDeTrabajo` | no | Directorio con cambios sin commit. Tiene **prioridad** sobre la fuente y el commit del proyecto, y un intento así **nunca llega a despliegue** |
| `HastaPaso` | no | Detenerse después de este paso (`"preparar"`, sin el `NN-`). Vacío: todos los pasos |
| `Metadatos` | no\* | Datos que solo sabe quien invoca. Alimentan las variables estándar |
| `Metadatos.ProjectName` | | → `${var.project_name}` |
| `Metadatos.ProjectId` | | → `${var.project_id}` |
| `Metadatos.ProjectOrganization` | | → `${var.project_organization}` |
| `Metadatos.ProjectTeam` | | → `${var.project_team}` |

\* Solo hacen falta los que el pipeline use.

`DirectorioDeEspacioDeTrabajo` y `DirectorioDelAlmacen` existen en el tipo pero **nadie los lee**: esos
directorios se dan con las variables `VEX_*`.

**Respuesta**

| Campo | Significado |
|---|---|
| `Intento` | Identidad del intento |
| `Estado` | `exitoso`, `fallido` o `cancelado` |
| `Despliegue` | El despliegue al que llegó. Vacío si no llegó a ninguno (falló, se canceló, usó `CopiaDeTrabajo`) |

```bash
vexd_demo intentar "{ \"Version\": \"1\", \"Ambiente\": \"prod\", \"Solicitante\": \"ana\",
  \"FuenteDelProyecto\": \"$D/proyecto\", \"FuenteDelPipeline\": \"$D/pipeline\",
  \"Metadatos\": { \"ProjectName\": \"vex-demo\", \"ProjectId\": \"p1\" } }"
```

**Solo hace el trabajo que hace falta.** Repetir un intento sin cambios no ejecuta ningún comando; si cambia
lo que un paso declara, solo se re-ejecuta ese paso (con todos sus comandos). Qué mira cada paso lo decide su
`rules` en `config.yaml`.

**Un ambiente, un intento a la vez.** Si hay otro intento sin desenlace en el mismo ambiente, falla con `-32004`
(salida 1) y dice cuál es en `data.intento` (véase `abandonar`).

### `rollback` — volver a un despliegue anterior

Necesita `VEX_ALMACEN` y `VEX_ESPACIO`. El ambiente y las fuentes las toma del propio despliegue destino: no se
repiten.

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Despliegue` | sí | El despliegue al que volver |
| `Solicitante` | sí | Quién lo pide |
| `Metadatos` | no | Igual que en `intentar` |

Respuesta: la misma que `intentar`. Un rollback es **un intento nuevo** y genera un despliegue nuevo cuyo padre
es el destino; no reescribe nada del historial.

```bash
vexd_demo rollback '{ "Version": "1", "Despliegue": "<id>", "Solicitante": "ana", "Metadatos": { "ProjectName": "vex-demo" } }'
```

### `simular` — recorrer el pipeline sin efectos

No ejecuta comandos ni escribe en el historial. Sirve para saber si el pipeline funciona antes de publicarlo.

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Ambiente` | sí | Valor del ambiente (`sand`), no su `name` (`sandbox`) |
| `Solicitante` | sí | Quién pide la simulación |
| `HastaPaso` | sí | Último paso que se simula |
| `Fuente` | \* | Directorio del repo git del pipeline |
| `Commit` | \* | Commit del pipeline. **Identificador completo de 40 caracteres**: aquí no vale vacío |
| `CopiaDeTrabajo` | \* | Directorio con el pipeline sin commit. Tiene prioridad |
| `Metadatos` | no | Igual que en `intentar` (`ProjectId`, `ProjectName`, …): dan valor a las variables estándar `project_*` |

\* O `Fuente` con `Commit`, o `CopiaDeTrabajo`.

**Respuesta**: el resumen de un intento sin `Id` (`Ambiente`, `Solicitante`, `HastaPaso`, `Estado`). Si `Estado` es
`fallido` (y el código de salida es 1), trae `Causa`: `Fallos` de la comprobación, o `Faltante` con el `Paso` y las
`Variables` que no se pudieron interpolar. Solo nombres, nunca valores. Un ambiente o un paso que el pipeline no
tiene es una petición inválida (error, no resultado). La simulación declara las
variables estándar como un intento: `project_*` salen de `Metadatos` (vacías si no se dan, igual que en un
intento), `environment` y `step_name` son las reales, y `project_hash`, `project_version`, `project_workdir`,
`tool_name` y `step_workdir` llevan un valor simulado (no hay material ni espacio de trabajo que leer, y la
simulación nunca devuelve valores).

```bash
SHA=$(git -C $D/pipeline rev-parse HEAD)
vexd_demo simular "{\"Version\":\"1\",\"Ambiente\":\"sand\",\"Solicitante\":\"jailux\",\"HastaPaso\":\"test\",\"Fuente\":\"$D/pipeline\",\"Commit\":\"$SHA\"}"
vexd_demo simular "{\"Version\":\"1\",\"Ambiente\":\"sand\",\"Solicitante\":\"jailux\",\"HastaPaso\":\"test\",\"CopiaDeTrabajo\":\"$D/pipeline\"}"
```

### `lanzar` — hacer visible un despliegue

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Ambiente` | sí | Ambiente del despliegue |
| `Despliegue` | sí | Despliegue a lanzar |
| `Nombre` | no | Nombre del lanzamiento. Vacío: toma el número de versión |

Respuesta: `Ambiente`, `Despliegue`, `Version` (número, sube de uno en uno por ambiente), `Nombre` e `Instante`.
Es **incondicional**: la reserva solo bloquea el lanzamiento automático, nunca este.

```bash
vexd_demo lanzar '{"Version":"1","Ambiente":"prod","Despliegue":"<id>","Nombre":"v1"}'
```

### `reservar` y `liberar` — quién decide los lanzamientos de un ambiente

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Ambiente` | sí | El ambiente |

Respuesta: `{}`. **Reservar**: desde ahora decide el dueño del negocio, y no se lanza solo. **Liberar**: se vuelve
a lanzar en cuanto un despliegue queda listo.

### `diagnosticar` — por qué falló un intento

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Ambiente` | sí | El ambiente |
| `Intento` | \* | El intento que falló |
| `Lanzamiento` | \* | Un lanzamiento (en vez de un intento) |
| `Referencia` | no | Despliegue con el que comparar, a mano. Vacío: lo elige el motor |

\* Uno de los dos, nunca los dos.

**Respuesta**: `Forma` es `con_atribucion`, `sin_referencia` o `no_se_atribuye`. Con `sin_referencia`, `Mensaje`
dice que no hay historial previo al intento que se diagnostica. Con atribución, `Atribucion` lista
los ejes que cambiaron (`codigo`, `instrucciones`, `variables`) y `Sustento` el detalle: en qué pasos, y con qué
despliegue se comparó (`Comparaciones`, con la `Razon`: `mismo_ambiente` o
`elegida_por_el_usuario`). Nunca lleva valores de variables, autores ni commits.

### `abandonar` — dar por perdido un intento

Para un intento que quedó **sin desenlace** (el proceso murió, se cortó la red…) y sigue ocupando el ambiente.

| Campo | Obligatorio | Significado |
|---|---|---|
| `Version` | sí | `"1"` |
| `Intento` | sí | El intento a abandonar |

Respuesta: `{}`. El ambiente queda libre.

### `intento`, `intentos`, `despliegues` — consultar el historial

Solo lectura, y **nunca devuelven el valor de una variable**.

| Operación | Campos | Devuelve |
|---|---|---|
| `intento` | `Version`, `Intento` | Un intento: `Id`, `Apertura` (ambiente, solicitante, pasos, `HastaPaso`, `ConCommits`, `HashDelCodigo`), `Registros`, `Estado`, `Destino` (si fue rollback), `Abandonado` |
| `intentos` | `Version`, `Ambiente` | La lista de intentos del ambiente |
| `despliegues` | `Version`, `Ambiente` | La lista de despliegues: `Id`, `Ambiente`, `Intento`, `Padre` (vacío en el primero), `Instante` |

El contenido de los registros (`Contenido.Datos`) viaja opaco, en base64. Un ambiente sin intentos o sin despliegues
responde `[]`.

### `logs` — la salida de los comandos de un intento

Solo lectura. Lo que imprimió cada comando se guarda en el historial al terminar el comando, también si falló.

| Campo | Obligatorio | Qué es |
|---|---|---|
| `Version` | sí | `"1"` |
| `Intento` | no | El intento a consultar. Sin él, el último que se abrió en cualquier ambiente |
| `Resultado` | no | `"exitoso"` o `"fallido"`: solo los comandos que salieron así. Sin él, todos |

La respuesta trae `Intento` (cuál es, el último si no se pidió uno), `Ambiente` y `Salidas`: una lista plana, en el
orden en que corrieron los comandos, con `Paso`, `Comando`, `Exitoso`, `Texto` (lo que escribió, salida y error
juntos, **tal cual**, con su salto de línea final) e `Instante`. Agrupar por paso o recortar es cosa de quien
presente la respuesta. Un intento que llegó hasta `test` solo trae `test`; sin salidas que mostrar, `Salidas` es `[]`.

```json
{
  "Intento": "01a106d1-94e1-727c-a252-4efac5ab306c",
  "Ambiente": "prod",
  "Salidas": [
    {"Paso": "test", "Comando": "comando-test-01", "Exitoso": true, "Texto": "hola vex-demo\n", "Instante": "2026-10-05T22:12:42.9Z"},
    {"Paso": "test", "Comando": "comando-test-02", "Exitoso": true, "Texto": "etiqueta=v1.0.0\n", "Instante": "2026-10-05T22:12:42.9Z"}
  ]
}
```

Un historial **sin ningún intento** no tiene logs que dar: es `-32003` (`no_existe`), no un mensaje.

### `describir` — qué es este motor

Sin parámetros, y es la única que **no necesita `VEX_ALMACEN`**. Sirve para saber, antes de pedir nada, si quien
invoca y este motor se entienden: la imagen del contenedor y la CLI se versionan por separado.

Respuesta: `VersionDelMotor`, `VersionesDelLenguaje` (las que acepta `Version`) y `Operaciones` (ordenadas).

```bash
echo '{"jsonrpc":"2.0","id":"1","method":"describir"}' | $VEXD
```

## Armar tu propio pipeline de prueba

Un pipeline es un repo git con esta forma. El de ejemplo, `internal/ejecucion/testdata/ejemplo`, es el modelo:

```
pipeline/
├── config.yaml            # schema_version + configuración de cada paso
├── environments.yaml      # los ambientes
├── steps/
│   ├── 01-preparar/
│   │   ├── commands.yaml  # los comandos del paso
│   │   └── plantilla.txt  # cualquier fichero que el paso use
│   └── 02-desplegar/commands.yaml
└── variables/             # opcional: variables declaradas
    └── prod/preparar.yaml # <value del ambiente>/<paso sin NN->.yaml
```

El nombre de un paso es `NN-<nombre>` con **dos dígitos exactos** y sin repetir el orden. El motor lo llama
`preparar`, sin el prefijo.

### `environments.yaml`

```yaml
- name: production        # nombre para personas
  description: producción
  value: prod             # lo que se escribe en "Ambiente"
```

### `config.yaml`

```yaml
schema_version: 1
steps:
  desplegar:
    rules: [instructions]  # qué mira el paso para decidir si se re-ejecuta
    max_age: 1h            # pasado este tiempo, se re-ejecuta aunque nada cambie
    scope: environment     # environment (por defecto) o shared
```

| Clave | Valores | Significado |
|---|---|---|
| `rules` | `code`, `instructions`, `variables` | Qué cambio provoca que el paso se re-ejecute. Sin `rules`, mira las tres. `rules: []` no mira ninguna |
| `max_age` | duración (`1h`, `30m`) | Antigüedad máxima de la última ejecución |
| `scope` | `environment`, `shared` | Si lo que el paso deja es de su ambiente o compartido entre ambientes |

Una clave que el formato no tiene hace ilegible el fichero.

### `steps/NN-nombre/commands.yaml`

```yaml
- name: generar-etiqueta
  description: produce la variable que consume el siguiente paso
  cmd: echo etiqueta=v1.0.0
  workdir: .               # opcional: subdirectorio donde se ejecuta
  templates:               # ficheros que se interpolan antes de ejecutar
    - plantilla.txt
  outputs:                 # variables que el comando produce
    - name: etiqueta
      description: la etiqueta que se despliega
      probe: 'etiqueta=(\S+)'   # expresión regular; el grupo 1 es el valor
      scope: environment        # opcional
```

- **`${var.<nombre>}`** se interpola en `cmd`, en las plantillas y en `variables/`. Un `$VAR` o `${VAR}` **no** es del
  motor: llega tal cual al shell.
- Las variables **estándar** son `project_name`, `project_id`, `project_organization` y `project_team` (de
  `Metadatos`), y `environment`.
- Una variable producida por un paso la ve el paso siguiente, con `${var.etiqueta}`.
- Los comandos corren con `sh -c` (`cmd /C` en Windows), en el directorio del paso dentro del espacio de trabajo.
- Un comando con código de salida distinto de 0 hace fallar el paso, y el intento termina `fallido`.

### Publicar un cambio para la prueba

El motor lee **commits**, así que después de editar hay que confirmar:

```bash
git -C $D/pipeline -c user.name=demo -c user.email=demo@vex.test commit -qam "cambio"
```

Con `CopiaDeTrabajo` puedes probar sin commit (pero ese intento no llega a despliegue).

## Recetas

Todas parten de `scripts/demo.sh --limpio` y `source /tmp/demo-vex/entorno.sh`. Define antes:

```bash
D=$DEMO_DIR
intentar() { vexd_demo intentar "{ \"Version\":\"1\",\"Ambiente\":\"prod\",\"Solicitante\":\"yo\",\"FuenteDelProyecto\":\"$D/proyecto\",\"FuenteDelPipeline\":\"$D/pipeline\",
  \"Metadatos\":{\"ProjectName\":\"vex-demo\",\"ProjectId\":\"p1\"} }"; }
confirmar() { git -C $D/pipeline -c user.name=demo -c user.email=demo@vex.test commit -qam "$1"; }
```

**Un intento sin cambios no hace nada.** Ejecuta `intentar` dos veces: la segunda no imprime ningún comando y
responde `exitoso` con otro despliegue.

**Cambiar una plantilla re-ejecuta solo su paso.**

```bash
echo 'hola de nuevo ${var.project_name}' > $D/pipeline/steps/01-preparar/plantilla.txt
confirmar plantilla
intentar     # se re-ejecuta preparar, no desplegar
```

**Hacer fallar un paso y diagnosticar.**

```bash
sed -i.bak 's/cmd: echo desplegando/cmd: false \&\& echo desplegando/' $D/pipeline/steps/02-desplegar/commands.yaml
rm $D/pipeline/steps/02-desplegar/commands.yaml.bak
confirmar romper
intentar; echo "código: $?"       # Estado "fallido", código 1
INT=<Intento de la respuesta>
vexd_demo diagnosticar "{\"Version\":\"1\",\"Intento\":\"$INT\",\"Ambiente\":\"prod\"}"
# → con_atribucion, eje "instrucciones", paso "desplegar"
```

**Rollback.** Toma un `Id` de `despliegues` y vuelve a él con `rollback`. Genera un despliegue nuevo.

**Ambiente ocupado.** Pon un `cmd: sleep 60` en un paso, lanza `intentar` y mátalo con `kill -9` (con Ctrl-C el
intento se cierra como `cancelado` y no ocupa nada). El siguiente `intentar` falla con `-32004` (salida 1)
diciendo en `data.intento` qué intento ocupa el ambiente, y `abandonar` con ese id lo libera.

**Intento con copia de trabajo.** Añade `"CopiaDeTrabajo": "$D/proyecto"` a la petición: responde `exitoso` con
`Despliegue` vacío.

**Solo un paso.** Añade `"HastaPaso": "preparar"`: el intento se detiene ahí.

**Errores esperados.**

```bash
vexd_demo reservar '{"Version":"9","Ambiente":"prod"}'; echo $?     # -32001, versión no soportada; 2
vexd_demo reservar '{"Version":"1","Ambient":"prod"}'; echo $?      # -32602, campo desconocido; 2
VEX_ALMACEN=/no/existe vexd_demo intentos '{"Version":"1","Ambiente":"prod"}'; echo $?   # -32006, almacén inexistente; 2
```

**Empezar de cero.** `scripts/demo.sh --limpio`, o borra `$DEMO_DIR/almacen/*` para olvidar el historial sin
tocar los repos.

## Si algo falla

| Síntoma | Causa probable |
|---|---|
| `-32006` «falta VEX_ALMACEN» | No exportaste la variable ni hiciste `source entorno.sh` |
| `-32006` «el almacén del historial (VEX_ALMACEN): … no such file» | El directorio del almacén no existe. Créalo con `mkdir` |
| `-32602` «parámetros ilegibles: json: unknown field "X"» | El campo no existe en esa operación; revisa su tabla |
| `-32700` o `-32600` | El mensaje no es una línea JSON-RPC: un salto dentro de la petición, o falta `id`/`method` |
| `-32001` «versión no soportada» | Falta `"Version": "1"` en `params` |
| `-32000` «error interno» | Mira stderr: ahí va la causa |
| `simulación: … "" no es un commit` | `simular` con `Fuente` necesita `Commit` completo; o usa `CopiaDeTrabajo` |
| `-32004` «el ambiente "prod" tiene en curso el intento …» | Hay un intento sin desenlace: espera, o `abandonar` |
| El intento no ejecuta nada | Nada cambió respecto al anterior; edita algo y haz commit |
| Cambié un fichero del pipeline y no se nota | Falta el commit (salvo con `CopiaDeTrabajo`) |
| `Estado: fallido` sin más | Mira `logs` con `"Resultado": "fallido"`: es la salida del comando que falló |

Las pruebas automáticas del protocolo están en `cmd/vexd/main_test.go` y `internal/protocolo`: son un buen modelo de
peticiones válidas.
