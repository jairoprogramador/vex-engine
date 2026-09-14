# RD-03 — Suministro de Fuentes

> Contexto: **Suministro de Fuentes** (`contextos/suministro.md`) · Depende de: RD-01 · Frente 2: **A**

## §1 Problema

El motor nuevo no tiene material con el que trabajar.

## §2 Por qué importa

Definición, Ejecución y Simulación necesitan el material y sus hashes. Y dejar lo que se compra detrás de
un ACL desde el primer día es lo que permite cambiarlo sin tocar a nadie más.

## §3 Objetivo

Traer una fuente como está hoy, como estaba en un commit o como está en una copia de trabajo, con su hash
y, si lo hay, su commit.

## §4 Alternativas

- **Comprar ya el acceso a repositorios remotos.** Aplazada a RD-11: el primer corte se hace con
  repositorios locales, detrás del mismo puerto.

## §5 Solución

- **Dominio**: value objects fuente, commit, hash, copia de trabajo y material.
- **Aplicación**: *traer una fuente*.
- **Publicado**: traer, con su hash y su commit.
- **Infraestructura**: acceso a repositorios locales (un directorio y un commit) y el hash del contenido,
  detrás del puerto *repositorios*.

## §6 Alcance

**Dentro**: lo anterior, en local. **Fuera**: el acceso comprado a repositorios remotos (RD-11).

## §7 Verificación

- El mismo contenido da el mismo hash, y un cambio de contenido cambia el hash.
- Si se revierte un cambio, el commit es nuevo y el hash vuelve a ser el de antes.
- Una copia de trabajo tiene hash y no tiene commit.

## §8 Decisiones

`DEC-01.7` · `DEC-02.18` · `DEC-03.9` · `DEC-07.7` · `DEC-10.2` · `DEC-10.6`

## §9 Hallazgos al implementar

*Vacío.*
