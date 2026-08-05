package step

// SkipReason es la razón por la que un step no se ejecutó. Existe porque
// `skipped` sin razón no es un hecho consultable: la disciplina del registro es
// guardar hechos, nunca conclusiones (spec 04 §5.3).
type SkipReason string

const (
	// SkipReasonNone es el step que no se saltó.
	SkipReasonNone SkipReason = ""

	// SkipReasonNoCommands es el step cuyo `commands.yaml` no declara ningún
	// comando (o no existe).
	SkipReasonNoCommands SkipReason = "no_commands"
)

func (r SkipReason) String() string {
	return string(r)
}
