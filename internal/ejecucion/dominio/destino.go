package dominio

// Destino es el despliegue al que vuelve un rollback (EJ-2): su ambiente y los dos commits — el del código del
// proyecto y el del pipeline — con los que se hizo, de las dos fuentes de distinto dueño (docs/modelo/
// dominio.md, «Suministro de Fuentes»). Un rollback repite el pipeline entero contra ese mismo par de commits,
// y cierra con este despliegue como padre del nuevo.
type Destino struct {
	despliegue        string
	ambiente          string
	fuenteDelProyecto string
	commitDelProyecto string
	fuenteDelPipeline string
	commitDelPipeline string
}

// ComprobarDespliegue rechaza un despliegue vacío: sin él no hay a qué volver, y no hace falta preguntar al
// Historial para saberlo. Que el despliegue no exista es otra cosa, y es del Historial.
func ComprobarDespliegue(despliegue string) error {
	if despliegue == "" {
		return invalidoElCampo("Despliegue", despliegue, "un rollback necesita el despliegue al que volver")
	}
	return nil
}

func NuevoDestino(
	despliegue, ambiente, fuenteDelProyecto, commitDelProyecto, fuenteDelPipeline, commitDelPipeline string,
) (Destino, error) {
	switch {
	case despliegue == "":
		return Destino{}, invalidoElCampo("Despliegue", despliegue, "un destino no puede tener el despliegue vacío")
	case ambiente == "":
		return Destino{}, invalidoElCampo("Ambiente", ambiente, "%q: un destino no puede tener el ambiente vacío", despliegue)
	case fuenteDelProyecto == "" || commitDelProyecto == "":
		return Destino{}, invalido("%q: un destino necesita la fuente y el commit del proyecto", despliegue)
	case fuenteDelPipeline == "" || commitDelPipeline == "":
		return Destino{}, invalido("%q: un destino necesita la fuente y el commit del pipeline", despliegue)
	}
	return Destino{
		despliegue: despliegue, ambiente: ambiente,
		fuenteDelProyecto: fuenteDelProyecto, commitDelProyecto: commitDelProyecto,
		fuenteDelPipeline: fuenteDelPipeline, commitDelPipeline: commitDelPipeline,
	}, nil
}

func (d Destino) Despliegue() string { return d.despliegue }

func (d Destino) Ambiente() string { return d.ambiente }

func (d Destino) FuenteDelProyecto() string { return d.fuenteDelProyecto }

func (d Destino) CommitDelProyecto() string { return d.commitDelProyecto }

func (d Destino) FuenteDelPipeline() string { return d.fuenteDelPipeline }

func (d Destino) CommitDelPipeline() string { return d.commitDelPipeline }
