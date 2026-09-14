package deployment

import "fmt"

// Lineage es la historia de un ambiente: la cadena de despliegues de un proyecto
// sobre un destino, y su cabeza.
//
// # Por qué el linaje es (sujeto, destino) y no lleva la operación
//
// La pregunta que `deployment_id` responde es «¿en qué posición de la historia de
// ESTE AMBIENTE cae esto?». Un `supply` y un `deploy` sobre `prod` son dos cosas
// que le pasaron a `prod`, en un orden, y partir la historia por operación
// produciría dos líneas de tiempo paralelas sobre el mismo ambiente real —donde
// los efectos sí se acumulan en una sola—.
//
// # Es inmutable, como todo lo de este paquete
//
// `Advance` no muta: devuelve el linaje nuevo y el identificador derivado. Quien
// persiste la cabeza (spec 18) escribe el resultado; nadie reescribe un linaje
// en memoria y se olvida de guardarlo.
//
// # No define su propia serialización
//
// La dirección donde vive un linaje la compone su almacén, igual que
// `state.Key` deja la ruta a `FileRecordsRepository`. Aquí se exponen sus dos
// componentes y nada más: inventar aquí un formato con separador obligaría a
// inventar también su escape, y la ruta ya tiene quien la resuelva.
type Lineage struct {
	subject     Subject
	destination Destination
	head        DeploymentID
}

// NewLineage abre —o rehidrata— el linaje de un ambiente.
//
// La cabeza cero es legítima y es el caso normal la primera vez: significa «este
// ambiente no tiene historia todavía», y de ahí sale un primer `deployment_id`
// sin padre.
func NewLineage(subject Subject, destination Destination, head DeploymentID) (Lineage, error) {
	if subject.IsZero() {
		return Lineage{}, fmt.Errorf("deployment: el linaje no tiene sujeto")
	}
	if destination.IsZero() {
		return Lineage{}, fmt.Errorf("deployment: el linaje no tiene destino")
	}
	return Lineage{subject: subject, destination: destination, head: head}, nil
}

// LineageOf abre el linaje al que pertenece un contenido. Es el uso normal: el
// linaje se deduce del propio objeto, no se elige aparte.
func LineageOf(content Content, head DeploymentID) (Lineage, error) {
	return NewLineage(content.Subject(), content.Destination(), head)
}

func (l Lineage) Subject() Subject         { return l.subject }
func (l Lineage) Destination() Destination { return l.destination }

// Head es el último despliegue registrado, o el valor cero si no hay ninguno.
func (l Lineage) Head() DeploymentID { return l.head }

// IsEmpty dice que este ambiente no tiene historia todavía.
func (l Lineage) IsEmpty() bool { return l.head.IsZero() }

// Advance deriva la posición de un contenido nuevo y devuelve el linaje que
// resulta de encadenarlo.
//
// **El mismo contenido dos veces produce dos `deployment_id` distintos**, porque
// el segundo cuelga del primero. No es un defecto: es exactamente la diferencia
// entre las dos identidades. `content_id` dice qué se pretende hacer y se repite;
// `deployment_id` dice en qué posición cae y no se repite jamás. De ahí la regla
// que gobierna el resto del motor: la decisión de saltar un step NO puede
// consultar `deployment_id`.
func (l Lineage) Advance(content ContentID) (Lineage, DeploymentID, error) {
	if l.subject.IsZero() {
		return Lineage{}, DeploymentID{}, fmt.Errorf("deployment: linaje sin abrir")
	}

	next, err := DeploymentIDOf(content, l.head)
	if err != nil {
		return Lineage{}, DeploymentID{}, err
	}
	return Lineage{subject: l.subject, destination: l.destination, head: next}, next, nil
}

// IsZero indica que no hay linaje.
func (l Lineage) IsZero() bool { return l.subject.IsZero() || l.destination.IsZero() }

// Equals compara la POSICIÓN del linaje, cabeza incluida: dos linajes del mismo
// ambiente con cabezas distintas son dos momentos distintos de la misma historia.
func (l Lineage) Equals(other Lineage) bool {
	return l.subject.Equals(other.subject) &&
		l.destination.Equals(other.destination) &&
		l.head.Equals(other.head)
}
