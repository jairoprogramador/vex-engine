# RD-08 — Diagnóstico *(el core)*

> Contexto: **Diagnóstico** (`contextos/diagnostico.md`) · Depende de: RD-02 · Frente 2: **B, C**

## §1 Problema

El core no existe.

## §2 Por qué importa

Es la razón por la que se elige Vex (`DEC-01.1`), y donde tiene que ir la mayor inversión (frente 1.4 de
`DEC-01.10`). Puede empezar en paralelo con RD-03 a RD-07 usando historiales de prueba.

## §3 Objetivo

*Preguntar la causa*, con los escenarios ES-1 a ES-8, y sus invariantes cumplidas por construcción.

## §4 Alternativas

- **Una vista de consulta sobre el Historial.** Descartada: el core decide una atribución, no presenta datos
  (`DEC-01.2`, `DEC-03.3`).

## §5 Solución

- **Dominio**:
  - Value objects: eje, ejes de un paso, referencia, comparación, atribución, sustento y respuesta.
  - Servicios: *elegir las referencias* y *eliminación*.
  - Sin entidades, agregados ni repositorios (`DEC-06.14`).
- **Aplicación**: *preguntar la causa*.
- **Publicado**: la operación y la respuesta.
- **Infraestructura**: el ACL hacia el Historial, que construye los ejes de cada paso siguiendo la evidencia
  y separando las variables declaradas de las producidas.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: autores y commits como sustento (`DEC-07.7`).

## §7 Verificación

- Una prueba por escenario, ES-1 a ES-8.
- **El caso normal** (ES-1 + ES-2) responde un solo candidato: las variables.
- Invariantes por construcción:
  - la respuesta nunca lleva un valor;
  - la atribución es un conjunto sin orden, así que nunca hay un «probablemente»;
  - un intento cancelado o sin desenlace no se atribuye;
  - la referencia es siempre un despliegue;
  - una variable producida no es eje.

## §8 Decisiones

`DEC-01.2` · `DEC-01.11` · `DEC-01.12` · `DEC-03.3` · `DEC-06.1` a `DEC-06.16` · `DEC-06.19` · `DEC-07.6`
· `DEC-07.7` · `DEC-08.6` · `DEC-08.7`

## §9 Hallazgos al implementar

*Vacío.*
