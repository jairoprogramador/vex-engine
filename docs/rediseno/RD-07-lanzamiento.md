# RD-07 — Lanzamiento

> Contexto: **Lanzamiento** (`contextos/lanzamiento.md`) · Depende de: RD-02 · Frente 2: **B**

## §1 Problema

Ningún despliegue se hace visible, ni con nombre ni con versión.

## §2 Por qué importa

Es la decisión del dueño del negocio, y la regla del actor ausente tiene que cumplirse sin que nadie llame a
Lanzamiento desde la ejecución (`DEC-04.8`).

## §3 Objetivo

Escuchar *despliegue registrado* (LAN-1, poniéndose al día), lanzar (LAN-2), reservar y liberar (LAN-3), y
obtener la versión.

## §4 Alternativas

- **Que Ejecución llame a Lanzamiento al terminar.** Descartada: convertiría lanzar en un efecto del
  despliegue (`DEC-04.8`).

## §5 Solución

- **Dominio**:
  - Servicio *decidir si se lanza en nombre del actor ausente*.
  - Factoría de un lanzamiento: si no tiene nombre, toma la versión.
  - Versión: un número por proyecto que crece con cada código que se lanza por primera vez, calculado
    recorriendo el Historial.
- **Aplicación**: escuchar, lanzar, reservar y liberar.
- **Infraestructura**: el Historial, adoptando su forma, con la escritura condicional para la versión.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: las estrategias de lanzamiento (`dominio.md`).

## §7 Verificación

- En un ambiente reservado no se lanza en nombre del actor ausente. En uno no reservado, se lanza el último
  despliegue que todavía no esté lanzado.
- El mismo hash del código tiene la misma versión en todos los ambientes.
- Si no hay nombre, el nombre toma la versión.
- Un intento con copia de trabajo no provoca ningún lanzamiento.

## §8 Decisiones

`DEC-02.17` · `DEC-03.17` · `DEC-04.1` · `DEC-04.8` · `DEC-04.9` · `DEC-05.4` · `DEC-10.5` · `DEC-10.8`

## §9 Hallazgos al implementar

*Vacío.*
