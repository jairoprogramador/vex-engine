# Huella de variables — regla `vars-v1`

> **Estado:** congelada · **Prefijo:** `vars-v1:` · **Implementación de
> referencia:** `variables.go` (la regla) + `command.VolatileVarNames` (el
> filtro de §3.1) · **Vectores:** §5

Este documento es **normativo**. Responde a «¿cambiaron las variables con las
que se ejecuta este paso?» y es una de las tres huellas que componen la
`cache_key` (spec 10 §5.1).

Si el código y este documento discrepan, **discrepan los dos**. Lo que no se
puede hacer es cambiar la regla y dejar el prefijo en `vars-v1:`.

---

## 1. Vocabulario

| Término | Significado |
|---|---|
| **variable** | un par nombre/valor del mapa acumulado, con su marca de ámbito compartido |
| **mapa acumulado** | el conjunto de variables visible para un paso en el momento de decidir si se ejecuta |
| **volátil** | variable que el motor **deriva** de la ejecución en curso, no declarada por nadie |
| **entrada** | la cadena con la que una variable participa en la huella |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`), y `H(x)` el SHA-256 de `x` en hexadecimal minúsculo.

## 2. De dónde sale el material

El mapa acumulado en el momento en que el paso decide si se ejecuta: las
variables iniciales del motor, las cargadas del almacén (ámbitos `shared` y del
paso) y las declaradas por el pipelinecode en `variables/<ambiente>/<paso>.yaml`,
en ese orden de precedencia —lo declarado gana sobre lo almacenado—.

> **Límite conocido y vivo, heredado por esta regla.** El mapa incluye también
> las variables que **el propio paso produjo** en una ejecución anterior y que se
> guardaron en el almacén. Como se cargan *antes* de decidir, la huella de la
> segunda corrida no coincide con la que se guardó en la primera, y todo paso con
> `outputs` se re-ejecuta exactamente una vez de más antes de alcanzar un punto
> fijo (spec 10 §1(d)). **No es un defecto de esta regla**: la regla hashea lo
> que se le da. Lo corrige la spec 14, al distinguir lo que un paso consume de lo
> que produce; cuando eso ocurra, el material de entrada cambia y esta regla
> seguirá siendo la misma.

## 3. La regla

### 3.1 Qué participa — el filtro de volátiles

**No participan** las variables que el motor deriva de la ejecución en curso.
La lista es cerrada y normativa:

| Nombre | Por qué es volátil |
|---|---|
| `project_version` | se recalcula en cada ejecución |
| `project_revision` | el hash de HEAD |
| `project_revision_full` | ídem |
| `tool_name` | lo pone el motor |
| `project_workdir` | ruta absoluta: depende de la máquina |
| `step_workdir` | ídem, y además cambia por paso |

Sin este filtro **ningún paso se saltaría jamás**: las seis cambian solas.
Y las tres últimas romperían además la comparabilidad entre máquinas, que es la
razón de ser de la `cache_key` compartida (spec 16).

El filtro se aplica **antes** de la regla, en `step.NewCacheMaterial`, con la
lista que declara `command.VolatileVarNames`. La partición es deliberada: los
nombres volátiles son vocabulario del modelo de ejecución, no de la regla de
huella. Lo que esta especificación exige es que la lista esté **escrita**, no
dónde vive; un test la fija contra esta tabla.

Todo lo demás participa, incluidas `environment`, `project_name`,
`project_organization`, `project_team`, `project_id` y `shared_workdir`.

> Que `environment` participe **no es lo que aísla los ambientes**. El
> aislamiento lo da el campo `Scope` de la `cache_key`, que lleva el ambiente por
> derecho propio (spec 10 §5.1). Hasta la spec 10 dependía de esta coincidencia,
> y bastaba con que alguien añadiera `environment` a la lista de arriba —una
> decisión que parece razonable— para que desplegar a `sand` hiciera que `prod` se
> saltara. Sacar `environment` de este material **no** debe hacer que un ambiente
> acierte con la entrada de otro; hay un test de regresión que lo fija.

### 3.2 La entrada de una variable

Tres campos unidos con **U+001E** (RS):

```
Q(nombre) ␞ Q(valor) ␞ <compartida>
```

- `<compartida>` es `true` o `false`, sin comillas. Cambiar **sólo** el ámbito
  de una variable cambia la huella: dónde se guarda un valor es parte de lo que
  el paso consume.
- `Q("")` es `""`, no ausencia: **«declarada y vacía» y «no declarada» son
  estados distintos** y la huella los separa (spec 03 §5.3).

Como `Q` escapa todo carácter de control, ni el separador ni el salto de línea
pueden aparecer dentro de un campo. La regla es inyectiva y no hereda la
ambigüedad teórica del separador de la huella del árbol (`SPEC-v1.md` §6).

### 3.3 El orden

Las entradas se ordenan **ascendentemente por comparación byte a byte de la
entrada completa**, no del nombre.

Un conjunto de variables no tiene secuencia: el orden en que llegan depende del
recorrido de un mapa, que en Go es deliberadamente aleatorio. Ordenar es lo que
hace la huella reproducible. Se ordena por la entrada completa —y no por el
nombre— por la misma disciplina que la huella del árbol (`SPEC-v1.md` §3.3) y
para que el orden sea total aunque el llamador entregue dos variables con el
mismo nombre, cosa que un mapa no puede pero un slice sí.

Es la diferencia deliberada con la huella de instrucciones, que **no** ordena:
allí el orden es la secuencia de ejecución.

### 3.4 La composición

Las entradas ordenadas se unen con `\n` (U+000A) **sin salto final**, y la
huella es `H(...)`.

Un conjunto vacío produce la huella de la cadena vacía:
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

La representación externa es:

```
vars-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo, y el token es
distinto del de las otras dos reglas a propósito.

## 4. Lo que la regla NO mira

- **El origen de una variable.** Que un valor venga del almacén, del
  pipelinecode o de un `outputs` anterior no cambia nada: lo que importa es el
  valor con el que se va a ejecutar. Quién lo puso es material de la spec 14.
- **El ambiente como tal.** Ver el recuadro de §3.1: eso es campo de la clave.

## 5. Vectores

Están ejecutados en `variables_test.go`.

Convención de la columna «variables»: `nombre=valor` es una variable no
compartida, `nombre=valor*` una compartida.

| # | Variables | Huella |
|---|---|---|
| 1 | *(conjunto vacío)* | `vars-v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| 2 | `region=us-east-1` | `vars-v1:7c7617e3218e094a48668b8d8ffa2d0993bb5c479a02a2c739b6e1a37213a5f0` |
| 3 | `region=us-east-1`, `replicas=3`, `bucket=artefactos*` | `vars-v1:a4941beb499496b2b29565fe8d0860acd7f9a286a4db2e5aa1e1661d58b1fced` |
| 4 | el 3 con `bucket` **no** compartida | `vars-v1:dbadfd75e41583fa7cb333ca7f81809f6e2b2db30d517e4431a518711baa1ed8` |
| 5 | el 3 más `instance_count=` *(declarada y vacía)* | `vars-v1:22e49da2bbbb22b13b3b5b8c763dad14a310b9b9da761dc46c71e6de4fed4f41` |

El 3 y el 4 se diferencian **sólo** en la marca de ámbito, y son distintos: es
§3.2. El vector 3 debe salir igual entregando sus tres variables en cualquier
orden: es §3.3.

## 6. Cómo validar una implementación independiente

1. Reproducir los vectores de §5.
2. Comprobar que el orden de entrega **no** cambia la huella.
3. Comprobar las sensibilidades: cambiar un valor, cambiar sólo la marca de
   ámbito, añadir una variable o añadir una variable declarada y vacía cambian
   la huella.
4. Comprobar que ninguna de las seis volátiles de §3.1 la cambia, y que
   `environment` **sí** la cambia.

## 7. Historia

| Versión | Cambio |
|---|---|
| `vars-v1` | Primera regla congelada (spec 10). Respecto de lo que el motor calculaba antes: la huella lleva prefijo de versión y token propio, el orden pasa a ser por entrada completa en vez de por nombre —mismo resultado, orden total—, el filtro de volátiles deja de estar duplicado en dos archivos y el material queda escrito |
