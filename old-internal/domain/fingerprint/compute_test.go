package fingerprint_test

// Vectores NORMATIVOS de la regla v1 (spec 08).
//
// A diferencia de los de la spec 00 —que eran descriptivos y vivían sobre
// disco—, estos son la especificación ejecutable: SPEC-v1.md los reproduce en
// una tabla, y una implementación independiente de la regla debe obtener estos
// mismos valores. Se ejecutan en memoria, sin tocar el sistema de archivos.
//
// Si un vector se pone en rojo sin que nadie haya cambiado SPEC-v1.md a
// propósito, hay una regresión. Si la regla cambia a propósito, cambia de
// versión: `v1:` deja de ser `v1:`.

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"strings"
	"testing"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	infraFingerprint "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// La huella de un árbol sin ninguna entrada: sha256 de la cadena vacía.
const fingerprintOfEmptyTree = "v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// --- declaración del árbol -------------------------------------------------

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

func sourceOf(nodes []node) *infraFingerprint.MemTreeSource {
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

func fingerprintOf(t *testing.T, nodes []node) string {
	t.Helper()
	fp, err := domFingerprint.Compute(sourceOf(nodes))
	require.NoError(t, err)
	return fp.String()
}

// --- vectores --------------------------------------------------------------

func TestCompute_Vectores(t *testing.T) {
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
			want: "v1:6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
		},
		{
			name:  "archivo vacío",
			pinea: "un archivo vacío participa con sha256(\"\") como cualquier otro",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("vacio.txt", ""),
			},
			want: "v1:58873ec8ef3abdaf28dedac7d5b045d508e20491e6d2314f944cfacb4afe5bb3",
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
			want: "v1:6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
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
			want: "v1:fb44111d5c62394ae511463fcaf76f8b2c6379d23b97659a071e69901f6169af",
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
			want: "v1:7faaa15e671a4adb045b376b53b6cef4c77dbc508fb48642070e1aeb9b3a9479",
		},
		{
			name:  "patrón ! de inclusión",
			pinea: "divergencia (a): gana la ÚLTIMA regla que casa en el recorrido, no la más profunda",
			nodes: []node{
				file(".gitignore", "*.log\n!keep.log\n"),
				file("sub/.gitignore", "!deep.log\n"),
				file("keep.log", "re-incluido en raíz"),
				file("ruido.log", "ignorado"),
				file("sub/deep.log", "re-incluido en sub"),
				file("sub/ruido.log", "ignorado"),
			},
			want: "v1:32e3e2193ec399c579dc0d38aa82b2dcc1d717056c02240b9407039b9fc3fbbf",
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
			want: "v1:f053a46f82c8d608e8bb7c3391a0e2a994929491e0f253c7aeb4f2a103b83d28",
		},
		{
			name:  "patrón anclado /build",
			pinea: "divergencia (b): sólo compara prefijo, así que /build ignora también build/x/y",
			nodes: []node{
				file(".gitignore", "/build\n"),
				file("build/x/y.txt", "ignorado"),
				file("build/z.txt", "ignorado"),
				file("src/build/keep.txt", "visible: el patrón está anclado a la raíz"),
				file("main.txt", "visible"),
			},
			want: "v1:acbdfa0e12f3452a0b5720fefec37c2e1ae3087f0302c847af14c4fe2eb1ef80",
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
			want: "v1:365131bb6b6b5016dd2045e2a0b6c66bed74ba2b895eccf53f09ab5366ef2625",
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
			want: "v1:735d799faf41c269be22142880cc1ced683b72c056e272b5db239de81967c603",
		},
		{
			name:  "directorio .git",
			pinea: "el directorio .git se salta entero, sin regla que lo diga",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
				file(".git/HEAD", "ref: refs/heads/main"),
				file(".git/objects/ab/cdef", "binario"),
			},
			want: "v1:6cea0fdde9f2637bd9df676baf9057815330c27aca9e89b3c517ae8918adda22",
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

		// --- lo que la spec 08 corrige -------------------------------------

		{
			name:  "symlink dentro del árbol",
			pinea: "corrección (c): el enlace aporta entrada `path:sha256(destino):l`, sin seguirlo",
			nodes: []node{
				file("a.txt", "contenido a"),
				file("b.txt", "contenido b"),
				file("c.txt", "contenido c"),
				symlink("link-a.txt", "a.txt"),
				symlink("link-dir", "."),
			},
			want: "v1:fd5c03e1e9bb7b2cb0b44cac46f8a41e8dc4e34800433ff93f644d13530df715",
		},
		{
			name:  "bit de ejecución",
			pinea: "corrección (d): un ejecutable aporta `path:hash:x`, no `path:hash`",
			nodes: []node{
				file("a.txt", "contenido a"),
				executable("deploy.sh", "#!/bin/sh\necho hola\n"),
			},
			want: "v1:da09ecfdebafb25a6841743107eed46cca165b9ab7b4a35fb3223ed33a3feddd",
		},
		{
			name:  "escape de espacio final en .gitignore",
			pinea: "corrección (e): `ruido\\ ` es el patrón «ruido » y casa con el archivo «ruido »",
			nodes: []node{
				file(".gitignore", "ruido\\ \n"),
				file("a.txt", "contenido a"),
				file("ruido ", "ignorado: el espacio final está escapado"),
			},
			want: "v1:5d2068fc2f1ff88970c37f5a7f3e40c065d791c207352f8127f30b44b8d722f8",
		},
		{
			name:  "escape de # y ! en .gitignore",
			pinea: "corrección (e): `\\#raro` y `\\!raro` son patrones literales, no comentario ni negación",
			nodes: []node{
				file(".gitignore", "\\#raro\n\\!raro\n"),
				file("a.txt", "contenido a"),
				file("#raro", "ignorado"),
				file("!raro", "ignorado"),
			},
			want: "v1:5d2068fc2f1ff88970c37f5a7f3e40c065d791c207352f8127f30b44b8d722f8",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprintOf(t, tc.nodes)

			if tc.want == "" {
				t.Fatalf("vector sin valor. pinea: %s\nhuella actual: %s", tc.pinea, got)
			}
			assert.Equal(t, tc.want, got, "cambió la huella; pinea: %s", tc.pinea)
		})
	}
}

// Relaciones entre vectores que un valor aislado no expresa.
func TestCompute_RelacionesEntreVectores(t *testing.T) {
	plano := []node{
		file("a.txt", "contenido a"),
		file("b.txt", "contenido b"),
		file("c.txt", "contenido c"),
	}

	t.Run("los directorios vacíos no cambian la huella", func(t *testing.T) {
		conDirs := append(append([]node{}, plano...), dir("vacio"), dir("anidado/tambien/vacio"))
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conDirs))
	})

	t.Run("los symlinks SÍ cambian la huella", func(t *testing.T) {
		conLinks := append(append([]node{}, plano...), symlink("link-a.txt", "a.txt"))
		assert.NotEqual(t, fingerprintOf(t, plano), fingerprintOf(t, conLinks))
	})

	t.Run("cambiar el destino de un symlink cambia la huella", func(t *testing.T) {
		haciaA := append(append([]node{}, plano...), symlink("link.txt", "a.txt"))
		haciaB := append(append([]node{}, plano...), symlink("link.txt", "b.txt"))
		assert.NotEqual(t, fingerprintOf(t, haciaA), fingerprintOf(t, haciaB))
	})

	t.Run("un symlink no es el archivo al que apunta", func(t *testing.T) {
		// El destino es el contenido del enlace: `link.txt → a.txt` hashea la
		// cadena "a.txt", no "contenido a".
		conLink := append(append([]node{}, plano...), symlink("link.txt", "a.txt"))
		conCopia := append(append([]node{}, plano...), file("link.txt", "contenido a"))
		assert.NotEqual(t, fingerprintOf(t, conLink), fingerprintOf(t, conCopia))
	})

	t.Run("el bit de ejecución cambia la huella", func(t *testing.T) {
		sinBit := []node{file("a.txt", "contenido a"), file("deploy.sh", "echo hola")}
		conBit := []node{file("a.txt", "contenido a"), executable("deploy.sh", "echo hola")}
		assert.NotEqual(t, fingerprintOf(t, sinBit), fingerprintOf(t, conBit))
	})

	t.Run("el contenido de .git no cambia la huella", func(t *testing.T) {
		conGit := append(append([]node{}, plano...),
			file(".git/HEAD", "ref: refs/heads/main"),
			file(".git/objects/ab/cdef", "binario"),
		)
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conGit))
	})

	t.Run("un espacio final escapado conserva el patrón", func(t *testing.T) {
		soloA := []node{file("a.txt", "contenido a")}

		// `ruido\ ` es el patrón «ruido », con su espacio: casa con el archivo.
		escapado := []node{
			file(".gitignore", "ruido\\ \n"),
			file("a.txt", "contenido a"),
			file("ruido ", "ignorado"),
		}
		assert.Equal(t, fingerprintOf(t, soloA), fingerprintOf(t, escapado))

		// Sin escapar, los espacios finales se recortan: el patrón es «ruido» y
		// el archivo «ruido » sobrevive.
		sinEscapar := []node{
			file(".gitignore", "ruido   \n"),
			file("a.txt", "contenido a"),
			file("ruido ", "visible"),
		}
		assert.NotEqual(t, fingerprintOf(t, soloA), fingerprintOf(t, sinEscapar))
	})

	t.Run("un # o un ! escapados son literales", func(t *testing.T) {
		soloA := []node{file("a.txt", "contenido a")}

		escapado := []node{
			file(".gitignore", "\\#raro\n\\!raro\n"),
			file("a.txt", "contenido a"),
			file("#raro", "ignorado"),
			file("!raro", "ignorado"),
		}
		assert.Equal(t, fingerprintOf(t, soloA), fingerprintOf(t, escapado))

		// Sin escapar: la primera línea es un comentario y la segunda una
		// negación. Ninguna ignora nada.
		sinEscapar := []node{
			file(".gitignore", "#raro\n!raro\n"),
			file("a.txt", "contenido a"),
			file("#raro", "visible"),
			file("!raro", "visible"),
		}
		assert.NotEqual(t, fingerprintOf(t, soloA), fingerprintOf(t, sinEscapar))
	})

	t.Run("el contenido del propio .gitignore no cambia la huella", func(t *testing.T) {
		conIgnoreA := append(append([]node{}, plano...), file(".gitignore", "# comentario\n"))
		conIgnoreB := append(append([]node{}, plano...), file(".gitignore", "# otro comentario distinto\n"))
		assert.Equal(t, fingerprintOf(t, conIgnoreA), fingerprintOf(t, conIgnoreB))
		assert.Equal(t, fingerprintOf(t, plano), fingerprintOf(t, conIgnoreA))
	})
}

// --- propiedades -----------------------------------------------------------

func TestCompute_Propiedades(t *testing.T) {
	base := []node{
		file("a.txt", "contenido a"),
		file("sub/b.txt", "contenido b"),
		file("sub/deep/c.txt", "contenido c"),
		file(".gitignore", "*.log\n"),
		file("ruido.log", "ignorado"),
	}

	t.Run("determinismo: dos llamadas sobre el mismo árbol dan la misma huella", func(t *testing.T) {
		src := sourceOf(base)

		primera, err := domFingerprint.Compute(src)
		require.NoError(t, err)
		segunda, err := domFingerprint.Compute(src)
		require.NoError(t, err)

		assert.True(t, primera.Equals(segunda))
	})

	t.Run("sensibilidad: un byte distinto en un archivo no ignorado cambia la huella", func(t *testing.T) {
		modificado := append([]node{}, base...)
		modificado[1] = file("sub/b.txt", "contenido B")

		assert.NotEqual(t, fingerprintOf(t, base), fingerprintOf(t, modificado))
	})

	t.Run("un byte distinto en un archivo ignorado NO cambia la huella", func(t *testing.T) {
		modificado := append([]node{}, base...)
		modificado[4] = file("ruido.log", "ignorado, con otro contenido")

		assert.Equal(t, fingerprintOf(t, base), fingerprintOf(t, modificado))
	})

	t.Run("insensibilidad al orden de declaración", func(t *testing.T) {
		invertido := make([]node, 0, len(base))
		for i := len(base) - 1; i >= 0; i-- {
			invertido = append(invertido, base[i])
		}

		assert.Equal(t, fingerprintOf(t, base), fingerprintOf(t, invertido))
	})

	t.Run("renombrar un archivo cambia la huella aunque el contenido sea el mismo", func(t *testing.T) {
		renombrado := append([]node{}, base...)
		renombrado[0] = file("renombrado.txt", "contenido a")

		assert.NotEqual(t, fingerprintOf(t, base), fingerprintOf(t, renombrado))
	})

	t.Run("toda huella lleva la versión de la regla", func(t *testing.T) {
		fp, err := domFingerprint.Compute(sourceOf(base))
		require.NoError(t, err)

		assert.Equal(t, domFingerprint.Version, fp.Version())
		assert.Len(t, fp.Hash(), 64)
		assert.Equal(t, "v1:"+fp.Hash(), fp.String())
	})
}

// --- la composición, construida a mano -------------------------------------

// Un valor dorado dice «esto cambió»; no dice QUÉ es. Este test construye la
// huella siguiendo SPEC-v1.md §3.2–§3.4 a mano —formato de cada entrada, orden
// literal, separador, ausencia de salto final— sin reutilizar nada de la
// implementación salvo el sha256. Es el test que traduce la especificación.
func TestCompute_LaComposicionEsLaEspecificada(t *testing.T) {
	h := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}

	nodes := []node{
		file("a", "contenido de a"),
		file("a.txt", "contenido de a.txt"),
		executable("run", "#!/bin/sh\n"),
		symlink("link", "a.txt"),
		dir("vacio"),
	}

	// El orden está escrito a mano, no calculado: se ordena por la entrada
	// completa, no por la ruta. Por eso "a.txt:…" va ANTES que "a:…" — el byte
	// que sigue a "a" es "." (0x2E) en un caso y ":" (0x3A) en el otro.
	entries := []string{
		"a.txt:" + h("contenido de a.txt"),
		"a:" + h("contenido de a"),
		"link:" + h("a.txt") + ":l", // el destino es el contenido del enlace
		"run:" + h("#!/bin/sh\n") + ":x",
	}

	want := "v1:" + h(strings.Join(entries, "\n"))

	assert.Equal(t, want, fingerprintOf(t, nodes))
}

// --- casos de borde --------------------------------------------------------

func TestCompute_FuenteNula(t *testing.T) {
	_, err := domFingerprint.Compute(nil)
	require.Error(t, err)
}

func TestCompute_ArbolVacio(t *testing.T) {
	fp, err := domFingerprint.Compute(infraFingerprint.NewMemTreeSource())
	require.NoError(t, err)
	assert.Equal(t, fingerprintOfEmptyTree, fp.String())
}

// Un fallo al leer una entrada no puede degradarse a «esa entrada no existía»:
// eso produciría una huella válida de un árbol que nadie leyó entero.
func TestCompute_UnErrorDeLecturaNoSeSilencia(t *testing.T) {
	_, err := domFingerprint.Compute(&fuenteQueFallaAlAbrir{
		MemTreeSource: infraFingerprint.NewMemTreeSource().
			AddFile("a.txt", "contenido a").
			AddFile("b.txt", "contenido b"),
		rota: "b.txt",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "b.txt")
}

type fuenteQueFallaAlAbrir struct {
	*infraFingerprint.MemTreeSource
	rota string
}

func (f *fuenteQueFallaAlAbrir) Open(path string) (io.ReadCloser, error) {
	if path == f.rota {
		return nil, fs.ErrPermission
	}
	return f.MemTreeSource.Open(path)
}
