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

*Vacío.*
