// Package dominio contiene el modelo del contexto: sus conceptos, sus reglas y los puertos que necesita.
// Solo usa su propio dominio y la biblioteca estándar.
//
// Contexto: Simulación de Pipeline: recorrer un pipeline sin efectos, para saber si funciona antes de publicarlo.
// Modelo: docs/modelo/contextos/simulacion.md · Capas: docs/modelo/arquitectura.md.
//
// # Invariantes del modelo
//
// Este paquete no tiene agregado con estado propio: a diferencia de IntentoEnCurso en Ejecución, la
// invocación en curso vive fuera de él — en el id de EspacioTemporal y en el mapa en memoria de Resolución
// que ese id indexa (DEC-04.10). Lo que este paquete protege son dos invariantes sin estado:
//
//   - «salidas con forma»: FabricarSalidaSimulada siempre devuelve un valor que cumple la expresión regular
//     recibida — nunca un valor arbitrario que el llamador deba validar después.
//   - el ámbito de un paso (AmbitoDelPaso) se deriva solo de si el paso es compartido y de qué ambiente se
//     está recorriendo, nunca del nombre del paso ni de ninguna decisión de re-ejecución: Simulación no
//     decide re-ejecuciones (DEC-03.10).
//
// # Cómo agregar una interpolación nueva
//
// Si un nuevo tipo de material necesita interpolarse (hoy solo Comando.Linea y los Fichero marcados
// Plantilla), añadir el campo a Pipeline/Paso y consumirlo desde aplicacion — este paquete no decide qué se
// interpola, solo qué forma tiene el pipeline y cómo se fabrica una salida simulada.
package dominio
