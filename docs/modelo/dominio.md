# Dominio y subdominios — Vex

> **Espacio del problema.** Aquí no se habla de código, paquetes ni ficheros: se habla de qué
> necesita el negocio, quién lo necesita y por qué. El impacto técnico vive en
> `../iteraciones/IT-01-destilacion-del-dominio.md` §7.
>
> **Vigente** desde el cierre de IT-03 (2026-09-13). Nació en IT-01 tras seis rondas de validación
> contra *Implementing DDD* cap. 2; IT-02 le aplicó tres enmiendas y dos renombrados, e IT-03 y las
> iteraciones siguientes cerraron lo que la solución necesitaba saber del problema. Si contradice a un
> documento de `iteraciones/`, manda éste. El lenguaje está en `lenguaje.md`; las fronteras de la
> solución, en `bounded-contexts.md`.

---

## El dominio

Desplegar software mediante pipelines declarativos, y poder responder después **qué pasó y por
qué**.

## Quién lo vive

| Actor | De qué es dueño | Qué pregunta |
|---|---|---|
| **Programador** | del código del producto | *«Falla. ¿Es mío?»* |
| **DevOps** | del pipeline y de los valores de cada ambiente | *«¿Está bien mi pipeline antes de publicarlo?»* · *«¿Lo tocó alguien?»* |
| **Dueño del negocio** | de qué llega al cliente final, y cuándo | *«¿Cuándo lo enviamos, y cómo se llama?»* |

**Desplegar y lanzar son dos actos, no uno.**

- **Desplegar** es colocar el código **sucesivamente** en dev → staging → producción. Son pasos
  de validación y posicionamiento: cada uno es una **preparación**. Ese orden lo declara
  **Definición de Pipeline**, junto con los ambientes.
- **Lanzar** es *«ahora sí: actívalo, anúncialo, hazlo visible»*. Es una decisión **separada y
  posterior**, y puede tomarse en el mismo minuto del despliegue a producción o semanas después.

**Llegar al último despliegue no dispara el lanzamiento.** Que hoy, sin nadie que decida, lo que
queda listo se lance solo **no es un efecto del despliegue**: es actuar en nombre del actor
ausente. La distinción parece sutil y no lo es — si el automatismo fuera parte del despliegue, la
separación no se podría introducir después sin romper el modelo.

## La ventaja competitiva

Cuando algo falla, saber con certeza —y en segundos— **si la causa está en tu código o en el
pipeline/ambiente**. La promesa tiene dos mitades, y conviene saber cuál se cumple siempre:

- *«¿tocó alguien el pipeline?»* — **siempre** tiene respuesta exacta. Las instrucciones son un
  solo eje, con una sola identidad, que no varía por ambiente: o cambiaron o no. Es la mitad que
  evita interrumpir al DevOps sin motivo, o buscar horas en el propio código algo que no estaba
  ahí.
- *«¿es mío o del ambiente?»* — se resuelve cuando la eliminación deja un candidato. Cuando deja
  dos, se muestran los dos.

Y solo es posible por una razón: **las instrucciones del pipeline no varían por ambiente** —las
variables sí— y un cambio en ellas no se puede introducir sin que se note. Ésa no es una condición
cualquiera: es la **premisa** que convierte la respuesta en una deducción forzosa, porque deja un
eje fijo que eliminar. Sin ella no hay respuesta peor — no hay respuesta.

## Cómo se clasifica aquí

Regla adoptada, para que clasificar sea una deducción y no un juicio caso por caso. **Las
preguntas están ordenadas y la primera que responde «sí» decide** — probado contra el caso de
Suministro de Fuentes, donde en paralelo daban dos respuestas a la vez:

1. **¿Es la respuesta por la que te eligen?** → **Core**.
2. **¿Se puede comprar?** → **Generic**, por mucho que el core se apoye en ello. Que algo sea
   imprescindible no lo hace diferenciador: un genérico puede ser crítico.
3. **¿El core depende de una garantía suya, o es capacidad propia que nadie vende?** →
   **Supporting**.

---

## El mapa

| Subdominio | Pregunta de negocio que responde | Quién pregunta | Tipo |
|---|---|---|---|
| **Diagnóstico** | ¿De quién es la causa de este fallo? | **programador**, y el dueño del negocio desde producción | **Core** |
| Definición de Pipeline | ¿Cómo se despliega este producto, y qué cambia en cada ambiente? | DevOps | Supporting |
| Historial | ¿Qué se ha desplegado, cuándo y con qué? | DevOps y programador | Supporting |
| Lanzamiento | ¿Cuándo llega al cliente, y con qué nombre? | **dueño del negocio** | Supporting |
| Ejecución de Pipeline | ¿Cómo llevo a cabo un intento haciendo solo el trabajo que hace falta? | programador | Supporting |
| Resolución de Variables | ¿Con qué valores concretos se despliega aquí, sin que nadie los vea? | DevOps | Supporting |
| Simulación de Pipeline | ¿Funcionará este pipeline antes de publicarlo? | DevOps | Supporting |
| Suministro de Fuentes | ¿Tengo delante el material, y sé si cambió? | ninguno propio: sirve a los demás | Generic |
| Sincronización del Historial | ¿Lo que dejó dicho el último despliegue está disponible aquí? | ninguno propio: sirve a los demás | Generic |

**Diagnóstico tiene dos clientes**, y es el único que los tiene: el programador pregunta desde el
despliegue, el dueño del negocio desde el cliente. Son **dos puertas de entrada a la misma
comparación** — ver abajo.

Espacio de Trabajo salió del mapa y «saber si algo cambió» nunca fue un subdominio —ver
*Lo que no es subdominio*, abajo—, y Lanzamiento entró al separarlo del Historial.

---

## Cómo se relacionan

La relación no es un flujo de datos: es una **cadena de garantías**. Cada subdominio Supporting
existe porque el core no puede responder sin algo que solo él puede prometer.

| Le promete al core | Subdominio | Si esa promesa cae |
|---|---|---|
| Las **instrucciones** del pipeline **no varían por ambiente** —las variables sí— | **Definición de Pipeline** | Sin un eje fijo que eliminar no hay deducción posible: no es que la respuesta empeore, **deja de haber respuesta**. No es una condición externa: es la **premisa** del razonamiento del core |
| Los ambientes tienen un **orden declarado**, y una cosa avanza por él | **Definición de Pipeline** | Comparar producción con staging deja de ser más significativo que compararla con cualquier otro ambiente |
| Existe un **último despliegue** en ese ambiente, y se sabe **con qué se hizo** — no con qué se haría hoy | **Historial** | No hay contra qué comparar. Y como el DevOps puede cambiar el pipeline mientras hay ejecuciones en curso, comparar contra «el pipeline de hoy» daría una respuesta exacta sobre otra cosa |
| Se sabe **qué está vivo ante el cliente**, y desde cuándo | **Lanzamiento** | Quien pregunta desde producción recibe una respuesta exacta sobre la cosa equivocada, que es peor que una inexacta |
| Se sabe **qué se hizo en este intento y qué se dio por bueno sin rehacer**, y por qué | **Ejecución de Pipeline** | Un paso que no se re-ejecutó es un agujero en la comparación: la causa podría estar en algo que hoy nadie tocó |
| El **valor efectivo** de cada variable está determinado, y se sabe si cambió — **sin que el valor circule** | **Resolución de Variables** | «Es del ambiente» deja de ser distinguible de «es del pipeline». El espacio de respuestas se queda en dos categorías |

Los dos Generic no prometen nada al core: **prestan un servicio a los demás**. Suministro pone el
material delante de Ejecución y calcula el hash de cada fuente; Sincronización lleva y trae
lo que el Historial necesita conservar. Por eso son Generic — nadie les pide una garantía, y lo que
hacen se puede comprar.

**El core nunca habla con la fuente**: lee el **registro de lo que se usó**. Suministro calcula el
hash al ejecutar, el Historial lo conserva, y el core compara registros. Es lo único compatible
con que el DevOps pueda cambiar el pipeline con ejecuciones en curso.

Y hay una prueba que lo confirma: **el proyecto y el pipeline son repositorios; para saber si
cambiaron basta con mirar sus diferencias**, sin nada propio. Las variables no: su valor efectivo
también nace en ejecución, y eso no lo resuelve ninguna herramienta de fuera. **Ésa es exactamente
la línea entre los dos genéricos y el supporting** — dos ejes los responde algo que se compra, el
tercero no.

**El core compara el intento que falla contra un despliegue del mismo ambiente y contra uno del ambiente anterior en el
orden**, y por defecto entrega las dos comparaciones: contra el despliegue anterior en ese ambiente y
contra el último del ambiente anterior **con el mismo código**. El usuario puede elegir otra referencia *(IT-06 `DEC-06.6`, `DEC-06.7`, `DEC-06.8`)*.
*(Enmienda de IT-02 `DEC-02.4`: IT-01 le había dado dos puntos de referencia, uno por cliente.)*

Preguntar desde el cliente **no cambia el mecanismo**: un lanzamiento sabe cuál es su despliegue,
así que resuelve a la misma operación. Hay **un punto de referencia y dos puertas de entrada**, no
dos mecanismos — y eso no debilita `DEC-01.13`: desplegar y lanzar siguen siendo dos actos con dos
dueños; lo que cae es la suposición de que dos actos obligaban a comparar de dos maneras.

*«El último resultado exitoso»* no es una expresión de este modelo: se dice **el último despliegue
en ese ambiente**.

**«El último despliegue» es único**, y no porque la historia sea lineal —un rollback la bifurca—
sino porque **el tiempo es un orden total**: en un ambiente no hay dos despliegues a la vez.

**Un solo Supporting no aparece en la tabla, y eso es información**, no un defecto que tapar.
**Simulación** entra por la segunda pata de la regla —capacidad propia que ningún proveedor
cubre—, que es la más débil: sirve al DevOps antes de publicar, no al programador cuando algo
falla.

### La historia, en una lectura

El **DevOps** declara cómo se despliega un producto —igual para todos los ambientes— y aparta lo
que cambia en cada uno. Antes de publicar esa declaración quiere probarla. El **programador**
pide un intento: se pone delante el material, se determinan los valores de este ambiente y se
hace **solo el trabajo que hace falta**, porque lo que no ha cambiado desde la última vez no se
rehace. Lo que el intento hizo y dejó dicho se conserva y viaja a donde haga falta. Y cuando
algo falla, alguien pregunta de quién es — y esa pregunta tiene respuesta **porque las
instrucciones eran las mismas, porque hay un despliegue exitoso con el que compararlo, y porque se
sabe qué se tocó y qué no**. Y cuando ya no falla, queda un último acto que no es técnico: el **dueño del
negocio** decide cuándo eso llega al cliente y con qué nombre.

---

## Los subdominios

### Diagnóstico · **Core**

**Responde**: ante un fallo o un resultado, ¿la causa está en el código del proyecto, en el
pipeline, o en las variables de este ambiente? Tres categorías, no una narración.

**Por qué es el core**: es la única respuesta del mapa que nadie más sabe dar, y es la razón por
la que alguien elige Vex. No presenta lo que otros saben: **decide una atribución** que ninguno
de sus proveedores puede emitir por su cuenta — el Historial sabe qué pasó, Definición sabe qué
se declaró, y ninguno de los dos sabe de quién es la culpa.

**Coste de clasificarlo mal**: es lo que ya pasó — todo el esfuerzo fue a capacidades que en
buena parte se pueden comprar, y el producto se quedó sin diferenciador.

**Lenguaje propio** — lo que confirma que es un ámbito y no una consulta sobre el de otro. El
inventario completo está en `lenguaje.md`; lo que lo hace un ámbito es que **cuatro de sus siete
términos nombran el método**, no el objeto:

| Término | Qué nombra |
|---|---|
| **causa** | dónde está el origen: en el código, en las instrucciones o en las variables |
| **sustento** | lo que sostiene la atribución: qué ejes cambiaron, en qué pasos y qué variables |
| **eje** · **eliminación** · **atribución** · **deducción**/**inferencia** | su procedimiento: qué compara, cómo descarta, qué emite y qué nunca hará |

*(Enmienda de IT-02 `DEC-02.6` y `DEC-02.16`: «evidencia» se cedió al Historial —donde ya nombraba
qué justificó no re-ejecutar un paso— y el core dice **sustento**; «reincidencia» se retiró en
favor de **cantidad de intentos**, que es un hecho del Historial. La lista de tres de `Q-01.14` era
una prueba, no un inventario.)*

**Su contrato: solo hechos exactos, nunca inferencia.** Y **nombrar la causa es un hecho**,
porque es una **deducción forzosa**, no una probabilidad:

> el pipeline es idéntico en los dos ambientes · funcionó allí y falla aquí · lo único distinto
> es esta variable · **luego** la causa está en las variables.

Lo que nunca hará es lo contrario: *«se ha repetido cuatro veces, probablemente sea del
ambiente»*. Eso es inferir. Y lo que atribuye es el **origen**, nunca a una persona: quién hizo un cambio no
forma parte de la respuesta *(IT-07 `DEC-07.7`)*.

**Por qué la exactitud es lo que se vende, y no un lujo.** La respuesta **encamina una acción**:

| Si la causa es… | Quién lo arregla |
|---|---|
| el código del proyecto | el **programador**, él mismo |
| el pipeline · las variables del ambiente | el **DevOps** — hay que pedir ayuda |

Una atribución adivinada no queda fea: hace que el programador pierda el día buscando en su
código, o que interrumpa a otro equipo sin motivo. Por eso las tres categorías no son una
taxonomía elegida por gusto — **son un encaminamiento**.

**Qué compara: tres ejes.** No compara «ejecuciones»: compara tres cosas que cambian con reglas
distintas, y **atribuye por eliminación**.

| Eje | Cuántos hay | ¿Varía por ambiente? | Si cambia, ¿a qué afecta? |
|---|---|---|---|
| **Código del producto** | uno — el proyecto tiene identidad propia | no | a lo que se despliegue desde ahí |
| **Instrucciones del pipeline** | uno, agrupadas por paso | **no** | a **todos** los despliegues de **todos** los ambientes |
| **Variables** | muchas, las **declaradas**: por paso **y** por ámbito, que por defecto es el ambiente. Las que produce un paso son consecuencia y no cuentan *(IT-06 `DEC-06.10`, `DEC-06.12`)* | **sí**, salvo las de ámbito compartido | a **un** ambiente, y a un paso |

Esa asimetría —un eje que no varía por ambiente y otro que sí— **es el mecanismo de la
deducción**. No es una curiosidad del formato: es lo que permite eliminar.

| La pregunta | Qué fija la situación por sí sola | Qué queda |
|---|---|---|
| *«ayer funcionó, hoy falla»* — mismo ambiente | nada: en ese tiempo otro programador pudo tocar el código **y** el DevOps el pipeline | si las instrucciones no cambiaron, **dos** candidatos: código o variables |
| *«funcionó en staging y falla en producción»* | el código es el mismo por identidad; las instrucciones, porque no varían por ambiente | **uno**: las variables |

No son dos deducciones distintas: es **el mismo procedimiento con distinto punto de partida**.
Comparar entre ambientes es lo potente porque **elimina dos ejes de golpe** — de ahí el «en
segundos».

**Nunca le falta sustento**: saber qué cambió es siempre un hecho disponible. Y cuando la
eliminación deja **dos** candidatos, **muestra los dos**. *«Las instrucciones no cambiaron; el
código y las variables sí»* sigue siendo un hecho exacto, y **sigue encaminando**: descarta al
DevOps del pipeline. Cuál de los dos es más probable lo decide el programador — la herramienta
muestra todo.

**La regla, dicha una sola vez:**

> **La herramienta muestra hechos. La persona juzga probabilidades.**

Es lo mismo en los tres sitios donde aparece: la cantidad de intentos se entrega como número y no
como lectura; la causa se **deduce** y nunca se infiere; y cuando quedan dos candidatos se muestran los
dos. «Exacto, nunca inferencial» no es una aspiración: es una **división del trabajo**.

**Qué significa «todo»**, para que no choque con la promesa de no exponer valores: todo lo que se
sabe —qué ejes cambiaron, en qué pasos y qué variables—, **no** el valor. Se muestra que
`DB_POOL_SIZE` cambió, no de cuánto a cuánto.

---

### Definición de Pipeline · **Supporting**

**Responde**: cómo se despliega este producto, y qué cambia en cada ambiente. Incluye
**validar** que una definición es correcta antes de intentar nada con ella.

**Por qué es negocio**: la invariante —**las instrucciones no varían por ambiente; las variables
sí**— no es una característica del formato, es la **premisa del razonamiento del core**. Y tiene
una consecuencia que el core explota directamente: un cambio en las instrucciones afecta a
**todos** los despliegues de **todos** los ambientes, mientras que uno en las variables afecta a
**un** ambiente y a un paso. Debilitarla no haría la atribución menos precisa: la dejaría **sin
fundamento**. Es la relación más fuerte del mapa.

**También declara el orden de los ambientes** —dev → staging → producción—, que es lo que hace
que comparar producción con staging signifique algo: lo que hay en producción vino de ahí.

**Coste de clasificarlo mal**: como genérico se erosiona la invariante sin que nadie lo note, y
el core se queda sin premisa sin que nadie se entere. Como core, se confunde la premisa con la
ventaja.

---

### Historial · **Supporting**

**Responde**: qué se ha desplegado, cuándo, con qué y con qué resultado. Es la **memoria** del
negocio.

**Guarda tres eslabones**, cada uno con identificador propio y con memoria de dónde viene:
**intento** —una ejecución de uno o varios pasos en un ambiente, con estado exitoso, fallido o
cancelado—; **despliegue** —un intento exitoso de **todos** los pasos, que es un **punto de
retorno** y que sabe cuál es su **padre**—; y **lanzamiento**, que es de abajo. Cada despliegue
tiene un intento, pero **no todo intento tiene despliegue**, y un intento exitoso que ejecutó tres
pasos de cinco no tiene nombre propio: es un intento. Tampoco llega a despliegue un intento hecho con una
**copia de trabajo**, sin commit, porque no se podría volver a él *(IT-10 `DEC-10.7`)*. Si un intento tiene registros y ninguno dice
cómo terminó (porque la máquina murió, o porque sigue corriendo en otra), está **sin desenlace**: es
un hecho, no un fallo, y no tiene causa *(IT-05 `DEC-05.8`)*.

**Volver atrás —*rollback*— crea un despliegue nuevo cuyo padre no es el último**, así que dos
despliegues pueden compartir padre. Eso **no es comparar**: es elegir hacia dónde regresar. Y se
vuelve **con los recursos con que se hizo** aquel despliegue: por eso el historial guarda, además del
hash, el commit de cada fuente *(IT-03 `DEC-03.9`)*.

**Se llamaba Registro de Despliegue.** Un *registro* es **un hecho**; el **historial** es el
conjunto, y el subdominio es el conjunto *(IT-02 `DEC-02.11`)*.

**Por qué es negocio**: sin un punto de comparación fiable no hay atribución posible. Pero la
trazabilidad la ofrece mucha gente: ser imprescindible no es ser diferenciador.

**Coste de clasificarlo mal**: como core absorbe la inversión y el core real se queda en una
vista. Como genérico se pierden reglas que nadie más tiene — el linaje de despliegues y el volver
a uno anterior.

**Ya no lleva dentro el lanzamiento** como capacidad: entregar al cliente tiene otro dueño y otro
lenguaje, y está abajo. El historial sí guarda los lanzamientos, como guarda todo lo que pasó.

---

### Lanzamiento · **Supporting**

**Responde**: cuándo lo que está listo llega al cliente final, y con qué nombre sale.

**Por qué es negocio y no una anotación sobre el despliegue**: tiene **otro experto** —el dueño
del negocio, no el DevOps—, **otro lenguaje** —lanzamiento, *listo para el cliente*, etiqueta de
negocio— y existe fuera del software como oficio entero: decidir cuándo un producto sale al
mercado. Que en la práctica hoy vaya pegado al despliegue no lo convierte en lo mismo.

**Su regla propia**: cuando el dueño del negocio no está, se asume que acepta enviar en cuanto
esté listo, y lo que queda listo se lanza solo. Eso es actuar **en su nombre**, nunca un efecto
del despliegue — un valor por defecto sobre **quién decide** es una decisión de negocio. Por eso
Lanzamiento se entera escuchando el anuncio de cada despliegue registrado, y no porque quien despliega
lo llame *(IT-04 `DEC-04.8`)*. Qué ambientes se reserva el dueño del negocio no está en el pipeline,
que es del DevOps: es una decisión suya que queda como registro en el historial *(IT-04 `DEC-04.9`)*.

**Lleva dos etiquetas y no son la misma cosa**: la **versión** es la etiqueta técnica, clave-valor,
que pone la herramienta; el **nombre del lanzamiento** es el que le pone el dueño del negocio, y si
está ausente toma el valor de la versión. Que coincidan por defecto no las funde — es otra vez
actuar en nombre del actor ausente.

**Ocurre en cualquier ambiente** *(IT-03 `DEC-03.17`)*, y hace visible el despliegue **a quien usa
ese ambiente**: el equipo en dev, quien valida en staging, el cliente final en producción. El dueño del
negocio decide en los ambientes que él elija, y en el resto se actúa en su nombre *(IT-04
`DEC-04.1`)*.

**Le promete al core** saber qué está vivo ante el cliente y desde cuándo. La pregunta que viene de
producción entra por ahí, y **resuelve al despliegue de ese lanzamiento**: el mecanismo de
comparación es siempre el mismo.

**Coste de clasificarlo mal**: dentro del Historial, la capacidad del actor que no está en la sala
pierde siempre contra la del que sí — y no se modela nunca bien.

---

### Ejecución de Pipeline · **Supporting**

**Responde**: cómo llevar a cabo un **intento** haciendo solo el trabajo que hace falta, y
dejando dicho qué se hizo, qué se dio por bueno sin rehacer y por qué.

**Por qué es negocio, y un solo subdominio**: no hay un experto para «decidir» y otro para
«ejecutar». Un DevOps haciéndolo a mano no lo vive como dos actividades: es *ejecutar el
pipeline de forma inteligente*. Decidir con exactitud qué rehacer ahorra dinero, pero es el
mecanismo que hace posible la atribución, no la atribución.

**Coste de clasificarlo mal**: partirlo en dos multiplica fronteras donde el negocio ve una sola
cosa. Tratarlo como core repite el error de invertir en la parte difícil en vez de en la que
diferencia.

**Absorbe**: que cada ambiente trabaje aislado de los demás, y que cada intento empiece con el
material declarado, sin tocar lo que la tecnología dejó *(IT-06 `DEC-06.19`)*. No son capacidades aparte: son **cómo** Ejecución cumple su promesa. Cada ambiente tiene su espacio de trabajo, que es el mismo
en todos sus intentos *(IT-06 `DEC-06.17`)*.

---

### Resolución de Variables · **Supporting**

**Responde**: con qué valores concretos se despliega en este ambiente — vengan declarados o
salgan de lo que un paso produjo — y si cambiaron desde la última vez.

**Por qué es negocio**: hay experto, existe como práctica sin software y se entiende fuera de lo
técnico. Y tiene una promesa propia de la que **depende el core**: se sabe si una variable cambió
**sin que su valor circule**.

**La tensión, y cómo se resuelve**: proteger no puede impedir que el core haga su trabajo. Se
compara **sin ver** — lo que se conserva y se compara es un hash del valor, no el valor. El
core podrá decir *«esta variable cambió desde el último despliegue exitoso»* y **no** *«pasó de
10 a 50»*. Pérdida de detalle, no de función: las categorías de respuesta se resuelven sin el valor.
Ese hash es **simple, sin clave**: un valor corto o previsible se puede adivinar, y se acepta, porque la
seguridad no es el objetivo de este producto y gestionar secretos es otro *(IT-08 `DEC-08.8`)*.

**Lo que no es suyo**: ofuscar lo que se guarda **no es cifrar** y no es dominio — es de quien
persiste, y está abajo como restricción declarada. Aquí vive la promesa de comparar sin exponer;
no la forma de dejarlo en reposo.

**Coste de clasificarlo mal**: la protección vuelve a ser un asunto de todos —que es de donde
viene— y lo que es de todos no es de nadie.

---

### Simulación de Pipeline · **Supporting**

**Responde**: ¿va a funcionar este pipeline, antes de publicarlo? Recorre el flujo entero **sin
efectos y sin guardar nada**. Y lo único que finge es la **ejecución de los comandos**: la
interpolación de variables y las validaciones se hacen de verdad.

**Por qué es negocio y no la validación de Definición**: distinto **actor** (el DevOps que
diseña, no el programador que despliega), distinto **momento** (antes de publicar el pipeline,
no antes de cada intento) y distinto **propósito** (validar el diseño completo, no evitar fallos
evidentes). La relación con la validación es de contención: es su primer paso, no su gemela.

**Contexto propio, no un modo de Ejecución** *(IT-03 `DEC-03.10`)*: comparte casi todo el mecanismo,
pero dentro de Ejecución *intento* y *variable producida* significarían dos cosas. No lee el
historial: recorre siempre todos los pasos, porque de un pipeline sin publicar no hay *última vez*.

Sigue siendo el Supporting del que el core no depende, y no tiene usuario real todavía.

---

### Suministro de Fuentes · **Generic**

**Responde**: ¿tengo delante el material a desplegar y la definición con la que desplegarlo, y
sé si han cambiado desde la última vez?

**Por qué es Generic**: *«tener mi repo delante y saber si cambió»* es higiene básica de control
de versiones. Existe desde antes que Vex, lo hace todo el mundo igual y se puede comprar. Que
sus dos fuentes tengan dueño distinto —el programador es dueño del producto, el DevOps del
pipeline— explica que sean **dos fuentes**, no que sean dos capacidades.

**También trae el material tal como era**, a partir de un commit, para poder volver atrás *(IT-03
`DEC-03.9`)*. Quién hizo cada cambio es asunto de la herramienta de repositorios, no de Vex *(IT-07
`DEC-07.7`)*. Sigue siendo algo que se compra.

**Coste de clasificarlo mal**: como supporting consume presupuesto de diseño en algo que no
diferencia.

---

### Sincronización del Historial · **Generic**

**Responde**: lo que el último despliegue dejó dicho, ¿está disponible aquí y ahora, aunque
«aquí» sea otra máquina y otra persona?

**Se llamaba Sincronización de Estado**, por la carpeta que copiaba. No mueve un hecho suelto:
mueve lo que el historial necesita conservar *(IT-02 `DEC-02.12`)*. En la solución **no es un
contexto propio**: se realiza dentro del contexto Historial, porque lleva registros sin
interpretarlos *(IT-03 `DEC-03.11`)*. Cada registro se lleva en cuanto se escribe, y si no se puede
llevar, el intento se detiene antes del siguiente paso: no se hace en el mundo nada que no pueda
quedar escrito *(IT-05 `DEC-05.9`)*.

**Por qué es Generic**: es transporte y custodia. **No** sabe qué hay en producción —eso es el
Historial— ni decide ni efectúa nada —eso es Ejecución—. Que en la práctica manual el mismo
DevOps haga las tres cosas no las funde: son papeles distintos con dueño distinto.

**Coste de clasificarlo mal**: como supporting se le inventa un vocabulario que no tiene.

---

## Lo que **no** es subdominio

**Saber si algo cambió.** El negocio sí lo necesita —el programador quiere saber si su código
cambió, el DevOps si el pipeline cambió, Ejecución si un paso hay que rehacerlo—, pero es una
**pregunta que se hacen tres subdominios distintos**, no una capacidad con dueño. Lo que
comparten es la técnica de responderla, y **la técnica no es dominio**.

**Y la regla de qué cuenta como cambio tampoco lo es** *(enmienda de IT-02 `DEC-02.18` a
`DEC-01.4`)*. IT-01 la había separado del algoritmo como *«lo único con carga de negocio»*, pero su
contenido —nombres de fichero, permisos, fines de línea, comentarios— **no es enunciable en
lenguaje de negocio**, y no existe la persona que sepa de despliegues y opine sobre CRLF. Se había
separado por su **consecuencia**, no por su contenido, y una consecuencia grave no convierte en
dominio a lo que no lo es.

**Pérdida aceptada, escrita para que sea elección y no olvido**: si esa regla cambia en silencio,
el mismo repositorio sin tocar da otro hash, el core dice *«cambió»* sobre algo que nadie tocó, y
todas las comparaciones anteriores dejan de significar lo mismo **sin que nada avise**. Eso pasa a
ser responsabilidad de quien implementa; el modelo no tiene nada que decir al respecto.

Lo que queda de `DEC-01.4` son dos piezas: la **técnica** —que no entra en el mapa— y los **hashes**
que produce, que son conceptos de quien los usa.

**El aislamiento entre ambientes.** Que cada ambiente trabaje sin pisar a otro es una
expectativa del negocio, pero no una capacidad que nadie venda ni que nadie reconozca como
oficio. Es **cómo** Ejecución cumple lo suyo.

**Ofuscar lo que se guarda.** No es cifrar y no es una promesa de negocio: es responsabilidad de
quien persiste. Lo que sí es de negocio —comparar sin exponer— vive en Resolución de Variables.

---

## Restricciones declaradas

No son subdominios y no se pueden perder. Se escriben aquí por eso.

**El motor no recuerda nada entre intentos.** Cada intento empieza de cero y muere al terminar.
Toda continuidad tiene que estar escrita en algún sitio, y por eso el Historial y Sincronización
existen como capacidades separadas en vez de ser memoria del proceso. La continuidad está en **dos
sitios de naturaleza distinta**: el historial, que son registros, y el espacio de trabajo de cada
ambiente, que son los archivos que genera la tecnología de los pasos *(IT-06 `DEC-06.18`)*. Consecuencia sobre el
modelo: **la concurrencia no es un concepto de este dominio**, y por eso un ambiente no admite dos intentos
a la vez *(IT-07 `DEC-07.8`)*.

**Lo que se guarda no queda legible de un vistazo.** Ofuscar **no es cifrar**, y no es una
promesa de negocio: es responsabilidad de quien persiste, y **quien persiste es el Historial**. El
valor de una variable producida queda ahí ofuscado, y solo Resolución de Variables lo pide de vuelta,
por una relación que nadie más tiene: el valor no forma parte de lo que el Historial publica *(IT-03
`DEC-03.16`, IT-04 `DEC-04.7`)*. Lo que sí es de negocio —comparar sin exponer— vive en Resolución de
Variables y está en la cadena de garantías.

**Fuera de alcance por ahora, por decisión explícita**: las estrategias de lanzamiento. Que existan dos
etiquetas —la *versión* y el *nombre del lanzamiento*— está decidido (IT-02 `DEC-02.17`), y también cómo se
obtiene la versión: un número por proyecto que crece con cada código que se lanza por primera vez *(IT-10
`DEC-10.8`)*.
