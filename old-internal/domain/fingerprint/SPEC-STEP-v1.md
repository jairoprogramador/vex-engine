# Huella de un step — regla `sf-v1`

> **Estado:** congelada · **Prefijo:** `sf-v1:` · **Implementación de
> referencia:** `step.go`, en este mismo directorio · **Vectores:** §5

Este documento es **normativo**, con la misma disciplina que `SPEC-v1.md`.
Responde a la pregunta que decide si un step vuelve a ejecutarse:

> **¿Es el mismo trabajo que la última vez?**

Si el código y este documento discrepan, **discrepan los dos**. Lo que no se
puede hacer es cambiar la regla y dejar el prefijo en `sf-v1:`.

---

## 1. Por qué existe esta regla

Sustituye a `ck-v1` (`cache.CacheKey`), que la spec 27 retira. La diferencia no
es de cálculo: es de **material**.

`ck-v1` hasheaba la DIRECCIÓN —`Subject`, `Pipeline`, `Scope` y `Step`— junto con
el contenido. Mientras hubo una sola tienda direccionada por contenido eso era
coherente; con la clave de posición de la spec 11 era la misma información dos
veces, y tenía dos consecuencias:

- el mismo trabajo calculado sobre el mismo material producía identidades
  distintas según **dónde** se guardara, así que «¿este contenido ya corrió en
  algún sitio?» —la pregunta del índice (spec 11 §5.5) y de `vex plan`— no tenía
  respuesta;
- un step con `scope: project` desplegado a `sand` y luego a `prod` escribía y
  leía en la misma clave —eso lo entregó la spec 13— pero **no revivía**, porque
  su huella llevaba el ambiente dentro. El ámbito de proyecto no servía para
  nada.

Aquí la dirección sale del hash entera, y las dos preguntas vuelven a ser dos:

```
clave  (subject, scope, step_id)  →  ¿DÓNDE se recuerda este trabajo?
huella  sf-v1:…                    →  ¿ES el mismo trabajo?
```

## 2. Vocabulario

| Término | Significado |
|---|---|
| **declaración** | la huella `pipe-v1` de lo que el step declara hacer (`SPEC-PIPELINE-v1.md`) |
| **proyecto** | la huella `v1` del árbol del código a desplegar (`SPEC-v1.md`) |
| **término condicional** | el sumando del proyecto, que entra sólo si el step lo declara |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`), y `H(x)` el SHA-256 de `x` en hexadecimal minúsculo.

## 3. La regla

El material son **tres líneas**, unidas por `\n` (U+000A) **sin salto final**:

```
vex-step-fingerprint/sf-v1
Q(declaración)
Q(proyecto)
```

y la huella es `H(material)`.

### 3.1 Los términos entran por su forma canónica COMPLETA

`Q("pipe-v1:8f3a…")`, no `Q("8f3a…")`. Nunca el hash pelado.

De ahí sale que subir `pipe-v1` a `v2` invalide **todas** las `sf-v1` emitidas
sin una línea de código extra, que es el OCP que la spec 08 §5.2' prometía y la
propiedad de la que cuelga toda la disciplina de versiones de este motor. Se
conserva sólo mientras la composición sea sobre `String()`.

Por lo mismo, **esta regla NO comprueba la versión de sus términos**. Hacerlo
obligaría a tocarla para aceptar un `pipe-v2`, y la propiedad se perdería en el
momento en que hiciera falta.

### 3.2 El término del proyecto es CONDICIONAL

`Q(proyecto)` entra **si y sólo si el step lo declara** en su `config.yaml`, con
`state_changed: [pipeline, project]` o con la forma corta `- state_changed`, que
equivale a las dos fuentes (spec 15 §5.2).

Cuando no lo declara, la línea es `Q("")`. Es inyectivo: ninguna forma canónica
de una huella real es la cadena vacía, así que una huella con proyecto no puede
coincidir con una sin él.

La razón del término es el caso que motiva el ámbito de proyecto: un step que
crea un registro de contenedores **no depende en absoluto del código de la
aplicación**. Si el código entrara siempre, ese step se re-ejecutaría en cada
commit, y eso vacía de sentido compartir el recurso entre ambientes. Es la
granularidad que la spec 10 §5.3 sacrificó al meter las tres huellas en la clave
de todos los steps, recuperada **declarada** en vez de cableada por nombre de
step.

**Ausencia del término no es ausencia de material.** `pipe-v1` está siempre; lo
condicional es el segundo sumando.

### 3.3 La exclusión va DECLARADA, y en negativo

La implementación toma un booleano `projectExcluded`, no un `includesProject`, y
exige que el hueco esté declarado por los dos lados: excluir el proyecto y a la
vez traerlo también es un error.

- **En negativo** porque su valor cero tiene que ser el SEGURO. Olvidar
  `projectExcluded` cuesta una re-ejecución de más; olvidar un `includesProject`
  costaría un step que deja de re-ejecutarse ante un cambio de código, que es la
  peor omisión que este motor puede cometer.
- **Por los dos lados** porque, sin esa mitad, la forma canónica de un mismo step
  dependería de si alguien se acordó de limpiar el campo.

### 3.4 Material incompleto ⇒ error, nunca huella degradada

Una huella con un hueco es válida y **colisiona** con la de cualquier material al
que le falte lo mismo, y esa colisión se manifiesta como un step que revive sin
haberse ejecutado jamás — el peor fallo posible del motor.

Aguas arriba, «no se pudo componer la huella» significa **ejecutar el step y
escribir su registro SIN huella**. Un registro sin huella no revive nunca
(`state.StepRecord.Revives`), así que el fail-open de la spec 09 §5.3 —ante la
duda, ejecutar— se conserva por construcción y no por código. Es la asimetría de
la spec 11 §4: guardar de más cuesta espacio, guardar de menos cuesta un recurso
duplicado.

### 3.5 La representación externa

```
sf-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo, y hace la
sustitución de `ck-v1` gratis: un registro escrito con `ck-v1:` **no puede**
revivir contra una huella `sf-v1:`, porque las cadenas difieren en el prefijo. No
se escribe migrador. La primera ejecución tras desplegar re-ejecuta todo y
repuebla, que es el efecto correcto de cambiar la regla de identidad.

El prefijo hace además que los hechos ya emitidos queden **distinguibles**, no
ambiguos: `step_finished.step_fingerprint` viaja en el registro (spec 19) y se
empuja al destino (spec 21), y un consumidor puede contar las dos épocas por
separado en vez de mezclarlas.

## 4. Lo que la regla NO mira

- **Nada de la dirección**: ni el sujeto, ni el `step_id`, ni la ruta del ámbito,
  ni la url del pipelinecode. Ver `SPEC-PIPELINE-v1.md` §4.
- **El tiempo.** La expiración es una regla sobre la EDAD del registro
  (`max_age`, spec 15), no una propiedad del contenido. Un material que llevara
  el instante no sería comparable consigo mismo.
- **`deployment_id`, `content_id` ni `parent`.** Responden otras preguntas, y
  `deployment_id` además cambia en cada ejecución nueva al mismo ambiente aunque
  el step no haya cambiado nada. Que no estén en el material es lo que permite
  que la decisión de saltar un step sea independiente del registro de despliegue.

## 5. Vectores

Están ejecutados en `step_test.go`. Se dan con huellas de término FIJAS —no
calculadas— para que la regla se pueda reproducir sin implementar las otras dos:

```
D  = pipe-v1:1111111111111111111111111111111111111111111111111111111111111111
D' = pipe-v1:2222222222222222222222222222222222222222222222222222222222222222
P  = v1:3333333333333333333333333333333333333333333333333333333333333333
V2 = pipe-v2:1111111111111111111111111111111111111111111111111111111111111111
```

| # | Declaración | Proyecto | Huella |
|---|---|---|---|
| 1 | `D` | *(excluido)* | `sf-v1:73925e33d40bb78cfef488ff4f4ddd2a7700cbc483202e4549fccd36bcbe82f7` |
| 2 | `D` | `P` | `sf-v1:d32eb1c4e9aa19d36e059bb0207e98bd5f6e860659054072f5110db6b11a1ea6` |
| 3 | `D'` | *(excluido)* | `sf-v1:ddcf8fe6824402dd789df9c350e2a8db71a78c2380343744ff2e5e65894537b9` |
| 4 | `V2` | *(excluido)* | `sf-v1:dae5ca9ee06cc0e9251bc11abc97b68669615d7da904c36a6295eda37abf4499` |

El 1 y el 2 fijan §3.2: el mismo trabajo con y sin el término del proyecto son
huellas distintas. El 1 y el 4 fijan §3.1: un salto de versión de `pipe-v1`
cambia la `sf-v1` **sin tocar el código de esta regla**.

## 6. Cómo validar una implementación independiente

1. Reproducir los vectores de §5.
2. Comprobar que cambiar la declaración cambia la huella.
3. Comprobar que añadir o quitar el término del proyecto cambia la huella.
4. Comprobar que cambiar SÓLO el token de versión de un término cambia la
   huella.
5. Comprobar los tres errores: declaración ausente; proyecto ausente sin
   declararlo excluido; proyecto presente declarándolo excluido.

## 7. Historia

| Versión | Cambio |
|---|---|
| `sf-v1` | Primera regla congelada (spec 27). Sustituye `ck-v1`: salen del material las cuatro dimensiones de dirección, entra `pipe-v1` en lugar de `inst-v1` + `vars-v1`, y el término del proyecto pasa a ser condicional por diseño en vez de por una ampliación del dominio de `cache.Material` |
