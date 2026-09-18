# RD-10 — Borde completo

> Contexto: **Borde** (`arquitectura.md`) · Depende de: RD-06, RD-07, RD-08, RD-09 · Frente 2: **A**

## §1 Problema

Solo *intentar* y *hacer rollback* llegan al motor desde fuera.

## §2 Por qué importa

El CLI y el portal tienen que poder hacer todo lo que el motor ofrece, a través de un solo lenguaje
publicado (`DEC-04.4`).

## §3 Objetivo

- Todas las operaciones del lenguaje publicado: intentar, hacer rollback, simular, lanzar, reservar o
  liberar un ambiente, preguntar la causa, consultar el historial (sin valores) y dar por abandonado un
  intento.
- Cada petición declara su versión.
- El documento del lenguaje publicado, en español, para que el CLI y el portal se adapten más adelante
  (`DEC-11.6`).

## §4 Alternativas

- **Un contrato distinto para cada cliente.** Descartada (`DEC-04.4`).

## §5 Solución

- `internal/borde/` recibe una petición por invocación, la envía al `publicado/` de su contexto de entrada y
  devuelve su respuesta.
- Rechaza cualquier versión que no soporte.
- **La salida de los comandos** de *intentar* y *hacer rollback* va a la salida de la invocación, en vivo y
  tal cual (`DEC-12.5`). La respuesta de cada operación es aparte.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: adaptar el CLI y el portal (`DEC-11.6`).

## §7 Verificación

- Una petición con una versión no soportada se rechaza.
- Cada operación llega a su contexto de entrada.
- Consultar el historial no devuelve ningún valor.
- La regla de dependencias comprueba que el borde solo usa el `publicado/` de los contextos de entrada.

## §8 Decisiones

`DEC-02.2` · `DEC-04.4` · `DEC-05.6` · `DEC-07.8` · `DEC-11.6` · `DEC-12.5`

## §9 Hallazgos al implementar

*Implementada el 2026-09-18.* Compila, pasan sus pruebas y pasa la regla de dependencias.

1. **La versión viaja en cada petición, y quien no tenía petición recibe una del borde.** `PeticionDeIntento` y
   `PeticionDeRollback` ya traían `Version` (RD-06). Se añadió el campo a `simulacion/publicado.PeticionDeSimulacion`
   y a `diagnostico/publicado.PeticionDeDiagnostico`. Lanzar, reservar, liberar, abandonar y las tres consultas
   del Historial reciben strings sueltos en sus contextos; en vez de cambiar esas firmas, el borde declara sus
   propias peticiones (`internal/borde/peticiones.go`). El borde es el dueño de «petición = versión + datos».

2. **`NuevoServicio` recibe `Dependencias`** con los cinco `publicado.ParaBorde`, como los demás contextos. Los
   que una prueba no usa quedan en `nil`: es la raíz de composición (RD-12) quien tiene que conectar los cinco.

3. **«Sin valores» se comprueba, no se supone.** La prueba de punta a punta primero exige que el valor producido
   exista por la relación reservada (`ValoresDeUnPaso`) y solo entonces comprueba que ninguna consulta del borde
   lo contiene. Ese guard cazó un supuesto mío: los pasos se registran como `preparar`, sin el prefijo `NN-`.

4. **«Consultar el historial» son tres operaciones**: un intento, los intentos de un ambiente y los despliegues
   de un ambiente. `VariablesDeUnPaso` (con hashes, sin valores) sigue siendo de Diagnóstico, no del borde.

5. **La regla mecánica de dependencias no cambió**: ya permitía `borde → publicado` de los cinco contextos de
   entrada. Sigue sin ver Historial `reservado/`, que es de Resolución.

6. **El lenguaje publicado quedó en `docs/modelo/lenguaje-publicado.md`** (DEC-11.6). Versión soportada: `"1"`.
