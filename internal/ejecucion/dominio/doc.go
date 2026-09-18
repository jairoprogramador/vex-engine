// Package dominio contiene el modelo del contexto: sus conceptos, sus reglas y los puertos que necesita.
// Solo usa su propio dominio y la biblioteca estándar.
//
// Contexto: Ejecución de Pipeline: llevar a cabo un intento haciendo solo el trabajo que hace falta.
// Modelo: docs/modelo/contextos/ejecucion.md · Capas: docs/modelo/arquitectura.md.
//
// # Invariantes del modelo
//
// El agregado IntentoEnCurso vive durante la invocación (DEC-09.2) y hace cumplir, por construcción, las cinco
// invariantes del modelo: orden (SiguientePaso solo da el paso que toca), nada sin escribir (el agregado no
// hace I/O — es la aplicación quien solo llama a Completar después de confirmar el registro en el Historial),
// un fallo cierra, la cancelación gana (incluso a un fallo que ella misma provocó) y un solo desenlace. No
// tiene protección de concurrencia: un intento no lo comparte más de un hilo.
//
// DecidirPaso es un servicio de dominio sin efectos (DEC-09.3): no lee nada, no calcula ningún hash y no
// escribe nada — recibe la regla, los recursos de ahora y la última vez, todos ya resueltos por quien llama.
// Cada hash tiene su dueño fuera de este paquete: el del código, Suministro; el de las instrucciones, la
// infraestructura de este contexto; si cambiaron las variables, Resolución (DEC-09.6, las producidas por pasos
// anteriores sí cuentan aquí — al revés que en Diagnóstico). Un paso solo puede no re-ejecutarse si su última
// vez es válida: un final exitoso, o una no-reejecución, que nunca se escribe si no apunta a uno (DEC-09.7). Un
// conjunto de reglas vacío re-ejecuta siempre — nunca degenera en "no re-ejecutar nunca" por accidente.
//
// El hash de instrucciones (DEC-08.7) cubre los comandos de un paso y el material de su directorio, y nunca su
// configuración (reglas, edad máxima, ámbito): ese reparto lo hace la infraestructura al traducir, no este
// paquete, que solo compara dos HashDeInstrucciones ya calculados.
package dominio
