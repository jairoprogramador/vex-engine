# RD-04 — Definición de Pipeline

> Contexto: **Definición de Pipeline** (`contextos/definicion.md`) · Depende de: RD-03 · Frente 2: **B**

## §1 Problema

No hay ningún pipeline comprobado que usar.

## §2 Por qué importa

El pipeline lleva la premisa del core: las instrucciones no varían por ambiente. Ejecución, Resolución y
Simulación lo necesitan, y ninguno debería poder recibir uno mal formado.

## §3 Objetivo

- Un agregado **Pipeline** inmutable, que solo existe si pasó la comprobación.
- Un repositorio de pipelines apoyado en Suministro.

## §4 Alternativas

- **Validar cuando se usa.** Descartada: dejaría circular pipelines mal formados (`DEC-08.2`).

## §5 Solución

- **Dominio**:
  - **Pipeline**: pasos identificados por su nombre, con sus comandos, su material y su configuración;
    variables declaradas por paso, y por ambiente o para el ámbito compartido; ambientes en orden; formas
    declaradas.
  - **Comprobación**, como factoría que devuelve un Pipeline o la lista de fallos.
- **Publicado**: el pipeline comprobado, con su versión.
- **Infraestructura**: el ACL hacia Suministro, que lee los ficheros del pipeline (`definicion.md`, «Los
  ficheros del pipeline») y los convierte en una declaración.
- **Plantillas**: los 15 `config.yaml` y los `vexpipeline.yaml` de `pipelines/vex-tpl-*` pasan a
  `schema_version: 3` (`DEC-12.6`).

## §6 Alcance

**Dentro**: lo anterior, incluidas las tres plantillas. **Fuera**: leer una `schema_version` anterior a 3.

## §7 Verificación

- No hay forma de obtener un Pipeline que no haya pasado la comprobación.
- Cada fila de la comprobación produce su fallo: formato, nombres y orden de pasos, paso sin comandos,
  variable usada no declarada o no visible en su ámbito en algún ambiente, expresión regular mal formada.
- También los fallos del formato: `schema_version` distinta de 3, regla desconocida, paso `shared` con
  variables por ambiente y paso `environment` con `variables/<paso>.yaml`.
- Las tres plantillas actualizadas pasan la comprobación.
- Renumerar un paso no cambia su identidad.
- Un fichero del material que no está en `templates` se copia sin interpolar.

## §8 Decisiones

`DEC-02.7` · `DEC-03.6` · `DEC-03.7` · `DEC-06.12` · `DEC-08.2` · `DEC-08.5` · `DEC-08.7` · `DEC-12.6` · `DEC-12.7`

## §9 Hallazgos al implementar

*Vacío.*
