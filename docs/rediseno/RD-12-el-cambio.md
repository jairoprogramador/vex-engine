# RD-12 — El cambio

> Contexto: — · Depende de: RD-10, RD-11, **y del CLI y el portal adaptados** (fuera de este plan) · Frente 2:
> **A**

## §1 Problema

Conviven dos motores, y el CLI y el portal siguen usando el antiguo.

## §2 Por qué importa

Mientras exista el código antiguo, el motor carga con dos modelos. El cambio se hace de una vez, cuando el
motor nuevo está terminado (`DEC-11.6`).

## §3 Objetivo

El motor nuevo sustituye al antiguo y no queda nada del código antiguo.

## §4 Alternativas

- **Que el motor nuevo entienda también el contrato de hoy.** Descartada (`DEC-11.6`).

## §5 Solución

- La imagen que lleva el motor pasa a llevar el nuevo (la de `runtime/`, que está fuera de `vex-engine`).
- `cmd/motor/` pasa a `cmd/vexd/` (hecho).
- Se borran:
  - `old-internal/` entero (RD-01 §9);
  - el `cmd/vexd/` antiguo;
  - las cinco `SPEC-*.md`;
  - `vex-guia-logica.md`, `vex-plan-registro-local.md` y `vex-plan-revision.md`.
- `reglas.yml` pasa a compilar, analizar y probar `./...`, y el release construye el `cmd/vexd` nuevo.
- Se reescribe la sección del motor en `CLAUDE.md`.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: adaptar el CLI y el portal, que es la condición para empezar esta unidad.

## §7 Verificación

- No queda ningún paquete antiguo, y el comando de `arquitectura.md` pasa sobre `./...`.
- El CLI y el portal funcionan contra el motor nuevo.
- `CLAUDE.md` describe el motor nuevo.

## §8 Decisiones

`DEC-11.1` · `DEC-11.5` · `DEC-11.6`

## §9 Hallazgos al implementar

*Hecho a medias el 2026-09-18*, sin esperar al CLI ni al portal, que quedan fuera por ahora.

**Hecho**: se borraron `old-internal/` y el `cmd/vexd/` antiguo (y con ellos `cobra` de `go.mod`); `cmd/motor/`
(ahora `cmd/vexd/`) conecta los contextos y atiende con el borde las once operaciones (`docs/modelo/lenguaje-publicado.md`, «La
invocación»); `reglas.yml` compila, analiza y prueba `./...`; `goreleaser.yaml` construye `./cmd/vexd` con el
binario `vexd`.

**Pendiente**: la imagen de `runtime/`; borrar las cinco `SPEC-*.md`,
`vex-guia-logica.md`, `vex-plan-registro-local.md` y `vex-plan-revision.md`; reescribir la sección del motor de
`CLAUDE.md`; y adaptar el CLI y el portal, sin lo cual **un release con este binario rompe a los dos** (ya no
existen `vexd run`, `RequestInput` ni `--state-config`).
