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
  condicional por agregado (§9), y que ofusca el valor.

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

*Implementada el 2026-09-14.* Compila, pasan sus pruebas (también con `-race`) y pasa la regla de
dependencias.

1. **La escritura condicional es por agregado** (incongruencia traída al usuario al empezar; respuesta: *sí,
   por agregado*).
   - `DEC-10.3` la pedía solo para la Ocupación. Pero `DEC-07.8` promete que la máquina de un intento
     abandonado «no podrá escribir», y eso cruza máquinas: sin escritura condicional sobre el intento, su
     cierre se escribiría detrás del abandono. `DEC-10.8` ya la usaba para la versión.
   - La tercera garantía queda así, corregida en `contextos/historial.md`: añadir un registro a un agregado
     solo si nadie añadió otro desde que se leyó. El almacén da secuencias que solo crecen, y añadir en una
     posición que ya existe devuelve conflicto. Ante un conflicto, el servicio vuelve a leer y a decidir.
   - Una secuencia por intento. Una por ambiente para despliegues, ocupaciones y reservas, porque el padre de
     un despliegue y la ocupación vigente se comprueban contra los anteriores del ambiente. Y una para todos
     los lanzamientos, por la versión de `DEC-10.8`.
2. **La ocupación no registra la liberación.** El ambiente está libre cuando el intento de su última
   ocupación está cerrado o abandonado. Así, cerrar y abandonar escriben un solo registro, y no puede quedar
   un ambiente ocupado por un intento que ya terminó.
3. **El orden de los intentos de un ambiente es el de sus ocupaciones**: la escritura condicional lo hace
   total aunque los relojes de dos máquinas no coincidan. En el ámbito compartido no hay un orden total entre
   ambientes, y la última vez de un paso es el registro con el instante mayor.
4. **Dos operaciones escriben en dos agregados**, en un orden elegido:
   - *Abrir* ocupa y después abre. Si la apertura no se escribe, el ambiente queda ocupado por un intento sin
     apertura, que se puede dar por abandonado y que no se consulta ni se cuenta. En el orden contrario
     quedaría en el historial un intento que nunca tuvo su ambiente.
   - *Cerrar* escribe el cierre y después el despliegue. Repetir el mismo cierre no escribe otro y completa el
     despliegue. *Despliegue registrado* se publica solo cuando el despliegue se escribe.
5. **Invariantes que el modelo no listaba**, dentro de su agregado: un paso tiene como mucho un comienzo y un
   final; una variable va bajo un paso en curso; un cierre exitoso exige los pasos pedidos hechos; un intento
   tiene como mucho un despliegue. Una más lee otro agregado sin escribirlo: la evidencia de una no
   re-ejecución apunta a un final exitoso (`DEC-09.7`).
6. **Lo que la ficha no nombraba y el modelo sí**: *registrar el hash de una variable* y las variables de un
   paso, para Resolución. *La última vez de una variable* es la última vez del paso más las variables de ese
   intento; seguir la evidencia es de Resolución. El nombre de la variable es una clave que pidió Resolución
   (`DEC-07.6`), como el hash del código lo es de Diagnóstico.
7. **Lo publicado va por cliente**: `ParaEjecucion`, `ParaResolucion`, `ParaLanzamiento`, `ParaDiagnostico` y
   `ParaBorde`. Tiene sus propios errores: rechazado, no existe, y ambiente ocupado con el intento en curso.
   `ParaDiagnostico` lo completa RD-08. Ningún tipo publicado tiene un campo para un valor, y una prueba lo
   comprueba.
8. **El almacén local** publica cada registro con un enlace duro desde un temporal: no sobrescribir y
   escribir de forma condicional son la misma operación, y un lector nunca ve un registro a medias. Por eso
   necesita un sistema de ficheros con enlaces duros.
   - Cada registro lleva `formato: 1`.
   - Los nombres que vienen de fuera se escriben con `%XX`, y no salen de su directorio.
   - El valor se ofusca con una máscara y base64, sin clave por máquina: tiene que volver en cualquiera.
9. **Fuera, como decía la ficha**: *traer el historial antes de leerlo* no hace nada en local (RD-11), y la
   raíz de composición no conecta el Historial hasta RD-06.
10. **`context.Context` por valor**, como es idiomático en Go. Al principio se copió `*context.Context` de la
    convención del código antiguo, y se retiró a pedido del usuario: el código antiguo no es normativo, y el
    modelo no dice nada del contexto.
