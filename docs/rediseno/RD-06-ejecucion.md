# RD-06 — Ejecución de Pipeline y borde mínimo

> Contexto: **Ejecución de Pipeline** (`contextos/ejecucion.md`) · Depende de: RD-02, RD-03, RD-04, RD-05 ·
> Frente 2: **A, B, C**

## §1 Problema

Nada lleva a cabo un intento.

## §2 Por qué importa

Es el **primer corte que funciona de punta a punta**: la herramienta vuelve a desplegar. Y es donde caen las
tres cadenas (`DEC-09.4`): baja el acoplamiento, las reglas del desenlace se cumplen por construcción y la
decisión de re-ejecutar se puede probar sin ejecutar nada.

## §3 Objetivo

- El agregado **Intento en curso**.
- El servicio de dominio *decidir un paso*.
- El servicio de aplicación *intentar*, que cubre EJ-1 a EJ-5, rollback incluido.
- El espacio de trabajo, con la parte del motor y la de la tecnología.
- Un borde mínimo con las operaciones *intentar* y *hacer rollback*.

## §4 Alternativas

- **Mantener una cadena por debajo del servicio de aplicación.** Descartada (`DEC-09.4`).

## §5 Solución

- **Dominio**:
  - **Intento en curso**: orden de los pasos; ningún paso empieza sin el registro del anterior escrito; un
    fallo cierra el intento; la cancelación gana; un solo desenlace.
  - *Decidir un paso*: sin efectos. Solo puede no re-ejecutarse si la última vez es un final exitoso, y las
    variables producidas cuentan.
  - Value objects: recursos de un paso, regla, decisión, destino y resultado de un comando.
- **Aplicación**: *intentar*, con un bucle explícito; cada paso ejecutado deja un comienzo y un final.
- **Infraestructura**:
  - Puertos hacia Definición, Suministro, Resolución e Historial.
  - El puerto **comandos**, que ejecuta un comando en el espacio de trabajo del ambiente.
  - El puerto **espacio de trabajo**, que rehace la parte del motor.
  - El hash de las instrucciones de un paso: sus comandos y su material, sin su configuración.
- **Borde mínimo**: una petición por invocación, con la versión del lenguaje publicado (`DEC-05.6`). La
  invocación dice dónde está el espacio de trabajo del ambiente y cómo llegar al almacén.
- **La salida de los comandos**: el puerto **salida** la entrega en vivo, tal cual, a quien pidió el intento,
  y no se guarda (`DEC-12.5`). El borde mínimo la conecta con la salida de la invocación.

## §6 Alcance

**Dentro**: lo anterior, en local. **Fuera**: el resto de operaciones del borde (RD-10) y lo que se compra
(RD-11).

## §7 Verificación

- *Decidir un paso*: una prueba por regla, y por cada caso de la última vez (final exitoso, final fallido,
  comienzo sin final).
- La cancelación gana a un fallo que ella misma provoca; un fallo cierra el intento; nada empieza sin el
  registro anterior escrito.
- La parte del motor del espacio de trabajo se rehace, y la de la tecnología no se toca.
- **De punta a punta**, en local y con un pipeline de prueba:
  - un segundo intento sin cambios no re-ejecuta nada;
  - cambiar una plantilla re-ejecuta ese paso;
  - un intento con copia de trabajo no llega a despliegue;
  - un segundo intento en el mismo ambiente se rechaza;
  - lo que imprime un comando llega a la salida mientras corre, y no aparece en ningún registro.

## §8 Decisiones

`DEC-05.6` · `DEC-06.17` a `DEC-06.19` · `DEC-07.8` · `DEC-08.7` · `DEC-09.2` a `DEC-09.8` · `DEC-10.7` · `DEC-12.5`

## §9 Hallazgos al implementar

*2026-09-17.*

1. **Dos fuentes, dos commits, y los dos van en la apertura.** `docs/modelo/dominio.md` («Suministro de
   Fuentes») deja claro que el código del proyecto y el pipelinecode son dos fuentes de distinto dueño, cada
   una con su propio commit. `PeticionDeIntento` los lleva por separado
   (`FuenteDelProyecto`/`CommitDelProyecto`, `FuenteDelPipeline`/`CommitDelPipeline`), y
   `AperturaDeIntento.Contenido` (codificado `ejecucion/apertura-v1`, JSON) guarda los cuatro — es lo único que
   permite que `DespliegueParaRollback` reconstruya el commit del pipeline además del código en EJ-2, que la
   ficha no distingue de forma explícita.

2. **El puente estándar se cierra en la aplicación, no en infraestructura.** `variables_estandar.go` construye
   el `map[string]string` que cierra el comentario "mientras RD-06 no exista" de
   `resolucion/publicado.VariablesDeUnPaso` y `resolucion/dominio.VariableEstandar`, con las nueve claves de
   RD-04 §9.1. `project_workdir` es el directorio del **material del proyecto** que trae Suministro
   (`Material.Directorio`), no el espacio de trabajo del pipeline — son dos cosas de distinto dueño y el
   nombre por sí solo no lo distingue.

3. **`HastaPaso` tiene que llegar resuelto a Historial.** `historial/dominio.Intento` exige que
   `Apertura.HastaPaso` sea el nombre de un paso declarado — un `HastaPaso` vacío ("hasta el último", que sí
   entiende `dominio.NuevoIntentoEnCurso` de este contexto) lo rechaza con «el paso pedido "" no es un paso
   del pipeline». `resolverHastaPaso` (en `intentar.go`) resuelve el vacío al último paso del pipeline antes de
   construir la apertura, y el mismo valor resuelto es el que recibe el agregado — nunca dos criterios para
   "hasta dónde" a la vez.

4. **EJ-5 se comprueba antes de abrir en el Historial.** `RehacerParteDelMotor` se llama antes de
   `AbrirIntento`: así un ambiente sin espacio de trabajo alcanzable no deja un intento abierto que nunca
   llega a tener un desenlace — el Historial nunca se entera de un intento que no pudo ni empezar.

5. **El hash de instrucciones es dominio, no infraestructura**, a pesar de que §5 lo lista bajo
   infraestructura. `CalcularHashDeInstrucciones` es un cálculo puro sobre `ComandoDeclarado` y
   `FicheroDeclarado` — los dos ya son tipos de este dominio —, sin E/S: el mismo patrón que
   `resolucion/dominio.CalcularHashDeVariable`. Ponerlo en infraestructura habría obligado a exponer un
   constructor de `HashDeInstrucciones` sin validar (o a un `panic` en un camino que nunca falla), y las dos
   opciones son peores que seguir el precedente ya sentado por Resolución.

6. **La edad de la última vez se mide desde el último registro, no desde la ejecución real original.**
   `historial/publicado.ParaEjecucion` no tiene una consulta "dame el registro de este intento y este paso"
   (a diferencia de `ParaResolucion.VariablesDeUnPaso(intento, paso)`, que sí la tiene y es lo que le permite a
   Resolución seguir el salto de evidencia hasta el hash de verdad). Sin ese punto de apoyo, `UltimaVezDeUnPaso`
   de este contexto calcula la edad con el `Instante` del **último** registro —Final o No-reejecución—, así
   que una cadena de intentos que no re-ejecutan un paso va corriendo el punto de partida de `max_age` en cada
   uno. Aceptado: un paso que se comprueba seguido sigue siendo "reciente" a efectos de `max_age`, que es una
   lectura razonable de "vigencia", aunque no sea "cuánto hace que se hizo de verdad".

7. **`regla: variables` solo es fiable si el paso no ve nada declarado sin producir nada propio** —límite de
   Resolución (RD-05), no de esta unidad. `resolucion/aplicacion.cambiaronLasVariables` compara **todo lo
   visible ahora** (`inv.Visibles`, declaradas + producidas) contra lo que hay en el Historial bajo ese paso, y
   solo lo producido llega alguna vez al Historial (`registrarProducido` es el único llamador de
   `RegistrarVariable`; declarar una variable —literal o estándar— nunca escribe nada, ver
   `variables_de_un_paso.go`). Un paso que ve variables declaradas o estándar pero no produce ninguna termina
   comparando un conjunto no vacío contra uno vacío, y `dominio.Cambiaron` lo cuenta como cambio — siempre. El
   pipeline de prueba de esta unidad (`internal/ejecucion/testdata/ejemplo`) evita el caso a propósito
   (`desplegar` no declara `variables` en su regla) en vez de forzarlo, porque corregirlo es tocar RD-05, fuera
   de alcance aquí. Queda para quien retome Resolución: o declarar también escribe un hash al Historial, o
   `cambiaronLasVariables` compara solo lo producido.

8. **El espacio de trabajo es `<raíz>/<proyecto>/<pipeline>/<ambiente>/<paso>/`.** `<proyecto>` y `<pipeline>`
   no son `Metadatos.ProjectName`: salen de `FuenteDelProyecto` y `FuenteDelPipeline` —la URL del remoto
   `origin` si la fuente lo tiene, o el nombre de su directorio— y se vuelven nombre de directorio con
   `dominio.NombreDeDirectorio`: los 16 primeros hex del SHA-256 de la forma canónica (`host/ruta`, sin usuario,
   sin `.git`), así que la misma fuente da siempre el mismo nombre y `ssh`/`https` del mismo repositorio
   coinciden. Con `CopiaDeTrabajo`, la fuente que identifica al proyecto es la propia copia. Motivo: un
   proyecto usa varios pipelines a lo largo de su vida, y un pipeline lo usan varios proyectos; sus datos no se
   mezclan. `RehacerParteDelMotor` borra y reescribe solo el directorio de cada paso (`DEC-06.19`), nunca el
   del ambiente: lo que los comandos escriban fuera de ellos —p. ej. un `tfstate`— no se toca. Un paso que
   desaparece del pipeline deja su directorio viejo. La disposición no la fija el modelo, así que queda
   registrada aquí como la que implementan `EspacioDeTrabajo.Ubicar`/`DirectorioDelPaso`.

9. **`cmd/vexd` sigue sin tocarse (`DEC-11.1`/`DEC-11.2`).** `internal/borde` queda listo — `Servicio`,
   `comprobarVersion`, `Intentar`, `HacerRollback` — pero nadie lo cablea todavía a un binario real: eso es
   RD-12, que también decide qué da `DirectorioDeEspacioDeTrabajo`/`DirectorioDelAlmacen` de cada petición (hoy
   viajan en `publicado.PeticionDeIntento`/`PeticionDeRollback` sin que ningún código de esta unidad los lea —
   son la raíz de composición quien los necesita, para construir `EspacioDeTrabajo`/el almacén antes de llamar
   al servicio).
