# Ejecución de Pipeline — modelo del contexto

> **Supporting.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; sus relaciones, en `context-map.md` (filas #2, #5, #7 y #10).
>
> **Vigente** desde el cierre de IT-09 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**Cómo llevar a cabo un intento haciendo solo el trabajo que hace falta**, y dejar dicho qué se hizo, qué
no se re-ejecutó y por qué.

---

## Escenarios

| # | Escenario | Qué pasa |
|---|---|---|
| **EJ-1** | *Intentar hasta un paso en un ambiente* | 1. Abre el intento en el Historial, que lo rechaza si el ambiente está ocupado (`DEC-07.8`). 2. Pide el pipeline comprobado y el material: de hoy, de un commit o de una copia de trabajo. Con una copia de trabajo, el intento nunca llega a despliegue (`DEC-10.7`). 3. Pone su material en el espacio de trabajo. 4. **Por cada paso, en orden, hasta el pedido**: decide si se re-ejecuta; si sí, interpola, ejecuta sus comandos y entrega a Resolución lo que produjeron; si no, deja la razón y la evidencia. Cada paso que se ejecuta deja un registro **al empezar**, antes de su primer comando, y otro **al terminar** (`DEC-09.7`). 5. Si un comando falla, cierra el intento como **fallido**. Si llega al final, lo cierra como **exitoso**, y el Historial crea el despliegue si se hicieron todos los pasos del pipeline (`DEC-07.9`) |
| **EJ-2** | *Rollback a un destino* | Como EJ-1, pero con el material y las declaraciones del **commit** del destino (`DEC-03.9`), con todos los pasos del pipeline, y cerrando con el destino, que será el padre del despliegue nuevo |
| **EJ-3** | *Cancelación* | Llega la orden de cancelar: el comando en curso se detiene y el intento se cierra como **cancelado**, aunque la cancelación haya hecho fallar el comando |
| **EJ-4** | *El Historial no acepta un registro* | El almacén no responde, o el intento fue abandonado: se detiene antes del siguiente paso, y en el historial compartido queda sin desenlace (`DEC-05.9`, `DEC-07.8`) |
| **EJ-5** | *El espacio de trabajo no está disponible* | El intento no empieza (`DEC-06.18`) |

---

## Modelo táctico

### Agregado: **Intento en curso**

**El intento mientras se lleva a cabo** (en el Historial es un hecho; aquí puede cambiar). Vive durante la
invocación (`DEC-09.2`).

| Invariante | Qué dice |
|---|---|
| orden | los pasos se hacen en el orden del pipeline, y ninguno después del pedido |
| nada sin escribir | un paso no empieza si el registro del anterior no está escrito (`DEC-05.9`) |
| un fallo cierra | si un comando de un paso falla, no empieza ningún paso más y el desenlace es **fallido** |
| la cancelación gana | si se pidió cancelar, el desenlace es **cancelado**, aunque la cancelación haya hecho fallar un comando |
| un solo desenlace | exitoso, fallido o cancelado, y una sola vez |

### Value objects

| | Qué es |
|---|---|
| **recursos de un paso** | lo que un paso usa: el hash del código, el hash de sus instrucciones (comandos y material, `DEC-08.7`) y si cambiaron sus variables |
| **regla** | qué mira un paso para decidir si se re-ejecuta: código, instrucciones, las variables que ve, tiempo |
| **decisión** | *se re-ejecuta*, o *no se re-ejecuta*, con su **razón** y su **evidencia** |
| **destino** | el despliegue al que vuelve un rollback |
| **resultado de un comando** | terminó bien o falló, y lo que produjo |

### Servicio de dominio: **decidir un paso**

Recibe la regla del paso, sus recursos de ahora y su última vez, y devuelve una **decisión**. **No lee
nada, no calcula ningún hash y no escribe nada** (`DEC-09.3`). Un paso **solo puede no re-ejecutarse si su última vez es un final exitoso** (`DEC-09.7`). Así se separa lo que antes hacía una sola
pieza:

| Antes, la misma pieza | Ahora |
|---|---|
| decide | *decidir un paso*, un servicio de dominio sin efectos |
| hashea | cada hash tiene su dueño: el de las instrucciones, en la infraestructura de Ejecución; el del código, en Suministro; *¿cambiaron las variables?*, en Resolución |
| escribe | el servicio de aplicación, en el Historial |

**Para decidir**, las variables de un paso incluyen las **producidas por pasos anteriores**: una etiqueta de
imagen nueva tiene que re-ejecutar el despliegue. Es lo contrario que en Diagnóstico, donde una variable
producida no es eje (`DEC-06.10`), y es a propósito: aquí se decide qué hacer, allí se busca una causa
(`DEC-09.6`).

### Espacio de trabajo

**Tiene dos partes** (`DEC-09.5`):

- **la del motor**: su material, copiado e interpolado, que **se rehace entera** al empezar cada intento
  (`DEC-06.19`);
- **la de la tecnología**: todo lo demás, que el motor **no toca nunca**.

Dónde está el espacio de trabajo de un ambiente y dónde están sus registros se lo dice al motor la
invocación. Es infraestructura (`DEC-06.18`).

### Puertos

| Puerto | Hacia |
|---|---|
| **pipelines** | Definición: el pipeline comprobado, de hoy o de un commit |
| **fuentes** | Suministro: el material de hoy o de un commit, y el hash del código |
| **variables** | Resolución: las variables de un paso, interpolar, *¿cambiaron?*, registrar lo producido |
| **historial** | Historial: abrir, registrar un paso, cerrar; la última vez de un paso; un despliegue y su intento |
| **comandos** | ejecutar un comando en el espacio de trabajo de su ambiente y recoger su resultado |
| **espacio de trabajo** | rehacer la parte del motor |
| **salida** | el Historial: lo que imprimió cada comando, al terminar, para consultarlo con `logs` |

### La salida de los comandos

Lo que imprimen los comandos **no se muestra a quien pidió el intento: se guarda en el Historial**, un
registro por comando al terminar, con el paso, el nombre del comando, si salió bien y lo que escribió (salida
y error juntos). Se guarda también la de un comando que falló o que se canceló. Los registros viven aparte de
los del intento. No se tapan valores: la seguridad no es el objetivo del producto (`DEC-08.8`), y la promesa
de no exponer valores es sobre registros, consulta de intentos y diagnóstico, no sobre `logs`.

### Servicio de aplicación: **intentar**

Recorre EJ-1, o EJ-2 si trae un destino, con un **bucle explícito** sobre los pasos. Coordina y no decide:
la decisión es de *decidir un paso*, y las reglas del desenlace son del **Intento en curso**.

### Eventos, repositorios

**Ninguno propio.** Lo que queda se escribe en el Historial, que es quien publica *despliegue registrado*.

### Lo que publica

Hacia el borde: **intentar hasta un paso en un ambiente** y **hacer rollback a un destino**
(`DEC-05.6`). La salida de los comandos no sale por aquí: va al Historial.

### Las tres cadenas de hoy

**Caen** (`DEC-09.4`). El recorrido de un intento es una secuencia fija: abrir, preparar, y por cada paso
decidir, hacer y registrar, y cerrar. Una cadena de responsabilidad sirve cuando los eslabones pueden cambiar
o decidir si pasan el testigo. Aquí el orden nunca cambia, y lo que la cadena escondía (que el orden de
carga decide qué está completo) queda a la vista en un bucle y en invariantes.

**El objeto de contexto compartido desaparece**, repartido entre lo que ya existe en el modelo:

| Llevaba | Pasa a |
|---|---|
| el estado de la ejecución | el agregado **Intento en curso** |
| las variables | el agregado **Variables de un intento**, en Resolución (`DEC-08.3`) |
| el espacio de trabajo | el puerto **espacio de trabajo** |
| lo que se emite y se escribe | el puerto **historial** |

