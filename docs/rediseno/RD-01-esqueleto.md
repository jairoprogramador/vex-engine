# RD-01 — Esqueleto

> Contexto: — · Depende de: — · Frente 2: **B**

## §1 Problema

El motor nuevo no tiene dónde vivir, y la regla de dependencias solo existe como documento.

## §2 Por qué importa

Sin la regla desde el primer paquete, el código nuevo puede importar el antiguo o saltarse el context map
sin que nadie lo note. La regla tiene que cumplirse por construcción, no por disciplina.

## §3 Objetivo

- Existe el árbol de `arquitectura.md` para los ocho contextos, el borde y la relación reservada del
  Historial.
- Existe una raíz de composición `cmd/motor/` que compila.
- La regla de dependencias corre en la integración continua sobre los paquetes nuevos.

## §4 Alternativas

- **Un árbol provisional fuera de `internal/`.** Descartada: al terminar habría que moverlo todo
  (`DEC-11.2`).
- **Lo nuevo al lado del antiguo, dentro del mismo `internal/`.** Descartada al empezar la unidad, a
  pedido del usuario: obligaba a filtrar los paquetes nuevos en cada comando. El antiguo pasa a
  `old-internal/`.

## §5 Solución

- `internal/<contexto>/{dominio,aplicacion,infraestructura,publicado}/` para `diagnostico`, `definicion`,
  `historial`, `lanzamiento`, `ejecucion`, `resolucion`, `simulacion` y `suministro`, cada paquete con un
  `doc.go` que diga qué contiene.
- `internal/historial/reservado/` e `internal/borde/`.
- `cmd/motor/main.go`, que arranca y termina sin hacer nada.
- **El código antiguo, a `old-internal/`**, con sus importaciones reescritas. Sigue compilando, y el release
  sigue construyendo `cmd/vexd` hasta RD-12.
- La regla de dependencias, en `.github/workflows/reglas.yml`: compila, analiza y prueba `internal/` y
  `cmd/motor/`, y pasa el comando de `arquitectura.md` sobre `./...`.

## §6 Alcance

**Dentro**: el árbol, la raíz y la regla. **Fuera**: cualquier modelo.

## §7 Verificación

- `go build ./cmd/motor/` compila.
- El código antiguo compila y pasa sus pruebas desde `old-internal/`.
- El comando de la regla pasa sobre los paquetes nuevos.
- Un paquete de prueba que importe código antiguo, o `publicado/` de un contexto que no está arriba, hace
  fallar el comando.

## §8 Decisiones

`DEC-05.1` · `DEC-05.2` · `DEC-05.7` · `DEC-11.2`

## §9 Hallazgos al implementar

*Implementada el 2026-09-14.*

1. **El antiguo, a `old-internal/`** (a pedido del usuario al empezar la unidad).
   - Se reescribieron las importaciones de 178 ficheros, y ninguna cadena del código antiguo nombraba la
     ruta.
   - Build, vet y pruebas del antiguo pasan.
   - `go list ./internal/...` ya solo da lo nuevo, así que **sobra el comando filtrado de `migracion.md`**: el
     de `arquitectura.md` corre sobre `./...` y el awk ignora lo que no está bajo `internal/`.
   - Se pierde la protección de Go para `internal` en el antiguo, y no importa: nada fuera del módulo lo
     importa.
2. **Al sacar el antiguo de `internal/`, la regla dejó de detectar que el código nuevo lo importara**
   desde `aplicacion/` o `infraestructura/`. Antes lo detectaba porque `domain` no estaba arriba de ningún
   contexto. Fuera de `internal/`, solo lo marcaba en `dominio/` y `publicado/`, como si fuera de
   terceros. Se añadió al awk: *el código nuevo no usa el código antiguo*.
3. **Verificación de la regla.** Con tres violaciones puestas a propósito, el comando detecta las tres y sale
   con 1:
   - un `dominio/` que usa lo publicado de su propio contexto;
   - un contexto que usa otro que no está arriba;
   - código nuevo que usa el antiguo.
4. **La CI necesita `docs/`** en el repositorio, porque el awk vive en `docs/modelo/`. Hasta que se suba,
   solo está comprobado en local.
