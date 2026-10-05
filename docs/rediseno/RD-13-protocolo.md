# RD-13 — El protocolo del motor (vexd ↔ vex)

> Estado: **plan aprobado, en implementación** (fases en el plan de implementación). Sustituye la sección «La invocación» de `docs/modelo/lenguaje-publicado.md`
> cuando se implemente. Las operaciones y sus tipos (`borde`) **no cambian**; cambia cómo viajan.

## Contexto

Tres piezas: `vexd` (el motor), `vex` (la CLI) y, más adelante, la web. `vexd` corre en un contenedor
**efímero**: lo levanta quien invoca (primero `vex`, luego la web), atiende **una** petición y muere. Muchos
contenedores pueden correr en paralelo; comparten el almacén (un volumen) pero no se hablan entre sí.

De ahí salen las decisiones:

- No hay servidor: nada escucha en un puerto, no hay nada que atacar mientras el contenedor no existe.
- El canal natural es la entrada y la salida estándar del contenedor (`docker run -i`).
- Hay **un solo adaptador de entrada**. No existe un modo «humano» por argumentos que mantener en paralelo.

## Decisiones

| # | Decisión |
|---|---|
| P-1 | Protocolo: **JSON-RPC 2.0**, un mensaje JSON por línea (NDJSON) en `stdin`/`stdout`. |
| P-2 | **Una petición por proceso.** `vexd` lee una, responde y termina. |
| P-3 | `stdout` es solo protocolo. Los logs del propio motor van a `stderr`. |
| P-4 | Un único adaptador (stdio). `vexd` no tiene subcomandos ni flags de operación. |
| P-5 | Los directorios (almacén, espacio, material) son configuración del proceso, **solo** por variables `VEX_*`; no van en la petición (`DEC-06.18`) ni hay flags. Los valores por defecto los fija la imagen (`ENV` del `Dockerfile`); quien lanza el contenedor solo elige qué monta en cada ruta. |
| P-6 | La petición puede llevar **variables de entorno para los comandos**, en un miembro propio (`entorno`). El motor no gestiona secretos (`DEC-08.8`): son variables que pueden o no ser sensibles. |
| P-7 | Presentación para humanos (resúmenes, colores, tablas) **no es del motor**: es de `vex`. |

## Ciclo de vida

```
vex ── docker run -i vexd ──►  contenedor
vex ── {request} ───────────►  vexd lee UNA línea
vex ◄─ {notificación progreso} (0..n)
vex ◄─ {response} ──────────   vexd escribe UNA respuesta y sale
```

- `stdin` se cierra **antes** de recibir la petición → error de invocación, salida `2`.
- Que `stdin` se cierre **después** de la petición **no cancela nada**: significa «no envío más», como en
  `echo '{…}' | vexd`, donde el pipe cierra la entrada nada más escribir. Cancelar es explícito: la notificación
  `cancelar` o `SIGTERM`/`SIGINT`. Se cancela el `context`.
  - Si el intento ya se abrió, la respuesta es un `result` con el `Resultado` `cancelado` y la salida es `130`.
  - Si se cancela **antes** de abrirlo (aún se resolvían las fuentes), no hay intento: es el error `-32007`
    `cancelado`, salida `130`.
- **Cancelar acaba con todo lo que lanzó el comando, no solo con el shell.** Cada comando corre en su propio
  grupo de procesos: al cancelar recibe `SIGTERM` (así terraform puede soltar el bloqueo de su estado) y, pasado el
  plazo de gracia (5 s, menor que los 10 s de `docker stop`), `SIGKILL` el grupo entero. Más señales no matan a
  `vexd`: dejarían huérfano lo que lanzó y el intento sin cerrar. Por eso `vexd` siempre termina solo, como mucho
  a los 5 s, con el intento `cancelado`.
- Si quien invoca muere sin cancelar, el contenedor sigue vivo. Evitarlo es de quien lo lanzó (`--rm` y detener el
  contenedor al terminar), no del protocolo.
- Una línea no puede pasar de un límite fijo (propuesta: 1 MiB); excederlo es petición inválida.
- Si el proceso muere sin escribir respuesta, quien invoca lo toma como fallo del motor y mira `stderr`.
- Hay que invocar **sin TTY** (`-i`, nunca `-t`): un TTY mezcla `stdout` y `stderr`.

## Mensajes

### Petición

```json
{"jsonrpc":"2.0","id":"1","method":"intentar",
 "params":{"Version":"1","Ambiente":"sand","Solicitante":"jailux","FuenteDelProyecto":"/proyecto",
           "FuenteDelPipeline":"/pipeline","HastaPaso":"supply",
           "Metadatos":{"ProjectName":"vex-demo","ProjectId":"p1"}},
 "entorno":{"REGISTRY_URL":"registry.local"}}
```

- `method` es el nombre de la operación, igual que hoy: `intentar`, `rollback`, `simular`, `lanzar`,
  `reservar`, `liberar`, `diagnosticar`, `abandonar`, `intento`, `intentos`, `despliegues`, `logs`, más
  `describir` (abajo).
- `params` es **exactamente** el tipo de petición del borde, con su `Version`. La lectura sigue siendo estricta:
  un campo desconocido es `-32602`.
- `entorno` (opcional) es un mapa de nombre → valor, un miembro de la petición fuera de `params` para no tocar
  los tipos del borde. Es la única extensión sobre JSON-RPC. Los nombres deben tener forma de variable de
  entorno (`[A-Za-z_][A-Za-z0-9_]*`); si no, `-32602`. Mientras no esté implementado, un `entorno` no vacío se
  rechaza con `-32602`: nunca se ignora en silencio.
- `id` es obligatorio: sin `id` sería una notificación y no habría respuesta.

### Respuesta

Éxito — `result` es lo que hoy devuelve el borde, **completo** (sin resumir):

```json
{"jsonrpc":"2.0","id":"1","result":{"…":"Resultado"}}
```

Error — no se pudo atender la petición:

```json
{"jsonrpc":"2.0","id":"1","error":{"code":-32001,"message":"el ambiente tiene otro intento en curso",
  "data":{"tipo":"ambiente_ocupado","intento":"01a1…"}}}
```

**Resultado de negocio ≠ error de protocolo.** Un intento que termina `fallido` o `cancelado` es un `result`
correcto cuyo `Resultado` lo dice. Solo es `error` lo que impide atender la petición. El código de salida del
proceso sí refleja ambos (ver abajo), como hoy.

### Notificaciones

`vexd → vex`, sin `id`, sin respuesta. Quien no entienda una, la ignora.

```json
{"jsonrpc":"2.0","method":"progreso","params":{"evento":"paso_iniciado","paso":"test"}}
```

Eventos previstos: `intento_iniciado`, `paso_iniciado`, `paso_terminado`, `comando_terminado`
(comando y resultado, **sin su salida**: esa se guarda en el Historial y se lee con `logs`). Ninguno es
obligatorio en la versión 1; se añaden sin romper a nadie.

`vex → vexd`: `cancelar` (notificación sin `id` ni parámetros), equivalente a la señal. `vexd` la espera en lo que queda de la entrada.

### `describir`

Sin parámetros. Responde la versión del motor, las versiones del lenguaje que soporta
(`borde.VersionesSoportadas`) y las operaciones. Existe porque la imagen del contenedor y la CLI se versionan
por separado: `vex` puede comprobar compatibilidad antes de enviar nada.

## Errores

| `code` | `data.tipo` | Origen | Salida |
|---|---|---|---|
| `-32700` | `json_invalido` | la línea no es JSON | 2 |
| `-32600` | `peticion_invalida` | no es JSON-RPC válido (falta `method`/`id`, línea demasiado larga) | 2 |
| `-32601` | `operacion_desconocida` | `method` no existe | 2 |
| `-32602` | `parametros_invalidos` | campo desconocido o `ErrPeticionInvalida`, `*.ErrInvalido` | 2 |
| `-32001` | `version_no_soportada` | `borde.ErrVersionNoSoportada` | 2 |
| `-32002` | `rechazado` | `ejecucion.ErrRechazado`, `historial.ErrRechazado` | 1 |
| `-32003` | `no_existe` | `historial.ErrNoExiste`, incluido `borde.ErrHistorialSinIntentos` (`logs` sin ningún intento: ya no es un mensaje con salida 0, el motor no tiene texto para humanos) | 1 |
| `-32004` | `ambiente_ocupado` | `*historial.AmbienteOcupadoError` (`data.intento`) | 1 |
| `-32005` | `no_disponible` | `ejecucion.ErrNoDisponible` | 1 |
| `-32006` | `configuracion_invalida` | falta una variable `VEX_*` obligatoria, o su directorio no existe | 2 |
| `-32007` | `cancelado` | cancelación antes de abrir el intento (`context.Canceled`) | 130 |
| `-32000` | `interno` | cualquier otro; el detalle va a `stderr` | 1 |

Cancelado con el intento abierto: `result` con `Resultado` cancelado, salida `130`. Intento `fallido`: salida `1`.
Todo lo demás, `0`.

**Orden de clasificación.** `*historial.AmbienteOcupadoError` se comprueba con `errors.As` **antes** que los
sentinelas: también cumple `errors.Is` contra los dos `ErrRechazado`, y si se mirara después perdería `data.intento`.

**Errores de contextos ajenos.** Los de Definición, Suministro y Resolución los traduce, como un ACL, la
`infraestructura` de Ejecución y de Simulación a su error de dominio (conservando la causa, así que `errors.Is` y
`errors.As` siguen viendo el original), y la `aplicacion` de cada una lo lleva al `ErrInvalido`/`ErrRechazado`
publicado. El adaptador de `cmd/vexd` no importa esos contextos: solo clasifica lo que el borde publica.

| Origen | Se vuelve | Respuesta |
|---|---|---|
| fuente, commit o copia de trabajo que no están (`ErrNoExiste`), o que no se pueden pedir (`ErrInvalido`) | `ErrInvalido` | `-32602`, salida 2 |
| pipeline que no pasa la comprobación (`ErrNoComprobado`), o variable que no existe (`ErrRechazado` de Resolución) | `ErrRechazado` | `-32002`, salida 1 |
| cualquier otro | sin cambio | `-32000` interno, salida 1 |

En `simular`, un pipeline que no pasa la comprobación no es un error sino el resultado (`Causa.Fallos`).

Los códigos y `data.tipo` son **estables**: añadir uno es compatible, cambiar o reutilizar uno no.

## Nombres de campo

Los tipos publicados no llevan tags `json`: en el cable van los nombres de Go (`Estado`, `HastaPaso`), como hoy.
Cambiarlos es un cambio de contrato y exigiría una versión nueva del lenguaje.

## Configuración del proceso

| Variable | Obligatoria | Para qué |
|---|---|---|
| `VEX_ALMACEN` | siempre | almacén del Historial; el directorio tiene que existir |
| `VEX_ESPACIO` | solo `intentar` y `rollback` | espacio de trabajo de los ambientes |
| `VEX_MATERIAL` | no | copia desechable del material de las fuentes; sin ella, un directorio temporal |

- Solo `main` lee el entorno. `ejecutar` recibe la configuración ya resuelta, para poder probarlo entero.
- Una variable obligatoria ausente **no es culpa de la petición**: responde `error` con
  `data.tipo = configuracion_invalida`, código `-32006`, salida `2`, y el mensaje nombra la variable
  (`falta VEX_ALMACEN: dónde está el historial`).
- La validación sigue siendo por operación (`usaEspacio`), y ocurre antes de tocar ningún contexto.
- En la imagen: `ENV VEX_ALMACEN=/vex/almacen VEX_ESPACIO=/vex/espacio VEX_MATERIAL=/vex/material`. `vex` solo
  monta volúmenes en esas rutas; no se las pasa a `vexd`.

## Entorno de los comandos

**El motor no gestiona secretos**: es otro producto (`DEC-08.8`, y `DEC-12.5`: tapar valores es una regla de
seguridad que este producto no asume). `entorno` solo permite pasar variables de entorno a los comandos. Que
alguna sea sensible es decisión de quien la pasa.

- Llegan por `entorno` en la petición (P-6) y se añaden al entorno del proceso de **cada comando** del intento.
  No pasan por `docker inspect` ni por argumentos.
- No son variables del pipeline: no pasan por Resolución, no entran en ningún hash ni se guardan en el Historial
  (`DEC-04.7`: el motor no guarda valores de variables).
- No hay enmascarado ni transformación: lo que un comando imprima se guarda como cualquier salida y se lee con
  `logs`. Un comando que haga `env` mostrará lo que recibió.
- Higiene mínima, no gestión: el motor no copia los valores a sus propios errores, a `stderr` ni a `progreso`.
- Dentro del contenedor, el proceso hijo las tiene en `/proc/<pid>/environ`. Es el riesgo que asume quien las pasa.
- Valen para todos los comandos del intento. Declararlas por paso exigiría un cambio en Definición y no está
  previsto.

## Lo que desaparece de `cmd/vexd`

- `leerOpciones`, `--entrada`, `uso`, el subcomando `version`: el único modo de entrada es stdio.
- Las flags `--almacen`, `--espacio`, `--material`: pasan a ser solo `VEX_ALMACEN`, `VEX_ESPACIO`,
  `VEX_MATERIAL`. Una sola forma de configurarlo.
- La presentación para humanos (`resumir`, `presentarDiagnostico`, `mostrarDespliegues`, `presentarLogs`, la
  salida con sangría): se mueve a `vex`, que recibe la respuesta completa y decide qué mostrar.

## Lo que se queda

- `internal/borde` y los contextos, sin cambios.
- La tabla `operaciones` pasa a ser el registro de métodos: añadir una operación sigue siendo una fila y un tipo.
- `decodificar` estricta.
- Los códigos de salida.
- `scripts/demo.sh` se adapta: arma el mensaje y lo envía por `stdin`.

## Cómo evoluciona

- Operación nueva: fila nueva en la tabla y tipo nuevo en el borde.
- Cambio incompatible de una operación: versión nueva (`Version`), sin quitar la anterior mientras haya clientes.
- Evento de progreso nuevo o `data` nuevo en un error: compatible; los clientes ignoran lo que no conocen.
- Varias peticiones por sesión, o peticiones del motor a quien invoca: caben sin rediseñar, porque JSON-RPC
  ya es bidireccional y lleva `id`.

## Pendiente de verificar al implementar

1. **Eventos de progreso.** Ejecución tiene que ofrecer un punto donde emitirlos; es el único cambio real dentro
   de `internal`. Hasta entonces, la versión 1 puede no emitir ninguno.
2. **Inyección de `entorno`.** Hoy el hijo hereda el entorno de `vexd` y no se le añade nada
   (`ejecucion/infraestructura/comandos.go`); hay que ampliar el puerto de comandos.
3. **Almacén compartido entre contenedores.** Escritura atómica y decisión atómica de `ambiente_ocupado` con
   varios escritores; los bloqueos de fichero no son fiables en volúmenes de red.
4. **Contrapresión.** Si `vex` no lee `stdout`, emitir progreso no debe bloquear al motor.
