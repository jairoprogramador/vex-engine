// Package infraestructura contiene los adaptadores del contexto: persistencia, traducciones (ACL) hacia los
// contextos de arriba y lo que se compra.
//
// Contexto: Simulación de Pipeline: recorrer un pipeline sin efectos, para saber si funciona antes de publicarlo.
// Modelo: docs/modelo/contextos/simulacion.md · Capas: docs/modelo/arquitectura.md.
//
// Tres adaptadores, uno por puerto de dominio: Pipelines sobre definicion/publicado.ParaSimulacion, Variables
// sobre resolucion/publicado.ParaSimulacion, y EspacioTemporal, sin contraparte publicada por otro contexto —
// un id de invocación nuevo por llamada (uuid.NewV7), mismo patrón que
// historial/infraestructura.IdentidadesUUID. Sin adaptador hacia Suministro: RD-09 §9 explica por qué (el
// puerto ParaSimulacion de Resolución no tiene dónde recibir un valor traído de Suministro).
package infraestructura
