package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

func pasoDeEjecucionConMaterial(t *testing.T, nombre string, material []dominio.FicheroDeclarado) dominio.PasoDeEjecucion {
	t.Helper()
	base, err := dominio.NuevoPasoDelPipeline(nombre, false)
	require.NoError(t, err)
	return dominio.PasoDeEjecucion{PasoDelPipeline: base, Material: material}
}

func TestEspacioDeTrabajo_RehacerParteDelMotorEscribeElMaterialDeCadaPaso(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	fichero, err := dominio.NuevoFicheroDeclarado("a.txt", "contenido", false, "", false)
	require.NoError(t, err)
	paso := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{fichero})

	err = e.RehacerParteDelMotor(context.Background(), "prod", []dominio.PasoDeEjecucion{paso})
	require.NoError(t, err)

	contenido, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso("prod", "01-pruebas"), "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "contenido", string(contenido))
}

func TestEspacioDeTrabajo_RehacerParteDelMotorLaDejaLimpiaCadaVez(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	f1, err := dominio.NuevoFicheroDeclarado("a.txt", "uno", false, "", false)
	require.NoError(t, err)
	paso1 := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f1})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), "prod", []dominio.PasoDeEjecucion{paso1}))

	f2, err := dominio.NuevoFicheroDeclarado("b.txt", "dos", false, "", false)
	require.NoError(t, err)
	paso2 := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f2})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), "prod", []dominio.PasoDeEjecucion{paso2}))

	_, err = os.Stat(filepath.Join(e.DirectorioDelPaso("prod", "01-pruebas"), "a.txt"))
	require.True(t, os.IsNotExist(err), "el fichero del intento anterior no debería sobrevivir")
	_, err = os.Stat(filepath.Join(e.DirectorioDelPaso("prod", "01-pruebas"), "b.txt"))
	require.NoError(t, err)
}

func TestEspacioDeTrabajo_NuncaTocaLaParteDeLaTecnologia(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)

	tecnologia := filepath.Join(raiz, "prod", "tecnologia")
	require.NoError(t, os.MkdirAll(tecnologia, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tecnologia, "state.tfstate"), []byte("estado"), 0o644))

	f, err := dominio.NuevoFicheroDeclarado("a.txt", "contenido", false, "", false)
	require.NoError(t, err)
	paso := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), "prod", []dominio.PasoDeEjecucion{paso}))

	contenido, err := os.ReadFile(filepath.Join(tecnologia, "state.tfstate"))
	require.NoError(t, err)
	require.Equal(t, "estado", string(contenido))
}

func TestEspacioDeTrabajo_InterpolarPlantillasSoloReescribeLasMarcadas(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	plantilla, err := dominio.NuevoFicheroDeclarado("plantilla.txt", "hola ${var.nombre}", false, "", true)
	require.NoError(t, err)
	normal, err := dominio.NuevoFicheroDeclarado("normal.txt", "hola ${var.nombre}", false, "", false)
	require.NoError(t, err)
	paso := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{plantilla, normal})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), "prod", []dominio.PasoDeEjecucion{paso}))

	interpolar := func(texto string) (string, error) { return "hola mundo", nil }
	require.NoError(t, e.InterpolarPlantillas(context.Background(), "prod", paso, interpolar))

	contenidoPlantilla, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso("prod", "01-pruebas"), "plantilla.txt"))
	require.NoError(t, err)
	require.Equal(t, "hola mundo", string(contenidoPlantilla))

	contenidoNormal, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso("prod", "01-pruebas"), "normal.txt"))
	require.NoError(t, err)
	require.Equal(t, "hola ${var.nombre}", string(contenidoNormal))
}
