package notify

import "testing"

// Un panic en el camino del log es un panic en el camino feliz (spec 07 §5.5).
//
// La abreviatura del id hacía `id[:4] + "..." + id[len(id)-4:]` sin mirar la
// longitud: con menos de 4 caracteres panicaba, y entre 4 y 7 solapaba las dos
// mitades. El id de ejecución lo fija quien invoca (`--execution-id`), así que
// no hay garantía de que sea un UUID.
func TestStdoutLogObserver_NoPanicaConIdsCortos(t *testing.T) {
	observer := NewStdoutLogObserver()

	for _, id := range []string{"", "a", "abc", "abcd", "abcdefg"} {
		t.Run("id "+id, func(t *testing.T) {
			observer.Notify(id, "una línea")
		})
	}
}

func TestStdoutLogObserver_AbreviaSoloCuandoDiceAlgo(t *testing.T) {
	casos := map[string]string{
		"":                                     "",
		"abc":                                  "abc",
		"abcdefg":                              "abcdefg",
		"abcdefgh":                             "abcd...efgh",
		"11111111-2222-3333-4444-555555555555": "1111...5555",
	}

	for id, esperado := range casos {
		if got := abreviar(id); got != esperado {
			t.Errorf("abreviar(%q) = %q; se esperaba %q", id, got, esperado)
		}
	}
}
