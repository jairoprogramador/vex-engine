package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/protocolo"
)

func peticionDeCatalogo(e entorno) string {
	return `{"Version":"1","FuenteDelPipeline":` + quote(e.repoPipeline) + `}`
}

type ambienteDelCatalogo struct {
	Nombre, Valor string
	Reservado     bool
}

func TestAmbientes_ListaLosDelPipelineYDiceCualesEstanReservados(t *testing.T) {
	e := nuevoEntorno(t)
	reservar := invocar(t, e.rutas, "reservar", `{"Version":"1","Ambiente":"prod"}`)
	require.Equal(t, salidaBien, reservar.codigo, reservar.errores)

	r := invocar(t, e.rutas, "ambientes", peticionDeCatalogo(e))

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var ambientes []ambienteDelCatalogo
	r.resultado(t, &ambientes)
	require.NotEmpty(t, ambientes)
	reservados := map[string]bool{}
	for _, a := range ambientes {
		reservados[a.Valor] = a.Reservado
	}
	require.Contains(t, reservados, "prod")
	require.True(t, reservados["prod"])
	require.Contains(t, reservados, "sand")
	require.False(t, reservados["sand"], "un ambiente que nunca se reservó no lo está")
}

func TestAmbientes_LiberarQuitaLaReserva(t *testing.T) {
	e := nuevoEntorno(t)
	invocar(t, e.rutas, "reservar", `{"Version":"1","Ambiente":"prod"}`)
	invocar(t, e.rutas, "liberar", `{"Version":"1","Ambiente":"prod"}`)

	var ambientes []ambienteDelCatalogo
	invocar(t, e.rutas, "ambientes", peticionDeCatalogo(e)).resultado(t, &ambientes)

	for _, a := range ambientes {
		require.False(t, a.Reservado, a.Valor)
	}
}

func TestPasos_ListaLosDelPipelineEnSuOrden(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "pasos", peticionDeCatalogo(e))

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var pasos []struct {
		Nombre     string
		Orden      int
		Compartido bool
	}
	r.resultado(t, &pasos)
	carpetas, err := os.ReadDir(filepath.Join(pipelineDeEjemplo, "steps"))
	require.NoError(t, err)
	require.Len(t, pasos, len(carpetas), "un paso por carpeta de steps/ del pipeline de ejemplo")
	for i, p := range pasos {
		require.NotEmpty(t, p.Nombre)
		require.Equal(t, i+1, p.Orden)
		require.Contains(t, carpetas[i].Name(), p.Nombre, "en el orden de las carpetas")
	}
}

func TestAmbientesYPasos_UnaFuenteQueNoExisteEsUnaPeticionInvalida(t *testing.T) {
	e := nuevoEntorno(t)
	for _, operacion := range []string{"ambientes", "pasos"} {
		t.Run(operacion, func(t *testing.T) {
			r := invocar(t, e.rutas, operacion, `{"Version":"1","FuenteDelPipeline":"/no/existe"}`)

			require.Equal(t, salidaInvalida, r.codigo, r.errores)
			require.Equal(t, tipoParametrosInvalidos, r.error(t).Error.Data["tipo"])
		})
	}
}

func TestAmbientesYPasos_UnPipelineQueNoPasaLaComprobacionEsRechazadoConSusFallos(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) { escribir(t, dir, "config.yaml", "schema_version: 99\n") })

	r := invocar(t, e.rutas, "pasos", peticionDeCatalogo(e))

	require.Equal(t, salidaFallo, r.codigo, r.errores)
	respuesta := r.error(t)
	require.Equal(t, tipoRechazado, respuesta.Error.Data["tipo"])
	require.NotEmpty(t, respuesta.Error.Data["fallos"])
}

func TestLanzar_DevuelveElIdYLanzamientosLoListaConEseId(t *testing.T) {
	e := nuevoEntorno(t)
	intentar(t, e, e.intento(), salidaBien)
	var despliegues []struct{ Id string }
	invocar(t, e.rutas, "despliegues", `{"Version":"1","Ambiente":"prod"}`).resultado(t, &despliegues)
	require.Len(t, despliegues, 1)

	lanzar := invocar(t, e.rutas, "lanzar",
		`{"Version":"1","Ambiente":"prod","Despliegue":`+quote(despliegues[0].Id)+`,"Nombre":"estreno"}`)

	require.Equal(t, salidaBien, lanzar.codigo, lanzar.errores)
	var lanzado struct{ Id, Despliegue, Nombre string }
	lanzar.resultado(t, &lanzado)
	require.NotEmpty(t, lanzado.Id)

	var lanzamientos []struct{ Id, Despliegue, Nombre string }
	lista := invocar(t, e.rutas, "lanzamientos", `{"Version":"1","Ambiente":"prod"}`)
	require.Equal(t, salidaBien, lista.codigo, lista.errores)
	lista.resultado(t, &lanzamientos)
	require.Contains(t, lanzamientos, lanzado)
}

func TestLanzamientos_AmbienteSinLanzamientosEsUnaListaVacia(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "lanzamientos", `{"Version":"1","Ambiente":"sand"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.JSONEq(t, `[]`, string(r.respuesta(t).Result))
}

func TestParametrosInvalidos_DicenElCampoYElValorComoDatos(t *testing.T) {
	e := nuevoEntorno(t)
	casos := map[string]struct {
		operacion, params, campo, valor string
	}{
		"campo desconocido":       {"intentos", `{"Version":"1","Ambient":"prod"}`, "Ambient", ""},
		"tipo equivocado":         {"intentos", `{"Version":"1","Ambiente":5}`, "Ambiente", "number"},
		"valor que no vale":       {"logs", `{"Version":"1","Resultado":"roto"}`, "Resultado", "roto"},
		"ambiente vacío":          {"lanzamientos", `{"Version":"1"}`, "Ambiente", ""},
		"fuente vacía":            {"ambientes", `{"Version":"1"}`, "FuenteDelPipeline", ""},
		"intento y lanzamiento":   {"diagnosticar", `{"Version":"1","Ambiente":"prod","Intento":"i","Lanzamiento":"l"}`, "Lanzamiento", "l"},
		"despliegue sin ambiente": {"lanzar", `{"Version":"1","Despliegue":"d"}`, "Ambiente", ""},
		"simular sin solicitante": {"simular", `{"Version":"1","Ambiente":"prod","HastaPaso":"test","Fuente":"x"}`, "Solicitante", ""},
		"simular con una copia de trabajo que no existe": {"simular",
			`{"Version":"1","Ambiente":"prod","Solicitante":"ana","HastaPaso":"test","CopiaDeTrabajo":"/no/existe"}`,
			"CopiaDeTrabajo", "/no/existe"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := invocar(t, e.rutas, c.operacion, c.params)

			require.Equal(t, salidaInvalida, r.codigo, r.errores)
			respuesta := r.error(t)
			require.Equal(t, protocolo.CodigoParametrosInvalidos, respuesta.Error.Code)
			require.Equal(t, tipoParametrosInvalidos, respuesta.Error.Data["tipo"])
			require.Equal(t, c.campo, respuesta.Error.Data["campo"], respuesta.Error.Data)
			require.Equal(t, c.valor, respuesta.Error.Data["valor"], respuesta.Error.Data)
		})
	}
}

func TestParametrosInvalidos_UnJSONMalFormadoNoApuntaANingunCampo(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "intentos", `"no es un objeto"`)

	require.Equal(t, salidaInvalida, r.codigo, r.errores)
	datos := r.error(t).Error.Data
	require.Equal(t, tipoParametrosInvalidos, datos["tipo"])
	require.NotContains(t, datos, "campo")
}

// Lo que el motor rechaza al intentar, al hacer rollback o al simular también dice qué campo no vale.
func TestParametrosInvalidos_DeIntentarRollbackYSimularDicenElCampo(t *testing.T) {
	e := nuevoEntorno(t)
	con := func(campos string) string {
		return `{"Version":"1","Solicitante":"ana",` + campos +
			`"FuenteDelProyecto":` + quote(e.repoProyecto) + `,"FuenteDelPipeline":` + quote(e.repoPipeline) + `}`
	}
	casos := map[string]struct {
		linea        string
		campo, valor string
	}{
		"ambiente que el pipeline no declara": {peticion("intentar", con(`"Ambiente":"dev",`)), "Ambiente", "dev"},
		"paso que el pipeline no declara":     {peticion("intentar", con(`"Ambiente":"prod","HastaPaso":"no-existe",`)), "HastaPaso", "no-existe"},
		"rollback sin despliegue":             {peticion("rollback", `{"Version":"1","Solicitante":"ana"}`), "Despliegue", ""},
		"fuente del proyecto vacía": {peticion("intentar",
			`{"Version":"1","Solicitante":"ana","Ambiente":"prod","FuenteDelPipeline":`+quote(e.repoPipeline)+`}`), "FuenteDelProyecto", ""},
		"entorno con un nombre inválido": {peticionConEntorno("intentar", con(`"Ambiente":"prod",`),
			map[string]string{"NOMBRE-MALO": valorSensible}), "Entorno", "NOMBRE-MALO"},
		"simular con un paso que no existe": {peticion("simular",
			`{"Version":"1","Ambiente":"prod","Solicitante":"ana","HastaPaso":"no-existe","CopiaDeTrabajo":`+quote(e.repoPipeline)+`}`),
			"HastaPaso", "no-existe"},
		"simular con un ambiente que no existe": {peticion("simular",
			`{"Version":"1","Ambiente":"dev","Solicitante":"ana","HastaPaso":"test","CopiaDeTrabajo":`+quote(e.repoPipeline)+`}`),
			"Ambiente", "dev"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			r := invocarLinea(t, e.rutas, c.linea)

			require.Equal(t, salidaInvalida, r.codigo, r.salida+r.errores)
			datos := r.error(t).Error.Data
			require.Equal(t, tipoParametrosInvalidos, datos["tipo"])
			require.Equal(t, c.campo, datos["campo"], datos)
			require.Equal(t, c.valor, datos["valor"], datos)
			require.NotContains(t, r.salida, valorSensible, "el valor de una variable de entorno nunca sale")
		})
	}
}
