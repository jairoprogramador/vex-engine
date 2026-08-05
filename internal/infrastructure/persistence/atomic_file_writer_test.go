package persistence_test

// Tests del colaborador que da durabilidad a los cinco repositorios de archivo
// (spec 02 §5.1). Lo que se afirma aquí es la garantía completa: o el contenido
// nuevo está entero, o el previo sigue entero y hay un error. Nunca un archivo
// a medias, nunca un temporal huérfano.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
)

func escribir(contenido string) func(io.Writer) error {
	return func(out io.Writer) error {
		_, err := out.Write([]byte(contenido))
		return err
	}
}

func TestWrite_CreaElArchivoYSusDirectorios(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "a", "b", "estado.gob")

	err := persistence.NewAtomicFileWriter().Write(destino, escribir("contenido"))
	require.NoError(t, err)

	leido, err := os.ReadFile(destino)
	require.NoError(t, err)
	require.Equal(t, "contenido", string(leido))

	info, err := os.Stat(destino)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestWrite_ReemplazaElContenidoPrevio(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "estado.gob")
	writer := persistence.NewAtomicFileWriter()

	require.NoError(t, writer.Write(destino, escribir("viejo-y-mucho-mas-largo")))
	require.NoError(t, writer.Write(destino, escribir("nuevo")))

	leido, err := os.ReadFile(destino)
	require.NoError(t, err)
	require.Equal(t, "nuevo", string(leido), "el rename publica el archivo entero, no lo sobrescribe encima")
}

// El caso que motiva la spec: una escritura que se rompe a mitad NO puede dejar
// el archivo previo truncado ni un temporal en el directorio.
func TestWrite_EncodeQueFallaDejaElArchivoPrevioIntacto(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "estado.gob")
	writer := persistence.NewAtomicFileWriter()

	require.NoError(t, writer.Write(destino, escribir("previo")))

	fallo := errors.New("se murió a mitad")
	err := writer.Write(destino, func(out io.Writer) error {
		if _, err := out.Write([]byte("a medias")); err != nil {
			return err
		}
		return fallo
	})

	require.ErrorIs(t, err, fallo)

	leido, readErr := os.ReadFile(destino)
	require.NoError(t, readErr)
	require.Equal(t, "previo", string(leido))
	require.Equal(t, []string{"estado.gob"}, nombresEn(t, raiz), "el temporal no puede quedar en el directorio")
}

func TestWrite_EncodeQueFallaNoCreaElArchivo(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "estado.gob")

	err := persistence.NewAtomicFileWriter().Write(destino, func(io.Writer) error {
		return errors.New("nada que escribir")
	})
	require.Error(t, err)

	_, statErr := os.Stat(destino)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	require.Empty(t, nombresEn(t, raiz))
}

// El temporal se crea en el mismo directorio que el destino: un rename entre
// filesystems distintos no es atómico.
func TestWrite_ElTemporalViveJuntoAlDestino(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "sub", "estado.gob")

	var vecinos []string
	err := persistence.NewAtomicFileWriter().Write(destino, func(out io.Writer) error {
		vecinos = nombresEn(t, filepath.Dir(destino))
		return nil
	})
	require.NoError(t, err)
	require.Len(t, vecinos, 1, "durante el encode solo existe el temporal, en el directorio del destino")
	require.NotEqual(t, "estado.gob", vecinos[0])
}

func TestWrite_PermisosExplicitos(t *testing.T) {
	raiz := t.TempDir()
	destino := filepath.Join(raiz, "estado.gob")

	err := persistence.NewAtomicFileWriterWithPerm(0600).Write(destino, escribir("x"))
	require.NoError(t, err)

	info, err := os.Stat(destino)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func nombresEn(t *testing.T, dir string) []string {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	require.NoError(t, err)
	nombres := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		nombres = append(nombres, entrada.Name())
	}
	return nombres
}
