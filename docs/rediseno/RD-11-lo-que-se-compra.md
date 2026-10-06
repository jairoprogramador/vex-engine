# RD-11 — Lo que se compra

> Contextos: **Historial** y **Suministro de Fuentes** · Depende de: RD-02, RD-03 · Frente 2: **D**

## §1 Problema

El almacén y el acceso a repositorios solo funcionan en local.

## §2 Por qué importa

Los intentos corren en máquinas efímeras: el historial tiene que estar disponible en cualquiera de ellas, y
los repositorios, alcanzables desde cualquiera. Como las dos piezas están detrás de su ACL desde RD-02 y
RD-03, cambiarlas toca un solo contexto cada una.

## §3 Objetivo

- Un almacén comprado, detrás del ACL del Historial, que cumpla las tres garantías de `DEC-10.3`.
- Un acceso comprado a repositorios remotos, detrás del ACL de Suministro.

## §4 Alternativas

- **Escribir un almacén propio.** Descartada, salvo que ningún almacén que se pueda comprar dé la escritura
  condicional (`DEC-10.3`).

## §5 Solución

Un adaptador nuevo en la `infraestructura/` de cada contexto, detrás del mismo puerto que usan los locales.

## §6 Alcance

**Dentro**: los dos adaptadores. **Fuera**: cualquier cambio en el dominio o en la aplicación.

## §7 Verificación

- Las pruebas de los repositorios del Historial pasan también contra el almacén comprado, incluida la
  ocupación con dos procesos a la vez.
- Se puede traer una fuente desde un commit de un repositorio remoto.
- Ningún paquete fuera de `infraestructura/` cambia.

## §8 Decisiones

`DEC-01.7` · `DEC-06.18` · `DEC-10.2` · `DEC-10.3`

## §9 Hallazgos al implementar

*Vacío.*
