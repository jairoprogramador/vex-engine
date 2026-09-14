package record_test

// El secreto local del que se derivan las claves de resumen (spec 20 §5.2).
//
// Lo que estos casos fijan no es la criptografía —eso es del dominio y lo mide
// `TestParameterDigester`— sino las dos propiedades de las que depende que el
// digest signifique algo: que se cree UNA vez y que no se pise nunca.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infraRecord "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
)

func TestReadOrCreateDigestSecret_SeCreaUnaVezYNoSePisa(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "digest-v1.key")

	primero, err := infraRecord.ReadOrCreateDigestSecret(path)
	require.NoError(t, err)
	require.Len(t, primero, 32)

	segundo, err := infraRecord.ReadOrCreateDigestSecret(path)
	require.NoError(t, err)

	// **Es la propiedad de la que cuelga todo lo demás.** Un secreto que se
	// regenerara invalidaría toda la historia de resúmenes del destino sin decir
	// nada: los digests dejarían de coincidir y el consumidor concluiría que los
	// valores cambiaron.
	assert.Equal(t, primero, segundo)
}

// Dos destinos son dos secretos. Es lo que impide construir una tabla de
// resúmenes que valga en todas partes.
func TestReadOrCreateDigestSecret_DosDestinosDosSecretos(t *testing.T) {
	uno, err := infraRecord.ReadOrCreateDigestSecret(
		filepath.Join(t.TempDir(), "keys", "digest-v1.key"))
	require.NoError(t, err)

	otro, err := infraRecord.ReadOrCreateDigestSecret(
		filepath.Join(t.TempDir(), "keys", "digest-v1.key"))
	require.NoError(t, err)

	assert.NotEqual(t, uno, otro)
}

// Nace 0600, y es la única pieza del motor que lo hace: todo lo demás que el
// motor escribe está pensado para leerse.
func TestReadOrCreateDigestSecret_NaceConPermisosDeSecreto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "digest-v1.key")

	_, err := infraRecord.ReadOrCreateDigestSecret(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// Un secreto ilegible NO se regenera. Regenerarlo sería el mismo fallo
// silencioso que el archivo existe para evitar, sólo que disfrazado de
// recuperación.
func TestReadOrCreateDigestSecret_UnSecretoIlegibleEsUnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digest-v1.key")
	require.NoError(t, os.WriteFile(path, []byte("esto no es hexadecimal\n"), 0o600))

	_, err := infraRecord.ReadOrCreateDigestSecret(path)
	require.Error(t, err)

	// Y sigue ahí: nadie lo ha sustituido por su cuenta.
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "esto no es hexadecimal\n", string(data))
}

// Un secreto demasiado corto tampoco se «arregla»: la clave sería adivinable y
// el resumen volvería a ser una búsqueda en una tabla.
func TestReadOrCreateDigestSecret_UnSecretoCortoEsUnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "digest-v1.key")
	require.NoError(t, os.WriteFile(path, []byte("abcdef\n"), 0o600))

	_, err := infraRecord.ReadOrCreateDigestSecret(path)
	require.Error(t, err)
}
