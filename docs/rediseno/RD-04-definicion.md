# RD-04 — Definición de Pipeline

> Contexto: **Definición de Pipeline** (`contextos/definicion.md`) · Depende de: RD-03 · Frente 2: **B**

## §1 Problema

No hay ningún pipeline comprobado que usar.

## §2 Por qué importa

El pipeline lleva la premisa del core: las instrucciones no varían por ambiente. Ejecución, Resolución y
Simulación lo necesitan, y ninguno debería poder recibir uno mal formado.

## §3 Objetivo

- Un agregado **Pipeline** inmutable, que solo existe si pasó la comprobación.
- Un repositorio de pipelines apoyado en Suministro.

## §4 Alternativas

- **Validar cuando se usa.** Descartada: dejaría circular pipelines mal formados (`DEC-08.2`).

## §5 Solución

- **Dominio**:
  - **Pipeline**: pasos identificados por su nombre, con sus comandos, su material y su configuración;
    variables declaradas, **cada una de un ámbito**, el de un ambiente o el compartido; ambientes en orden;
    formas declaradas.
  - **Comprobación**, como factoría que devuelve un Pipeline o la lista de fallos.
- **Publicado**: el pipeline comprobado, con su versión.
- **Infraestructura**: el ACL hacia Suministro, que lee los ficheros del pipeline (`definicion.md`, «Los
  ficheros del pipeline») y los convierte en una declaración.
- **Plantillas**: los 15 `config.yaml` y los `vexpipeline.yaml` de `pipelines/vex-tpl-*` pasan a
  `schema_version: 3` (`DEC-12.6`).

## §6 Alcance

**Dentro**: lo anterior, incluidas las tres plantillas. **Fuera**: leer una `schema_version` anterior a 3.

## §7 Verificación

- No hay forma de obtener un Pipeline que no haya pasado la comprobación.
- Cada fila de la comprobación produce su fallo: formato, nombres y orden de pasos, paso sin comandos,
  variable usada no declarada o no visible desde su ámbito en algún ambiente, variable usada antes de que se
  produzca lo que hace falta para resolverla, expresión regular mal formada.
- También los fallos del formato: `schema_version` distinta de 3, regla desconocida, `scope` desconocido,
  `outputs` sin `name` ni `probe`, aserción con `scope`, y un mismo nombre declarado o producido en dos
  sitios que se ven a la vez.
- Las tres plantillas actualizadas pasan la comprobación.
- Renumerar un paso no cambia su identidad.
- Un fichero del material que no está en `templates` se copia sin interpolar.

## §8 Decisiones

`DEC-02.7` · `DEC-03.6` · `DEC-03.7` · `DEC-06.12` · `DEC-08.2` · `DEC-08.5` · `DEC-08.7` · `DEC-12.6` · `DEC-12.7`

## §9 Hallazgos al implementar

*Implementada el 2026-09-14; corregida el 2026-09-15 con la respuesta del usuario sobre el ámbito y las
aserciones (hallazgos 3, 5 y 16); ampliada el 2026-09-16 con el ámbito propio de un paso y con retirar
`rules.yaml` hacia `config.yaml` (hallazgos 19 y 20).* Compila, pasan sus pruebas (también con `-race`) y
pasa la regla de dependencias.

1. **Variables estándar** (incongruencia traída al usuario al empezar). La comprobación exige que toda variable
   usada esté declarada, pero los pipelines usan variables que nadie declara (`project_name`, `environment`,
   `step_workdir`…), y el modelo no las nombraba. Respuesta del usuario: son variables que el motor garantiza
   siempre, se usen o no, y la herramienta las documenta. Hay dos clases:
   - **Metadatos**: los da quien invoca (el CLI o el portal) y se comportan como compartidos: `project_id`,
     `project_name`, `project_organization`, `project_team` y `environment`. `environment` es un dato de la
     solicitud, como el paso hasta el que se ejecuta, y el motor lo ofrece como un metadato más.
   - **Generadas** por el motor, que sigue un estándar propio de la herramienta:
     - compartidas: `project_hash` (sustituye a `project_revision`: es el hash del código), `project_version`,
       `project_workdir` y `tool_name`;
     - **del paso**: `step_name` y `step_workdir`. Cada paso tiene la suya y solo vale mientras se ejecuta:
       el ámbito es el paso, y el mismo nombre existe en pasos distintos con valores distintos.
   - **Definición es dueña de sus nombres, no de sus valores**, porque forman parte del formato que escribe el
     DevOps. Las publica para que Resolución y el borde usen la misma lista.
   - Declarar o producir una variable con uno de esos nombres es un fallo: chocaría dentro de un mismo ámbito.
   - **Para RD-05 y RD-06**: dar los valores. El de `project_hash` (`contenido-v1:…`) lleva `:`, así que no sirve
     tal cual como etiqueta de una imagen ni de Kubernetes, y las plantillas lo usan así.
2. **Las plantillas quedan fuera** (a pedido del usuario: `pipelines/` es solo referencia, y la unidad se ocupa de
   `vex-engine`). §3, §6 y §7 lo pedían. En su lugar hay un **pipeline de ejemplo** propio, en
   `internal/definicion/testdata/ejemplo/`, con la forma de las plantillas y todo lo que el formato 3 permite
   (su `README.md` dice qué usa cada parte), y no se depende de ningún otro proyecto. Para pasar la
   comprobación, las plantillas tendrían que cambiar:
   - `config.yaml`: `scope` desaparece (hallazgo 16), y `rules` y `max_age` pasan al lenguaje del modelo;
   - `schema_version: 3`;
   - los `resolve: step-output` pasan a `value: ${var.<salida>}` (hallazgo 17);
   - `project_revision` pasa a `project_hash`.
   El `probe` sin `name` de `01-test` ya no hay que cambiarlo: es una aserción (hallazgo 3).
3. **Un `outputs` sin `name` y con `probe` es una aserción** *(respuesta del usuario, 2026-09-15; revierte lo
   que esta ficha decidió por defecto)*. No produce ninguna variable: es lo que la salida del comando tiene que
   cumplir **para que el comando se dé por bueno**, y su significado es de Ejecución, como el de una regla. Por
   eso es un **término nuevo** del lenguaje, y no se llama *comprobación*: dentro de Definición esa palabra ya
   nombra lo que se verifica del pipeline entero antes del intento, y dos sentidos dentro de un contexto sí son
   un error (`lenguaje.md`, «Cómo se lee este documento»). Lo que sigue siendo un fallo es un `outputs` **sin
   `name` ni `probe`**, que no declara nada, y una aserción con `scope`: el ámbito es de lo que se produce.
4. **`from` nombraba el paso sin su `NN-`**, para que renumerar un paso no rompiera las referencias a él
   (`DEC-12.7`). **Retirado por el hallazgo 17**: ya no hay `from`.
5. **La visibilidad, que el modelo no precisaba** *(reescrita el 2026-09-15 con el hallazgo 16)*:
   - una variable declarada **es de su ámbito**, y la ve todo paso que se ejecuta en él, se declare en el
     fichero que se declare;
   - lo que produce un comando lo ven **los comandos posteriores del mismo paso** y los pasos posteriores del
     mismo ámbito. Ya no es cierto que un paso no vea lo suyo, y el motor antiguo tampoco lo era;
   - lo declarado en el **ámbito compartido** solo ve lo compartido: un valor compartido que dependiera de un
     ambiente dejaría de ser el mismo en todos. Al revés sí, desde un ambiente se ve lo compartido;
   - un literal puede usar otras variables visibles, también las de su mismo fichero. Si se usan en círculo, es
     un fallo, y un círculo entre compartidas se dice una sola vez;
   - un fallo que ocurre en todos los ambientes es uno solo, sin ambiente.
6. **El ACL no decide nada.** Convierte un directorio en una **declaración** tal como está escrita. Lo que no
   puede leer (YAML roto, claves que el formato no tiene) y lo que no reconoce bajo `steps/` o `variables/` van
   dentro de la declaración, y el vocabulario del formato (`scope`, `rules`, `schema_version`) lo
   comprueba el dominio. Así la comprobación es la única que produce fallos y la única puerta. Un
   `vexpipeline.yaml` de otra versión se lee sin exigir sus claves, para poder decir qué cambió.
7. **Formas que el modelo no listaba**, cada una en su fila:
   - el nombre de un paso y el `value` de un ambiente usan letras, dígitos, `-` y `_`, y no se repiten ni
     cambiando mayúsculas, porque son nombres de ficheros de `variables/`;
   - `workdir` y `templates` no salen del directorio del paso, y una plantilla está en el material y no es un
     enlace. Un enlace del material tampoco sale del paso, o su hash no vería lo que usa;
   - en un ámbito, una variable de salida se produce en **un solo sitio**: dos comandos que produjeran el mismo
     nombre dejarían a quien lo usa sin saber cuál ve;
   - `rules: []` es no mirar nada, distinto de no escribir `rules`;
   - `max_age` es una duración de Go: `720h`, no `30d`.
8. **El pipeline sale entero en memoria**, material incluido (contenido, bit de ejecución y enlaces), y el ACL
   retira siempre el material de Suministro. Nadie depende de que la copia siga ahí, y el contenido es un
   `string`, así que no se puede cambiar. Si el material no se puede retirar, no se entrega el pipeline.
   **Pérdida aceptada**: el material de un pipeline tiene que caber en memoria.
9. **`Comando.Plantillas` son rutas relativas al directorio del paso** (`templates` resueltas contra `workdir`), y
   cada fichero del material dice si es plantilla. `workdir` se publica limpio; qué significa vacío o `.` lo
   decide Ejecución.
10. **Lo publicado va por cliente**: `ParaEjecucion` (hoy y commit), `ParaResolucion` y `ParaSimulacion`
    (commit), las tres con `VariablesEstandar`. Resolución no puede recibir el pipeline a través de lo que
    publica, porque lo publicado solo usa la biblioteca estándar, así que lo pide por su commit y lee el mismo
    que el intento. Si RD-05 o RD-09 necesitan otra cosa, como una copia de trabajo, se corrige allí. *Con su
    versión* es la del formato, en `Pipeline.Version`.
11. **Qué garantiza «no hay forma»**: el agregado no tiene campos exportados, sus consultas devuelven copias, y
    `Comprobar` es la única función que da uno (una prueba lo recorre con `go/parser`). Los tipos publicados son
    datos, como en los demás contextos: se pueden construir a mano, pero ninguna operación publicada da uno sin
    comprobar.
12. **Las pruebas**, en dos niveles, con cada caso de la comprobación en los dos:
    - el dominio, sobre una declaración en memoria;
    - el pipeline de ejemplo, que se lee del disco y pasa la comprobación. Después se rompe en una copia,
      fichero a fichero, y cada caso ve su fallo pasando por la lectura y por la comprobación. Un cambio que
      reemplaza texto exige que el texto esté: si el ejemplo cambia, la prueba falla en vez de pasar sin
      romper nada;
    - el ACL, sobre el Suministro real, con el ejemplo en un repositorio go-git temporal.

    Cada caso exige además que su fallo no arrastre otros. Las pruebas se **validaron con mutaciones**: se rompe
    cada regla con `go test -overlay`, sin tocar el código, y alguna prueba tiene que caer. Las primeras dejaron
    vivas siete reglas sin vigilar, y cada una ganó su caso: `from` del mismo paso, nombre de paso inválido,
    ambiente sin `name`, `step-output` sin `key`, `max_age` no positivo, y plantilla que sale del paso o que es
    un enlace. Los dos primeros casos de `from` se fueron con el hallazgo 17, y las reglas nuevas del 16 y del
    17 se validaron con las mismas mutaciones.
13. **Una variable con el valor mal escrito sigue declarada.** Sin `value`, su nombre cuenta como declarado:
    lo que la usa no suma un «no declarada» que no es verdad, y con el fallo no hay pipeline. Lo encontró el
    ejemplo roto.
14. **`steps` o `variables` que no son un directorio** son un fallo de formato. Antes uno se ignoraba y el otro
    era un error de disco.
15. **Fuera, como decía la ficha**: leer una `schema_version` anterior a 3. La raíz de composición no conecta
    Definición hasta RD-06.
16. **El ámbito es de la variable, no del paso** *(respuesta del usuario, 2026-09-15; reescribe el modelo)*. Un
    **ámbito** es un espacio de variables: el de un ambiente, nombrado por su `value`, o el **compartido**, que
    no representa a ningún ambiente. Lo que cambia:
    - **Dónde se escribe.** El directorio dice el ámbito: `variables/<ambiente>/*.yaml` son de ese ambiente y
      `variables/*.yaml`, del compartido. **El nombre del fichero solo organiza**: ya no tiene que ser el de un
      paso, y un ámbito puede repartirse en los ficheros que se quieran. Deja de existir el fallo «variables de
      un paso que no existe».
    - **Quién las ve.** Todas las de su ámbito, cualquier paso que se ejecute en él. Una variable declarada en
      el fichero de otro paso se ve igual, y por eso **un nombre pertenece a un solo ámbito**: declararlo en el
      compartido y en el de un ambiente, o dos veces en el mismo ámbito, es un fallo, porque un paso vería dos.
      Dos ambientes distintos sí pueden declarar el mismo nombre: nunca se ven a la vez.
    - **Las variables de salida también tienen ámbito**: `scope: shared` en el `outputs`, o el del ambiente en
      ejecución si no se declara. Es lo que permite que una variable compartida use lo que produjo un paso sin
      que su valor dependa del ambiente.
    - **`config.yaml` ya no declara `scope`**, y un paso ya no tiene ámbito: se ejecuta en el ambiente pedido y
      ve ese ámbito más el compartido. Un `scope` en `config.yaml` falla como clave que el formato no tiene.
    - **El orden pasa a comprobarse donde se usa, no donde se declara.** Como una variable declarada no es de
      ningún paso, «nombra un paso anterior» ya no se puede preguntar al declararla: lo que se comprueba es
      que, en el punto donde se **usa**, ya esté producido todo lo que hace falta para resolverla, siguiendo
      los valores declarados uno tras otro. El mensaje lo dice entero
      (*«usa `${var.destino}`, que necesita la salida "ip", y la produce el comando 1 de "despliegue"»*).
      **Pérdida aceptada**: una variable declarada que nadie usa puede nombrar la salida de un paso posterior
      sin que nadie lo diga, porque nunca se resuelve.
    - **Pérdida aceptada, y es la de siempre**: un paso que usa una salida compartida producida por un paso
      posterior falla, aunque en el ambiente de al lado ya existiera de un intento anterior. La comprobación no
      lee el historial, y el primer intento del primer ambiente tiene que poder pasar.
    - **Lo que queda abierto, y no es de esta unidad**: si un paso no declara ámbito, **bajo qué ámbito se
      busca su última vez** (`DEC-06.12`). Tal como queda, todo paso guardaría su historia en cada ambiente y
      se re-ejecutaría en todos, que es justo lo que `scope: shared` evitaba. El Historial ya tiene la forma
      (`Ambito{Ambiente, Compartido}`, RD-02) y nadie se la da todavía. La salida natural es **derivarlo**: un
      paso es compartido si todo lo que ve y usa es compartido. Lo deciden RD-05 y RD-06, y hasta entonces está
      escrito como abierto en `lenguaje.md`, `contextos/resolucion.md` y `contextos/historial.md`.
    - **El formato sigue siendo el 3**, no el 4: la 3 no ha salido de este repositorio —las plantillas de
      `pipelines/` están en la 2—, así que se redefine en vez de dejar detrás una versión muerta. Lo que
      cambia lo dice el mensaje de rechazo de una versión anterior.
17. **`resolve: step-output`, `from` y `key` se retiran del formato** *(pregunta del usuario, 2026-09-15)*.
    Desde el hallazgo 16 una variable de salida pertenece a un ámbito y se usa **por su nombre**, así que
    «traer» la salida de un paso a una variable declarada dejó de nombrar nada: `resolve: step-output` solo
    renombraba, y es **exactamente** lo que hace un literal.

    ```yaml
    - name: servidor
      value: ${var.registro_servidor}    # antes: resolve: step-output, from: registro, key: registro_servidor
    ```

    La comprobación trata los dos igual: el mismo fallo si la salida no existe, y el mismo fallo de orden si se
    usa antes de producirse, porque `necesita` ya seguía los literales. `from` tampoco distinguía nada: un
    nombre se produce en un solo sitio de su ámbito (hallazgo 16), así que decir de qué paso viene es repetir
    lo que la comprobación ya sabe. Lo que se va con ellos: `SalidaDeUnPaso`, `VariableDeclarada.Salida`,
    `salidaNombrada`, la rama de `necesita`, la lista de pasos descartados —que solo existía para no repetir
    fallos de un `from`— y cinco casos de prueba, que se quedan en uno: «un valor declarado que usa un nombre
    que no existe». Un fichero de `variables/` lleva ahora `name`, `description` y `value`, y un `resolve`
    escrito falla como clave que el formato no tiene.

18. **La versión vuelve a 1, y los dos `config.yaml` cambian de nombre** *(respuesta del usuario, 2026-09-15)*.
    El «3» no describía nada de este motor: era el número de la última versión del formato en el motor viejo
    (`old-internal/`), que el nuevo nunca lee —ni la 1 ni la 2 de esa historia— y rechaza igual que rechazaría
    cualquier otra. Mantenerlo dejaba una pregunta sin respuesta para quien lea el código sin conocer esa
    historia: *¿dónde están la 1 y la 2 de este motor?* No hay ninguna, porque este motor no tiene historia
    propia todavía —no está en producción, no tiene clientes—, así que `VersionDelFormato` vuelve a `"1"`.
    Sigue siendo `string`, no `int` (hallazgo posterior a este, revertido por el mismo motivo que `DEC-08.6`
    no aplica aquí: no se aritmetiza, solo se compara contra lo que declare `schema_version`). Es la misma
    lógica del hallazgo 16 —redefinir en vez de dejar detrás una versión muerta— llevada un paso más lejos:
    allí no convenía **subir** a un 4 inventado; aquí no convenía **empezar** en un 3 inventado.

    De paso, `vexpipeline.yaml` pasa a llamarse **`config.yaml`**, y el `config.yaml` de cada paso pasa a
    **`rules.yaml`**. Los dos compartían el mismo nombre para dos ficheros que no tienen nada en común: uno
    declara la versión del formato del pipeline entero: el otro, qué mira un paso para decidir si se
    re-ejecuta. El segundo se llama por lo único que declara hoy: `rules` y `max_age`.

    **Qué cambia**: `VersionDelFormato`, el `case` de `schema_version` distinta que faltaba en `version()` (se
    restaura de paso —era una regresión de una edición anterior, no de este hallazgo—), los tipos `pipelineV1`…
    `variableV1` de `lectura.go` (venían de `*V3`), y los literales de fichero en `dominio/`,
    `infraestructura/` y `testdata/ejemplo/`. **Qué no cambia**: los identificadores del dominio
    (`Configuracion`, `ConfiguracionEscrita`) siguen llamándose así — nombran el concepto, no el fichero del
    que se leen hoy. Las plantillas de `pipelines/` siguen fuera de alcance (hallazgo 2): continúan en su
    `schema_version: 2` y con su `config.yaml` de siempre, sin relación con este cambio.

19. **Un paso declara su propio ámbito, y ese ámbito decide qué ve** *(respuesta del usuario, 2026-09-16;
    cierra `DEC-06.12`, que el hallazgo 16 dejó abierto para RD-05/RD-06; corrige una primera versión de este
    mismo hallazgo, escrita el mismo día, que decía lo contrario — ver «Corrección», más abajo)*. Lo que el
    hallazgo 16 dejó sin resolver era **bajo qué ámbito se archiva la historia de un paso**, y la respuesta no
    es derivarlo (como proponía el hallazgo 16), sino **declararlo**, porque el DevOps ya sabe qué pasos son
    de negocio compartido (el registro de contenedores, un backend de Terraform) antes de escribir ningún
    `outputs`.

    - **El ámbito de un paso es del mismo tipo que el de una variable, no un tercer concepto.** Un `ambiente`
      y un `ámbito` siguen sin ser lo mismo (`lenguaje.md`, «Pares que se confunden»): el ambiente es la
      separación de negocio (dev, staging, producción); el ámbito, el espacio de variables. Pero **el ámbito
      por defecto de un paso es el que representa a su ambiente** — el mismo `Ambito` que tendría una variable
      declarada en `variables/<ese ambiente>/`, solo que asignado por defecto y no por escrito —, y por eso
      decide igual que decidiría el de cualquier variable: **qué ve el paso**, con la misma regla (`Ambito.Ve`,
      `salidaProducida.laVe`), no solo bajo qué ámbito archiva su historia o qué hereda por defecto lo que
      produce.
    - **Dónde se escribe.** `rules.yaml` gana un campo más, con el mismo vocabulario que ya tenía `outputs`:
      `scope: environment | shared`. Sin `scope`, el ámbito del paso es el que representa al ambiente en que
      se ejecuta — el comportamiento de siempre, sin cambiar nada de lo que ya pasaba la comprobación.
      `Configuracion` y `ConfiguracionEscrita` no se dividen: `scope` es una entrada más de la misma
      configuración, junto a `rules` y `max_age`.
    - **Qué decide.** Tres cosas: **qué ve** el paso (antes solo se comprobaba con el ambiente de la
      solicitud; ahora, con el ámbito propio del paso — que por defecto *es* el de ese ambiente, así que un
      paso sin `scope` no cambia de comportamiento), bajo qué ámbito el paso archiva y busca su historia
      (Historial, Resolución — RD-05/RD-06 lo consumen, no lo deciden), y el ámbito que hereda por defecto una
      variable de salida de ese paso sin `scope` propio.
    - **Lo que cambia de verdad es el paso `scope: shared`.** Antes de este hallazgo, todo paso veía siempre
      el ambiente de la solicitud más el compartido, sin excepción — incluido `steps/02-registro`, que ya
      «era» compartido de hecho (todos sus `outputs` llevaban `scope: shared`) pero podía haber usado, sin que
      la comprobación lo impidiera, una variable declarada solo en `prod` o una salida de otro paso que no
      fuera `shared`. Con este hallazgo, un paso `scope: shared` ve **solo** lo compartido — ni una variable de
      un ambiente, ni una salida que no sea `shared`, aunque la produzca un paso anterior —, por la misma
      razón que un valor compartido no puede depender de un ambiente (hallazgo 16): si dependiera, un paso que
      «es el mismo en todos los ambientes» dejaría de serlo.
    - **La herencia.** Un `outputs` sin `scope` ya no es siempre del ambiente en ejecución: es del ámbito de su
      paso. Si el paso es `shared`, sus salidas sin marcar nacen compartidas sin repetir `scope: shared` en
      cada una; si alguna necesita ser del ambiente igual, sigue pudiendo escribir `scope: environment`
      explícito, que anula la herencia en cualquier sentido. En el dominio, heredar es copiar el mismo campo
      (`salida.Ambito = paso.Ambito`) — ver la corrección de tipo, más abajo.
    - **Por qué en `rules.yaml` y no en un fichero nuevo** (a pedido del usuario). Es la única configuración
      que ya tiene un paso, y `scope` es del mismo tipo de decisión que `rules` y `max_age`: cómo se trata un
      paso, no qué produce. Abrir un fichero más para un solo campo hubiera repetido la pregunta que
      `config.yaml`/`rules.yaml` ya resolvió en el hallazgo 18.
    - **No es el `scope` de la vieja versión, aunque ahora se le parezca más.** El motor viejo
      (`old-internal/`) tenía un `scope` de paso que decidía qué veía, y el hallazgo 16 lo retiró porque
      mezclaba visibilidad con la identidad del paso. Este `scope` también decide visibilidad, pero no vuelve
      a esa mezcla: la regla sigue siendo la de la variable (`Ambito.Ve`), aplicada al ámbito del paso en vez
      de solo al ambiente de la solicitud — no hay un vocabulario ni una comprobación distintos para el paso.
    - **La comprobación rechaza además**: un `scope` de paso que no sea `environment` ni `shared` (mismo
      mensaje que el de un `outputs`, con «el paso tiene scope…» en vez de «produce "x" con scope…»); y, ahora,
      un paso `shared` que usa una variable declarada de un ambiente o una variable de salida que no sea
      `shared` (mismo mensaje que ya existía para un valor compartido en ese caso, aplicado también dentro de
      un paso).
    - **El ejemplo lo usa de verdad**: `steps/02-registro/rules.yaml` declara `scope: shared` — «el registro es
      el mismo en todos los ambientes»—, y sus dos `outputs` (`registro_nombre`, `registro_servidor`) dejan de
      repetir `scope: shared` cada uno: lo heredan. El resultado es el mismo pipeline comprobado de antes; lo
      que cambia es que ya no hace falta decirlo dos veces, y que ahora la comprobación impediría que ese paso
      dependiera, por accidente, de una variable de un ambiente.
    - **Pruebas**: un caso feliz de herencia (un paso `shared` cuyo `outputs` sin `scope` sale compartido), un
      caso de que sin `scope` el paso sigue siendo del ambiente (el de siempre), la fila de mutación del
      `scope` de paso desconocido, y dos casos nuevos de visibilidad — un paso `shared` que usa una variable
      declarada de un ambiente, y uno que usa una variable de salida que no es `shared` —, en los dos niveles
      (`dominio` y el ejemplo en disco). Se validaron con mutaciones, igual que las de los hallazgos 16 y 17:
      quitar la asignación de `paso.Ambito` en `case "shared"` (`configuracion`) hace caer un número grande de
      pruebas de golpe (todas las que dependen del pipeline de ejemplo, porque `steps/02-registro` deja de ser
      compartido y dos de sus usos dejan de verse), y dejar que `seVe` ignore `salidaProducida.laVe` para lo
      que ya está en `produccion` hace caer el caso de la variable de salida que no es `shared` (cae en la
      comprobación de orden, con un mensaje distinto, en vez de en la de visibilidad), así que ninguna de las
      dos filas queda sin vigilar.

    **Corrección de tipo** *(mismo día, respuesta del usuario)*: la primera versión de este hallazgo
    representaba el ámbito propio de `Paso` y de `VariableDeSalida` con un booleano (`Paso.Compartido`,
    `VariableDeSalida.Compartida`). El usuario objetó que **compartido no es un concepto aparte de ámbito**:
    es su valor comodín, el mismo `Ambito` que ya usa `VariableDePipeline.Ambito`, y guardar «¿es compartido?»
    en un campo separado duplica lo que el tipo `Ambito` ya sabe decir con `EsCompartido()`. La dificultad —
    que `Ambito("")` ya significa `Compartido`, así que no queda ningún valor libre para «sin declarar,
    resuélvase contra el ambiente de la ejecución»— la resuelve el propio flujo de ejecución, que el usuario
    señaló: **siempre hay un ambiente de entrada** (nadie pide ejecutar un paso sin decir en qué ambiente), así
    que «sin declarar» nunca necesita ser un valor guardado — se resuelve en el momento, con el ambiente que
    ya se tiene a mano. Por eso el campo pasa a ser **puntero**: `Ambito *Ambito` en los dos tipos. `nil` es
    «no declaró uno propio»; no-nil solo puede apuntar a `Compartido`, porque un paso o una salida no se
    pueden declarar «del ambiente prod», solo `shared`. Heredar (`outputs()`) pasa de `salida.Compartida =
    paso.Compartido` a `salida.Ambito = clonarAmbito(paso.Ambito)`: copiar el ámbito, no un booleano derivado
    de él. `Paso.copia()` clona los punteros (`clonarAmbito`, en `valores.go`) para no compartirlos entre
    copias (§9.11: las consultas devuelven copias). `publicado.Paso.Compartido` y
    `publicado.VariableDeSalida.Compartida` **siguen siendo booleanos**: `publicado/` no depende del dominio
    y ya tenía su propia traducción explícita (`aplicacion/servicio.go`), así que el cambio no cruza esa
    frontera — se traduce como `paso.Ambito != nil`.

    **Corrección** *(mismo día, respuesta del usuario)*: la primera versión de este hallazgo decía que el
    `scope` de un paso «no decide qué ve» — que seguía viendo siempre el ambiente de la solicitud más el
    compartido, sin excepción, y que solo `Ambito.Ve` seguía fijando la visibilidad. Eso estaba mal: como el
    ámbito de un paso es del mismo tipo que el de una variable, tiene que participar de la misma regla de
    visibilidad, no quedar al margen de ella. El cambio de código fue en `usosEnLosPasos` (usa el ámbito del
    paso, no el de la solicitud, para decidir qué ve) y en `seVe` (ahora exige `salidaProducida.laVe(ambito)`
    para una variable de salida, en vez de darla por vista con solo que exista en `produccion`).

20. **`rules.yaml` se retira: la configuración de un paso vive en `config.yaml`** *(respuesta del usuario,
    2026-09-16)*. El hallazgo 18 le había dado su propio nombre porque compartía el de `config.yaml` sin
    compartir nada más; este hallazgo va más allá y **retira el fichero**, moviendo lo que declaraba —
    `rules`, `max_age`, `scope`— a `config.yaml`, bajo una clave `steps` nueva, indexada por el nombre del
    paso (sin `NN-`, el mismo que usa todo lo demás para referirse a él).

    ```yaml
    schema_version: 1
    steps:
      registro:
        rules: [instructions, variables]
        scope: shared
      imagen:
        max_age: 24h
    ```

    Un paso ausente de `steps` — o si `config.yaml` no declara `steps` en absoluto — usa los tres valores por
    defecto de siempre: las tres reglas, sin caducar, del ambiente en que se ejecuta.

    - **Dónde vive en el dominio.** `PasoEscrito.Configuracion` desaparece: un `PasoEscrito` (un directorio de
      `steps/`) ya no trae su configuración consigo, porque esa configuración no vive en su directorio.
      `Declaracion` gana `ConfiguracionesDePaso map[string]ConfiguracionEscrita`, con una entrada por cada
      nombre bajo `steps` — tal como está escrito, sin decidir nada, igual que el resto de `Declaracion`.
      `pasos()` hace el cruce: por cada directorio real, busca su nombre en el mapa (`c.d.ConfiguracionesDePaso[nombre]`)
      y llama a `configuracion()` con lo que encuentre o con `nil` — la función no cambia, solo cambia de
      dónde sale su argumento.
    - **Un nombre bajo `steps` que no es ningún paso, es un fallo nuevo.** Antes, `rules.yaml` vivía *dentro*
      del directorio del paso: no había forma de escribir la configuración de un paso que no existiera. Ahora
      que la relación es por nombre y no por ubicación, sí la hay — un `steps.notificar` cuando ningún paso
      se llama `notificar` sería un nombre mal escrito que nadie notaría, en silencio, hasta que alguien se
      preguntara por qué `notificar` no caduca nunca. `pasos()` lleva un conjunto de nombres consumidos
      mientras recorre los directorios reales, y al final reporta, ordenados, los que quedan en
      `ConfiguracionesDePaso` sin consumir: *«config.yaml: declara la configuración de "notificar", que no es
      un paso: no hay ningún steps/NN-notificar/»* (invariante `pasos`, como el resto de fallos sobre nombres
      de paso).
    - **Pérdida aceptada, y es nueva de este hallazgo.** Antes, un `rules.yaml` roto solo afectaba a *su*
      paso: los demás seguían leyéndose de sus propios ficheros. Ahora todos los pasos comparten un único
      documento YAML, así que **una entrada de `steps` que no se puede decodificar hace ilegible el
      `config.yaml` entero**, y ningún paso recibe su configuración — todos caen a los valores por defecto,
      no solo el que tenía el error. Esto puede arrastrar fallos de otro tipo: en el ejemplo, si `steps`
      queda sin leerse, `registro` pierde su `scope: shared`, y sus dos `outputs` (que ya no repiten el scope
      porque lo heredan) dejan de ser compartidos — lo que hace que la literal compartida `servidor`
      (`${var.registro_servidor}`) falle al usar una salida que ya no ve. Es la misma clase de pérdida que ya
      aceptaba `config.yaml` con `schema_version` (un `config.yaml` ilegible detiene todo), extendida a un
      fichero que ahora declara más cosas.
    - **Por qué no un fichero por paso, ni una lista** (a pedido del usuario, que fijó la forma exacta:
      `steps` como mapa, anidado bajo la raíz de `config.yaml`, no una clave plana ni una lista con `name`).
      Un mapa es la forma más corta de decir «esta configuración es de este paso» sin repetir el nombre como
      campo, y coincide con cómo el propio usuario lo escribió al proponerlo.
    - **Qué no cambia.** `ConfiguracionEscrita` (`Reglas`, `ReglasEscritas`, `EdadMaxima`, `Ambito`) es la
      misma estructura que ya existía para `rules.yaml`: solo cambia de dónde la llena el ACL. `configuracion()`,
      la validación de `rules`/`max_age`/`scope` y la herencia hacia `outputs()` no cambian una línea: todas
      recibían ya un `*ConfiguracionEscrita` opcional, y siguen recibiéndolo.
    - **El ejemplo**: los cinco `steps/NN-*/rules.yaml` se borran, y su contenido pasa a `config.yaml` bajo
      `steps`, con las mismas cinco entradas (`pruebas`, `registro`, `infraestructura`, `imagen`, `aviso`);
      `despliegue` sigue sin entrada, como seguía sin `rules.yaml`. El pipeline comprobado que resulta es
      idéntico al de antes.
    - **Pruebas**: un caso nuevo («`config.yaml` declara la configuración de un paso que no existe»), y todas
      las mutaciones que antes escribían sobre un `rules.yaml` de un paso ahora escriben sobre `config.yaml`
      — algunas ganan `varios: true` que no tenían, porque ahora sí arrastran la configuración de los demás
      pasos (ver la pérdida aceptada, arriba). Se validó con mutación: quitar el reporte de los nombres sin
      consumir hace caer un caso; dejar caer la asignación desde el mapa no compila (el argumento de
      `configuracion()` deja de tener de dónde salir), así que esa fila no tiene ni forma de quedar sin
      vigilar.
