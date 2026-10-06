# IT-08 — Definición de Pipeline y Resolución de Variables

> Etapa: E8 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §10 (agregados), §11 (factorías) y §12 (repositorios)
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **2 incongruencias · 8 decisiones.** **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Cierre común del bloque B**, para **cada uno** de los dos contextos: agregados con sus invariantes (o por
qué no los tiene), entidades y value objects, eventos, servicios de dominio, repositorios y factorías,
servicios de aplicación y puertos.

**La vara del frente 2 de `DEC-01.10`**: una propuesta de diseño entra solo si cumple al menos una de
estas cuatro:

- baja acoplamiento medible;
- fuerza una regla por construcción;
- simplifica un test existente;
- se puede revertir tocando un solo contexto.

**Las preguntas que el plan le hace a E8.** Están escritas sobre el código, y el modelo ya las responde:

| Pregunta del plan | Respuesta del modelo |
|---|---|
| ¿La precedencia entre orígenes de una variable es de Resolución o de Ejecución? | **De Resolución**: *precedencia* es término suyo (`lenguaje.md`), y la regla está en `DEC-08.4` |
| ¿Las reglas de re-ejecución son de Definición o de quien ejecuta? | **Las dos cosas**: la forma se escribe en Definición y el significado lo aplica Ejecución (`DEC-03.6`, `DEC-06.12`) |
| ¿De quién es el cálculo de los hashes, que hoy no tiene dueño? | *Hash* es una palabra común con un dueño por cada caso: Suministro (código y pipeline), Ejecución (instrucciones de un paso) y Resolución (variable). Qué entra en cada uno es técnica (`DEC-02.18`, `DEC-03.16`) |

**Dudas que vencen aquí:**

| Origen | Duda | Dónde queda |
|---|---|---|
| IT-01 §9 **#3** | Si la re-ejecución compara la declaración de las variables o su valor resuelto | **Resuelta por el modelo**: el eje variables compara el hash del valor de las declaradas (`DEC-06.10`), y *¿cambiaron?* lo responde Resolución con hashes de valores (`DEC-03.15`) |
| IT-01 §9 **#4** | Cómo se ofusca el valor: qué se conserva para comparar y dónde vive lo que hace falta para compararlo | `DEC-08.8` |
| IT-06 §6 **#2** | Que el hash de variable sea comparable entre ambientes de un mismo proyecto | `DEC-08.6` |
| IT-06 §6 **#3** | Qué entra en el hash de las instrucciones de un paso | `DEC-08.7` |

---

## 2. Propuesta

Los modelos están en `modelo/contextos/definicion.md` y en `modelo/contextos/resolucion.md`. Lo que los
sostiene:

1. **Definición no cambia nada, como Diagnóstico.** El pipeline lo escribe el DevOps en su repositorio, no el
   motor. Por eso su agregado, **Pipeline**, es inmutable y lo que protege es que no exista uno mal formado.
   La comprobación es su factoría (`guia-ddd.md` §11).
2. **El repositorio de Definición se apoya en otro contexto.** Los pipelines se obtienen de Suministro, a
   través del ACL que convierte un repositorio en una declaración.
3. **Resolución sí tiene estado, pero solo durante una invocación.** Las variables efectivas de un intento
   cambian a medida que los pasos producen valores. Ese es su agregado, y lo que tiene que quedar se escribe
   en el Historial.
4. **Las invariantes que protegen el valor salen por construcción**: el valor solo sale hacia el comando y,
   ofuscado, hacia el Historial; y una petición sin intento no deja nada.

---

## 3. Incongruencias

### Q-08.1 — Las instrucciones de un paso, ¿son solo sus comandos? · **resuelta**

**La incongruencia.** El lenguaje dice que las instrucciones son *el conjunto de comandos*, y `DEC-06.9`
compara el eje instrucciones con *el hash de las instrucciones de cada paso, que solo cubre sus
comandos*. Pero un paso no es solo comandos. El DevOps también escribe en el directorio del paso el
material que esos comandos usan: plantillas, manifiestos, ficheros de terraform. Si ese material no entra:

- cambiar una plantilla **no re-ejecuta** el paso, aunque su regla mire las instrucciones;
- el core dirá *«las instrucciones no cambiaron»* cuando sí cambió lo que el paso hace, y mandará a buscar
  la causa en el código o en las variables.

**Propuesta.** Las **instrucciones de un paso** son **sus comandos y el material de su directorio**: todo
lo que el DevOps escribe para ese paso, **salvo** sus variables, que son el eje variables, y su
configuración (la regla y el ámbito), que dice *cuándo* se re-ejecuta y *qué ve*, no *qué hace*. Ese
material tampoco varía por ambiente, porque las variables se interpolan, así que cumple la premisa del
core. El hash de las instrucciones de un paso cubre las dos cosas.

**La alternativa que cambiaría el resultado.** Dejar fuera el material del directorio. El hash de las
instrucciones sería más estable, pero un cambio en una plantilla sería invisible tanto para la
re-ejecución como para el diagnóstico.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-08.7`).

---

### Q-08.2 — Un hash de variable que no se pueda revertir probando valores · **disuelta por el experto**

**La incongruencia.** `DEC-08.6` hace que el mismo valor dé el mismo hash en todos los ambientes de un
proyecto. Pero muchos valores son cortos o previsibles, como `3`, `true` o `prod`, y un hash sin nada más
se revierte probando: basta con calcular el hash de `3` y compararlo. Con esos valores, *«sin que el valor
circule»* sería falso. Para que no se pueda probar, el hash tiene que llevar una **clave del proyecto** que
solo conozca Resolución. Y esa clave tiene que ser **la misma en cualquier máquina y no cambiar nunca**:
si cambiara, todos los hashes anteriores dejarían de ser comparables sin que nada avisara.

**Propuesta.** La **clave de los hashes de un proyecto** se guarda en el Historial, por la relación
reservada, y solo la lee Resolución. Se crea la primera vez y **nunca se cambia ni se reemplaza**. Si ya
existe y no se puede leer, Resolución no calcula ningún hash y el intento no empieza, porque un hash con
otra clave diría *«cambió»* sobre todo.

**La alternativa que cambiaría el resultado.** Que la clave la dé en cada petición quien pide el intento,
por ejemplo el portal. El motor no la guardaría, pero si dos clientes usaran claves distintas, los hashes
dejarían de ser comparables sin que nada lo avisara.

**Respuesta.** *«Solo realiza el hash sin clave. No es un tema de seguridad, ya que gestionar secretos es
otro producto. Aquí solamente hacemos lo mínimo, que es trabajar con un hash simple. No garantiza seguridad
al 100%, y eso está bien, porque no es el objetivo de este producto.»* (`DEC-08.8`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-08.1 — Las preguntas del plan para E8 ya están respondidas por el modelo *(por defecto)*

**Decisión.** La tabla de §1.

### DEC-08.2 — El Pipeline es un agregado inmutable que nace comprobado *(por defecto)*

**Decisión.** La comprobación es la factoría del Pipeline: o devuelve un pipeline válido, o devuelve la
lista de fallos. El repositorio de pipelines obtiene el de hoy o el de un commit a través del ACL hacia
Suministro.

**Por qué.** El motor nunca modifica un pipeline. Lo único que hay que proteger es que no exista uno mal
formado, y eso lo garantiza quien lo crea (`guia-ddd.md` §11).

**Verificación.** No hay forma de obtener un Pipeline que no haya pasado la comprobación.

### DEC-08.3 — Las variables de un intento son un agregado que vive durante la invocación *(por defecto)*

**Decisión.** Las variables efectivas de cada paso de un intento forman un agregado, con las invariantes
de `resolucion.md`. No tiene repositorio propio: lo que se conserva va al Historial.

**Por qué.** Sus reglas (ámbito, orden, precedencia, que el valor no circule) cambian a medida que los pasos
producen valores, y tienen que cumplirse en todo momento. Y nada vive entre invocaciones fuera del
Historial y del espacio de trabajo (`DEC-06.18`).

### DEC-08.4 — La variable producida gana a la declarada *(por defecto)*

**Decisión.** Si dos orígenes dan valor al mismo nombre, gana la variable producida. Entre producidas,
gana la del paso más reciente.

**Por qué.** Lo declarado es un valor escrito de antemano, y lo producido es un hecho de esta ejecución.
Un valor declarado no puede tapar lo que un paso acaba de producir.

### DEC-08.5 — Una variable de ámbito compartido se declara una vez para todos los ambientes *(por defecto)*

**Por qué.** Si se declarara por ambiente, dejaría de ser compartida.

### DEC-08.6 — El hash de variable se puede comparar entre ambientes de un mismo proyecto *(por defecto)*

**Decisión.** El mismo valor da el mismo hash en todos los ambientes de un proyecto.

**Por qué.** ES-2 (staging contra producción) lo necesita (`DEC-06.6`). Cierra la duda **#2** de IT-06.

**Consecuencia aceptada.** Si una variable tiene el mismo valor en staging y en producción, se sabe que es
igual, aunque no se sepa cuál es.

### DEC-08.7 — Las instrucciones de un paso son sus comandos y el material de su directorio *(respuesta a `Q-08.1`)*

**Decisión.** Las instrucciones de un paso son todo lo que el DevOps escribe para ese paso, salvo sus
variables y su configuración (la regla y el ámbito). El hash de las instrucciones de un paso cubre los
comandos y el material.

**Por qué.** Cambiar una plantilla cambia lo que hace el paso. Si no contara, el paso no se re-ejecutaría
y el core buscaría la causa en otro eje. Y el material no varía por ambiente, así que cumple la premisa.

**Consecuencias.** Precisa `DEC-06.9` y la definición de *instrucciones* en `lenguaje.md`. Cierra la duda
**#3** de IT-06.

**Verificación.** Un cambio solo en una plantilla de un paso marca el eje instrucciones de ese paso.

### DEC-08.8 — El hash de variable es simple, sin clave *(respuesta a `Q-08.2`, que el experto disolvió)*

**Decisión.** El hash de variable es un hash simple, sin clave. No hay ningún secreto que guardar.

**Por qué.** La seguridad no es el objetivo de este producto, y gestionar secretos es otro producto. Vex
hace lo mínimo que necesita para saber si una variable cambió.

**Consecuencias.** **Pérdida aceptada**: un valor corto o previsible se puede adivinar desde su hash.
*«Sin que el valor circule»* significa que el valor no circula en claro, no que sea imposible deducirlo.
Cierra la duda **#4** de IT-01 y la **#4** de IT-03: lo que se conserva para comparar es un hash simple,
ofuscar el valor en reposo es infraestructura del Historial, y que solo Resolución lo pida de vuelta ya lo
garantiza la relación reservada (`DEC-04.7`).

**Qué descarta.** Una clave por proyecto guardada en el Historial, y una clave que mande cada cliente.

**Verificación.** Ningún documento de `modelo/` presenta el hash de variable como una garantía de
seguridad.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/contextos/definicion.md` | **Nace**, provisional |
| `modelo/contextos/resolucion.md` | **Nace**, provisional |
| `plan-ddd.md` | Tablero |
| `modelo/lenguaje.md` · `contextos/definicion.md` | Las instrucciones de un paso incluyen el material de su directorio (`DEC-08.7`) |
| `modelo/lenguaje.md` · `dominio.md` · `contextos/resolucion.md` | El hash de variable es simple, sin clave (`DEC-08.8`) |
| `modelo/contextos/definicion.md` · `contextos/resolucion.md` | Pasan a **vigentes** |

## 6. Dudas diferidas

*Ninguna.*
