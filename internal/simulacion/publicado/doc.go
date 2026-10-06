// Package publicado contiene lo que el contexto ofrece a otros, en su lenguaje publicado.
// Solo usa la biblioteca estándar.
//
// Contexto: Simulación de Pipeline: recorrer un pipeline sin efectos, para saber si funciona antes de publicarlo.
// Modelo: docs/modelo/contextos/simulacion.md · Capas: docs/modelo/arquitectura.md.
//
// Una sola operación, Simular, y un solo resultado, Resultado — sin eventos ni repositorios (simulacion.md,
// «Puertos»: «Eventos y repositorios: ninguno»). Resultado nunca lleva valores, solo nombres: qué se resolvió y
// qué no.
package publicado
