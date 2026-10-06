# Historial — modelo del contexto

> **Supporting**, y realiza también Sincronización del Historial (*generic*). Qué es, en `dominio.md`;
> su lenguaje, en `lenguaje.md`; su frontera, en `bounded-contexts.md`; sus relaciones, en
> `context-map.md` (filas #1 a #4); cómo vive en paquetes, en `arquitectura.md`.
>
> **Vigente** desde el cierre de IT-07 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Qué ha pasado**: qué intentos hubo, cuáles llegaron a despliegue, qué se lanzó y qué se reservó, con qué
se hizo cada cosa y en qué orden. Es la memoria del negocio, y el único sitio del que el core lee.

**Su regla de fondo** (`DEC-03.13`): **la forma es suya, y lo que dice cada registro es de quien lo
produjo.** Guarda contenido de otros contextos sin aplicarle sus reglas. Su forma incluye las **claves**
por las que sus clientes necesitan buscar, como el hash del código de cada intento, que para el Historial
es una clave más (`DEC-07.6`).

---

## Lo que le piden

Se escribe a partir de sus clientes, igual que Diagnóstico empezó por sus escenarios.

### Quién escribe

| Quién | Qué escribe |
|---|---|
| **Ejecución** | que se abre un intento: ambiente, solicitante, los pasos del pipeline con su ámbito y hasta cuál se pide, y como contenido el commit de cada fuente (o que es una copia de trabajo, sin commit), los hashes y el orden de los ambientes · por cada paso que se ejecuta, un registro al empezar y otro al terminar, exitoso o fallido; por cada comando que termina, su salida y si salió bien, aparte de los registros del intento · por cada paso que no se re-ejecuta, un registro con su evidencia (`DEC-09.7`); como contenido, la razón, el hash de sus instrucciones y su declaración congelada · que se cierra el intento, con su estado y, si es un rollback, su destino |
| **Resolución** | el hash de cada variable, bajo paso e intento · el valor ofuscado, por la relación reservada (`DEC-04.7`) |
| **Lanzamiento** | un lanzamiento, con su versión y su nombre como contenido · una reserva o una liberación de un ambiente |
| **Borde** | que un intento sin desenlace se da por abandonado (`DEC-07.8`) |

### Quién lee

| Quién | Qué lee |
|---|---|
| **Diagnóstico** | la tabla de `contextos/diagnostico.md`: despliegues de un ambiente en orden, el último con un hash del código dado, los ejes de cada paso, la evidencia, el estado, la cantidad de intentos, el despliegue de un lanzamiento |
| **Ejecución** | la última vez de un paso en su ámbito · un despliegue anterior y su intento, para un rollback |
| **Resolución** | el hash y el valor de la última vez de cada variable (el valor, por la relación reservada) |
| **Lanzamiento** | que se registró un despliegue (evento) · el último despliegue y la última reserva de un ambiente |
| **Borde** | el historial, sin valores, para el CLI y el portal · la salida de los comandos de un intento, con un filtro por resultado, y cuál fue el último intento |

---

## Modelo táctico

### Agregados

Cada agregado **son sus registros**: se añaden y no se cambian (`DEC-05.5`). Cada registro se escribe
solo, en cuanto ocurre el hecho (`DEC-05.9`), después de comprobar las invariantes de su agregado contra
los registros que ese agregado ya tiene.

| Agregado | Qué es | Invariantes que protege |
|---|---|---|
| **Intento** | una apertura, con los pasos del pipeline y hasta cuál se pide; los registros de sus pasos; y, si llega, un cierre o un abandono | una sola apertura · como mucho un cierre · **ningún registro después del cierre o del abandono** · por cada paso pedido, un comienzo y, si llega, un final, o bien una no re-ejecución; ningún final sin comienzo (`DEC-09.7`) · el estado sale del cierre, y sin cierre está **sin desenlace** (`DEC-05.8`) · solo se abandona si está sin desenlace (`DEC-07.8`) |
| **Despliegue** | un intento exitoso de todos sus pasos, con identidad propia y un padre | su intento está cerrado como exitoso y tiene, en **todos** los pasos del pipeline, un final exitoso o una no re-ejecución (`DEC-07.9`, `DEC-09.7`) · su padre es un despliegue anterior del mismo ambiente, o ninguno si es el primero · su intento se hizo con commits: un intento con una copia de trabajo nunca llega a despliegue (`DEC-10.7`) · no admite grados |
| **Lanzamiento** | un despliegue que se hace visible, con fecha | su despliegue existe y es de ese ambiente |
| **Reserva** | que el dueño del negocio reserva o libera un ambiente | ninguna más allá de su propio registro |
| **Ocupación** | qué intento tiene un ambiente en curso | **como mucho una ocupación vigente por ambiente**: se ocupa al abrir un intento y se libera con su cierre o su abandono (`DEC-07.8`, `DEC-07.9`) · **un dueño muerto no la retiene**: quien encuentra el ambiente ocupado y ve que el intento no late —ni latidos ni registros nuevos durante la **ventana de vida**— lo cierra como fallido con causa **interrumpido** (o lo abandona, si ni llegó a abrirse) y ocupa el ambiente |

**El despliegue lo crea el Historial**, no quien ejecuta (`DEC-07.3`). Cuando Ejecución cierra un
intento como exitoso, el **Intento** comprueba que tiene todos sus pasos y actúa de factoría del
**Despliegue**, con el padre por defecto (el último despliegue del ambiente) o con el destino del
rollback. Después de escribirlo, publica *despliegue registrado* (`DEC-04.8`, `DEC-05.9`).

**Lo anterior se deduce; el padre se guarda** (`DEC-07.4`). El intento anterior y el lanzamiento
anterior salen del orden temporal del ambiente, que es un orden total. El padre de un despliegue no se
puede deducir, porque un rollback lo elige.

### Entidades y value objects

| | Qué es |
|---|---|
| **registro de paso** *(entidad, dentro de Intento)* | lo que dejó un paso en un intento. Su identidad es el intento y el paso, y por eso una evidencia puede apuntar a él |
| **identidad** de intento, de despliegue y de lanzamiento | propia y única |
| **paso en su ámbito** | la posición bajo la que se buscan los registros de un paso (`DEC-06.12`). El Historial la guarda y no la decide: **quién le da ámbito a un paso lo declara Definición** (`config.yaml`, `steps.<paso>.scope`; RD-04 §9.19) |
| **clave** | por lo que se busca un registro sin saber qué significa: la forma propia y lo que piden sus clientes. Hoy, el hash del código de cada intento, pedido por Diagnóstico (`DEC-07.6`) |
| **estado** | exitoso · fallido · cancelado |
| **evidencia** | un enlace: el intento y el paso del registro que vale |
| **contenido** | lo que dice un registro, con el contexto al que pertenece. **Opaco para el Historial** |
| **valor ofuscado** | solo por la relación reservada, y solo hacia Resolución |

### Evento de dominio

**Despliegue registrado**: lo publica el agregado Despliegue después de escribirse, y lo escucha
Lanzamiento.

### Repositorios

Uno por agregado: **intentos**, **despliegues**, **lanzamientos** y **reservas**, y uno de **salidas**, que
guarda la de cada comando de un intento en su propia secuencia y no cambia lo que el intento dice de sí mismo. Solo **añaden** y
**recorren** (`DEC-07.5`). Por debajo, todos escriben en el mismo sitio, detrás de la interfaz del
Historial, y *sincronizar* vive ahí (`DEC-06.18`).

**El almacén se compra** (IT-10 `DEC-10.3`), y tiene que garantizar tres cosas: que lo escrito se pueda leer
desde cualquier máquina antes de seguir, que un registro no se sobrescriba nunca, y una **escritura
condicional por agregado**: añadir un registro solo si nadie añadió otro a ese agregado desde que se leyó
(`RD-02` §9). Es lo que hace que la **Ocupación** solo tenga éxito si el ambiente está libre, que dos
lanzamientos no tomen la misma versión (IT-10 `DEC-10.8`) y que la máquina de un intento abandonado no pueda
seguir escribiendo (IT-07 `DEC-07.8`).

### Si el dueño del ambiente murió

Un intento sin desenlace puede haber muerto, o seguir corriendo en otra máquina: el Historial no puede saberlo
mirando solo sus registros, y los relojes de dos máquinas no tienen por qué coincidir. Por eso no compara
instantes para decidir que murió: **observa**.

- Mientras vive, el proceso de un intento escribe un **latido** cada pocos segundos, aunque su comando no escriba
  nada. Los latidos son una secuencia propia por intento: no son un hecho del negocio ni forman parte de sus
  registros.
- Quien encuentra el ambiente ocupado cuenta los latidos y los registros del dueño, espera la **ventana de vida**
  (el triple del intervalo entre latidos) y vuelve a contar. Si ninguno creció, el dueño murió.
- Si el último latido es más reciente que la ventana, se da al dueño por vivo **sin esperar**. Ahí sí se mira el
  reloj, pero solo para ahorrar la espera: **nunca para declarar una muerte**. Un reloj desfasado hace, como mucho,
  que un huérfano tarde más en recuperarse; jamás que se libere uno vivo.
- Dar por muerto escribe un cierre **fallido** con causa **interrumpido**, protegido por la misma escritura
  condicional de siempre: si el dueño escribe mientras se decide, no se cierra. Si varios intentos recuperan a la
  vez, solo uno ocupa el ambiente.
- Un intento de una versión que no latía parece muerto aunque corra: solo importa si dos versiones del motor
  comparten almacén a la vez.

Es lo que garantiza que un ambiente no queda bloqueado para siempre por un proceso caído (`kill -9`, falta de
memoria, una máquina que se apaga). `abandonar` sigue existiendo para quien no quiere esperar.

### Lo que publica

| Hacia | Qué |
|---|---|
| **Ejecución** | abrir un intento, que se rechaza si el ambiente está ocupado · registrar un paso · registrar la salida de un comando · cerrar un intento · la última vez de un paso en su ámbito · un despliegue y su intento |
| **Resolución** | registrar el hash de una variable · la última vez de una variable · **por la relación reservada**: el valor ofuscado |
| **Lanzamiento** | registrar un lanzamiento · registrar una reserva · el último despliegue y la última reserva de un ambiente · el evento *despliegue registrado* |
| **Diagnóstico** | las consultas de su tabla de requisitos |
| **Borde** | consultar el historial, **sin valores** (`DEC-04.7`) · las salidas de un intento y el último intento · dar por abandonado un intento (`DEC-07.8`) |

