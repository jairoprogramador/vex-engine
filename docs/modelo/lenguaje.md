# Lenguaje Ubicuo — Motor de Vex

> **Espacio del problema.** Aquí no se habla de código, paquetes ni ficheros: se habla de las
> palabras con las que este dominio se piensa. El impacto técnico vive en `../iteraciones/`.
>
> **Vigente** desde el cierre de IT-03 (2026-09-13). Nació en IT-02 partido por subdominio; IT-03
> trazó las fronteras y lo dejó partido **por contexto**. Si contradice a un documento de
> `iteraciones/`, manda éste. Se lee junto a `dominio.md` (**qué** es cada subdominio) y
> `bounded-contexts.md` (**dónde** acaba cada contexto).

---

## Cómo se lee este documento

**No existe un lenguaje ubicuo del sistema: existe uno por contexto.** Que un término signifique
dos cosas en dos contextos **no es un error**, es una frontera, y está en la tabla de homónimos. El
error es que signifique dos cosas **dentro** de un mismo contexto.

Cada apartado es un **bounded context** de `bounded-contexts.md`. Son ocho: los nueve subdominios
menos Sincronización del Historial, que se realiza dentro del Historial.

Cuando un término aparece en varios apartados, cada uno lo marca con *(aquí)* y dice lo que
significa en ese contexto.

Orden: primero el core, luego los supporting, luego el generic.

---

## Reglas del lenguaje

**1. Español, y el término del modelo *es* el identificador.** No hay tabla de traducción: lo que
aquí se llama *intento* se llama *intento* en el código. Es lo que hace comprobable el criterio de
`DEC-01.10`: un nombre se lee en voz alta a un DevOps y significa lo mismo que aquí, sin necesidad de
vigilar nada, porque se comprueba leyendo.

**2. Cuatro préstamos, lista cerrada**: **pipeline**, **rollback**, **hash** y **commit** (este
último, desde IT-03). Se conservan porque son las palabras que el actor usa de verdad. Cualquier otro
préstamo exige una decisión escrita.

**3. Se nombran hechos, no lecturas.** Un término que empuja a interpretar está mal elegido, aunque
sea preciso.

**4. Fuera las palabras que vienen del mecanismo.** *Caché*, *revivir* y *huella* no eran palabras
de ningún actor.

**5. Lo declarado tiene forma en Definición y significado en quien lo aplica** (IT-03 `DEC-03.6`).
*Ámbito*, *regla*, *variable de salida* y *orden de los ambientes* se escriben en el pipeline; qué
significan lo dice el contexto que los aplica.

**6. Palabra común, dueño en cada caso.** *Hash* se usa en cuatro contextos y siempre significa lo
mismo: una cadena que cambia si cambia lo que resume, y que solo responde *¿cambió?*. Por eso se dice
siempre calificado (*hash del código*, *hash de variable*), y cada hash concreto es del contexto que
lo calcula.

---

## Diagnóstico · *core*

Nombra la **causa** de un fallo comparando **el intento que falla** contra despliegues **del mismo
ambiente y del ambiente anterior en el orden**, leídos del historial, y solo entrega hechos.

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen: en el código del producto, en las instrucciones del pipeline o en las variables de este ambiente |
| **sustento** | lo que sostiene la atribución: qué ejes cambiaron, en qué pasos y qué variables, y entre qué dos momentos. Nunca quién hizo un cambio (IT-07 `DEC-07.7`) |
| **eje** | cada una de las tres cosas que se comparan: código, instrucciones y variables declaradas. Una variable producida no es eje, porque es consecuencia (IT-06 `DEC-06.10`) |
| **eliminación** | el procedimiento: descartar ejes hasta que queden uno o dos |
| **atribución** | lo que emite el core: a qué eje se asigna la causa |
| **deducción** | razonamiento **forzoso**: *«las instrucciones son idénticas; funcionó allí y falla aquí; lo único distinto es esta variable; luego la causa está en las variables»*. Es un hecho |
| **inferencia** | razonamiento **probable**: *«se repitió cuatro veces, probablemente sea del ambiente»*. **El core nunca infiere** |
| **orden de los ambientes** *(aquí)* | que lo que hay en un ambiente vino del anterior. Es lo que hace que comparar producción con staging signifique algo |

**Cuatro de sus términos nombran el método**, no el objeto. Eso distingue un ámbito propio de una
consulta sobre el ámbito de otro.

**Términos que consume, todos del Historial** (el core no lee a nadie más): *despliegue*, *intento*,
*cantidad de intentos*, *evidencia*, *paso* y *lanzamiento*, este último como puerta de entrada que
lleva a su despliegue. Y lo que el Historial guarda de otros contextos: *hash del código*, *hash de las
instrucciones de un paso* y *hash de variable*. El *hash del pipeline* no lo usa, porque
mezcla los tres ejes (IT-06 `DEC-06.9`).

**Un intento sin desenlace no tiene causa**: no es un fallo, y el core no lo atribuye a ningún eje (IT-05
`DEC-05.8`).

**Lo que el core no dice nunca**: el **valor** de una variable. Dice que `DB_POOL_SIZE` cambió, no
que pasó de 10 a 50.

---

## Definición de Pipeline · *supporting*

Cómo se despliega este producto y qué cambia en cada ambiente. Lo escribe el **DevOps**.
Definición **declara**; lo que declara lo aplican otros contextos.

| Término | Qué nombra |
|---|---|
| **pipeline** *(aquí)* | la declaración de cómo se despliega un producto. Tiene tres partes: la declaración de variables, las instrucciones de cada paso y la declaración de ambientes |
| **paso** *(aquí)* | la unidad declarada: un **nombre** y una lista de comandos. El nombre dice **cuál** es; los comandos, **si cambió**. Renombrar un paso corta su historia |
| **comando** | lo que un paso manda ejecutar. Un paso **sin comandos es un error de definición** |
| **material de un paso** | lo que el DevOps escribe en el directorio de un paso para que lo usen sus comandos: plantillas, manifiestos, ficheros de la tecnología |
| **instrucciones** | lo que el DevOps escribe para cada paso: sus comandos y el material de su directorio, sin sus variables ni su configuración. **No varían por ambiente**: es la premisa del core (IT-08 `DEC-08.7`) |
| **variable declarada** | una variable con su valor escrito en el pipeline, por paso y por ambiente |
| **ambiente** | la separación: dev, staging, producción. Cada uno tiene sus propias variables |
| **comprobación** | lo que se verifica sobre el pipeline antes de un intento: que lo escrito esté **bien formado y bien referenciado** |

**Formas declaradas.** Se escriben aquí y su significado es de otro contexto:

| Forma | Qué se escribe | Significa en |
|---|---|---|
| **ámbito** | qué variables ve cada paso | Resolución de Variables |
| **regla** | qué mira un paso para decidir si se re-ejecuta: el código del producto, sus instrucciones, las variables de su ámbito o el tiempo. Se escribe en el fichero de configuración de cada paso | Ejecución de Pipeline |
| **variable de salida** | un nombre y la **expresión regular** que dice qué forma tendrá | Ejecución de Pipeline y Resolución de Variables |
| **orden de los ambientes** | la secuencia dev → staging → producción | Diagnóstico |

**Qué comprueba la comprobación, y qué no.**

| Comprueba | No comprueba |
|---|---|
| el **formato** de lo declarado | nada del **acto de ejecutar** |
| que toda variable usada en un comando o en un fichero de configuración **esté declarada** (como variable de salida de un paso anterior o como variable declarada) **y sea visible según el ámbito escrito** | la **interpolación completa**: el valor de algunas variables solo aparece al ejecutar |
| que las **expresiones regulares** de las variables de salida sean correctas | |
| que ningún paso esté **sin comandos** | |

Con eso, un pipeline es verificable de principio a fin **sin tocar la nube**, que es lo que el
DevOps quiere antes de publicar.

---

## Historial · *supporting*

La memoria del negocio: qué se ha desplegado, cuándo, con qué y con qué resultado. Realiza también
**Sincronización del Historial**: lo que guarda está disponible en cualquier máquina.

| Término | Qué nombra |
|---|---|
| **registro** | **un hecho** concreto que quedó escrito |
| **historial** | el **conjunto** de todos los registros. Consultar es recorrerlo |
| **intento** *(aquí)* | un hecho: una ejecución de uno o varios pasos de un pipeline en un ambiente, ya ocurrida. Tiene identificador propio, sabe cuál fue el anterior y quién lo pidió |
| **estado** | cómo terminó un intento, según sus registros: **exitoso · fallido · cancelado**. Una cancelación **no es un fallo de nadie** |
| **sin desenlace** | un intento que tiene registros y ninguno que diga cómo terminó. **No es un estado**: es la ausencia del registro que lo da. El historial no sabe por qué falta (la máquina pudo morir, o el intento sigue en otra) |
| **abandonado** | un intento sin desenlace que alguien dio por abandonado. Libera su ambiente y ya no acepta registros. **No es un estado** (IT-07 `DEC-07.8`) |
| **ocupación** | qué intento tiene un ambiente en curso. Un ambiente tiene como mucho una (IT-07 `DEC-07.8`) |
| **despliegue** | un intento **exitoso de todos** los pasos de un pipeline en un ambiente. Tiene identificador propio, distinto del de su intento, y sabe cuál es su intento y cuál su **padre**. Solo nace de un intento hecho con commits: un intento con una copia de trabajo nunca llega a despliegue (IT-10 `DEC-10.7`) |
| **padre** | el despliegue del que se creó éste. Normalmente es el último; en un rollback, uno anterior, y por eso **dos despliegues pueden compartir padre** |
| **punto de retorno** | lo que es un despliegue: un sitio al que se puede volver |
| **lanzamiento** *(aquí)* | un registro que sabe cuál es su despliegue y cuál fue el lanzamiento anterior. La decisión de lanzar **no** es de aquí |
| **cantidad de intentos** | cuántos intentos hay **desde un despliegue hasta un intento posterior**. Se cuenta, no se guarda |
| **evidencia** | el **enlace** al registro con el que se hizo de verdad un paso que no se re-ejecutó |
| **paso** *(aquí)* | la posición bajo la que se guardan los registros de un paso **en su ámbito**. Se identifica por su nombre |
| **declaración congelada** | un paso tal como estaba declarado cuando se hizo |
| **solicitante** | quien pidió un intento |
| **clave** | por lo que se busca un registro, sin saber qué significa: la forma propia y lo que piden sus clientes (IT-07 `DEC-07.6`) |
| **sincronizar** | llevar y traer el historial para que esté disponible en otra máquina. Cada registro se lleva **en cuanto se escribe**, y el historial se trae antes de leerlo |
| **despliegue registrado** | el anuncio de que el historial guardó un despliegue nuevo. El Historial no sabe quién lo escucha |
| **reserva** *(aquí)* | un registro: desde cuándo el dueño del negocio decide él los lanzamientos de un ambiente, o cuándo lo liberó |

**La forma es suya; lo que dice cada registro, no** (IT-03 `DEC-03.13`). El Historial es dueño de
a qué intento pertenece un registro, cuál fue el anterior, cuál es el padre y las **claves** por las que sus clientes necesitan buscar, como el
hash del código, que para el Historial es una clave más (IT-07 `DEC-07.6`). Además guarda, **sin
aplicarles sus reglas**, cosas que son de otros contextos:

| Guarda | Es de |
|---|---|
| la razón por la que un paso no se re-ejecutó | Ejecución de Pipeline |
| el hash de las instrucciones de un paso | Ejecución de Pipeline |
| el hash de variable y el **valor, ofuscado** | Resolución de Variables |
| el hash del código, el hash del pipeline y el commit | Suministro de Fuentes |
| la versión y el nombre del lanzamiento | Lanzamiento |
| el orden de los ambientes con que se desplegó | Definición de Pipeline |

**Ofusca lo que guarda**, porque es quien persiste. El valor de una variable queda aquí ofuscado y
**solo Resolución de Variables lo pide de vuelta**. El valor **no forma parte de lo que el Historial
publica**, ni hacia otros contextos ni hacia fuera (IT-04 `DEC-04.7`).

**Un registro no está escrito hasta que se puede leer desde cualquier máquina** (IT-05 `DEC-05.9`,
IT-06 `DEC-06.18`). Si por debajo es un fichero o una base de datos queda escondido. Si un registro no se
puede escribir, el intento se detiene antes del siguiente paso y, en el historial compartido, queda sin
desenlace: no se hace en el mundo nada que no pueda quedar escrito.

**Tres reglas de forma que hay que saber decir:**

- **Cada despliegue tiene un intento, pero no todo intento tiene despliegue.** Un intento exitoso
  que ejecutó tres pasos de cinco **no tiene nombre propio**: es un intento, y se consulta como tal.
- **El despliegue no admite grados.** No hay medio despliegue ni despliegue incompleto.
- **«El último despliegue» es único**, y no por la forma de la historia, que se bifurca, sino porque
  **el tiempo es un orden total**: en un ambiente no hay dos despliegues a la vez.

**La referencia de una comparación es un parámetro**, no una regla. Por defecto hay dos: el despliegue
anterior en el mismo ambiente y el último del ambiente anterior en el orden con el mismo hash del
código. El usuario puede elegir otra (IT-06 `DEC-06.6`, `DEC-06.8`).

---

## Lanzamiento · *supporting*

Cuándo un despliegue se hace visible, a quién y con qué nombre sale. Lo decide el **dueño del
negocio** en los ambientes que él elija; en el resto se actúa en su nombre.

| Término | Qué nombra |
|---|---|
| **lanzar** | *«ahora sí: actívalo, anúncialo, hazlo visible»*. Es una decisión separada del despliegue y posterior a él |
| **lanzamiento** *(aquí)* | la decisión de hacer visible un despliegue a su destinatario. **Ocurre en cualquier ambiente**. Sabe cuál es su despliegue y cuál fue el lanzamiento anterior |
| **destinatario** | quien usa ese ambiente: el equipo en dev, quien valida en staging, el cliente final en producción |
| **reserva** *(aquí)* | la decisión del dueño del negocio de lanzar él mismo en un ambiente. Mientras no la libere, en ese ambiente no se lanza en su nombre |
| **fecha de publicación** | cuándo ocurre ese lanzamiento |
| **versión** | la **etiqueta técnica** del lanzamiento, un par clave-valor cuya clave es «version». El valor lo pone la herramienta: un número por proyecto que crece de uno en uno con cada código que se lanza por primera vez, y el mismo código conserva su versión en todos los ambientes (IT-10 `DEC-10.8`) |
| **nombre del lanzamiento** | la **etiqueta de negocio** del lanzamiento, un par clave-valor cuya clave es «nombre» y cuyo valor pone el dueño del negocio. Si no lo pone, **toma el valor de la versión** |

Que por defecto coincidan **no las funde**: una es etiqueta técnica y la otra de negocio. Es el mismo
patrón que el lanzamiento automático: **actuar en nombre del actor ausente**, nunca un efecto del
despliegue.

**Quién decide en cada ambiente** (IT-04 `DEC-04.1`): el dueño del negocio decide en los ambientes
que él elija, normalmente producción. En los demás rige la regla del actor ausente: se lanza solo en
cuanto el despliegue queda listo. Lanzamiento se entera escuchando *despliegue registrado*, que publica
el Historial (IT-04 `DEC-04.8`), y consultando la última **reserva** de ese ambiente (IT-04
`DEC-04.9`).

---

## Ejecución de Pipeline · *supporting*

Llevar a cabo un intento haciendo **solo el trabajo que hace falta**.

| Término | Qué nombra |
|---|---|
| **intentar** | pedir la ejecución de los pasos hasta uno dado, en un ambiente |
| **intento** *(aquí)* | el que se está llevando a cabo: puede fallar a mitad o cancelarse. Cuando termina, pasa a ser un hecho del historial |
| **ejecución** | **el acto** de ejecutar. **Nunca es una cosa identificable**: la entidad es el intento |
| **paso** *(aquí)* | la unidad que se ejecuta o no se re-ejecuta |
| **re-ejecución de un paso** | volver a hacer lo que hace un paso. La comparación es **por paso** |
| **no se re-ejecuta** | se dice siempre con su razón: *«no había nada diferente en los recursos de un paso, por lo tanto no se re-ejecuta»* |
| **recursos de un paso** | lo que un paso necesita para ejecutarse: instrucciones, variables y código del producto |
| **hash de las instrucciones de un paso** | dice si cambiaron las instrucciones de un paso: sus comandos o su material (IT-08 `DEC-08.7`). De los otros dos recursos lo dicen Suministro (el código) y Resolución (las variables) |
| **regla** *(aquí)* | lo que decide si un paso se re-ejecuta: se re-ejecuta si cambió, **desde su última vez**, algo de lo que la regla mira. **Cada paso decide con su propia información** |
| **última vez de un paso** | su último registro **en su ámbito**. No depende del ambiente: un paso de ámbito compartido puede no re-ejecutarse en el siguiente ambiente si lo que mira no cambió (IT-06 `DEC-06.12`). **Solo evita re-ejecutar si es un final exitoso**, o una no re-ejecución que apunta a uno; un comienzo sin final o un final fallido obligan a re-ejecutar (IT-09 `DEC-09.7`) |
| **variable** *(aquí)* | siempre el **valor**, el que entra en un comando. Ejecución nunca ve un hash de variable |
| **variable de salida** *(aquí)* | la que produce un comando, extraída de su salida |
| **rollback** | volver a un despliegue anterior **con los recursos con que se hizo**. Crea un despliegue nuevo, y **existe en cualquier ambiente** |
| **destino** | el despliegue al que vuelve un rollback. Por defecto, el anterior al último; se puede elegir cualquiera anterior |
| **aislamiento** | que un ambiente no pise a otro, y que un paso solo vea las variables de su ámbito. Los pasos de un mismo ambiente **comparten** su espacio de trabajo, a propósito (IT-09 `DEC-09.8`) |
| **espacio de trabajo** | la copia mutable **de cada ambiente** donde trabajan los pasos. **Es el mismo en cada intento de ese ambiente** y no interfiere con el de otro. Entre ambientes, lo que un paso le deja a otro ambiente son variables producidas y lo que nombran en el mundo (IT-06 `DEC-06.17`). **Es memoria**: vive en un lugar fijo por ambiente que el motor conoce, y guarda los archivos que genera la tecnología de los pasos, que el motor no interpreta (IT-06 `DEC-06.18`). Cada intento empieza con el material declarado: lo que pone el motor se vuelve a poner, y lo que generó la tecnología no se toca (IT-06 `DEC-06.19`). **Término interno**: no es un subdominio |

**Un negativo se dice con su razón, no con un verbo.** *No se re-ejecuta* nombra el hecho que lo
justifica; *revivir* u *omitir* nombraban un estado del objeto.

**Comparar y volver atrás son dos operaciones distintas.** Comparar es una consulta. Un rollback no
compara: **elige a dónde volver**.

---

## Resolución de Variables · *supporting*

Con qué valores concretos se despliega aquí, y si cambiaron, **sin que el valor circule**.

| Término | Qué nombra |
|---|---|
| **variable** *(aquí)* | el **valor efectivo** con el que se despliega en este ambiente |
| **ámbito** *(aquí)* | **qué variables ve un paso**. No es el ambiente: un ámbito puede *ser* el de un ambiente sin que sean lo mismo. Por defecto, el ámbito de un paso es **su ambiente**; también puede ser **compartido** entre ambientes (IT-06 `DEC-06.12`) |
| **precedencia** | qué valor gana cuando dos sitios le dan valor al mismo nombre |
| **origen** | de dónde sale el valor de una variable: escrito en el pipeline o producido por un paso |
| **variable de salida** *(aquí)* | un valor efectivo cuyo origen es «producido por un paso» |
| **interpolación** | sustituir una variable por su valor dentro de un comando o de un fichero |
| **hash de variable** | dice si una variable cambió sin mostrar su valor. **Solo nace aquí**. Es un hash **simple, sin clave**: no es una garantía de seguridad, y los secretos son asunto de otro producto (IT-08 `DEC-08.8`) |

**Su promesa al core**: se sabe **si** una variable cambió sin que su valor circule. Pérdida
aceptada: el core dirá *«`DB_POOL_SIZE` cambió»* y no *«pasó de 10 a 50»*. El valor no circula en claro,
pero un valor corto o previsible se puede adivinar desde su hash, y se acepta: la seguridad no es el
objetivo de este producto (IT-08 `DEC-08.8`).

**Quién tiene el valor en claro**: Resolución y el comando que se ejecuta, nadie más. En el historial
el valor queda ofuscado y solo Resolución lo pide de vuelta, por una relación que nadie más tiene
(IT-04 `DEC-04.7`).

**Petición sin intento** (IT-04 `DEC-04.10`): si lo que se le pide no pertenece a ningún intento,
Resolución resuelve e interpola, pero no calcula hashes, no deja nada en el historial y no conserva
nada. No sabe por qué no hay intento.

**Lo que no es suyo**: ofuscar lo que se guarda. No es cifrar, y le corresponde a quien persiste.

---

## Simulación de Pipeline · *supporting*

¿Va a funcionar este pipeline antes de publicarlo? Sirve al **DevOps que diseña**.

| Término | Qué nombra |
|---|---|
| **simulación** | un recorrido entero del pipeline **sin efectos y sin guardar nada**. Es la entidad: *una simulación*, como *un intento* en Ejecución |
| **simular un comando** | lo **único** que se finge |
| **salida simulada** | lo que devuelve un comando simulado: un valor que cumple la expresión regular de su variable de salida y que nadie produjo |

**La interpolación y la comprobación se hacen de verdad**, y por eso Simulación depende de
Definición de Pipeline, de Resolución de Variables y de Suministro de Fuentes.

**No lee el historial**: recorre siempre todos los pasos. Ninguna salida simulada llega al historial
ni al core. Para Resolución, una simulación es una **petición sin intento**: interpola sus salidas
simuladas sin saber que son inventadas, y no deja rastro (IT-04 `DEC-04.10`).

---

## Suministro de Fuentes · *generic*

Tener delante el material, como está hoy o como estaba, y saber si cambió.

| Término | Qué nombra |
|---|---|
| **fuente** | de dónde viene el material: el repositorio del producto o el del pipeline |
| **pipeline** *(aquí)* | una fuente: un repositorio cuyo contenido no se interpreta |
| **hash** *(aquí)* | el del código y el del pipeline: cambia si cambia el contenido del repositorio. Responde a **una sola pregunta: ¿cambió algo?** |
| **commit** | un punto de la historia de una fuente, con el que se puede **volver a tener delante** el material tal como era. Se guarda **para volver atrás**, no para saber quién cambió qué (IT-07 `DEC-07.7`) |
| **copia de trabajo** | el directorio donde alguien está trabajando, con cambios sin commit. Tiene hash, pero no commit (IT-10 `DEC-10.6`) |

**Hash y commit no son lo mismo.** Si alguien revierte un cambio, el commit es nuevo y el hash es el
de antes: solo el hash dice la verdad, que *no cambió*.

**Qué entra en el hash y qué no es técnica**, no dominio: nombres de fichero, permisos, fines de
línea, comentarios.

---

## Homónimos — y la frontera de cada uno

Un término con varios sentidos en varios contextos es legítimo. Cada fila es una frontera de
`bounded-contexts.md`, dicha desde el lenguaje.

| Término | Sentidos, por contexto | Qué cambia al cruzar |
|---|---|---|
| **paso** | nombre y comandos *(Definición)* · la unidad que se ejecuta o no *(Ejecución)* · la posición de sus registros *(Historial)* | de declarar a hacer, y de hacer a recordar |
| **intento** | el que se está llevando a cabo *(Ejecución)* · el hecho con estado *(Historial)* | de algo que cambia a algo que ya no cambia |
| **variable** | declarada *(Definición)* · valor efectivo *(Resolución)* · valor que entra en un comando *(Ejecución)* | de lo escrito a lo vigente, y de lo vigente a lo que se usa sin conservarlo |
| **variable de salida** | nombre y expresión regular *(Definición)* · lo que produce un comando *(Ejecución)* · un valor efectivo producido por un paso *(Resolución)* | de la forma al producto, y del producto a la precedencia |
| **ámbito** | lo escrito *(Definición)* · qué variables ve un paso *(Resolución)* | de la forma al significado |
| **regla** | lo escrito *(Definición)* · lo que decide si se re-ejecuta *(Ejecución)* | de la forma a una decisión con resultado |
| **orden de los ambientes** | la secuencia escrita *(Definición)* · de dónde vino lo que hay en un ambiente *(Diagnóstico)* | de la forma al significado |
| **pipeline** | la declaración *(Definición)* · una fuente *(Suministro)* | del envoltorio al contenido |
| **lanzamiento** | la decisión *(Lanzamiento)* · el registro *(Historial)* | de una decisión con actor a un hecho con forma |
| **reserva** | la decisión *(Lanzamiento)* · el registro *(Historial)* | igual que *lanzamiento* |

**No son homónimos:**

- ***hash***: es una palabra común con dueño en cada caso (regla 6). El del código y el del pipeline
  son de Suministro, el de las instrucciones de un paso es de Ejecución y el de variable es de
  Resolución.
- ***estado***: le queda un solo sentido, cómo terminó un intento (Historial).
- ***evidencia***: solo existe en el Historial, y es el enlace.

**Ámbito y ambiente no son homónimos**: son dos términos distintos que se confunden al hablar. Ver
abajo.

---

## Sinónimos resueltos

Lo único que este documento **no** admite son dos términos para una sola cosa.

| Se retira | Se queda | Por qué |
|---|---|---|
| parámetro | **variable** | era el nombre que una parte del sistema le daba a la variable; no aportaba matiz |
| tarea | **comando** | *tarea* era «acción a realizar, típicamente un comando»: una generalidad que nadie usa |
| huella · firma · instantánea | **hash** | *firma* sugiere autoría; *instantánea*, que se guardó el contenido; *huella* nombraba el mecanismo |
| marca *(del valor)* | **hash de variable** | hace el mismo trabajo que cualquier hash: responder *¿cambió?* |
| revivir · omitir | **no se re-ejecuta** | nombraban un estado del objeto en vez del hecho que lo justifica |
| reincidencia · repetición | **cantidad de intentos** | nombraban una lectura; un número contado no se puede leer como un juicio |
| plan automático · validación | **comprobación** | *validación* acabó nombrando dos momentos distintos |
| caché | **índice** | venía de una carpeta, y lo que nombra es derivable y no participa en ninguna decisión |
| el último resultado exitoso | **el último despliegue en ese ambiente** | nombraba dos cosas: el último despliegue y el último lanzamiento |
| Registro de Despliegue *(subdominio)* | **Historial** | se nombraba con la palabra de la parte |
| Sincronización de Estado *(subdominio)* | **Sincronización del Historial** | se nombraba por la carpeta que copiaba |

---

## Pares que se confunden

Distinciones que hay que poder hacer sin dudar.

| | No es lo mismo que | La diferencia |
|---|---|---|
| **desplegar** | **lanzar** | desplegar es colocar el código sucesivamente en dev → staging → producción, y cada paso es una **preparación**. Lanzar es *«ahora sí: actívalo»*: una decisión **separada, posterior y de otro actor** |
| **deducción** | **inferencia** | la deducción es forzosa y es un hecho; la inferencia es probable. El core **solo deduce** |
| **comparar** | **volver atrás** | comparar es una consulta: un intento contra un despliegue de referencia; un rollback **no compara, elige a dónde volver** |
| **registro** | **historial** | un registro es **un hecho**; el historial es el **conjunto** |
| **ámbito** | **ambiente** | el ambiente es la **separación** (dev, staging, producción); el ámbito es **qué variables ve un paso** |
| **versión** | **nombre del lanzamiento** | la versión es la etiqueta **técnica**; el nombre lo pone el **dueño del negocio**. Coinciden por defecto y no son lo mismo |
| **hash** | **commit** | el hash responde *¿cambió?*; el commit, *¿cómo vuelvo a tenerlo delante?* |
| **padre** | **destino** | el padre es una relación del Historial; el destino es a dónde vuelve un rollback, en Ejecución. El destino de un rollback acaba siendo el padre del despliegue nuevo |
| **evidencia** | **razón** | la evidencia es el enlace al registro que vale (Historial); la razón, *«no había nada diferente»*, la da Ejecución |

---

## Términos que no existen en este modelo

- **«Plan automático»**: se partió en **comprobación** (Definición) y **atribución** (Diagnóstico).
- **«Espacio de Trabajo» como subdominio**: es *cómo* Ejecución cumple lo suyo, y sobrevive como
  término interno.
- **«Último ambiente»**: salió en IT-03, porque un lanzamiento ocurre en cualquier ambiente.
- **«Decidir / Planificación del Intento»** como contexto: es táctica dentro de Ejecución (E9).
- **«Caché»**, **«huella»**, **«marca»**, **«parámetro»**, **«tarea»**, **«revivir»**,
  **«reincidencia»**, **«validación»**: retirados arriba, con su razón.
- **«Autor»**, y los commits como sustento: quién cambió qué es asunto de la herramienta de
  repositorios, no de esta (IT-07 `DEC-07.7`).
- **La concurrencia.** El motor es one-shot: cada intento empieza de cero y muere al terminar, y **un
  ambiente no admite dos intentos a la vez** (IT-07 `DEC-07.8`). El único vocabulario de simultaneidad es
  *ocupación*.
