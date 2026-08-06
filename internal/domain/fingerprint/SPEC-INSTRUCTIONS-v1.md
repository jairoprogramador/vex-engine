# Huella de instrucciones — regla `inst-v1`

> **Estado:** congelada · **Prefijo:** `inst-v1:` · **Implementación de
> referencia:** `instructions.go`, en este mismo directorio · **Vectores:** §6

Este documento es **normativo**, con la misma disciplina que `SPEC-v1.md` (la
huella del árbol). Responde a «¿cambiaron las instrucciones de este paso?» y es
una de las tres huellas que componen la `cache_key` (spec 10 §5.1).

Si el código y este documento discrepan, **discrepan los dos**: uno de los dos
tiene un bug y hay que decidir cuál. Lo que no se puede hacer es cambiar la
regla y dejar el prefijo en `inst-v1:`.

---

## 1. Por qué existe este documento

Hasta la spec 10 esta huella era un `string` calculado dentro de
`inst_pipeline_rule.go`, sin tipo, sin versión y sin una línea escrita de qué
entraba en ella. Que `show` no participara —y que por tanto añadir `show: true`
para depurar no invalidara el caché— **no era una decisión: era un olvido**, y
sólo se descubrió cuando la spec 00 escribió un test que lo caracterizaba.

La spec 08 congeló y especificó la huella del árbol y dejó estas dos sin hacer.
La 10 las paga porque `cache_key` se comparte entre máquinas desde la spec 16 y
alimenta el `content_id` permanente de la 17: un material sin especificar no se
puede comparar entre organizaciones ni congelar en una identidad.

## 2. Vocabulario

| Término | Significado |
|---|---|
| **comando** | una entrada de la lista de `steps/NN-<paso>/commands.yaml` |
| **instrucciones** | la lista completa de comandos de un paso, en el orden declarado |
| **entrada** | la cadena con la que un comando participa en la huella |
| **campo** | uno de los valores que componen una entrada |

Sea `Q(x)` la representación de `x` con la sintaxis de literal de cadena de Go
(`strconv.Quote`): comillas dobles alrededor, `\` y `"` escapados, y **todo
carácter de control escapado** — en particular `\n` (U+000A) y `\x1e` (U+001E).

Sea `H(x)` el SHA-256 de `x` en hexadecimal minúsculo (64 caracteres).

## 3. La regla

### 3.1 Qué participa

Participan **todos** los campos declarables de un comando en `commands.yaml`
que llegan al modelo de ejecución:

| Campo | Participa | Por qué |
|---|---|---|
| `name` | sí | identifica al comando en los logs y en el registro |
| `cmd` | sí | es lo que se ejecuta |
| `workdir` | sí | cambia dónde se ejecuta |
| `show` | **sí** | ver §3.5 |
| `templates` | sí, con su orden | qué archivos se interpolan y en qué orden |
| `outputs[].name` | sí | qué variable se produce |
| `outputs[].probe` | sí | de dónde se extrae su valor |
| `description` | **no** | ver §3.5 |

La regla general, enunciada por la spec 10 §5.1bis y que la spec 15 hereda:
**todo campo declarable en `commands.yaml` entra**. Excluir uno exige una
justificación escrita por campo — no se obtiene por olvido. §3.5 es la única
exclusión justificada de la `inst-v1`.

### 3.2 La entrada de un comando

Los campos se unen con **U+001E** (RS) en este orden exacto:

```
Q(name) ␞ Q(cmd) ␞ Q(workdir) ␞ <show> ␞ <n_templates> [␞ Q(template_i)]* ␞ <n_outputs> [␞ Q(name_j) ␞ Q(probe_j)]*
```

- `<show>` es `true` o `false`, sin comillas.
- `<n_templates>` y `<n_outputs>` son la cardinalidad de cada lista en decimal.
  Van **delante** de la lista y no son decorativas: sin ellas, dos comandos con
  el mismo total de elementos repartido de otra forma entre las dos listas
  producirían la misma cadena.
- Las plantillas y los outputs conservan **el orden declarado**. Reordenar dos
  plantillas cambia el orden de interpolación, así que cambia la huella.
- Un `workdir` ausente es la cadena vacía, y `Q("")` es `""`, no ausencia.
- Las rutas de plantilla y el workdir se normalizan a `/` **antes** de
  entrecomillarse: la misma declaración en Windows y en Linux produce la misma
  huella.

`Q` es lo que hace la regla inyectable: como escapa los caracteres de control,
ni el separador ni el salto de línea pueden aparecer dentro de un campo. Esta
regla **no** hereda la ambigüedad teórica del separador que la huella del árbol
dejó congelada (`SPEC-v1.md` §6).

### 3.3 El orden

Las entradas van en **el orden declarado en `commands.yaml`**, sin ordenar.

Es la diferencia deliberada con las otras dos huellas, que sí ordenan: en un
árbol de archivos y en un conjunto de variables el orden no significa nada; en
una lista de comandos es la secuencia de ejecución. Ejecutar `build` y luego
`deploy` no es lo mismo que al revés, y la huella tiene que verlo.

### 3.4 La composición

Las entradas se unen con `\n` (U+000A) **sin salto final**, y la huella es
`H(...)`.

Un paso sin comandos produce la huella de la cadena vacía:
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`. En la
práctica no se alcanza —un `commands.yaml` vacío es `skipped{no_commands}` antes
de llegar aquí (spec 04 §5.3)—, pero la regla está definida para ese caso.

La representación externa es:

```
inst-v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo. El token es
**distinto** del `v1:` del árbol a propósito: las dos huellas son del mismo tipo
y entran en el mismo material, pero son reglas distintas y sus versiones tienen
que poder moverse por separado.

### 3.5 Las dos decisiones por campo

**`show` entra, aunque no cambie qué se ejecuta.** El criterio de la spec 08
—«se corrige lo que produce un falso negativo del caché: lo que cambia el
despliegue sin cambiar la huella»— dejaría `show` fuera: sólo decide si la salida
del comando se imprime. Entra igualmente porque aquel criterio está formulado
para la huella del **árbol**, donde el consumidor es la máquina. Aquí el
consumidor es una persona editando `commands.yaml`, y excluirlo produce esta
secuencia:

1. un comando falla o hace algo raro;
2. el autor añade `show: true` para ver qué pasa;
3. la huella no cambia ⇒ el paso se **salta** ⇒ no se imprime nada;
4. el autor concluye que `show` no funciona.

Un caché que ignora una edición deliberada del pipelinecode es indistinguible de
un caché roto.

**`description` no entra, y ésta es su justificación por campo.** No llega al
modelo de ejecución: el mapeo `PipelineCommandDTO → command.Command` lo descarta,
así que no existe para nada de lo que ocurre después. No cambia qué se ejecuta,
ni dónde, ni qué se imprime, ni qué se extrae: editarla no tiene **ningún**
efecto observable que un caché pudiera suprimir, que es justo lo contrario del
caso de `show`. Si algún día `description` llega al dominio —a un log, al
registro de despliegue—, esta exclusión deja de estar justificada y hay que
reabrirla en una `inst-v2`.

## 4. Lo que la regla NO mira

- **Nada del entorno de ejecución.** Ni el ambiente, ni el proyecto, ni el
  pipeline, ni el paso. Esta huella responde sólo «¿cambiaron estas
  instrucciones?»; el resto de la pregunta —«¿ya se ejecutaron *aquí*?»— la
  responde la `cache_key`, que lleva esos cuatro campos por derecho propio
  (spec 10 §5.1).
- **El resultado de interpolar.** El material es la declaración, con sus
  `${var....}` sin sustituir. Lo que las variables valgan es material de la
  huella de variables, y contarlo dos veces no añadiría nada.

## 5. Límites conocidos de la `inst-v1`

- **La huella no distingue un paso sin comandos de un paso cuyo `commands.yaml`
  no existe.** No importa: los dos son `skipped{no_commands}` y ninguno llega a
  componer una clave.
- **`description` no participa** (§3.5). Es una exclusión justificada, no un
  límite accidental, pero se lista aquí para que aparezca al buscar.

## 6. Vectores

Estos vectores son la tabla que una implementación independiente debe
reproducir. Están ejecutados en `instructions_test.go`.

Convención de la columna «instrucciones»: cada comando se escribe como
`name | cmd`, y los campos que no se nombran valen su cero — `workdir` vacío,
`show: false`, sin plantillas y sin outputs.

| # | Instrucciones | Huella |
|---|---|---|
| 1 | *(lista vacía)* | `inst-v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| 2 | `build \| mvn package` | `inst-v1:6957cd1bfdfa202bc93165b6e2d627ce8543b2ce119b03d89c8043165356abd8` |
| 3 | el 2 con `workdir: app` | `inst-v1:2e85db12dbba7e1f2f6f04afefe0093c22b81956c2b62470239663464f32f0f4` |
| 4 | el 2 con `show: true` | `inst-v1:0cf50eb19529cd6cc6e884e0ebd239d593cc788fdeaae9b635642b1d0179dc8d` |
| 5 | el 2 con `templates: [a.yaml, b.yaml]` | `inst-v1:c0ed247e9890e0606b5a61a0549f0adb2a977d8e50506eda3ae7b8576c189593` |
| 6 | el 2 con `outputs: [artefacto ← `version: (\S+)`]` | `inst-v1:ea1c8f0b489b3ae8ef2634b4cccf8a5389cad346db2fa059d1b885535f6dcb8f` |
| 7 | `a \| echo a`, `b \| echo b` | `inst-v1:ad02cacda4192098d6dd576f1e1cc57bc9e551ea7a8cd15347a330da005cc3a8` |
| 8 | el 7 con los dos comandos al revés | `inst-v1:87adcfbf395b6f75f1665b801627145486048a4162672388a75d8b753e1dc258` |

Los vectores 2, 3 y 4 se diferencian en un solo campo cada uno, y el 4 es el que
fija la corrección de §3.5: si `show` no participara, el 4 sería idéntico al 2.
Los vectores 7 y 8 tienen los mismos comandos en distinto orden y **son
distintos**, que es §3.3.

## 7. Cómo validar una implementación independiente

1. Reproducir los vectores de §6.
2. Comprobar las sensibilidades: cambiar `name`, `cmd`, `workdir`, `show`,
   cualquier plantilla, cualquier `outputs[].name` o `outputs[].probe`, el orden
   de las plantillas, el orden de los outputs o el orden de los comandos cambia
   la huella.
3. Comprobar la insensibilidad: cambiar `description` **no** la cambia.
4. Comprobar que un `workdir` declarado con `\` produce la misma huella que el
   mismo declarado con `/`.

## 8. Historia

| Versión | Cambio |
|---|---|
| `inst-v1` | Primera regla congelada (spec 10). Respecto de lo que el motor calculaba antes: entra `show`, la huella lleva prefijo de versión y token propio, y el material queda escrito. El resto de los campos y su orden se conservan tal cual estaban |
