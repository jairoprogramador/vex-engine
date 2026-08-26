package dto

// RollbackInput nombra la ejecución pasada a la que se vuelve (spec 28 §5.5).
//
// # Es OPCIONAL, y por eso `schema_version` no sube
//
// Un campo que los clientes viejos no envían no rompe el contrato: ausente ⇒
// ejecución normal, presente ⇒ el motor resuelve el ancla y rechaza la ejecución
// antes del primer step si el destino no existe o no sirve.
//
// La objeción que la spec 16 le pone —`supportedSchemaVersion` se compara por
// IGUALDAD, así que un motor v2 que no conociera el campo lo descartaría en
// silencio y el usuario vería un despliegue normal donde pidió volver atrás—
// está contestada por el otro lado: **el motor lo confirma en el registro**. El
// `attempt_started` de esta ejecución lleva `rollback_to`, así que «esto fue una
// vuelta atrás» es auditable desde fuera de la máquina, que es exactamente lo
// que la spec 21 exige de este campo (recuadro). No entra en el objeto de
// despliegue a propósito: eso cambiaría el `content_id`, y que R y E compartan
// `content_id` es la mitad del diseño.
//
// # Y es un par, no un identificador suelto
//
// `{deployment_id, attempt}` es exactamente la firma de
// `record.ResultProjection.Attempt`, que es de donde sale el ancla. Un
// `deployment_id` mal formado o un `attempt: 0` fallan EN EL BORDE —los dos son
// value objects— en vez de convertirse en una consulta sin sujeto.
type RollbackInput struct {
	DeploymentID string `json:"deployment_id"`
	Attempt      int    `json:"attempt"`
}
