# Diagnóstico — modelo del contexto

> **Core.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; su única relación, con el Historial, en `context-map.md` (fila #1).
>
> **Vigente** desde el cierre de IT-06 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

Ante un fallo, **dónde está la causa**: en el código del producto, en las instrucciones del pipeline
o en las variables del ambiente. Lo responde por **eliminación** sobre tres ejes y **solo con
hechos**. Cuando la eliminación deja dos candidatos, muestra los dos. Nunca infiere y nunca muestra
un valor.

---

## Escenarios

**Cómo se compara**, en todos los escenarios:

- **Lo que falla es siempre un intento**: uno fallido, o el de un despliegue que quien pregunta dice
  que falla. **La referencia es siempre un despliegue** (`DEC-06.7`).
- **Por defecto hay una referencia**: el último despliegue anterior en el mismo ambiente. El usuario
  puede elegir otra referencia (`DEC-06.6`). Si no hay ninguna anterior al intento, o la referencia es
  un despliegue del propio intento, el core dice que no hay historial previo (`DEC-06.21`).
- **La respuesta combina las comparaciones**: un eje que descarta cualquiera de ellas queda descartado
  (`DEC-06.11`).
- **Los ejes se comparan paso a paso, con los recursos con que se hizo de verdad cada paso**
  (`DEC-06.13`):
  - las instrucciones, con el hash de las instrucciones de cada paso, nunca con el del pipeline
    (`DEC-06.9`);
  - las variables, solo las declaradas y visibles desde el paso; las producidas van al sustento
    (`DEC-06.10`);
  - si un paso no se re-ejecutó, sus recursos son los del registro al que apunta su evidencia
    (`DEC-06.5`).
- **Qué pasos entran**: los que el intento no alcanzó quedan fuera. Un paso que está en un lado y no en
  el otro cuenta como cambio de instrucciones.

| # | Escenario | Quién pregunta | Qué se compara | Qué entrega |
|---|---|---|---|---|
| **ES-1** | *Ayer funcionó, hoy falla* | programador | el intento que falla contra el último despliegue anterior a él en **el mismo ambiente** | qué ejes cambiaron y cuáles no. Si las instrucciones no cambiaron, quedan dos candidatos, código y variables |
| **ES-2** | *Funcionó en staging, falla en producción* | programador | el intento que falla en producción contra un despliegue de **staging** que el usuario elige como referencia (ya no es referencia por defecto, `DEC-06.21`) | si también coinciden las instrucciones de cada paso, el código y las instrucciones quedan descartados. Queda **un** candidato: las variables. Las de ámbito compartido son las mismas en los dos ambientes, así que las que pueden diferir son las del ámbito de cada ambiente |
| **ES-1 + ES-2** | *El caso normal* | programador | las dos a la vez | ES-1 deja el código y las variables; ES-2 descarta el código. **Respuesta: las variables** |
| **ES-3** | *Falla ante el cliente* | dueño del negocio | entra por un lanzamiento, que lleva a su despliegue; el intento de ese despliegue es el que falla, y sigue como ES-1 y ES-2 | lo mismo que el caso al que lleva |
| **ES-4** | *Un paso no se re-ejecutó* | dentro de cualquier escenario | para ese paso, los recursos del registro al que apunta su evidencia, que puede venir de otro ambiente si el paso no se re-ejecuta entre ambientes | la comparación contra los recursos con los que ese paso se hizo de verdad |
| **ES-5** | *Nada cambió en ningún eje* | programador | como ES-1 y ES-2 | *«ninguno de los tres ejes cambió»*: la causa no está en ellos. Es un hecho, no un fallo del core. Si cambió alguna variable producida, aparece en el sustento |
| **ES-6** | *No hay contra qué comparar* | cualquiera | no existe ningún despliegue anterior al intento, o la referencia es un despliegue del propio intento | *«no hay historial previo al intento actual que se pretende diagnosticar»*, sin atribución |
| **ES-7** | *Intento sin desenlace o cancelado* | cualquiera | — | no se atribuye causa: una cancelación no es un fallo de nadie, y de un intento sin desenlace no se sabe cómo terminó |
| **ES-8** | *¿Cuántas veces se intentó?* | programador | el intento que falla y la referencia del mismo ambiente | la cantidad de intentos desde ese despliegue hasta el que falla, como número |

**El sustento**, en todos los escenarios, dice:

- qué ejes cambiaron y en qué pasos;
- qué variables declaradas, por nombre y nunca con su valor;
- qué variables producidas cambiaron;
- entre qué dos momentos: la fecha de la referencia y la del intento que falla. Nunca quién hizo un
  cambio (IT-07 `DEC-07.7`);
- qué pasos se compararon por su evidencia;
- las comparaciones que se hicieron, cada una con su referencia.

---

## Lo que Diagnóstico le pide al Historial

Es el efecto buscado por el plan: lo que el core necesita pasa a ser requisito de quien está arriba,
en vez de adivinarse (Customer–Supplier, `context-map.md` fila #1).

| Necesita | Para |
|---|---|
| los despliegues de cada ambiente, en orden temporal | encontrar las referencias por defecto |
| de cada intento y cada despliegue, por paso: el hash del código y el de las instrucciones del paso | los ejes código e instrucciones. El hash del pipeline no lo usa, porque mezcla los tres ejes (`DEC-06.9`) |
| de cada variable, por paso: su hash, comparable entre ambientes del mismo proyecto, y su origen, declarada o producida | el eje variables, sin ver el valor, y las producidas para el sustento (`DEC-06.10`) |
| de cada paso que no se re-ejecutó, el registro al que apunta su evidencia | ES-4 |
| qué pasos alcanzó un intento | dejar fuera los que no alcanzó |
| el estado de cada intento, o que está sin desenlace | ES-7 |
| la cantidad de intentos desde un despliegue hasta un intento posterior | ES-8 |
| de cada lanzamiento, su despliegue | ES-3 |

---

## Modelo táctico

### Value objects

| Value object | Qué es |
|---|---|
| **eje** | código · instrucciones · variables. Lista cerrada: no hay un cuarto (`DEC-01.2`) |
| **ejes de un paso** | para un paso, lo que identifica cada eje con los recursos con que se hizo de verdad: el hash del código, el de sus instrucciones y el de cada variable declarada que ve. **Solo hashes** |
| **referencia** | un despliegue, los ejes de cada uno de sus pasos y por qué se eligió: *mismo ambiente* o *elegida por el usuario* |
| **comparación** | el intento que falla contra una referencia: el estado de cada eje en cada paso, *cambió* o *no cambió* |
| **atribución** | el conjunto de ejes candidatos. **Sin orden**: ningún candidato va delante de otro |
| **sustento** | lo que se sabe de cada cambio (sección anterior) |
| **respuesta** | una de tres formas: **atribución**, con su sustento y sus comparaciones · **sin referencia** · **no se atribuye** |

### Servicios de dominio

| Servicio | Qué hace |
|---|---|
| **elegir las referencias** | dado el intento que falla, devuelve la referencia por defecto (`DEC-06.6`), o la que eligió el usuario; ninguna si es un despliegue del propio intento (`DEC-06.21`) |
| **eliminación** | dadas una o dos comparaciones, devuelve la atribución: un eje queda descartado si alguna comparación lo descarta (`DEC-06.11`) |

### Entidades, agregados y repositorios

**No tiene** (`DEC-06.14`). Diagnóstico no cambia ningún estado: compara lo que el Historial guardó, y la
misma pregunta sobre el mismo historial da siempre la misma respuesta. No hay ningún cambio que
proteger, así que no hay agregado ni repositorio (`guia-ddd.md` §10).

**Sus invariantes viven en los value objects y en los servicios, por construcción** (frente 2.B de
`DEC-01.10`):

| Invariante | Por qué no se puede romper |
|---|---|
| nunca muestra un valor | *ejes de un paso* solo contiene hashes |
| nunca dice «probablemente» | la atribución es un conjunto sin orden |
| nunca atribuye causa a un intento cancelado o sin desenlace | *respuesta* no admite esa combinación |
| la referencia es siempre un despliegue | *referencia* solo se construye a partir de un despliegue |
| una variable producida no es eje | *ejes de un paso* solo lleva variables declaradas |

### Eventos de dominio

Ninguno: nadie espera enterarse de que alguien preguntó.

### La respuesta no se guarda

Se puede volver a calcular: la misma pregunta sobre el mismo historial da la misma respuesta, y el
historial solo crece (`DEC-06.15`).

### Factoría

**El ACL hacia el Historial** construye *ejes de un paso* y *referencia* a partir de registros
(`guia-ddd.md` §11). Es donde se sigue la evidencia de un paso que no se re-ejecutó y donde se separan
las variables declaradas de las producidas.

### Puerto hacia el Historial

Lo declara el dominio de Diagnóstico y lo implementa el ACL. Pide lo que dice la tabla *Lo que
Diagnóstico le pide al Historial*, dicho en el lenguaje del core.

### Servicio de aplicación: *preguntar la causa*

1. Determina el intento que falla: el que se indica, o el de un lanzamiento. Si no se indica ninguno, el
   último intento del ambiente (`DEC-06.16`).
2. Si ese intento está cancelado o sin desenlace, responde **no se atribuye**.
3. Elige la referencia. Si no hay ninguna anterior al intento, responde **sin referencia**, con el mensaje
   *«no hay historial previo al intento actual que se pretende diagnosticar»*.
4. Pide los ejes de cada paso, a través del ACL.
5. Compara, elimina y arma el sustento.
6. Devuelve la **respuesta**.

### Lo que publica

La operación **preguntar la causa**, que recibe un intento o un lanzamiento y, si se quiere, una
referencia elegida, y devuelve la **respuesta**. Es lo que el borde expone al CLI y al portal
(`arquitectura.md`).

