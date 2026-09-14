# IT-11 — Estrategia de migración

> Etapa: E11 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `modelo/arquitectura.md` y `modelo/context-map.md`
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **1 incongruencia · 6 decisiones.** **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Según el plan** (E11): cómo se llega de los paquetes de hoy a los del modelo, con permiso para romper el
motor actual. Qué invariantes se fijan en pruebas, en qué orden se migra, qué se estrangula y qué se
reescribe, y qué se hace con las `SPEC-*.md`. **Entregable**: `modelo/migracion.md`, con la reconciliación
con las specs `00`–`28`.

**Según el experto**, al abrir la iteración:

> *«Iniciemos con un nuevo código, no hace falta basarse en código actual; se puede descartar el código
> anterior. Para analizar lógica lo podemos dejar como idea general, pero podemos crear un nuevo paquete
> con el nuevo código, y luego al final eliminar el código antiguo. No importa si el código antiguo se
> rompe.»*

**Lo que se encontró al abrir.** Las specs `00`–`28` que el plan sitúa en `/Vex/specs/` **no están** en el
espacio de trabajo, ni en disco ni en el historial de git de ningún repositorio (solo aparece un
`specs/03_plan.md`). `ecosistema/specs/` contiene otra cosa: los planes M0–M7 del despliegue remoto.

---

## 2. Propuesta

El entregable está en `modelo/migracion.md`. Lo que lo sostiene:

1. **Reescribir, no estrangular**, porque lo pide el experto y porque el modelo cambia la forma entera del
   motor: las cadenas caen, el objeto compartido desaparece y el lenguaje pasa al español.
2. **El código nuevo nace en su sitio definitivo**, junto al antiguo y sin tocarlo, y la regla de
   dependencias detecta desde el primer día cualquier importación de código antiguo.
3. **Se prueba el modelo, no el código de hoy**: la regla de dependencias, las invariantes por construcción y
   los escenarios de cada contexto.
4. **Orden por dependencias**, con un primer corte que funciona de punta a punta en cuanto existe Ejecución.
5. **Reconciliación honesta**: como las specs no están, se derogan en bloque, y lo que valía ya está en el
   modelo.

---

## 3. Incongruencias

### Q-11.1 — El CLI y el portal, mientras se reescribe y cuando se borra el motor antiguo · **resuelta**

**La incongruencia.** El motor nuevo habla un lenguaje publicado nuevo, en español (`DEC-04.4`, `DEC-05.6`).
El CLI y el portal viven en otros repositorios, están en uso, y hablan el contrato de hoy. Este plan solo
alcanza a `vex-engine`, y ya se decidió que son ellos quienes se adaptan (`DEC-02.2`). Si al final se borra
el código antiguo y el motor nuevo lo sustituye de golpe, el CLI y el portal dejan de funcionar hasta que se
adapten.

**Propuesta.**

- **Mientras se reescribe, el CLI y el portal siguen usando el motor antiguo.** No se mantiene, pero
  tampoco se toca.
- **El cambio es un solo paso, al final.** Cuando el motor nuevo cubra lo que usan el CLI y el portal
  (intentar, hacer rollback y consultar el historial), se adaptan los dos a su lenguaje publicado, se cambia
  la imagen que lleva el motor, y en ese mismo paso se borra el código antiguo.
- **Adaptar el CLI y el portal queda fuera de este plan**, pero es la condición para borrar el código
  antiguo.

**La alternativa que cambiaría el resultado.** Que el borde del motor nuevo entienda también el contrato de
hoy, con una traducción hacia fuera, para que el CLI y el portal no tengan que cambiar. No habría corte,
pero el motor cargaría con un segundo contrato, en inglés y contra `DEC-02.2`, sin fecha para retirarlo.

**Respuesta.** *«Sí, eso se actualiza luego de que tengamos el motor terminado, ahora no.»* Se adopta
(`DEC-11.6`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-11.1 — Se reescribe desde cero; el código de hoy es solo idea general *(pedida por el experto)*

**Decisión.** El motor nuevo se escribe en paquetes nuevos, siguiendo `arquitectura.md` y `modelo/contextos/`.
El código de hoy no se adapta ni se estrangula por partes, y no se mantiene mientras tanto. Al final se
borra.

**Por qué.** Lo pide el experto. Y el modelo cambia la forma entera del motor: los contextos sustituyen a
las cadenas, el objeto de contexto compartido desaparece y el lenguaje del código pasa al español
(`DEC-02.2`). Estrangular algo que no va a sobrevivir en ninguna de sus piezas solo añade un camino
intermedio.

**Qué descarta.** Estrangular el código de hoy por partes.

### DEC-11.2 — El código nuevo nace en su sitio definitivo, junto al antiguo *(por defecto)*

**Decisión.** Los contextos en `internal/<contexto>/`, el borde en `internal/borde/` y la raíz de
composición en `cmd/motor/` mientras exista `cmd/vexd/`. Mientras convivan los dos códigos, la regla de
dependencias se aplica solo a los paquetes nuevos.

**Por qué.** Ninguna ruta nueva choca con las de hoy, salvo `cmd/vexd/`. Y como nada se escribe en un sitio
provisional, al terminar no hay que mover nada más que la raíz.

**Verificación.** Ningún paquete nuevo importa código antiguo; el comando de `migracion.md` lo comprueba.

### DEC-11.3 — Se prueba el modelo nuevo *(por defecto)*

**Decisión.** La regla de dependencias desde el primer paquete; las invariantes por construcción, con
pruebas unitarias del dominio; los escenarios de cada contexto, con pruebas de sus servicios de aplicación
sobre adaptadores en memoria; y un intento de punta a punta desde el primer corte que funcione.

**Por qué.** El arnés de hoy prueba un cableado que desaparece. Lo que tiene que quedar a salvo son las
invariantes y los escenarios del modelo, y ya están escritos.

### DEC-11.4 — Orden por dependencias, con un primer corte de punta a punta *(por defecto)*

**Decisión.** Las doce unidades de `migracion.md`: esqueleto, Historial, Suministro, Definición, Resolución,
Ejecución con un borde mínimo (primer corte que funciona), Lanzamiento, Diagnóstico, Simulación, borde
completo, lo que se compra y el cambio.

**Por qué.** Cada contexto se construye contra lo que tiene arriba, que ya existe. Diagnóstico puede empezar
en paralelo en cuanto exista el Historial, usando historiales de prueba.

### DEC-11.5 — Las specs `00`–`28` quedan derogadas en bloque *(por defecto)*

**Decisión.** Todas quedan derogadas como normativa. Las cinco `SPEC-*.md` y los documentos del rediseño en
curso se borran con el código antiguo, y la sección del motor en `CLAUDE.md` se reescribe al final.

**Por qué.** No están en el espacio de trabajo, así que no se puede hacer una cuenta spec por spec. Lo que
valía de ellas ya se volvió a decidir en `modelo/` con el libro, y el porqué está en `iteraciones/`.

**Verificación.** Ningún documento de `modelo/` cita una spec numerada como fuente de una regla.

### DEC-11.6 — El CLI y el portal cambian al final, en un solo paso *(respuesta a `Q-11.1`)*

**Decisión.** Mientras se escribe el motor nuevo, el CLI y el portal siguen usando el antiguo, que no se
toca. Cuando el motor nuevo esté terminado, en un solo paso (la unidad 12): se adaptan el CLI y el portal a
su lenguaje publicado, se cambia la imagen que lleva el motor, `cmd/motor/` pasa a `cmd/vexd/`, se borra el
código antiguo y se reescribe la sección del motor en `CLAUDE.md`.

**Por qué.** No hay corte mientras dura la reescritura, y el motor no carga con un segundo contrato.

**Consecuencias.** Adaptar el CLI y el portal queda fuera de este plan, y es condición para borrar el código
antiguo.

**Qué descarta.** Que el motor nuevo entienda también el contrato de hoy.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/migracion.md` | **Nace**, y pasa a **vigente** al cerrar; la unidad 12 queda definida (`DEC-11.6`) |
| `plan-ddd.md` | Tablero |

## 6. Dudas diferidas

| # | Duda | A | Por qué |
|---|---|---|---|
| 1 | Adaptar el CLI y el portal al lenguaje publicado del motor nuevo | **fuera de este plan** | Son otros repositorios. Es condición de la unidad 12 (`DEC-11.6`) |
