# Clave de caché — regla `ck-v1`

> **Estado:** congelada · **Prefijo:** `ck-v1:` · **Implementación de
> referencia:** `cache_key.go`, en este mismo directorio · **Vectores:** §5

Este documento es **normativo**. La `cache_key` responde a **«¿esto ya se
ejecutó *aquí*?»** — nótese que no es «¿son iguales las entradas?»: un paso tiene
efectos sobre un ambiente real, y que dos ambientes tengan entradas idénticas no
significa que ejecutar en uno haya dejado algo hecho en el otro.

Desde la spec 16 la clave se comparte entre máquinas, y desde la 17 el mismo
material alimenta un `content_id` **permanente**. Las dos cosas exigen que un
tercero pueda reimplementar la regla y obtener el mismo valor bit a bit.

---

## 1. Vocabulario

| Término | Significado |
|---|---|
| **material** | las siete dimensiones que determinan el resultado de ejecutar un paso |
| **clave** | el resumen del material |
| **entrada** | lo que el almacén guarda bajo una clave (§4) |
| **ámbito** | `shared` o el ambiente; ver §3.1 |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`), y `H(x)` el SHA-256 de `x` en hexadecimal minúsculo.

## 2. Las tres capas de versión

| Capa | Token | Documento |
|---|---|---|
| huella del árbol del proyecto | `v1:` | `fingerprint/SPEC-v1.md` |
| huella de las instrucciones | `inst-v1:` | `fingerprint/SPEC-INSTRUCTIONS-v1.md` |
| huella de las variables | `vars-v1:` | `fingerprint/SPEC-VARIABLES-v1.md` |
| composición de la clave | `ck-v1:` | **este** |

La clave se compone sobre las formas canónicas **completas** de las tres
huellas, con su prefijo, **no** sobre los hashes pelados. De ahí sale la
propiedad que hace barato el versionado: **un salto a `v2` de cualquiera de las
tres reglas invalida todas las claves emitidas sin una línea de código extra.**

Si esa composición se hiciera sobre el hash pelado, corregir una divergencia
congelada de la huella del árbol dejaría claves antiguas acertando con material
nuevo, que es indistinguible de un bug.

## 3. El material

### 3.1 Las siete dimensiones

| # | Campo | Qué es | Obligatorio |
|---|---|---|---|
| 1 | `subject` | url del proyecto | sí |
| 2 | `pipeline` | url del pipelinecode | sí |
| 3 | `scope` | `shared` o el ambiente | sí |
| 4 | `step` | nombre del paso, sin el prefijo `NN-` | sí |
| 5 | `instructions` | huella `inst-v1` de los comandos declarados | sí |
| 6 | `variables` | huella `vars-v1` de las variables del paso | sí |
| 7 | `code` | huella `v1` del árbol del proyecto | **condicional**, ver §3.1bis |

**Ninguno es anulable por omisión.** Un material incompleto no produce una clave
degradada: produce un **error**. La razón es que una clave compuesta con un hueco
es perfectamente válida y colisiona con la de cualquier otro material al que le
falte el mismo campo — y esa colisión se manifiesta como un paso que se salta sin
haberse ejecutado jamás.

Aguas arriba, «no se pudo componer la clave» significa **ejecutar el paso y no
escribir entrada**. Y como *ausencia de entrada ⇒ ejecutar*, la semántica «sin
evidencia ⇒ ejecutar» de la spec 05 §5.1 se conserva por construcción, sin
código que la defienda.

### 3.1bis `code` es condicional, y su ausencia se DECLARA (spec 15)

Desde la spec 15 un paso declara en su `config.yaml` qué invalida su trabajo, y
`state_changed: [pipeline]` dice que el código del proyecto **no forma parte de
su identidad** — el caso cotidiano es un paso que crea un registro de
contenedores, que no depende del código de la aplicación y que con «todo importa
siempre» se re-ejecutaba en cada commit.

Para ese material, `code` va **ausente y su ausencia va marcada**. Son dos
condiciones, y las dos se comprueban:

| Estado declarado | `code` | Veredicto |
|---|---|---|
| el paso vigila el proyecto | presente | válido |
| el paso vigila el proyecto | ausente | **error** — es un hueco, no una decisión |
| el paso NO lo vigila | ausente | válido; la línea 8 del material canónico es `Q("")` |
| el paso NO lo vigila | presente | **error** — la forma canónica sería ambigua |

**Esto no es una `ck-v2`, y el argumento es de inyectividad.** La regla de
composición no cambia ni un byte: siguen siendo las ocho líneas de §5.1 en el
mismo orden. Lo que cambia es el **dominio** de materiales aceptados, y se amplía
sin remapear nada: `Q("")` no era producible antes —un `code` vacío era un
error— y ninguna huella tiene la forma canónica vacía, así que

- **ninguna clave emitida cambia de valor** (los vectores de §6 se conservan
  literales), y
- **ninguna clave nueva puede coincidir con una vieja**.

La spec 27 sustituye `ck-v1` entera por `sf-v1`, donde el término del proyecto es
condicional por diseño (`sf-v1(pipe-v1 [, v1])`); esto es esa forma, expresada
con la regla que hay hoy.

### 3.2 `scope`, y por qué es un solo campo

`scope` es lo que hace que la clave responda «¿ya se ejecutó **aquí**?» en vez de
«¿son iguales las entradas?».

Hasta la spec 10, tres de las cuatro claves anteriores **no llevaban el
ambiente**. Que desplegar a `sand` no dejara escrito «sin cambios» para `prod`
dependía únicamente de que la variable `environment` estuviera en el mapa
acumulado y no figurara en la lista de exclusiones de la huella de variables.
Nadie lo escribió con esa intención, y bastaba una decisión que parece razonable
—«`environment` es volátil, fuera»— para que desplegar a sandbox hiciera que
producción se saltara, sin una sola señal.

Es **un** campo y no un par `{environment, scope}`: serían redundantes mientras
coinciden y ambiguos cuando difirieran. `shared` es palabra reservada como
ambiente (spec 04), así que un solo campo no puede colisionar. **Vale siempre el
ambiente**, también para un paso que declara `scope: project` (spec 13): la
dirección donde vive su registro es otra, pero su huella sigue llevando el
ambiente aquí. Sacarlo es de la spec 27, que retira de la huella las cuatro
dimensiones de DIRECCIÓN; está desde ya porque añadirlo después invalidaría
todas las claves emitidas.

### 3.3 Lo que NO entra

- **El tiempo.** La expiración no es propiedad del contenido, así que volver a
  ejecutar por caducidad produce **exactamente la misma clave**. Desde la
  spec 15 la ventana la declara el paso (`max_age` en su `config.yaml`) y se
  mide contra la edad de su último REGISTRO; hasta la 11 fue un `expires_at` en
  la entrada, y entre la 11 y la 15 un TTL global de 30 días en el motor. Las
  tres formas comparten esto: ninguna entra en el material.
- **La máquina.** Ni rutas absolutas, ni el usuario, ni el sistema operativo. Es
  lo que hace que el caché compartido de la spec 16 signifique algo. Las tres
  variables de ruta absoluta se excluyen en la huella de variables
  (`SPEC-VARIABLES-v1.md` §3.1).

  > **Defecto heredado, no resuelto aquí.** Si el proyecto es un *worktree* de
  > git o un submódulo, `.git` es un **archivo** cuyo contenido es
  > `gitdir: <ruta absoluta de la máquina>`, y la huella del árbol lo incluye
  > porque sólo excluye `.git` cuando es un directorio (`SPEC-v1.md` §6). En
  > local eso cuesta una re-ejecución de más; desde la spec 16, con el caché
  > compartido, el efecto es que **la entrada de otra máquina nunca acierta** y
  > el caché parece no funcionar sin que nada falle. Corregirlo cambia todas las
  > huellas: es una `v2` de la regla del árbol, y la decisión tiene que estar
  > tomada antes de la 16.
- **El nombre del paso como vocabulario cerrado.** La clave se compone del
  material, no de una lista de pasos conocidos. Un paso llamado `notify` produce
  su clave igual que uno llamado `deploy`.

### 3.4 El intento y el resultado

Una entrada existe **si y sólo si** un paso con ese material terminó bien. No hay
entrada de «se intentó», ni de «falló», ni de «se canceló». Un paso que empieza y
no termina —incluida la muerte dura: `SIGKILL`, un OOM, un corte de luz— no deja
nada, y la corrida siguiente lo vuelve a ejecutar.

Es la propiedad estructural que la spec 09 compró al mover la escritura al camino
de éxito, y esta especificación la hereda: no hay nada que compensar porque no se
escribe nada antes.

## 4. La entrada

```
Entry {
  state_key   la clave de posición del registro al que apunta
  record_id   cuál de sus registros
}
```

> **La entrada cambió de papel dos veces y esta sección lo dice tarde.** Con la
> spec 11 dejó de ser una afirmación —«esto ya se ejecutó»— y pasó a ser un
> ÍNDICE: `contenido → {clave de estado, registro}`. Perdió `expires_at` y
> `produced_by` con ello, y sobre todo perdió el voto: **el índice no participa
> en ninguna decisión**, y `rm -rf` sobre él no cambia nada de lo que el motor
> decide. Lo que sigue describe el papel que tuvo entre las specs 10 y 11, y se
> conserva porque explica por qué las dos tiendas están separadas.

**Las entradas eran de SOLA PRESENCIA.** No guardan resultado reutilizable:
guardan que este contenido exacto ya se ejecutó con éxito aquí, quién lo hizo y
hasta cuándo vale. El contenido reutilizable —el almacén de variables— vive
aparte, y sigue viviendo aparte a propósito: los dos tienen **reglas de borrado
opuestas**. El caché se puede borrar entero sin consecuencias; un ARN guardado
en el almacén es la pista de un recurso real y no se borra nunca. Fundirlos es
la spec 11, y merece serlo.

`produced_by` no es adorno: sin él, «se salta porque ya está en caché» con una
clave opaca deja sin respuesta «¿cuándo se probó esto por última vez?», que es
la afirmación de valor del motor.

El TTL por defecto era de **30 días**, y **ya no existe**: lo retiró la spec 15
§5.7, después de que la 11 lo moviera de sujeto —de la entrada al registro—. Un
paso sin `max_age` no caduca. Lo que se conserva de aquella regla es la frontera:
un registro caduca **cuando** se alcanza su instante, no después (borde
exclusivo).

## 5. La regla

### 5.1 El material canónico

Ocho líneas unidas con `\n` (U+000A), **sin salto final**:

```
vex-cache-key/ck-v1
Q(subject)
Q(pipeline)
Q(scope)
Q(step)
Q(instructions)          ← la forma canónica COMPLETA, "inst-v1:<hash>"
Q(variables)             ← "vars-v1:<hash>"
Q(code)                  ← "v1:<hash>", o "" si el paso no lo vigila (§3.1bis)
```

- La primera línea es una cabecera fija. Existe para que una clave no pueda
  coincidir con el hash de otra cosa que se le parezca.
- El orden de las siete líneas de material es **fijo**. Permutar dos campos es
  cambiar la regla.
- `Q` escapa todo carácter de control, incluido `\n`: **ningún valor puede
  simular un salto de línea** y hacerse pasar por otro reparto de campos. La
  regla es inyectiva, y no hereda la ambigüedad teórica del separador que la
  huella del árbol dejó congelada.

### 5.2 La composición y la forma externa

La clave es `H(material canónico)`, y su representación externa:

```
ck-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo. Dos claves de
versiones distintas **nunca** son iguales, aunque el hash coincida.

## 6. Vectores

Están ejecutados en `cache_key_test.go`. Las tres huellas del material base son
literales fijos —no calculados— para que este documento se pueda validar sin
reimplementar antes las otras tres reglas:

```
subject      = "https://vex.test/acme/demo-app"
pipeline     = "https://vex.test/acme/pipelinecode"
scope        = "sand"
step         = "supply"
instructions = "inst-v1:" + "11" repetido 32 veces
variables    = "vars-v1:" + "22" repetido 32 veces
code         = "v1:"      + "33" repetido 32 veces
```

| # | Material | Clave |
|---|---|---|
| 1 | el base | `ck-v1:6d12da1d013dd8ea8cefb1d095900471afba22828ec4cd3dcc2c972718545126` |
| 2 | el base con `scope = "prod"` | `ck-v1:a6ba010f5849846aa8f44bbff1acd1aacb501a131e89cd40698a52525f0ab241` |
| 3 | el base con `scope = "shared"` | distinta de la 1 y de la 2 |
| 4 | el base con `pipeline` de otro pipelinecode | distinta de la 1 |
| 5 | el base con `code` en `v2:` y el mismo hash | distinta de la 1 |

> Las claves 1 y 2 son las que fija el test como literales; las filas 3–5 se
> afirman por relación porque lo que demuestran es una **sensibilidad**, y un
> literal de 64 caracteres no la haría más clara. El test es la autoridad; si la
> tabla y el test discrepan, discrepan los dos.

## 7. Cómo validar una implementación independiente

1. Reproducir los vectores de §6.
2. Comprobar que cambiar **cualquiera** de las siete dimensiones cambia la
   clave. Son siete comprobaciones, no una.
3. Comprobar que un material al que le falte cualquiera de las siete produce un
   **error**, no una clave.
4. Comprobar que cambiar sólo el **prefijo de versión** de una huella, dejando
   su hash igual, cambia la clave (§2).
5. Comprobar que componer la misma clave dos veces da el mismo valor, y que
   componerla no escribe nada en ninguna parte.
6. Comprobar §3.1bis en sus cuatro filas: que un `code` ausente y **declarado
   ausente** produce clave, que esa clave difiere de la del mismo material con
   `code`, y que las dos combinaciones contradictorias producen error.

## 8. Historia

| Versión | Cambio |
|---|---|
| `ck-v1` | **Ampliación del dominio, no de la regla** (spec 15): `code` pasa a ser condicional y su ausencia se declara (§3.1bis). La composición de §5 no cambia, y la ampliación es inyectiva: ninguna clave emitida cambia de valor y ninguna nueva colisiona con una vieja. Por eso NO hay salto de versión |
| `ck-v1` | Primera regla (spec 10). Sustituye a cuatro claves distintas —`(proyecto, pipeline, paso)` para instrucciones y código, `(proyecto, pipeline, ambiente, paso)` para variables y `(proyecto, ambiente, paso)`, sin pipeline, para tiempo— por una sola que lleva las siete dimensiones. El caché arranca **frío**: ninguna entrada del esquema anterior se encuentra, y no se escribe migrador porque el caché es desechable por definición |
