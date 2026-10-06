# IT-05 — Arquitectura objetivo

> Etapa: E5 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §4
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **2 incongruencias · 10 decisiones.** Con ella se cierra el bloque estratégico.
> **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Según el plan** (E5): decidir la arquitectura donde vive el modelo. **Cierre**: existe una regla de
dependencias que se puede comprobar mecánicamente, con el comando escrito aunque todavía no se
ejecute.

**Las preguntas que el plan le hace a E5, y dónde quedan:**

| Pregunta del plan | Dónde queda |
|---|---|
| ¿La secuencia de hechos del Historial es su modelo o su mecanismo de persistencia? | `DEC-05.5` |
| ¿CQRS explícito para leer el historial? | `DEC-05.5` |
| ¿Un módulo por contexto, o por capa dentro de cada contexto? | `DEC-05.1` |
| ¿Cómo se hablan los contextos dentro de un mismo proceso? | `DEC-05.3` · `DEC-05.4` |
| ¿Qué papel le queda al objeto de contexto compartido que hoy recorre las cadenas? | **No es material de E5**: es código (`DEC-02.10`), y su destino se decide al reconciliar lo construido, en E11 |

**Lo que arrastra de antes:**

| Origen | Duda | Dónde queda |
|---|---|---|
| IT-03 §9 **#3** · IT-04 §6 **#2** | ¿Los registros del Historial son su modelo o su persistencia? | `DEC-05.5` |
| IT-04 §6 **#1** | Cómo viaja un evento de dominio dentro del proceso | `DEC-05.4` |
| IT-04 §6 **#3** | La forma concreta del lenguaje publicado del motor | `DEC-05.6` |

---

## 2. Propuesta

La arquitectura completa está en `modelo/arquitectura.md`, y la regla, en
`modelo/reglas-dependencias.awk`. Lo que la sostiene, contrastado con el libro:

1. **Hexagonal dentro de cada contexto.** El libro no impone arquitectura: pide que el dominio no
   dependa de nada de fuera, y las capas `dominio → aplicación → infraestructura` lo garantizan
   contexto a contexto.
2. **Los módulos se nombran por el dominio, no por la técnica.** El libro desaconseja agrupar por
   tipo técnico. Por eso el primer nivel son los **contextos**, y las capas quedan dentro de cada
   uno. Hoy es al revés: capas arriba y conceptos mezclados debajo.
3. **El context map se vuelve importaciones.** Cada patrón tiene un sitio: Customer–Supplier en lo
   que publica el de arriba, ACL en la infraestructura del de abajo, Conformist en su aplicación. La
   regla de dependencias es la tabla de «quién está arriba» del mapa, comprobable con un comando.
4. **Un proceso que empieza y acaba no necesita consistencia eventual.** Los contextos se llaman de
   forma directa y síncrona. Un bus en memoria sería complejidad sin nada a cambio: no hay nada que
   desacoplar en el tiempo.
5. **El evento avisa; el registro es la verdad.** Si el proceso muere entre escribir un despliegue y
   avisar a Lanzamiento, el aviso se pierde pero el hecho no. Lanzamiento se pone al día la próxima
   vez que escucha.
6. **Los registros son el modelo del Historial.** `lenguaje.md` ya lo decía: *un registro es un hecho
   que quedó escrito; consultar es recorrer; la cantidad de intentos se cuenta, no se guarda*. No hace
   falta un modelo de lectura aparte mientras recorrer baste.

---

## 3. Incongruencias

### Q-05.1 — Un intento cuyo proceso muere antes de terminar · **resuelta**

**La incongruencia.** Para que lo que un paso ya hizo en el mundo no se pierda, la arquitectura
escribe cada registro **en cuanto ocurre el hecho**. Si un paso creó recursos en la nube, el siguiente
intento tiene que saberlo para no re-ejecutarlo. Pero el Historial dice que un intento es *un hecho
ya ocurrido*, con estado **exitoso, fallido o cancelado**. Si el proceso muere a mitad (la máquina se
apaga, se queda sin memoria), nadie queda vivo para escribir cómo terminó. En el historial queda un
intento con pasos registrados y sin estado. Y como el motor no recuerda nada, tampoco puede
distinguir *murió* de *sigue corriendo en otra máquina*.

**Propuesta.** El estado de un intento **se deriva de sus registros**, y el modelo nombra el caso que
falta: **sin desenlace**, cuando hay registros del intento y ninguno que diga cómo terminó.

- **No es un cuarto estado**: es la ausencia del registro que da el estado, y eso es un hecho, no una
  lectura. El historial no dice por qué falta, porque no lo sabe.
- **El core no le atribuye causa**: no apunta al código, ni a las instrucciones, ni a las variables.
- ***No se re-ejecuta* sigue funcionando**, porque los pasos que sí terminaron tienen su registro.
- **Cuenta** en la cantidad de intentos, porque es un intento.

**La alternativa que cambiaría el resultado.** Escribir todo al final del intento, de una vez. Nunca
habría un intento a medias en el historial. Pero si el proceso muere, lo que los pasos hicieron en el
mundo no queda en ninguna parte, y el siguiente intento lo rehace, con riesgo de duplicar recursos.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-05.8`).

---

### Q-05.2 — ¿Cuándo se lleva un registro al almacén, y qué pasa si no se puede? · **resuelta**

**La incongruencia.** `DEC-05.8` escribe cada registro en cuanto ocurre el hecho, para que lo que
hizo un paso no se pierda. Pero el motor no recuerda nada (*cada intento empieza de cero y muere al
terminar*) y puede correr en otra máquina, así que lo que solo está escrito en esa máquina muere con
ella. Si los registros se llevan al almacén al final del intento y la máquina muere antes, lo que los
pasos hicieron en el mundo vuelve a perderse, que es justo lo que `DEC-05.8` quería evitar. Y falta la
otra mitad: qué hacer si el almacén no responde.

**Propuesta.** Cada registro **se lleva al almacén en cuanto se escribe**: para los demás, un hecho no
existe hasta que está llevado. **Si no se puede llevar, el intento se detiene antes del siguiente
paso**, porque no debe hacer en el mundo nada que no pueda quedar escrito. En el historial compartido,
ese intento queda *sin desenlace* (`DEC-05.8`), que es exactamente lo que se sabe de él.

**La alternativa que cambiaría el resultado.** Seguir aunque el almacén falle, y llevar todo al final
o cuando vuelva a responder. El pipeline nunca se detiene por el almacén, pero si la máquina muere
antes de llevar los registros, lo hecho se pierde y el siguiente intento puede duplicar recursos.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-05.9`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`). Las demás responden a una incongruencia.

### DEC-05.1 — Un paquete de primer nivel por contexto, con las capas dentro, en un solo módulo *(por defecto)*

**Decisión.** `internal/<contexto>/`, con `dominio/`, `aplicacion/`, `infraestructura/` y `publicado/`
dentro, más `internal/borde/` hacia fuera y `cmd/vexd/` como raíz de composición. Un solo módulo de
Go.

**Por qué.** El libro pide nombrar los módulos por el dominio. Un módulo de Go por contexto obligaría
a versionar cada frontera sin darle a ninguna una traducción que las capas no den ya.

**Qué descarta.** Las capas arriba y los contextos debajo; un módulo por contexto.

### DEC-05.2 — La regla de dependencias se comprueba con un comando *(por defecto)*

**Decisión.** Las seis reglas de `arquitectura.md`, comprobadas con `go list` y
`modelo/reglas-dependencias.awk`. La tabla de contextos de arriba del script es la del context map.

**Por qué.** Es la condición de cierre de E5 en el plan.

**Verificación.** El comando existe y nombra cada importación que rompe la regla. Se ejecutará cuando
exista el árbol objetivo (E11).

### DEC-05.3 — Los contextos se hablan con llamadas directas y síncronas *(por defecto)*

**Decisión.** Un contexto llama a lo que publica el de arriba, en el mismo proceso. No hay bus de
mensajes.

**Por qué.** El motor es de un solo uso: cada invocación empieza y acaba. No hay nada que desacoplar
en el tiempo, y un bus solo añadiría indirección.

**Qué descarta.** Un bus en memoria; consistencia eventual entre contextos.

### DEC-05.4 — Un evento se entrega en el mismo proceso, después de escribir su registro *(por defecto)*

**Decisión.** El Historial escribe el registro del despliegue y a continuación avisa a quien escucha,
registrado en la raíz de composición. Si el proceso muere entre las dos cosas, el aviso se pierde y
el hecho no. **Cada vez que escucha, Lanzamiento se pone al día**: lanza el último despliegue del
ambiente si no está lanzado y el ambiente no está reservado.

**Pérdida aceptada.** Si el aviso se pierde y no hay otro despliegue en ese ambiente, el último queda
sin lanzar hasta que alguien lo lance o llegue el siguiente despliegue.

**Qué descarta.** Una bandeja de salida persistente para los eventos, que sería una segunda memoria
junto al Historial.

### DEC-05.5 — Los registros son el modelo del Historial; leer es recorrerlos *(por defecto)*

**Decisión.** Los registros son hechos que se añaden y no se cambian, y **son** el modelo. Lo
derivado se calcula recorriéndolos. Guardar y sincronizar es infraestructura. No hay CQRS explícito:
consultar es recorrer, y si hiciera falta velocidad se añadiría un índice derivable que no decide
nada.

**Por qué.** Es lo que el lenguaje ya dice del Historial. Un modelo de lectura aparte solo se
justifica cuando leer el modelo deforma el modelo, y aquí leer es recorrer hechos.

**Cierra.** IT-03 §9 **#3** e IT-04 §6 **#2**.

### DEC-05.6 — Una invocación del motor atiende una operación del lenguaje publicado, que declara su versión *(por defecto)*

**Decisión.** El borde recibe **una** petición por invocación. Cada petición dice qué operación pide
y en qué versión del lenguaje publicado está escrita, y el motor rechaza las versiones que no
soporta. Operaciones: intentar, hacer rollback, simular, lanzar, reservar o liberar un ambiente,
preguntar la causa y consultar el historial.

**Por qué.** Es la forma de un Open Host para un proceso de un solo uso. Declarar la versión en cada
petición hace imposible que un cambio de contrato pase sin que nadie lo note.

**Cierra.** IT-04 §6 **#3**.

### DEC-05.7 — Los identificadores son los términos del lenguaje, sin tildes ni eñes *(por defecto)*

**Decisión.** `Resolución de Variables` es `resolucion`; `evidencia`, `evidencia`.

**Por qué.** Las rutas de paquete y las herramientas trabajan en ASCII, y quitar las tildes no cambia
qué término es.

**Consecuencia.** La verificación de `DEC-02.2` se lee así: *un término de `lenguaje.md` se busca en el
código y aparece con ese nombre, sin tildes*.

### DEC-05.8 — Un intento sin registro de cierre queda sin desenlace *(respuesta a `Q-05.1`)*

**Decisión.** Cada registro se escribe en cuanto ocurre el hecho. El estado de un intento se deduce
de sus registros. Si un intento tiene registros y ninguno dice cómo terminó, está **sin desenlace**.
No es un cuarto estado: es la ausencia del registro que da el estado, y eso es un hecho. El historial
no dice por qué falta, porque no lo sabe.

**Consecuencias.** El core no atribuye causa a un intento sin desenlace. Los pasos que sí terminaron
conservan su registro, así que *no se re-ejecuta* sigue valiendo para ellos. El intento cuenta en la
cantidad de intentos. `DEC-02.3` no se enmienda: los estados siguen siendo tres.

**Qué descarta.** Escribir todo al final del intento.

**Verificación.** Ningún escenario de E6 atribuye causa a un intento sin desenlace.

### DEC-05.9 — Cada registro se lleva al almacén en cuanto se escribe; si no se puede, el intento se detiene *(respuesta a `Q-05.2`)*

**Decisión.** Para los demás, un hecho no existe hasta que está llevado. Cada registro se lleva al
almacén en cuanto se escribe. Si no se puede llevar, el intento se detiene antes del siguiente paso
y, en el historial compartido, queda sin desenlace (`DEC-05.8`).

**Por qué.** El proceso muere al terminar y puede correr en otra máquina: lo que solo está escrito ahí
muere con ella. No se debe hacer en el mundo nada que no pueda quedar escrito.

**Consecuencias.**

- El aviso *despliegue registrado* se entrega **después** de llevar el registro, para que Lanzamiento
  no lance un despliegue que el historial compartido no conoce. Esto precisa `DEC-05.4`.
- Toda invocación **trae** el historial antes de leerlo.
- Un problema del almacén detiene el intento. Es el precio de no duplicar recursos.

**Qué descarta.** Seguir con el almacén caído y llevar los registros al final, o cuando vuelva a
responder.

**Verificación.** Ningún paso empieza mientras quede un registro anterior sin llevar.

### DEC-05.10 — Lo que la infraestructura guarda para ir más rápido no es memoria *(por defecto)*

**Decisión.** Entre invocaciones se pueden guardar una copia local de un repositorio o un índice, pero
hay que poder borrarlos sin que cambie ninguna decisión. La única memoria del motor es el Historial.

**Por qué.** *Nada vive entre invocaciones* es una regla sobre lo que el motor sabe, no sobre lo que la
infraestructura acelera. Es el mismo criterio que `lenguaje.md` aplica al índice.

**Verificación.** Borrar esas copias no cambia el resultado de ninguna operación.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/arquitectura.md` | **Nace**, provisional mientras IT-05 esté abierta |
| `modelo/reglas-dependencias.awk` | **Nace**: el comando de la regla de dependencias |
| `plan-ddd.md` | Tablero |
| `modelo/arquitectura.md` · `lenguaje.md` · `dominio.md` · `bounded-contexts.md` | *Sin desenlace* (`DEC-05.8`) |
| `modelo/arquitectura.md` · `lenguaje.md` · `dominio.md` | Llevar cada registro en cuanto se escribe, y detener el intento si no se puede (`DEC-05.9`) |
| `modelo/arquitectura.md` | Lo que se guarda para ir más rápido no es memoria (`DEC-05.10`). Pasa a **vigente** |

## 6. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Qué pasa con los registros que quedaron escritos en la máquina y no llegaron al almacén porque el intento se detuvo (`DEC-05.9`): si se descartan o se intentan llevar después | **IT-07** | Es táctico del Historial |
| 2 | El destino de lo que hoy existe en código (el objeto de contexto compartido, las tres cadenas) frente a este árbol | **IT-09** · **IT-11** | Las cadenas son de E9, y la reconciliación es de E11 |
