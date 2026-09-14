# RD-02 — Historial

> Contexto: **Historial** (`contextos/historial.md`) · Depende de: RD-01 · Frente 2: **B**

## §1 Problema

El motor nuevo no guarda nada de lo que pasa.

## §2 Por qué importa

Es la única memoria del motor (`DEC-06.18`) y el único sitio del que lee el core (`DEC-03.3`). Todos los
demás contextos escriben en él o leen de él.

## §3 Objetivo

- Los cinco agregados, con sus invariantes, y un repositorio por agregado que solo añade y recorre.
- El evento *despliegue registrado*, publicado después de escribir su registro.
- Las consultas que piden sus clientes, sin valores, y la relación reservada para el valor ofuscado.
- Un almacén local que cumple las tres garantías de `DEC-10.3`.

## §4 Alternativas

- **Solo un almacén en memoria.** Descartada: el primer corte de punta a punta (RD-06) necesita que lo
  escrito persista. El almacén en memoria sirve para las pruebas.

## §5 Solución

- **Dominio**:
  - **Intento**: apertura con los pasos del pipeline, hasta cuál se pide y si hay commit; comienzo y final
    por paso, o una no re-ejecución con su evidencia; cierre; abandono.
  - **Despliegue**: lo crea el Intento como factoría; tiene padre; solo nace con commits.
  - **Lanzamiento**, **Reserva** y **Ocupación**.
  - Value objects: identidades, estado, evidencia, clave, contenido opaco con su contexto dueño.
- **Aplicación**:
  - Escrituras: abrir, registrar un paso, cerrar y abandonar un intento; registrar un lanzamiento y una
    reserva.
  - Consultas: la última vez de un paso en su ámbito, los despliegues de un ambiente, el último con un hash
    del código dado, la cantidad de intentos, la última reserva y el último lanzamiento.
- **Publicado**: las operaciones, los registros sin valores y la interfaz de quien escucha el evento.
- **Reservado**: guardar y devolver el valor ofuscado, solo para Resolución.
- **Infraestructura**: un almacén local en un directorio que escribe sin sobrescribir, con una escritura
  condicional para la ocupación, y que ofusca el valor.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: el almacén que se compra y la disponibilidad desde otras máquinas
(RD-11).

## §7 Verificación

- Una prueba por invariante de cada agregado.
- Ningún repositorio tiene *actualizar* ni *borrar*.
- Una segunda apertura en un ambiente ocupado se rechaza, diciendo cuál es el intento en curso.
- Un registro después del cierre o del abandono se rechaza.
- Un despliegue solo nace de un intento cerrado como exitoso, hecho con commits y con un final exitoso o una
  no re-ejecución en todos los pasos del pipeline.
- *Despliegue registrado* se publica después de escribir el despliegue, nunca antes.
- Nada de lo publicado contiene un valor.

## §8 Decisiones

`DEC-03.13` · `DEC-04.7` · `DEC-04.8` · `DEC-05.5` · `DEC-05.8` · `DEC-05.9` · `DEC-06.18` · `DEC-07.2` a
`DEC-07.9` · `DEC-09.7` · `DEC-10.3` · `DEC-10.7`

## §9 Hallazgos al implementar

*Vacío.*
