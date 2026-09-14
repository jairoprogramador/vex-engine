package command

// CommandWorkdir es el directorio donde corre un comando y la base contra la que
// se resuelven sus `templates`. Dos cosas, no tres.
//
// Aquí vivían `scope()`, `IsShared()`, `WorkdirScope`, `ScopeShared`,
// `ScopeEnvironment` y `SharedScopeName`: el ámbito del almacén se deducía del
// PRIMER segmento de esta ruta. Los retira la spec 13 §5.5, y no se conservan
// como azúcar de compatibilidad — dos mecanismos para lo mismo es exactamente
// cómo se llega a que uno esté muerto y nadie lo note, que es lo que pasó: los
// tres templates reales escriben `./terraform/shared`, cuyo primer segmento es
// `.`, así que el ámbito compartido nunca se activó.
//
// El ámbito lo declara ahora el step en su `config.yaml` (`step.Scope`), y con
// ello `shared` deja de ser una palabra reservada en el vocabulario del usuario:
// un directorio real llamado `shared` ya no colisiona con nada.
type CommandWorkdir struct {
	relativePath string
}

func NewCommandWorkdir(relativePath string) CommandWorkdir {
	return CommandWorkdir{relativePath: relativePath}
}

func (cw CommandWorkdir) String() string {
	return cw.relativePath
}
