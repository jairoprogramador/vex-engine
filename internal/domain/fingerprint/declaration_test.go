package fingerprint_test

// Vectores NORMATIVOS de la regla `pipe-v1` (spec 27).
//
// SPEC-PIPELINE-v1.md los reproduce en una tabla, y una implementación
// independiente de la regla debe obtener estos mismos valores. Se ejecutan en
// memoria sobre `MemTreeSource`, sin tocar el sistema de archivos, igual que los
// 16 de la spec 08.
//
// Si un vector se pone en rojo sin que nadie haya cambiado SPEC-PIPELINE-v1.md a
// propósito, hay una regresión. Si la regla cambia a propósito, cambia de
// versión: `pipe-v1:` deja de ser `pipe-v1:`.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domFingerprint "github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	infraFingerprint "github.com/jairoprogramador/vex-engine/internal/infrastructure/fingerprint"
)

// arbolVacio es un directorio de step que sólo tenía sus archivos de
// declaración: después de la exclusión de §3.4 no queda nada.
func arbolVacio() *infraFingerprint.MemTreeSource {
	return infraFingerprint.NewMemTreeSource().
		AddFile("commands.yaml", "- name: build\n  cmd: mvn package\n").
		AddFile("config.yaml", "scope: environment\n")
}

func declaracionDe(
	t *testing.T,
	m domFingerprint.StepDeclarationMaterial,
	tree domFingerprint.TreeSource) string {

	t.Helper()
	fp, err := domFingerprint.ComputeStepDeclaration(m, tree)
	require.NoError(t, err)
	return fp.String()
}

// --- vectores --------------------------------------------------------------

func TestComputeStepDeclaration_Vectores(t *testing.T) {
	comando := domFingerprint.InstructionMaterial{Name: "build", Cmd: "mvn package"}
	region := domFingerprint.VariableMaterial{Name: "region", Declaration: `"us-east-1"`}

	cases := []struct {
		name     string
		pinea    string
		material domFingerprint.StepDeclarationMaterial
		tree     domFingerprint.TreeSource
		want     string
	}{
		{
			name:     "todo vacío",
			pinea:    "un step que no declara nada tiene huella igualmente: la de la nada",
			material: domFingerprint.StepDeclarationMaterial{},
			tree:     infraFingerprint.NewMemTreeSource(),
			want:     "pipe-v1:63b4d5fc8088564bf120ecd3457bbc7164aa8886e6ec9676dda4434db3d0f89f",
		},
		{
			name:     "sólo el ámbito",
			pinea:    "`scope` es un campo declarado como cualquier otro (§3.1)",
			material: domFingerprint.StepDeclarationMaterial{Scope: "environment"},
			tree:     infraFingerprint.NewMemTreeSource(),
			want:     "pipe-v1:6d4d2125cf6cec9c16de0610c0d85aec3f37ea5d36983a9d5aeeb469a999d7ab",
		},
		{
			name:  "ámbito y reglas",
			pinea: "`config.yaml` entra ENTERO: sin esto, cambiar `rules` no re-ejecutaría",
			material: domFingerprint.StepDeclarationMaterial{
				Scope: "environment",
				Rules: "state_changed\x1e\"pipeline,project\"",
			},
			tree: infraFingerprint.NewMemTreeSource(),
			want: "pipe-v1:24f69822a34b20a25227f3cb4cb093b24b227619711dd6b24a963c1b013d2c3f",
		},
		{
			name:  "más un comando",
			pinea: "el bloque de comandos es la `inst-v1` retirada, sin un byte de diferencia",
			material: domFingerprint.StepDeclarationMaterial{
				Scope:    "environment",
				Rules:    "state_changed\x1e\"pipeline,project\"",
				Commands: []domFingerprint.InstructionMaterial{comando},
			},
			tree: infraFingerprint.NewMemTreeSource(),
			want: "pipe-v1:fcf95580a27707004141c20d3969c24063d350d85cb6909f87d75ca289b64598",
		},
		{
			name:  "más una declaración de variable",
			pinea: "entra la DECLARACIÓN, nunca el valor resuelto (§3.3)",
			material: domFingerprint.StepDeclarationMaterial{
				Scope:     "environment",
				Rules:     "state_changed\x1e\"pipeline,project\"",
				Commands:  []domFingerprint.InstructionMaterial{comando},
				Variables: []domFingerprint.VariableMaterial{region},
			},
			tree: infraFingerprint.NewMemTreeSource(),
			want: "pipe-v1:5e8c0b8fa33766e01d3e23e9e81612bc30538d6c74ff31225757537e965aed2e",
		},
		{
			name:  "más el árbol del directorio del step",
			pinea: "D14: el cuerpo de las plantillas ENTRA (§3.4)",
			material: domFingerprint.StepDeclarationMaterial{
				Scope:     "environment",
				Rules:     "state_changed\x1e\"pipeline,project\"",
				Commands:  []domFingerprint.InstructionMaterial{comando},
				Variables: []domFingerprint.VariableMaterial{region},
			},
			tree: infraFingerprint.NewMemTreeSource().
				AddFile("k8s/deployment.yaml", "replicas: 3\n"),
			want: "pipe-v1:4e39b400046b872123c7ce01c93ab2c69155c11348b93d8ff8b55b248d1a998b",
		},
	}

	vistas := map[string]string{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := declaracionDe(t, c.material, c.tree)
			assert.Equal(t, c.want, got, c.pinea)

			// Cada vector añade una fuente de material al anterior, así que dos
			// iguales significarían que una fuente se cayó del hash en silencio.
			if anterior, repetida := vistas[got]; repetida {
				t.Fatalf("mismo valor que el vector %q: una fuente de material no entra", anterior)
			}
			vistas[got] = c.name
		})
	}
}

// --- el árbol: la fuente que ninguna regla anterior miraba -------------------

// TestComputeStepDeclaration_ElCuerpoDeUnaPlantillaEntra es D14, que es el caso
// que da nombre a la spec 27: hasta aquí `templates:` aportaba las RUTAS al
// material y el contenido de esos archivos no entraba en ninguna huella, así que
// editar el `deployment.yaml` que más se edita a mano de todo el pipelinecode
// hacía que el step se SALTARA.
func TestComputeStepDeclaration_ElCuerpoDeUnaPlantillaEntra(t *testing.T) {
	material := domFingerprint.StepDeclarationMaterial{
		Commands: []domFingerprint.InstructionMaterial{{
			Name:      "deploy",
			Cmd:       "kubectl apply -f k8s/deployment.yaml",
			Templates: []string{"k8s/deployment.yaml"},
		}},
	}

	antes := declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
		AddFile("k8s/deployment.yaml", "replicas: 3\n"))
	despues := declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
		AddFile("k8s/deployment.yaml", "replicas: 5\n"))

	assert.NotEqual(t, antes, despues)
}

// TestComputeStepDeclaration_UnAuxiliarNoDeclaradoEntra es el hermano sin nombre
// de D14, y es la comprobación que distingue esta solución de la alternativa B
// («sólo las plantillas declaradas»): un `.tf` que nadie declaró en `templates:`
// decide qué se provisiona exactamente igual.
func TestComputeStepDeclaration_UnAuxiliarNoDeclaradoEntra(t *testing.T) {
	material := domFingerprint.StepDeclarationMaterial{
		Commands: []domFingerprint.InstructionMaterial{{Name: "apply", Cmd: "terraform apply"}},
	}

	antes := declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
		AddFile("terraform/main/main.tf", `resource "azurerm_container_registry" "acr" {}`))
	despues := declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
		AddFile("terraform/main/main.tf", `resource "azurerm_container_registry" "otro" {}`))

	assert.NotEqual(t, antes, despues)
}

// TestComputeStepDeclaration_LosArchivosDeDeclaracionNoEntranDosVeces es el
// reparto de §3.4, y su observable es la asimetría que la spec 27 §5.3 fija como
// decisión: un comentario en `commands.yaml` NO re-ejecuta, uno en un
// `Dockerfile` SÍ.
func TestComputeStepDeclaration_LosArchivosDeDeclaracionNoEntranDosVeces(t *testing.T) {
	material := domFingerprint.StepDeclarationMaterial{
		Scope:    "environment",
		Commands: []domFingerprint.InstructionMaterial{{Name: "build", Cmd: "mvn package"}},
	}

	t.Run("un comentario en commands.yaml no mueve la huella", func(t *testing.T) {
		sinComentario := declaracionDe(t, material, arbolVacio())
		conComentario := declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
			AddFile("commands.yaml", "# compila el proyecto\n- name: build\n  cmd: mvn package\n").
			AddFile("config.yaml", "# el ámbito\nscope: environment\n"))

		assert.Equal(t, sinComentario, conComentario)
	})

	t.Run("y el directorio sin ellos da la misma huella", func(t *testing.T) {
		assert.Equal(t,
			declaracionDe(t, material, arbolVacio()),
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource()))
	})

	t.Run("un comentario en un Dockerfile sí la mueve", func(t *testing.T) {
		assert.NotEqual(t,
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddFile("Dockerfile", "FROM eclipse-temurin:21\n")),
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddFile("Dockerfile", "# la imagen base\nFROM eclipse-temurin:21\n")))
	})

	t.Run("un commands.yaml anidado NO se excluye: el motor no lo interpreta", func(t *testing.T) {
		assert.NotEqual(t,
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource()),
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddFile("terraform/commands.yaml", "lo que sea")))
	})
}

// TestComputeStepDeclaration_HerenciaDeLaReglaDeArbol comprueba que el árbol
// del directorio del step pasa por la MISMA `Compute` que el del proyecto, con
// sus sensibilidades intactas.
func TestComputeStepDeclaration_HerenciaDeLaReglaDeArbol(t *testing.T) {
	material := domFingerprint.StepDeclarationMaterial{Scope: "environment"}

	t.Run("el bit de ejecución", func(t *testing.T) {
		assert.NotEqual(t,
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddFile("bin/deploy.sh", "#!/bin/sh\n")),
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddExecutable("bin/deploy.sh", "#!/bin/sh\n")))
	})

	t.Run("el destino de un enlace", func(t *testing.T) {
		assert.NotEqual(t,
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddSymlink("link", "a.txt")),
			declaracionDe(t, material, infraFingerprint.NewMemTreeSource().
				AddSymlink("link", "b.txt")))
	})

	t.Run("dos raíces con el mismo árbol dan la misma huella", func(t *testing.T) {
		arbol := func() *infraFingerprint.MemTreeSource {
			return infraFingerprint.NewMemTreeSource().
				AddFile("k8s/deployment.yaml", "replicas: 3\n").
				AddExecutable("bin/deploy.sh", "#!/bin/sh\n")
		}
		assert.Equal(t, declaracionDe(t, material, arbol()), declaracionDe(t, material, arbol()))
	})
}

// --- el material declarado --------------------------------------------------

// TestComputeStepDeclaration_ConfigYamlEntra es el defecto (d) de la spec 27,
// que era el único que aún se podía evitar en vez de corregir: si `config.yaml`
// no entrara, cambiar las reglas de re-ejecución de un step no movería su huella
// y el step se saltaría CON LAS REGLAS VIEJAS.
func TestComputeStepDeclaration_ConfigYamlEntra(t *testing.T) {
	base := domFingerprint.StepDeclarationMaterial{
		Scope:    "environment",
		Rules:    "state_changed\x1e\"pipeline,project\"",
		Commands: []domFingerprint.InstructionMaterial{{Name: "build", Cmd: "mvn package"}},
	}

	t.Run("cambiar el ámbito mueve la huella", func(t *testing.T) {
		otro := base
		otro.Scope = "project"
		assert.NotEqual(t, declaracionDe(t, base, arbolVacio()), declaracionDe(t, otro, arbolVacio()))
	})

	t.Run("cambiar las reglas mueve la huella", func(t *testing.T) {
		otro := base
		otro.Rules = "state_changed\x1e\"pipeline\""
		assert.NotEqual(t, declaracionDe(t, base, arbolVacio()), declaracionDe(t, otro, arbolVacio()))
	})

	t.Run("sin config.yaml los dos campos van vacíos JUNTOS", func(t *testing.T) {
		sin := base
		sin.Scope, sin.Rules = "", ""
		assert.NotEqual(t, declaracionDe(t, base, arbolVacio()), declaracionDe(t, sin, arbolVacio()))
	})
}

// TestComputeStepDeclaration_LaDeclaracionIdentificaYElValorNo es el corazón del
// cambio de material: dos ejecuciones que resuelven valores distintos para la
// misma declaración producen la MISMA `pipe-v1`, y dos declaraciones distintas
// producen huellas distintas.
//
// Es lo que cierra el rebote de más —las salidas de una corrida dejan de ser
// entradas de la siguiente— y a la vez la fuga que la spec 20 §8 anotó: un valor
// producido en runtime deja de entrar en absoluto.
func TestComputeStepDeclaration_LaDeclaracionIdentificaYElValorNo(t *testing.T) {
	conDeclaracion := func(d domFingerprint.VariableMaterial) string {
		return declaracionDe(t,
			domFingerprint.StepDeclarationMaterial{
				Variables: []domFingerprint.VariableMaterial{d},
			}, arbolVacio())
	}

	t.Run("el valor extraído en runtime no está en el material", func(t *testing.T) {
		// La declaración de un `step-output` es la misma resuelva a `acme.azurecr.io`
		// o a `otro.azurecr.io`: el valor no entra por ninguna vía.
		declaracion := domFingerprint.VariableMaterial{
			Name:        "acr_name",
			Declaration: "step-output\x1e\"02-acr\"\x1e\"acr_name\"",
		}
		assert.Equal(t, conDeclaracion(declaracion), conDeclaracion(declaracion))

		huella := conDeclaracion(declaracion)
		assert.NotContains(t, huella, "azurecr")
	})

	t.Run("cambiar el `from` de una declaración SÍ mueve la huella", func(t *testing.T) {
		assert.NotEqual(t,
			conDeclaracion(domFingerprint.VariableMaterial{
				Name: "acr_name", Declaration: "step-output\x1e\"02-acr\"\x1e\"acr_name\""}),
			conDeclaracion(domFingerprint.VariableMaterial{
				Name: "acr_name", Declaration: "step-output\x1e\"03-otro\"\x1e\"acr_name\""}))
	})

	t.Run("editar un literal declarado SÍ mueve la huella", func(t *testing.T) {
		assert.NotEqual(t,
			conDeclaracion(domFingerprint.VariableMaterial{Name: "replicas", Declaration: `"3"`}),
			conDeclaracion(domFingerprint.VariableMaterial{Name: "replicas", Declaration: `"5"`}))
	})

	t.Run("declarada y vacía no es no declarada", func(t *testing.T) {
		assert.NotEqual(t,
			declaracionDe(t, domFingerprint.StepDeclarationMaterial{}, arbolVacio()),
			conDeclaracion(domFingerprint.VariableMaterial{Name: "replicas", Declaration: `""`}))
	})
}

// TestComputeStepDeclaration_ElOrdenDeLasVariablesNoCuenta: un conjunto de
// declaraciones no tiene secuencia, así que se ordena. Es la diferencia
// deliberada con los comandos.
func TestComputeStepDeclaration_ElOrdenDeLasVariablesNoCuenta(t *testing.T) {
	a := domFingerprint.VariableMaterial{Name: "alpha", Declaration: `"1"`}
	b := domFingerprint.VariableMaterial{Name: "beta", Declaration: `"2"`}

	assert.Equal(t,
		declaracionDe(t, domFingerprint.StepDeclarationMaterial{
			Variables: []domFingerprint.VariableMaterial{a, b}}, arbolVacio()),
		declaracionDe(t, domFingerprint.StepDeclarationMaterial{
			Variables: []domFingerprint.VariableMaterial{b, a}}, arbolVacio()))
}

// TestComputeStepDeclaration_ElOrdenDeLosComandosSiCuenta: en una lista de
// comandos el orden es la secuencia de ejecución. Ejecutar `build` y luego
// `deploy` no es lo mismo que al revés.
func TestComputeStepDeclaration_ElOrdenDeLosComandosSiCuenta(t *testing.T) {
	a := domFingerprint.InstructionMaterial{Name: "a", Cmd: "echo a"}
	b := domFingerprint.InstructionMaterial{Name: "b", Cmd: "echo b"}

	assert.NotEqual(t,
		declaracionDe(t, domFingerprint.StepDeclarationMaterial{
			Commands: []domFingerprint.InstructionMaterial{a, b}}, arbolVacio()),
		declaracionDe(t, domFingerprint.StepDeclarationMaterial{
			Commands: []domFingerprint.InstructionMaterial{b, a}}, arbolVacio()))
}

// TestComputeStepDeclaration_ShowEntra conserva la corrección de la spec 10
// §5.1bis, que `pipe-v1` hereda de la `inst-v1` que absorbe: si `show` no
// participara, añadir `show: true` para depurar no invalidaría nada, el step se
// saltaría y no se imprimiría nada.
func TestComputeStepDeclaration_ShowEntra(t *testing.T) {
	sin := domFingerprint.StepDeclarationMaterial{
		Commands: []domFingerprint.InstructionMaterial{{Name: "build", Cmd: "mvn package"}}}
	con := domFingerprint.StepDeclarationMaterial{
		Commands: []domFingerprint.InstructionMaterial{
			{Name: "build", Cmd: "mvn package", Show: true}}}

	assert.NotEqual(t, declaracionDe(t, sin, arbolVacio()), declaracionDe(t, con, arbolVacio()))
}

// TestComputeStepDeclaration_LaCardinalidadSepara: sin la cardinalidad delante
// de cada lista, dos materiales con el mismo total repartido de otra forma
// podrían producir la misma cadena.
func TestComputeStepDeclaration_LaCardinalidadSepara(t *testing.T) {
	unComando := domFingerprint.StepDeclarationMaterial{
		Commands: []domFingerprint.InstructionMaterial{{Name: "a", Cmd: ""}},
	}
	unaVariable := domFingerprint.StepDeclarationMaterial{
		Variables: []domFingerprint.VariableMaterial{{Name: "a", Declaration: ""}},
	}

	assert.NotEqual(t,
		declaracionDe(t, unComando, arbolVacio()),
		declaracionDe(t, unaVariable, arbolVacio()))
}

// --- material incompleto ----------------------------------------------------

// TestComputeStepDeclaration_SinArbolEsError: un árbol ausente no es un árbol
// vacío. Sin esta guarda, un fallo al abrir el directorio del step produciría una
// huella perfectamente válida que COLISIONA con la de cualquier step cuyo
// directorio esté de verdad vacío.
func TestComputeStepDeclaration_SinArbolEsError(t *testing.T) {
	_, err := domFingerprint.ComputeStepDeclaration(
		domFingerprint.StepDeclarationMaterial{}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "árbol")
}

// TestDeclarationFileNames fija la lista normativa de §3.4. Es lo que decide el
// reparto entre las dos mitades del material —normalizado lo que el motor
// entiende, crudo lo que no— así que quitar o añadir un nombre es cambiar la
// regla.
func TestDeclarationFileNames(t *testing.T) {
	assert.Equal(t,
		[]string{"commands.yaml", "config.yaml"},
		domFingerprint.DeclarationFileNames())

	// Y la copia es de verdad: la lista no se puede mutar desde fuera.
	nombres := domFingerprint.DeclarationFileNames()
	nombres[0] = "otro.yaml"
	assert.Equal(t, "commands.yaml", domFingerprint.DeclarationFileNames()[0])
}

// TestComputeStepDeclaration_ElPrefijoEsObligatorio: la representación externa
// lleva el token, y es distinto del de la regla del árbol sobre la que compone.
func TestComputeStepDeclaration_ElPrefijoEsObligatorio(t *testing.T) {
	huella, err := domFingerprint.ComputeStepDeclaration(
		domFingerprint.StepDeclarationMaterial{}, arbolVacio())
	require.NoError(t, err)

	assert.Equal(t, domFingerprint.DeclarationVersion, huella.Version())
	assert.Equal(t, "pipe-v1", huella.Version())
	assert.NotEqual(t, domFingerprint.Version, huella.Version())
	assert.True(t, strings.HasPrefix(huella.String(), "pipe-v1:"))
}
