# IT-10 — Genéricos, Simulación y Lanzamiento

> Etapa: E10 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §1 (genéricos: *¿se puede no escribir?*) y §10 (agregados)
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **3 incongruencias · 8 decisiones.** Con ella se cierra el bloque táctico. **Archivo cerrado: no se vuelve a
> editar.**

---

## 1. Qué se quiere

**Según el plan** (E10): por cada genérico, decidir si se aísla tras un ACL, se sustituye por algo que se
compra o se deja como está; y, para Simulación, si comparte modelo con Ejecución o se amolda a él.
**Cierre común del bloque B** para cada contexto que se modela aquí.

**La vara del frente 2 de `DEC-01.10`**: una propuesta de diseño entra solo si cumple al menos una de
estas cuatro:

- baja acoplamiento medible;
- fuerza una regla por construcción;
- simplifica un test existente;
- se puede revertir tocando un solo contexto.

**Qué entra aquí, respecto de lo que dice el plan:**

| Lo que nombra el plan | Hoy, en el modelo |
|---|---|
| Suministro del Proyecto · Suministro del Pipeline | **Suministro de Fuentes**, un solo contexto (`DEC-01.7`) |
| Sincronización de Estado | **Sincronización del Historial**, que se realiza **dentro** del Historial (`DEC-03.11`) |
| Espacio de Trabajo | No es subdominio: es memoria de Ejecución, por ambiente (`DEC-06.17`, `DEC-06.18`, `DEC-09.5`) |
| Simulación | Contexto propio; con Ejecución, **Separate Ways** (`DEC-04.6`) |
| *(no lo nombra)* | **Lanzamiento**, que nació después del plan y no tenía etapa (`DEC-10.1`) |

**Dudas que vencen aquí:**

| Origen | Duda | Dónde queda |
|---|---|---|
| IT-01 §9 **#5** | El espacio de trabajo por ambiente y con continuidad | **Resuelta** en IT-06 (`DEC-06.17`, `DEC-06.18`) |
| IT-01 §9 **#8** | Qué se compra y qué se escribe: acceso a repositorios, hash de un árbol, almacén | `DEC-10.2`, `DEC-10.3` |
| IT-01 §9 **#12** · IT-02 §9 **#6** | Las estrategias de lanzamiento, y cómo se obtiene el valor de la versión | Las estrategias siguen **fuera de alcance** (`dominio.md`). El valor de la versión, `DEC-10.8` |
| IT-07 §6 **#1** | Cómo garantiza el almacén que dos máquinas no ocupen el mismo ambiente a la vez | `DEC-10.3` |

---

## 2. Propuesta

Los modelos están en `modelo/contextos/suministro.md`, `simulacion.md` y `lanzamiento.md`. Lo que los
sostiene:

1. **Ante un genérico, la primera pregunta es si se puede no escribir** (`guia-ddd.md` §1). El acceso a
   repositorios y el almacén **se compran**, cada uno detrás de su ACL. Lo que se escribe es la traducción
   y lo mínimo del hash.
2. **Al almacén que se compre se le exige lo que el modelo necesita**, no más: que lo escrito se pueda leer
   en cualquier máquina antes de seguir, que no se sobrescriba y que admita una escritura condicional para
   la ocupación.
3. **Simulación es como Ejecución sin efectos**, pero con su propio agregado y sin compartir núcleo:
   recorre todo, en todos los ambientes, y fabrica salidas con la forma declarada.
4. **Lanzamiento decide y no guarda.** Sus registros son agregados del Historial; lo suyo es la regla del
   actor ausente y la factoría del lanzamiento.

---

## 3. Incongruencias

### Q-10.1 — *Antes de publicarlo*: ¿qué pipeline simula Simulación? · **resuelta**

**La incongruencia.** Simulación responde *«¿funcionará este pipeline antes de publicarlo?»*, y trae su
material de Suministro. Pero Suministro solo pone delante **el repositorio como está hoy o como estaba en
un commit**. Un pipeline que el DevOps todavía no ha publicado es, casi siempre, lo que tiene **en su copia
de trabajo**, con cambios que aún no están en ningún commit. Tal como está el modelo, Simulación solo podría
probar lo que ya está publicado, que es justo lo que ya no hace falta probar antes.

**Propuesta.** Suministro también sabe poner delante **una copia de trabajo**: el directorio en el que está
trabajando el DevOps, sin commit. Tiene hash, pero no tiene commit. **Simular** acepta una copia de trabajo
o un commit. Qué acepta *intentar* se decide aparte.

**La alternativa que cambiaría el resultado.** Que Simulación solo acepte commits: el DevOps sube sus cambios
a una rama y simula ese commit. Suministro no cambia, pero para probar cada cambio antes hay que hacer un
commit.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-10.6`).

---

### Q-10.2 — ¿Se puede intentar con una copia de trabajo? · **resuelta**

**La incongruencia.** Un despliegue es un **punto de retorno**: un sitio al que se puede volver. Volver
necesita el commit de cada fuente para traer el material de entonces (`DEC-03.9`), y una copia de trabajo no
tiene commit. Si un intento hecho con una copia de trabajo llegara a despliegue, sería un punto de retorno al
que no se puede volver, y *«el despliegue no admite grados»* dejaría de ser verdad. Pero prohibirlo del todo
obligaría a hacer un commit cada vez que un programador quiere probar un cambio en su ambiente de
desarrollo.

**Propuesta.** **Se puede intentar con una copia de trabajo, pero ese intento nunca llega a despliegue**,
aunque salga bien en todos los pasos: se queda como intento.

- Se registra, cuenta en la cantidad de intentos y el core puede diagnosticarlo, porque lo que falla es
  siempre un intento (`DEC-06.7`).
- No es un punto de retorno: no se puede volver a él, no provoca ningún lanzamiento y nunca sirve de
  referencia.
- El Historial lo sabe porque la apertura dice que no hay commit.

**La alternativa que cambiaría el resultado.** Que *intentar* solo acepte commits, de modo que todo intento
exitoso de todos los pasos sea un despliegue. El modelo no gana ningún caso nuevo, pero para probar cualquier
cambio hay que hacer antes un commit.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-10.7`).

---

### Q-10.3 — ¿De dónde sale el valor de la versión? · **resuelta**

**La incongruencia.** Todo lanzamiento lleva una **versión**, la etiqueta técnica, y *«el valor lo pone la
herramienta»* (`lenguaje.md`). Si el dueño del negocio no pone nombre, el nombre toma ese valor. Pero el
modelo nunca dijo **cómo** se obtiene: `dominio.md` lo dejó fuera de alcance, y sin eso Lanzamiento no puede
crear ningún lanzamiento. Y hay una trampa: un lanzamiento ocurre en **cualquier ambiente** (`DEC-03.17`),
así que el mismo código se lanza en dev, en staging y en producción. Si cada lanzamiento sacara un número
nuevo, el mismo producto tendría tres versiones distintas.

**Propuesta.** La versión es **un número por proyecto que crece de uno en uno cada vez que se lanza un código
que nunca se había lanzado**, en cualquier ambiente. El mismo código (el mismo hash del código) conserva su
versión en todos los ambientes, así que lo que llega a producción se llama igual que en staging. Se obtiene
recorriendo los lanzamientos del Historial, sin guardar ningún contador, y la escritura condicional del
almacén (`DEC-10.3`) evita que dos lanzamientos a la vez se lleven el mismo número.

**La alternativa que cambiaría el resultado.** Que la versión venga en la petición de lanzar, escrita por
quien lanza o leída de algún sitio del producto. La herramienta no inventaría nada, pero la etiqueta técnica
dejaría de ser suya, y un lanzamiento automático en nombre del actor ausente no tendría de dónde sacarla.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-10.8`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-10.1 — Lanzamiento entra en E10 *(por defecto)*

**Por qué.** Es un contexto supporting con modelo propio que nació en IT-01, después de escribirse el plan.
Ninguna etapa del bloque B lo tenía, y E10 es la última que modela contextos.

### DEC-10.2 — El acceso a los repositorios se compra; el hash es lo mínimo *(por defecto)*

**Decisión.** Traer el material de hoy o de un commit, y saber con qué commit se trabaja, lo da algo que se
compra, detrás del ACL de Suministro. Del hash se escribe lo mínimo, y si algo que se compre cumple que
cambie cuando cambia el contenido, se usa.

**Por qué.** Es un genérico, y lo primero que se pregunta ante uno es si se puede no escribir. Cierra la
duda **#8** de IT-01 en lo que toca a Suministro.

### DEC-10.3 — El almacén del Historial se compra, y tiene que dar tres cosas *(por defecto)*

**Decisión.** El almacén se compra, detrás del ACL del Historial. Tiene que garantizar:

1. que lo escrito se pueda leer desde cualquier máquina **antes de seguir** (`DEC-05.9`, `DEC-06.18`);
2. que un registro **no se sobrescriba nunca** (`DEC-05.5`);
3. una **escritura condicional**, que solo tenga éxito si el ambiente está libre, para la ocupación
   (`DEC-07.8`).

Un almacén que no dé la tercera no sirve.

**Por qué.** Son las garantías de las que depende el modelo, ni una más. Cierra la duda **#1** de IT-07 y la
**#8** de IT-01 en lo que toca al almacén.

### DEC-10.4 — Simulación: un agregado que vive durante la invocación, en todos los ambientes *(por defecto)*

**Decisión.** Las invariantes y los escenarios de `simulacion.md`: sin efectos, recorre todos los pasos de
**cada** ambiente, los comandos simulados terminan bien, las salidas simuladas cumplen su expresión regular,
interpola en un espacio temporal y el informe no muestra valores.

**Por qué.** *¿Funcionará este pipeline?* incluye las variables de cada ambiente. Y simular un fallo de un
comando sería inventar un resultado que nadie declaró.

### DEC-10.5 — Lanzamiento decide; sus registros son del Historial *(por defecto)*

**Decisión.** Sin agregados propios. Su dominio es el servicio *decidir si se lanza en nombre del actor
ausente* y la factoría de un lanzamiento. Las estrategias de lanzamiento siguen fuera de alcance.

**Por qué.** `DEC-07.2` puso *Lanzamiento* y *Reserva* como agregados del Historial, que protege su forma. Lo
que queda para este contexto son sus reglas.

### DEC-10.6 — Suministro también trae una copia de trabajo, y Simulación la acepta *(respuesta a `Q-10.1`)*

**Decisión.** Una fuente se puede traer como está hoy, como estaba en un commit, o como está en una **copia de
trabajo**: el directorio donde alguien está trabajando, con cambios sin commit. Una copia de trabajo tiene
hash, pero no tiene commit. Simular acepta una copia de trabajo o un commit.

**Por qué.** Lo que el DevOps quiere probar antes de publicar está en su copia de trabajo.

**Consecuencias.** *Copia de trabajo* entra en el lenguaje de Suministro. Qué acepta *intentar* queda para
`Q-10.2`.

**Qué descarta.** Que Simulación solo acepte commits.

### DEC-10.7 — Se puede intentar con una copia de trabajo, pero ese intento nunca llega a despliegue *(respuesta a `Q-10.2`)*

**Decisión.** *Intentar* acepta el material de hoy, de un commit o de una copia de trabajo. Un intento hecho
con una copia de trabajo se registra, cuenta en la cantidad de intentos y se puede diagnosticar, pero
**nunca llega a despliegue**, aunque salga bien en todos los pasos. El Historial lo sabe porque la apertura
dice que no hay commit.

**Por qué.** Un despliegue es un punto de retorno, y volver necesita un commit. Y probar un cambio en
desarrollo no debería exigir hacer un commit antes.

**Consecuencias.** El agregado Despliegue gana una invariante: su intento se hizo con commits. Un intento con
copia de trabajo no provoca lanzamientos y nunca sirve de referencia.

**Qué descarta.** Que *intentar* solo acepte commits.

**Verificación.** Ningún despliegue tiene un intento cuya apertura diga que no hay commit.

### DEC-10.8 — La versión es un número por proyecto que crece con cada código nuevo que se lanza *(respuesta a `Q-10.3`)*

**Decisión.** La versión de un lanzamiento es un número por proyecto que crece de uno en uno cada vez que se
lanza un código que nunca se había lanzado, en cualquier ambiente. El mismo hash del código conserva su
versión en todos los ambientes. Se obtiene recorriendo los lanzamientos del Historial, sin guardar ningún
contador, y la escritura condicional del almacén evita que dos lanzamientos simultáneos tomen el mismo
número.

**Por qué.** Un lanzamiento ocurre en cualquier ambiente, y el mismo producto tiene que llamarse igual en
todos. Además, el lanzamiento automático necesita que la herramienta pueda poner la versión sin ayuda.

**Consecuencias.** Cierra la duda **#12** de IT-01 y la **#6** de IT-02 en lo que toca a la versión; las
estrategias de lanzamiento siguen fuera de alcance. `dominio.md` deja de tener *cómo se obtiene la etiqueta
técnica* entre lo que está fuera de alcance.

**Qué descarta.** Que la versión venga en la petición de lanzar.

**Verificación.** Dos lanzamientos con el mismo hash del código tienen la misma versión.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/contextos/suministro.md` · `simulacion.md` · `lanzamiento.md` | **Nacen**, provisionales |
| `modelo/contextos/historial.md` | Lo que tiene que dar el almacén (`DEC-10.3`) |
| `modelo/contextos/suministro.md` · `simulacion.md` · `lenguaje.md` | La copia de trabajo (`DEC-10.6`) |
| `modelo/contextos/historial.md` · `ejecucion.md` · `suministro.md` · `lenguaje.md` · `dominio.md` | Un intento con copia de trabajo nunca llega a despliegue (`DEC-10.7`) |
| `modelo/contextos/lanzamiento.md` · `lenguaje.md` · `dominio.md` | Cómo se obtiene la versión (`DEC-10.8`) |
| `modelo/contextos/suministro.md` · `simulacion.md` · `lanzamiento.md` | Pasan a **vigentes** |
| `plan-ddd.md` | Tablero |

## 6. Dudas diferidas

*Ninguna.*
