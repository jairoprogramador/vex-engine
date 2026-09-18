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

*Implementada el 2026-09-14.* Compila, pasan sus pruebas (también con `-race`) y pasa la regla de
dependencias.

1. **Dos puertos, no uno.** La ficha ponía el hash detrás del puerto *repositorios*. Va en un puerto propio,
   *hashes*, y el servicio lo calcula sobre lo que el acceso dejó delante.
   - Así el hash es el mismo venga el material de hoy, de un commit o de una copia de trabajo.
   - RD-11 compra el acceso remoto sin reimplementar la regla: un hash distinto para el mismo contenido según
     el acceso sería la pérdida de `DEC-02.18`, y nada avisaría.
   - *Repositorios* sigue siendo lo que se compra; *hashes* es lo mínimo que se escribe (`DEC-10.2`). Corregido
     en `contextos/suministro.md`.
2. **El material es un directorio propio, y quien lo pide lo retira.**
   - *Fijo para quien lo pide* se cumple copiando: el árbol de un commit se extrae y una copia de trabajo se
     copia, a un directorio bajo una base que da la raíz de composición. Es una copia que se puede borrar
     (`DEC-05.10`).
   - El hash se calcula sobre esa copia, así que describe justo lo que se entregó aunque la copia de trabajo
     cambie mientras se lee.
   - Lo publicado añade *retirar*, que el modelo no nombraba. El acceso se niega a borrar un directorio que no
     puso él. Si no se puede calcular el hash, el material se retira y no se entrega.
3. **El material de una copia de trabajo es lo que entraría en un commit si se añadiera todo**: lo que sigue
   el índice, y lo que no sigue y no ignora ningún `.gitignore` de la copia. Sin eso, lo que genera la
   tecnología (`target/`, `node_modules/`) cambiaría el hash en cada build.
   - La tercera verificación se completó con la que lo prueba: *una copia de trabajo sin cambios tiene el hash
     de su commit*.
   - Solo cuentan los `.gitignore` de la copia: `.git/info/exclude` y la configuración global dependen de la
     máquina. Por eso las reglas se recorren aquí, y de go-git solo se usan su parser y su matcher
     (`ReadPatterns` lee `info/exclude`).
   - Un submódulo o un repositorio anidado no entran, igual que no entran al extraer un commit.
   - **Pérdida aceptada**: no se normalizan los fines de línea. Con `core.autocrlf`, una copia de trabajo no da
     el hash de su commit.
4. **La regla del hash, `contenido-v1:`**, escrita en `infraestructura/hash.go`: ficheros y enlaces, con su
   ruta, su contenido o su destino, y el bit de ejecución. Los directorios no entran, y los `.gitignore` sí,
   porque están en el commit. Si cambia lo que entra, cambia el prefijo, para que un cambio de regla no pase por
   un cambio de contenido.
5. **Un commit es su identificador completo**, 40 o 64 hexadecimales. Una abreviatura o una rama se rechazan:
   no son un punto fijo, y el commit se guarda para volver atrás. El acceso local solo lee SHA-1 (go-git v5).
   Resolver una abreviatura, si hace falta, es del borde (RD-10).
6. **En local, *hoy* es la cabeza del repositorio**: lo que no tiene commit no está hoy. Una fuente es el
   directorio raíz de su repositorio.
7. **Lo publicado va por cliente**, como en el Historial, según «Lo que le piden»: `ParaEjecucion` (hoy, commit
   y copia de trabajo), `ParaDefinicion` (hoy y commit) y `ParaSimulacion` (commit y copia de trabajo). Si RD-09
   necesita que Definición lea un pipeline de una copia de trabajo, se corrige allí. No hay clase de fuente
   (producto o pipeline): Suministro las trata igual, y quien pide sabe cuál pidió.
8. **Nada sale del material**: se rechaza una ruta que sube, un `.git` y escribir a través de un enlace. En un
   sistema de ficheros que no distingue mayúsculas, un enlace `A` y un fichero `a/b` son el mismo camino.
9. **Los escenarios se prueban sobre los adaptadores reales**, no en memoria (`DEC-11.3`): lo que verifica §7 es
   justo lo que hacen el acceso y el hash. go-git corre dentro del proceso, en directorios temporales, sin git
   instalado.
10. **Fuera, como decía la ficha**: el acceso remoto (RD-11). La raíz de composición no conecta Suministro
    hasta RD-06.
