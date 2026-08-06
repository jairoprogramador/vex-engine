package fingerprint_test

// Vectores y sensibilidades de la huella de instrucciones (SPEC-INSTRUCTIONS-v1.md).
//
// La tabla de §6 de la especificación es la transcripción de lo que este archivo
// afirma: si los dos discrepan, discrepan los dos.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

func instruccionBase() fingerprint.InstructionMaterial {
	return fingerprint.InstructionMaterial{Name: "build", Cmd: "mvn package"}
}

func huellaInst(t *testing.T, commands ...fingerprint.InstructionMaterial) string {
	t.Helper()
	huella, err := fingerprint.ComputeInstructions(commands)
	require.NoError(t, err)
	return huella.String()
}

// --- §6 vectores ------------------------------------------------------------

func TestComputeInstructions_Vectores(t *testing.T) {
	conWorkdir := instruccionBase()
	conWorkdir.Workdir = "app"

	conShow := instruccionBase()
	conShow.Show = true

	conPlantillas := instruccionBase()
	conPlantillas.Templates = []string{"a.yaml", "b.yaml"}

	conOutputs := instruccionBase()
	conOutputs.Outputs = []fingerprint.OutputMaterial{{Name: "artefacto", Probe: `version: (\S+)`}}

	a := fingerprint.InstructionMaterial{Name: "a", Cmd: "echo a"}
	b := fingerprint.InstructionMaterial{Name: "b", Cmd: "echo b"}

	casos := []struct {
		numero   int
		commands []fingerprint.InstructionMaterial
		huella   string
	}{
		{1, nil,
			"inst-v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{2, []fingerprint.InstructionMaterial{instruccionBase()},
			"inst-v1:6957cd1bfdfa202bc93165b6e2d627ce8543b2ce119b03d89c8043165356abd8"},
		{3, []fingerprint.InstructionMaterial{conWorkdir},
			"inst-v1:2e85db12dbba7e1f2f6f04afefe0093c22b81956c2b62470239663464f32f0f4"},
		{4, []fingerprint.InstructionMaterial{conShow},
			"inst-v1:0cf50eb19529cd6cc6e884e0ebd239d593cc788fdeaae9b635642b1d0179dc8d"},
		{5, []fingerprint.InstructionMaterial{conPlantillas},
			"inst-v1:c0ed247e9890e0606b5a61a0549f0adb2a977d8e50506eda3ae7b8576c189593"},
		{6, []fingerprint.InstructionMaterial{conOutputs},
			"inst-v1:ea1c8f0b489b3ae8ef2634b4cccf8a5389cad346db2fa059d1b885535f6dcb8f"},
		{7, []fingerprint.InstructionMaterial{a, b},
			"inst-v1:ad02cacda4192098d6dd576f1e1cc57bc9e551ea7a8cd15347a330da005cc3a8"},
		{8, []fingerprint.InstructionMaterial{b, a},
			"inst-v1:87adcfbf395b6f75f1665b801627145486048a4162672388a75d8b753e1dc258"},
	}

	for _, caso := range casos {
		t.Run(fmt.Sprintf("vector %d", caso.numero), func(t *testing.T) {
			assert.Equal(t, caso.huella, huellaInst(t, caso.commands...),
				"vector %d de SPEC-INSTRUCTIONS-v1.md §6", caso.numero)
		})
	}
}

// La lista vacía es la huella de la cadena vacía (§3.4).
func TestComputeInstructions_SinComandosEsLaHuellaDeLaCadenaVacia(t *testing.T) {
	assert.Equal(t,
		"inst-v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		huellaInst(t))
}

// --- §7.2 sensibilidades ----------------------------------------------------

func TestComputeInstructions_Sensibilidades(t *testing.T) {
	base := huellaInst(t, instruccionBase())

	mutaciones := map[string]func(*fingerprint.InstructionMaterial){
		"name":      func(c *fingerprint.InstructionMaterial) { c.Name = "compilar" },
		"cmd":       func(c *fingerprint.InstructionMaterial) { c.Cmd = "mvn verify" },
		"workdir":   func(c *fingerprint.InstructionMaterial) { c.Workdir = "app" },
		"templates": func(c *fingerprint.InstructionMaterial) { c.Templates = []string{"k8s/deployment.yaml"} },
		"outputs.name": func(c *fingerprint.InstructionMaterial) {
			c.Outputs = []fingerprint.OutputMaterial{{Name: "art", Probe: "p"}}
		},
		"outputs.probe": func(c *fingerprint.InstructionMaterial) {
			c.Outputs = []fingerprint.OutputMaterial{{Name: "", Probe: "p"}}
		},
		"show (§3.5)": func(c *fingerprint.InstructionMaterial) { c.Show = true },
	}

	for nombre, mutar := range mutaciones {
		t.Run(nombre, func(t *testing.T) {
			comando := instruccionBase()
			mutar(&comando)
			assert.NotEqual(t, base, huellaInst(t, comando))
		})
	}

	t.Run("control: sin mutar, la misma declaración da la misma huella", func(t *testing.T) {
		assert.Equal(t, base, huellaInst(t, instruccionBase()))
	})
}

// `show` es la corrección de la spec 10 §5.1bis, y merece su propio test: hasta
// aquí el test de la spec 00 afirmaba lo CONTRARIO —que añadir `show: true` no
// invalidaba el caché— y se borró al implementar la 10.
func TestComputeInstructions_ShowEntraEnElMaterial(t *testing.T) {
	sinShow := instruccionBase()
	conShow := instruccionBase()
	conShow.Show = true

	assert.NotEqual(t, huellaInst(t, sinShow), huellaInst(t, conShow),
		"añadir `show: true` para depurar tiene que invalidar el caché: si no, el "+
			"paso se salta y no se imprime nada")
}

// El orden de los comandos es la secuencia de ejecución, no un detalle (§3.3).
func TestComputeInstructions_ElOrdenDeLosComandosImporta(t *testing.T) {
	a := fingerprint.InstructionMaterial{Name: "a", Cmd: "echo a"}
	b := fingerprint.InstructionMaterial{Name: "b", Cmd: "echo b"}

	assert.NotEqual(t, huellaInst(t, a, b), huellaInst(t, b, a))
}

func TestComputeInstructions_ElOrdenDeLasPlantillasImporta(t *testing.T) {
	uno := instruccionBase()
	uno.Templates = []string{"a.yaml", "b.yaml"}
	otro := instruccionBase()
	otro.Templates = []string{"b.yaml", "a.yaml"}

	assert.NotEqual(t, huellaInst(t, uno), huellaInst(t, otro),
		"reordenar plantillas cambia el orden de interpolación")
}

func TestComputeInstructions_ElOrdenDeLosOutputsImporta(t *testing.T) {
	uno := instruccionBase()
	uno.Outputs = []fingerprint.OutputMaterial{{Name: "a", Probe: "pa"}, {Name: "b", Probe: "pb"}}
	otro := instruccionBase()
	otro.Outputs = []fingerprint.OutputMaterial{{Name: "b", Probe: "pb"}, {Name: "a", Probe: "pa"}}

	assert.NotEqual(t, huellaInst(t, uno), huellaInst(t, otro))
}

// La cardinalidad delante de cada lista impide que dos repartos distintos del
// mismo total produzcan la misma cadena (§3.2).
func TestComputeInstructions_LaCardinalidadSeparaLosRepartos(t *testing.T) {
	dosPlantillas := instruccionBase()
	dosPlantillas.Templates = []string{"x", "y"}

	unaYUnOutput := instruccionBase()
	unaYUnOutput.Templates = []string{"x"}
	unaYUnOutput.Outputs = []fingerprint.OutputMaterial{{Name: "y", Probe: ""}}

	assert.NotEqual(t, huellaInst(t, dosPlantillas), huellaInst(t, unaYUnOutput))
}

// Los campos entrecomillados hacen la regla inyectiva: un valor no puede
// simular el separador ni el salto de línea (§3.2).
func TestComputeInstructions_UnValorNoPuedeSimularElSeparador(t *testing.T) {
	conSeparador := fingerprint.InstructionMaterial{Name: "a\x1emvn package", Cmd: ""}
	partido := fingerprint.InstructionMaterial{Name: "a", Cmd: "mvn package"}

	assert.NotEqual(t, huellaInst(t, conSeparador), huellaInst(t, partido))
}

func TestComputeInstructions_UnValorNoPuedeSimularElSaltoDeLinea(t *testing.T) {
	unSoloComando := fingerprint.InstructionMaterial{Name: "a\nb", Cmd: "echo"}
	dosComandos := []fingerprint.InstructionMaterial{
		{Name: "a", Cmd: "echo"},
		{Name: "b", Cmd: "echo"},
	}

	assert.NotEqual(t, huellaInst(t, unSoloComando), huellaInst(t, dosComandos...))
}

// La huella lleva su propio token de versión, distinto del de la huella del
// árbol: las dos reglas tienen que poder saltar de versión por separado.
func TestComputeInstructions_LlevaSuPropioTokenDeVersion(t *testing.T) {
	huella, err := fingerprint.ComputeInstructions(nil)
	require.NoError(t, err)

	assert.Equal(t, fingerprint.InstructionsVersion, huella.Version())
	assert.NotEqual(t, fingerprint.Version, huella.Version())
	assert.Regexp(t, `^inst-v1:[0-9a-f]{64}$`, huella.String())
}
