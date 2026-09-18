package infraestructura

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
)

// El pipeline de ejemplo de Definición (testdata/ejemplo/README.md): se lee del disco, pasa la comprobación y,
// en una copia, se rompe fichero a fichero para ver cada fallo pasando por la lectura y por la comprobación.

var ejemplo = filepath.Join("..", "testdata", "ejemplo")

// compartido da un *dominio.Ambito con el valor Compartido (RD-04 §9.19).
func compartido() *dominio.Ambito {
	c := dominio.Compartido
	return &c
}

// copiarEjemplo pone una copia del ejemplo en un directorio temporal, para romperla sin tocar el original.
func copiarEjemplo(t *testing.T) string {
	t.Helper()
	destino := t.TempDir()
	require.NoError(t, os.CopyFS(destino, os.DirFS(ejemplo)))
	return destino
}

func comprobarDirectorio(t *testing.T, raiz string) (*dominio.PipelineComprobado, error) {
	t.Helper()
	d, err := leer(raiz)
	require.NoError(t, err, "leer solo falla si no puede leer el disco")
	return dominio.Comprobar(d)
}

func TestElPipelineDeEjemploPasaLaComprobacion(t *testing.T) {
	p, err := comprobarDirectorio(t, ejemplo)
	require.NoError(t, err)

	require.Equal(t, "1", p.Version())
	var ambientes []string
	for _, a := range p.Ambientes() {
		ambientes = append(ambientes, a.Valor)
	}
	require.Equal(t, []string{"sand", "stag", "prod"}, ambientes, "en su orden")

	type resumen struct {
		Nombre     string
		Orden      int
		Reglas     []dominio.Regla
		EdadMaxima time.Duration
		Compartido bool
	}
	tres := []dominio.Regla{dominio.ReglaCodigo, dominio.ReglaInstrucciones, dominio.ReglaVariables}
	dos := []dominio.Regla{dominio.ReglaInstrucciones, dominio.ReglaVariables}
	var pasos []resumen
	for _, paso := range p.Pasos() {
		pasos = append(pasos, resumen{paso.Nombre, paso.Orden, paso.Reglas, paso.EdadMaxima, paso.Ambito != nil})
	}
	require.Equal(t, []resumen{
		{"pruebas", 1, tres, 720 * time.Hour, false},
		{"registro", 2, dos, 0, true},
		{"infraestructura", 3, dos, 0, false},
		{"imagen", 4, tres, 24 * time.Hour, false},
		{"despliegue", 5, tres, 0, false},
		{"aviso", 6, []dominio.Regla{dominio.ReglaInstrucciones}, 0, false},
	}, pasos, "registro es de ámbito compartido (config.yaml, steps.registro.scope: shared); los demás, del ambiente en que se ejecutan")

	paso := func(nombre string) dominio.PasoComprobado {
		t.Helper()
		encontrado, ok := p.Paso(nombre)
		require.True(t, ok, nombre)
		return encontrado
	}

	t.Run("el material, y cuál se interpola", func(t *testing.T) {
		plantillas := map[string]bool{}
		for _, nombre := range []string{"pruebas", "registro", "infraestructura", "imagen", "despliegue", "aviso"} {
			for _, f := range paso(nombre).Material {
				plantillas[nombre+"/"+f.Ruta] = f.Plantilla
			}
		}
		require.Equal(t, map[string]bool{
			"registro/terraform/main.tf":                        false,
			"infraestructura/terraform/terraform.tfvars":        true,
			"infraestructura/terraform/terraform.tfvars.sample": false,
			"imagen/docker/Dockerfile":                          true,
			"imagen/scripts/esperar.sh":                         false,
			"despliegue/k8s/deployment.yaml":                    true,
			"despliegue/k8s/service.yaml":                       true,
		}, plantillas, "commands.yaml no es material, y solo se interpola lo que está en templates")

		for _, f := range paso("registro").Material {
			require.Contains(t, f.Contenido, "${var.nombre}", "lo que no es plantilla se lee tal cual")
		}
	})

	t.Run("las variables son del ámbito, no del paso", func(t *testing.T) {
		compartidas := map[string]dominio.VariableDePipelineComprobada{}
		replicas := map[dominio.Ambito]string{}
		porAmbito := map[dominio.Ambito]int{}
		for _, v := range p.Variables() {
			porAmbito[v.Ambito]++
			if v.Ambito.EsCompartido() {
				compartidas[v.Nombre] = v
			}
			if v.Nombre == "replicas" {
				replicas[v.Ambito] = v.Valor
			}
		}
		require.Equal(t, map[string]dominio.VariableDePipelineComprobada{
			"registro_sku": {
				Nombre: "registro_sku", Descripcion: "el plan del registro, el mismo en todos los ambientes",
				Valor: "Basic",
			},
			"registro": {
				Nombre:      "registro",
				Descripcion: "otro nombre para una variable de salida compartida, que produce el paso registro",
				Valor:       "${var.registro_nombre}",
			},
			"servidor": {
				Nombre: "servidor", Descripcion: "el servidor del registro, que produce una variable de salida compartida",
				Valor: "${var.registro_servidor}",
			},
		}, compartidas, "las de la raíz de variables/, declaradas una sola vez")
		require.Equal(t, map[dominio.Ambito]string{"sand": "1", "stag": "2", "prod": "4"}, replicas)
		require.Equal(t, map[dominio.Ambito]int{dominio.Compartido: 3, "sand": 5, "stag": 5, "prod": 5}, porAmbito,
			"dos ficheros por ambiente, y todo paso de ese ambiente las ve")
	})

	t.Run("los comandos", func(t *testing.T) {
		comandos := paso("despliegue").Comandos
		require.Len(t, comandos, 3)
		require.Equal(t, "k8s", comandos[1].Directorio, "el workdir limpio")
		require.Equal(t, []string{"k8s/deployment.yaml", "k8s/service.yaml"}, comandos[1].Plantillas,
			"relativas al directorio del paso")
		require.Equal(t, []dominio.VariableDeComandoComprobada{{Nombre: "direccion", Expresion: `^(\d+\.\d+\.\d+\.\d+)$`}},
			comandos[2].VariablesDeSalida, "sin scope, del ámbito del ambiente en ejecución")
		require.Equal(t, compartido(), paso("registro").Comandos[1].VariablesDeSalida[0].Ambito,
			"sin scope propio, hereda el del paso: steps.registro.scope: shared en config.yaml")
	})

	t.Run("un outputs sin name es una aserción", func(t *testing.T) {
		comando := paso("pruebas").Comandos[0]
		require.Empty(t, comando.VariablesDeSalida, "no produce ninguna variable")
		require.Equal(t, []dominio.AsercionComprobada{{
			Descripcion: "una aserción; sin name no produce ninguna variable, y dice cuándo el comando fue bien",
			Expresion:   "BUILD SUCCESS",
		}}, comando.Aserciones)
	})
}

// Un cambio rompe la copia del ejemplo. Los que reemplazan exigen que el texto esté: si el ejemplo cambia y el
// cambio ya no se aplica, la prueba falla en vez de pasar sin haber roto nada.
type cambio func(t *testing.T, raiz string)

func escribirFichero(ruta, contenido string) cambio {
	return func(t *testing.T, raiz string) { escribir(t, raiz, ruta, contenido) }
}

func anadir(ruta, texto string) cambio {
	return func(t *testing.T, raiz string) {
		t.Helper()
		contenido, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(ruta)))
		require.NoError(t, err)
		escribir(t, raiz, ruta, string(contenido)+texto)
	}
}

func reemplazar(ruta, viejo, nuevo string) cambio {
	return func(t *testing.T, raiz string) {
		t.Helper()
		contenido, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(ruta)))
		require.NoError(t, err)
		require.Contains(t, string(contenido), viejo, "el cambio tiene que aplicarse a %s", ruta)
		escribir(t, raiz, ruta, strings.Replace(string(contenido), viejo, nuevo, 1))
	}
}

func borrar(ruta string) cambio {
	return func(t *testing.T, raiz string) {
		t.Helper()
		camino := filepath.Join(raiz, filepath.FromSlash(ruta))
		_, err := os.Lstat(camino)
		require.NoError(t, err, "lo que se borra tiene que estar")
		require.NoError(t, os.RemoveAll(camino))
	}
}

func renombrar(de, a string) cambio {
	return func(t *testing.T, raiz string) {
		t.Helper()
		require.NoError(t, os.Rename(filepath.Join(raiz, filepath.FromSlash(de)), filepath.Join(raiz, filepath.FromSlash(a))))
	}
}

func enlazar(ruta, destino string) cambio {
	return func(t *testing.T, raiz string) {
		t.Helper()
		require.NoError(t, os.Symlink(destino, filepath.Join(raiz, filepath.FromSlash(ruta))))
	}
}

func TestElEjemploRotoFallaPorCadaFilaDeLaComprobacion(t *testing.T) {
	casos := []struct {
		nombre     string
		cambios    []cambio
		invariante dominio.Invariante
		fichero    string
		ambiente   string
		detalle    string
		// varios: el fallo arrastra otros que son su consecuencia.
		varios bool
	}{
		// El formato.
		{
			nombre:     "una schema_version distinta de la que se lee, con claves que ya no existen",
			cambios:    []cambio{escribirFichero("config.yaml", "schema_version: 2\nclone_window: 24h\n")},
			invariante: dominio.Formato, fichero: "config.yaml",
			detalle: `schema_version "2" no se lee: la única que se lee es la 1`,
		},
		{
			nombre:     "sin config.yaml",
			cambios:    []cambio{borrar("config.yaml")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "no está, y el pipeline declara ahí su schema_version",
		},
		{
			nombre:     "la versión que se lee, con una clave que no existe",
			cambios:    []cambio{anadir("config.yaml", "clone_window: 24h\n")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "field clone_window not found",
			// Con steps ahora en config.yaml, una clave rota ahí arrastra la de todos los pasos: ninguno
			// tiene su configuración, y registro pierde su scope: shared (RD-04 §9.20).
			varios: true,
		},
		{
			nombre:     "un YAML que no se puede leer",
			cambios:    []cambio{anadir("config.yaml", "steps: [roto\n")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "no se puede leer como el formato pide",
		},
		{
			nombre: "las reglas de la versión 2",
			cambios: []cambio{reemplazar("config.yaml",
				"  infraestructura:\n    rules: [instructions, variables]",
				"  infraestructura:\n    rules:\n      - state_changed: [pipeline]")},
			invariante: dominio.Formato, fichero: "config.yaml",
			detalle: "no se puede leer como el formato pide", varios: true,
		},
		{
			nombre:     "una clave que commands.yaml no tiene",
			cambios:    []cambio{reemplazar("steps/04-imagen/commands.yaml", "- name: esperar\n", "- name: esperar\n  retries: 3\n")},
			invariante: dominio.Formato, fichero: "steps/04-imagen/commands.yaml", detalle: "field retries not found",
		},
		{
			nombre: "un scope de paso que no existe",
			cambios: []cambio{reemplazar("config.yaml",
				"  pruebas:\n    rules: [code, instructions, variables]\n    max_age: 720h\n",
				"  pruebas:\n    rules: [code, instructions, variables]\n    max_age: 720h\n    scope: project\n")},
			invariante: dominio.Formato, fichero: "config.yaml",
			detalle: `el paso tiene scope "project", que no existe: es environment o shared`,
		},
		{
			nombre: "una variable de salida con un scope desconocido",
			cambios: []cambio{reemplazar("steps/03-infraestructura/commands.yaml",
				"    - name: cluster_fqdn\n", "    - name: cluster_fqdn\n      scope: project\n")},
			invariante: dominio.Formato, fichero: "steps/03-infraestructura/commands.yaml",
			detalle: `produce "cluster_fqdn" con scope "project", que no existe`,
		},
		{
			nombre: "una regla desconocida",
			cambios: []cambio{reemplazar("config.yaml",
				"  infraestructura:\n    rules: [instructions, variables]", "  infraestructura:\n    rules: [pipeline]")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `la regla "pipeline" no existe`,
		},
		{
			nombre:     "una regla repetida",
			cambios:    []cambio{reemplazar("config.yaml", "[instructions]", "[instructions, instructions]")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `la regla "instructions" está dos veces`,
		},
		{
			nombre:     "un max_age en días",
			cambios:    []cambio{reemplazar("config.yaml", "max_age: 720h", "max_age: 30d")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `max_age "30d"`,
		},
		{
			nombre:     "un comando sin cmd",
			cambios:    []cambio{anadir("steps/01-pruebas/commands.yaml", "- name: nada\n")},
			invariante: dominio.Formato, fichero: "steps/01-pruebas/commands.yaml", detalle: "el comando 2 no dice cmd",
		},
		{
			nombre:     "un workdir que sale del paso",
			cambios:    []cambio{anadir("steps/04-imagen/commands.yaml", "- name: fuera\n  cmd: ls\n  workdir: ../..\n")},
			invariante: dominio.Formato, fichero: "steps/04-imagen/commands.yaml", detalle: `el comando 3 tiene workdir "../.."`,
		},
		{
			nombre:     "una plantilla que no está en el material",
			cambios:    []cambio{reemplazar("steps/05-despliegue/commands.yaml", "- service.yaml", "- ingress.yaml")},
			invariante: dominio.Formato, fichero: "steps/05-despliegue/commands.yaml", detalle: "k8s/ingress.yaml no está en el material",
		},
		{
			nombre:     "un enlace que sale del paso",
			cambios:    []cambio{enlazar("steps/04-imagen/fuera", "../../..")},
			invariante: dominio.Formato, fichero: "steps/04-imagen/fuera", detalle: "fuera del directorio del paso",
		},
		{
			nombre:     "un outputs sin name ni probe",
			cambios:    []cambio{anadir("steps/01-pruebas/commands.yaml", "    - description: nada\n")},
			invariante: dominio.Formato, fichero: "steps/01-pruebas/commands.yaml", detalle: "un outputs sin name ni probe",
		},
		{
			nombre:     "una aserción con scope",
			cambios:    []cambio{anadir("steps/01-pruebas/commands.yaml", "      scope: shared\n")},
			invariante: dominio.Formato, fichero: "steps/01-pruebas/commands.yaml", detalle: `una aserción con scope "shared"`,
		},
		{
			nombre:     "un uso que no es un nombre de variable",
			cambios:    []cambio{reemplazar("steps/01-pruebas/commands.yaml", "mvn clean verify", "mvn clean verify -P${var.no-vale}")},
			invariante: dominio.Formato, fichero: "steps/01-pruebas/commands.yaml", detalle: "${var.no-vale} no es un nombre de variable",
		},
		{
			nombre:     "una variable sin value",
			cambios:    []cambio{reemplazar("variables/compartidas.yaml", "  value: Basic\n", "")},
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml",
			detalle: `"registro_sku" no dice value`,
		},
		{
			nombre:     "una clave que un fichero de variables no tiene",
			cambios:    []cambio{anadir("variables/compartidas.yaml", "  resolve: step-output\n")},
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml", detalle: "field resolve not found",
		},

		{
			nombre:     "un fichero de variables que no es YAML",
			cambios:    []cambio{escribirFichero("variables/sand/notas.txt", "")},
			invariante: dominio.Formato, fichero: "variables/sand/notas.txt", detalle: "no es parte del formato",
		},
		{
			nombre:     "un directorio dentro de un ambiente",
			cambios:    []cambio{escribirFichero("variables/sand/extra/despliegue.yaml", "")},
			invariante: dominio.Formato, fichero: "variables/sand/extra/", detalle: "no es parte del formato",
		},
		{
			nombre:     "un fichero suelto en steps/",
			cambios:    []cambio{escribirFichero("steps/LEEME.md", "")},
			invariante: dominio.Formato, fichero: "steps/LEEME.md", detalle: "no es parte del formato",
		},
		{
			nombre:     "variables/ que no es un directorio",
			cambios:    []cambio{borrar("variables"), escribirFichero("variables", "")},
			invariante: dominio.Formato, fichero: "variables", detalle: "no es parte del formato", varios: true,
		},
		{
			nombre:     "steps/ que no es un directorio",
			cambios:    []cambio{borrar("steps"), escribirFichero("steps", "")},
			invariante: dominio.Formato, fichero: "steps", detalle: "no es parte del formato", varios: true,
		},

		{
			nombre:     "un value que no sirve como directorio",
			cambios:    []cambio{anadir("environments.yaml", "\n- name: qa\n  value: qa/1\n")},
			invariante: dominio.Formato, fichero: "environments.yaml", detalle: `value "qa/1"`,
		},

		{
			nombre:     "un max_age negativo",
			cambios:    []cambio{reemplazar("config.yaml", "max_age: 720h", "max_age: -1h")},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `max_age "-1h" no es una duración positiva`,
		},
		{
			nombre:     "una plantilla que sale del paso",
			cambios:    []cambio{reemplazar("steps/05-despliegue/commands.yaml", "- service.yaml", "- ../../service.yaml")},
			invariante: dominio.Formato, fichero: "steps/05-despliegue/commands.yaml",
			detalle: `la plantilla "../../service.yaml", que sale del directorio del paso`,
		},
		{
			nombre: "una plantilla que es un enlace",
			cambios: []cambio{
				enlazar("steps/05-despliegue/k8s/enlace.yaml", "service.yaml"),
				reemplazar("steps/05-despliegue/commands.yaml", "- service.yaml", "- service.yaml\n    - enlace.yaml"),
			},
			invariante: dominio.Formato, fichero: "steps/05-despliegue/commands.yaml", detalle: "k8s/enlace.yaml es un enlace",
		},
		{
			nombre:     "un ambiente sin name",
			cambios:    []cambio{anadir("environments.yaml", "\n- value: qa\n")},
			invariante: dominio.Formato, fichero: "environments.yaml", detalle: "el ambiente 4 no dice name",
		},

		// Los pasos.
		{
			nombre:     "un nombre de paso que no sirve como nombre de fichero",
			cambios:    []cambio{escribirFichero("steps/07-mi.paso/commands.yaml", "- cmd: ls\n")},
			invariante: dominio.Pasos, fichero: "steps/07-mi.paso", detalle: `"mi.paso" no sirve como nombre de un paso`,
		},
		{
			nombre:     "un directorio sin NN de dos dígitos",
			cambios:    []cambio{renombrar("steps/06-aviso", "steps/6-aviso")},
			invariante: dominio.Pasos, fichero: "steps/6-aviso", detalle: "NN-<paso>, con NN de dos dígitos", varios: true,
		},
		{
			nombre:     "un orden repetido",
			cambios:    []cambio{escribirFichero("steps/06-zz/commands.yaml", "- cmd: ls\n")},
			invariante: dominio.Pasos, fichero: "steps/06-zz", detalle: "el orden 06 ya es de steps/06-aviso",
		},
		{
			nombre:     "un nombre repetido, aunque cambie de mayúsculas",
			cambios:    []cambio{escribirFichero("steps/07-Aviso/commands.yaml", "- cmd: ls\n")},
			invariante: dominio.Pasos, fichero: "steps/07-Aviso", detalle: `otro paso ya se llama "Aviso"`,
		},
		{
			nombre:     "un paso sin commands.yaml",
			cambios:    []cambio{borrar("steps/01-pruebas/commands.yaml")},
			invariante: dominio.Pasos, fichero: "steps/01-pruebas/commands.yaml",
			detalle: "no está, y el paso declara ahí sus comandos",
		},
		{
			nombre:     "un commands.yaml vacío",
			cambios:    []cambio{escribirFichero("steps/01-pruebas/commands.yaml", "")},
			invariante: dominio.Pasos, fichero: "steps/01-pruebas/commands.yaml", detalle: "no declara ningún comando",
		},
		{
			nombre:     "un pipeline sin pasos",
			cambios:    []cambio{borrar("steps")},
			invariante: dominio.Pasos, fichero: "steps/", detalle: "el pipeline no tiene pasos", varios: true,
		},

		// Las variables.
		{
			nombre:     "una variable usada que no está declarada",
			cambios:    []cambio{reemplazar("steps/01-pruebas/commands.yaml", "mvn clean verify", "mvn clean verify -Dnombre=${var.nadie}")},
			invariante: dominio.Variables, fichero: "steps/01-pruebas/commands.yaml", detalle: "usa ${var.nadie}",
		},
		{
			nombre:     "una variable declarada en unos ambientes y no en otro",
			cambios:    []cambio{reemplazar("variables/stag/despliegue.yaml", "- name: puerto\n  value: 8080\n", "")},
			invariante: dominio.Variables, fichero: "steps/05-despliegue/k8s/service.yaml", ambiente: "stag",
			detalle: "usa ${var.puerto}",
		},
		{
			nombre:     "una variable no declarada en una plantilla",
			cambios:    []cambio{anadir("steps/03-infraestructura/terraform/terraform.tfvars", "zona = \"${var.zona}\"\n")},
			invariante: dominio.Variables, fichero: "steps/03-infraestructura/terraform/terraform.tfvars", detalle: "usa ${var.zona}",
		},
		{
			nombre: "una variable compartida que usa una salida del ámbito de un ambiente",
			cambios: []cambio{anadir("variables/compartidas.yaml",
				"\n- name: url\n  value: http://${var.cluster_fqdn}\n")},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: "es una variable de salida del ámbito de un ambiente",
		},
		{
			nombre: "un paso de scope: shared usa una variable de salida del ámbito de un ambiente",
			cambios: []cambio{anadir("steps/02-registro/commands.yaml",
				"\n- name: extra\n  cmd: echo ${var.cluster_fqdn}\n")},
			invariante: dominio.Variables, fichero: "steps/02-registro/commands.yaml",
			detalle: "usa ${var.cluster_fqdn}, que es una variable de salida del ámbito de un ambiente, y desde " +
				"el ámbito compartido no se ve",
		},
		{
			nombre: "un comando no ve lo que él mismo produce",
			cambios: []cambio{reemplazar("steps/05-despliegue/commands.yaml",
				"kubectl get services -n ${var.espacio}", "kubectl get services -n ${var.direccion}")},
			invariante: dominio.Variables, fichero: "steps/05-despliegue/commands.yaml",
			detalle: "usa ${var.direccion}, y la produce este mismo comando",
		},
		{
			nombre: "una salida usada antes de producirse",
			cambios: []cambio{reemplazar("steps/01-pruebas/commands.yaml", "mvn clean verify",
				"mvn clean verify -Dregistro=${var.registro_nombre}")},
			invariante: dominio.Variables, fichero: "steps/01-pruebas/commands.yaml",
			detalle: `usa ${var.registro_nombre}, y la produce el comando 2 de "registro", que va después`,
		},
		{
			nombre: "una variable declarada que necesita una salida que aún no se produjo",
			cambios: []cambio{reemplazar("steps/01-pruebas/commands.yaml", "mvn clean verify",
				"mvn clean verify -Dservidor=${var.servidor}")},
			invariante: dominio.Variables, fichero: "steps/01-pruebas/commands.yaml",
			detalle: `usa ${var.servidor}, que necesita la salida "registro_servidor"`,
		},
		{
			nombre: "un valor declarado que usa un nombre que no existe",
			cambios: []cambio{reemplazar("variables/compartidas.yaml", "value: ${var.registro_servidor}",
				"value: ${var.registro_url}")},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: `"servidor" usa ${var.registro_url}, que no es una variable estándar`,
		},
		{
			nombre:     "el mismo nombre en el ámbito compartido y en el de un ambiente",
			cambios:    []cambio{escribirFichero("variables/prod/otras.yaml", "- name: registro_sku\n  value: Premium\n")},
			invariante: dominio.Variables, fichero: "variables/prod/otras.yaml",
			detalle: "un nombre pertenece a un solo ámbito",
		},
		{
			nombre:     "el mismo nombre dos veces en un ámbito, en dos ficheros",
			cambios:    []cambio{escribirFichero("variables/prod/otras.yaml", "- name: nodos\n  value: 9\n")},
			invariante: dominio.Variables, fichero: "variables/prod/otras.yaml", ambiente: "prod",
			detalle: `la variable "nodos" ya está declarada en variables/prod/infraestructura.yaml`,
		},
		{
			nombre:     "un directorio de variables/ que no es de ningún ambiente",
			cambios:    []cambio{escribirFichero("variables/qa/despliegue.yaml", "- name: puerto\n  value: 80\n")},
			invariante: dominio.Variables, fichero: "variables/qa/despliegue.yaml",
			detalle: "variables/qa/ no es de ningún ámbito",
		},
		{
			nombre:     "declarar una variable estándar",
			cambios:    []cambio{anadir("variables/compartidas.yaml", "- name: project_hash\n  value: x\n")},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: `"project_hash" es una variable estándar`,
		},
		{
			nombre: "producir una variable estándar",
			cambios: []cambio{anadir("steps/02-registro/commands.yaml",
				"\n- name: extra\n  cmd: echo\n  outputs:\n    - name: step_name\n      probe: (.*)\n")},
			invariante: dominio.Variables, fichero: "steps/02-registro/commands.yaml", detalle: `produce "step_name"`,
		},
		{
			nombre: "variables que se usan en círculo en un ambiente",
			cambios: []cambio{reemplazar("variables/prod/despliegue.yaml",
				"value: ${var.project_name}${var.project_id}-${var.environment}", "value: ${var.espacio}")},
			invariante: dominio.Variables, fichero: "variables/prod/despliegue.yaml", ambiente: "prod",
			detalle: "se usan en círculo: instancia → espacio → instancia",
		},

		// Las variables de salida.
		{
			nombre:     "una expresión regular mal formada",
			cambios:    []cambio{reemplazar("steps/05-despliegue/commands.yaml", `probe: ^(\d+\.\d+\.\d+\.\d+)$`, `probe: ^(\d+`)},
			invariante: dominio.VariablesDeSalida, fichero: "steps/05-despliegue/commands.yaml",
			detalle: `la expresión regular de "direccion" no es correcta`,
		},
		{
			nombre: "una variable de salida sin probe",
			cambios: []cambio{reemplazar("steps/03-infraestructura/commands.yaml",
				"      probe: cluster_fqdn\\s*=\\s*\"([^\"]+)\"\n", "")},
			invariante: dominio.VariablesDeSalida, fichero: "steps/03-infraestructura/commands.yaml",
			detalle: `"cluster_fqdn" no dice probe`,
		},
		{
			nombre: "una variable de salida producida dos veces por el mismo comando",
			cambios: []cambio{anadir("steps/03-infraestructura/commands.yaml",
				"    - name: cluster_nombre\n      probe: (.*)\n")},
			invariante: dominio.VariablesDeSalida, fichero: "steps/03-infraestructura/commands.yaml",
			detalle: `el comando 3 produce "cluster_nombre" dos veces`,
		},
		{
			nombre: "el mismo nombre producido por dos comandos del mismo ámbito",
			cambios: []cambio{anadir("steps/05-despliegue/commands.yaml",
				"\n- name: salida\n  cmd: terraform output\n  outputs:\n    - name: cluster_nombre\n      probe: (.*)\n")},
			invariante: dominio.VariablesDeSalida, fichero: "steps/05-despliegue/commands.yaml",
			detalle: `produce "cluster_nombre", que ya produce el comando 3 de "infraestructura"`,
		},
		{
			nombre: "el mismo nombre producido en dos ámbitos",
			cambios: []cambio{anadir("steps/05-despliegue/commands.yaml",
				"\n- name: salida\n  cmd: terraform output\n  outputs:\n    - name: registro_nombre\n      probe: (.*)\n")},
			invariante: dominio.VariablesDeSalida, fichero: "steps/05-despliegue/commands.yaml",
			detalle: "produce en el otro ámbito: un nombre pertenece a un solo ámbito",
		},

		// Las aserciones.
		{
			nombre:     "la expresión regular de una aserción mal formada",
			cambios:    []cambio{reemplazar("steps/01-pruebas/commands.yaml", "probe: BUILD SUCCESS", "probe: BUILD (")},
			invariante: dominio.Aserciones, fichero: "steps/01-pruebas/commands.yaml",
			detalle: "la expresión regular de una aserción no es correcta",
		},

		// Los ambientes.
		{
			nombre:     "sin environments.yaml",
			cambios:    []cambio{borrar("environments.yaml")},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: "no está", varios: true,
		},
		{
			nombre:     "un environments.yaml vacío",
			cambios:    []cambio{escribirFichero("environments.yaml", "")},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: "no declara ningún ambiente", varios: true,
		},
		{
			nombre:     "un name repetido",
			cambios:    []cambio{anadir("environments.yaml", "\n- name: staging\n  value: stag2\n")},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: `el name "staging" está dos veces`,
		},
		{
			nombre:     "un value repetido, aunque cambie de mayúsculas",
			cambios:    []cambio{anadir("environments.yaml", "\n- name: otra\n  value: PROD\n")},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: `el value "PROD" está dos veces`,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			raiz := copiarEjemplo(t)
			for _, c := range caso.cambios {
				c(t, raiz)
			}
			p, err := comprobarDirectorio(t, raiz)
			require.Nil(t, p, "si falla la comprobación no hay pipeline")
			require.ErrorIs(t, err, dominio.ErrNoComprobado)
			lista := err.(*dominio.FallosDeComprobacion).Fallos

			encontrado := false
			for _, f := range lista {
				encontrado = encontrado || f.Invariante == caso.invariante && f.Fichero == caso.fichero &&
					f.Ambiente == caso.ambiente && strings.Contains(f.Detalle, caso.detalle)
			}
			require.True(t, encontrado, "no está el fallo esperado entre:\n%v", err)
			if !caso.varios {
				require.Len(t, lista, 1, "un fallo no arrastra otros:\n%v", err)
			}
		})
	}
}

func TestLoQueProduceUnComandoLoVenLosPosteriores(t *testing.T) {
	raiz := copiarEjemplo(t)
	anadir("steps/05-despliegue/commands.yaml", "\n- name: probar\n  cmd: curl http://${var.direccion}\n")(t, raiz)

	_, err := comprobarDirectorio(t, raiz)
	require.NoError(t, err, "un comando ve lo que produjo un comando anterior del mismo paso")
}

func TestRenumerarLosPasosDelEjemploNoCambiaSuIdentidad(t *testing.T) {
	raiz := copiarEjemplo(t)
	for _, par := range [][2]string{
		{"01-pruebas", "10-pruebas"}, {"02-registro", "20-registro"}, {"03-infraestructura", "30-infraestructura"},
		{"04-imagen", "40-imagen"}, {"05-despliegue", "50-despliegue"}, {"06-aviso", "60-aviso"},
	} {
		renombrar("steps/"+par[0], "steps/"+par[1])(t, raiz)
	}

	antes, err := comprobarDirectorio(t, ejemplo)
	require.NoError(t, err)
	despues, err := comprobarDirectorio(t, raiz)
	require.NoError(t, err, "ningún from ni ningún fichero de variables nombra el orden")

	pasosAntes, pasosDespues := antes.Pasos(), despues.Pasos()
	require.Len(t, pasosDespues, len(pasosAntes))
	for i := range pasosAntes {
		require.Equal(t, pasosAntes[i].Orden*10, pasosDespues[i].Orden)
		pasosAntes[i].Orden, pasosDespues[i].Orden = 0, 0
		require.Equal(t, pasosAntes[i], pasosDespues[i], "lo único que cambia es el orden")
	}
}
