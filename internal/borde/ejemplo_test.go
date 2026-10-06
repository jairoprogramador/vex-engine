package borde_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Las pruebas de punta a punta no conocen los pasos del pipeline de ejemplo: los leen del disco. Así el ejemplo
// puede cambiar — renombrar pasos, añadir o quitar — sin tocar las pruebas.

var raizDelEjemplo = filepath.Join("..", "ejecucion", "testdata", "ejemplo")

var prefijoDeOrden = regexp.MustCompile(`^\d{2}-`)

type comandoDelEjemplo struct {
	Workdir   string   `yaml:"workdir"`
	Templates []string `yaml:"templates"`
	Outputs   []struct {
		Name string `yaml:"name"`
	} `yaml:"outputs"`
}

// pasoDelEjemplo es un paso de steps/NN-nombre. Nombre no lleva el prefijo NN-: es el que ve el Historial.
type pasoDelEjemplo struct {
	Nombre     string
	Directorio string
	Comandos   []comandoDelEjemplo
	Reglas     []string // las escritas en config.yaml; vacío = las tres por defecto
}

func (p pasoDelEjemplo) mira(regla string) bool {
	return len(p.Reglas) == 0 || slices.Contains(p.Reglas, regla)
}

// plantilla es el primer fichero de plantilla que declara, con su ruta en el pipeline, o vacío si no tiene.
func (p pasoDelEjemplo) plantilla() string {
	for _, c := range p.Comandos {
		if len(c.Templates) > 0 {
			return filepath.ToSlash(filepath.Join("steps", filepath.Base(p.Directorio), c.Workdir, c.Templates[0]))
		}
	}
	return ""
}

func (p pasoDelEjemplo) productos() []string {
	var nombres []string
	for _, c := range p.Comandos {
		for _, o := range c.Outputs {
			nombres = append(nombres, o.Name)
		}
	}
	return nombres
}

func leerYaml(t *testing.T, ruta string, destino any) {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(datos, destino), ruta)
}

// pasosDelEjemplo lee los pasos en su orden, con las reglas de config.yaml.
func pasosDelEjemplo(t *testing.T) []pasoDelEjemplo {
	t.Helper()
	var config struct {
		Steps map[string]struct {
			Rules []string `yaml:"rules"`
		} `yaml:"steps"`
	}
	leerYaml(t, filepath.Join(raizDelEjemplo, "config.yaml"), &config)

	directorios, err := filepath.Glob(filepath.Join(raizDelEjemplo, "steps", "*"))
	require.NoError(t, err)
	sort.Strings(directorios)

	var pasos []pasoDelEjemplo
	for _, dir := range directorios {
		nombre := prefijoDeOrden.ReplaceAllString(filepath.Base(dir), "")
		paso := pasoDelEjemplo{Nombre: nombre, Directorio: dir, Reglas: config.Steps[nombre].Rules}
		if _, err := os.Stat(filepath.Join(dir, "commands.yaml")); err == nil {
			leerYaml(t, filepath.Join(dir, "commands.yaml"), &paso.Comandos)
		}
		pasos = append(pasos, paso)
	}
	require.NotEmpty(t, pasos, "el pipeline de ejemplo no tiene pasos")
	return pasos
}

func nombresDeLosPasos(pasos []pasoDelEjemplo) []string {
	nombres := make([]string, 0, len(pasos))
	for _, p := range pasos {
		nombres = append(nombres, p.Nombre)
	}
	return nombres
}

func estadosPorPaso(d ejecucionpublicado.Detalle) map[string]string {
	estados := make(map[string]string, len(d.Pasos))
	for _, p := range d.Pasos {
		estados[p.Nombre] = p.Estado
	}
	return estados
}

func nombresDelDetalle(d ejecucionpublicado.Detalle) []string {
	nombres := make([]string, 0, len(d.Pasos))
	for _, p := range d.Pasos {
		nombres = append(nombres, p.Nombre)
	}
	return nombres
}
