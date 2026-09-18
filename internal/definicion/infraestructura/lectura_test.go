package infraestructura

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
)

func escribir(t *testing.T, raiz, ruta, contenido string) {
	t.Helper()
	camino := filepath.Join(raiz, filepath.FromSlash(ruta))
	require.NoError(t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(t, os.WriteFile(camino, []byte(contenido), 0o644))
}

func TestLeerLosFicherosDelPipeline(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, "config.yaml", "schema_version: 1\nsteps:\n  infra:\n    rules: []\n    max_age: 24h\n    scope: shared\n")
	escribir(t, raiz, "environments.yaml", "- name: sandbox\n  description: pruebas\n  value: sand\n")
	escribir(t, raiz, "README.md", "no es del formato, y no se lee\n")
	escribir(t, raiz, "steps/01-infra/commands.yaml", `
- name: plan
  cmd: terraform plan
  workdir: ./terraform
  templates: [terraform.tfvars]
  outputs:
    - name: grupo
      description: el grupo
      probe: grupo = "(.+)"
      scope: shared
    - probe: Apply complete
`)
	escribir(t, raiz, "steps/01-infra/terraform/terraform.tfvars", "nombre = \"${var.project_name}\"\n")
	escribir(t, raiz, "steps/01-infra/terraform/commands.yaml", "un fichero más del material\n")
	escribir(t, raiz, "steps/01-infra/esperar.sh", "#!/bin/sh\n")
	require.NoError(t, os.Chmod(filepath.Join(raiz, "steps/01-infra/esperar.sh"), 0o755))
	require.NoError(t, os.Symlink("terraform/terraform.tfvars", filepath.Join(raiz, "steps/01-infra/enlace")))
	escribir(t, raiz, "variables/compartidas.yaml", "- name: replicas\n  value: 2\n")
	escribir(t, raiz, "variables/sand/lo-que-sea.yaml", "- name: grupo\n  value: ${var.project_name}-g\n")

	d, err := leer(raiz)
	require.NoError(t, err)

	require.True(t, d.Configuracion.Existe)
	require.Equal(t, "1", *d.Configuracion.Datos.Version)
	require.True(t, d.Ambientes.Existe)
	require.Equal(t, []dominio.AmbienteDeclarado{{Nombre: "sandbox", Descripcion: "pruebas", Valor: "sand"}}, d.Ambientes.Datos)
	require.Empty(t, d.Ilegibles)
	require.Empty(t, d.Desconocidos)

	require.Len(t, d.Pasos, 1)
	paso := d.Pasos[0]
	require.Equal(t, "01-infra", paso.Directorio)
	require.Equal(t, dominio.Declarado[[]dominio.ComandoDeclarado]{Existe: true, Datos: []dominio.ComandoDeclarado{{
		Nombre: "plan", Linea: "terraform plan", Directorio: "./terraform",
		Plantillas: []string{"terraform.tfvars"},
		Variables: []dominio.VariableDeComandoDeclarada{
			{Nombre: "grupo", Descripcion: "el grupo", Expresion: `grupo = "(.+)"`, Ambito: "shared"},
			{Expresion: "Apply complete"},
		},
	}}}, paso.Comandos)
	require.Equal(t, map[string]dominio.ConfiguracionDePasoDeclarada{
		"infra": {Reglas: []string{}, ReglasEscritas: true, EdadMaxima: "24h", Ambito: "shared"},
	}, d.Configuracion.Datos.Pasos, "rules: [] está escrito, y steps se lee de config.yaml")
	require.Equal(t, []dominio.FicheroDeclarado{
		{Ruta: "enlace", Enlace: "terraform/terraform.tfvars"},
		{Ruta: "esperar.sh", Contenido: "#!/bin/sh\n", Ejecutable: true},
		{Ruta: "terraform/commands.yaml", Contenido: "un fichero más del material\n"},
		{Ruta: "terraform/terraform.tfvars", Contenido: "nombre = \"${var.project_name}\"\n"},
	}, paso.Material, "todo el directorio del paso salvo su commands.yaml")

	dos, grupo := "2", "${var.project_name}-g"
	require.Equal(t, []dominio.VariablesDePipelineDeclarada{
		{Fichero: "variables/compartidas.yaml", Variables: []dominio.VariableDePipelineDeclarada{{Nombre: "replicas", Valor: &dos}}},
		{Fichero: "variables/sand/lo-que-sea.yaml", Ambito: "sand", Variables: []dominio.VariableDePipelineDeclarada{
			{Nombre: "grupo", Valor: &grupo},
		}},
	}, d.Variables, "el directorio es el ámbito, y el nombre del fichero solo organiza")
}

func TestLoQueNoSeLeeNoSeDecideAlLeer(t *testing.T) {
	raiz := t.TempDir()
	escribir(t, raiz, "config.yaml", "schema_version: 2\nclone_window: 24h\nsteps:\n  test:\n    scope: project\n")
	escribir(t, raiz, "steps/01-test/commands.yaml", "- cmd: mvn verify\n  retries: 3\n")
	escribir(t, raiz, "steps/02-vacio/commands.yaml", "")
	escribir(t, raiz, "steps/notas.md", "")
	escribir(t, raiz, "variables/sand/test.yml", "")
	escribir(t, raiz, "variables/sand/extra/test.yaml", "")

	d, err := leer(raiz)
	require.NoError(t, err)

	require.Equal(t, "2", *d.Configuracion.Datos.Version, "una versión distinta dice cuál es, aunque traiga claves que ya no existen")
	require.Empty(t, d.Configuracion.Datos.Pasos, "de una versión distinta no se lee ni siquiera steps")
	require.False(t, d.Ambientes.Existe)
	require.Len(t, d.Pasos, 2)
	require.True(t, d.Pasos[1].Comandos.Existe, "un commands.yaml vacío existe, solo que no declara nada")
	require.Empty(t, d.Pasos[1].Comandos.Datos, "un commands.yaml vacío no declara nada")
	require.ElementsMatch(t, []string{"steps/01-test/commands.yaml"}, ficheros(d.Ilegibles))
	require.Equal(t, []string{"steps/notas.md", "variables/sand/extra/", "variables/sand/test.yml"}, d.Desconocidos)
}

func ficheros(ilegibles []dominio.FicheroIlegibleDeclarado) []string {
	var lista []string
	for _, i := range ilegibles {
		lista = append(lista, i.Fichero)
	}
	return lista
}
