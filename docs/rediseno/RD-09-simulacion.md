# RD-09 — Simulación de Pipeline

> Contexto: **Simulación de Pipeline** (`contextos/simulacion.md`) · Depende de: RD-03, RD-04, RD-05 ·
> Frente 2: **B**

## §1 Problema

El DevOps no puede saber si su pipeline funciona antes de publicarlo.

## §2 Por qué importa

Es lo que el DevOps quiere hacer antes de publicar, y no puede tener ningún efecto.

## §3 Objetivo

*Simular* (SIM-1 y SIM-2), con su informe.

## §4 Alternativas

- **Un modo de Ejecución.** Descartada: *intento* y *variable producida* significarían dos cosas dentro de
  Ejecución (`DEC-03.10`).

## §5 Solución

- **Dominio**:
  - **Simulación**: sin efectos; recorre todos los pasos de cada ambiente; las salidas simuladas cumplen su
    expresión regular; el informe no lleva valores.
  - Servicio *fabricar una salida simulada*.
- **Aplicación**: *simular*.
- **Infraestructura**: un espacio temporal, y la interpolación de Resolución en una petición sin intento.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: simular fallos de comandos (`DEC-10.4`).

## §7 Verificación

- No escribe en el Historial ni toca el espacio de trabajo de ningún ambiente.
- Recorre todos los pasos de cada ambiente.
- Toda salida simulada cumple su expresión regular.
- Si una variable no se puede interpolar, aparece en el informe y la simulación sigue con el siguiente
  ambiente.
- Acepta una copia de trabajo.

## §8 Decisiones

`DEC-03.10` · `DEC-04.6` · `DEC-04.10` · `DEC-10.4` · `DEC-10.6`

## §9 Hallazgos al implementar

*Vacío.*
