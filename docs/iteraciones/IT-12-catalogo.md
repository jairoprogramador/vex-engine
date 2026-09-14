# IT-12 — Catálogo del rediseño

> Etapa: E12 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `modelo/migracion.md`
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **2 incongruencias · 7 decisiones.** **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Según el plan** (E12): traducir el modelo cerrado a unidades implementables y ordenadas, cada una con su
ficha y con un grafo de dependencias. **Cierre**: cada unidad tiene problema, objetivo, alcance,
verificación y dependencias, y el grafo no tiene ciclos (`plan-ddd.md` §8: *y cada unidad es implementable
por sí sola*). Y el frente 2 de `DEC-01.10`: E12 se cierra cuando la siguiente propuesta no cumple ninguno
de los cuatro criterios.

---

## 2. Propuesta

El catálogo está en `docs/rediseno/`: el `README.md`, con el grafo y las reglas comunes, y una ficha por
unidad, de `RD-01` a `RD-12`. Lo que lo sostiene:

1. **Las unidades son las doce de `migracion.md`**, así que el orden y el grafo ya estaban decididos, sin
   ciclos.
2. **Cada ficha dice qué decisiones del modelo implementa** y cómo se verifica. No introduce reglas nuevas.
3. **Cada ficha cita el criterio del frente 2 que cumple**, que es la vara para que una propuesta entre.
4. **§9 queda vacío hasta implementar la unidad**, que es donde el código contará lo que descubra.

---

## 3. Incongruencias

### Q-12.1 — La salida de los comandos: ¿quién la ve, y se tapan los valores? · **cerrada** → `DEC-12.5`

**La incongruencia.** El modelo dice qué queda de un intento (registros sin valores) y qué ve cada contexto,
pero **nunca dice qué pasa con lo que imprimen los comandos mientras corren**. Hoy el CLI y el portal lo
muestran en vivo, y quien despliega lo necesita para saber qué está pasando. Y un comando puede imprimir el
valor de una variable. Sin decidirlo, RD-06 y RD-10 no saben qué hacer con esa salida.

**Propuesta.** La salida de los comandos **se entrega en vivo a quien pidió el intento, tal cual, y no se
guarda** en el Historial. No se tapan valores, porque la seguridad no es el objetivo del producto
(`DEC-08.8`). La promesa de no exponer valores sigue valiendo para lo que se **guarda** y se **publica**:
registros, historial y diagnóstico.

**La alternativa que cambiaría el resultado.** Tapar en la salida los valores de las variables que el motor
conoce, como se hace hoy. Así un valor no aparece en pantalla ni en los registros del portal, pero se añade
una regla de seguridad que el producto dijo que no le corresponde, y solo tapa los valores que aparecen
literalmente.

**Respuesta.** Sí, la propuesta.

### Q-12.2 — ¿El motor nuevo lee los pipelines que ya existen? · **cerrada** → `DEC-12.6`

**La incongruencia.** El modelo dice qué declara un pipeline, pero no en qué ficheros. Las plantillas de hoy
(`pipelines/vex-tpl-*`) usan una estructura: `steps/NN-<paso>/commands.yaml` y `config.yaml`,
`variables/<ambiente>/<paso>.yaml`, `environments.yaml` y `vexpipeline.yaml`. Casi todo encaja con el modelo:
comandos, material, `outputs` con su `probe` (la variable de salida con su expresión regular), y `resolve:
step-output` (una variable de salida de un paso anterior). **`config.yaml` no encaja**:

| Hoy | El modelo |
|---|---|
| `rules: - state_changed: [pipeline, project]`, donde `pipeline` es todo lo que declara el paso, **instrucciones y variables juntas** | la regla mira por separado **código**, **instrucciones**, **variables de su ámbito** y **tiempo** (`DEC-06.12`, tu respuesta en IT-06) |
| `scope: project \| environment`, que dice dónde se guarda el estado del paso | **ámbito** de las variables: el del ambiente por defecto, o el **compartido** |

Sin cambiar `config.yaml`, no se puede decir «re-ejecuta si cambian las instrucciones» sin que también
cuenten las variables.

**Propuesta.**
- Se mantienen la estructura y las claves de hoy.
- **Solo cambia `config.yaml`**, para hablar como el modelo. Por ejemplo, `rules: [code, instructions,
  variables]`, `max_age: 720h` y `scope: environment | shared`.
- `vexpipeline.yaml` pasa a `schema_version: 3`. El motor nuevo **rechaza las versiones anteriores**
  diciendo qué cambió, sin leer las dos.
- Los 15 `config.yaml` de las tres plantillas se actualizan en RD-04.

**La alternativa que cambiaría el resultado.** Leer los pipelines de hoy sin tocarlos, con el ACL de
Definición traduciendo `pipeline` como «instrucciones y variables juntas». No hay que reescribir nada, pero
la regla del modelo nunca se podría escribir entera. Una tercera vía, un formato nuevo desde cero, cuesta más
y no resuelve nada que no resuelva la propuesta.

**Respuesta.** Sí, la propuesta: «hace falta `config.yaml`».

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-12.1 — Las fichas usan el formato heredado, sin las secciones de justificación *(por defecto)*

**Decisión.** Una cabecera con contexto, dependencias y criterio del frente 2, y después §1 Problema · §2 Por
qué importa · §3 Objetivo · §4 Alternativas · §5 Solución · §6 Alcance · §7 Verificación · §8 Decisiones · §9
Hallazgos al implementar.

**Por qué.** Es el formato que fija el plan para E12. Se retiran las subsecciones de justificación DDD, SOLID y
patrones, porque en este proceso el DDD es el cuerpo del documento, no algo que se justifica al final.

### DEC-12.2 — Las doce unidades de `migracion.md`, con su grafo *(por defecto)*

**Decisión.** De `RD-01` a `RD-12`, con las dependencias del `README.md`.

**Por qué.** El orden ya se decidió en `DEC-11.4`. Cada flecha va de un número menor a uno mayor, así que no
hay ciclos.

### DEC-12.3 — En inglés, solo lo que impone Go *(por defecto)*

**Decisión.** Los identificadores van en español, sin tildes. En inglés, solo los métodos de las interfaces de
la biblioteca estándar (`Error()`, `String()`) y los prefijos de las pruebas (`Test…`).

**Por qué.** `DEC-02.2` pide español hasta el borde, y el lenguaje impone esas pocas excepciones.

### DEC-12.4 — RD-12 depende de algo que está fuera del plan, y se dice *(por defecto)*

**Decisión.** RD-12 declara como dependencia que el CLI y el portal estén adaptados.

**Por qué.** Técnicamente se puede implementar sola, pero no debe empezarse antes: borraría el motor que usan
el CLI y el portal (`DEC-11.6`). Escribirlo en la cabecera evita que la condición se olvide.

### DEC-12.5 — La salida de los comandos se entrega en vivo, tal cual, y no se guarda *(Q-12.1)*

**Decisión.**
- Lo que imprimen los comandos se entrega **en vivo a quien pidió el intento**, sin tapar nada.
- **No se guarda** en el Historial ni forma parte de ningún registro.
- *Intentar* y *hacer rollback* reciben, en lo que publican, **a dónde entregar la salida**. El borde la
  conecta con la salida de la invocación.
- Resolución sigue extrayendo las variables de salida de lo que produjo cada comando (`DEC-08.3`).

**Por qué.** Quien despliega necesita ver qué está pasando. Tapar valores sería una regla de seguridad que
el producto no asume (`DEC-08.8`). La promesa de no exponer valores es sobre lo que **se guarda y se
publica**, y eso no cambia: registros, consulta del historial y diagnóstico.

### DEC-12.6 — Los ficheros del pipeline: la estructura de hoy, con `config.yaml` en el lenguaje del modelo *(Q-12.2)*

**Decisión.**
- Se mantienen la estructura y las claves de hoy.
- `config.yaml` declara:
  - `scope: environment | shared`, el ámbito de las variables del paso;
  - `rules`, con `code`, `instructions` y `variables`, cada una por separado;
  - `max_age`, la regla del tiempo.
- `vexpipeline.yaml` pasa a `schema_version: 3`, y el motor nuevo rechaza las versiones anteriores
  diciendo qué cambió.
- Las tres plantillas se actualizan en RD-04.

**Por qué.** La regla del modelo mira por separado código, instrucciones y variables (`DEC-06.12`), y
`state_changed: [pipeline]` no lo puede decir. Todo lo demás del formato ya habla como el modelo.

### DEC-12.7 — Lo que el modelo no decía del formato *(por defecto)*

**Decisión.**
- **Identidad y orden.** Un paso se identifica por su nombre sin `NN-`, y `NN` es solo su orden
  (`DEC-03.7`). Renumerar un paso no le hace perder su historia.
- **Variables de un paso compartido.** Se declaran **una vez**, en `variables/<paso>.yaml`. Las de un paso
  de su ambiente, en `variables/<ambiente>/<paso>.yaml`. Un fichero es un fichero y un ambiente es un
  directorio, así que ningún nombre de ambiente queda reservado.
- **Interpolación.** Solo se interpola el material listado en `templates`, y el resto se copia tal cual. La
  comprobación de variables mira los comandos y esos ficheros.
- **Valores por defecto.** Sin `rules`, un paso mira las tres cosas. Sin `max_age`, no caduca. Sin `scope`,
  su ámbito es su ambiente.
- **Lo que sale del formato.** `clone_window` desaparece: la fuente de hoy es la de hoy.

**Por qué.**
- Son las respuestas que ya estaban en el modelo o en tus palabras de IT-06.
- Interpolar todo el material rompería los ficheros de la tecnología que usan `${…}` propio.
- La ventana de clonado era una optimización del motor antiguo, y el modelo no la tiene.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `docs/rediseno/README.md` y `RD-01` a `RD-12` | **Nacen**, vigentes |
| `modelo/contextos/ejecucion.md` | La salida de los comandos, y el puerto **salida** (`DEC-12.5`) |
| `modelo/contextos/definicion.md` | Nueva sección «Los ficheros del pipeline» (`DEC-12.6`, `DEC-12.7`) |
| `RD-04`, `RD-06`, `RD-10` | Ya no queda nada pendiente (`DEC-12.5` a `DEC-12.7`) |
| `plan-ddd.md` | Tablero, y el plan queda **cerrado** |

## 6. Dudas diferidas

**Ninguna.** Fuera del plan queda adaptar el CLI y el portal al lenguaje publicado nuevo, que es la
condición para empezar RD-12 (`DEC-11.6`).

---

## Cierre

| Criterio | Cumple |
|---|---|
| Cada unidad tiene problema, objetivo, alcance, verificación y dependencias | sí: RD-01 a RD-12, sin ningún *pendiente* |
| El grafo no tiene ciclos | sí: cada flecha va de un número menor a uno mayor |
| Cada unidad se puede implementar por sí sola | sí: cada una depende solo de unidades anteriores. RD-12 depende además de algo que está fuera del plan, y lo declara (`DEC-12.4`) |
| Frente 2 de `DEC-01.10`: la siguiente propuesta no cumple ninguno de los cuatro criterios | sí: más allá de RD-12 no hay ninguna unidad que baje acoplamiento, fuerce una regla, simplifique un test o se pueda revertir tocando un solo contexto que no esté ya en una ficha |
