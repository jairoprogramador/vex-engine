# IT-06 — Diagnóstico *(el core)*

> Etapa: E6 · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §6–§13 *(el bloque táctico, escrito al abrir esta iteración)*
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **10 incongruencias · 20 decisiones.** El core no salió vacío: tiene lenguaje, reglas y un modelo
> táctico propio, sin agregados porque no cambia nada. Tres de las incongruencias (`Q-06.7` a `Q-06.9`)
> eran de Ejecución y salieron aquí. **Archivo cerrado: no se vuelve a editar.**

---

## 1. Qué se quiere

**Según el plan** (E6): modelar el core **empezando por los escenarios** que responde, no por las
entidades. Lo que el core necesite se convierte en requisito de quien está arriba. **Riesgo
declarado**: es el contexto del que menos se sabe; si sale vacío, la clasificación de E1 estaba mal y
se vuelve a E1.

**Cierre común del bloque B**: el contexto tiene escritos:

- sus agregados, con sus invariantes y la transacción que los protege;
- sus entidades y value objects;
- sus eventos de dominio;
- sus servicios de dominio;
- sus repositorios y factorías;
- sus servicios de aplicación;
- sus puertos hacia otros contextos.

**La vara del frente 2 de `DEC-01.10`**, que el plan manda citar al abrir cada iteración de este
bloque. Una propuesta de diseño entra solo si cumple al menos una de estas cuatro:

- baja acoplamiento medible;
- fuerza una regla por construcción;
- simplifica un test existente;
- se puede revertir tocando un solo contexto.

**Verificaciones que decisiones anteriores dejaron para E6:**

| Decisión | Qué hay que comprobar |
|---|---|
| `DEC-01.2` | Cada escenario termina en un subconjunto de los tres ejes, y ninguno necesita un cuarto |
| `DEC-01.6` | Diagnóstico responde sus escenarios sin leer ningún valor |
| `DEC-01.11` | Ningún escenario entrega la palabra «probablemente» |
| `DEC-01.12` | Todo escenario se resuelve nombrando qué ejes cambiaron y cuáles no |
| `DEC-02.4` | Todo escenario nombra dos despliegues, nunca un lanzamiento. Reescrita por `DEC-06.6` y `DEC-06.7`: todo escenario nombra un intento y un despliegue de referencia, y un lanzamiento solo es una puerta de entrada |
| `DEC-05.8` | Ningún escenario atribuye causa a un intento sin desenlace |

---

## 2. Propuesta

Los escenarios están en `modelo/contextos/diagnostico.md`, junto con lo que Diagnóstico le pide al
Historial. Lo que los sostiene:

1. **Primero los escenarios, después el modelo táctico** (`DEC-06.1`). El libro empieza por lo que el
   modelo responde. Además, aquí no hay código que empuje hacia unas entidades concretas.
2. **Los escenarios salen del modelo, no se inventan.** ES-1 y ES-2 son las dos escenas de
   `DEC-01.12`. ES-3 es la puerta del dueño del negocio (`DEC-02.4`). ES-4 es el *agujero en la
   comparación* de la cadena de garantías, que `Q-03.23` convirtió en enlace. ES-7 aplica `DEC-05.8` y
   que una cancelación no es un fallo de nadie. ES-8 es la cantidad de intentos.
3. **Tres respuestas que el core tiene que poder dar sin sonar a fallo:** *«ningún eje cambió»*
   (ES-5), *«no hay referencia»* (ES-6) y *«no se atribuye»* (ES-7). Las tres son hechos.
4. **Lo que el core necesita, dicho como requisito.** La tabla del Historial en `diagnostico.md` es la
   primera vez que el core dicta lo que publica un supporting.
5. **Lectura previa de lo que vendrá**: el core no escribe nada, así que es posible que no tenga
   agregados (`guia-ddd.md` §10). No se decide todavía.

---

## 3. Incongruencias

### Q-06.1 — ¿El core compara solo dentro de un ambiente, o también entre ambientes? · **resuelta**

**La incongruencia.** `dominio.md` dice dos cosas que no pueden ser verdad a la vez:

- *«El core compara siempre dos despliegues **del mismo ambiente**»*, que es la enmienda de IT-02
  (`DEC-02.4`).
- Su escena más fuerte es *«funcionó en staging y falla en producción»*, que compara **dos ambientes**.
  Es la que elimina dos ejes de golpe y la que sostiene el «en segundos» de la ventaja competitiva
  (`DEC-01.12`).

Si solo compara dentro de un ambiente, la ventaja competitiva pierde su mitad más fuerte, y el orden
de los ambientes que Definición le promete al core no tendría uso.

**Propuesta.** El core compara **dos despliegues, del mismo ambiente o de dos ambientes del orden**.
El procedimiento es el mismo (eliminación sobre tres ejes); lo único que cambia es cuántos ejes quedan
eliminados de entrada. **Por defecto entrega las dos comparaciones a la vez**, porque mostrar las dos
es mostrar hechos:

- contra el despliegue anterior en el mismo ambiente (ES-1);
- contra el último despliegue en el ambiente anterior del orden, si existe (ES-2).

El usuario puede elegir otra referencia. `DEC-02.4` se enmienda solo en *«del mismo ambiente»*.

**La alternativa que cambiaría el resultado.** Mantener *«del mismo ambiente»* y que comparar entre
ambientes consista en elegir a mano, como referencia, un despliegue de otro ambiente. El modelo tiene
un caso menos, pero la escena que da la ventaja competitiva depende de que el usuario sepa pedirla.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.6`).

---

### Q-06.2 — Lo que falla, ¿es un despliegue o un intento? · **resuelta**

**La incongruencia.** `DEC-02.4` dice que *todo escenario nombra dos despliegues*. Pero un despliegue
es, por definición, un intento **exitoso** de todos los pasos. Cuando *«hoy falla»*, lo que falla puede
ser dos cosas distintas:

- **un intento fallido**: un comando falló durante el despliegue. Es el caso más frecuente del
  programador, y lo que falla **no es un despliegue**;
- **un despliegue que funciona mal en uso**: el intento salió bien, pero el producto falla delante de
  quien lo usa. Eso el motor no lo ve; lo dice quien pregunta.

Con *«dos despliegues»*, el primer caso, que es el más frecuente, no tiene escenario.

**Propuesta.** El core compara **un intento contra un despliegue de referencia**:

- **el lado que falla es siempre un intento**: uno fallido, o el intento de un despliegue que quien
  pregunta dice que falla. *Que falla* lo dice el estado del intento o lo dice la persona; el core no lo
  comprueba, y lo toma como premisa de la pregunta;
- **la referencia es siempre un despliegue**, porque es un punto de retorno, algo que funcionó;
- en un intento fallido, los pasos a los que no llegó **no entran** en la comparación de variables,
  porque no tienen recursos en ese intento.

La verificación de `DEC-02.4` se reescribe: *todo escenario nombra un intento y un despliegue de
referencia; un lanzamiento solo es una puerta de entrada*.

**La alternativa que cambiaría el resultado.** Diagnosticar solo despliegues que fallan en uso, y dejar
los intentos fallidos con su propio error: el comando que falló y su salida. El modelo es más simple,
pero el programador pierde la respuesta a *«¿es mío?»* justo cuando un comando del pipeline falla por
una variable del ambiente.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.7`).

---

### Q-06.3 — En el ambiente anterior, ¿contra qué despliegue se compara? · **resuelta**

**La incongruencia.** `DEC-06.6` dejó como referencia por defecto *el último despliegue del ambiente
anterior*. Pero la escena *«funcionó en staging y falla en producción»* habla del **mismo código** que
funcionó en staging. Si, después de pasarlo a producción, se desplegó código nuevo en staging, *el
último* de staging ya no es el que funcionó con ese código. La comparación dirá que el código cambió,
quedarán dos candidatos, y la escena pierde justo lo que la hacía fuerte.

**Propuesta.** La referencia en el ambiente anterior es **el último despliegue de ese ambiente con el
mismo hash del código** que el intento que falla. Si no existe ninguno, el core lo dice (*«este código
no se desplegó en staging»*) y no hace la comparación entre ambientes; la del mismo ambiente sigue
saliendo. De paso se precisa ES-2: el código y las instrucciones quedan descartados porque sus hashes
coinciden, no porque lo diga la premisa.

**La alternativa que cambiaría el resultado.** Mantener el último despliegue del ambiente anterior,
tenga el código que tenga. Siempre habría comparación entre ambientes, pero muchas veces con dos
candidatos donde podría haber uno.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.8`).

---

### Q-06.4 — El eje *instrucciones*, ¿con qué hash se compara? · **resuelta**

**La incongruencia.** Al reescribir ES-2 en la vuelta anterior dejé que el código y las instrucciones
se descartan *si coinciden el hash del código y el del pipeline*. Pero el pipeline tiene tres partes
(variables, instrucciones de cada paso y ambientes), y su hash cambia si cambia **cualquiera** de
ellas. Si el DevOps cambia una variable declarada de producción, el hash del pipeline cambia y el core
diría *«las instrucciones cambiaron»* sin que haya cambiado ningún comando. La eliminación fallaría
justo en el caso que tiene que resolver, y la mitad de la promesa que se cumple siempre, *«¿tocó alguien
el pipeline?»*, daría una respuesta falsa.

**Propuesta.** El eje instrucciones se compara con **el hash de las instrucciones de cada paso**, que
solo cubre los comandos, paso a paso y en los pasos que tiene el intento. El core **no usa el hash del
pipeline**, porque mezcla los tres ejes. Se corrige ES-2, y el hash del pipeline sale de la tabla de
lo que Diagnóstico le pide al Historial.

**La alternativa que cambiaría el resultado.** Seguir comparando con el hash del pipeline y aceptar que
un cambio en las variables declaradas marque también las instrucciones. Habría menos que pedirle al
Historial, pero el eje instrucciones ya no se podría descartar cuando cambian las variables, y la
deducción se rompería.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.9`).

---

### Q-06.5 — Una variable que produjo un paso, ¿cuenta en el eje variables? · **resuelta**

**La incongruencia.** El eje variables compara el valor efectivo de las variables de cada paso, y ese
valor también puede nacer al ejecutar: es la variable de salida que produce un comando, como un
identificador de recurso o una etiqueta de imagen. Pero una variable producida **cambia porque su
paso se volvió a ejecutar**, y un paso se re-ejecuta porque cambió algo de sus recursos.

Ejemplo: cambia el código, se re-ejecuta el paso que construye la imagen, que produce una etiqueta
nueva, y un paso posterior falla. El core vería dos ejes cambiados, código y variables, y mostraría dos
candidatos. Pero el cambio de la variable **es consecuencia** del cambio del código. La deducción pierde
exactitud justo en el caso normal.

**Propuesta.** El eje variables compara **solo las variables declaradas**, las que el DevOps escribe
para cada ambiente. Las variables producidas no cuentan como causa: aparecen en el sustento (*«esta
variable producida cambió»*). Y si lo único que cambió es una variable producida, la respuesta es ES-5,
*ningún eje cambió*, con ese cambio en el sustento, que es exactamente lo que ocurre cuando algo de
fuera devuelve otro valor. Para distinguirlas, lo que el core le pide al Historial incluye el origen de
cada variable.

**La alternativa que cambiaría el resultado.** Que el eje variables cuente todas, declaradas y
producidas. Es la lectura literal, pero cada re-ejecución que produce un valor nuevo mantiene vivo el
eje variables, y en el caso normal quedarían dos candidatos donde hay uno.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.10`).

---

### Q-06.6 — Las dos comparaciones, ¿se quedan separadas o dan una sola respuesta? · **resuelta**

**La incongruencia.** `DEC-06.6` entrega **dos comparaciones**, una en el mismo ambiente y otra con el
ambiente anterior. Pero la escena que sostiene la ventaja competitiva es **una deducción**:
*«funcionó en staging y falla en producción, luego la causa está en las variables»*. Con las dos
comparaciones por separado, el caso normal se ve así:

- **mismo ambiente**: el código y las variables cambiaron desde ayer, así que hay dos candidatos;
- **ambiente anterior**: este mismo código con estas mismas instrucciones funcionó en staging, así que
  hay un candidato: las variables.

Las dos son hechos, pero el programador tiene que juntarlas él para llegar a la respuesta, y el *«en
segundos»* depende de que sepa hacerlo.

**Propuesta.** La respuesta **combina** las dos comparaciones: un eje queda descartado si **cualquiera**
de ellas lo descarta, y los candidatos son los ejes que sobreviven a las dos. Sigue siendo deducción y
no inferencia: descartar un eje en una comparación es forzoso, y lo sigue siendo aunque la otra no lo
descarte. Las dos comparaciones quedan visibles en el sustento. En el ejemplo, la respuesta es *«la
causa está en las variables»*.

**La alternativa que cambiaría el resultado.** Entregar las dos comparaciones sin combinarlas y dejar
que la persona saque la conclusión. El core tiene una regla menos, pero la respuesta más valiosa del
producto se queda sin dar.

**Respuesta.** **Sí** (`DEC-06.11`). Y el experto explicó cómo decide un paso si se re-ejecuta
(`DEC-06.12`):

> *«Cada paso tiene su archivo de configuración […] en él se especifica qué se debe comparar […]. Si
> cambió, entonces se ejecuta nuevamente; si no, no se ejecuta […]. Se pueden especificar las
> variables, en este caso el ámbito de las variables; por defecto el ámbito es el propio ambiente, pero
> también pueden tener un ámbito compartido […]. Esto no depende de un ambiente; lo más parecido a un
> ambiente es el ámbito de las variables. Eso quiere decir que un paso puede no ejecutarse en el
> siguiente ambiente si las condiciones que hacen que se ejecute no han cambiado […]. Cada paso tiene su
> propia información […]. Es muy diferente al espacio de trabajo, que sí es por ambiente, y este no
> interfiere en otro espacio de trabajo de otro ambiente.»*

---

### Q-06.7 — Lo que un paso deja a los siguientes, ¿dónde vive si ese paso puede no volver a ejecutarse? · **resuelta, con corrección**

> No es de Diagnóstico sino de Ejecución, pero sale de la explicación de `DEC-06.12`, y aquí se analiza.
> También responde la duda **#5** de IT-01: si el espacio de trabajo tiene continuidad entre
> ejecuciones.

**La incongruencia.** Tres cosas del modelo, ciertas por separado:

1. **Un paso puede no re-ejecutarse**, incluso en otro ambiente, si su ámbito es compartido y lo que mira
   no cambió (`DEC-06.12`).
2. **El espacio de trabajo es de cada ambiente** y no interfiere con el de otro.
3. **Nada vive entre invocaciones** salvo el Historial, y lo que se guarde para ir más rápido tiene que
   poder borrarse sin que cambie ninguna decisión (`DEC-05.10`).

Juntas: supongamos que un paso *construir* deja archivos en el espacio de trabajo y un paso *desplegar*
los necesita. En producción, *construir* no se re-ejecuta, porque el código no cambió desde staging, y
*desplegar* no encuentra los archivos: están en el espacio de trabajo de staging, o en el de una
invocación que ya terminó. El paso se da por bueno sin rehacerlo, y lo que dejó no está.

**Propuesta.** El espacio de trabajo **solo vive durante un intento**. Cuando un paso puede no volver a
ejecutarse, lo único que puede dejarle a los siguientes son **variables producidas** (su valor queda
ofuscado en el Historial) y lo que esas variables nombran **en el mundo**, como una imagen en un
registro o un recurso en la nube. Si un paso necesita archivos que otro genera en el espacio de trabajo,
los dos tienen que ejecutarse en el mismo intento, así que la regla del segundo tiene que mirar al menos
lo mismo que la del primero.

**La alternativa que cambiaría el resultado.** Que el espacio de trabajo de cada ambiente **se conserve
entre intentos** y viaje con el historial. Dentro de un ambiente los archivos seguirían ahí, pero pasaría
a ser una segunda memoria junto al Historial. Y entre ambientes no resolvería nada: un paso que no se
re-ejecuta en producción seguiría sin los archivos de staging.

**Respuesta.** *«Casi, pero el espacio de trabajo es por ambiente, y no vive solo durante el intento:
es el mismo en cada intento del mismo ambiente.»* Se adopta la propuesta con esa corrección
(`DEC-06.17`).

---

### Q-06.8 — Un espacio de trabajo que se conserva, en un motor que no recuerda nada · **resuelta, con corrección**

**La incongruencia.** `DEC-06.17` dice que el espacio de trabajo de un ambiente **es el mismo en cada
intento**. Pero el modelo dice que *el motor no recuerda nada entre intentos* y que *toda continuidad
está en el Historial* (`dominio.md`, `arquitectura.md`), y que lo que se guarde para ir más rápido *se
puede borrar sin que cambie ningún resultado* (`DEC-05.10`). El espacio de trabajo no es eso: si se
borra, un paso que no se re-ejecutó deja a los siguientes sin sus archivos. Además, un intento puede
correr en otra máquina. El Historial está disponible en cualquiera; el espacio de trabajo, tal como está
ahora, no.

El riesgo es concreto: el historial dice *«este paso no hace falta re-ejecutarlo»*, y el espacio de
trabajo que lo respaldaba no está en esta máquina.

**Propuesta.** El espacio de trabajo de cada ambiente **es memoria**, igual que el historial, y se
declara así:

- **viaja con el historial**: se lleva y se trae para que esté disponible en cualquier máquina;
- un paso **solo se da por hecho** si su registro está en el historial **y** el espacio de trabajo de
  su ambiente está disponible. Si no se pudo traer, los pasos de ese ambiente se re-ejecutan;
- se enmiendan la restricción de `dominio.md`, que pasa a decir que la continuidad está en el historial
  y en los espacios de trabajo, y `DEC-05.10`, que sigue valiendo para copias e índices.

**La alternativa que cambiaría el resultado.** Que el espacio de trabajo se quede en la máquina donde
corre el motor. Es más simple, pero un intento en otra máquina lo encontraría vacío mientras el
historial dice que no hace falta rehacer nada, y los pasos siguientes fallarían sin que nada avise.

**Respuesta.** *«Al ser máquinas efímeras, sí, pero hay un detalle: estas máquinas tienen un volumen
montado que siempre apunta al mismo lugar, por lo tanto siempre tienen ese espacio de trabajo por cada
ambiente. Pero este espacio es simplemente un conjunto de archivos generados según la tecnología que se
use […]. El historial es algo muy diferente: son los registros, que no necesariamente están en el mismo
lugar […]. Pensemos en una interfaz donde podemos escribir o leer registros; pueden ser archivos locales
o una base de datos, pero eso está escondido. Lo importante es saber que el contenedor sabe de alguna
manera cuál es su espacio de trabajo y cuáles son sus registros.»* Se adopta con esa corrección
(`DEC-06.18`).

---

### Q-06.9 — ¿Qué se restaura cuando un intento falla, si el espacio de trabajo se conserva? · **resuelta**

**La incongruencia.** `dominio.md` dice que Ejecución se ocupa de *que un intento fallido no deje el
material a medio tocar*. Mientras el espacio de trabajo vivía solo durante un intento, restaurar era
inofensivo. Ahora se conserva, y guarda los archivos que genera la tecnología, como el estado de
terraform, que refleja lo que **ya existe en el mundo**. Si un paso de terraform falla a mitad, ya creó
recursos, y su estado lo dice. Si ese estado se restaurara al de antes, el siguiente intento no sabría
que esos recursos existen y los volvería a crear.

**Propuesta.** Separar dos cosas que viven en el espacio de trabajo:

- **Lo que pone el motor**: el material del pipeline que copia e interpola para trabajar, como las
  plantillas con las variables ya sustituidas. **Se vuelve a poner al empezar cada intento**, a partir
  del pipeline declarado, en lugar de restaurarse cuando falla. Así también funciona cuando el proceso
  muere a mitad (un intento sin desenlace), donde no queda nadie para restaurar nada.
- **Lo que genera la tecnología** al ejecutar, como el estado de terraform. **No se toca nunca**, porque
  dice lo que pasó en el mundo.

En `dominio.md`, *que un intento fallido no deje el material a medio tocar* pasa a ser *que cada
intento empiece con el material declarado, sin tocar lo que la tecnología dejó*.

**La alternativa que cambiaría el resultado.** Restaurar, cuando el intento falla, lo que puso el motor,
que es lo que decía el modelo hasta ahora. Funciona cuando el intento llega a fallar, pero no cuando el
proceso muere, que es justo el caso de un intento sin desenlace.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.19`).

---

### Q-06.10 — El autor de un cambio: ¿de cada variable, o de los commits que hubo entre medias? · **resuelta**

**La incongruencia.** El sustento promete *el autor de cada cambio y cuándo*, y `DEC-06.16` promete
además, entre ambientes, *el último cambio de cada declaración que difiere*. Pero lo que Suministro sabe
es quién hizo cada **commit** de un repositorio, y no interpreta el pipeline: no sabe qué commit cambió
la variable `DB_POOL_SIZE` ni las instrucciones del paso *desplegar*. Para decir *quién cambió esa
variable*, alguien tendría que leer el pipeline commit a commit, y eso es trabajo de Definición. Tal como
está el mapa, nadie lo hace, y el core solo lee el Historial.

**Propuesta.** El sustento da **lo que se sabe sin interpretar el pipeline**:

- **dentro de un ambiente**: por cada eje que cambió, **los commits de su repositorio entre la referencia
  y el intento, con autor y fecha** (el repositorio del producto para el código, y el del pipeline para
  las instrucciones y las variables). No precisa qué commit cambió cada variable o cada paso. Suelen ser
  uno o dos commits;
- **entre ambientes**: una diferencia no es un cambio y no hay commits entre medias, así que el sustento
  dice qué variables difieren, **sin autor**. Se revoca la segunda mitad de `DEC-06.16`.

**La alternativa que cambiaría el resultado.** Precisar por variable y por paso. Definición leería el
pipeline de cada commit, y el Historial conservaría qué cambió en cada uno. El sustento diría exactamente
quién cambió cada cosa, también entre ambientes, a cambio de una responsabilidad nueva y de leer el
pipeline commit a commit.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-06.20`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-06.1 — Primero los escenarios; el modelo táctico, cuando no tengan incongruencias *(por defecto)*

**Por qué.** Es el orden del libro y el que fija el plan para E6.

**Verificación.** `contextos/diagnostico.md` no tiene agregados ni value objects escritos mientras
quede una incongruencia abierta en sus escenarios.

### DEC-06.2 — *«Ningún eje cambió»* es una respuesta del core *(por defecto)*

**Decisión.** Si la eliminación descarta los tres ejes, el core lo dice (ES-5).

**Por qué.** El subconjunto vacío también es un subconjunto: `DEC-01.2` se cumple sin inventar un cuarto
eje. Y saber que la causa **no** está en el código, ni en las instrucciones, ni en las variables también
encamina: nadie de los dos equipos tiene que buscar en lo suyo.

### DEC-06.3 — *«No hay referencia»* es una respuesta del core *(por defecto)*

**Decisión.** Si no existe un despliegue contra el que comparar, el core lo dice y no atribuye (ES-6).

**Por qué.** Un primer despliegue o un ambiente nuevo no tienen contra qué compararse, y decirlo es un
hecho. `Q-02.4` ya lo había anticipado: tiene que poder decirse sin que suene a fallo.

### DEC-06.4 — Un intento cancelado o sin desenlace no se atribuye *(por defecto)*

**Decisión.** Ante un intento cancelado o sin desenlace, el core no nombra ninguna causa (ES-7).

**Por qué.** *Una cancelación no es un fallo de nadie* (`DEC-02.3`), y de un intento sin desenlace no se
sabe cómo terminó (`DEC-05.8`).

### DEC-06.5 — Un paso que no se re-ejecutó se compara por su evidencia *(por defecto)*

**Decisión.** Para ese paso, los recursos que se comparan son los del registro al que apunta su
evidencia (ES-4).

**Por qué.** Es lo que resuelve *el agujero en la comparación* de la cadena de garantías, y es para lo
que `Q-03.23` dejó la evidencia como enlace.

### DEC-06.6 — El core compara dentro de un ambiente y entre ambientes, y entrega las dos comparaciones *(respuesta a `Q-06.1`)*

**Decisión.** La referencia puede ser un despliegue **del mismo ambiente** o **de otro ambiente del
orden**. Por defecto, el core entrega las dos comparaciones: contra el despliegue anterior en el mismo
ambiente, y contra el último despliegue del ambiente anterior en el orden, si existe. El usuario puede
elegir otra referencia.

**Por qué.** La comparación entre ambientes es la que elimina dos ejes de golpe y sostiene la ventaja
competitiva. Mostrar las dos comparaciones es mostrar hechos; elegir entre ellas le toca a la persona.

**Consecuencias.** **Enmienda `DEC-02.4`** en *«del mismo ambiente»*. El orden de los ambientes que
promete Definición tiene por fin un uso concreto: decidir cuál es *el ambiente anterior*.

**Qué descarta.** Comparar entre ambientes solo si el usuario elige a mano la referencia.

**Verificación.** ES-1 y ES-2 salen juntos cuando existe un ambiente anterior.

### DEC-06.7 — Lo que falla es siempre un intento; la referencia, siempre un despliegue *(respuesta a `Q-06.2`)*

**Decisión.** El core compara **un intento contra un despliegue de referencia**. El intento es uno
fallido, o el de un despliegue que quien pregunta dice que falla. *Que falla* lo dice el estado del
intento o la persona, y el core lo toma como premisa sin comprobarlo. La referencia es un despliegue:
algo que funcionó. En un intento fallido, los pasos a los que no llegó no entran en la comparación de
variables.

**Por qué.** Con *«dos despliegues»*, el caso más frecuente del programador, un comando que falla al
desplegar, no tenía escenario.

**Consecuencias.** La verificación de `DEC-02.4` se reescribe: *todo escenario nombra un intento y un
despliegue de referencia; un lanzamiento solo es una puerta de entrada*. La *cantidad de intentos* se
cuenta desde el despliegue de referencia hasta el intento que falla.

**Qué descarta.** Diagnosticar solo los despliegues que fallan en uso.

**Verificación.** En ningún escenario la referencia es un intento que no sea despliegue.

### DEC-06.8 — En el ambiente anterior, la referencia es el último despliegue con el mismo código *(respuesta a `Q-06.3`)*

**Decisión.** La referencia por defecto en el ambiente anterior es el último despliegue de ese ambiente
**con el mismo hash del código** que el intento que falla. Si no existe, el core lo dice (*«este código
no se desplegó en <ambiente>»*) y no hace esa comparación; la del mismo ambiente sigue saliendo.

**Por qué.** *«Funcionó en staging»* habla del mismo código. Si la referencia fuera *el último* a secas,
bastaría un despliegue posterior en staging para que el código apareciera como cambiado y la escena
perdiera su candidato único.

**Consecuencias.** Precisa `DEC-06.6`. En ES-2, un eje se descarta porque sus hashes coinciden, no
porque lo diga la premisa.

**Qué descarta.** El último despliegue del ambiente anterior, tenga el código que tenga.

**Verificación.** En ES-2, el hash del código de la referencia y el del intento que falla son siempre
iguales.

### DEC-06.9 — El eje instrucciones se compara paso a paso, con el hash de sus comandos *(respuesta a `Q-06.4`)*

**Decisión.** El eje instrucciones se compara con el hash de las instrucciones de cada paso, en los
pasos que tiene el intento. El core no usa el hash del pipeline.

**Por qué.** El hash del pipeline cambia si cambia cualquiera de sus tres partes. Con él, un cambio en
las variables declaradas marcaría también las instrucciones, y la eliminación dejaría de funcionar.

**Consecuencias.** Corrige ES-2, que se había reescrito con el hash del pipeline en la vuelta anterior.
Lo que Diagnóstico le pide al Historial deja de incluir ese hash.

**Qué descarta.** Comparar con el hash del pipeline.

**Verificación.** Un cambio que solo afecta a variables declaradas nunca marca el eje instrucciones.

### DEC-06.10 — El eje variables compara solo las variables declaradas *(respuesta a `Q-06.5`)*

**Decisión.** El eje variables compara las variables declaradas, las que el DevOps escribe para cada
ambiente. Una variable producida no cuenta como causa: su cambio aparece en el sustento. Si lo único que
cambió es una variable producida, la respuesta es ES-5, *ningún eje cambió*, con ese cambio en el
sustento.

**Por qué.** Una variable producida cambia porque su paso se re-ejecutó, y eso es consecuencia de otro
eje. Si contara, el eje variables seguiría vivo en el caso normal.

**Consecuencias.** Lo que Diagnóstico le pide al Historial incluye el origen de cada variable. Precisa el
eje *variables* de `DEC-01.12` en `dominio.md` y `lenguaje.md`.

**Qué descarta.** Contar a la vez las declaradas y las producidas.

**Verificación.** Un cambio en una variable producida nunca aparece como candidato.

### DEC-06.11 — La respuesta combina las comparaciones *(respuesta a `Q-06.6`)*

**Decisión.** Un eje queda descartado si cualquiera de las comparaciones lo descarta, y los candidatos
son los que sobreviven a todas. Las comparaciones quedan en el sustento.

**Por qué.** La escena de la ventaja competitiva es una sola deducción. Con dos respuestas sueltas, el
programador tenía que juntarlas él.

**Qué descarta.** Entregar las comparaciones sin combinar.

**Verificación.** En el caso normal (ES-1 + ES-2) la respuesta tiene un solo candidato: las variables.

### DEC-06.12 — Cada paso decide si se re-ejecuta con su propia información *(explicada por el experto; la lectura por ámbito, por defecto)*

**Decisión.**

- La **regla** de un paso, escrita en su fichero de configuración, dice qué mira: el código del
  producto, sus instrucciones, las variables de su ámbito o el tiempo. Casi siempre mira las tres cosas,
  y como el código es lo que más cambia, casi siempre se re-ejecuta.
- Un paso se re-ejecuta si cambió, **desde su última vez**, algo de lo que mira. Cada paso compara con
  su propia información, no con la de otro paso.
- El **ámbito** de las variables de un paso es, por defecto, **su ambiente**; también puede ser
  **compartido**.
- **Lectura por defecto**: la última vez de un paso es su último registro **en su ámbito**. Así, un paso
  de ámbito compartido puede no re-ejecutarse en el siguiente ambiente si lo que mira no cambió, y un
  paso con el ámbito de su ambiente guarda su propia historia en cada ambiente.

**Por qué la lectura por ámbito.** Si la última vez fuera la de cualquier ambiente, un paso con
variables del ámbito de su ambiente se re-ejecutaría cada vez que se alterna entre staging y producción,
porque siempre encontraría las variables del otro ambiente.

**Consecuencias.** `lenguaje.md` precisa *regla* y *ámbito* y añade *última vez de un paso*, y el eje
variables de `dominio.md` se precisa por ámbito. El espacio de trabajo, que es de cada ambiente, es otra
cosa y no interviene en esta decisión.

### DEC-06.13 — Los tres ejes se comparan paso a paso, con los recursos con que se hizo de verdad cada paso *(por defecto)*

**Decisión.** El código, las instrucciones y las variables se comparan en cada paso con los recursos que
ese paso usó de verdad. Si no se re-ejecutó, son los del registro al que apunta su evidencia, que puede
venir de otro ambiente.

**Por qué.** Con reglas por paso, dos pasos de un mismo intento pueden haberse hecho con código
distinto: uno se re-ejecutó con el código nuevo y el otro no.

**Verificación.** Ningún eje se compara con un recurso que el paso no usó.

### DEC-06.14 — Diagnóstico no tiene entidades, agregados ni repositorios *(por defecto)*

**Decisión.** Su modelo son value objects (eje, ejes de un paso, referencia, comparación, atribución,
sustento y respuesta) y dos servicios de dominio (elegir las referencias y eliminación). Sus
invariantes se cumplen por construcción.

**Por qué.** No cambia ningún estado: compara lo que el Historial guardó. Un agregado protege
invariantes cuando algo cambia, y aquí nada cambia (`guia-ddd.md` §10).

**Consecuencias.** El cierre común del bloque B en `plan-ddd.md` se lee *«sus agregados, o por qué no
los tiene»*. **No es un core vacío**: el riesgo declarado del plan era un contexto sin lenguaje ni reglas
propias, y éste tiene las dos cosas.

**Verificación.** Las cinco invariantes de `contextos/diagnostico.md` se cumplen sin ninguna
comprobación en tiempo de ejecución.

### DEC-06.15 — La respuesta no se guarda *(por defecto)*

**Por qué.** La misma pregunta sobre el mismo historial da la misma respuesta, y el historial solo
crece. Guardarla sería una segunda copia de algo que se puede volver a calcular.

### DEC-06.16 — Por defecto se pregunta por el último intento; entre ambientes, el sustento da el último cambio *(por defecto; revisada por `DEC-06.20`)*

**Decisión.** Si no se indica ningún intento, *preguntar la causa* toma el último intento del ambiente.
Entre ambientes, donde una diferencia no es un cambio, el sustento da el último cambio de cada
declaración que difiere en el ambiente que falla.

### DEC-06.17 — El espacio de trabajo es de cada ambiente y se conserva entre sus intentos *(respuesta a `Q-06.7`, con la corrección del experto)*

**Decisión.**

- El espacio de trabajo es **de cada ambiente**: es el mismo en todos los intentos de ese ambiente, y
  no interfiere con el de otro.
- Dentro de un ambiente, un paso que no se re-ejecuta puede dejarle archivos a los siguientes en el
  espacio de trabajo, porque siguen ahí.
- **Entre ambientes**, lo único que un paso que no se re-ejecuta le deja a otro ambiente son variables
  producidas (su valor, ofuscado, en el Historial) y lo que esas variables nombran en el mundo. El
  espacio de trabajo de staging no existe para producción.

**Qué descarta.** Un espacio de trabajo que solo vive durante un intento.

**Consecuencias.** Responde la duda **#5** de IT-01. Y choca con *«nada vive entre invocaciones»*
(`arquitectura.md`, `DEC-05.10`) y con la restricción de `dominio.md` *«el motor no recuerda nada entre
intentos»* → `Q-06.8`.

### DEC-06.18 — El espacio de trabajo es memoria en un lugar fijo por ambiente; el historial son registros detrás de una interfaz *(respuesta a `Q-06.8`, con la corrección del experto)*

**Decisión.**

- El espacio de trabajo de cada ambiente **es memoria**: se conserva entre intentos y está disponible en
  cualquier máquina donde corra el motor, porque vive en **un lugar fijo por ambiente** que el motor
  conoce. **No viaja** con el historial.
- **Es de otra naturaleza que el historial**: son los archivos que genera la tecnología que usan los
  pasos, como los de terraform, y el motor no los interpreta.
- **El historial son registros**, detrás de una interfaz para escribirlos y leerlos. Si por debajo es un
  fichero o una base de datos queda escondido.
- **El motor sabe cuál es su espacio de trabajo y cuáles son sus registros.** Cómo lo sabe es
  infraestructura.
- **Por defecto**: si el motor no alcanza el espacio de trabajo de su ambiente, el intento no empieza.
  Re-ejecutar sin él podría repetir en el mundo lo que la tecnología ya hizo.

**Consecuencias.** La restricción de `dominio.md` pasa a decir que la continuidad está en dos sitios.
`DEC-05.10` sigue valiendo para copias e índices, y el espacio de trabajo no es una copia. `DEC-05.9` se
lee sin «llevar»: un registro no está escrito hasta que se puede leer desde cualquier máquina.

**Qué descarta.** Llevar el espacio de trabajo junto con el historial, y dejarlo en la máquina donde
corre el motor.

### DEC-06.19 — Cada intento empieza con el material declarado, sin tocar lo que dejó la tecnología *(respuesta a `Q-06.9`)*

**Decisión.** En el espacio de trabajo conviven dos cosas. **Lo que pone el motor**, el material del
pipeline copiado e interpolado, se vuelve a poner al empezar cada intento a partir del pipeline
declarado; no se restaura cuando algo falla. **Lo que genera la tecnología**, como el estado de
terraform, no se toca nunca.

**Por qué.** Restaurar lo que generó la tecnología lo desconectaría de lo que ya existe en el mundo, y
el siguiente intento volvería a crearlo. Y restaurar cuando algo falla no sirve si el proceso muere a
mitad.

**Consecuencias.** En `dominio.md`, *que un intento fallido no deje el material a medio tocar* pasa a
*que cada intento empiece con el material declarado, sin tocar lo que la tecnología dejó*.

**Qué descarta.** Restaurar cuando el intento falla.

**Verificación.** Un intento sin desenlace no deja al siguiente con plantillas interpoladas de antes, ni
con el estado de la tecnología modificado por el motor.

### DEC-06.20 — El sustento da los commits que hubo entre medias, no el autor de cada variable *(respuesta a `Q-06.10`)*

**Decisión.** Dentro de un ambiente, por cada eje que cambió, el sustento da los commits de su
repositorio entre la referencia y el intento, con autor y fecha: el repositorio del producto para el
código, y el del pipeline para las instrucciones y las variables. No precisa qué commit cambió cada
variable o cada paso. Entre ambientes, dice qué variables difieren, sin autor.

**Por qué.** Suministro sabe quién hizo cada commit, pero no interpreta el pipeline. Precisar el autor
por variable obligaría a leer el pipeline commit a commit.

**Consecuencias.** **Revisa `DEC-06.16`**: se retira *«el último cambio de cada declaración que
difiere»*. Lo que Diagnóstico le pide al Historial pasa a ser *los commits de cada repositorio entre dos
puntos, con autor y fecha*. La promesa de autoría de la cadena de garantías queda precisada.

**Qué descarta.** Precisar el autor por variable y por paso.

**Verificación.** Ningún sustento atribuye a una persona el cambio de una variable concreta.

### DEC-06.21 — Por defecto se compara solo con el último despliegue del mismo ambiente; sin historial previo se dice *(revoca `DEC-06.8`)*

**Decisión.** Sin referencia indicada, la única referencia es el último despliegue del mismo ambiente
anterior al intento (`DEC-06.6`). Se retira la referencia por defecto del ambiente anterior con el
mismo hash del código. Si no existe ningún despliegue anterior al intento, o la referencia (por defecto
o elegida) es un despliegue del propio intento que se diagnostica, la respuesta es **sin referencia** y
lleva el mensaje *«no hay historial previo al intento actual que se pretende diagnosticar»*.

**Por qué.** Comparar un intento consigo mismo no dice nada, y quien pregunta necesita saber que falta
historial, no recibir una respuesta vacía.

**Consecuencias.** **Revoca `DEC-06.8`**: `RazonDeReferencia` pierde `ambiente_anterior`, y el puerto de
Diagnóstico al Historial pierde la consulta por hash del código y el orden de ambientes del intento. La
comparación con staging sigue posible eligiendo a mano su despliegue. `Respuesta` gana `Mensaje`.

**Verificación.** `ElegirReferencias` no devuelve despliegues del propio intento; ES-6 lleva el mensaje.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `guia-ddd.md` | **Bloque táctico escrito**: §6–§13 |
| `modelo/contextos/diagnostico.md` | **Nace**: escenarios en borrador y lo que Diagnóstico le pide al Historial |
| `plan-ddd.md` | Tablero |
| `modelo/dominio.md` · `lenguaje.md` · `bounded-contexts.md` · `contextos/diagnostico.md` | Se compara dentro de un ambiente y entre ambientes (`DEC-06.6`) |
| `modelo/dominio.md` · `lenguaje.md` · `bounded-contexts.md` · `contextos/diagnostico.md` | Se compara un intento contra un despliegue de referencia (`DEC-06.7`) |
| `modelo/dominio.md` · `lenguaje.md` · `contextos/diagnostico.md` | En el ambiente anterior, la referencia tiene el mismo código (`DEC-06.8`) |
| `modelo/lenguaje.md` · `bounded-contexts.md` · `contextos/diagnostico.md` | El eje instrucciones se compara con el hash de las instrucciones de cada paso (`DEC-06.9`) |
| `modelo/dominio.md` · `lenguaje.md` · `contextos/diagnostico.md` | El eje variables compara solo las declaradas (`DEC-06.10`) |
| `modelo/contextos/diagnostico.md` | La respuesta combina las comparaciones (`DEC-06.11`); **modelo táctico escrito** (`DEC-06.13` a `DEC-06.16`) |
| `modelo/lenguaje.md` · `dominio.md` | Cómo decide un paso si se re-ejecuta: regla, ámbito y última vez de un paso (`DEC-06.12`) |
| `plan-ddd.md` | Cierre común del bloque B: *sus agregados, o por qué no los tiene* (`DEC-06.14`) |
| `modelo/lenguaje.md` · `dominio.md` · `arquitectura.md` | El espacio de trabajo es de cada ambiente y se conserva entre sus intentos (`DEC-06.17`) |
| `modelo/lenguaje.md` · `dominio.md` · `arquitectura.md` | El espacio de trabajo es memoria en un lugar fijo por ambiente; el historial, registros detrás de una interfaz (`DEC-06.18`) |
| `modelo/lenguaje.md` · `dominio.md` | Cada intento empieza con el material declarado (`DEC-06.19`) |
| `modelo/lenguaje.md` · `dominio.md` · `contextos/diagnostico.md` | El sustento da los commits entre medias (`DEC-06.20`) |
| `modelo/contextos/diagnostico.md` | Pasa a **vigente** |

## 6. Dudas diferidas

| # | Duda | A | Por qué se difiere |
|---|---|---|---|
| 1 | Cómo separa el motor, dentro del espacio de trabajo, lo que pone él de lo que genera la tecnología, y cómo conoce su espacio de trabajo y sus registros | **IT-09** | Es táctico de Ejecución (`DEC-06.18`, `DEC-06.19`) |
| 2 | Que el hash de variable sea comparable entre ambientes del mismo proyecto, como necesita el core para ES-2 | **IT-08** | El hash de variable nace en Resolución |
| 3 | Qué entra en el hash de las instrucciones de un paso: solo los comandos, o también los ficheros del directorio del paso | **IT-08** | Qué entra en un hash es técnica (`DEC-02.18`), pero decide el eje instrucciones del core |
