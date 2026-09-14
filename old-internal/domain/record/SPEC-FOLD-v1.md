# El plegado de un intento — `Fold`

> **Estado:** congelada · **Implementación de referencia:** `fold.go`, en este
> mismo directorio · **Casos:** `fold_test.go`

Este documento es **normativo**, y lo es por una razón concreta: `Fold` se ejecuta
**en dos lugares** —el motor y el backend (spec 26)— y las dos implementaciones
tienen que coincidir. Misma lógica, dos lenguajes.

---

## 1. Qué es

```
Fold(events []Event) AttemptResult
```

Una función de dominio **pura**: sin estado, sin dependencias, determinista. No
es un servicio, y que se pueda ejecutar idéntica en el backend es consecuencia de
eso, no una decisión aparte.

**El resultado no se guarda, se deriva.** Guardarlo sería duplicar lo que esta
función calcula, y dos fuentes de verdad para el mismo dato divergen — por eso se
descarta Memento (spec 17 §5.3').

## 2. Las tres invariantes

### 2.1 Ordena por `seq`, nunca por `time`

El reloj no es confiable: un contenedor, una máquina remota, un ajuste NTP. El
orden sí importa para plegar, y por eso `seq` existe **además** del instante.

El orden es **estable**: dos hechos con la misma posición conservan el orden en
que llegaron, que es lo único razonable cuando la posición miente.

### 2.2 No muta lo que recibe

El slice se clona antes de ordenarlo. Un plegado que reordena los hechos de quien
lo llama no es puro por mucho que lo parezca.

### 2.3 Tolera lo que no entiende

Un tipo de evento desconocido —escrito por un motor más nuevo— se **ignora**, no
hace fallar el pliegue. Es la mitad lectora del OCP: añadir un tipo no toca a
`Fold` para los existentes, y encontrarse uno no invalida los que sí se entienden.
Lo mismo con un `Event` cero.

## 3. El desenlace

| Condición | `status` |
|---|---|
| llegó `attempt_finished` | lo que ese hecho declara |
| **no** llegó | `interrupted` |

**Es el caso que justifica el modelo entero.** Si no llega `attempt_finished`, el
resultado igual existe y dice exactamente hasta dónde se llegó: `interrupted`,
con `last_step`. Es lo que hace que un `Ctrl-C` o una Fly Machine que muere dejen
datos útiles en vez de nada.

`interrupted` **no se emite jamás**: se deriva de una ausencia, y quien está vivo
para escribir el desenlace, por definición, no fue interrumpido.
`AttemptFinished.Validate` lo rechaza explícitamente.

Un slice vacío pliega a `interrupted` con todo lo demás en cero. Es coherente:
la regla es «sin `attempt_finished` ⇒ interrumpido», y no hay excepción para el
caso en que tampoco haya nada más.

## 4. La derivación, hecho por hecho

| Hecho | Efecto sobre el resultado |
|---|---|
| `attempt_started` | fija `deployment`, `attempt`, `actor`, `runner` y `started_at` |
| `step_started` | abre el step: `status = RUNNING`, `finished = false`; fija `scope` y `step_fingerprint`; actualiza `last_step` |
| `step_finished` | cierra el step con su estado, duración, `from_cache`, evidencia, exit code y clase de error; actualiza `last_step` |
| `command_finished` | `commands++`, y `failed_commands++` si falló |
| `artifact_produced` | `artifacts++` |
| `stale_clone_used` | `stale_clones++` |
| `attempt_finished` | fija `status` y `finished_at` |
| `command_started`, `parameter_resolved`, `sync_failed` | no alteran el resultado; su valor está en la tira, no en el pliegue |

Reglas de composición:

- **`last_step` es el último step ABIERTO o CERRADO**, en orden de `seq`. Es lo
  que hace útil un intento interrumpido, y por eso es un campo propio y no algo
  que el consumidor tenga que buscar en la lista.
- **El fallo del intento es el del PRIMER step que falló.** Los posteriores no
  llegaron a correr, así que atribuirle el último sería contar el desenlace al
  revés.
- **`duration` sólo existe si existen los dos instantes.** Un intento
  interrumpido tiene principio, no duración.
- **Los steps salen en el orden en que se abrieron**, no en el de sus
  identificadores.
- **`attempt` sale del sobre del primer hecho** si no hay `attempt_started`. Un
  archivo truncado por delante sigue diciendo de qué intento es.

## 5. Los bordes que no son anomalías

| Situación | Por qué es normal |
|---|---|
| un `step_finished` **sin** su `step_started` | un archivo truncado por delante es cómo se ve una máquina efímera que murió y dejó la cola. El hecho ocurrió: perderlo no ayuda |
| un `step_started` **sin** su cierre | el step empezó y no terminó, que no es lo mismo que fallar. `finished = false` y `status = RUNNING` lo dicen |
| un intento con steps exitosos y **cero** registros | tres clases de step ejecutan, terminan bien y no escriben registro: sin `config.yaml`, sin comandos y sin `rules`. El hecho es DEL STEP, no de la escritura |
| un `step_finished` con `step_fingerprint` vacío | desde la spec 15, un step que no declara `state_changed` no compara contenidos y el motor no le calcula huella. Hay steps que se saltan por vigencia y no por contenido |
| `error_class: unknown` | es un valor legítimo y **preferible** a forzar una clasificación. Guardar hechos, nunca conclusiones |

## 6. Cómo validar una implementación independiente

1. Una tira completa produce el desenlace que declara `attempt_finished`.
2. La misma tira sin su último hecho produce `interrupted`, con el mismo
   `last_step` que tenía en ese punto. **Es el caso que justifica el modelo.**
3. Desordenar la tira por instante **no cambia nada**: se ordena por `seq`.
4. Plegar dos veces la misma tira da el mismo resultado, y no altera la tira.
5. Una tira con dos steps fallidos atribuye al intento el exit code y la clase
   del **primero**.
6. Una cancelación (`attempt_finished{canceled}`) es distinguible de una
   interrupción.
7. Un tipo de evento desconocido no rompe el pliegue.
8. Un slice vacío pliega a `interrupted`, no a un error.
