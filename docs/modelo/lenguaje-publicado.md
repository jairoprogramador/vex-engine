# El lenguaje publicado del motor — versión 1

> Lo que el motor ofrece al CLI y al portal (Open Host Service + Published Language, `DEC-04.4`). Lo implementa
> `internal/borde/`. El CLI y el portal se adaptan a este documento (`DEC-11.6`), no al revés.

## Reglas comunes

- **Una petición por invocación.** El motor atiende una operación y termina.
- **Se habla por JSON-RPC 2.0**, un mensaje por línea, por la entrada y la salida estándar (`docs/rediseno/RD-13-protocolo.md`).
- **Toda petición declara su versión** (`Version`). Hoy solo se soporta `"1"`. Una versión distinta, o vacía, se
  rechaza con `borde.ErrVersionNoSoportada` **antes** de tocar ningún contexto.
- **Nada de lo que se consulta lleva el valor de una variable** (`DEC-04.7`). Las variables se nombran y, a lo
  sumo, llevan un hash.
- **Errores**: cada contexto publica los suyos y siguen distinguibles con `errors.Is` / `errors.As`:

  | Error | Significa |
  |---|---|
  | `borde.ErrVersionNoSoportada` | la versión de la petición no se entiende |
  | `ejecucion.ErrInvalido` · `simulacion.ErrInvalido` · `lanzamiento.ErrInvalido` · `diagnostico.ErrInvalido` | lo pedido no se puede pedir |
  | `ejecucion.ErrRechazado` · `historial.ErrRechazado` | la operación rompería una invariante |
  | `historial.ErrNoExiste` | lo consultado no está en el historial |
  | `ejecucion.ErrNoDisponible` | el espacio de trabajo del ambiente no se alcanzó; el intento no empezó |
  | `*historial.AmbienteOcupadoError` | el ambiente tiene otro intento en curso, y dice cuál (se puede abandonar) |

> Guía práctica para probarlo a mano, con ejemplos: `docs/guia-de-pruebas.md` y `scripts/demo.sh`.

## La invocación

`cmd/vexd` (el binario `vexd`) es la raíz de composición: conecta los contextos y atiende **una operación por
invocación** con `internal/borde/`. No tiene subcomandos ni opciones: habla **JSON-RPC 2.0, un mensaje JSON por línea,
por la entrada y la salida estándar** (`docs/rediseno/RD-13-protocolo.md`). No hay servidor.

```
{"jsonrpc":"2.0","id":"1","method":"intentar","params":{"Version":"1", …}}   → vexd, por stdin
{"jsonrpc":"2.0","id":"1","result":{…}}                                       ← vexd, por stdout
```

| | |
|---|---|
| **`method`** | `intentar` `rollback` `simular` `lanzar` `reservar` `liberar` `diagnosticar` `abandonar` `intento` `intentos` `despliegues` `logs` `describir` |
| **`params`** | Los campos del tipo de la tabla de abajo, con su `Version` (los nombres de campo de Go, sin distinguir mayúsculas). Un campo que el tipo no tiene se rechaza |
| **`result`** | Lo que devuelve la operación, completo. Las que no devuelven nada responden `{}`; una lista sin elementos es `[]` |
| **`progreso`** | Notificaciones (sin `id`) que `intentar` y `rollback` envían **antes** de la respuesta, que es la última línea: `intento_iniciado`, `paso_iniciado`, `comando_terminado`, `paso_terminado`. Solo nombres y resultados, nunca la salida de un comando. Ver `RD-13` |
| **`error`** | `{code, message, data}`; `data.tipo` es el nombre estable. Ver el catálogo en `RD-13` |
| **`entorno`** | Miembro opcional de la petición, fuera de `params`: variables de entorno (nombre → valor) para los comandos. Solo `intentar` y `rollback`. No son variables del pipeline ni secretos que el motor gestione: no se guardan ni pasan por Resolución. Ver `RD-13`, «Entorno de los comandos» |
| **Salida de los comandos** | no se muestra al intentar: se guarda en el Historial y se consulta con `logs` |
| **Configuración** | variables `VEX_ALMACEN` (obligatoria, el directorio tiene que existir), `VEX_ESPACIO` (solo `intentar` y `rollback`) y `VEX_MATERIAL` |

Los tres directorios los da el proceso y no la petición (`DEC-06.18`): por eso `DirectorioDelAlmacen` y
`DirectorioDeEspacioDeTrabajo` de `PeticionDeIntento` y `PeticionDeRollback` no los lee nadie.

Un intento que termina `fallido` o `cancelado` es un `result` correcto. Solo es `error` lo que impide atender la
petición.

Código de salida: `0` bien · `1` la operación falló, o el intento terminó `fallido` · `2` la petición o la
configuración son inválidas (incluida una versión no soportada) · `130` cancelado, por señal o por la notificación
`cancelar`, o el intento `cancelado`. Cerrar la entrada **no** cancela.

## Operaciones

| Operación | Petición | Respuesta | Contexto |
|---|---|---|---|
| **Intentar** hasta un paso en un ambiente | `ejecucion.PeticionDeIntento` | `Resultado` | Ejecución (EJ-1) |
| **Hacer rollback** a un despliegue | `ejecucion.PeticionDeRollback` | `Resultado` | Ejecución (EJ-2) |
| **Simular** un pipeline, sin efectos | `simulacion.PeticionDeSimulacion` | `Resultado` | Simulación (SIM-1, SIM-2) |
| **Lanzar** un despliegue | `borde.PeticionDeLanzamiento` | `lanzamiento.Lanzamiento` | Lanzamiento (LAN-2) |
| **Reservar** un ambiente | `borde.PeticionDeReserva` | — | Lanzamiento (LAN-3) |
| **Liberar** un ambiente | `borde.PeticionDeLiberacion` | — | Lanzamiento (LAN-3) |
| **Preguntar la causa** de un fallo | `diagnostico.PeticionDeDiagnostico` | `{Ambiente, IntentoExitoso, IntentoFallido, Sustento}`; sin diagnóstico que dar, solo `{SinDiagnostico}` con el motivo | Diagnóstico |
| **Dar por abandonado** un intento | `borde.PeticionDeAbandono` | — | Historial |
| **Consultar un intento** | `borde.PeticionDeConsultaDeIntento` | `historial.Intento` | Historial |
| **Consultar los intentos** de un ambiente | `borde.PeticionDeIntentosDeUnAmbiente` | `[]ResumenDeIntento`: `Id`, `Ambiente`, `Solicitante`, `HastaPaso`, `Estado` (vacío si no tiene desenlace) | Historial |
| **Consultar los despliegues** de un ambiente | `borde.PeticionDeDesplieguesDeUnAmbiente` | `[]historial.Despliegue` | Historial |
| **Consultar los logs** de un intento | `borde.PeticionDeLogs` | `borde.RespuestaDeLogs` | Historial |
| **Describir** el motor | — | `{VersionDelMotor, VersionesDelLenguaje, Operaciones}` | (ninguno: no usa el motor) |

### Detalles por operación

- **Intentar / Hacer rollback.** Lo que imprimen los comandos de cada paso **no se muestra**: Ejecución lo
  entrega al Historial al terminar cada comando, y se consulta con `logs`. La respuesta es solo el `Resultado`.
  Un intento con copia de trabajo (`CopiaDeTrabajo`) nunca llega a despliegue. Un rollback
  toma el ambiente y las fuentes del propio despliegue destino.
- **Simular.** Se pide como un intento: `Ambiente` (su valor, `sand`), `Solicitante` y `HastaPaso`, más fuente y
  commit, o una copia de trabajo (que tiene prioridad). El `Resultado` es el resumen de un intento sin `Id`
  (nada se guarda); si habría fallado, trae la `Causa` por nombre, nunca valores: los fallos de la comprobación
  o las variables que faltan en el primer paso que falla. Declara las variables estándar como un intento: los
  `Metadatos` de quien invoca, `environment` y `step_name` reales, y valores simulados para las que el motor
  genera a partir del material. Un ambiente o paso que el pipeline no tiene es
  `simulacion.ErrInvalido`, no un resultado fallido.
- **Lanzar.** Es incondicional: la reserva de un ambiente solo bloquea el lanzamiento en nombre del actor
  ausente, nunca al dueño del negocio. Sin nombre, el nombre toma la versión.
- **Preguntar la causa.** Un intento **o** un lanzamiento (nunca los dos), en un ambiente, y opcionalmente una
  referencia elegida a mano. El borde publica una `Respuesta` de una de tres formas (`con_atribucion`,
  `sin_referencia`, `no_se_atribuye`); `vexd` responde su estructura compacta, **sin texto para el usuario final**: con diagnóstico,
  `Ambiente`, `IntentoExitoso`, `IntentoFallido` y `Sustento` (cada eje, solo si cambió, con sus pasos), sin
  discriminador; sin diagnóstico, solo `SinDiagnostico`, con el motivo (`sin_referencia` o `no_se_atribuye`). Si trae
  `SinDiagnostico`, no hay diagnóstico. El texto que el Diagnóstico trae para `sin_referencia` no sale: el motivo ya
  dice qué pasó, y quien presenta decide qué decir.
- **Logs.** `Intento` es opcional: sin él, es el último que se abrió en cualquier ambiente. `Resultado` también:
  `"exitoso"` o `"fallido"` filtra por cómo terminó cada comando, y vacío los muestra todos; otro valor es una
  petición inválida (`-32602`). La respuesta dice el intento (`Intento`, que es el último si no se pidió uno) y su
  `Ambiente`, y trae `Salidas`: una lista plana, en el orden en que corrieron los comandos, con `Paso`, `Comando`,
  `Exitoso`, `Texto` (salida y error juntos, tal cual) e `Instante`. Solo aparecen los pasos que ejecutaron
  comandos en ese intento: uno que llegó hasta `test` solo trae `test`. Un paso que se precargó no tiene salidas en
  ese intento, y sin ninguna que mostrar, `Salidas` es `[]`:

  ```json
  {
    "Intento": "01a106d1-94e1-727c-a252-4efac5ab306c",
    "Ambiente": "prod",
    "Salidas": [
      {"Paso": "test", "Comando": "comando-test-01", "Exitoso": true, "Texto": "hola vex-demo\n", "Instante": "2026-10-05T22:12:42.9Z"}
    ]
  }
  ```

  Un intento que no existe es `no_existe`. Un historial sin ningún intento, si no se pide uno, es
  `historial_sin_intentos`: el mismo caso con tipo propio, para distinguirlos sin leer el texto. Sin salidas que
  mostrar, `Salidas` es `[]`. El motor no lleva texto para el usuario final: lo dice quien presenta.
- **Describir.** Sin parámetros y sin `VEX_ALMACEN`. Dice la versión del motor, las versiones del lenguaje que
  entiende (`borde.VersionesSoportadas`) y las operaciones, para que quien invoca compruebe que se entienden
  antes de pedir nada.
- **Consultas del Historial.** Solo lectura. Lo que dicen los registros de cada contexto (`Contenido`) viaja
  opaco. `intentos` responde la estructura compacta de cada intento, que `vexd` arma a partir del `historial.Intento`
  que publica el borde; el intento completo (`Apertura`, `Registros`…) se pide con `intento`.

## Cómo evoluciona

Una versión nueva de una operación o de sus tipos es una versión nueva del lenguaje: se añade a
`borde.VersionesSoportadas` sin quitar la anterior mientras haya clientes que la usen.
