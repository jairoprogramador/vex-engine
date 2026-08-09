package pipeline_test

// Tests del adaptador de disco de la huella de contenido (specs 00 y 08).
//
// Los vectores NORMATIVOS ya no están aquí: viven en memoria, junto a la regla,
// en `internal/domain/fingerprint`. Lo que se comprueba aquí es lo que sólo se
// puede comprobar contra un sistema de archivos real:
//
//   - que `DirTreeSource` sirva el mismo árbol que la fuente en memoria, que es
//     lo que hace que aquellos vectores signifiquen algo;
//   - que un `chmod +x` y el destino de un enlace cambien la huella —las dos
//     correcciones de la spec 08 que la spec 00 pineó al revés—;
//   - los bordes de la API: un directorio que no existe es un error, no una
//     huella vacía.

import (
	"os"
	"path/filepath"
	"testing"

	domFingerprint "github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	infraFingerprint "github.com/jairoprogramador/vex-engine/internal/infrastructure/fingerprint"
	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// La huella de un árbol sin ninguna entrada: sha256 de la cadena vacía.
const fingerprintOfEmptyTree = "v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// --- construcción del árbol fixture ---------------------------------------
//
// El árbol se declara en código y se materializa en t.TempDir() en vez de vivir
// en testdata/ porque varios casos necesitan archivos `.gitignore` anidados: git
// los aplicaría al propio repositorio y dejaría de versionar los archivos que el
// fixture pretende ignorar, vaciando el caso en silencio.

type nodeKind int

const (
	kindFile nodeKind = iota
	kindExecutable
	kindDir
	kindSymlink
)

type node struct {
	kind nodeKind
	path string
	data string // contenido para los archivos, destino para los enlaces
}

func file(path, content string) node { return node{kind: kindFile, path: path, data: content} }
func executable(path, content string) node {
	return node{kind: kindExecutable, path: path, data: content}
}
func dir(path string) node             { return node{kind: kindDir, path: path} }
func symlink(path, target string) node { return node{kind: kindSymlink, path: path, data: target} }

func buildTree(t *testing.T, root string, nodes []node) {
	t.Helper()
	for _, n := range nodes {
		abs := filepath.Join(root, filepath.FromSlash(n.path))
		switch n.kind {
		case kindDir:
			require.NoError(t, os.MkdirAll(abs, 0o755))
		case kindFile:
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
			require.NoError(t, os.WriteFile(abs, []byte(n.data), 0o644))
		case kindExecutable:
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
			require.NoError(t, os.WriteFile(abs, []byte(n.data), 0o755))
		case kindSymlink:
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
			require.NoError(t, os.Symlink(filepath.FromSlash(n.data), abs))
		}
	}
}

func memSourceOf(nodes []node) *infraFingerprint.MemTreeSource {
	src := infraFingerprint.NewMemTreeSource()
	for _, n := range nodes {
		switch n.kind {
		case kindFile:
			src.AddFile(n.path, n.data)
		case kindExecutable:
			src.AddExecutable(n.path, n.data)
		case kindDir:
			src.AddDir(n.path)
		case kindSymlink:
			src.AddSymlink(n.path, n.data)
		}
	}
	return src
}

func fingerprintOnDisk(t *testing.T, nodes []node) string {
	t.Helper()
	root := t.TempDir()
	buildTree(t, root, nodes)

	fp, err := infraPipeline.NewContentFingerprint().FromDirectory(root)
	require.NoError(t, err)
	return fp.String()
}

// --- equivalencia de fuentes ----------------------------------------------
//
// Este es el test que le da valor a los vectores en memoria: si las dos fuentes
// no sirvieran el mismo árbol, una implementación independiente podría
// reproducir los vectores y aun así no coincidir con lo que el motor calcula en
// producción.

func TestContentFingerprint_DiscoYMemoriaDanLaMismaHuella(t *testing.T) {
	arboles := map[string][]node{
		"plano": {
			file("a.txt", "contenido a"),
			file("b.txt", "contenido b"),
			file("c.txt", "contenido c"),
		},
		"con reglas anidadas": {
			file(".gitignore", "*.log\n!keep.log\n"),
			file("sub/.gitignore", "!deep.log\n"),
			file("keep.log", "re-incluido en raíz"),
			file("ruido.log", "ignorado"),
			file("sub/deep.log", "re-incluido en sub"),
			file("sub/ruido.log", "ignorado"),
			file("sub/main.txt", "visible"),
		},
		"con ejecutables y enlaces": {
			file("a.txt", "contenido a"),
			executable("scripts/deploy.sh", "#!/bin/sh\necho hola\n"),
			file("scripts/notas.md", "no es ejecutable"),
			symlink("link-a.txt", "a.txt"),
			symlink("scripts/link-dir", "../sub"),
			dir("sub"),
			file("sub/b.txt", "contenido b"),
		},
		"con escapes en las reglas": {
			file(".gitignore", "ruido\\ \n\\#raro\n"),
			file("a.txt", "contenido a"),
			file("ruido ", "ignorado"),
			file("#raro", "ignorado"),
		},
		"todo ignorado": {
			file(".gitignore", "*\n"),
			file("a.txt", "ignorado"),
			file("sub/b.txt", "ignorado"),
		},
	}

	for name, nodes := range arboles {
		t.Run(name, func(t *testing.T) {
			enMemoria, err := domFingerprint.Compute(memSourceOf(nodes))
			require.NoError(t, err)

			assert.Equal(t, enMemoria.String(), fingerprintOnDisk(t, nodes))
		})
	}
}

// --- lo que la spec 08 corrige --------------------------------------------

func TestContentFingerprint_SensibilidadAlBitDeEjecucion(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, []node{
		file("a.txt", "contenido a"),
		file("deploy.sh", "#!/bin/sh\necho hola\n"),
	})
	repo := infraPipeline.NewContentFingerprint()

	sinBit, err := repo.FromDirectory(root)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(filepath.Join(root, "deploy.sh"), 0o755))

	conBit, err := repo.FromDirectory(root)
	require.NoError(t, err)

	assert.False(t, sinBit.Equals(conBit),
		"un chmod +x cambia lo que pasa al desplegar: tiene que cambiar la huella")
}

func TestContentFingerprint_SensibilidadAlDestinoDeUnSymlink(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, []node{
		file("a.txt", "contenido a"),
		file("b.txt", "contenido b"),
		symlink("actual.txt", "a.txt"),
	})
	repo := infraPipeline.NewContentFingerprint()

	haciaA, err := repo.FromDirectory(root)
	require.NoError(t, err)

	enlace := filepath.Join(root, "actual.txt")
	require.NoError(t, os.Remove(enlace))
	require.NoError(t, os.Symlink("b.txt", enlace))

	haciaB, err := repo.FromDirectory(root)
	require.NoError(t, err)

	assert.False(t, haciaA.Equals(haciaB))
}

// El enlace no se sigue: lo que entra en la huella es su destino como texto, no
// el contenido del archivo apuntado.
func TestContentFingerprint_ElSymlinkNoSeSigue(t *testing.T) {
	conEnlace := []node{
		file("a.txt", "contenido a"),
		symlink("copia.txt", "a.txt"),
	}
	conCopiaReal := []node{
		file("a.txt", "contenido a"),
		file("copia.txt", "contenido a"),
	}

	assert.NotEqual(t, fingerprintOnDisk(t, conEnlace), fingerprintOnDisk(t, conCopiaReal))
}

// Un enlace roto —o que apunta fuera del árbol— no rompe el cálculo ni saca la
// identidad del proyecto.
func TestContentFingerprint_EnlaceRoto(t *testing.T) {
	conEnlaceRoto := fingerprintOnDisk(t, []node{
		file("a.txt", "contenido a"),
		symlink("roto.txt", "no-existe-en-ninguna-parte"),
	})
	sinEnlace := fingerprintOnDisk(t, []node{
		file("a.txt", "contenido a"),
	})

	assert.NotEqual(t, sinEnlace, conEnlaceRoto)
}

// --- propiedades sobre disco -----------------------------------------------

func TestContentFingerprint_Propiedades(t *testing.T) {
	base := []node{
		file("a.txt", "contenido a"),
		file("sub/b.txt", "contenido b"),
		file("sub/deep/c.txt", "contenido c"),
		file(".gitignore", "*.log\n"),
		file("ruido.log", "ignorado"),
	}

	t.Run("determinismo: dos llamadas sobre el mismo árbol dan la misma huella", func(t *testing.T) {
		root := t.TempDir()
		buildTree(t, root, base)
		repo := infraPipeline.NewContentFingerprint()

		primera, err := repo.FromDirectory(root)
		require.NoError(t, err)
		segunda, err := repo.FromDirectory(root)
		require.NoError(t, err)

		assert.True(t, primera.Equals(segunda))
	})

	t.Run("sensibilidad: un byte distinto en un archivo no ignorado cambia la huella", func(t *testing.T) {
		modificado := append([]node{}, base...)
		modificado[1] = file("sub/b.txt", "contenido B")

		assert.NotEqual(t, fingerprintOnDisk(t, base), fingerprintOnDisk(t, modificado))
	})

	t.Run("un byte distinto en un archivo ignorado NO cambia la huella", func(t *testing.T) {
		modificado := append([]node{}, base...)
		modificado[4] = file("ruido.log", "ignorado, con otro contenido")

		assert.Equal(t, fingerprintOnDisk(t, base), fingerprintOnDisk(t, modificado))
	})

	t.Run("el contenido de .git no cambia la huella", func(t *testing.T) {
		conGit := append(append([]node{}, base...),
			file(".git/HEAD", "ref: refs/heads/main"),
			file(".git/objects/ab/cdef", "binario"),
		)

		assert.Equal(t, fingerprintOnDisk(t, base), fingerprintOnDisk(t, conGit))
	})

	// LAS DOS HUELLAS SON LA MISMA REGLA, no dos parecidas (spec 18 §7).
	//
	// Desde la spec 18 este adaptador sirve DOS árboles: el del proyecto (handler
	// 08) y el del pipelinecode (handler 09). Es barato de comprobar —los dos son
	// `DirTreeSource`— y es lo único que impide que alguien introduzca una
	// variante «para el pipeline», y con ella la comparabilidad entre
	// organizaciones. El renombre `ProjectFingerprint` → `ContentFingerprint` de
	// la spec 08 se hizo por esta razón exacta.
	t.Run("el árbol del proyecto y el del pipelinecode pasan por la misma regla", func(t *testing.T) {
		comoProyecto := t.TempDir()
		comoPipelinecode := t.TempDir()
		buildTree(t, comoProyecto, base)
		buildTree(t, comoPipelinecode, base)

		repo := infraPipeline.NewContentFingerprint()
		delProyecto, err := repo.FromDirectory(comoProyecto)
		require.NoError(t, err)
		delPipelinecode, err := repo.FromDirectory(comoPipelinecode)
		require.NoError(t, err)

		assert.True(t, delProyecto.Equals(delPipelinecode),
			"el mismo contenido tiene que dar el mismo valor, sea cual sea el árbol del que salga")
	})
}

// --- casos de borde de la API ---------------------------------------------

func TestContentFingerprint_DirectorioInexistente(t *testing.T) {
	_, err := infraPipeline.NewContentFingerprint().
		FromDirectory(filepath.Join(t.TempDir(), "no-existe"))

	require.Error(t, err)
}

func TestContentFingerprint_LaRutaEsUnArchivo(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, []node{file("a.txt", "contenido a")})

	_, err := infraPipeline.NewContentFingerprint().
		FromDirectory(filepath.Join(root, "a.txt"))

	require.Error(t, err)
}

// En modo local el proyecto ES un enlace al volumen montado. Si la raíz no se
// resolviera, WalkDir no entraría —un enlace no es un directorio— y la huella
// saldría vacía sin un solo error: exactamente la identidad silenciosa que la
// spec 08 existe para erradicar.
func TestContentFingerprint_LaRaizPuedeSerUnSymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	require.NoError(t, os.MkdirAll(real, 0o755))
	buildTree(t, real, []node{
		file("a.txt", "contenido a"),
		file("sub/b.txt", "contenido b"),
	})

	enlace := filepath.Join(base, "enlace")
	require.NoError(t, os.Symlink(real, enlace))

	repo := infraPipeline.NewContentFingerprint()

	directa, err := repo.FromDirectory(real)
	require.NoError(t, err)
	porElEnlace, err := repo.FromDirectory(enlace)
	require.NoError(t, err)

	assert.True(t, directa.Equals(porElEnlace))
	assert.NotEqual(t, fingerprintOfEmptyTree, directa.String(),
		"la huella de un árbol con archivos no puede ser la del árbol vacío")
}
