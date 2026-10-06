# RD-09 — Simulación de Pipeline

> Contexto: **Simulación de Pipeline** (`contextos/simulacion.md`) · Depende de: RD-03, RD-04, RD-05 ·
> Frente 2: **B**

## §1 Problema

El DevOps no puede saber si su pipeline funciona antes de publicarlo.

## §2 Por qué importa

Es lo que el DevOps quiere hacer antes de publicar, y no puede tener ningún efecto.

## §3 Objetivo

*Simular* (SIM-1 y SIM-2), con su informe.

## §4 Alternativas

- **Un modo de Ejecución.** Descartada: *intento* y *variable producida* significarían dos cosas dentro de
  Ejecución (`DEC-03.10`).

## §5 Solución

- **Dominio**:
  - **Simulación**: sin efectos; recorre todos los pasos de cada ambiente; las salidas simuladas cumplen su
    expresión regular; el informe no lleva valores.
  - Servicio *fabricar una salida simulada*.
- **Aplicación**: *simular*.
- **Infraestructura**: un espacio temporal, y la interpolación de Resolución en una petición sin intento.

## §6 Alcance

**Dentro**: lo anterior. **Fuera**: simular fallos de comandos (`DEC-10.4`).

## §7 Verificación

- No escribe en el Historial ni toca el espacio de trabajo de ningún ambiente.
- Recorre todos los pasos de cada ambiente.
- Toda salida simulada cumple su expresión regular.
- Si una variable no se puede interpolar, aparece en el informe y la simulación sigue con el siguiente
  ambiente.
- Acepta una copia de trabajo.

## §8 Decisiones

`DEC-03.10` · `DEC-04.6` · `DEC-04.10` · `DEC-10.4` · `DEC-10.6`

## §9 Hallazgos al implementar

*Implementada el 2026-09-18.* Compila, pasan sus pruebas y pasa la regla de dependencias.

1. **Prerequisito real, no opcional: `ParaSimulacion` de Definición no aceptaba copia de trabajo.**
   `DEC-10.6` exige que Simulación acepte las dos, pero `definicion/publicado.ParaSimulacion` solo tenía
   `DeUnCommit`. Se corrigió antes de tocar `internal/simulacion/` — ver RD-03 §9 hallazgo 11 y RD-04 §9
   hallazgo 21, ambos ya anticipaban exactamente esta corrección.

2. **Sin puerto `Fuentes` (Suministro) directo en Simulación**, aunque `simulacion.md` y `context-map.md`
   fila #11 lo declaren y la regla de dependencias mecánica ya permita `simulacion → suministro`. La razón no
   es evitar una redundancia arquitectónica: es que `resolucion/publicado.ParaSimulacion.Declarar` /
   `.Interpolar` (ya implementado en RD-05) **no tienen** el parámetro `estandar map[string]string` que sí
   tiene `ParaEjecucion.VariablesDeUnPaso` — no hay ningún sitio donde enchufar un valor traído de Suministro
   aunque Simulación lo trajera, sin tocar RD-05 (fuera de alcance). `suministro/publicado.ParaSimulacion`
   queda sin cliente real tras esta unidad: sigue siendo una relación anticipada válida, no se borra.

3. **"espacio temporal" no es un directorio en disco.** `Paso.Material[].Contenido` y `Comando.Linea` llegan
   como `string` desde Definición: interpolar es una operación en memoria. Se realiza como
   `dominio.EspacioTemporal.Nuevo() (string, error)` — un id de invocación efímero para
   `resolucion.ParaSimulacion` (que ya gestiona su propio mapa en memoria, liberado por `Cerrar` —
   `DEC-04.10`) — implementado con `uuid.NewV7()`, mismo patrón que `historial/dominio.Identidades` +
   `historial/infraestructura.IdentidadesUUID`.

4. **Corrección crítica de diseño, encontrada al escribir la prueba de aislamiento**:
   `resolucion/dominio.VariablesDeUnaInvocacion.porNombre` está indexado **solo por nombre**, no por
   (nombre, ámbito) — `Declarar` no sobreescribe. Los literales de `variables/` se repiten con el mismo
   nombre y distinto valor entre ambientes (el caso normal, no el raro). **Por eso `EspacioTemporal.Nuevo()`
   se llama una vez POR AMBIENTE** (`recorrerAmbiente` en `aplicacion/bucle.go`), no una vez por llamada a
   `Simular`: cada ambiente usa su propio id de invocación de Resolución, aislado de los demás, y se cierra
   (`defer Cerrar`, con `errors.Join` si falla) antes de pasar al siguiente. Compartir un solo id entre
   ambientes habría producido fugas silenciosas de valores de un ambiente a otro.
   `TestSimularAislaLasVariablesDeCadaAmbiente` (`aplicacion/simular_test.go`) lo fija.

5. **Sin variables estándar (`VariablesEstandar()`) en el MVP** — misma razón que el hallazgo 2: la firma de
   `ParaSimulacion` no tiene dónde recibirlas. Si un comando o una plantilla las referencia, aparecen como
   `Faltante` en el informe — comportamiento correcto de SIM-2, no una laguna.

6. **Aserciones se ignoran por completo**: no hay salida real de comando contra la que evaluarlas, y
   `DEC-10.4` excluye simular fallos de comando. `simulacion/dominio.Comando` ni siquiera tiene un campo
   `Aserciones`.

7. **`Interpolado` y `Faltante` del informe, precisados**: `Interpolado` son los nombres de las variables de
   salida para las que se fabricó y registró una salida simulada en ese paso — la única lectura implementable
   sin tocar RD-05, cuyo `Interpolar` no expone qué nombres se sustituyeron con éxito, solo el primero que
   falta. `Faltante` se acumula dentro de un mismo paso (todas las plantillas y líneas de comando de ese paso
   se intentan interpolar, juntando todos los nombres que faltan) antes de abandonar el resto del ambiente —
   SIM-2 dice "sigue con el siguiente ambiente", no "con el siguiente paso", así que un paso roto no corta
   antes de dar toda la información que pueda. Un comando cuya línea no interpola no fabrica sus salidas (no
   se inventa nada de más, `DEC-10.4`), pero si su línea sí interpola, sus salidas se fabrican aunque un
   comando *anterior* del mismo paso haya fallado — el fallo no se propaga hacia adelante dentro del paso,
   solo se acumula.

8. **`suministro/publicado.ParaSimulacion` y `definicion/publicado.ParaSimulacion.VariablesEstandar()` quedan
   sin cliente real** en esta unidad (hallazgos 2 y 5). No se retiran: RD-10 o una futura extensión de RD-05
   podrían activarlos sin volver a tocar Suministro o Definición.

9. **Sugerencia para el modelo** (`docs/modelo/contextos/simulacion.md`, tabla «Puertos»): la fila `fuentes` no
   se realizó como una dependencia directa de código — queda transitiva, vía el `ParaSimulacion` de
   Definición. Si se contrasta con el modelo, la fila podría anotarse como «realizada transitivamente vía
   Definición» en vez de retirarse, porque `context-map.md` fila #11 sigue siendo una relación anticipada
   correcta (RD-03 §9 hallazgo 7).
