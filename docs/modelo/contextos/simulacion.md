# Simulación de Pipeline — modelo del contexto

> **Supporting.** Qué es, en `dominio.md`; su lenguaje, en `lenguaje.md`; su frontera, en
> `bounded-contexts.md`; sus relaciones, en `context-map.md` (filas #6, #9 y #11, y *Separate Ways* con
> Ejecución).
>
> **Vigente** desde el cierre de IT-10 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## Qué responde

**¿Funcionará este pipeline antes de publicarlo?** Recorre el flujo entero **sin efectos y sin guardar
nada**. Lo único que finge son los comandos. La comprobación y la interpolación se hacen de verdad.

---

## Escenarios

| # | Escenario | Qué pasa |
|---|---|---|
| **SIM-1** | *Simular un pipeline* | 1. Trae el pipeline, de una copia de trabajo o de un commit (`DEC-10.6`), y lo comprueba; si la comprobación falla, ese es el resultado. 2. **Para cada ambiente del pipeline**, recorre **todos** los pasos en orden: interpola de verdad, en una petición sin intento (`DEC-04.10`) y en un espacio temporal, y simula cada comando. Un comando simulado termina bien y devuelve una **salida simulada** para cada variable de salida, que cumple su expresión regular. 3. Entrega el **informe** |
| **SIM-2** | *Algo no se resuelve* | Si en un paso de un ambiente una variable no se puede interpolar, el informe lo dice y la simulación sigue con el siguiente ambiente |

---

## Modelo táctico

### Agregado: **Simulación**

Vive durante la invocación, igual que el Intento en curso de Ejecución (`DEC-10.4`).

| Invariante | Qué dice |
|---|---|
| sin efectos | no ejecuta ningún comando, no escribe en el Historial y no toca el espacio de trabajo de ningún ambiente |
| todo el recorrido | recorre todos los pasos de cada ambiente, sin decidir re-ejecuciones y sin leer el historial (`DEC-03.10`) |
| salidas con forma | toda salida simulada cumple la expresión regular de su variable de salida |
| sin valores | el informe dice qué se resolvió y qué no, **por nombre**, nunca con su valor |

### Value objects

| | Qué es |
|---|---|
| **salida simulada** | un valor que cumple la expresión regular de su variable de salida y que nadie produjo |
| **informe** | por ambiente y por paso: si pasó la comprobación, qué se interpoló y qué faltó |

### Servicio de dominio

**Fabricar una salida simulada**: a partir de la expresión regular de una variable de salida.

### Servicio de aplicación

**Simular**: recorre SIM-1.

### Puertos

| Puerto | Hacia |
|---|---|
| **pipelines** | Definición: el pipeline comprobado |
| **fuentes** | Suministro: el material que se simula |
| **variables** | Resolución: interpolar en una petición sin intento |
| **espacio temporal** | un lugar donde interpolar el material, que desaparece al terminar |

Eventos y repositorios: ninguno.

### Relación con Ejecución

**Separate Ways** (`DEC-04.6`): no se hablan y no comparten núcleo.
