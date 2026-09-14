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

*Vacío.*
