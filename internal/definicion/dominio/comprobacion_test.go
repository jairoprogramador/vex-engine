package dominio_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
)

func texto(s string) *string { return &s }

func version(v string) *string { return &v }

// compartido da un *dominio.Ambito con el valor Compartido: lo único declarable como ámbito propio de un
// paso o de una variable de salida (RD-04 §9.19).
func compartido() *dominio.Ambito {
	c := dominio.Compartido
	return &c
}

// mutarConfiguracion cambia la entrada de un paso bajo steps, en config.yaml (RD-04 §9.20): un mapa no es
// direccionable, así que hace falta leer, mutar y volver a escribir.
func mutarConfiguracion(d *dominio.PipelineDeclarado, nombre string, mutar func(*dominio.ConfiguracionDePasoDeclarada)) {
	c := d.Configuracion.Datos.Pasos[nombre]
	mutar(&c)
	d.Configuracion.Datos.Pasos[nombre] = c
}

// valida es un pipeline pequeño que pasa la comprobación: un paso que crea el registro y produce una variable
// de salida compartida, y otro que la usa en una plantilla, en dos ambientes.
func valida() dominio.PipelineDeclarado {
	return dominio.PipelineDeclarado{
		Commit: strings.Repeat("a", 40),
		Hash:   "contenido-v1:abc",
		Configuracion: dominio.Declarado[dominio.ConfiguracionDeclarada]{Existe: true, Datos: dominio.ConfiguracionDeclarada{
			Version: version("1"),
			Pasos: map[string]dominio.ConfiguracionDePasoDeclarada{
				"registro": {Reglas: []string{"instructions"}, ReglasEscritas: true, EdadMaxima: "720h"},
			},
		}},
		Ambientes: dominio.DeclaradoDe(
			dominio.AmbienteDeclarado{Nombre: "sandbox", Descripcion: "pruebas", Valor: "sand"},
			dominio.AmbienteDeclarado{Nombre: "production", Valor: "prod"},
		),
		Pasos: []dominio.PasoDeclarado{
			{
				Directorio: "01-registro",
				Comandos: dominio.DeclaradoDe(dominio.ComandoDeclarado{
					Nombre:     "crear",
					Linea:      "terraform apply -var project=${var.project_id} -var sku=${var.sku}",
					Directorio: "./terraform",
					Variables: []dominio.VariableDeComandoDeclarada{
						{Nombre: "registro", Expresion: `registro = "([^"]+)"`, Ambito: "shared"},
						// Sin name: una aserción sobre la salida del comando, que no produce ninguna variable.
						{Descripcion: "terraform terminó", Expresion: "Apply complete"},
					},
				}),
				// Terraform usa ${var.…} propio: como no está en templates, no es del motor.
				Material: []dominio.FicheroDeclarado{{Ruta: "terraform/main.tf", Contenido: `name = "${var.name}"`}},
			},
			{
				Directorio: "02-despliegue",
				Comandos: dominio.DeclaradoDe(
					dominio.ComandoDeclarado{
						Nombre: "aplicar", Linea: "kubectl apply -f . -n ${var.espacio}", Directorio: "k8s",
						Plantillas: []string{"deployment.yaml"},
						Variables:  []dominio.VariableDeComandoDeclarada{{Nombre: "ip", Expresion: `ip=(\S+)`}},
					},
					// Ve lo que produjo el comando anterior del mismo paso.
					dominio.ComandoDeclarado{Nombre: "comprobar", Linea: "curl ${var.ip}"},
				),
				Material: []dominio.FicheroDeclarado{
					{Ruta: "k8s/deployment.yaml", Contenido: "image: ${var.imagen}/${var.project_name}:${var.project_hash}\n" +
						"env: ${var.environment}\nworkdir: ${var.step_workdir}\n"},
					{Ruta: "scripts/esperar.sh", Contenido: "#!/bin/sh\n", Ejecutable: true},
				},
			},
		},
		Variables: []dominio.VariablesDePipelineDeclarada{
			// En la raíz de variables/: el ámbito compartido. El nombre del fichero solo organiza.
			{Fichero: "variables/compartidas.yaml", Variables: []dominio.VariableDePipelineDeclarada{
				{Nombre: "sku", Valor: texto("Basic")},
				// Otro nombre para una variable de salida compartida: un literal que la usa.
				{Nombre: "imagen", Valor: texto("${var.registro}")},
			}},
			{Fichero: "variables/sand/despliegue.yaml", Ambito: "sand", Variables: []dominio.VariableDePipelineDeclarada{
				{Nombre: "espacio", Valor: texto("${var.project_name}-${var.equipo}")},
				{Nombre: "equipo", Valor: texto("plataforma")},
			}},
			{Fichero: "variables/prod/despliegue.yaml", Ambito: "prod", Variables: []dominio.VariableDePipelineDeclarada{
				{Nombre: "espacio", Valor: texto("${var.project_name}")},
			}},
		},
	}
}

func comprobar(t *testing.T, d dominio.PipelineDeclarado) *dominio.PipelineComprobado {
	t.Helper()
	p, err := dominio.Comprobar(d)
	require.NoError(t, err)
	return p
}

func fallos(t *testing.T, d dominio.PipelineDeclarado) []dominio.Fallo {
	t.Helper()
	p, err := dominio.Comprobar(d)
	require.Nil(t, p, "si falla la comprobación no hay pipeline")
	require.ErrorIs(t, err, dominio.ErrNoComprobado)
	var lista *dominio.FallosDeComprobacion
	require.True(t, errors.As(err, &lista))
	require.NotEmpty(t, lista.Fallos)
	return lista.Fallos
}

func TestUnaDeclaracionBienFormadaDaUnPipeline(t *testing.T) {
	p := comprobar(t, valida())

	require.Equal(t, "1", p.Version())
	require.Equal(t, strings.Repeat("a", 40), p.Commit())
	require.Equal(t, "contenido-v1:abc", p.Hash())
	require.Equal(t, []dominio.AmbienteComprobado{
		{Nombre: "sandbox", Descripcion: "pruebas", Valor: "sand"},
		{Nombre: "production", Valor: "prod"},
	}, p.Ambientes(), "en su orden")

	pasos := p.Pasos()
	require.Len(t, pasos, 2)

	registro := pasos[0]
	require.Equal(t, "registro", registro.Nombre, "el nombre es el del directorio sin NN-")
	require.Equal(t, 1, registro.Orden)
	require.Equal(t, []dominio.Regla{dominio.ReglaInstrucciones}, registro.Reglas)
	require.Equal(t, 720*time.Hour, registro.EdadMaxima)
	require.Equal(t, "terraform", registro.Comandos[0].Directorio)

	despliegue := pasos[1]
	require.Equal(t, "steps/02-despliegue", despliegue.Directorio())
	require.Equal(t, []string{"k8s/deployment.yaml"}, despliegue.Comandos[0].Plantillas)
	require.True(t, despliegue.Material[1].Ejecutable)

	t.Run("una variable pertenece a un ámbito, no a un paso", func(t *testing.T) {
		require.Equal(t, []dominio.VariableDePipelineComprobada{
			{Nombre: "sku", Valor: "Basic"},
			{Nombre: "imagen", Valor: "${var.registro}"},
			{Nombre: "espacio", Ambito: "sand", Valor: "${var.project_name}-${var.equipo}"},
			{Nombre: "equipo", Ambito: "sand", Valor: "plataforma"},
			{Nombre: "espacio", Ambito: "prod", Valor: "${var.project_name}"},
		}, p.Variables())
		require.Equal(t, dominio.Compartido, p.Variables()[0].Ambito, "la raíz de variables/ es el ámbito compartido")
	})

	t.Run("un outputs con name es una variable de salida, y sin name una aserción", func(t *testing.T) {
		require.Equal(t, []dominio.VariableDeComandoComprobada{
			{Nombre: "registro", Expresion: `registro = "([^"]+)"`, Ambito: compartido()},
		}, registro.Comandos[0].VariablesDeSalida)
		require.Equal(t, []dominio.AsercionComprobada{
			{Descripcion: "terraform terminó", Expresion: "Apply complete"},
		}, registro.Comandos[0].Aserciones)
		require.Empty(t, despliegue.Comandos[0].Aserciones)
		require.Nil(t, despliegue.Comandos[0].VariablesDeSalida[0].Ambito, "sin scope, la del ambiente en ejecución")
	})

	t.Run("sin rules.yaml, los valores por defecto", func(t *testing.T) {
		require.Equal(t, []dominio.Regla{dominio.ReglaCodigo, dominio.ReglaInstrucciones, dominio.ReglaVariables},
			despliegue.Reglas, "sin rules, las tres cosas")
		require.Zero(t, despliegue.EdadMaxima, "sin max_age, no caduca")
	})

	t.Run("rules vacío no mira nada", func(t *testing.T) {
		d := valida()
		d.Configuracion.Datos.Pasos["despliegue"] = dominio.ConfiguracionDePasoDeclarada{ReglasEscritas: true}
		paso, _ := comprobar(t, d).Paso("despliegue")
		require.Empty(t, paso.Reglas)
	})

	t.Run("Paso de un nombre que no existe, no está", func(t *testing.T) {
		_, esta := p.Paso("no-existe")
		require.False(t, esta)
	})
}

func TestAmbitoString(t *testing.T) {
	require.Equal(t, "compartido", dominio.Compartido.String())
	require.Equal(t, "sand", dominio.Ambito("sand").String())
}

func TestLoQueVeUnPasoEsSuAmbito(t *testing.T) {
	t.Run("las variables de un ámbito las ve cualquier paso que se ejecute en él", func(t *testing.T) {
		d := valida()
		// espacio y equipo se declaran en el fichero del paso despliegue, y los usa el paso registro.
		d.Variables[2].Variables = append(d.Variables[2].Variables,
			dominio.VariableDePipelineDeclarada{Nombre: "equipo", Valor: texto("plataforma")})
		d.Pasos[0].Comandos.Datos[0].Linea += " -var equipo=${var.equipo}"
		comprobar(t, d)
	})

	t.Run("un comando ve lo que produjo un comando anterior del mismo paso", func(t *testing.T) {
		// La línea del segundo comando de despliegue usa ${var.ip}, que produce el primero.
		comprobar(t, valida())
	})

	t.Run("dos ambientes pueden declarar el mismo nombre", func(t *testing.T) {
		// espacio está en sand y en prod.
		comprobar(t, valida())
	})
}

func TestUnPasoSinScopeEsDelAmbienteEnQueSeEjecuta(t *testing.T) {
	p := comprobar(t, valida())
	registro, _ := p.Paso("registro")
	despliegue, _ := p.Paso("despliegue")
	require.Nil(t, registro.Ambito, "sin scope en su rules.yaml")
	require.Nil(t, despliegue.Ambito, "tampoco tiene rules.yaml")
}

func TestElAmbitoDeUnPasoLoHeredaLoQueProduceSinScopePropio(t *testing.T) {
	d := valida()
	require.Equal(t, "shared", d.Pasos[0].Comandos.Datos[0].Variables[0].Ambito,
		"ya declara scope propio: no sirve para probar la herencia")
	d.Pasos[0].Comandos.Datos[0].Variables[0].Ambito = "" // ahora depende del ámbito del paso
	mutarConfiguracion(&d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Ambito = "shared" })

	p := comprobar(t, d)
	registro, _ := p.Paso("registro")
	require.Equal(t, compartido(), registro.Ambito, "rules.yaml declaró scope: shared")
	require.Equal(t, compartido(), registro.Comandos[0].VariablesDeSalida[0].Ambito, "sin scope propio, hereda el del paso")
}

func TestUnPasoDeScopeSharedNoVeLoDeUnAmbiente(t *testing.T) {
	t.Run("una variable declarada de un ambiente", func(t *testing.T) {
		d := valida()
		mutarConfiguracion(&d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Ambito = "shared" })
		d.Pasos[0].Comandos.Datos[0].Linea += " -var espacio=${var.espacio}" // declarada solo en sand y en prod
		lista := fallos(t, d)
		require.Len(t, lista, 1)
		require.Equal(t, dominio.Variables, lista[0].Invariante)
		require.Empty(t, lista[0].Ambiente, "un paso compartido no varía por ambiente")
		require.Contains(t, lista[0].Detalle, "usa ${var.espacio}, que no es una variable estándar, ni está "+
			"declarada en un ámbito que se vea desde aquí")
	})

	t.Run("una variable de salida del ámbito de un ambiente", func(t *testing.T) {
		d := valida()
		mutarConfiguracion(&d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Ambito = "shared" })
		d.Pasos[0].Comandos.Datos[0].Linea += " -var ip=${var.ip}" // la produce despliegue, sin scope: shared
		lista := fallos(t, d)
		require.Len(t, lista, 1)
		require.Equal(t, dominio.Variables, lista[0].Invariante)
		require.Empty(t, lista[0].Ambiente)
		require.Contains(t, lista[0].Detalle,
			"usa ${var.ip}, que es una variable de salida del ámbito de un ambiente, y desde el ámbito compartido no se ve")
	})
}

func TestAmbitoEfectivo(t *testing.T) {
	sand := dominio.Ambito("sand")

	t.Run("un paso sin scope propio es del ambiente que se le pida", func(t *testing.T) {
		paso := dominio.PasoComprobado{}
		require.Equal(t, sand, paso.AmbitoEfectivo(sand))
		require.Equal(t, dominio.Ambito("prod"), paso.AmbitoEfectivo("prod"), "es constante por ejecución, no fijo")
	})

	t.Run("un paso con scope propio ignora el ambiente que se le pida", func(t *testing.T) {
		paso := dominio.PasoComprobado{Ambito: compartido()}
		require.Equal(t, dominio.Compartido, paso.AmbitoEfectivo(sand))
		require.Equal(t, dominio.Compartido, paso.AmbitoEfectivo("prod"))
	})

	t.Run("una salida sin scope propio hereda el ámbito ya resuelto de su paso", func(t *testing.T) {
		salida := dominio.VariableDeComandoComprobada{}
		require.Equal(t, sand, salida.AmbitoEfectivo(sand))
		require.Equal(t, dominio.Compartido, salida.AmbitoEfectivo(dominio.Compartido))
	})

	t.Run("una salida con scope propio ignora el de su paso", func(t *testing.T) {
		salida := dominio.VariableDeComandoComprobada{Ambito: compartido()}
		require.Equal(t, dominio.Compartido, salida.AmbitoEfectivo(sand))
	})
}

func TestCadaFilaDeLaComprobacionProduceSuFallo(t *testing.T) {
	casos := []struct {
		nombre     string
		mutar      func(d *dominio.PipelineDeclarado)
		invariante dominio.Invariante
		fichero    string
		ambiente   string
		detalle    string
		// varios: el fallo arrastra otros que son su consecuencia.
		varios bool
	}{
		// El formato.
		{
			nombre: "sin config.yaml",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Configuracion = dominio.Declarado[dominio.ConfiguracionDeclarada]{}
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "no está, y el pipeline declara ahí su schema_version",
		},
		{
			nombre: "sin schema_version", mutar: func(d *dominio.PipelineDeclarado) { d.Configuracion.Datos.Version = nil },
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "no dice schema_version",
		},
		{
			nombre:     "una schema_version distinta de la que se lee",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Configuracion.Datos.Version = version("2") },
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `schema_version "2" no se lee: la única que se lee es la 1`,
		},
		{
			nombre: "un fichero que no se puede leer",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ilegibles = append(d.Ilegibles, dominio.FicheroIlegibleDeclarado{Fichero: "config.yaml", Motivo: "yaml: línea 1"})
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: "yaml: línea 1",
		},
		{
			nombre: "algo que no es parte del formato",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Desconocidos = append(d.Desconocidos, "variables/sand/notas.txt")
			},
			invariante: dominio.Formato, fichero: "variables/sand/notas.txt", detalle: "no es parte del formato",
		},
		{
			nombre: "una variable de salida con un scope desconocido",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[0].Comandos.Datos[0].Variables[0].Ambito = "project"
			},
			invariante: dominio.Formato, fichero: "steps/01-registro/commands.yaml",
			detalle: `produce "registro" con scope "project", que no existe`, varios: true,
		},
		{
			nombre: "una regla desconocida",
			mutar: func(d *dominio.PipelineDeclarado) {
				mutarConfiguracion(d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Reglas = []string{"state_changed"} })
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `la regla "state_changed" no existe`,
		},
		{
			nombre: "una regla repetida",
			mutar: func(d *dominio.PipelineDeclarado) {
				mutarConfiguracion(d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Reglas = []string{"code", "code"} })
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `la regla "code" está dos veces`,
		},
		{
			nombre: "un max_age que no es una duración",
			mutar: func(d *dominio.PipelineDeclarado) {
				mutarConfiguracion(d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.EdadMaxima = "30d" })
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `max_age "30d"`,
		},
		{
			nombre: "un scope de paso que no existe",
			mutar: func(d *dominio.PipelineDeclarado) {
				mutarConfiguracion(d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.Ambito = "project" })
			},
			invariante: dominio.Formato, fichero: "config.yaml",
			detalle: `el paso tiene scope "project", que no existe: es environment o shared`,
		},
		{
			nombre: "un comando sin cmd",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos = append(d.Pasos[1].Comandos.Datos, dominio.ComandoDeclarado{Nombre: "nada"})
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml", detalle: "el comando 3 no dice cmd",
		},
		{
			nombre: "un workdir que sale del paso",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos = append(d.Pasos[1].Comandos.Datos, dominio.ComandoDeclarado{Linea: "ls", Directorio: "../.."})
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml", detalle: "sale del directorio del paso",
		},
		{
			nombre: "una plantilla que no está en el material",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos[0].Plantillas = append(d.Pasos[1].Comandos.Datos[0].Plantillas, "service.yaml")
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml",
			detalle: "k8s/service.yaml no está en el material",
		},
		{
			nombre: "un enlace que sale del paso",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Material = append(d.Pasos[1].Material, dominio.FicheroDeclarado{Ruta: "k8s/fuera", Enlace: "../../../etc"})
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/k8s/fuera", detalle: "fuera del directorio del paso",
		},
		{
			nombre: "un outputs sin name ni probe",
			mutar: func(d *dominio.PipelineDeclarado) {
				c := &d.Pasos[0].Comandos.Datos[0]
				c.Variables = append(c.Variables, dominio.VariableDeComandoDeclarada{Descripcion: "nada"})
			},
			invariante: dominio.Formato, fichero: "steps/01-registro/commands.yaml", detalle: "un outputs sin name ni probe",
		},
		{
			nombre: "una variable de salida con un nombre inválido",
			mutar: func(d *dominio.PipelineDeclarado) {
				c := &d.Pasos[0].Comandos.Datos[0]
				c.Variables = append(c.Variables, dominio.VariableDeComandoDeclarada{Nombre: "1invalido", Expresion: "x"})
			},
			invariante: dominio.Formato, fichero: "steps/01-registro/commands.yaml", detalle: "que no es un nombre de variable",
		},
		{
			nombre: "una aserción con scope",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[0].Comandos.Datos[0].Variables[1].Ambito = "shared"
			},
			invariante: dominio.Formato, fichero: "steps/01-registro/commands.yaml",
			detalle: "una aserción con scope \"shared\"",
		},

		{
			nombre:     "una variable sin value",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Variables[0].Variables[0].Valor = nil },
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml", detalle: `"sku" no dice value`,
		},
		{
			nombre: "una variable declarada sin name",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables, dominio.VariableDePipelineDeclarada{Valor: texto("x")})
			},
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml", detalle: "una variable no dice name",
		},
		{
			nombre: "una variable declarada con un nombre inválido",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables, dominio.VariableDePipelineDeclarada{Nombre: "1invalido", Valor: texto("x")})
			},
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml", detalle: `"1invalido" no es un nombre de variable`,
		},
		{
			nombre: "un valor declarado que usa un nombre de variable malformado",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables, dominio.VariableDePipelineDeclarada{Nombre: "otra", Valor: texto("${var.no-vale}")})
			},
			invariante: dominio.Formato, fichero: "variables/compartidas.yaml", detalle: "${var.no-vale} no es un nombre de variable",
		},
		{
			nombre:     "un uso que no es un nombre de variable",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Pasos[1].Comandos.Datos[0].Linea += " ${var.no-vale}" },
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml", detalle: "${var.no-vale} no es un nombre",
		},

		{
			nombre: "un max_age que no es positivo",
			mutar: func(d *dominio.PipelineDeclarado) {
				mutarConfiguracion(d, "registro", func(c *dominio.ConfiguracionDePasoDeclarada) { c.EdadMaxima = "0s" })
			},
			invariante: dominio.Formato, fichero: "config.yaml", detalle: `max_age "0s" no es una duración positiva`,
		},
		{
			nombre: "una plantilla que sale del paso",
			mutar: func(d *dominio.PipelineDeclarado) {
				c := &d.Pasos[1].Comandos.Datos[0]
				c.Plantillas = append(c.Plantillas, "../../fuera.yaml")
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml",
			detalle: `la plantilla "../../fuera.yaml", que sale del directorio del paso`,
		},
		{
			nombre: "una plantilla que es un enlace",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Material = append(d.Pasos[1].Material, dominio.FicheroDeclarado{Ruta: "k8s/enlace.yaml", Enlace: "deployment.yaml"})
				c := &d.Pasos[1].Comandos.Datos[0]
				c.Plantillas = append(c.Plantillas, "enlace.yaml")
			},
			invariante: dominio.Formato, fichero: "steps/02-despliegue/commands.yaml", detalle: "k8s/enlace.yaml es un enlace",
		},
		{
			nombre: "un ambiente sin name",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ambientes.Datos = append(d.Ambientes.Datos, dominio.AmbienteDeclarado{Valor: "qa"})
			},
			invariante: dominio.Formato, fichero: "environments.yaml", detalle: "el ambiente 3 no dice name",
		},

		// Los pasos.
		{
			nombre: "un nombre de paso que no sirve como nombre de fichero",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{Directorio: "03-mi paso", Comandos: dominio.DeclaradoDe(dominio.ComandoDeclarado{Linea: "ls"})})
			},
			invariante: dominio.Pasos, fichero: "steps/03-mi paso", detalle: `"mi paso" no sirve como nombre de un paso`,
		},
		{
			nombre: "un directorio sin NN de dos dígitos",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{Directorio: "3-extra", Comandos: dominio.DeclaradoDe(dominio.ComandoDeclarado{Linea: "ls"})})
			},
			invariante: dominio.Pasos, fichero: "steps/3-extra", detalle: "NN-<paso>, con NN de dos dígitos",
		},
		{
			nombre: "un orden repetido",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{Directorio: "02-extra", Comandos: dominio.DeclaradoDe(dominio.ComandoDeclarado{Linea: "ls"})})
			},
			invariante: dominio.Pasos, fichero: "steps/02-extra", detalle: "el orden 02 ya es de steps/02-despliegue",
		},
		{
			nombre: "un nombre repetido, aunque cambie de mayúsculas",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{Directorio: "03-Despliegue", Comandos: dominio.DeclaradoDe(dominio.ComandoDeclarado{Linea: "ls"})})
			},
			invariante: dominio.Pasos, fichero: "steps/03-Despliegue", detalle: `otro paso ya se llama "Despliegue"`,
		},
		{
			nombre: "un paso sin commands.yaml",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{Directorio: "03-vacio"})
			},
			invariante: dominio.Pasos, fichero: "steps/03-vacio/commands.yaml", detalle: "no está, y el paso declara ahí sus comandos",
		},
		{
			nombre: "un commands.yaml vacío",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos = append(d.Pasos, dominio.PasoDeclarado{
					Directorio: "03-vacio", Comandos: dominio.Declarado[[]dominio.ComandoDeclarado]{Existe: true},
				})
			},
			invariante: dominio.Pasos, fichero: "steps/03-vacio/commands.yaml", detalle: "no declara ningún comando",
		},
		{
			nombre: "un pipeline sin pasos", mutar: func(d *dominio.PipelineDeclarado) { d.Pasos = nil },
			invariante: dominio.Pasos, fichero: "steps/", detalle: "no tiene pasos", varios: true,
		},
		{
			nombre: "config.yaml declara la configuración de un paso que no existe",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Configuracion.Datos.Pasos["notificar"] = dominio.ConfiguracionDePasoDeclarada{}
			},
			invariante: dominio.Pasos, fichero: "config.yaml",
			detalle: `declara la configuración de "notificar", que no es un paso: no hay ningún steps/NN-notificar/`,
		},

		// Las variables.
		{
			nombre:     "una variable usada que no está declarada",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Pasos[1].Comandos.Datos[0].Linea += " ${var.nadie}" },
			invariante: dominio.Variables, fichero: "steps/02-despliegue/commands.yaml", detalle: "usa ${var.nadie}",
		},
		{
			nombre:     "una variable declarada en un ambiente y no en otro",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Pasos[1].Comandos.Datos[0].Linea += " -l equipo=${var.equipo}" },
			invariante: dominio.Variables, fichero: "steps/02-despliegue/commands.yaml", ambiente: "prod",
			detalle: "usa ${var.equipo}",
		},
		{
			nombre: "una variable usada en una plantilla que no está declarada",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Material[0].Contenido += "replicas: ${var.replicas}\n"
			},
			invariante: dominio.Variables, fichero: "steps/02-despliegue/k8s/deployment.yaml", detalle: "usa ${var.replicas}",
		},
		{
			nombre: "una variable compartida que usa una salida del ámbito de un ambiente",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables,
					dominio.VariableDePipelineDeclarada{Nombre: "url", Valor: texto("http://${var.ip}")})
			},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: "es una variable de salida del ámbito de un ambiente",
		},
		{
			nombre: "un comando no ve lo que él mismo produce",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos[0].Linea += " --ip ${var.ip}"
			},
			invariante: dominio.Variables, fichero: "steps/02-despliegue/commands.yaml",
			detalle: "usa ${var.ip}, y la produce este mismo comando",
		},
		{
			nombre: "una salida usada antes de producirse",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[0].Comandos.Datos[0].Linea += " --ip ${var.ip}"
			},
			invariante: dominio.Variables, fichero: "steps/01-registro/commands.yaml",
			detalle: `usa ${var.ip}, y la produce el comando 1 de "despliegue", que va después`,
		},
		{
			nombre: "una variable declarada que necesita una salida que aún no se produjo",
			mutar: func(d *dominio.PipelineDeclarado) {
				destino := dominio.VariableDePipelineDeclarada{Nombre: "destino", Valor: texto("http://${var.ip}")}
				d.Variables[1].Variables = append(d.Variables[1].Variables, destino)
				d.Variables[2].Variables = append(d.Variables[2].Variables, destino)
				d.Pasos[0].Comandos.Datos[0].Linea += " --destino ${var.destino}"
			},
			invariante: dominio.Variables, fichero: "steps/01-registro/commands.yaml",
			detalle: `usa ${var.destino}, que necesita la salida "ip", y la produce el comando 1 de "despliegue"`,
		},
		{
			nombre: "una variable declarada necesita la misma salida por dos caminos distintos",
			mutar: func(d *dominio.PipelineDeclarado) {
				// "ip" pasa a ser compartida para que "a", "b" y "combo", declaradas en el ámbito compartido, la vean.
				d.Pasos[1].Comandos.Datos[0].Variables[0].Ambito = "shared"
				d.Variables[0].Variables = append(d.Variables[0].Variables,
					dominio.VariableDePipelineDeclarada{Nombre: "a", Valor: texto("${var.ip}")},
					dominio.VariableDePipelineDeclarada{Nombre: "b", Valor: texto("${var.ip}")},
					dominio.VariableDePipelineDeclarada{Nombre: "combo", Valor: texto("${var.a}-${var.b}")},
				)
				d.Pasos[0].Comandos.Datos[0].Linea += " --combo ${var.combo}"
			},
			invariante: dominio.Variables, fichero: "steps/01-registro/commands.yaml",
			detalle: `usa ${var.combo}, que necesita la salida "ip", y la produce el comando 1 de "despliegue"`,
		},
		{
			nombre:     "un valor declarado que usa un nombre que no existe",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Variables[0].Variables[1].Valor = texto("${var.otra}") },
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: `"imagen" usa ${var.otra}, que no es una variable estándar`,
		},
		{
			nombre: "el mismo nombre declarado en el ámbito compartido y en el de un ambiente",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[2].Variables = append(d.Variables[2].Variables,
					dominio.VariableDePipelineDeclarada{Nombre: "sku", Valor: texto("Premium")})
			},
			invariante: dominio.Variables, fichero: "variables/prod/despliegue.yaml",
			detalle: "un nombre pertenece a un solo ámbito",
		},
		{
			nombre: "el mismo nombre declarado dos veces en el mismo ámbito, en dos ficheros",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables = append(d.Variables, dominio.VariablesDePipelineDeclarada{
					Fichero: "variables/prod/otras.yaml", Ambito: "prod",
					Variables: []dominio.VariableDePipelineDeclarada{{Nombre: "espacio", Valor: texto("otro")}},
				})
			},
			invariante: dominio.Variables, fichero: "variables/prod/otras.yaml", ambiente: "prod",
			detalle: `la variable "espacio" ya está declarada en variables/prod/despliegue.yaml`,
		},
		{
			nombre: "un directorio de variables/ que no es de ningún ambiente",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables = append(d.Variables, dominio.VariablesDePipelineDeclarada{
					Fichero: "variables/stag/despliegue.yaml", Ambito: "stag",
				})
			},
			invariante: dominio.Variables, fichero: "variables/stag/despliegue.yaml",
			detalle: "variables/stag/ no es de ningún ámbito",
		},
		{
			nombre: "declarar una variable estándar",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables, dominio.VariableDePipelineDeclarada{Nombre: "environment", Valor: texto("x")})
			},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml", detalle: `"environment" es una variable estándar`,
		},
		{
			nombre: "producir una variable estándar",
			mutar: func(d *dominio.PipelineDeclarado) {
				c := &d.Pasos[0].Comandos.Datos[0]
				c.Variables = append(c.Variables, dominio.VariableDeComandoDeclarada{Nombre: "project_hash", Expresion: "x"})
			},
			invariante: dominio.Variables, fichero: "steps/01-registro/commands.yaml", detalle: `produce "project_hash"`,
		},
		{
			nombre: "variables que se usan en círculo",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[1].Variables = append(d.Variables[1].Variables,
					dominio.VariableDePipelineDeclarada{Nombre: "a", Valor: texto("${var.b}")},
					dominio.VariableDePipelineDeclarada{Nombre: "b", Valor: texto("x-${var.a}")},
				)
			},
			invariante: dominio.Variables, fichero: "variables/sand/despliegue.yaml", ambiente: "sand",
			detalle: "se usan en círculo: a → b → a",
		},
		{
			nombre: "variables compartidas que se usan en círculo",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Variables[0].Variables = append(d.Variables[0].Variables,
					dominio.VariableDePipelineDeclarada{Nombre: "a", Valor: texto("${var.b}")},
					dominio.VariableDePipelineDeclarada{Nombre: "b", Valor: texto("x-${var.a}")},
				)
			},
			invariante: dominio.Variables, fichero: "variables/compartidas.yaml",
			detalle: "se usan en círculo: a → b → a",
		},

		// Las variables de salida.
		{
			nombre: "una expresión regular mal formada",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[0].Comandos.Datos[0].Variables[0].Expresion = `registro = "(`
			},
			invariante: dominio.VariablesDeSalida, fichero: "steps/01-registro/commands.yaml",
			detalle: `la expresión regular de "registro" no es correcta`,
		},
		{
			nombre:     "una variable de salida sin probe",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Pasos[0].Comandos.Datos[0].Variables[0].Expresion = "" },
			invariante: dominio.VariablesDeSalida, fichero: "steps/01-registro/commands.yaml",
			detalle: `"registro" no dice probe`,
		},
		{
			nombre: "una variable de salida producida dos veces por el mismo comando",
			mutar: func(d *dominio.PipelineDeclarado) {
				c := &d.Pasos[0].Comandos.Datos[0]
				c.Variables = append(c.Variables, dominio.VariableDeComandoDeclarada{Nombre: "registro", Expresion: "(.*)", Ambito: "shared"})
			},
			invariante: dominio.VariablesDeSalida, fichero: "steps/01-registro/commands.yaml",
			detalle: `el comando 1 produce "registro" dos veces`,
		},
		{
			nombre: "el mismo nombre producido por dos comandos del mismo ámbito",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos[1].Variables = []dominio.VariableDeComandoDeclarada{{Nombre: "ip", Expresion: "(.*)"}}
			},
			invariante: dominio.VariablesDeSalida, fichero: "steps/02-despliegue/commands.yaml",
			detalle: `el comando 2 produce "ip", que ya produce el comando 1 de "despliegue"`,
		},
		{
			nombre: "el mismo nombre producido en dos ámbitos",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[1].Comandos.Datos[1].Variables = []dominio.VariableDeComandoDeclarada{
					{Nombre: "registro", Expresion: "(.*)"},
				}
			},
			invariante: dominio.VariablesDeSalida, fichero: "steps/02-despliegue/commands.yaml",
			detalle: "produce en el otro ámbito: un nombre pertenece a un solo ámbito",
		},

		// Las aserciones.
		{
			nombre: "la expresión regular de una aserción mal formada",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Pasos[0].Comandos.Datos[0].Variables[1].Expresion = "Apply ("
			},
			invariante: dominio.Aserciones, fichero: "steps/01-registro/commands.yaml",
			detalle: "la expresión regular de una aserción no es correcta",
		},

		// Los ambientes.
		{
			nombre:     "sin environments.yaml",
			mutar:      func(d *dominio.PipelineDeclarado) { d.Ambientes = dominio.Declarado[[]dominio.AmbienteDeclarado]{} },
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: "no está", varios: true,
		},
		{
			nombre: "sin ambientes",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ambientes = dominio.Declarado[[]dominio.AmbienteDeclarado]{Existe: true}
			},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: "no declara ningún ambiente", varios: true,
		},
		{
			nombre: "un value repetido, aunque cambie de mayúsculas",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ambientes.Datos = append(d.Ambientes.Datos, dominio.AmbienteDeclarado{Nombre: "otra", Valor: "PROD"})
			},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: `el value "PROD" está dos veces`,
		},
		{
			nombre: "un name repetido",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ambientes.Datos = append(d.Ambientes.Datos, dominio.AmbienteDeclarado{Nombre: "sandbox", Valor: "otro"})
			},
			invariante: dominio.Ambientes, fichero: "environments.yaml", detalle: `el name "sandbox" está dos veces`,
		},
		{
			nombre: "un value que no sirve como directorio",
			mutar: func(d *dominio.PipelineDeclarado) {
				d.Ambientes.Datos = append(d.Ambientes.Datos, dominio.AmbienteDeclarado{Nombre: "staging", Valor: "stag/1"})
			},
			invariante: dominio.Formato, fichero: "environments.yaml", detalle: `value "stag/1"`,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			d := valida()
			caso.mutar(&d)
			lista := fallos(t, d)
			esperado := func(f dominio.Fallo) bool {
				return f.Invariante == caso.invariante && f.Fichero == caso.fichero && f.Ambiente == caso.ambiente &&
					strings.Contains(f.Detalle, caso.detalle)
			}
			encontrado := false
			for _, f := range lista {
				encontrado = encontrado || esperado(f)
			}
			require.True(t, encontrado, "no está el fallo esperado entre:\n%v", lista)
			if !caso.varios {
				require.Len(t, lista, 1, "un fallo no arrastra otros:\n%v", lista)
			}
		})
	}
}

func TestRenumerarUnPasoNoCambiaSuIdentidad(t *testing.T) {
	d := valida()
	d.Pasos[0].Directorio = "00-registro"
	d.Pasos[1].Directorio = "07-despliegue"

	antes, despues := comprobar(t, valida()).Pasos(), comprobar(t, d).Pasos()

	require.Equal(t, []int{1, 2}, []int{antes[0].Orden, antes[1].Orden})
	require.Equal(t, []int{0, 7}, []int{despues[0].Orden, despues[1].Orden})
	for i := range antes {
		require.Equal(t, antes[i].Nombre, despues[i].Nombre)
		antes[i].Orden, despues[i].Orden = 0, 0
		require.Equal(t, antes[i], despues[i], "lo único que cambia es el orden")
	}
}

func TestUnFicheroQueNoEstaEnTemplatesSeCopiaSinInterpolar(t *testing.T) {
	registro, _ := comprobar(t, valida()).Paso("registro")
	require.Equal(t, "terraform/main.tf", registro.Material[0].Ruta)
	require.False(t, registro.Material[0].Plantilla, "se copia tal cual")
	require.Equal(t, `name = "${var.name}"`, registro.Material[0].Contenido)

	despliegue, _ := comprobar(t, valida()).Paso("despliegue")
	require.True(t, despliegue.Material[0].Plantilla)
	require.False(t, despliegue.Material[1].Plantilla)

	t.Run("si está en templates, sus variables se comprueban", func(t *testing.T) {
		d := valida()
		d.Pasos[0].Comandos.Datos[0].Plantillas = []string{"main.tf"}
		lista := fallos(t, d)
		require.Len(t, lista, 1)
		require.Equal(t, "steps/01-registro/terraform/main.tf", lista[0].Fichero)
		require.Contains(t, lista[0].Detalle, "usa ${var.name}")
	})
}

func TestLasVariablesEstandarSeUsanSinDeclararlas(t *testing.T) {
	// valida usa project_id, project_name, project_hash, environment y step_workdir sin declararlas.
	comprobar(t, valida())

	clases := map[string]dominio.VariableEstandar{}
	for _, v := range dominio.VariablesEstandar() {
		clases[v.Nombre] = v
	}
	require.Len(t, clases, 11)
	require.Equal(t, dominio.Metadato, clases["project_name"].Origen)
	require.Equal(t, dominio.Metadato, clases["environment"].Origen, "es un dato de la solicitud")
	require.Equal(t, dominio.Generada, clases["project_hash"].Origen)
	require.False(t, clases["project_hash"].DelPaso)
	require.True(t, clases["step_workdir"].DelPaso)
	require.True(t, clases["step_name"].DelPaso)
	require.NotContains(t, clases, "project_revision", "se llama project_hash")
}

func TestNoHayFormaDeObtenerUnPipelineQueNoHayaPasadoLaComprobacion(t *testing.T) {
	t.Run("Comprobar es la única función que da un PipelineComprobado", func(t *testing.T) {
		entradas, err := os.ReadDir(".")
		require.NoError(t, err)
		var fabricas []string
		for _, entrada := range entradas {
			nombre := entrada.Name()
			if !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
				continue
			}
			fichero, err := parser.ParseFile(token.NewFileSet(), nombre, nil, 0)
			require.NoError(t, err)
			for _, decl := range fichero.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !fn.Name.IsExported() || fn.Type.Results == nil {
					continue
				}
				for _, resultado := range fn.Type.Results.List {
					tipo := resultado.Type
					if estrella, ok := tipo.(*ast.StarExpr); ok {
						tipo = estrella.X
					}
					if id, ok := tipo.(*ast.Ident); ok && id.Name == "PipelineComprobado" {
						fabricas = append(fabricas, fn.Name.Name)
					}
				}
			}
		}
		require.Equal(t, []string{"Comprobar"}, fabricas)
	})

	t.Run("el Pipeline no tiene campos que se puedan escribir desde fuera", func(t *testing.T) {
		tipo := reflect.TypeFor[dominio.PipelineComprobado]()
		for i := range tipo.NumField() {
			require.False(t, tipo.Field(i).IsExported(), tipo.Field(i).Name)
		}
	})

	t.Run("lo que devuelve es una copia", func(t *testing.T) {
		p := comprobar(t, valida())
		pasos := p.Pasos()
		pasos[0].Nombre = "otro"
		pasos[0].Comandos[0].Linea = "rm -rf /"
		pasos[0].Comandos[0].Aserciones[0].Expresion = "otra"
		pasos[1].Material[0].Plantilla = false
		p.Ambientes()[0].Valor = "otro"
		p.Variables()[0].Valor = "otro"

		require.Equal(t, comprobar(t, valida()).Pasos(), p.Pasos())
		require.Equal(t, comprobar(t, valida()).Ambientes(), p.Ambientes())
		require.Equal(t, comprobar(t, valida()).Variables(), p.Variables())
	})
}

func TestLosFallosDicenDondeEstan(t *testing.T) {
	d := valida()
	d.Pasos[1].Comandos.Datos[0].Linea += " ${var.equipo}"
	_, err := dominio.Comprobar(d)
	require.EqualError(t, err, "definicion: el pipeline no pasa la comprobación:\n"+
		"  - steps/02-despliegue/commands.yaml: en el ambiente prod, usa ${var.equipo}, que no es una variable estándar, "+
		"ni está declarada en un ámbito que se vea desde aquí, ni la produce ningún comando (variables)")
}
