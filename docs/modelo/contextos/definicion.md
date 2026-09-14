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
| **Ejecución** | los pasos en orden, con sus comandos y su material · las formas de *regla* y de *variable de salida* de cada paso · el orden de los ambientes, para dejarlo en el Historial |
| **Resolución** | las variables declaradas · las formas de *ámbito* y de *variable de salida* |
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
| variables | toda variable usada en un comando o en el material de un paso está declarada, como variable de salida de un paso anterior o como variable declarada, y es **visible según el ámbito escrito**, en cada ambiente |
| variables de salida | sus expresiones regulares son correctas |
| ambientes | tienen un orden |

### Entidades y value objects

| | Qué es |
|---|---|
| **paso** *(entidad)* | un nombre, que es su identidad (`DEC-03.7`), sus comandos, el material de su directorio y su configuración |
| **comando** | lo que un paso manda ejecutar |
| **material de un paso** | lo que el DevOps escribe en el directorio del paso para que lo usen sus comandos: plantillas, manifiestos, ficheros de la tecnología. Forma parte de las **instrucciones** del paso (`DEC-08.7`) |
| **variable declarada** | un nombre y su valor escrito, para un paso, y para un ambiente o para el ámbito compartido |
| **ambiente** | un nombre y su lugar en el orden |
| **formas declaradas** | *regla*, *ámbito*, *variable de salida* (nombre y expresión regular) y *orden de los ambientes*. Su significado es de quien las aplica |

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
comandos, su material y sus formas; las variables declaradas; los ambientes en orden. O los fallos de la
comprobación.

---

## Los ficheros del pipeline *(IT-12 `DEC-12.6`, `DEC-12.7`)*

**Cómo los escribe el DevOps.** Traducirlos al agregado es trabajo del ACL.

| Fichero | Qué declara |
|---|---|
| `vexpipeline.yaml` | `schema_version: 3`. Una versión anterior se rechaza, diciendo qué cambió |
| `environments.yaml` | los ambientes, en orden: `name`, `description`, `value` |
| `steps/NN-<paso>/commands.yaml` | los comandos: `name`, `description`, `cmd`, `workdir`, `show`, `templates` y `outputs` (variables de salida: `name`, `description`, `probe`) |
| `steps/NN-<paso>/config.yaml` | la configuración del paso: `scope: environment \| shared` · `rules: [code, instructions, variables]` · `max_age` |
| `steps/NN-<paso>/…` | todo lo demás es **material** del paso |
| `variables/<value>/<paso>.yaml` | variables declaradas de un paso de ámbito `environment`, una por ambiente: `value`, o `resolve: step-output` con `from` y `key` |
| `variables/<paso>.yaml` | variables declaradas de un paso de ámbito `shared`, una sola vez |

- **Identidad y orden.** Un paso se identifica por su nombre sin `NN-`, y `NN` es solo el orden.
- **Instrucciones.** Son los comandos y el material. `config.yaml` no forma parte de ellas (`DEC-08.7`).
- **Interpolación.** Solo se interpola el material listado en `templates`, y el resto se copia tal cual.
- **Valores por defecto.** Sin `rules`, un paso mira las tres cosas. Sin `max_age`, no caduca. Sin `scope`,
  su ámbito es `environment`.
- **La comprobación rechaza además**:
  - un paso `shared` con variables por ambiente;
  - un paso `environment` con `variables/<paso>.yaml`;
  - una regla desconocida;
  - una `schema_version` distinta de 3.

