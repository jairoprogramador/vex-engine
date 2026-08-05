package status

type Action int

const (
	ActionRun Action = iota
	ActionSkip

	// ActionUndetermined es «no pude averiguar si cambió»: un fallo de
	// infraestructura, no una observación (spec 09 §5.3).
	//
	// Es un estado del enum y no un booleano paralelo junto a la decisión a
	// propósito: un booleano invita a que alguien lo ignore, un estado obliga a
	// tratarlo en cada `switch`. Y es distinto de `Run` aunque los dos terminen
	// ejecutando, porque «sé que cambió» y «no pude averiguarlo» son hechos
	// distintos, y hasta ahora el motor solo sabía decir el primero.
	ActionUndetermined
)

func (a Action) String() string {
	return []string{"run", "skip", "undetermined"}[a]
}
