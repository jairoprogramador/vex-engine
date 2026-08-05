package command_test

// Test de CARACTERIZACIÓN de CommandWorkdir.scope() (spec 00 §5.4).
//
// `scope()` es privado; se observa a través de `IsShared()`, que es su único
// consumidor.
//
// La primera fila es la que importa: `./terraform/shared` es el workdir REAL de
// los tres templates de producción, y hoy resuelve a ScopeEnvironment porque la
// regla mira el PRIMER segmento del path. El autor quiso ScopeShared. El ámbito
// compartido está muerto en producción (S-1).
//
// Este test afirma el comportamiento ACTUAL. La spec 13 hace explícito el
// ámbito compartido; cuando lo haga, esta tabla se pone en rojo y hay que
// reescribirla a propósito — que es exactamente lo que se busca.

import (
	"runtime"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/stretchr/testify/assert"
)

func TestCommandWorkdir_IsShared(t *testing.T) {
	cases := []struct {
		workdir  string
		isShared bool
		nota     string
		soloUnix bool
	}{
		{
			workdir:  "./terraform/shared",
			isShared: false,
			nota:     "S-1: workdir real de los templates; el autor quiso ScopeShared",
		},
		{
			workdir:  "shared/terraform",
			isShared: true,
			nota:     "único caso que hoy resuelve a ScopeShared",
		},
		{
			workdir:  "shared",
			isShared: true,
			nota:     "sin separador: el path entero es el primer segmento",
		},
		{
			workdir:  "",
			isShared: false,
			nota:     "workdir vacío: cortocircuito explícito a ScopeEnvironment",
		},
		{
			workdir:  "/shared/x",
			isShared: false,
			nota:     "path absoluto: el primer segmento es la cadena vacía",
		},
		{
			workdir:  "Shared/x",
			isShared: false,
			nota:     "la comparación es sensible a mayúsculas",
		},
		{
			workdir:  "./shared",
			isShared: false,
			nota:     "el primer segmento es '.', no 'shared'",
		},
		{
			workdir:  "shared-infra/x",
			isShared: false,
			nota:     "coincidencia exacta del segmento, no por prefijo",
		},
		{
			workdir:  "app/shared",
			isShared: false,
			nota:     "'shared' en cualquier posición que no sea la primera no cuenta",
		},
		{
			workdir:  `shared\terraform`,
			isShared: false,
			soloUnix: true,
			nota: "ToSlash sólo traduce '\\' en Windows; en Linux/macOS el path " +
				"entero es un único segmento y no casa",
		},
	}

	for _, tc := range cases {
		t.Run(tc.workdir, func(t *testing.T) {
			if tc.soloUnix && runtime.GOOS == "windows" {
				t.Skip("filepath.ToSlash traduce '\\' en Windows")
			}
			workdir := command.NewCommandWorkdir(tc.workdir)

			assert.Equal(t, tc.isShared, workdir.IsShared(), tc.nota)
			assert.Equal(t, tc.workdir, workdir.String(), "String devuelve el path sin normalizar")
		})
	}
}
