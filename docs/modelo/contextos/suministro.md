# Suministro de Fuentes — modelo del contexto

> **Generic.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; sus relaciones, en `context-map.md` (filas #10, #11 y #12).
>
> **Vigente** desde el cierre de IT-10 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Poner delante el material** (el código del producto y el pipeline), como está hoy, como estaba en un
commit o como está en una **copia de trabajo**, y **decir su hash** (`DEC-10.6`).

---

## Lo que le piden

| Quién | Qué necesita |
|---|---|
| **Ejecución** | el material de hoy, de un commit o de una copia de trabajo (`DEC-10.7`) · el hash del código · el commit con el que trabaja |
| **Definición** | el pipeline como fuente, el de hoy o el de un commit |
| **Simulación** | el pipeline que se quiere simular: una copia de trabajo o un commit (`DEC-10.6`) |

---

## Qué se compra y qué se escribe

| Pieza | Decisión |
|---|---|
| **el acceso a los repositorios**: traer el material de hoy o de un commit, y saber con qué commit se trabaja | **se compra**, detrás del ACL de Suministro hacia los repositorios (`DEC-10.2`) |
| **el hash del código y el del pipeline** | **lo mínimo**: la única regla es que cambie si cambia el contenido, y qué entra es técnica (`DEC-02.18`). Si algo que se compre cumple esa regla, se usa |

---

## Modelo táctico

**Ni entidades ni agregados**: Suministro no cambia nada, solo trae.

| | Qué es |
|---|---|
| **fuente** *(value object)* | el repositorio del producto o el del pipeline |
| **commit** *(value object)* | un punto de la historia de una fuente con el que se vuelve a tener delante el material |
| **hash** *(value object)* | el del código o el del pipeline: cambia si cambia el contenido |
| **copia de trabajo** *(value object)* | el directorio donde alguien está trabajando, con cambios sin commit. Tiene hash, pero no commit |
| **material** *(value object)* | lo que se pone delante, fijo para quien lo pide hasta que lo retira |
| **traer una fuente** *(servicio de aplicación)* | la de hoy, la de un commit o una copia de trabajo, con su hash y, si lo hay, su commit. El hash se calcula sobre lo que quedó delante |
| **repositorios** *(puerto)* | hacia lo que se compra: pone delante el material |
| **hashes** *(puerto)* | hacia lo mínimo que se escribe: el hash de lo que quedó delante, el mismo sea cual sea el acceso (RD-03 §9) |

Eventos: ninguno.

