# Migración — del motor de hoy al modelo

> **Espacio de la solución.** Cómo se llega al motor que describen `modelo/` y `modelo/contextos/`.
>
> **Vigente** desde el cierre de IT-11 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Estrategia: reescritura, no estrangulamiento

**El motor se escribe desde cero, en paquetes nuevos** (`DEC-11.1`). El código de hoy sirve solo como idea
general de la lógica: no se adapta, no se estrangula por partes y no se mantiene mientras tanto, así que
puede romperse. **Al final se borra.**

---

## Dónde vive el código nuevo mientras se escribe

En su **sitio definitivo**, el árbol de `arquitectura.md`, sin compartir nada con el código antiguo
(`DEC-11.2`). **Enmienda al empezar RD-01** (2026-09-14, a pedido del usuario): el código antiguo se
aparta a `old-internal/`, y `internal/` queda solo para lo nuevo.

| Qué | Dónde |
|---|---|
| los ocho contextos | `internal/<contexto>/` |
| el borde | `internal/borde/` |
| la raíz de composición | `cmd/motor/` mientras exista el antiguo; pasa a `cmd/vexd/` al borrarlo |
| el código antiguo | `old-internal/`, con sus importaciones reescritas, y `cmd/vexd/`. Sigue compilando y se puede seguir publicando hasta RD-12 |

**La regla de dependencias funciona desde el primer paquete**, y es el mismo comando de `arquitectura.md`
sobre `./...`: el awk ignora lo que no está bajo `internal/`, y **rechaza que el código nuevo importe algo
del módulo fuera de `internal/`** (RD-01 §9). La integración continua lo corre en
`.github/workflows/reglas.yml`.

---

## Red de seguridad

Se prueba **el modelo nuevo**, no el código de hoy (`DEC-11.3`). El arnés de integración actual prueba un
cableado que va a desaparecer, y no se reutiliza.

| Qué se prueba | Cómo | Desde cuándo |
|---|---|---|
| **la regla de dependencias** | el comando de arriba, en la integración continua | el primer paquete |
| **las invariantes por construcción** de cada contexto | pruebas unitarias del dominio: las listas de invariantes de cada `contextos/<contexto>.md` y sus verificaciones | con cada agregado, value object y servicio de dominio |
| **los escenarios** de cada contexto (ES, EJ, SIM, LAN y los del Historial) | pruebas de sus servicios de aplicación con adaptadores en memoria | con cada servicio de aplicación |
| **un intento de punta a punta** | el motor entero, con un almacén y un repositorio locales | desde el primer corte que funciona (paso 6 del orden) |

---

## Orden

**Por dependencias**: primero los contextos de arriba del context map, para que cada uno se construya contra
algo que ya existe (`DEC-11.4`).

| # | Unidad | Depende de | Qué deja funcionando |
|---|---|---|---|
| 1 | **Esqueleto** | — | el árbol vacío, la raíz `cmd/motor/` y la regla de dependencias en la integración continua |
| 2 | **Historial** | 1 | sus cinco agregados, sus repositorios sobre un almacén local, la ocupación y el evento *despliegue registrado* |
| 3 | **Suministro** | 1 | traer una fuente de hoy, de un commit o de una copia de trabajo, con su hash, sobre un acceso local a repositorios |
| 4 | **Definición** | 3 | el Pipeline comprobado |
| 5 | **Resolución** | 2, 4 | las variables de un intento, la interpolación y el hash simple |
| 6 | **Ejecución** y un **borde mínimo** | 2, 3, 4, 5 | *decidir un paso* e *intentar*. **Primer corte que funciona de punta a punta**: intentar en local |
| 7 | **Lanzamiento** | 2 | escuchar el evento, lanzar, reservar y la versión |
| 8 | **Diagnóstico** | 2 | los escenarios ES-1 a ES-8 |
| 9 | **Simulación** | 3, 4, 5 | simular, con su informe |
| 10 | **Borde completo** | 6 a 9 | todas las operaciones del lenguaje publicado (`DEC-05.6`) |
| 11 | **Lo que se compra** | 2, 3 | el almacén con escritura condicional (`DEC-10.3`) y el acceso a repositorios (`DEC-10.2`), detrás de sus ACL |
| 12 | **El cambio** | 1 a 11, y el CLI y el portal adaptados | el motor nuevo sustituye al antiguo: imagen nueva, `cmd/motor/` pasa a `cmd/vexd/`, se borra el código antiguo y se reescribe `CLAUDE.md` (`DEC-11.6`) |

**El CLI y el portal** siguen usando el motor antiguo hasta la unidad 12, y el antiguo no se toca. Se
adaptan cuando el motor nuevo esté terminado, fuera de este plan (`DEC-11.6`).

**Sobre *Diagnóstico primero*** (`plan-ddd.md` §3). El core es donde va la mayor inversión, pero sin
Historial no tiene nada que comparar. Por eso se construye después de la unidad 2. Puede empezar en
paralelo con las unidades 3 a 7, usando historiales de prueba, sin esperar a que Ejecución los produzca.

---

## Reconciliación con lo anterior

| Qué | Qué pasa |
|---|---|
| **Las specs `00`–`28`** | **Derogadas como normativa** (`DEC-11.5`). No están en el espacio de trabajo (ni en disco ni en el historial de git), así que no se puede hacer una cuenta spec por spec. Lo que valía de ellas ya está en `modelo/`, decidido otra vez con el libro |
| **Las cinco `SPEC-*.md`** que viven junto al código de hoy | Se borran con el código antiguo (`DEC-04.5`). Si el código nuevo necesita documentar qué entra en un hash, lo hace como documentación técnica suya |
| **`vex-guia-logica.md`**, **`vex-plan-registro-local.md`** y **`vex-plan-revision.md`** | Se borran con el código antiguo. Lo que se decidió a partir de ellos está razonado en `iteraciones/` |
| **La sección del motor en `CLAUDE.md`** | Se reescribe en el paso 12, para que describa el motor nuevo |
| **`ecosistema/specs/`** (los planes M0–M7 del despliegue remoto) | Fuera de este plan: son del ecosistema, no del motor |
