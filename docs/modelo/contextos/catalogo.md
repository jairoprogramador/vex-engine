# Catálogo — modelo del contexto

> **Supporting**, de solo lectura. Su relación con los demás, en `context-map.md` (filas #13 y #14); su
> frontera, en `bounded-contexts.md`.

---

## Qué responde

**Qué ambientes tiene un pipeline y cuáles están reservados, y qué pasos tiene.** Quien presenta (el CLI y el
portal) lo necesita antes de pedir un intento o un lanzamiento: para saber qué ambientes y qué pasos puede nombrar.

Es un contexto propio porque la respuesta cruza dos contextos que no se pueden mirar entre sí: los ambientes y
los pasos son del pipeline (Definición), y la reserva es un registro del Historial. Lanzamiento solo puede
depender del Historial, así que no puede leer el pipeline. Catálogo está por encima de los dos y los junta, sin
decidir nada.

---

## Escenarios

| # | Escenario | Qué pasa |
|---|---|---|
| **CAT-1** | *Listar los ambientes* | Lee el pipeline (de hoy, o el de un commit) y, de cada ambiente, pregunta si está reservado |
| **CAT-2** | *Listar los pasos* | Lee el pipeline y devuelve sus pasos, en su orden |

---

## Modelo táctico

**Ningún agregado, ninguna regla de negocio propia.** Es una lectura: no escribe nada, ni en el Historial ni en
ningún otro sitio.

### Objetos de valor

| | Qué es |
|---|---|
| **fuente** | dónde está el pipeline que se consulta. Opaca: Catálogo nunca la interpreta. No puede estar vacía |
| **commit** | el del pipeline que se consulta. Vacío es el de hoy |
| **ambiente** | nombre, descripción y *valor* (con el que se nombra al pedir un intento o un lanzamiento) |
| **paso** | nombre, orden y si es compartido |

### Servicios de aplicación

**Listar los ambientes** · **listar los pasos**. Ambos conservan el orden del pipeline.

### Puertos

- **Pipelines**: el pipeline ya comprobado de una fuente y un commit. Lo implementa un ACL sobre lo que publica
  Definición.
- **Reservas**: si un ambiente, por su valor, está reservado ahora mismo. Lo implementa un ACL sobre lo que
  publica el Historial. Un ambiente que nunca se reservó no lo está, igual que para Lanzamiento.

---

## Errores

- Una fuente vacía, o una fuente o un commit que no están donde se dice, es `ErrInvalido`: quien invoca lo puede
  corregir (igual que en Ejecución). Lleva el `campo` que no vale cuando es la validación propia.
- Un pipeline que no pasa la comprobación es `ErrRechazado`, y conserva sus fallos para quien presenta.
- Lo demás es un fallo del motor y no se disfraza.
