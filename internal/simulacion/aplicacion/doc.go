// Package aplicacion contiene los casos de uso del contexto: lo que hace cuando se le pide algo.
//
// Contexto: Simulación de Pipeline: recorrer un pipeline sin efectos, para saber si funciona antes de publicarlo.
// Modelo: docs/modelo/contextos/simulacion.md · Capas: docs/modelo/arquitectura.md.
//
// # Cómo se recorre SIM-1
//
// Simular trae el pipeline (§1); si la comprobación falla, ese es el resultado entero — Ambientes queda
// vacío, ningún ambiente se recorre (Simular en simular.go). Si pasa, recorrerAmbientes (bucle.go) abre un id
// de EspacioTemporal por ambiente — no uno por llamada a Simular: los literales del pipeline se repiten con
// distinto valor entre ambientes y Resolución no distingue el ámbito al declarar por nombre, así que
// compartir un id filtraría en silencio el valor de un ambiente a los demás (RD-09 §9) — y lo cierra siempre,
// con éxito o al abandonarlo. simularPaso (simular_paso.go) interpola las plantillas y las líneas de comando
// de un paso, acumulando en Faltante todos los nombres que no se resolvieron antes de devolver el control:
// quien llama (recorrerAmbiente) abandona el resto del ambiente en cuanto ve Faltante no vacío y sigue con el
// siguiente (SIM-2).
package aplicacion
