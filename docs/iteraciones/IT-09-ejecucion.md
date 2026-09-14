# IT-09 — Ejecución de Pipeline

> Etapa: E9 *(en el plan, «Orquestación y Ejecución de Step»)* · Estado: **cerrada**
> Abierta: 2026-09-14 · Cerrada: 2026-09-14
> Lectura previa: `guia-ddd.md` §8 (servicios de dominio), §10 (agregados) y §13 (servicios de aplicación)
>
> Proceso de `DEC-03.15`: Claude propone a partir del modelo y de las respuestas, y trae las
> incongruencias **una a una** para analizarlas juntos.
>
> **1 incongruencia · 8 decisiones.** Las tres cadenas caen, con su argumento escrito. **Archivo cerrado:
> no se vuelve a editar.**

---

## 1. Qué se quiere

**Cierre común del bloque B**: agregados con sus invariantes (o por qué no los tiene), entidades y value
objects, eventos, servicios de dominio, repositorios y factorías, servicios de aplicación y puertos.
**Cierre propio de E9** (`plan-ddd.md` §8): el destino de las tres cadenas está decidido, con su
argumento escrito.

**La vara del frente 2 de `DEC-01.10`**: una propuesta de diseño entra solo si cumple al menos una de
estas cuatro:

- baja acoplamiento medible;
- fuerza una regla por construcción;
- simplifica un test existente;
- se puede revertir tocando un solo contexto.

**Las preguntas que el plan le hace a E9:**

| Pregunta del plan | Respuesta |
|---|---|
| ¿Qué agregado protege qué invariante? | **Intento en curso**: el orden de los pasos, que nada empiece sin estar escrito lo anterior, que un fallo cierre el intento, que la cancelación gane y que haya un solo desenlace (`DEC-09.2`) |
| ¿Es la ejecución la raíz de la orquestación? | No: *ejecución* es un acto, nunca una entidad (`DEC-02.13`). La raíz es el **Intento en curso** |
| ¿Sobrevive la cadena de responsabilidad? | **Cae** (`DEC-09.4`) |
| ¿Cómo se separa «la misma pieza decide, hashea y escribe»? | Decide *decidir un paso*, un servicio de dominio sin efectos; cada hash lo calcula su dueño; escribe el servicio de aplicación, en el Historial (`DEC-09.3`) |

**Dudas que vencen aquí:**

| Origen | Duda | Dónde queda |
|---|---|---|
| IT-01 §9 **#6** | Dónde va la restauración del material tras un intento fallido | **Sin objeto**: no se restaura; el material del motor se rehace al empezar cada intento (`DEC-06.19`) |
| IT-01 §9 **#7** | Si se retira la protección contra accesos simultáneos y la guarda de transiciones del intento | La protección se retira: no hay dos hilos que compartan el intento. La guarda se **justifica** como invariante: la cancelación gana al fallo que ella misma provoca (`DEC-09.2`) |
| IT-03 §9 **#5** | La separación táctica entre decidir y hacer dentro de Ejecución | `DEC-09.3` |
| IT-05 §6 **#2** | El destino de las tres cadenas y del objeto de contexto compartido | `DEC-09.4`. Cómo se llega desde el código es de E11 |
| IT-06 §6 **#1** | Cómo separa el motor lo que pone él de lo que genera la tecnología, y cómo conoce su espacio de trabajo y sus registros | `DEC-09.5` |

---

## 2. Propuesta

El modelo está en `modelo/contextos/ejecucion.md`. Lo que lo sostiene:

1. **Primero los escenarios** (EJ-1 a EJ-5), como en los demás contextos.
2. **Un agregado que vive durante la invocación**: el Intento en curso, con las reglas de su desenlace.
3. **Decidir es un servicio de dominio sin efectos.** Recibe los hechos y devuelve una decisión, sin leer,
   calcular hashes ni escribir. Es lo que el libro llama servicio de dominio (`guia-ddd.md` §8), y lo que
   hace posible probar la regla más delicada del motor sin ejecutar nada.
4. **El servicio de aplicación coordina con un bucle a la vista** (`guia-ddd.md` §13). Las reglas quedan
   en el dominio, y el orden, a la vista.
5. **Las cadenas caen.** El recorrido es una secuencia fija, y lo que la cadena escondía queda escrito.

---

## 3. Incongruencias

### Q-09.1 — ¿Qué cuenta como *la última vez* de un paso, si el paso falló o no llegó a terminar? · **resuelta**

**La incongruencia.** Un paso no se re-ejecuta si nada de lo que mira cambió desde su **última vez**, que es
*su último registro en su ámbito* (`DEC-06.12`). Pero hay dos casos en los que ese último registro engaña:

- **El paso falló.** Por ejemplo, un terraform que aplicó la mitad de los cambios. Si en el siguiente intento
  nada cambió, la comparación dice *«no había nada diferente»* y el paso **no se re-ejecuta**, aunque nunca
  llegó a terminar bien.
- **El paso no llegó a terminar**, porque la máquina murió a mitad. Si el registro de un paso solo se
  escribe cuando termina, ese intento no deja ningún registro del paso. La última vez vuelve a ser la
  anterior, la que salió bien, y el paso **no se re-ejecuta**, aunque el mundo quedó a medio tocar.

**Propuesta.**

- **Cada paso deja dos registros**: uno **al empezar**, antes de su primer comando, y otro **al terminar**,
  bien o mal.
- **Un paso solo puede no re-ejecutarse si su último registro es un final exitoso.** Si su último registro
  es un comienzo sin final, o un final fallido, se re-ejecuta, cambie lo que cambie.

El agregado Intento del Historial pasa de *un registro por paso* a *un comienzo y, si llega, un final por
paso*.

**La alternativa que cambiaría el resultado.** Tomar como última vez el último final **exitoso** de ese
paso. Es más simple, pero un paso que dejó el mundo a medias en un intento posterior podría no volver a
ejecutarse, porque su último éxito sigue coincidiendo.

**Respuesta.** **Sí**: se adopta la propuesta (`DEC-09.7`).

---

## 4. Decisiones

> Las marcadas *(por defecto)* salen del modelo y de lo ya respondido, y se revierten si no sirven
> (`DEC-03.15`).

### DEC-09.1 — Las preguntas del plan para E9 ya tienen respuesta *(por defecto)*

**Decisión.** Las tablas de §1.

### DEC-09.2 — El Intento en curso es el agregado de Ejecución *(por defecto)*

**Decisión.** El intento mientras se lleva a cabo, con estas invariantes: orden de los pasos; ningún paso
empieza sin que esté escrito el registro del anterior; un fallo cierra el intento; la cancelación gana;
un solo desenlace. Vive durante la invocación.

**Por qué.** Son las reglas que tienen que cumplirse en todo momento mientras el intento avanza. La
protección contra accesos simultáneos sobra, porque un intento no lo comparten dos hilos (`DEC-01.9`). La
guarda de transiciones se queda, convertida en invariante: una cancelación provoca un fallo, y ese fallo
no puede taparla.

**Consecuencias.** Cierra la duda **#7** de IT-01.

### DEC-09.3 — Decidir un paso es un servicio de dominio sin efectos *(por defecto)*

**Decisión.** *Decidir un paso* recibe la regla, los recursos de ahora y la última vez, y devuelve una
decisión, sin leer, calcular hashes ni escribir. Cada hash lo calcula su dueño, y escribe el servicio de
aplicación.

**Por qué.** Separa las tres cosas que una sola pieza hacía a la vez (decidir, hashear y escribir), y hace
que la regla más delicada del motor se pueda comprobar sin ejecutar nada. Cumple el frente 2.B (fuerza una
regla por construcción) y el 2.C (simplifica la prueba de esa regla).

**Consecuencias.** Cierra la duda **#5** de IT-03.

### DEC-09.4 — Las tres cadenas caen *(por defecto)*

**Decisión.** Las tres cadenas de hoy (pipeline, paso y comando) se sustituyen por el servicio de
aplicación *intentar*, con un bucle explícito sobre los pasos, el servicio de dominio *decidir un paso* y
el agregado **Intento en curso**. El objeto de contexto compartido desaparece: su contenido se reparte
según la tabla de `ejecucion.md`.

**Por qué.** Una cadena de responsabilidad se justifica cuando los eslabones cambian o pueden cortar el
paso. Aquí el recorrido es fijo, y la cadena escondía reglas en su orden: el orden de carga decidía qué
estaba completo. Con un bucle y unas invariantes, esas reglas quedan a la vista. Cumple el frente 2.A (baja
el acoplamiento: ya no hay un objeto que atraviese todo) y el 2.B.

**Qué descarta.** Mantener la cadena como patrón de aplicación por debajo de los agregados.

**Consecuencias.** Cumple el cierre propio de E9. Cómo se pasa del código de hoy a esto es de E11.

### DEC-09.5 — El espacio de trabajo tiene una parte del motor y otra de la tecnología *(por defecto)*

**Decisión.** La parte del motor se rehace entera al empezar cada intento, y el resto no se toca nunca. Dónde
está el espacio de trabajo de un ambiente, y dónde están sus registros, se lo dice al motor la invocación.

**Por qué.** Es la forma más simple de cumplir `DEC-06.19`: no hace falta distinguir archivo por archivo, sino
saber qué parte es del motor.

**Consecuencias.** Cierra la duda **#1** de IT-06.

### DEC-09.6 — Para decidir, las variables producidas cuentan *(por defecto)*

**Decisión.** Al decidir si un paso se re-ejecuta, sus variables incluyen las producidas por pasos
anteriores.

**Por qué.** Si un paso anterior produjo una etiqueta de imagen nueva, el paso que la despliega tiene que
re-ejecutarse. En Diagnóstico es al revés (`DEC-06.10`) porque allí se busca una causa, y una variable
producida es consecuencia; aquí se decide qué hacer, y ese cambio obliga a actuar.

### DEC-09.7 — Un paso deja un comienzo y un final, y solo un final exitoso evita re-ejecutarlo *(respuesta a `Q-09.1`)*

**Decisión.** Cada paso que se ejecuta deja un registro al empezar, antes de su primer comando, y otro al
terminar, exitoso o fallido. Un paso que no se re-ejecuta deja un registro con su razón y su evidencia, y
esa evidencia apunta a un final exitoso. Un paso solo puede no re-ejecutarse si su último registro es un
final exitoso, o una no re-ejecución que apunta a uno. Si es un comienzo sin final o un final fallido, se
re-ejecuta.

**Por qué.** Un paso que falló o que murió a mitad puede haber dejado el mundo a medio tocar, y comparar
sus recursos no lo detecta.

**Consecuencias.** El agregado Intento del Historial pasa de un registro por paso a un comienzo y, si
llega, un final. El Despliegue exige, en todos los pasos del pipeline, un final exitoso o una no
re-ejecución. Precisa *última vez de un paso* (`DEC-06.12`).

**Qué descarta.** Tomar como última vez el último final exitoso.

**Verificación.** Un paso cuyo último registro no es un final exitoso nunca recibe *no se re-ejecuta*.

### DEC-09.8 — El aislamiento es entre ambientes y de variables; los pasos de un ambiente comparten su espacio de trabajo *(por defecto)*

**Decisión.** *Aislamiento* es que un ambiente no pise a otro y que un paso solo vea las variables de su
ámbito. Los pasos de un mismo ambiente comparten su espacio de trabajo.

**Por qué.** `lenguaje.md` decía *«que un paso no alcance lo que no es suyo»*, y eso contradice
`DEC-06.17`: un paso les deja archivos a los siguientes en el espacio de trabajo de su ambiente.

---

## 5. Impacto en el modelo

| Documento | Qué cambió |
|---|---|
| `modelo/contextos/ejecucion.md` | **Nace**, y pasa a **vigente** al cerrar |
| `modelo/contextos/historial.md` · `lenguaje.md` · `contextos/ejecucion.md` | Comienzo y final por paso; solo un final exitoso evita re-ejecutar (`DEC-09.7`) |
| `modelo/lenguaje.md` | *Aislamiento* es entre ambientes y de variables (`DEC-09.8`) |
| `plan-ddd.md` | Tablero |

## 6. Dudas diferidas

*Ninguna.* Cómo se pasa del código de hoy a este modelo es de E11.
