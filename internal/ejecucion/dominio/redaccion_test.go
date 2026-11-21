package dominio_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// jwtDePrueba tiene la forma de un JWT de verdad (cabecera, cuerpo y firma en base64url, con - y _), pero no es
// de nadie.
const jwtDePrueba = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6Ii1LSTNROW5OUjdiUm9meG1lWm9YcWJIWkdldyJ9" +
	".eyJhdWQiOiJhcGk6Ly9BenVyZUFEVG9rZW5FeGNoYW5nZSIsImlzcyI6Imh0dHBzOi8vdmV4LmludmFsaWQiLCJzdWIiOiJwYWdvcy1hcGkifQ" +
	".dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func TestRedactarJWT(t *testing.T) {
	casos := []struct {
		nombre, texto, esperado string
	}{
		{"un JWT en medio de una línea", "token " + jwtDePrueba + " emitido\n", "token [REDACTADO] emitido\n"},
		{"dos JWT en el mismo texto", "a=" + jwtDePrueba + "\nb=" + jwtDePrueba + "\n", "a=[REDACTADO]\nb=[REDACTADO]\n"},
		{"dos JWT en la misma línea", jwtDePrueba + " " + jwtDePrueba, "[REDACTADO] [REDACTADO]"},
		{"un JWT al final, sin salto de línea", "token=" + jwtDePrueba, "token=[REDACTADO]"},
		{"todo el texto es un JWT", jwtDePrueba, "[REDACTADO]"},
		{"en una cabecera Authorization", "> Authorization: Bearer " + jwtDePrueba + "\r\n", "> Authorization: Bearer [REDACTADO]\r\n"},
		{"entre comillas, en un JSON", `{"access_token":"` + jwtDePrueba + `","expires_in":600}`, `{"access_token":"[REDACTADO]","expires_in":600}`},
		{"en la traza de set -x", "+ export TOKEN=" + jwtDePrueba + "\n", "+ export TOKEN=[REDACTADO]\n"},
		{"en una URL", "https://x.invalid/cb?id_token=" + jwtDePrueba + "&state=1", "https://x.invalid/cb?id_token=[REDACTADO]&state=1"},
		{"pegado a un código de color del terminal", "\x1b[32m" + jwtDePrueba + "\x1b[0m", "\x1b[32m[REDACTADO]\x1b[0m"},
		{"pegado a lo que tenga delante", "%3D" + jwtDePrueba, "%3D[REDACTADO]"},
		{"uno mínimo, el del plan", "echo eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ4In0.c2ln", "echo [REDACTADO]"},
		{"un JWE de cinco partes pierde las tres primeras, que bastan para inutilizarlo",
			"eyJhbGciOiJSU0EtT0FFUCJ9.clave.iv.cifrado.etiqueta", "[REDACTADO].cifrado.etiqueta"},

		{"texto vacío", "", ""},
		{"texto sin nada que redactar", "Apply complete! Resources: 3 added.\n", "Apply complete! Resources: 3 added.\n"},
		{"eyJ sin ningún punto no se toca", "eyJhbGciOiJSUzI1NiJ9 no es un token\n", "eyJhbGciOiJSUzI1NiJ9 no es un token\n"},
		{"eyJhbGciOi suelto no se toca", "busca eyJhbGciOi en el log", "busca eyJhbGciOi en el log"},
		{"eyJ con un solo punto no se toca", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ4In0", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ4In0"},
		{"un segmento vacío no es un JWT", "eyJhbGciOiJub25lIn0..firma", "eyJhbGciOiJub25lIn0..firma"},
		{"tres segmentos que no empiezan por eyJ no se tocan", "registro.azurecr.io/pagos-api:v1.0.0", "registro.azurecr.io/pagos-api:v1.0.0"},
		{"una versión o una IP no se tocan", "terraform 1.9.5 en 10.0.0.1", "terraform 1.9.5 en 10.0.0.1"},
		{"las mayúsculas importan", "EYJhbGciOiJSUzI1NiJ9.eyjzdwi.c2ln", "EYJhbGciOiJSUzI1NiJ9.eyjzdwi.c2ln"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			require.Equal(t, c.esperado, dominio.RedactarJWT(c.texto))
		})
	}
}

func TestRedactarJWT_NoDejaNingunaParteDelToken(t *testing.T) {
	redactado := dominio.RedactarJWT("Authorization: Bearer " + jwtDePrueba + "\n")

	for _, parte := range strings.Split(jwtDePrueba, ".") {
		require.NotContains(t, redactado, parte, "ni la cabecera, ni el cuerpo, ni la firma")
	}
	require.NotContains(t, redactado, "eyJ")
}

func TestRedactarJWT_EsIdempotente(t *testing.T) {
	texto := "a " + jwtDePrueba + " b eyJsuelto c " + dominio.Redactado + " " + jwtDePrueba
	una := dominio.RedactarJWT(texto)

	require.Equal(t, una, dominio.RedactarJWT(una))
	require.Equal(t, dominio.Redactado, dominio.RedactarJWT(dominio.Redactado), "la marca no tiene forma de JWT")
}

// Los dos límites conocidos de la forma (AJ-21). Estas pruebas no dicen que esté bien: dicen qué pasa, para que
// cambiar la expresión sea una decisión y no un accidente.
func TestRedactarJWT_Limites(t *testing.T) {
	t.Run("tapa de más una palabra que contiene eyJ seguida de dos puntos", func(t *testing.T) {
		require.Equal(t, "k[REDACTADO]()", dominio.RedactarJWT("keyJoin.tabla.columna()"),
			"es el precio de no dejar pasar un token pegado a otra cosa")
	})

	t.Run("un token partido por un salto de línea después de su segundo punto pierde lo que importa", func(t *testing.T) {
		cabecera, cuerpo, firma := partes(t)
		redactado := dominio.RedactarJWT(cabecera + "." + cuerpo + "." + firma[:10] + "\n" + firma[10:])

		require.NotContains(t, redactado, cabecera)
		require.NotContains(t, redactado, cuerpo)
		require.Equal(t, "[REDACTADO]\n"+firma[10:], redactado, "queda el final de la firma, que solo no sirve")
	})

	t.Run("un token partido por un salto de línea antes de su segundo punto NO se detecta", func(t *testing.T) {
		cabecera, cuerpo, firma := partes(t)
		partido := cabecera + "." + cuerpo[:20] + "\n" + cuerpo[20:] + "." + firma

		require.Equal(t, partido, dominio.RedactarJWT(partido),
			"ninguna de las dos líneas tiene la forma completa: es el límite de mejor esfuerzo, como base64")
	})

	t.Run("el token codificado de otra forma NO se detecta", func(t *testing.T) {
		enBase64 := "ZXlKaGJHY2lPaUpTVXpJMU5pSjkuZXlKemRXSWlPaUo0SW4wLmMybG4="

		require.Equal(t, enBase64, dominio.RedactarJWT(enBase64))
	})
}

func partes(t *testing.T) (cabecera, cuerpo, firma string) {
	t.Helper()
	p := strings.Split(jwtDePrueba, ".")
	require.Len(t, p, 3)
	return p[0], p[1], p[2]
}

func TestRedactarJWT_UnaSalidaGrandeConMuchosCandidatosTerminaPronto(t *testing.T) {
	// 5 MiB de lo peor para la expresión: candidatos que empiezan como un token y no lo son, mezclados con
	// tokens de verdad.
	linea := "eyJ" + strings.Repeat("a", 60) + ".eyJ" + strings.Repeat("b", 60) + " eyJ.eyJ. " + jwtDePrueba + "\n"
	texto := strings.Repeat(linea, 5<<20/len(linea)+1)
	require.GreaterOrEqual(t, len(texto), 5<<20)

	inicio := time.Now()
	redactado := dominio.RedactarJWT(texto)

	require.Less(t, time.Since(inicio), 10*time.Second, "la expresión es de tiempo lineal")
	require.Equal(t, strings.Count(texto, jwtDePrueba), strings.Count(redactado, dominio.Redactado))
	require.NotContains(t, redactado, jwtDePrueba)
}
