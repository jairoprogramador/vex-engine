# RD-04 — Definición de Pipeline: estructura del código y casos probados

> Mapa del código tal como está hoy (verificado contra `internal/definicion/`, pruebas en verde con `-race`).
> Sirve para refactorizar sin perder de vista el objetivo. Ficha de la unidad: `RD-04-definicion.md`.
> Modelo: `docs/modelo/contextos/definicion.md`.

## §1 El objetivo, en una línea

**Convertir lo que el DevOps escribe en un pipeline que nadie puede recibir mal formado.**
De ahí salen los dos invariantes que cualquier refactor tiene que preservar:

1. **Puerta única**: `dominio.Comprobar` es la *única* forma de obtener un `Pipeline` (`DEC-08.2`).
2. **La comprobación no depende del ambiente**: la misma declaración da la misma respuesta para todos
   (`DEC-02.7`), porque no resuelve ni ejecuta nada — solo mira lo escrito.

Todo lo demás (lectura de ficheros, traducción, puertos) existe para servir a esos dos.

## §2 Las cinco carpetas

| Carpeta | Qué es | Depende de | Tamaño |
|---|---|---|---|
| `dominio/` | el modelo y la regla: `Declaracion` → `Comprobar` → `Pipeline` | solo stdlib | 1.226 líneas |
| `aplicacion/` | el caso de uso: pedir el pipeline y traducirlo al lenguaje publicado | `dominio`, `publicado` | 138 |
| `infraestructura/` | los adaptadores: leer el disco (`lectura.go`) y el ACL hacia Suministro (`pipelines.go`) | `dominio`, `suministro/publicado`, `yaml.v3` | 428 |
| `publicado/` | lo que otros contextos ven: tipos de datos + una interfaz por cliente | solo stdlib | 197 |
| `testdata/ejemplo/` | un pipeline real en formato 3 que usa todo lo que el formato permite | — | 28 ficheros |

Dirección de dependencias: `interfaces → aplicacion → dominio ← infraestructura`, y `publicado` no depende de
nadie (por eso no puede exponer tipos del dominio y hay una traducción explícita).

### Ficheros del dominio

| Fichero | Responsabilidad | Líneas |
|---|---|---|
| `declaracion.go` | **lo escrito**, sin juicio: `Declaracion` (incluida `ConfiguracionesDePaso`, de `steps` en `config.yaml` — RD-04 §9.20), `PasoEscrito`, `VariableEscrita`, `Ilegible`, `Desconocidos` | 113 |
| `pipeline.go` | **lo comprobado**: agregado inmutable `Pipeline` + `Paso`, `Comando`, `VariableDeSalida`, `Asercion` | 151 |
| `comprobacion.go` | **la regla**: `Comprobar` y las seis filas de invariantes | 749 |
| `valores.go` | objetos-valor y patrones: `Ambito`, `Regla`, `VariableEstandar`, regex, `usos()`, `rutaLocal()` | 122 |
| `errores.go` | `Invariante`, `Fallo`, `FallosDeComprobacion`, `ErrInvalido`, `ErrNoExiste`, `ErrNoComprobado` | 72 |
| `puertos.go` | el puerto `Pipelines` (lo implementa el ACL) | 13 |

## §3 El flujo, de punta a punta

```
cliente (Ejecución / Resolución / Simulación)
  │  publicado.ParaEjecucion.DeHoy(ctx, fuente)
  ▼
aplicacion.Servicio.DeHoy ─────────────────────────────── traduce errores y tipos
  │  dominio.Pipelines.DeHoy      (puerto, definido en el dominio)
  ▼
infraestructura.PipelinesDeSuministro.DeHoy             ← ACL, context-map #12
  │  1. suministro.TraerDeHoy(fuente)  → Material{Directorio, Commit, Hash}
  │  2. leer(directorio)               → dominio.Declaracion      (lectura.go)
  │  3. dominio.Comprobar(declaracion) → *dominio.Pipeline | *FallosDeComprobacion
  │  4. suministro.Retirar(material)   → SIEMPRE, con context.WithoutCancel
  ▼
aplicacion.traducirPipeline / traducir → publicado.Pipeline | *publicado.FallosDeComprobacion
```

Tres cosas que este flujo decide y conviene no mover:

- **El paso 4 corre siempre**, y si falla no se entrega el pipeline: el error de limpieza no se pierde
  (`pipelines.go:43-55`).
- **`leer` solo devuelve error si no puede leer el disco.** Un YAML roto, una clave que el formato no tiene o
  un fichero que no pertenece al formato **no son errores**: van dentro de la `Declaracion`, en `Ilegibles` y
  `Desconocidos`. Así la comprobación sigue siendo la única que produce fallos.
- **El pipeline sale entero en memoria**, material incluido, y el material se retira. Pérdida aceptada: tiene
  que caber en memoria.

## §4 Los dos modelos, y por qué son dos

| | `Declaracion` | `Pipeline` |
|---|---|---|
| Qué es | lo que está escrito | lo que además está bien escrito |
| Campos | exportados, mutables | privados, con consultas que devuelven copias |
| Quién la crea | el ACL (`leer`) | solo `Comprobar` |
| Garantiza | nada | las seis filas de la comprobación |
| Vocabulario | el del formato (`scope`, `rules`, `probe`, strings crudos) | el del modelo (`Ambito`, `Regla`, `Asercion`) |

Esta separación es la que permite que el adaptador **no decida nada**. Si el ACL construyera el `Pipeline`
directamente, la validación se repartiría entre disco y dominio y aparecería un segundo camino para obtener uno.

`Declaracion` guarda además los valores del formato **como se escribieron** (`Reglas []string`,
`EdadMaxima string`, `Ambito string`): rechazar un token desconocido es de la comprobación, no de la lectura.
Los punteros (`Version *int`, `Valor *string`, `Rules *[]string`) distinguen «no escrito» de «escrito vacío»,
que son fallos distintos (`rules: []` = no mirar nada ≠ sin `rules` = mirar las tres).

## §5 La comprobación: fases y por qué ese orden

`Comprobar` acumula fallos en un `*comprobacion` y no aborta en el primero: un DevOps quiere la lista entera.

```
version()              ← puerta: si no es schema_version 1, NO se comprueba nada más
Ilegibles/Desconocidos ← lo que la lectura no supo leer, como fallos de formato
ambientes()            → c.ambientesComprobados
pasos()                → c.pasosComprobados   (configuracion → material → comandos → outputs)
salidas()              → c.produccion  map[nombre]salidaProducida
variablesDeclaradas()  → c.declaradas  []declarada
usos()                 → circulos() + valoresDeclarados() + usosEnLosPasos()
────────────────────── si hay algún fallo: *FallosDeComprobacion, y no hay pipeline
```

El orden **no es arbitrario**, y esa es la restricción principal de cualquier reordenamiento:

| Fase | Necesita antes | Por qué |
|---|---|---|
| `version` | — | un formato distinto haría fallar todo lo demás por la misma causa |
| `variablesDeclaradas` | `ambientes` | un directorio de `variables/` es válido solo si es el `value` de un ambiente |
| `salidas` | `pasos` | indexa por posición `(paso, comando)`, que solo existe tras ordenar los pasos |
| `usos` | `salidas` + `variablesDeclaradas` | resolver un nombre es buscar en los dos índices |

## §6 El núcleo: ámbito, visibilidad y orden

Es donde vive la complejidad real, y lo que hay que entender antes de tocar nada.

**Ámbito** (`valores.go`): una variable pertenece a un ámbito, **no a un paso**. `Compartido` es el ámbito vacío;
los demás son el `value` de un ambiente. La regla cabe en una línea:

```go
func (a Ambito) Ve(otro Ambito) bool { return otro == a || otro.EsCompartido() }
```

Desde un ambiente se ve lo compartido; desde lo compartido **no** se ve lo de un ambiente (un valor compartido
que dependiera del ambiente dejaría de ser el mismo en todos).

**Visibilidad de una salida** (`salidaProducida.laVe`): lo compartido se ve desde todos; lo del ámbito de un
ambiente, solo desde un ambiente.

**El ámbito propio de un paso** (`Paso.Ambito *Ambito`, RD-04 §9.19): es lo que `config.yaml` declara con
`steps.<paso>.scope: environment | shared` (`configuracion()`, junto a `Reglas` y `EdadMaxima`; sin `scope`,
`Ambito` queda en `nil`). Es del mismo tipo `Ambito` que una variable declarada, no un booleano aparte ni un
tercer concepto: `nil` es «no declaró uno propio» (se resuelve contra el ambiente en que se ejecuta cada
vez, que siempre está — RD-04 §9.19, «Corrección de tipo»); no-nil solo puede apuntar a `Compartido`, porque
un paso no se declara «del ambiente prod», solo `shared`. `VariableDeSalida.Ambito *Ambito` es igual.
Participa de la **misma** regla de visibilidad que una variable, no queda al margen de ella:

- **Qué ve.** `usosEnLosPasos` ya no usa el ambiente de la solicitud para revisar un paso: usa el ámbito del
  paso (`*paso.Ambito` si no es `nil`, si no el ambiente de la solicitud — que es lo mismo que antes, así que
  un paso sin `scope` no cambia). Y `seVe` ya no da una variable de salida por vista con solo que exista en
  `produccion`: exige `salida.laVe(ambito)`, la misma regla que ya aplicaba `valoresDeclarados` para un
  literal. Un paso `shared` que use una variable declarada de un ambiente, o una de salida que no sea
  `shared`, falla igual que fallaría un valor compartido en su lugar.
- **Bajo qué ámbito se archiva la historia del paso** (Historial/Resolución, `DEC-06.12`) — fuera de este
  paquete, que solo lo publica.
- **Qué hereda por defecto** (`outputs()`) una variable de salida sin `scope` propio: `salida.Ambito =
  clonarAmbito(paso.Ambito)` — copiar el ámbito del paso, no derivar un booleano de él. Un `scope: environment`
  u otro `scope: shared` explícito en el `outputs` sigue anulando la herencia en cualquier sentido.
  `Paso.copia()` y el bucle de `Comando.VariablesDeSalida` clonan el puntero (`clonarAmbito`, en
  `valores.go`) para que dos copias del pipeline no lo comparta.

**Orden** (`posicion{paso, comando}` + `antesDe`): un texto interpolado en un punto puede usar lo que se
produjo **estrictamente antes**. Ni siquiera lo que produce su propio comando.

**Resolución transitiva** (`necesita`): usar `${var.x}` donde `x` es un literal que usa `${var.y}` exige que
`y` esté producida antes de ese punto. La recursión lleva un `visto` para no ciclar; los círculos entre
literales se rechazan aparte, en `circulos()`, con un DFS de tres estados por ámbito.

**Fallos por ambiente** (`problemas`): `usosEnLosPasos` recorre el pipeline **una vez por ambiente**, porque lo
visible cambia con él. Un fallo que ocurre en todos se reporta **una sola vez, sin ambiente**; si ocurre en
algunos, una vez por ambiente. Esa es toda la razón de existir del tipo `problemas`.

## §7 Conceptos aplicados, y qué compran

| Concepto | Dónde | Qué garantiza | Qué pasaría sin él | SOLID |
|---|---|---|---|---|
| **Agregado siempre válido** | `Pipeline` sin campos exportados, `Comprobar` única factoría | nadie recibe un pipeline mal formado | validación repetida en cada consumidor, y divergente | SRP |
| **Factoría que devuelve fallos, no excepciones** | `Comprobar → (*Pipeline, error)` con `*FallosDeComprobacion` | el DevOps ve todo lo que está mal de una vez | corregir de uno en uno, N iteraciones | — |
| **Separar «lo escrito» de «lo comprobado»** | `Declaracion` vs `Pipeline` | el adaptador no decide | reglas de negocio en el lector de YAML | SRP |
| **ACL (anticorruption layer)** | `PipelinesDeSuministro` | el vocabulario de Suministro (`Material`, sus errores) no entra al dominio | acoplamiento a cómo se clona un repo | DIP |
| **Puerto en el consumidor** | `dominio.Pipelines` definido en `dominio/` | el dominio no conoce a quien lo sirve | dominio dependiendo de infraestructura | DIP |
| **Lenguaje publicado** | `publicado/` solo stdlib, traducción explícita en `aplicacion` | otros contextos no ven el modelo interno; se puede refactorizar el dominio sin romperlos | cada cambio del agregado rompe a tres contextos | ISP, DIP |
| **Una interfaz por cliente** | `ParaEjecucion`, `ParaResolucion`, `ParaSimulacion` | cada uno depende solo de lo que usa; `ParaResolucion` no puede pedir el de hoy | una interfaz gorda que invita a usarlo todo | **ISP** |
| **Copias defensivas** | `Pasos()`, `Ambientes()`, `Variables()`, `Paso.copia()` | inmutabilidad real, no por convención | un consumidor muta el pipeline de otro | — |
| **Traducción de errores en cada frontera** | `infraestructura.traducir`, `aplicacion.traducir`, `errorTraducido` | `errors.Is` funciona en cada capa **sin perder el mensaje original** | fugas de errores de git hasta el usuario | — |
| **Objetos-valor con la regla dentro** | `Ambito.Ve`, `posicion.antesDe`, `salidaProducida.laVe` | la regla se escribe una vez | `if` repetidos y divergentes | SRP |
| **Tabla en vez de `switch`** | `reglasDelFormato` | añadir una regla del formato es una entrada | tocar el flujo de control | **OCP** |
| **Puerta única verificada por prueba** | `TestNoHayForma…` recorre el paquete con `go/parser` | el invariante no se rompe por descuido | la regla queda como comentario | — |

## §8 Casos probados

24 funciones de prueba, **113 casos de tabla**, 153 ejecuciones. Las dos tablas grandes prueban **las mismas
filas en dos niveles**: el dominio sobre una `Declaracion` en memoria, e infraestructura rompiendo una copia
del pipeline de ejemplo fichero a fichero (así se prueba también la lectura).

Las dos tablas exigen `len(fallos) == 1` salvo que el caso se marque `varios`: **un fallo no arrastra otros**.

### 8.1 Camino feliz y propiedades del agregado (`dominio/comprobacion_test.go`)

| Prueba | Qué fija |
|---|---|
| `UnaDeclaracionBienFormadaDaUnPipeline` | ambientes en orden, nombre sin `NN-`, reglas, `max_age`, workdir limpio, plantillas, material ejecutable; + 4 subcasos (ámbito de la variable, `outputs` con/sin `name`, defaults sin entrada en `steps`, `rules: []`) |
| `LoQueVeUnPasoEsSuAmbito` | las variables del ámbito las ve cualquier paso; un comando ve lo del comando anterior; dos ambientes pueden declarar el mismo nombre |
| `UnPasoSinScopeEsDelAmbienteEnQueSeEjecuta` | sin entrada en `steps` de `config.yaml`, `Paso.Ambito` es `nil` (RD-04 §9.19) |
| `ElAmbitoDeUnPasoLoHeredaLoQueProduceSinScopePropio` | un paso `scope: shared` hace que su `outputs` sin `scope` propio salga compartido (RD-04 §9.19) |
| `UnPasoDeScopeSharedNoVeLoDeUnAmbiente` | un paso `scope: shared` no ve una variable declarada de un ambiente ni una variable de salida que no sea `shared` (RD-04 §9.19) |
| `RenumerarUnPasoNoCambiaSuIdentidad` | `00-`/`07-` cambia el orden y **nada más** |
| `UnFicheroQueNoEstaEnTemplatesSeCopiaSinInterpolar` | el `${var.…}` de terraform no es del motor |
| `LasVariablesEstandarSeUsanSinDeclararlas` | las 11, su clase y `project_hash` (no `project_revision`) |
| `NoHayFormaDeObtenerUnPipelineQueNoHayaPasadoLaComprobacion` | `go/parser`: solo `Comprobar` devuelve `*Pipeline`; sin campos exportados; las consultas devuelven copias |
| `LosFallosDicenDondeEstan` | el texto exacto del error, con fichero, ambiente e invariante |

### 8.2 Las seis filas de la comprobación

Casos por invariante (**dominio 51 / ejemplo 62**; el ejemplo añade los que solo existen al leer disco):

| Invariante | Casos | Qué cubre |
|---|---|---|
| `formato` | 22 / 30 | `schema_version` ausente o distinta de la que se lee; claves que el formato no tiene (`clone_window`, `retries`, `resolve`); YAML ilegible; `scope` de salida desconocido; `scope` de paso desconocido (RD-04 §9.19); regla desconocida o repetida; `max_age` no-duración, en días, o negativo; comando sin `cmd`; `workdir` fuera del paso; plantilla ausente, fuera del paso o enlace; enlace fuera del paso; `outputs` sin `name` ni `probe`; aserción con `scope`; uso `${var.no-vale}`; variable sin `value`; ambiente sin `name`; `value` que no sirve como directorio; **solo ejemplo**: fichero suelto en `steps/`, `.txt` en `variables/`, subdirectorio en un ambiente, `steps/` o `variables/` que no son directorios |
| `pasos` | 6 / 7 | nombre que no sirve como fichero; `NN` que no son dos dígitos; orden repetido; nombre repetido ignorando mayúsculas; paso sin comandos (fichero ausente o vacío); pipeline sin pasos |
| `variables` | 14 / 15 | usada sin declarar (comando y plantilla); declarada en unos ambientes y no en otro; compartida que usa una salida de ambiente; comando que usa lo que él mismo produce; salida usada antes de producirse; literal que necesita una salida posterior; literal que usa un nombre inexistente; mismo nombre en compartido y en un ambiente; mismo nombre dos veces en un ámbito; directorio de `variables/` que no es de ningún ambiente; declarar o producir una variable estándar; círculo entre literales; **solo ejemplo**: un paso `scope: shared` que usa una variable de salida del ámbito de un ambiente (RD-04 §9.19) |
| `variables de salida` | 5 / 5 | regex mal formada; sin `probe`; producida dos veces por el mismo comando; mismo nombre por dos comandos del mismo ámbito; mismo nombre en dos ámbitos |
| `aserciones` | 1 / 1 | regex de una aserción mal formada |
| `ambientes` | 3 / 4 | sin `environments.yaml`; vacío; `name` repetido; `value` repetido ignorando mayúsculas |

### 8.3 Lectura (`infraestructura/lectura_test.go`)

| Prueba | Qué fija |
|---|---|
| `LeerLosFicherosDelPipeline` | todo el directorio del paso es material salvo su `commands.yaml` de raíz (un `terraform/commands.yaml` es un fichero más); la configuración del paso sale de `steps` en `config.yaml`, no de su directorio; bit de ejecución; enlaces; el directorio de `variables/` es el ámbito y el nombre del fichero solo organiza |
| `LoQueNoSeLeeNoSeDecideAlLeer` | una versión anterior **dice cuál es** aunque traiga claves muertas; `commands.yaml` vacío no declara nada; `Ilegibles` y `Desconocidos` en su sitio, sin fallos |

### 8.4 ACL sobre el Suministro real (`infraestructura/pipelines_test.go`)

Repositorio git temporal con el pipeline de ejemplo: `DeHoy` vs `DeUnCommit` (un rollback lee el pipeline de su
commit), el bit de ejecución sobrevive al repositorio, hashes distintos por commit, **el material se retira
siempre** (también cuando la comprobación falla) y la traducción de `ErrNoExiste` / `ErrInvalido`.

### 8.5 Servicio (`aplicacion/servicio_test.go`)

El pipeline traducido campo a campo al lenguaje publicado; los fallos llegan con su lista y su mensaje;
`ErrNoExiste`/`ErrInvalido` traducidos sin perder el detalle; las variables estándar se publican sin valores.

## §9 Tensiones de diseño (candidatos de refactor, por valor)

1. **`comprobacion.go` (749 líneas, un struct con 25 métodos y 6 campos de estado) hace las seis filas.**
   Extraer una unidad por fase (`ambientes`, `pasos`, `salidas`, `variables`, `usos`) sobre un `registroDeFallos`
   compartido daría SRP y, sobre todo, permitiría probar cada fase sin construir una `Declaracion` entera.
   **Restricción**: `Comprobar` tiene que seguir siendo la única función exportada que devuelve `*Pipeline`
   (hay una prueba con `go/parser` que lo verifica).
2. **La visibilidad está codificada en cinco sitios**: `Ambito.Ve`, `salidaProducida.laVe`, `declaradaVisible`,
   `seVe` y `choque`. Un solo tipo «vista desde un ámbito» con una consulta `Resuelve(nombre) (origen, bool)`
   colapsaría los cinco y haría trivial `revisar`/`necesita`. Es el refactor con mejor relación valor/riesgo.
3. **`falla(inv, fichero, paso, ambiente, formato, args…)`**: cinco posicionales, más de 40 llamadas, y dos de
   ellos son strings intercambiables (`paso` y `ambiente`). Un literal de struct o un pequeño builder elimina
   una clase entera de error que el compilador no ve.
4. **`usos(texto)` se recalcula por ambiente.** `usosEnLosPasos` recorre pasos × comandos × plantillas **una vez
   por ambiente**, y vuelve a pasar la regex sobre los mismos textos. Memoizar por texto es un cambio local;
   hoy no duele, pero el coste es `O(ambientes × material)`.
5. **El formato 1 está cableado en `lectura.go`** (tipos `*V1`, `leer` con cuatro fases fijas). Leer una versión
   anterior está fuera de alcance (§6 de la ficha), pero si algún día entra, el punto de extensión es una tabla
   `schema_version → lector`, no un `if` dentro de `decodificar`.
6. **`dominio` y `publicado` duplican siete structs y 50 líneas de traducción manual.** Es el precio del
   lenguaje publicado y **conviene pagarlo**; lo que falta es una prueba que falle si se añade un campo al
   dominio y se olvida en `traducirPaso` (reflexión sobre los nombres de campo).
7. **Detalles menores**: `configuracion` asigna `paso.EdadMaxima` aunque `max_age` haya fallado (inocuo hoy
   porque entonces no hay pipeline, pero es estado sucio); `outputs` registra en el índice de producción una
   salida cuyo nombre acaba de rechazar (hoy no arrastra fallos, pero es un derivado latente).

## §10 Huecos de prueba detectados

Ninguno es un fallo conocido; son ramas que hoy nadie ejecuta.

| Rama sin cubrir | Dónde |
|---|---|
| `config.yaml` ilegible **sin** poder leer su versión (`Version == nil && ilegible`) | `comprobacion.go:version` |
| `config.yaml` que existe y no es un fichero (enlace o directorio) → «no es un fichero» | `lectura.go:fichero` |
| `environments.yaml` ilegible (la guarda `!c.ilegible(fichero)` de `ambientes()`) | `comprobacion.go:ambientes` |
| enlace del material con destino **absoluto** (`path.IsAbs`); hoy solo se prueba `../../..` | `comprobacion.go:material` |
| entrada del material que no es fichero, directorio ni enlace → `Desconocidos` | `lectura.go:material` |
| `leer` devolviendo error de disco: la **única** razón por la que devuelve error | `lectura.go` |
| `Retirar` que falla ⇒ **no se entrega el pipeline** (lo promete RD-04 §9.8) | `pipelines.go:comprobar` |
| `publicar` con `(nil, nil)`: «el repositorio no devolvió pipeline ni error» | `aplicacion/servicio.go` |
| `publicado.FallosDeComprobacion.Error()` sin mensaje (rama de respaldo) | `publicado/tipos.go` |
| contexto cancelado en `DeHoy` / `DeUnCommit` | ACL |
| `publicado/` no tiene ningún fichero de prueba propio | — |
| `name` de ambiente repetido: está en la tabla del ejemplo y **falta en la del dominio** (3 / 4) | `comprobacion.go:ambientes` |

Candidatos a **casos nuevos de comprobación** (formas que el formato permite y nadie prueba): un pipeline con
un solo ambiente; una variable declarada que usa una salida **compartida** producida antes (camino feliz del
`necesita` transitivo); un `workdir: .` y un `workdir` vacío; una plantilla nombrada dos veces por el mismo
comando (hoy se deduplica en silencio); dos ficheros de variables de la raíz declarando nombres distintos;
un círculo de literales que pasa por tres variables y dos ficheros.
