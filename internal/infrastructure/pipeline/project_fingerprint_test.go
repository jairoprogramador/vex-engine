package pipeline_test

// Tests de CARACTERIZACIÓN de la huella del proyecto (spec 00).
//
// Estos vectores son DESCRIPTIVOS, no normativos: congelan lo que
// `ProjectFingerprint.FromDirectory` hace HOY, incluidas sus divergencias
// respecto de git. La spec 08 decide cuáles se conservan y cuáles se corrigen;
// el valor de este archivo es que esa decisión aparezca como un diff rojo en
// vez de como un cambio silencioso de huella en producción.
//
// Si un vector se pone en rojo sin que nadie haya tocado el algoritmo a
// propósito, hay una regresión. Si se toca a propósito, se actualiza el valor
// en el mismo commit que el cambio.

import (
	"os"
	"path/filepath"
	"testing"

	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sha256 de la cadena vacía: la huella de un árbol sin ninguna entrada.
const fingerprintOfEmptyTree = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// --- construcción del árbol fixture ---------------------------------------
//
// El árbol se declara en código y se materializa en t.TempDir() en vez de
// vivir en testdata/ porque varios casos necesitan archivos `.gitignore`
// anidados: git los aplicaría al propio repositorio y dejaría de versionar los
// archivos que el fixture pretende ignorar, vaciando el vector en silencio.

type nodeKind int

const (
	kindFile nodeKind = iota
	kindDir
	kindSymlink
)

type node struct {
	kind nodeKind
	path string
	data string // contenido para kindFile, destino para kindSymlink
}

func file(path, content string) node { return node{kind: kindFile, path: path, data: content} }
func dir(path string) node           { return node{kind: kindDir, path: path} }
func symlink(path, target string) node {
	return node{kind: kindSymlink, path: path, data: target}
}

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
		case kindSymlink:
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
			require.NoError(t, os.Symlink(filepath.FromSlash(n.data), abs))
		}
	}
}

func fingerprintOf(t *testing.T, nodes []node) string {
	t.Helper()
	root := t.TempDir()
	buildTree(t, root, nodes)

	fp, err := infraPipeline.NewProjectFingerprint().FromDirectory(root)
	require.NoError(t, err)
	return fp
}

// --- 5.1 vectores dorados --------------------------------------------------

func TestProjectFingerprint_FromDirectory_VectoresDorados(t *testing.T) {
	cases := []struct {
		name  string
		pinea string
		nodes []node
		want  string
	}{
		{
			name:  "árbol plano de 3 archivos",
			pinea: "orden lexicográfico de las entradas `path:hash` unidas por \\n",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
			},
			want: "6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
		},
		{
			name:  "archivo vacío",
			pinea: "un archivo vacío participa con sha256(\"\") como cualquier otro",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("vacio.txt", ""),
			},
			want: "58873ec8ef3abdaf28dedac7d5b045d508e20491e6d2314f944cfacb4afe5bb3",
		},
		{
			name:  "directorio vacío",
			pinea: "los directorios no aportan entradas: mismo hash que el árbol plano",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
				dir("vacio"),
				dir("anidado/tambien/vacio"),
			},
			want: "6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
		},
		{
			name:  ".gitignore en raíz",
			pinea: "el propio .gitignore se excluye del hash, y su regla se aplica",
			nodes: []node{
				file(".gitignore", "*.log\n"),
				file("a.txt", "contenido a"),
				file("ruido.log", "descartado"),
				file("b.txt", "contenido b"),
			},
			want: "fb44111d5c62394ae511463fcaf76f8b2c6379d23b97659a071e69901f6169af",
		},
		{
			name:  ".gitignore anidado",
			pinea: "el dominio de la regla: sub/secret.txt se ignora, secret.txt no",
			nodes: []node{
				file("sub/.gitignore", "secret.txt\n"),
				file("secret.txt", "raíz visible"),
				file("sub/secret.txt", "ignorado"),
				file("sub/otro.txt", "visible"),
			},
			want: "7faaa15e671a4adb045b376b53b6cef4c77dbc508fb48642070e1aeb9b3a9479",
		},
		{
			name:  "patrón ! de inclusión",
			pinea: "gana la ÚLTIMA regla que casa en el recorrido, no la más profunda",
			nodes: []node{
				file(".gitignore", "*.log\n!keep.log\n"),
				file("sub/.gitignore", "!deep.log\n"),
				file("keep.log", "re-incluido en raíz"),
				file("ruido.log", "ignorado"),
				file("sub/deep.log", "re-incluido en sub"),
				file("sub/ruido.log", "ignorado"),
			},
			want: "32e3e2193ec399c579dc0d38aa82b2dcc1d717056c02240b9407039b9fc3fbbf",
		},
		{
			name:  "patrón **",
			pinea: "la rama recursiva: **/generated.txt casa a cualquier profundidad",
			nodes: []node{
				file(".gitignore", "**/generated.txt\n"),
				file("generated.txt", "ignorado en raíz"),
				file("a/generated.txt", "ignorado a 1 nivel"),
				file("a/b/generated.txt", "ignorado a 2 niveles"),
				file("a/b/real.txt", "visible"),
				file("a/real.txt", "visible tambien"),
			},
			want: "f053a46f82c8d608e8bb7c3391a0e2a994929491e0f253c7aeb4f2a103b83d28",
		},
		{
			name:  "patrón anclado /build",
			pinea: "sólo compara prefijo: /build ignora también build/x/y",
			nodes: []node{
				file(".gitignore", "/build\n"),
				file("build/x/y.txt", "ignorado"),
				file("build/z.txt", "ignorado"),
				file("src/build/keep.txt", "visible: el patrón está anclado a la raíz"),
				file("main.txt", "visible"),
			},
			want: "acbdfa0e12f3452a0b5720fefec37c2e1ae3087f0302c847af14c4fe2eb1ef80",
		},
		{
			name:  "patrón sin anclar",
			pinea: "se prueba contra CADA componente del path, no sólo contra el último",
			nodes: []node{
				file(".gitignore", "target\n"),
				file("target/out.bin", "ignorado: componente intermedio casa"),
				file("mod/target/out.bin", "ignorado: componente intermedio casa"),
				file("otro/target", "ignorado: archivo llamado target"),
				file("mod/src/main.txt", "visible"),
				file("mod/src/util.txt", "visible tambien"),
			},
			want: "365131bb6b6b5016dd2045e2a0b6c66bed74ba2b895eccf53f09ab5366ef2625",
		},
		{
			name:  "dirOnly build/",
			pinea: "el sufijo / restringe la regla a directorios: un ARCHIVO build sobrevive",
			nodes: []node{
				file(".gitignore", "build/\n"),
				file("build/out.bin", "ignorado"),
				file("mod/build/out.bin", "ignorado"),
				file("otro/build", "visible: es un archivo, no un directorio"),
				file("otro/keep.txt", "visible tambien"),
			},
			want: "735d799faf41c269be22142880cc1ced683b72c056e272b5db239de81967c603",
		},
		{
			name:  "symlink dentro del árbol",
			pinea: "los symlinks se ignoran por completo: no aportan entrada",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
				symlink("link-a.txt", "a.txt"),
				symlink("link-dir", "."),
			},
			want: "6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
		},
		{
			name:  "directorio .git",
			pinea: "el directorio .git se salta con SkipDir",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
				file(".git/HEAD", "ref: refs/heads/main"),
				file(".git/objects/ab/cdef", "binario"),
			},
			want: "6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
		},
		{
			name:  "árbol totalmente ignorado",
			pinea: "sin entradas la huella es sha256(\"\")",
			nodes: []node{
				file(".gitignore", "*\n"),
				file("a.txt", "ignorado"),
				file("sub/b.txt", "ignorado"),
			},
			want: fingerprintOfEmptyTree,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprintOf(t, tc.nodes)

			if tc.want == "" {
				t.Fatalf("vector dorado sin valor. pinea: %s\nhuella actual: %s", tc.pinea, got)
			}
			assert.Equal(t, tc.want, got, "cambió la huella; pinea: %s", tc.pinea)
		})
	}
}

// Relaciones entre vectores que un valor aislado no expresa.
func TestProjectFingerprint_FromDirectory_RelacionesEntreVectores(t *testing.T) {
	plano := []node{
		file("a.txt", "contenido a"),
		file("b.txt", "contenido b"),
		file("c.txt", "contenido c"),
	}

	t.Run("los directorios vacíos no cambian la huella", func(t *testing.T) {
		conDirs := append(append([]node{}, plano...), dir("vacio"), dir("anidado/tambien/vacio"))
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conDirs))
	})

	t.Run("los symlinks no cambian la huella", func(t *testing.T) {
		conLinks := append(append([]node{}, plano...),
			symlink("link-a.txt", "a.txt"),
			symlink("link-dir", "."),
		)
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conLinks))
	})

	t.Run("el contenido de .git no cambia la huella", func(t *testing.T) {
		conGit := append(append([]node{}, plano...),
			file(".git/HEAD", "ref: refs/heads/main"),
			file(".git/objects/ab/cdef", "binario"),
		)
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conGit))
	})

	t.Run("el contenido del propio .gitignore no cambia la huella", func(t *testing.T) {
		conIgnoreA := append(append([]node{}, plano...), file(".gitignore", "# comentario\n"))
		conIgnoreB := append(append([]node{}, plano...), file(".gitignore", "# otro comentario distinto\n"))
		assert.Equal(t, fingerprintOf(t, conIgnoreA), fingerprintOf(t, conIgnoreB))
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conIgnoreA))
	})
}

// --- 5.2 propiedades -------------------------------------------------------

func TestProjectFingerprint_FromDirectory_Propiedades(t *testing.T) {
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
		repo := infraPipeline.NewProjectFingerprint()

		primera, err := repo.FromDirectory(root)
		require.NoError(t, err)
		segunda, err := repo.FromDirectory(root)
		require.NoError(t, err)

		assert.Equal(t, primera, segunda)
	})

	t.Run("sensibilidad: un byte distinto en un archivo no ignorado cambia la huella", func(t *testing.T) {
		modificado := make([]node, len(base))
		copy(modificado, base)
		modificado[1] = file("sub/b.txt", "contenido B")

		assert.NotEqual(t, fingerprintOf(t, base), fingerprintOf(t, modificado))
	})

	t.Run("un byte distinto en un archivo ignorado NO cambia la huella", func(t *testing.T) {
		modificado := make([]node, len(base))
		copy(modificado, base)
		modificado[4] = file("ruido.log", "ignorado, con otro contenido")

		assert.Equal(t, fingerprintOf(t, base), fingerprintOf(t, modificado))
	})

	t.Run("insensibilidad al orden de creación", func(t *testing.T) {
		invertido := make([]node, 0, len(base))
		for i := len(base) - 1; i >= 0; i-- {
			invertido = append(invertido, base[i])
		}

		assert.Equal(t, fingerprintOf(t, base), fingerprintOf(t, invertido))
	})

	t.Run("renombrar un archivo cambia la huella aunque el contenido sea el mismo", func(t *testing.T) {
		renombrado := make([]node, len(base))
		copy(renombrado, base)
		renombrado[0] = file("renombrado.txt", "contenido a")

		assert.NotEqual(t, fingerprintOf(t, base), fingerprintOf(t, renombrado))
	})
}

// --- casos de borde de la API ---------------------------------------------

func TestProjectFingerprint_FromDirectory_DirectorioInexistente(t *testing.T) {
	_, err := infraPipeline.NewProjectFingerprint().
		FromDirectory(filepath.Join(t.TempDir(), "no-existe"))

	require.Error(t, err)
}

func TestProjectFingerprint_FromFile(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, []node{file("a.txt", "contenido a"), file("vacio.txt", "")})
	repo := infraPipeline.NewProjectFingerprint()

	t.Run("hash del contenido", func(t *testing.T) {
		got, err := repo.FromFile(filepath.Join(root, "a.txt"))
		require.NoError(t, err)
		assert.Equal(t, "56d1c25cee76c7720e36bb63f7e65f64e46debc2e74ec87b7e0b9b8d822fe051", got)
	})

	t.Run("archivo vacío", func(t *testing.T) {
		got, err := repo.FromFile(filepath.Join(root, "vacio.txt"))
		require.NoError(t, err)
		assert.Equal(t, fingerprintOfEmptyTree, got)
	})

	t.Run("archivo inexistente devuelve cadena vacía SIN error", func(t *testing.T) {
		got, err := repo.FromFile(filepath.Join(root, "no-existe.txt"))
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})
}
