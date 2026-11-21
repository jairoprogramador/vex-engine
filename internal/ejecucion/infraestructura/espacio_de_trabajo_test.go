package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

var ub = dominio.Ubicacion{Proyecto: "proy", Pipeline: "pipe", Ambiente: "prod"}

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

	err = e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso})
	require.NoError(t, err)

	contenido, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "contenido", string(contenido))
}

func TestEspacioDeTrabajo_RehacerParteDelMotorLaDejaLimpiaCadaVez(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	f1, err := dominio.NuevoFicheroDeclarado("a.txt", "uno", false, "", false)
	require.NoError(t, err)
	paso1 := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f1})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso1}))

	f2, err := dominio.NuevoFicheroDeclarado("b.txt", "dos", false, "", false)
	require.NoError(t, err)
	paso2 := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f2})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso2}))

	_, err = os.Stat(filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "a.txt"))
	require.True(t, os.IsNotExist(err), "el fichero del intento anterior no debería sobrevivir")
	_, err = os.Stat(filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "b.txt"))
	require.NoError(t, err)
}

func TestEspacioDeTrabajo_NoTocaLoQueNoEsDeUnPaso(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)

	ajeno := filepath.Join(raiz, "proy", "pipe", "prod", "tecnologia")
	require.NoError(t, os.MkdirAll(ajeno, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ajeno, "state.tfstate"), []byte("estado"), 0o644))

	f, err := dominio.NuevoFicheroDeclarado("a.txt", "contenido", false, "", false)
	require.NoError(t, err)
	paso := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{f})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso}))

	contenido, err := os.ReadFile(filepath.Join(ajeno, "state.tfstate"))
	require.NoError(t, err)
	require.Equal(t, "estado", string(contenido))
}

func TestEspacioDeTrabajo_LaRutaEsProyectoPipelineAmbientePaso(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	require.Equal(t, filepath.Join(raiz, "proy", "pipe", "prod", "01-pruebas"), e.DirectorioDelPaso(ub, "01-pruebas"))
}

func TestEspacioDeTrabajo_UbicarUsaElOrigenYSiNoElNombreDelDirectorio(t *testing.T) {
	e := infraestructura.NuevoEspacioDeTrabajo(t.TempDir())

	conOrigen := t.TempDir()
	repo, err := git.PlainInit(conOrigen, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://GitHub.com/org/repo.git"}})
	require.NoError(t, err)
	esperado, err := dominio.NombreDeDirectorio("github.com/org/repo")
	require.NoError(t, err)

	u, err := e.Ubicar(conOrigen, conOrigen, "prod")
	require.NoError(t, err)
	require.Equal(t, esperado, u.Proyecto)
	require.Equal(t, "prod", u.Ambiente)

	sinOrigen := filepath.Join(t.TempDir(), "mi-proyecto")
	require.NoError(t, os.Mkdir(sinOrigen, 0o755))
	esperado, err = dominio.NombreDeDirectorio("mi-proyecto")
	require.NoError(t, err)
	u, err = e.Ubicar(sinOrigen, conOrigen, "prod")
	require.NoError(t, err)
	require.Equal(t, esperado, u.Proyecto)

	_, err = e.Ubicar("", conOrigen, "prod")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestEspacioDeTrabajo_InterpolarPlantillasSoloReescribeLasMarcadas(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	plantilla, err := dominio.NuevoFicheroDeclarado("plantilla.txt", "hola ${var.nombre}", false, "", true)
	require.NoError(t, err)
	normal, err := dominio.NuevoFicheroDeclarado("normal.txt", "hola ${var.nombre}", false, "", false)
	require.NoError(t, err)
	paso := pasoDeEjecucionConMaterial(t, "01-pruebas", []dominio.FicheroDeclarado{plantilla, normal})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso}))

	interpolar := func(texto string) (string, error) { return "hola mundo", nil }
	require.NoError(t, e.InterpolarPlantillas(context.Background(), ub, paso, interpolar))

	contenidoPlantilla, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "plantilla.txt"))
	require.NoError(t, err)
	require.Equal(t, "hola mundo", string(contenidoPlantilla))

	contenidoNormal, err := os.ReadFile(filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "normal.txt"))
	require.NoError(t, err)
	require.Equal(t, "hola ${var.nombre}", string(contenidoNormal))
}

// --- un paso no le deja nada al directorio de otro ---

func ficheroDeclarado(t *testing.T, ruta, contenido string, plantilla bool) dominio.FicheroDeclarado {
	t.Helper()
	f, err := dominio.NuevoFicheroDeclarado(ruta, contenido, false, "", plantilla)
	require.NoError(t, err)
	return f
}

func TestEspacioDeTrabajo_RehacerElPasoBorraLoQueOtroPasoDejoEnSuDirectorio(t *testing.T) {
	e := infraestructura.NuevoEspacioDeTrabajo(t.TempDir())
	primero := pasoDeEjecucionConMaterial(t, "01-pruebas", nil)
	segundo := pasoDeEjecucionConMaterial(t, "02-despliegue", []dominio.FicheroDeclarado{ficheroDeclarado(t, "main.tf", "declarado", false)})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{primero, segundo}))
	// Lo que haría un comando hostil del primer paso: cambiar el material del segundo y dejarle un fichero.
	directorioDelSegundo := e.DirectorioDelPaso(ub, "02-despliegue")
	require.NoError(t, os.WriteFile(filepath.Join(directorioDelSegundo, "main.tf"), []byte("cambiado"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(directorioDelSegundo, "override.tf"), []byte("plantado"), 0o644))

	require.NoError(t, e.RehacerElPaso(context.Background(), ub, segundo))

	contenido, err := os.ReadFile(filepath.Join(directorioDelSegundo, "main.tf"))
	require.NoError(t, err)
	require.Equal(t, "declarado", string(contenido))
	_, err = os.Lstat(filepath.Join(directorioDelSegundo, "override.tf"))
	require.True(t, os.IsNotExist(err), "lo plantado no llega a los comandos del paso")
}

func TestEspacioDeTrabajo_RehacerElPasoNoSigueUnEnlacePuestoEnSuLugar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear enlaces simbólicos en Windows pide privilegios")
	}
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	paso := pasoDeEjecucionConMaterial(t, "02-despliegue", []dominio.FicheroDeclarado{ficheroDeclarado(t, "main.tf", "declarado", false)})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso}))
	// El directorio del paso, cambiado por un enlace a un sitio con cosas que no son del paso.
	ajeno := filepath.Join(raiz, "ajeno")
	require.NoError(t, os.Mkdir(ajeno, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ajeno, "dato"), []byte("no se toca"), 0o644))
	directorioDelPaso := e.DirectorioDelPaso(ub, "02-despliegue")
	require.NoError(t, os.RemoveAll(directorioDelPaso))
	require.NoError(t, os.Symlink(ajeno, directorioDelPaso))

	require.NoError(t, e.RehacerElPaso(context.Background(), ub, paso))

	info, err := os.Lstat(directorioDelPaso)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "el directorio del paso vuelve a ser un directorio de verdad")
	entradas, err := os.ReadDir(ajeno)
	require.NoError(t, err)
	require.Len(t, entradas, 1, "en lo que apuntaba el enlace no se borró ni se escribió nada")
	require.Equal(t, "dato", entradas[0].Name())
}

func TestEspacioDeTrabajo_SiElDirectorioDelAmbienteEsUnEnlaceNoSeTrabajaAhi(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear enlaces simbólicos en Windows pide privilegios")
	}
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	paso := pasoDeEjecucionConMaterial(t, "02-despliegue", []dominio.FicheroDeclarado{ficheroDeclarado(t, "main.tf", "declarado", false)})
	otroSitio := filepath.Join(raiz, "otro-sitio")
	require.NoError(t, os.Mkdir(otroSitio, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(raiz, ub.Proyecto, ub.Pipeline), 0o755))
	require.NoError(t, os.Symlink(otroSitio, filepath.Join(raiz, ub.Proyecto, ub.Pipeline, ub.Ambiente)))

	err := e.RehacerElPaso(context.Background(), ub, paso)

	require.ErrorIs(t, err, dominio.ErrNoDisponible)
	require.ErrorContains(t, err, "enlace")
	entradas, errLeer := os.ReadDir(otroSitio)
	require.NoError(t, errLeer)
	require.Empty(t, entradas, "no se creó nada a través del enlace")
}

func TestEspacioDeTrabajo_InterpolarPlantillasNoLeeNiEscribeATravesDeUnEnlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear enlaces simbólicos en Windows pide privilegios")
	}
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	paso := pasoDeEjecucionConMaterial(t, "02-despliegue", []dominio.FicheroDeclarado{ficheroDeclarado(t, "backend.tf", "clave=${var.clave}", true)})
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{paso}))
	// La plantilla, cambiada por un enlace a un fichero de fuera: con os.WriteFile el valor interpolado acabaría ahí.
	victima := filepath.Join(raiz, "victima")
	require.NoError(t, os.WriteFile(victima, []byte("otra plantilla: ${var.otra}"), 0o644))
	plantilla := filepath.Join(e.DirectorioDelPaso(ub, "02-despliegue"), "backend.tf")
	require.NoError(t, os.Remove(plantilla))
	require.NoError(t, os.Symlink(victima, plantilla))
	var interpolados []string
	interpolar := func(texto string) (string, error) {
		interpolados = append(interpolados, texto)
		return "clave=secreta", nil
	}

	require.NoError(t, e.InterpolarPlantillas(context.Background(), ub, paso, interpolar))

	require.Equal(t, []string{"clave=${var.clave}"}, interpolados, "se interpola lo declarado, no lo que haya en el disco")
	contenidoDeLaVictima, err := os.ReadFile(victima)
	require.NoError(t, err)
	require.Equal(t, "otra plantilla: ${var.otra}", string(contenidoDeLaVictima), "el fichero de fuera queda como estaba")
	info, err := os.Lstat(plantilla)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "la plantilla vuelve a ser un fichero de verdad")
	contenido, err := os.ReadFile(plantilla)
	require.NoError(t, err)
	require.Equal(t, "clave=secreta", string(contenido))
}

func TestEspacioDeTrabajo_RehacerElPasoNoTocaLoQueLosPasosComparten(t *testing.T) {
	raiz := t.TempDir()
	e := infraestructura.NuevoEspacioDeTrabajo(raiz)
	primero := pasoDeEjecucionConMaterial(t, "01-pruebas", nil)
	segundo := pasoDeEjecucionConMaterial(t, "02-despliegue", nil)
	require.NoError(t, e.RehacerParteDelMotor(context.Background(), ub, []dominio.PasoDeEjecucion{primero, segundo}))
	// Lo que un paso deja a propósito para los siguientes: en el ambiente, fuera de los directorios de los pasos,
	// y en su propio directorio.
	compartido := filepath.Join(raiz, ub.Proyecto, ub.Pipeline, ub.Ambiente, "terraform.tfstate")
	require.NoError(t, os.WriteFile(compartido, []byte("estado"), 0o644))
	delPrimero := filepath.Join(e.DirectorioDelPaso(ub, "01-pruebas"), "informe.txt")
	require.NoError(t, os.WriteFile(delPrimero, []byte("informe"), 0o644))

	require.NoError(t, e.RehacerElPaso(context.Background(), ub, segundo))

	require.FileExists(t, compartido)
	require.FileExists(t, delPrimero, "lo que un paso dejó en su propio directorio sigue ahí para los siguientes")
}
