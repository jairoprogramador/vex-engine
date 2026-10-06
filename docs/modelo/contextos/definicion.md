# Definición de Pipeline — modelo del contexto

> **Supporting.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; sus relaciones, en `context-map.md` (filas #7, #8, #9 y #12).
>
> **Vigente** desde el cierre de IT-08 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Cómo se despliega un producto**: sus pasos con sus comandos, sus variables, sus ambientes en orden, y si
lo escrito está **bien formado y bien referenciado**. **Declara; no ejecuta ni resuelve**: lo que declara
lo aplican otros contextos (`DEC-03.6`).

---

## Lo que le piden

| Quién | Qué necesita |
|---|---|
| **Ejecución** | los pasos en orden, con sus comandos, su material y su ámbito propio · las formas de *regla*, de *variable de salida* y de *aserción* de cada comando · el orden de los ambientes, para dejarlo en el Historial |
| **Resolución** | las variables declaradas, cada una con su ámbito · el ámbito propio de cada paso — qué ve y bajo cuál busca su última vez · las formas de *ámbito* y de *variable de salida* |
| **Simulación** | el pipeline comprobado entero |

Y lee de **Suministro** el pipeline como fuente: el de hoy o el de un commit, para un rollback (`DEC-03.9`).

---

## Modelo táctico

### Agregado: **Pipeline**

**Inmutable, y nace comprobado** (`DEC-08.2`). El motor nunca escribe un pipeline, porque lo escribe el DevOps en
su repositorio. Por eso el agregado no protege cambios: protege que **no exista un pipeline mal formado**.
La comprobación es su factoría: o sale un pipeline válido, o sale la lista de lo que falla.

| Invariante | Qué comprueba |
|---|---|
| formato | lo declarado tiene la forma que se espera |
| pasos | nombres únicos, un orden único y **ningún paso sin comandos** |
| variables | toda variable usada en un comando, en el material de un paso o en un valor declarado es una **variable estándar**, o está declarada en un **ámbito que se vea desde donde se usa**, o la produce un comando; y lo que hace falta para resolverla está **producido antes** de usarse, en cada ambiente |
| variables de salida | sus expresiones regulares son correctas, y un nombre se produce en un solo sitio de su ámbito |
| aserciones | sus expresiones regulares son correctas |
| ambientes | tienen un orden |

### Entidades y value objects

| | Qué es |
|---|---|
| **paso** *(entidad)* | un nombre, que es su identidad (`DEC-03.7`), sus comandos, el material de su directorio y su configuración: sus reglas, su edad máxima y su propio ámbito |
| **comando** | lo que un paso manda ejecutar |
| **material de un paso** | lo que el DevOps escribe en el directorio del paso para que lo usen sus comandos: plantillas, manifiestos, ficheros de la tecnología. Forma parte de las **instrucciones** del paso (`DEC-08.7`) |
| **variable declarada** | un nombre y su valor escrito, que pertenece a un **ámbito**: el de un ambiente, o el compartido. **No es de un paso**: la ven todos los pasos que se ejecutan en su ámbito |
| **variable de salida** | un nombre y la expresión regular con la que se saca su valor de la salida de un comando. También pertenece a un ámbito: si no lo declara, el de su paso — el del ambiente en ejecución, salvo que el paso declare el suyo propio compartido —, o el que declare explícitamente |
| **aserción** | una expresión regular que la salida de un comando tiene que cumplir para que el comando se dé por bueno. **No produce ninguna variable**, y por eso no tiene ámbito |
| **variable estándar** | una variable que el pipeline usa sin declararla, porque el motor garantiza que está siempre. Es un **metadato**, que da quien invoca, o una **generada** por el motor; compartida, o **del paso**, que solo vale mientras el paso se ejecuta. Definición conoce su nombre, no su valor (RD-04 §9) |
| **ambiente** | un nombre y su lugar en el orden |
| **formas declaradas** | *regla*, *ámbito*, *variable de salida*, *aserción* y *orden de los ambientes*. Su significado es de quien las aplica |

### Servicio de dominio

**Comprobación**: recibe lo que trae la fuente y devuelve un **Pipeline** o la lista de fallos. Es la
factoría del agregado.

### Repositorio

**Pipelines**: obtiene el pipeline de hoy o el de un commit. Lo implementa el ACL hacia Suministro
(`context-map.md` fila #12), que convierte un repositorio en una declaración (`guia-ddd.md` §12: un
repositorio puede apoyarse en otro contexto).

### Eventos de dominio

Ninguno.

### Lo que publica

**El pipeline comprobado**, como Published Language versionado (`DEC-04.3`): los pasos en orden, con sus
comandos, su material y sus formas; las variables declaradas, cada una con su ámbito; los ambientes en orden.
O los fallos de la comprobación. Y las **variables estándar**, por nombre y clase, para que quien les da valor
use la misma lista.

---

## Los ficheros del pipeline *(IT-12 `DEC-12.6`, `DEC-12.7`)*

**Cómo los escribe el DevOps.** Traducirlos al agregado es trabajo del ACL.

| Fichero | Qué declara |
|---|---|
| `config.yaml` | `schema_version: 1`. Una versión distinta se rechaza, diciendo cuál se espera. Además, bajo `steps`, la configuración de cada paso por su nombre: `rules: [code, instructions, variables]` · `max_age` · `scope: environment \| shared`, los tres opcionales — el ámbito **propio** del paso: decide qué ve, bajo qué ámbito archiva su historia, y qué hereda por defecto lo que produce sin `scope` propio. Sin `scope`, es el que representa al ambiente en que se ejecuta. Un paso ausente de `steps` usa los tres valores por defecto (RD-04 §9.20) |
| `environments.yaml` | los ambientes, en orden: `name`, `description`, `value` |
| `steps/NN-<paso>/commands.yaml` | los comandos: `name`, `description`, `cmd`, `workdir`, `templates` y `outputs`. Cada `outputs` con `name` es una **variable de salida** (`name`, `description`, `probe`, `scope: environment \| shared`); sin `name` y con `probe` es una **aserción**; sin ninguno de los dos no declara nada |
| `steps/NN-<paso>/…` | todo lo demás es **material** del paso |
| `variables/<value>/*.yaml` | variables del **ámbito de ese ambiente**: `name`, `description` y `value` |
| `variables/*.yaml` | variables del **ámbito compartido**, declaradas una sola vez |

- **Identidad y orden.** Un paso se identifica por su nombre sin `NN-`, y `NN` es solo el orden.
- **Un valor declarado es un literal**, que puede usar otras variables visibles, también las de salida:
  `value: ${var.registro_servidor}` es cómo se le da otro nombre a lo que produce un paso *(RD-04 §9)*.
- **El directorio es el ámbito, y el nombre del fichero solo organiza.** `variables/prod/redes.yaml` declara
  variables del ámbito `prod`, no de un paso llamado `redes`: dentro de un ámbito se pueden repartir en los
  ficheros que se quieran.
- **Instrucciones.** Son los comandos y el material. La configuración de un paso, en `config.yaml`, no forma
  parte de ellas (`DEC-08.7`).
- **Interpolación.** Solo se interpola el material listado en `templates`, y el resto se copia tal cual.
- **Valores por defecto.** Sin `rules`, un paso mira las tres cosas. Sin `max_age`, no caduca. Sin `scope`
  para un paso en `config.yaml` (o sin entrada de `steps` para él), el paso es del ámbito del ambiente en que
  se ejecuta. Sin `scope` en un `outputs`, lo que produce hereda el ámbito de su paso.
- **Visibilidad** *(RD-04 §9, precisado en §9.19)*. Un paso ve desde **su propio ámbito**: las variables
  estándar siempre, las declaradas visibles desde ese ámbito, y las variables de salida **ya producidas** que
  se ven desde él. Sin `scope` propio, su ámbito es el que representa al ambiente en que se ejecuta, así que
  ve las declaradas de ese ambiente y las compartidas, y las salidas de los comandos anteriores del mismo
  paso y de los pasos anteriores — el comportamiento de siempre. Con `scope: shared`, su ámbito es el
  compartido, y por eso ve **solo** lo compartido: ni una variable declarada en un ambiente, ni una variable
  de salida que no sea `shared`, aunque la produzca un paso anterior. **Lo compartido no ve lo del
  ambiente**: un valor compartido que dependiera de un ambiente dejaría de ser el mismo en todos, y un paso
  de `scope: shared` que dependiera de uno tendría el mismo problema.
- **La comprobación rechaza además**:
  - un `outputs` sin `name` ni `probe`, y una aserción con `scope`;
  - un mismo nombre declarado en el ámbito compartido y en el de un ambiente, o dos veces en un mismo ámbito;
  - un mismo nombre producido en dos sitios que se ven a la vez;
  - una variable usada antes de que se produzca lo que hace falta para resolverla;
  - una regla desconocida y un `scope` desconocido, en un `outputs` o en la configuración de un paso;
  - una `schema_version` distinta de la que se lee;
  - `config.yaml` declarando, bajo `steps`, la configuración de un nombre que no es el de ningún paso
    (RD-04 §9.20);
  - declarar o producir una variable estándar *(RD-04 §9)*.

