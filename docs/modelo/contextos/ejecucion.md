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
| **EJ-4** | *El Historial no acepta un registro* | Se detiene antes del siguiente paso e intenta cerrar el intento como **fallido**. Si el almacén no responde, o el intento fue abandonado, el cierre tampoco se puede escribir y en el historial compartido queda sin desenlace (`DEC-05.9`, `DEC-07.8`) |
| **EJ-5** | *El espacio de trabajo no está disponible* | El intento no empieza (`DEC-06.18`) |
| **EJ-6** | *Un error impide seguir* | Con el intento ya abierto, algo falla que no es un comando: interpolar una variable que no está, un puerto que no responde. El intento se cierra como **fallido** —o como **cancelado**, si la causa fue la cancelación— y quien invocó recibe el error. El ambiente no queda ocupado. Si el cierre no se puede escribir, vale EJ-4 |
| **EJ-7** | *El proceso muere sin cerrar el intento* | Nadie puede escribir el cierre: `kill -9`, falta de memoria, una máquina que se apaga. Mientras vivía, el intento escribía **latidos** desde que abrió; al morir dejan de crecer. El siguiente intento que encuentre el ambiente ocupado lo observa, lo cierra como **fallido** con causa **interrumpido** y ocupa el ambiente (`historial.md`, «Si el dueño del ambiente murió») |

---

## Modelo táctico

### Agregado: **Intento en curso**

**El intento mientras se lleva a cabo** (en el Historial es un hecho; aquí puede cambiar). Vive durante la
invocación (`DEC-09.2`).

| Invariante | Qué dice |
|---|---|
| orden | los pasos se hacen en el orden del pipeline, y ninguno después del pedido |
| nada sin escribir | un paso no empieza si el registro del anterior no está escrito (`DEC-05.9`) |
| un fallo cierra | si un comando de un paso falla, o un error impide seguir (EJ-6), no empieza ningún paso más y el desenlace es **fallido** |
| un proceso vivo late | desde que el intento abre hasta que cierra, su proceso escribe un latido cada pocos segundos, en paralelo a los comandos: es lo que distingue un intento largo de uno muerto (EJ-7) |
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

### Contexto de ejecución

Quien invoca puede declarar un **contexto**: una cadena opaca que dice en qué entorno externo corre el intento
(por ejemplo, una huella de la conexión de nube). El motor **no la interpreta** y no conoce ninguna nube.

- **Qué hace.** Se mezcla en la huella de variables de cada paso, antes de compararla con la de su última vez.
  Con el mismo contexto un paso se reutiliza como siempre; con otro, se re-ejecutan los pasos cuya regla mira
  `variables`, y solo esos.
- **Qué no es.** No es una variable del pipeline (`${var.x}` no la ve, no pasa por Resolución) ni una variable
  de entorno, y no entra en ningún otro hash: ni en el del código ni en el de las instrucciones. No es un
  secreto, y no sirve para guardar uno.
- **Qué se guarda.** Solo la huella combinada, como cualquier huella de variables. La cadena no se escribe en el
  Historial.
- **Sin contexto**, la huella es exactamente la que da Resolución: quien no lo manda conserva su historial. La
  primera vez que un ambiente pasa de no tenerlo a tenerlo (o al revés), los pasos que miran `variables` se
  re-ejecutan una vez. No hay migración.
- **Forma.** Hasta 128 bytes de letras ASCII, números y `. _ : / @ + = -`. Cualquier otra cosa es una petición
  inválida, antes de abrir el intento ni traer nada. `simular` no lo recibe: no decide reutilización.

### Espacio de trabajo

**Tiene dos partes** (`DEC-09.5`):

- **la del motor**: su material, copiado e interpolado, que **se rehace entera** al empezar cada intento
  (`DEC-06.19`), y **el directorio de cada paso otra vez cuando le toca**, se re-ejecute o no: lo que un paso
  anterior haya dejado en el directorio de otro —ficheros o enlaces simbólicos— no llega a sus comandos. Al
  rehacerlo no se sigue ningún enlace: se borra el enlace y no aquello a lo que apunta, cada fichero se crea
  nuevo, las plantillas se interpolan desde lo declarado y no desde el disco, y si el directorio del ambiente
  es un enlace el paso no corre;
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
| **comandos** | ejecutar un comando en el espacio de trabajo de su ambiente y recoger su resultado. Al terminar cada comando (bien, mal o cancelado) se acaba con **todo** su grupo de procesos: un comando no puede dejar procesos en segundo plano para el siguiente. No aplica a un proceso que cree su propio grupo (`setsid`) ni en Windows. Lo que escribe se conserva hasta 16 MiB |
| **espacio de trabajo** | rehacer la parte del motor, entera al empezar y el directorio de cada paso cuando le toca |
| **directorio efímero** | un directorio vacío (0700) antes de cada paso que se ejecuta, borrado al terminar, salga como salga |
| **supervivientes** | acabar, al terminar cada paso, con los procesos que escaparon del grupo de su comando. Solo donde el motor es el proceso 1 de su contenedor |
| **salida** | el Historial: lo que imprimió cada comando, al terminar, para consultarlo con `logs` |

### La salida de los comandos

Lo que imprimen los comandos **no se muestra a quien pidió el intento: se guarda en el Historial**, un
registro por comando al terminar, con el paso, el nombre del comando, si salió bien y lo que escribió (salida
y error juntos). Se guarda también la de un comando que falló o que se canceló. Los registros viven aparte de
los del intento. No se tapan valores: la seguridad no es el objetivo del producto (`DEC-08.8`), y la promesa
de no exponer valores es sobre registros, consulta de intentos y diagnóstico, no sobre `logs`. **Una sola
excepción, de mejor esfuerzo (AJ-21):** antes de guardar la salida de un comando, lo que tiene forma de secreto
conocido se guarda como `[REDACTADO]`: un JWT (`eyJ…`.`…`.`…`), una clave privada en PEM, el valor de
`AccountKey=`, `SharedAccessKey=` y `SharedAccessSignature=`, la firma de una SAS (`?sig=`, `&sig=`) y el token
que sigue a `Bearer`. **No es una garantía** ni gestión de secretos: no detecta otras formas ni otras
codificaciones. **Lo que el comando captura con `probe` conserva su valor real, y así se guarda en el almacén**
(Resolución lo necesita para el paso siguiente y para los intentos que lo reutilicen): un pipeline que captura
un secreto lo deja escrito, en claro, para quien pueda leer el almacén.

De la salida de cada comando se conservan **16 MiB**. Lo que pase no se guarda, ni se usa para las aserciones
ni para capturar variables, y la salida guardada termina con `[SALIDA TRUNCADA: …]`. El comando no se entera:
su tubería se sigue vaciando. Un intento enseña como mucho **32** variables con `show`, y de cada valor,
**4 KiB**.

### La frontera de confianza es el intento, no el paso

Todos los pasos de un intento corren en el mismo contenedor, con el mismo usuario, uno detrás de otro. **Lo
que separa de verdad es el contenedor**: lo que no debe ver el código de un proyecto no debe estar en el
contenedor de su intento. Entre pasos el motor quita lo que puede quitar barato, para que algo que solo
existe durante un paso (un token de nube que quien invoca pone y quita, la sesión de una herramienta) no
quede al alcance de lo que corrió antes. Es reducir, no aislar.

| Medida | Qué quita | Qué no |
|---|---|---|
| Matar el grupo de cada comando | lo que el comando dejó en segundo plano | lo que creó su propia sesión (`setsid`) |
| Matar todos los procesos al terminar cada paso (`kill(-1)`) | también lo que escapó con `setsid`: ningún proceso de un paso sigue vivo en el siguiente | solo si `vexd` es el proceso 1 de un contenedor Linux (`ENTRYPOINT ["vexd"]`, sin `--init`). Con `--init`, fuera de un contenedor o en otro sistema no se mata nada y el límite de `setsid` sigue ahí |
| Rehacer el directorio de cada paso cuando le toca | ficheros y enlaces que un paso anterior dejó en el directorio de otro | lo que los pasos comparten a propósito: el resto del directorio del ambiente (el estado de terraform), el material del proyecto (`project_workdir`), las variables producidas, y los directorios de proyecto y pipeline que están por encima |
| Directorio efímero del paso (`VEX_STEP_TMP`) | lo que una herramienta guarde ahí —su sesión— no pasa al paso siguiente | solo lo que se guarde ahí. Una herramienta que escriba en `HOME` lo deja para los demás pasos |
| Validar los metadatos del proyecto | que un nombre de proyecto, equipo, organización o ambiente traiga código para el shell | el contenido del pipeline, que es quien escribe los comandos, y lo que un comando haga con los datos del proyecto |
| Redacción de la salida | las formas conocidas de secreto, en `logs` y en `visibles` | cualquier otra forma, y lo capturado con `probe` |
| Topes de salida y de `visibles` | que un comando agote la memoria del motor o use `visibles` para sacar datos en bloque | el disco: un comando puede llenar el espacio de trabajo |

Lo que **sigue siendo de todo el intento**, y un paso anterior puede usar contra uno posterior:

- **`HOME` y cualquier directorio en que el usuario pueda escribir**: `~/.bashrc`, `~/.m2/settings.xml`, la
  configuración de `git`, `~/.docker/config.json`. Un paso los cambia y los siguientes los leen.
- **Los binarios y bibliotecas del `PATH`** en que el usuario pueda escribir: un paso puede sustituir `az` o
  `terraform` por otro. La imagen debe dejarlos de `root` y correr con otro usuario.
- **El material del proyecto y el espacio de trabajo compartido**: un `pom.xml`, un script o un estado de
  terraform que un paso modifica es el que usa el siguiente.
- **El entorno de la petición** (`entorno`): llega igual a todos los comandos de todos los pasos.
- **El almacén**, si está montado con escritura: un comando puede leer salidas y variables de otros intentos.

Por eso el token de nube lo pone y lo quita quien invoca, por paso, y aun así **un pipeline no debe ejecutar
código del proyecto (pruebas, compilación) en el mismo intento en que usa credenciales que ese código no debe
tener**, salvo que acepte ese riesgo.

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

