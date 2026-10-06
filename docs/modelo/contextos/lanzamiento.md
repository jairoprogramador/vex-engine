# Lanzamiento — modelo del contexto

> **Supporting.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; su relación, con el Historial, en `context-map.md` (fila #4).
>
> Entra en E10 porque nació en IT-01, después de escribirse el plan, y ninguna etapa lo tenía (`DEC-10.1`).
>
> **Vigente** desde el cierre de IT-10 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Cuándo un despliegue se hace visible a quien usa su ambiente, y con qué nombre sale.** Lo decide el dueño
del negocio en los ambientes que se reserva; en los demás, Lanzamiento actúa en su nombre (`DEC-04.1`).

---

## Escenarios

| # | Escenario | Qué pasa |
|---|---|---|
| **LAN-1** | *Llega «despliegue registrado»* | Si el ambiente no está reservado, lanza en nombre del actor ausente el último despliegue del ambiente, si todavía no está lanzado (`DEC-04.8`, `DEC-05.4`) |
| **LAN-2** | *El dueño del negocio lanza* | Lanza un despliegue, con nombre o sin él |
| **LAN-3** | *Reservar o liberar un ambiente* | Registra la reserva o la liberación (`DEC-04.9`) |

---

## Modelo táctico

### Agregados

**Ninguno propio** (`DEC-10.5`). *Lanzamiento* y *Reserva* son agregados del Historial (`DEC-07.2`), que
protege su forma. Lanzamiento **decide**, y lo que decide queda allí.

### Servicio de dominio

**Decidir si se lanza en nombre del actor ausente**: con el despliegue nuevo, la última reserva del
ambiente y el último lanzamiento, devuelve *lanzar* o *no lanzar*.

### Factoría

**Un lanzamiento**: con su despliegue, su versión y su nombre. Si no hay nombre, toma el valor de la versión
(`DEC-02.17`).

### Value objects

| | Qué es |
|---|---|
| **versión** | la etiqueta técnica del lanzamiento, con clave «version». Su valor es **un número por proyecto que crece de uno en uno con cada código que se lanza por primera vez**; el mismo código conserva su versión en todos los ambientes (`DEC-10.8`) |
| **nombre del lanzamiento** | la etiqueta de negocio, con clave «nombre» |
| **destinatario** | quien usa el ambiente |

### Servicios de aplicación

**Escuchar *despliegue registrado*** · **lanzar** · **reservar o liberar un ambiente**.

### Puerto

**Historial**: registrar un lanzamiento y una reserva; el último despliegue, el último lanzamiento y la
última reserva de un ambiente; el evento *despliegue registrado*.

### Fuera de alcance

**Las estrategias de lanzamiento**, por decisión explícita de `dominio.md`.

