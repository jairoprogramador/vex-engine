# Huella de contenido — regla v1

> **Estado:** congelada · **Prefijo:** `v1:` · **Implementación de referencia:**
> `compute.go` + `ignore.go`, en este mismo directorio · **Vectores:** §7

> **Este paquete especifica TRES reglas, y este documento es sólo la primera.**
> La spec 10 pagó las otras dos, que la 08 había dejado sin hacer:
>
> | Regla | Prefijo | Documento |
> |---|---|---|
> | contenido de un árbol de archivos | `v1:` | **este** |
> | instrucciones de un paso | `inst-v1:` | `SPEC-INSTRUCTIONS-v1.md` |
> | variables de un paso | `vars-v1:` | `SPEC-VARIABLES-v1.md` |
>
> Los tres tokens de versión son **distintos** a propósito: las tres huellas son
> del mismo tipo y entran en el mismo material de `cache.Material`, pero son
> reglas distintas y sus versiones tienen que poder moverse por separado.

Este documento es **normativo**. La huella de contenido es la primitiva de
identidad del motor: responde a «¿cambió el código?» hoy y a «¿es este el mismo
despliegue?» cuando el registro exista. El valor entero de esa decisión es la
**comparabilidad** — dos organizaciones deben poder comparar huellas sin
coordinarse—, y eso exige que un tercero pueda reimplementar la regla y obtener
el mismo valor bit a bit.

Si el código y este documento discrepan, **discrepan los dos**: uno de los dos
tiene un bug y hay que decidir cuál. Lo que no se puede hacer es cambiar la
regla y dejar el prefijo en `v1:`.

---

## 1. Vocabulario

| Término | Significado |
|---|---|
| **árbol** | un directorio y todo lo que cuelga de él |
| **raíz** | ese directorio; nunca produce entrada |
| **ruta** | posición relativa a la raíz, con `/` como separador, sin `./` delante |
| **entrada** | la línea de texto con la que una hoja del árbol participa en la huella |
| **hoja** | cualquier cosa que no es un directorio: archivo regular o enlace simbólico |

Todas las rutas de esta especificación son rutas en el sentido de la tabla. La
huella **no depende del sistema operativo**: un árbol servido desde Windows y el
mismo árbol servido desde Linux producen el mismo valor.

## 2. La fuente del árbol

La regla no lee del sistema de archivos: lee de una fuente que debe cumplir el
contrato de `TreeSource` (ver `tree_source.go`).

1. El recorrido **no emite la raíz**, sólo sus descendientes.
2. Dentro de un directorio, las entradas se emiten en **orden lexicográfico
   ascendente por nombre** —comparación byte a byte del nombre—, y un directorio
   se emite **antes** que su contenido.
3. Los enlaces simbólicos **no se siguen jamás**. Un enlace no es un directorio,
   apunte a donde apunte.
4. Abrir un archivo regular devuelve su contenido. Abrir un enlace devuelve **su
   destino** como texto, normalizado a `/`, sin resolverlo.
5. El recorrido debe admitir que quien lo consume pida **omitir el contenido de
   un directorio** sin abandonar el resto del árbol.

El punto 2 no es cosmético: la precedencia de reglas `.gitignore` de la v1
depende del orden del recorrido (§5, divergencia **a**).

## 3. La regla

### 3.1 Qué participa

Se recorre el árbol y se produce **una entrada por hoja visible**. No producen
entrada:

1. La raíz.
2. Los **directorios**. Un directorio vacío es indistinguible de un directorio
   ausente.
3. El directorio `.git` y todo su contenido, se llame como se llame el resto.
   Es la única exclusión por nombre que no viene de una regla.
4. Cualquier hoja cuyo último componente sea `.gitignore`, sea archivo regular o
   enlace. El archivo de reglas no es código: cambiar un comentario dentro de él
   no cambia el despliegue.
5. Toda hoja que las reglas de exclusión (§4) marquen como ignorada.
6. El contenido de todo directorio que las reglas marquen como ignorado — sin
   descender: una re-inclusión escrita dentro de un directorio ignorado **no
   rescata nada**.
7. Lo que desaparece durante el recorrido. Un árbol vivo cambia mientras se lee;
   lo que ya no está no entra. Cualquier **otro** fallo de lectura aborta el
   cálculo: una huella válida de un árbol que nadie leyó entero es peor que un
   error.

### 3.2 La entrada

Sea `H(x)` el SHA-256 de `x` en hexadecimal minúsculo (64 caracteres).

| Hoja | Entrada |
|---|---|
| archivo regular sin bit de ejecución | `<ruta>:<H(contenido)>` |
| archivo regular con bit de ejecución | `<ruta>:<H(contenido)>:x` |
| enlace simbólico | `<ruta>:<H(destino)>:l` |

- **Bit de ejecución** es `modo & 0o111 != 0`. Cualquiera de los tres bits basta,
  y **sólo** esos tres entran: el resto del modo depende del `umask` y de cómo se
  hizo el checkout, y sería ruido no reproducible.
- Para un **enlace**, el bit de ejecución no se mira: los permisos que cuentan
  son los del destino, y el destino no se sigue. El contenido de un enlace *es*
  su destino, como texto.
- El separador entre campos es `:` (U+003A).

### 3.3 El orden

Las entradas se ordenan **ascendentemente por comparación byte a byte de la
entrada completa**, no de la ruta.

No es lo mismo, y por eso se especifica. Con dos archivos `a` y `a.txt`, ordenar
por ruta pone `a` primero; ordenar por entrada pone `a.txt:…` primero, porque el
byte que sigue a `a` es `:` (0x3A) en un caso y `.` (0x2E) en el otro. Una
implementación que ordene por ruta produce una huella distinta para el mismo
árbol.

### 3.4 La composición

Las entradas ordenadas se unen con `\n` (U+000A) **sin salto de línea final**, y
la huella es el SHA-256 de esa cadena, en hexadecimal minúsculo.

Un árbol sin entradas produce la huella de la cadena vacía:
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

### 3.5 La representación externa

```
v1:<64 caracteres hexadecimales minúsculos>
```

El prefijo es obligatorio en todo lo que salga del cálculo: lo que se persiste,
lo que se transmite y lo que se compara. Dos huellas de versiones distintas
**nunca** son iguales, aunque el hash coincida.

Sin el prefijo, corregir una de las divergencias congeladas de §5 en una `v2`
sería indistinguible de un bug.

## 4. Las reglas de exclusión

La v1 interpreta los archivos `.gitignore` del árbol con **su propio motor**, que
se especifica aquí. No se delega en una librería a propósito: la semántica de una
primitiva de identidad congelada no puede depender de la versión de un módulo
externo, donde un `go get -u` cambiaría todas las huellas del mundo sin un solo
cambio en Vex.

### 4.1 Recolección

Se recorre el árbol acumulando las reglas de cada `.gitignore` **en el orden en
que el recorrido visita los directorios que los contienen** — la raíz primero.

Se leen los `.gitignore` de **todos** los directorios excepto los que cuelgan de
`.git`, incluidos los de directorios que otra regla ignora.

Sólo un **archivo regular** llamado `.gitignore` es un archivo de reglas: seguir
un enlace para leer reglas sacaría la identidad fuera del árbol.

### 4.2 El dominio de una regla

Una regla escrita en `<dir>/.gitignore` tiene por **dominio** los componentes de
`<dir>`, y sólo se evalúa sobre rutas estrictamente contenidas en él. La
comparación se hace sobre la ruta **relativa al dominio**.

### 4.3 Interpretación de una línea

En este orden:

1. Se descarta un `\r` final —una línea escrita en CRLF y otra en LF son la misma
   regla—.
2. Se recortan los espacios y tabuladores finales **que no estén escapados**. Un
   carácter está escapado si viene precedido por un número **impar** de barras
   invertidas.
3. Una línea vacía se ignora.
4. Una línea que **empieza** por `#` es un comentario y se ignora. `\#` no
   empieza por `#`: es un patrón literal.
5. Un `!` inicial marca la regla como **re-inclusión** y se consume. `\!` no es un
   `!` inicial.
6. Un `/` final marca la regla como **sólo directorios** y se consume.
7. Un `/` inicial marca la regla como **anclada** y se consume. Si no lo hay pero
   el patrón contiene algún `/`, la regla queda anclada igual.
8. Lo que queda es el patrón.

### 4.4 Evaluación

Para cada hoja o directorio, se evalúan **todas** las reglas cuyo dominio la
contiene, en el orden de acumulación de §4.1. **Gana la última que casa.** Si
ninguna casa, la ruta no está ignorada.

Una regla marcada como sólo-directorios no se evalúa sobre una hoja. Un enlace
nunca es un directorio: `build/` no ignora un enlace llamado `build`.

Casan así, sobre los componentes de la ruta relativa al dominio:

| Forma del patrón | Semántica |
|---|---|
| contiene `**` | los componentes del patrón se casan contra los de la ruta; `**` absorbe cero o más componentes |
| anclada | los componentes del patrón se casan contra el **prefijo** de los de la ruta |
| ni una ni otra | el patrón se casa contra **cada** componente de la ruta por separado; basta uno |

Cada componente se casa con la sintaxis de globs de `path.Match` de Go: `*` no
cruza `/`, `?` es un carácter, `[…]` es una clase, y `\` vuelve literal al
carácter siguiente.

### 4.5 Un patrón inválido no casa con nada

Un patrón que no es un glob válido —`[` sin cerrar, `\` final— no casa con
ninguna ruta y no aborta el cálculo. Es una decisión, no un descuido: abortar una
ejecución por una línea rara en un `.gitignore` heredado sería peor.

## 5. Divergencias respecto de git — congeladas en la v1

La v1 **no reproduce** `git check-ignore`. Estas dos diferencias son conocidas,
están aquí para que nadie las descubra depurando, y son la razón de ser del
prefijo de versión:

| # | Divergencia | Qué hace git |
|---|---|---|
| **a** | Las reglas se evalúan en el orden en que el recorrido las acumuló, y gana la última que casa | Git da prioridad al `.gitignore` **más profundo**, con independencia del orden de lectura |
| **b** | Un patrón anclado compara sólo el **prefijo** de componentes: `/build` casa también con `build/x/y` | Para git, `/build` casa con la entrada `build`; que su contenido quede fuera es consecuencia de excluir el directorio, no de casar cada descendiente |

Ninguna de las dos produce un **falso negativo del caché** —no hay un cambio que
altere el despliegue y deje la huella igual—, que es el criterio con el que se
decidió qué corregir y qué congelar. Producen una huella distinta de la que daría
git, que es una divergencia, no una incorrección.

Corregir cualquiera de las dos exige reescribir el motor de reglas y **cambia
todas las huellas**: eso es una `v2`.

## 6. Límites conocidos de la v1

- **Ambigüedad teórica del separador.** Las entradas no se vuelven a leer nunca,
  sólo se concatenan y se hashean, pero una ruta que contenga `:` o `\n` podría en
  teoría construir la misma cadena que otra combinación de rutas. No se corrige en
  la v1 porque la corrección —longitudes explícitas o escapado— cambia todas las
  huellas.
- **La huella no distingue un archivo vacío de uno ausente si además está
  ignorado**, por construcción: lo ignorado no existe para la identidad.
- **Los directorios vacíos son invisibles.** Igual que en git.
- **`.git` se excluye por ser un directorio, no por llamarse así.** En un *worktree*
  de git o en un submódulo, `.git` es un **archivo** cuyo contenido es
  `gitdir: <ruta absoluta de la máquina>`. Ese archivo entra en la huella, así
  que el mismo código en dos máquinas distintas produce huellas distintas. Es un
  defecto de reproducibilidad conocido de la v1 y **la única razón por la que no
  se corrige aquí es que corregirlo cambia huellas**: la v1 se congela tal cual
  se encontró salvo en lo que producía un falso negativo del caché. Anotado como
  candidato de la v2.
- **Del modo sólo entra el bit de ejecución.** Un `chmod 600` sobre un archivo
  legible no cambia la huella.

## 7. Vectores

Estos vectores son la tabla que una implementación independiente debe
reproducir. Están ejecutados en `compute_test.go` sobre una fuente **en memoria**:
no hace falta materializar un sistema de archivos para validarse, y no hay un
`.gitignore` de fixture que git pueda aplicarle al propio repositorio.

Convenciones de la columna «árbol»: `nombre = contenido` es un archivo regular,
`nombre =x contenido` uno con el bit de ejecución, `nombre -> destino` un enlace,
`nombre/` un directorio vacío.

| # | Árbol | Huella |
|---|---|---|
| 1 | `a.txt = contenido a`, `b.txt = contenido b`, `c.txt = contenido c` | `v1:6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22` |
| 2 | `a.txt = contenido a`, `vacio.txt = ` | `v1:58873ec8ef3abdaf28dedac7d5b045d508e20491e6d2314f944cfacb4afe5bb3` |
| 3 | el árbol 1 más `vacio/` y `anidado/tambien/vacio/` | igual que el 1 |
| 4 | `.gitignore = *.log`, `a.txt = contenido a`, `b.txt = contenido b`, `ruido.log = descartado` | `v1:fb44111d5c62394ae511463fcaf76f8b2c6379d23b97659a071e69901f6169af` |
| 5 | `sub/.gitignore = secret.txt`, `secret.txt = raíz visible`, `sub/secret.txt = ignorado`, `sub/otro.txt = visible` | `v1:7faaa15e671a4adb045b376b53b6cef4c77dbc508fb48642070e1aeb9b3a9479` |
| 6 | `.gitignore = *.log⏎!keep.log`, `sub/.gitignore = !deep.log`, `keep.log`, `ruido.log`, `sub/deep.log`, `sub/ruido.log` | `v1:32e3e2193ec399c579dc0d38aa82b2dcc1d717056c02240b9407039b9fc3fbbf` |
| 7 | `.gitignore = **/generated.txt` + `generated.txt`, `a/generated.txt`, `a/b/generated.txt`, `a/b/real.txt`, `a/real.txt` | `v1:f053a46f82c8d608e8bb7c3391a0e2a994929491e0f253c7aeb4f2a103b83d28` |
| 8 | `.gitignore = /build` + `build/x/y.txt`, `build/z.txt`, `src/build/keep.txt`, `main.txt` | `v1:acbdfa0e12f3452a0b5720fefec37c2e1ae3087f0302c847af14c4fe2eb1ef80` |
| 9 | `.gitignore = target` + `target/out.bin`, `mod/target/out.bin`, `otro/target`, `mod/src/main.txt`, `mod/src/util.txt` | `v1:365131bb6b6b5016dd2045e2a0b6c66bed74ba2b895eccf53f09ab5366ef2625` |
| 10 | `.gitignore = build/` + `build/out.bin`, `mod/build/out.bin`, `otro/build`, `otro/keep.txt` | `v1:735d799faf41c269be22142880cc1ced683b72c056e272b5db239de81967c603` |
| 11 | el árbol 1 más `.git/HEAD` y `.git/objects/ab/cdef` | igual que el 1 |
| 12 | `.gitignore = *`, `a.txt`, `sub/b.txt` | `v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| 13 | el árbol 1 más `link-a.txt -> a.txt` y `link-dir -> .` | `v1:fd5c03e1e9bb7b2cb0b44cac46f8a41e8dc4e34800433ff93f644d13530df715` |
| 14 | `a.txt = contenido a`, `deploy.sh =x #!/bin/sh⏎echo hola⏎` | `v1:da09ecfdebafb25a6841743107eed46cca165b9ab7b4a35fb3223ed33a3feddd` |
| 15 | `.gitignore = ruido\ `, `a.txt = contenido a`, `ruido␣ = …` | `v1:5d2068fc2f1ff88970c37f5a7f3e40c065d791c207352f8127f30b44b8d722f8` |
| 16 | `.gitignore = \#raro⏎\!raro`, `a.txt = contenido a`, `#raro`, `!raro` | `v1:5d2068fc2f1ff88970c37f5a7f3e40c065d791c207352f8127f30b44b8d722f8` |

`⏎` es un salto de línea dentro del contenido del archivo; `␣` un espacio
significativo en un nombre. Los vectores 15 y 16 coinciden porque los dos árboles
se reducen a la misma entrada visible, `a.txt`: es lo que comprueban.

Todo `.gitignore` de la tabla termina en salto de línea.

## 8. Cómo validar una implementación independiente

1. Reproducir los 16 vectores de §7.
2. Comprobar que la huella de un árbol servido desde disco coincide con la del
   mismo árbol servido desde memoria. Sin esto, los vectores no dicen nada sobre
   lo que se calcula en producción.
3. Comprobar las tres sensibilidades que no se ven en un valor aislado: un byte
   distinto en un archivo visible cambia la huella; un `chmod +x` la cambia;
   cambiar el destino de un enlace la cambia.
4. Comprobar las tres insensibilidades: el contenido de `.git`, el contenido del
   propio `.gitignore` y los directorios vacíos **no** la cambian.

## 9. Historia

| Versión | Cambio |
|---|---|
| `v1` | Primera regla congelada (spec 08). Respecto de lo que el motor calculaba antes: entra el bit de ejecución, entran los enlaces con su destino, el parser de `.gitignore` respeta los escapes, y la huella lleva prefijo de versión. Las divergencias **a** y **b** se congelan tal cual estaban |
