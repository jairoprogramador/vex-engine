package dominio

// ParametrosDecisionDeLanzamiento es lo que hace falta para decidir si Lanzamiento actúa en nombre del
// actor ausente (LAN-1): si el dueño del negocio se reservó el ambiente, y cuál fue su último lanzamiento,
// si tiene alguno.
type ParametrosDecisionDeLanzamiento struct {
	Reservado                      bool
	HayUltimoLanzamiento           bool
	DespliegueDelUltimoLanzamiento IdDespliegue
	DespliegueNuevo                IdDespliegue
}

// DecidirSiLanzarEnNombreDelActorAusente: en un ambiente reservado no se lanza en su nombre nunca — decide
// el dueño del negocio. En uno no reservado, se lanza el último despliegue si todavía no está lanzado
// (IT-04 DEC-04.8, IT-05 DEC-05.4).
func DecidirSiLanzarEnNombreDelActorAusente(p ParametrosDecisionDeLanzamiento) bool {
	if p.Reservado {
		return false
	}
	return !p.HayUltimoLanzamiento || p.DespliegueDelUltimoLanzamiento != p.DespliegueNuevo
}
