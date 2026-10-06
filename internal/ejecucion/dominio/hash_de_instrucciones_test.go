package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func comandoDePrueba(t *testing.T, linea string, plantillas []string) dominio.ComandoDeclarado {
	t.Helper()
	c, err := dominio.NuevoComandoDeclarado("build", linea, "", plantillas, nil, nil)
	require.NoError(t, err)
	return c
}

func ficheroDePrueba(t *testing.T, ruta, contenido string, plantilla bool) dominio.FicheroDeclarado {
	t.Helper()
	f, err := dominio.NuevoFicheroDeclarado(ruta, contenido, false, "", plantilla)
	require.NoError(t, err)
	return f
}

func TestElHashDeInstruccionesEsElMismoParaLasMismasInstrucciones(t *testing.T) {
	comandos := []dominio.ComandoDeclarado{comandoDePrueba(t, "echo hola", nil)}
	material := []dominio.FicheroDeclarado{ficheroDePrueba(t, "a.txt", "contenido", false)}

	a := dominio.CalcularHashDeInstrucciones(comandos, material)
	b := dominio.CalcularHashDeInstrucciones(comandos, material)

	require.Equal(t, a, b)
	require.NotEmpty(t, a.String())
}

func TestElHashDeInstruccionesNoDependeDelOrdenDelMaterial(t *testing.T) {
	comandos := []dominio.ComandoDeclarado{comandoDePrueba(t, "echo hola", nil)}
	a := dominio.CalcularHashDeInstrucciones(comandos, []dominio.FicheroDeclarado{
		ficheroDePrueba(t, "b.txt", "2", false), ficheroDePrueba(t, "a.txt", "1", false),
	})
	b := dominio.CalcularHashDeInstrucciones(comandos, []dominio.FicheroDeclarado{
		ficheroDePrueba(t, "a.txt", "1", false), ficheroDePrueba(t, "b.txt", "2", false),
	})

	require.Equal(t, a, b)
}

func TestElHashDeInstruccionesCambiaSiCambiaUnaPlantilla(t *testing.T) {
	comandos := []dominio.ComandoDeclarado{comandoDePrueba(t, "echo hola", nil)}
	a := dominio.CalcularHashDeInstrucciones(comandos, []dominio.FicheroDeclarado{ficheroDePrueba(t, "a.txt", "uno", true)})
	b := dominio.CalcularHashDeInstrucciones(comandos, []dominio.FicheroDeclarado{ficheroDePrueba(t, "a.txt", "dos", true)})

	require.NotEqual(t, a, b)
}

func TestElHashDeInstruccionesCambiaSiCambiaElOrdenDeLosComandos(t *testing.T) {
	c1 := comandoDePrueba(t, "echo uno", nil)
	c2 := comandoDePrueba(t, "echo dos", nil)

	a := dominio.CalcularHashDeInstrucciones([]dominio.ComandoDeclarado{c1, c2}, nil)
	b := dominio.CalcularHashDeInstrucciones([]dominio.ComandoDeclarado{c2, c1}, nil)

	require.NotEqual(t, a, b)
}
