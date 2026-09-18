# El lenguaje publicado del motor — versión 1

> Lo que el motor ofrece al CLI y al portal (Open Host Service + Published Language, `DEC-04.4`). Lo implementa
> `internal/borde/`. El CLI y el portal se adaptan a este documento (`DEC-11.6`), no al revés.

## Reglas comunes

- **Una petición por invocación.** El motor atiende una operación y termina.
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

## Operaciones

| Operación | Petición | Respuesta | Contexto |
|---|---|---|---|
| **Intentar** hasta un paso en un ambiente | `ejecucion.PeticionDeIntento` | `Resultado` | Ejecución (EJ-1) |
| **Hacer rollback** a un despliegue | `ejecucion.PeticionDeRollback` | `Resultado` | Ejecución (EJ-2) |
| **Simular** un pipeline, sin efectos | `simulacion.PeticionDeSimulacion` | `Informe` | Simulación (SIM-1, SIM-2) |
| **Lanzar** un despliegue | `borde.PeticionDeLanzamiento` | `lanzamiento.Lanzamiento` | Lanzamiento (LAN-2) |
| **Reservar** un ambiente | `borde.PeticionDeReserva` | — | Lanzamiento (LAN-3) |
| **Liberar** un ambiente | `borde.PeticionDeLiberacion` | — | Lanzamiento (LAN-3) |
| **Preguntar la causa** de un fallo | `diagnostico.PeticionDeDiagnostico` | `Respuesta` | Diagnóstico |
| **Dar por abandonado** un intento | `borde.PeticionDeAbandono` | — | Historial |
| **Consultar un intento** | `borde.PeticionDeConsultaDeIntento` | `historial.Intento` | Historial |
| **Consultar los intentos** de un ambiente | `borde.PeticionDeIntentosDeUnAmbiente` | `[]historial.Intento` | Historial |
| **Consultar los despliegues** de un ambiente | `borde.PeticionDeDesplieguesDeUnAmbiente` | `[]historial.Despliegue` | Historial |

### Detalles por operación

- **Intentar / Hacer rollback.** Reciben además una `Salida` a la que se entrega, **en vivo y tal cual**, lo que
  imprimen los comandos de cada paso (`DEC-12.5`). No se guarda ni se tapa: no es un registro. La respuesta de la
  operación es aparte. Un intento con copia de trabajo (`CopiaDeTrabajo`) nunca llega a despliegue. Un rollback
  toma el ambiente y las fuentes del propio despliegue destino.
- **Simular.** Fuente y commit, o una copia de trabajo (que tiene prioridad). El `Informe` lleva nombres, nunca
  valores: qué se interpoló y qué faltó, por paso y por ambiente.
- **Lanzar.** Es incondicional: la reserva de un ambiente solo bloquea el lanzamiento en nombre del actor
  ausente, nunca al dueño del negocio. Sin nombre, el nombre toma la versión.
- **Preguntar la causa.** Un intento **o** un lanzamiento (nunca los dos), en un ambiente, y opcionalmente una
  referencia elegida a mano. La respuesta es de una de tres formas: `con_atribucion`, `sin_referencia`,
  `no_se_atribuye`.
- **Consultas del Historial.** Solo lectura. Lo que dicen los registros de cada contexto (`Contenido`) viaja
  opaco.

## Cómo evoluciona

Una versión nueva de una operación o de sus tipos es una versión nueva del lenguaje: se añade a
`borde.VersionesSoportadas` sin quitar la anterior mientras haya clientes que la usen.
