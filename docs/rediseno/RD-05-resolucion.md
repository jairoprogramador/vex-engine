# RD-05 — Resolución de Variables

> Contexto: **Resolución de Variables** (`contextos/resolucion.md`) · Depende de: RD-02, RD-04 · Frente 2:
> **B**

## §1 Problema

No hay valores efectivos, ni interpolación, ni forma de saber si una variable cambió.

## §2 Por qué importa

Sin ella no se puede ejecutar ningún comando. Y es donde se cumple, por construcción, que el valor no se
guarde ni se publique en claro.

## §3 Objetivo

El agregado **Variables de un intento**, la interpolación, el hash simple, *¿cambiaron las variables de un
paso?* y la petición sin intento.

## §4 Alternativas

- **Resolver dentro de Ejecución.** Descartada: la promesa de comparar sin exponer pasaría de frontera a
  disciplina (`DEC-03.5`).

## §5 Solución

- **Dominio**:
  - **Variables de un intento**, con sus invariantes: ámbito; orden; la producida gana a la declarada, y
    entre producidas la del paso más reciente; un paso que no se re-ejecuta aporta los valores de la última
    vez; el valor no circula; sin intento no se deja nada.
  - Value objects: variable efectiva, origen, ámbito y hash de variable (simple, sin clave).
  - Servicios: interpolar, calcular el hash y *¿cambiaron?*.
- **Aplicación**: las variables de un paso, interpolar y registrar lo que produjo un comando.
- **Publicado**: lo anterior, para Ejecución y para Simulación.
- **Infraestructura**: escribe en el Historial adoptando su forma (el hash, bajo paso e intento) y usa la
  relación reservada para el valor ofuscado.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: decidir bajo qué ámbito se busca la última vez de un paso, que RD-04
§9.19 cerró declarándolo en `config.yaml` (`DEC-06.12`) — aquí solo se **usa**, pidiéndoselo a Definición;
y decidir si un paso se re-ejecuta, que es de Ejecución.

## §7 Verificación

- Pruebas de precedencia y de ámbito, incluido el compartido: un paso ve las variables del ámbito de su
  ambiente y las del compartido, y lo que produce un comando lo ven los posteriores de su paso.
- El mismo valor da el mismo hash en todos los ambientes de un proyecto.
- Una petición sin intento no calcula hashes ni escribe nada.
- El valor solo sale hacia la interpolación y hacia la relación reservada.

## §8 Decisiones

`DEC-03.5` · `DEC-03.16` · `DEC-04.10` · `DEC-06.12` · `DEC-08.3` a `DEC-08.6` · `DEC-08.8` · `DEC-09.6`

## §9 Hallazgos al implementar

*Implementada el 2026-09-17.* Compila, pasan sus pruebas (dominio, aplicación e infraestructura) y pasa la
regla de dependencias.

1. **El agregado se llama `VariablesDeUnaInvocacion`, no «Variables de un intento».** `DEC-08.3` ya habla de
   «invocación», y el agregado sirve igual a un intento real que a una petición sin intento (`DEC-04.10`): el
   dominio no distingue entre las dos, y ponerle «intento» en el nombre hubiera dejado sin uno coherente a la
   mitad de sus llamadores.
2. **`Origen` tiene dos valores, no cuatro** como el motor antiguo (`OriginDeclared`, `OriginState`,
   `OriginInjected`, `OriginRuntime`). `DEC-08.4` solo distingue declarada de producida, y «la última vez que
   un paso no se re-ejecutó» no es un origen aparte: entra como producida, porque aporta estructuralmente lo
   mismo que si se hubiera producido ahora (hallazgo 3).
3. **`NoReejecutado` es una operación nueva**, no una de las cuatro que lista el §3. Hacía falta un punto de
   entrada para «un paso que no ejecuta ningún comando, pero cuyo valor sigue haciendo falta, aporta las
   variables de su última vez» — se cuenta como parte de «las variables de un paso» del catálogo original, no
   como alcance añadido. No escribe nada: los hashes y los valores ya están en Historial de cuando se
   produjeron de verdad.
4. **El ámbito de cada variable producida se pierde en el viaje de ida y vuelta por Historial si Resolución no
   lo guarda ella misma.** `Historial.RegistrarVariable` no acepta un ámbito (solo intento, paso y nombre), y
   la relación reservada (`ValoresDeUnPaso`) devuelve un `map[string]string` plano, sin metadato ninguno. Una
   variable de salida puede ser compartida aunque su paso no lo sea (`VariableDeSalida.Compartida`), así que
   el ámbito con el que se produjo tiene que sobrevivir para que `NoReejecutado` la vuelva a declarar donde
   corresponde, no donde se pregunta. Se guarda en el propio `Contenido` que Resolución escribe (`{hash,
   origen, compartido, ambiente}`, contexto `resolucion/variable-v1`) — Historial nunca lo interpreta (IT-04
   `DEC-04.7`), y `dominio.Historial.ValoresDeLaUltimaVez` devuelve `ValorDeLaUltimaVez{Valor, Ambito}` en vez
   de un `string` suelto.
5. **`UltimaVezDeUnPaso` puede devolver una no re-ejecución, y hay que seguir su evidencia una sola vez.** El
   registro con las variables de verdad (con su `Contenido`) vive en el intento y el paso que apunta la
   evidencia, nunca en el propio registro de no re-ejecución. Nunca hace falta más de un salto:
   `Intento.NoReejecutar` en `historial/dominio` exige que la evidencia apunte a un `final` exitoso, nunca a
   otra no re-ejecución. Resuelto en `resolucion/infraestructura`, así el dominio de Resolución nunca ve un
   intento ni una evidencia.
6. **`RegistrarProducido` escribe el hash y el valor en dos llamadas sin transacción entre ellas** (primero
   `Historial.RegistrarVariable`, después `GuardarValor` por la relación reservada, y solo si las dos llegan
   se pliega en memoria). Si `RegistrarVariable` falla, no se intenta `GuardarValor`. Si `GuardarValor` falla
   después de un `RegistrarVariable` exitoso, el hash queda escrito pero el valor no se recupera y la variable
   no se pliega en memoria — pérdida aceptada: son dos agregados distintos y no hay una segunda fase que
   deshaga la primera.
7. **Las variables estándar entran como declaradas, antes que los literales**, y su ámbito depende de
   `VariableEstandar.DelPaso`: si es de cada paso, el del paso que la pide; si no, el compartido — de otro
   modo una estándar compartida (como las que trae `RD-04 §9.1`, por ejemplo `project_hash`) quedaría
   invisible para los pasos de un ambiente. El choque de nombre entre una estándar y un literal del pipeline
   no se resuelve aquí (una declarada nunca sobreescribe, así que gana quien se declaró primero); detectarlo,
   si hace falta, queda para Definición o un hallazgo posterior.
8. **El orden de `Pipeline.Variables` no es de dependencia** (investigado antes de escribir el código: RD-04
   §9.16 dice «el orden pasa a comprobarse donde se usa, no donde se declara», y el orden real es alfabético
   por fichero — `filepath.WalkDir` — no de declaración). Un literal puede referenciar a otro que aparece
   después que él en la lista, y Definición solo garantiza que no hay ciclos ni referencias que no se vean,
   nunca un orden útil para declarar. Por eso `declararLiterales` no recorre la lista una sola vez: declara
   por **punto fijo** — en cada vuelta declara los que ya interpolan con lo que hay hasta ahora, hasta que no
   quede ninguno pendiente. Para un pipeline comprobado siempre converge, porque Definición ya garantiza que
   no hay ciclos ni referencias sin ver.
9. **Un solo tipo no puede implementar `ParaEjecucion` y `ParaSimulacion` a la vez.** Las dos interfaces
   declaran `Interpolar` y `RegistrarProducido` con nombres iguales pero firmas distintas (una lleva `paso`,
   la otra no), y Go no admite dos métodos del mismo nombre en un solo tipo. Se resolvió con dos vistas
   delgadas (`paraEjecucion`, `paraSimulacion`) que envuelven el mismo `*Servicio` y delegan en sus métodos
   internos sin nombre en conflicto (`variablesDeUnPaso`, `interpolar`, `cambiaronLasVariables`,
   `registrarProducido`, `noReejecutado`, `declarar`, `producirEnMemoria`, `cerrar`) — `Servicio.ParaEjecucion()`
   y `Servicio.ParaSimulacion()` dan cada vista a la raíz de composición.
10. **Las pruebas de aplicación son contra dobles propios** de `dominio.Historial` y `dominio.Definicion` —
    los puertos que el propio dominio de Resolución declara — en vez de contra los servicios reales de
    Historial o Definición: probar contra la frontera que Resolución ya decidió mantiene un contexto aislado
    del otro (`DEC-11.3`). El doble de Historial se puede poner en modo «prohibido» (falla la prueba si se le
    llama), y así prueba en positivo que una petición sin intento nunca lo toca. El adaptador de
    infraestructura sí se prueba contra el Historial real, sobre `historial/infraestructura.AlmacenEnMemoria`.
