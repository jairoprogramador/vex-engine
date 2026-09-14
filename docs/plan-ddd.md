# Plan de aplicación de DDD al motor de Vex

> Método: *Implementing Domain-Driven Design*, Vaughn Vernon (el libro rojo).
> Alcance: `ecosistema/vex-engine/` únicamente.
> Estado del plan: **cerrado** · Iteración en curso: — · Última cerrada: **IT-12** (2026-09-14)

---

## 1. Por qué existe este plan

El motor vex se esta rediseñando, hay trabajo estratégico hecho a mano (`modelo/dominio.md`, 
`modelo/bounded-contexts.md`, `modelo/lenguaje.md`), pero está sin contrastar contra el libro 
*Implementing Domain-Driven Design*, Vaughn Vernon y sin las piezas que el libro considera 
obligatorias antes de tocar nada táctico.

El análisis del código deja tres hechos que este plan tiene que atacar:

**1. El core domain no existe en código.** Se declara `Diagnóstico` como core y no hay ningún
paquete que lo implemente. Las 14.000 líneas viven en subdominios *supporting* y *generic*. Es
exactamente lo que el libro llama invertir el mejor diseño donde no está la ventaja
competitiva.

**2. El código está partido por cadenas, no por contextos.** `internal/domain/command/`
contiene `Execution`, `ExecutionContext`, `Step`, `StepName`, `Variable`, `Origin`… Es un
*shared kernel accidental*: no nació de una decisión de dominio sino del grafo de imports de
Go (el ciclo `record → deployment → step`, resuelto duplicando el puerto `FactSink` tres
veces). Cuatro de los once contextos declarados están repartidos entre tres y cuatro paquetes.

**3. Falta el Context Map entero.** No hay un documento que diga qué patrón de relación une a
dos contextos. Las relaciones existen y están resueltas dentro de
`internal/interfaces/cli/factory.go` a base de aserciones de contrato
(`var _ stepDom.AnchorLookup = deploymentDom.RollbackAnchor{}`, línea 177). Eso es un context
map escrito en el sitio equivocado.

---

## 2. Qué es el producto terminado

Un **modelo de dominio cerrado y justificado**, y un **catálogo de unidades de rediseño**
implementables y ordenadas por dependencias.

La implementación queda **fuera** de este plan. Lo que este plan entrega es todo lo que hay que
saber para implementar sin volver a discutir.

---

## 3. Encuadre decidido

| Decisión | Valor | Consecuencia |
|---|---|---|
| Alcance | Solo `vex-engine` | CLI, portal y backend son sistemas externos en el Context Map |
| Ambición | Rediseño profundo | La estructura de paquetes es materia de diseño, no un dato de partida |
| Orden táctico | Diagnóstico primero | El core dicta lo que los supporting deben publicar |
| Las tres cadenas | Negociables | El Chain of Responsibility se justifica o cae |
| Proceso | Sustituye al catálogo de specs | El formato nuevo debe seguir lo que dicte el libro de Vaughn Vernon  |
| Preguntas | Una a la vez, orientada a lo que se quiere lograr, y solo donde algo no encaja *(IT-03 `DEC-03.15`)* | Cada iteración tiene apertura y cierre |
| Didáctica | Documento aparte | `guia-ddd.md`, que crece etapa a etapa |

**Sobre el catálogo de specs.** Las specs `00`–`28` de `/Vex/specs/` quedan como registro
histórico: **no se editan nunca**. El catálogo nuevo que
produce E12 se escribe en el formato de este proceso, heredando de ellas lo que funciona

---

## 4. Mapa de archivos

```
docs/
├── plan-ddd.md                    ← este documento + el tablero
├── guia-ddd.md                    ← la didáctica: los patrones de Vernon sobre este dominio
├── modelo/                        ← el modelo vivo; cada iteración lo actualiza
│   ├── dominio.md
│   ├── bounded-contexts.md
│   ├── lenguaje.md               (reescrito en E2)
│   ├── context-map.md             (nace en E4)
│   ├── arquitectura.md            (nace en E5)
│   ├── migracion.md               (nace en E11)
│   └── contextos/<contexto>.md    (nacen en E6–E10)
├── iteraciones/
│   └── IT-NN-<enunciado>.md       ← uno por iteración; no se edita tras cerrarse
└── rediseno/
    ├── README.md                  (nace en E12)
    └── RD-NN-<enunciado>.md
```

Dos clases de documento, con reglas opuestas:

- **`iteraciones/`** es *historia*. Un archivo cerrado no se vuelve a tocar: es el registro de
  por qué se decidió lo que se decidió, con la información que había entonces.
- **`modelo/`** es *presente*. Se reescribe al cerrar cada iteración y siempre dice lo que es
  verdad hoy. Si contradice a una iteración vieja, gana el modelo — y la iteración nueva
  nombra la `DEC-` que deroga.

---

## 5. Las etapas

Una iteración por etapa, en vueltas cortas (§6).

### Bloque A — Estratégico

#### E1 — Destilación del dominio
**Objetivo**: cerrar la clasificación core / supporting / generic con su razón económica, y
fijar los criterios de éxito de todo el ejercicio.
**Entregable**: `modelo/dominio.md` reescrito.
**Cierre**: cada subdominio tiene propósito en una frase, clasificación, razón, y qué se pierde
si se clasifica mal.

#### E2 — Lenguaje ubicuo por contexto
**Objetivo**: partir el glosario único en un lenguaje *por contexto*. En IDDD el lenguaje vive
dentro de un bounded context, no por encima de todos ellos. Cazar homónimos y sinónimos.
El material ya lo exige: la spec 17 §5.6 se titula «Dos cosas llamadas *step*»; «variable»
tiene al menos tres sentidos; «estado» nombra el `state/` de un step y el ciclo de vida de la
`Execution`; «registro» nombra al contexto y al `state.StepRecord`.
**Entregable**: `modelo/lenguaje.md` reorganizado + tabla de homónimos con su resolución.
**Cierre**: ningún término con dos significados sin desambiguar, ni dos términos para lo mismo.

#### E3 — Frontera de los contextos
**Objetivo**: validar, fusionar o partir los once contextos declarados, y justificar por qué
cada superviviente merece existir separado. Contraste obligado: un desarrollador, un binario
one-shot, 14.000 líneas — once contextos es candidato claro a sobre-descomposición.
*(Enmendado en IT-03, `DEC-03.2`: en un proyecto que empieza de cero, el ideal es un contexto por
subdominio, y la carga de la prueba pesa del lado de fusionar. Resultado: ocho contextos.)*
**Entregable**: `modelo/bounded-contexts.md` con propósito, términos propios y prueba de
separación de cada uno.
**Cierre**: cada contexto tiene escrita su prueba: «si lo fusionara con X, se rompería esto».

#### E4 — Context Map
**Objetivo**: lo que falta entero. Por cada par que se integra: dirección upstream/downstream,
patrón de relación y contrato.
Casos ya sobre la mesa: los tres puertos `FactSink` que implementa un solo `record.Facts`; el
ciclo `record → deployment → step` resuelto duplicando puertos; `deployment.RollbackAnchor`
satisfaciendo `step.AnchorLookup`; `RequestInput` como contrato con el CLI externo; y las cinco
`SPEC-*.md` que viven junto al código, que son Published Language de manual.
**Entregable**: `modelo/context-map.md`.
**Cierre**: ninguna pareja se comunica sin patrón declarado; todo ciclo tiene resolución
escrita.

#### E5 — Arquitectura objetivo
**Objetivo**: decidir la arquitectura donde vive el modelo. Hexagonal ya está y funciona; las
preguntas abiertas son otras. ¿El event sourcing de `record` es *el modelo* del contexto
Registro o un mecanismo de persistencia? ¿CQRS explícito para el read-side de `vexd record`?
¿Un módulo Go por contexto, o por capa dentro de cada contexto? ¿Cómo se hablan los contextos
dentro de un mismo proceso? ¿Qué papel le queda a `ExecutionContext`, el God Object que
`vex-plan-revision.md` §A-1 ya señala?
**Entregable**: `modelo/arquitectura.md` + árbol de paquetes objetivo.
**Cierre**: existe una regla de dependencias verificable mecánicamente.

### Bloque B — Táctico, un contexto por iteración

Criterio de cierre común: el contexto tiene escritos sus **agregados con sus invariantes** —o por qué no los tiene *(IT-06 `DEC-06.14`)*— (y
qué transacción los protege), sus **entidades y value objects**, sus **eventos de dominio**,
sus **servicios de dominio**, sus **repositorios y factorías**, sus **servicios de aplicación**
y sus **puertos** hacia otros contextos.

#### E6 — Diagnóstico *(el core, en terreno limpio)*
Se empieza por los **escenarios** que el contexto responde, no por las entidades. Es lo que
manda el libro, y aquí importa el doble porque no hay código que sesgue el modelo.
Efecto lateral buscado: lo que Diagnóstico necesite se convierte en el requisito *upstream* de
Registro y de Definición, en vez de adivinarlo.
**Riesgo declarado**: es el contexto del que menos se sabe. Si sale vacío, la clasificación de
E1 estaba mal y se vuelve a E1. Está permitido y está escrito.

#### E7 — Registro de Despliegue
Hoy repartido en `deployment` (1.505 líneas), `record` (2.188), `state` (538) y `cache` (95).
¿Es un contexto o dos — la intención (`deployment.Content`) y la circunstancia (`record.Event`)
tienen modelos distintos? ¿`state.StepRecord` pertenece aquí o a Ejecución de Step? ¿Dónde cae
la frontera con Sincronización de Estado? ¿`cache` es dominio o infraestructura, si «no
participa en ninguna decisión»?

#### E8 — Definición de Pipeline y Resolución de Variables
Los dos más dispersos. Definición vive en `pipeline` (manifest, reglas estructurales), en
`step` (`StepConfig`, `RuleSet`, `VariableDeclaration`) y en `infrastructure/step` (repos
YAML). Resolución vive en `command` (`Variable`, `Origin`, `ExecutionVariableMap`) y en `step`
(`DeclarationResolver`).
¿La precedencia por `Origin` es de Resolución o de Ejecución? ¿Las reglas de re-ejecución son
de Definición o de Orquestación? ¿De quién es `fingerprint`, hoy un paquete sin dueño del que
dependen tres contextos?

#### E9 — Orquestación de Pipeline y Ejecución de Step
El grande: ~8.000 líneas y las tres cadenas. ¿Qué agregado protege qué invariante? ¿Es
`Execution` la raíz de Orquestación? ¿El Chain of Responsibility sobrevive como patrón de
aplicación por debajo de los agregados, o cae? ¿Cómo se parte la regla «el mismo eslabón
decide, hashea y escribe» que `vex-plan-revision.md` §A-2 denuncia?

#### E10 — Contextos genéricos y Simulación
Suministro del Proyecto, Suministro del Pipeline, Espacio de Trabajo, Sincronización de Estado
y Simulación (greenfield). Por cada genérico: ¿se aísla tras un ACL, se sustituye, o se deja?
Para Simulación: ¿comparte modelo con Orquestación, o lo conforma?

### Bloque C — Realización

#### E11 — Del modelo al código: estrategia de migración
Cómo se llega de los paquetes de hoy a los del modelo, se tiene permitido romper el motor actual
con tal de llegar al modelo. La red de seguridad
(existe el arnés de integración en `internal/interfaces/cli/harness_test.go`, pero §A-8 dice
«cero red de seguridad»: hay que decidir qué invariantes se fijan en tests *antes* de mover
nada); orden de migración por dependencias; qué se estrangula y qué se reescribe; qué se hace
con las cinco `SPEC-*.md` normativas, que al ser Published Language no se tocan a la ligera.
**Entregable**: `modelo/migracion.md`, que incluye la **reconciliación con las specs `00`–`28`**:
cuáles quedan derogadas por el modelo y cuáles siguen en pie. Es el único sitio donde se hace esa
cuenta (regla enmendada en IT-01).

#### E12 — El catálogo del rediseño
Traducir el modelo cerrado a unidades implementables y ordenadas.
Formato heredado: §1 Problema · §2 Por qué importa · §3 Objetivo · §4 Alternativas · §5
Solución · §6 Alcance · §7 Verificación · §8 Decisiones · §9 Hallazgos al implementar. Se
retiran las subsecciones 5.1'/5.2'/5.3' (DDD/SOLID/Patrones): en este proceso el DDD ya no es
una sección que se justifica al final, es el cuerpo del documento.
**Entregable**: `rediseno/README.md` con el grafo de dependencias + una ficha por unidad.
**Cierre**: cada unidad tiene problema, objetivo, alcance, verificación y dependencias, y el
grafo no tiene ciclos.

---

## 6. Reglas del proceso

**Numeración.** `Q-<etapa>.<n>` para preguntas y `DEC-<etapa>.<n>` para decisiones.
Deliberadamente **no** `D-` (colisiona con los defectos D1–D14 de `vex-guia-logica.md` §9.2) ni
`P-` (colisiona con los diseños pendientes P1–P17).

**Cómo se itera** *(enmendado en IT-03, `DEC-03.15`; sustituye a la puerta, a las fases de
apertura, respuesta y validación, y al umbral de ~12 preguntas)*.

1. **Se parte de lo que se quiere, no de un inventario.** El experto dice qué espera del producto;
   Claude lo convierte en modelo, lo contrasta con el libro y **propone** las decisiones con su
   porqué. **A partir de las respuestas, Claude va definiendo contextos y lenguaje y encuentra las
   incongruencias; las incongruencias se analizan juntos.**
2. **Solo se pregunta donde hay una incongruencia**: dos cosas del modelo que no pueden ser verdad a
   la vez, o una decisión que solo el experto puede tomar. Cada pregunta está **enfocada en lo que se
   espera**: la incongruencia en dos líneas, una **propuesta recomendada** y, si la hay, la
   alternativa que cambiaría el resultado.
   **Una a la vez**: se plantea, se espera la respuesta, se aplica al modelo, y solo entonces viene
   la siguiente. El plan y las etapas no cambian; cambia el proceso.
3. **Lo que no es incongruencia se decide por defecto** y se anota como tal. El experto lo revierte si
   no le sirve; revertir sale barato porque está escrito.
4. **Vueltas cortas.** Cada respuesta se aplica al modelo en la misma vuelta. La iteración se cierra
   cuando una vuelta ya no deja incongruencias que muevan lo que decide la etapa. Lo que quede sin
   moverlo se difiere con nombre, en §6 del archivo.
5. **El archivo de iteración registra las decisiones y su porqué**, no las opciones que nadie eligió.

**El contraste con el libro no se negocia.** La velocidad sale de no convertirlo en un cuestionario,
no de saltárselo.

**Inmutabilidad.** Un archivo de iteración cerrado no se vuelve a editar. Lo que cambia es
`modelo/`.

**Contra el código.** El código es **espacio de la solución** y **no es material de las iteraciones
estratégicas** (E2–E5): los términos que existen en él no se toman en cuenta al decidir cómo se
llaman las cosas. Una sola excepción, acotada: el código puede usarse para **detectar** una
ambigüedad —un homónimo se manifiesta donde se usa— pero **no para resolverla**. Dejarlo presente
mientras se modela hace que lo construido dirija lo que se modela, porque una palabra que ya existe
siempre pesa más que una que hay que inventar.
*(Regla nueva en IT-02, `DEC-02.10`.)*

**Contra las specs.** El modelo se diseña **siguiendo el libro, no reconciliándose con lo
implementado**: las specs `00`–`28` no son restricción del modelado, y muchas quedarán derogadas.
Averiguar cuáles es un entregable **único de E11**, no una contabilidad por decisión — que era lo
que hacía que lo ya construido dirigiera lo que se modela. Las specs no se editan nunca.
*(Enmendado en IT-01, `Q-01.15`.)*

**Plantilla del archivo de iteración** *(aligerada en IT-03, `DEC-03.15`)*.

```markdown
# IT-NN — <enunciado>

> Etapa: E-NN · Estado: abierta | cerrada
> Abierta: <fecha> · Cerrada: <fecha>
> Lectura previa: guia-ddd.md §<n>

## 1. Qué se quiere — en palabras del experto, y qué tiene que ser verdad al cerrar
## 2. Propuesta — el modelo que sale, contrastado contra el libro *(en E2–E5, solo con `modelo/`: `DEC-02.10`)*
## 3. Incongruencias — Q-NN.n: la incongruencia · propuesta · respuesta
## 4. Decisiones — DEC-NN.n: decisión · por qué · qué descarta · cómo se verifica *(las que se toman por defecto lo dicen)*
## 5. Impacto en el modelo — qué documentos de modelo/ cambian
## 6. Dudas diferidas — cuáles, a qué iteración y por qué
```

---

## 7. Tablero

| Etapa | Iteración | Archivo | Estado | Q resp./total | DEC | Cerrada |
|---|---|---|---|---|---|---|
| E1 Destilación del dominio | IT-01 | `iteraciones/IT-01-destilacion-del-dominio.md` | **cerrada** | 30/30 *(7 rondas)* | 14 | 2026-08-26 |
| E2 Lenguaje ubicuo por contexto | IT-02 | `iteraciones/IT-02-lenguaje-ubicuo-por-contexto.md` | **cerrada** | 37/37 *(5 rondas)* | 18 | 2026-08-27 |
| E3 Frontera de los contextos | IT-03 | `iteraciones/IT-03-frontera-de-los-contextos.md` | **cerrada** | 27/27 *(3 rondas)* | 18 | 2026-09-13 |
| E4 Context Map | IT-04 | `iteraciones/IT-04-context-map.md` | **cerrada** | 4/4 | 10 *(5 por defecto)* | 2026-09-13 |
| E5 Arquitectura objetivo | IT-05 | `iteraciones/IT-05-arquitectura-objetivo.md` | **cerrada** | 2/2 | 10 *(8 por defecto)* | 2026-09-14 |
| E6 Diagnóstico (core) | IT-06 | `iteraciones/IT-06-diagnostico.md` | **cerrada** | 10/10 | 20 *(9 por defecto)* | 2026-09-14 |
| E7 Registro de Despliegue *(hoy: Historial)* | IT-07 | `iteraciones/IT-07-historial.md` | **cerrada** | 3/3 | 9 *(6 por defecto)* | 2026-09-14 |
| E8 Definición y Variables | IT-08 | `iteraciones/IT-08-definicion-y-resolucion.md` | **cerrada** | 2/2 | 8 *(6 por defecto)* | 2026-09-14 |
| E9 Orquestación y Ejecución de Step *(hoy: Ejecución de Pipeline)* | IT-09 | `iteraciones/IT-09-ejecucion.md` | **cerrada** | 1/1 | 8 *(7 por defecto)* | 2026-09-14 |
| E10 Genéricos y Simulación *(y Lanzamiento)* | IT-10 | `iteraciones/IT-10-genericos-simulacion-lanzamiento.md` | **cerrada** | 3/3 | 8 *(5 por defecto)* | 2026-09-14 |
| E11 Estrategia de migración | IT-11 | `iteraciones/IT-11-migracion.md` | **cerrada** | 1/1 | 6 *(4 por defecto)* | 2026-09-14 |
| E12 Catálogo del rediseño | IT-12 | `iteraciones/IT-12-catalogo.md` | **cerrada** | 2/2 | 7 *(5 por defecto)* | 2026-09-14 |

---

## 8. Cómo se verifica que el proceso se está cumpliendo

No hay código que ejecutar. Lo que se verifica es la disciplina.

- **Al cerrar cada iteración**: cero `Q` sin respuesta o sin diferimiento razonado; `modelo/`
  actualizado; tablero al día.
- **E3**: cada contexto superviviente tiene su prueba de separación escrita.
- **E4**: el grafo de contextos no tiene ninguna arista sin patrón, y todo ciclo tiene
  resolución.
- **E5**: la regla de dependencias del árbol objetivo se puede comprobar mecánicamente (script
  de imports o `depguard`). Se escribe el comando aunque no se ejecute todavía.
- **E9**: el destino de las tres cadenas está decidido con argumento escrito, en cualquiera de
  los dos sentidos.
- **E12**: el grafo de dependencias del catálogo no tiene ciclos y cada unidad es implementable
  por sí sola.
- **De fondo, en todas**: cada término que aparece en cualquier documento de `modelo/` está
  definido en `modelo/lenguaje.md` bajo un contexto concreto.
