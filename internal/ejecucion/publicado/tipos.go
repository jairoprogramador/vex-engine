package publicado

// Metadatos son los datos de la variable estándar que solo puede dar quien invoca — nunca el motor: quién es
// el proyecto, su organización y su equipo, y en qué ambiente se intenta (RD-04 §9, hallazgo 1).
type Metadatos struct {
	ProjectId           string
	ProjectName         string
	ProjectOrganization string
	ProjectTeam         string
}

// PeticionDeIntento es lo que hace falta para intentar hasta un paso en un ambiente (EJ-1). Las dos fuentes son
// de distinto dueño (docs/modelo/dominio.md, «Suministro de Fuentes»): la del proyecto y la del pipeline. Un
// commit vacío es "de hoy"; CopiaDeTrabajo, si viene, tiene prioridad sobre la fuente y el commit del proyecto,
// y ese intento nunca llega a despliegue (DEC-10.7).
type PeticionDeIntento struct {
	Version     string // DEC-05.6
	Ambiente    string
	Solicitante string

	FuenteDelProyecto string
	CommitDelProyecto string
	CopiaDeTrabajo    string

	FuenteDelPipeline string
	CommitDelPipeline string

	HastaPaso string

	// DirectorioDeEspacioDeTrabajo y DirectorioDelAlmacen los da la invocación (DEC-06.18): dónde está el
	// espacio de trabajo del ambiente y cómo llegar al almacén del Historial.
	DirectorioDeEspacioDeTrabajo string
	DirectorioDelAlmacen         string

	Metadatos Metadatos
}

// PeticionDeRollback es lo que hace falta para volver a un despliegue (EJ-2). El ambiente y las dos fuentes con
// sus commits los da el propio despliegue destino (DEC-03.9): quien invoca no los repite.
type PeticionDeRollback struct {
	Version     string
	Despliegue  string
	Solicitante string

	DirectorioDeEspacioDeTrabajo string
	DirectorioDelAlmacen         string

	Metadatos Metadatos
}

// Resultado es lo que deja un intento: su identidad, cómo terminó y el despliegue al que llegó — vacío si no
// llegó a ninguno (DEC-10.7, o un intento fallido o cancelado).
type Resultado struct {
	Intento    string
	Estado     string
	Despliegue string
	Detalle    Detalle
}

// Detalle cuenta cómo fue el intento: cuánto tardó y qué pasó con cada paso que se llegó a dar. No se guarda —
// se arma con lo que el intento ya sabe al recorrerse.
type Detalle struct {
	Tiempo string
	Pasos  []PasoDelDetalle
}

// PasoDelDetalle es un paso dado y cómo terminó: "ejecutado", "precargado" (no se reejecutó), "fallido" o
// "cancelado".
type PasoDelDetalle struct {
	Nombre string
	Estado string
}
