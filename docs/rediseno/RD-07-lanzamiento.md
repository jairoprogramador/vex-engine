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

**El `ParaLanzamiento` del Historial no alcanzaba.** Calcular la versión (`DEC-10.8`) exige recorrer el
Historial de dos formas que el puerto no daba: el hash del código de un despliegue, y la lista completa de
lanzamientos ya hechos, de cualquier ambiente. El Historial no puede darlas por sí solo desde dentro de
`RegistrarLanzamiento`, porque su `Contenido` es opaco (`DEC-03.13`) — solo quien lo produjo, Lanzamiento,
puede decodificarlo. Se extendió `ParaLanzamiento` con `HashDelCodigoDeUnDespliegue` y
`TodosLosLanzamientos`, ambas de solo lectura, siguiendo el mismo patrón de recorrido que ya usaba
`UltimoDespliegueConHashDelCodigo`. El propio `Contenido` de Lanzamiento guarda también el hash del código
(además de versión y nombre) para no tener que volver a consultar el Historial por cada lanzamiento pasado
al decidir la versión de uno nuevo.

**Ventana de duplicado de versión en escrituras concurrentes, aceptada.** `RegistrarLanzamiento` del
Historial reintenta ante conflicto (`conReintento`, 3 veces), releyendo `Lanzamientos.Todos` y
`Despliegues.DeUnAmbiente` en cada vuelta, pero siempre reusa el mismo `Contenido` que se le pasó la
primera vez — no puede recalcularlo, porque le es opaco. Si dos códigos nunca antes lanzados se lanzan a la
vez en dos ambientes distintos, ambos pueden calcular la misma «próxima versión» antes de que cualquiera de
las dos escrituras llegue, y el reintento ciego del Historial deja pasar las dos con la misma versión en
lugar de forzar un conflicto que Lanzamiento pudiera resolver recalculando. Aceptado y documentado, no
resuelto: arreglarlo exigiría cambiar la forma del puerto `RegistrarLanzamiento` del Historial, y este
mismo documento (§5) dice que Lanzamiento se adapta a esa forma, no al revés.

**"Obtener la versión" (§3) no es una operación publicada aparte.** `contextos/lanzamiento.md` solo lista
tres servicios de aplicación (escuchar, lanzar, reservar/liberar). Se implementó como la capacidad interna
de la que depende `lanzar` (`DecidirVersion`, dominio puro), no como una consulta nueva en `publicado/`.
