# Lenguaje Ubicuo — Motor de Vex

## Diagnóstico *(Core)*

Explota Registro + Definición para dar información objetiva y accionable, antes y después de un intento.

- **Plan automático**: se ejecuta antes de cada intento, valida pipeline + variables, compara contra el último despliegue exitoso. La información debe ser siempre exacta, nunca inferencial.
- **Diagnóstico de errores**: clasifica si un fallo es del producto (código fuente) o del ambiente (pipeline, variables de pipeline).
    - Un error en las instrucciones del pipeline afecta a **todos los ambientes**.
    - Un error en variables de pipeline afecta a **un solo ambiente**, y más específico, a un solo paso.

---

## Registro de Despliegue *(Supporting — fuente de verdad)*

- **Intento**: ejecutar uno o varios pasos de un pipeline en un ambiente.
- **Despliegue**: intento **exitoso** de **todos** los pasos de un pipeline en un ambiente. Tiene `id de despliegue`.
    - Puede tener un despliegue padre
    - Cuando el despliegue padre es el ultimo despliegue no se considera un rollback, caso contrario es un rollback.
- **Lanzamiento**: se crea a partir de un despliegue en ambiente productivo. Tiene estrategias de lanzamiento. Por defecto, si no se especifica estrategia, se crea junto al despliegue en producción. Tiene `id de lanzamiento`.
- **Versión de producto**: etiqueta de negocio (ej. `v1.0.0`) asignada a un fingerprint **solo cuando ese fingerprint se convierte en lanzamiento**.
- **Logs**: registros de despliegues.
- **Rollback**: es un despliegue que se crea a partir de un despliegue anterior al ultimo. 

### Acciones de consulta (read-side, todas viven aquí):
- Listar/consultar intentos, despliegues, lanzamientos, versiones de producto por ambiente
- Listar logs de un intento / despliegue / lanzamiento por ambiente
- Listar variables de ejecución de un intento, despliegue o lanzamiento (se especifica el id de cualquiera de los tres)

---

## Orquestación de Pipeline *(Supporting)*

- **Intentar** (verbo): coordina el recorrido de pasos hasta el paso final indicado, ejecutando los anteriores según las reglas de re-ejecución.
- Evalúa las **reglas de re-ejecución** (definidas en Definición) contra el historial (consultando Registro) y los **recursos de ejecución** actuales.
- **Recursos de ejecución de un paso**: artefacto que Orquestación ensambla antes de invocar Ejecución de Step — junta definiciones del paso (Definición), variables de pipeline por ambiente (Resolución de Variables), código fuente (Suministro del Proyecto).
- Cada solicitud de intento se registra, asociando despliegue y lanzamiento si aplica (ambiente productivo).

---

## Ejecución de Step *(Supporting)*

- **Tarea**: acción a realizar, típicamente un comando.
- **Comando**: comando de Linux o herramienta de terceros (Docker, Terraform, etc).
- **Variables de ejecución**: resultado de una tarea, se transmiten a las tareas siguientes. Se generan aquí; su historial vive en Registro, su ámbito real en Resolución de Variables.
- Un paso no puede acceder a otro ámbito ni a otro espacio de trabajo que no sea el suyo propio.

---

## Simulación de Pipeline *(Supporting)*

- Mismo mecanismo de orquestar+ejecutar, pero sin invocar comandos reales.
- Genera valores de salida simulados usando la forma de salida esperada (regex) ya declarada en Definición.
- Nunca persiste en Registro ni en el ámbito real de variables — todo ocurre en memoria.
- Contexto aislado: solo depende de Definición de Pipeline.

---

## Definición de Pipeline *(Supporting)*

- **Pipeline**: un conjunto de pasos.
- **Paso**: un conjunto de tareas a realizar.
- **Variables de pipeline**: variables declaradas en el pipeline (diferentes por ambiente, separadas por paso).
- **Ambiente**: entorno de despliegue que tiene sus propias variables de pipeline.
- **Reglas de re-ejecución**: cada paso las define (por tiempo transcurrido o por cambios en recursos de ejecución).
- **Forma de salida esperada**: expresión regular que define qué shape debe tener la salida de un comando (usada tanto en ejecución real como en Simulación).
- Las instrucciones de despliegue son iguales en todos los ambientes; solo las variables de pipeline cambian por ambiente.

---

## Resolución de Variables *(Supporting)*

- **Ámbito**: espacio lógico y físico donde existen las variables
- Extrapola variables antes de ejecutar (validación) y durante la ejecución (runtime), acumulando entre pasos.
- Consume variables de pipeline (Definición) y variables de ejecución (Ejecución de Step).

---

## Espacio de Trabajo *(Supporting)*

- Copia física mutable del pipeline, por proyecto, donde se interpolan templates durante la ejecución (distinta del repo clonado, que es compartido/reusable).
- Ciclo de vida propio: crear, usar, limpiar — independiente del historial de ejecuciones.
- Es lo único que realmente se puede "limpiar" — no hay caché real que limpiar en ningún otro contexto.

---

## Suministro del Proyecto *(Generic)*

- Clona/symlink del repo del proyecto.
- Calcula el fingerprint del código fuente (firma que indica *si* algo cambió, no *qué* cambió).

---

## Suministro del Pipeline *(Generic)*

- Clona el repo del pipeline con un intervalo de refresco (es una configuración de frecuencia).
- Calcula el fingerprint del pipeline.

---

## Sincronización de Estado *(Generic)*

- Backend local o remoto para el estado del motor.
- Sin términos específicos identificados aún en el documento — pendiente de vocabulario propio si surge.

---

## Resolución final: "Ambiente" y "Store"

- Ambiente: no es un bounded context propio. Es solo un identificador/nombre declarado lógicamente en Definición de Pipeline, con su archivo de configuración físico dentro del pipeline.
- Store: nombre formal para "el lugar donde un paso persiste sus datos de ejecución" (con un directorio propio por paso dentro). Pertenece a Sincronización de Estado, no a Registro de Despliegue — Registro le pide a Sincronización que persista; Registro solo modela el concepto lógico (intento, despliegue, lanzamiento) sin saber dónde ni cómo se guarda físicamente.

## Estructura física de carpetas

Por cada {proyecto}/{ambiente}/ conviven dos sub-directorios, cada uno propiedad de un bounded context distinto:

{proyecto}/{ambiente}/
├── workspace/   → Espacio de Trabajo (copia mutable del pipeline, interpolación de templates)
├── store/       → Sincronización de Estado (datos de ejecución por paso, uno por sub-directorio)

Por cada {proyecto}/vars/ conviven un sub-directorios para cada ámbito, cada uno propiedad del bounded context 'Resolución de Variables':

{proyecto}/vars/
└── ámbito/      → Resolución de Variables