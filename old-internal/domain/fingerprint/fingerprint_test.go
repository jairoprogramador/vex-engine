package fingerprint_test

import (
	"testing"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const hashValido = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestFingerprint_RepresentacionCanonica(t *testing.T) {
	fp, err := domFingerprint.New(hashValido)
	require.NoError(t, err)

	assert.Equal(t, "v1:"+hashValido, fp.String())
	assert.Equal(t, "v1", fp.Version())
	assert.Equal(t, hashValido, fp.Hash())
	assert.False(t, fp.IsZero())
}

func TestFingerprint_NewRechazaHashesQueNoLoSon(t *testing.T) {
	cases := map[string]string{
		"vacío":             "",
		"corto":             "abc",
		"con mayúsculas":    "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",
		"no hexadecimal":    "z3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"con prefijo v1":    "v1:" + hashValido,
		"largo de sobra":    hashValido + "00",
		"con espacio final": hashValido[:63] + " ",
	}

	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domFingerprint.New(hash)
			require.Error(t, err)
		})
	}
}

func TestFingerprint_ParseIdaYVuelta(t *testing.T) {
	fp, err := domFingerprint.New(hashValido)
	require.NoError(t, err)

	leida, err := domFingerprint.Parse(fp.String())
	require.NoError(t, err)

	assert.True(t, fp.Equals(leida))
}

func TestFingerprint_ParseRechazaLoQueNoTieneVersion(t *testing.T) {
	_, err := domFingerprint.Parse(hashValido)
	require.Error(t, err)
}

// La versión forma parte de la identidad: sin esto, la corrección de una
// divergencia en una v2 futura sería indistinguible de un bug.
func TestFingerprint_DosVersionesDelMismoHashNoSonIguales(t *testing.T) {
	v1, err := domFingerprint.Parse("v1:" + hashValido)
	require.NoError(t, err)
	v2, err := domFingerprint.Parse("v2:" + hashValido)
	require.NoError(t, err)

	assert.False(t, v1.Equals(v2))
	assert.NotEqual(t, v1.String(), v2.String())
}

func TestFingerprint_ValorCeroNoEsUnaIdentidad(t *testing.T) {
	var cero domFingerprint.Fingerprint

	assert.True(t, cero.IsZero())
	assert.Empty(t, cero.String())
}
