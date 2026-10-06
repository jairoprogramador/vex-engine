package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"

	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

func TestPresentarDiagnostico_SinReferenciaSoloDiceQueNoHayDiagnosticoYPorQue(t *testing.T) {
	// El Diagnóstico trae un texto para esta forma; el motor no lo lleva: quien presenta decide qué decir.
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.SinReferencia, Mensaje: "no hay historial previo para poder diagnosticar"}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{"SinDiagnostico":"sin_referencia"}`, string(salida))
}

func TestPresentarDiagnostico_NoSeAtribuyeSoloDiceQueNoHayDiagnosticoYPorQue(t *testing.T) {
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.NoSeAtribuye}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{"SinDiagnostico":"no_se_atribuye"}`, string(salida))
}

func sustentoDeEjemplo() diagnosticopublicado.Sustento {
	instante := time.Date(2026, 10, 4, 22, 4, 47, 0, time.UTC)
	return diagnosticopublicado.Sustento{
		IntentoQueFalla:            "fallido",
		InstanteDelIntentoQueFalla: instante.Add(time.Hour),
		EjesCambiados: []diagnosticopublicado.CambioDeEje{
			{Eje: diagnosticopublicado.Codigo, Pasos: []string{"test"}},
			{Eje: diagnosticopublicado.Instrucciones, Pasos: []string{"test"}},
		},
		Comparaciones: []diagnosticopublicado.Comparacion{{
			Despliegue: "despliegue", Ambiente: "sand", Intento: "exitoso", Instante: instante,
			CantidadDeIntentos: 3, HayCantidadDeIntentos: true,
		}},
	}
}

func TestPresentarDiagnostico_ConAtribucionEsCompactoYSinTexto(t *testing.T) {
	r := diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustentoDeEjemplo()}

	salida, err := json.Marshal(presentarDiagnostico(r))

	require.NoError(t, err)
	require.JSONEq(t, `{
		"Ambiente": "sand",
		"IntentoExitoso": {"Id": "exitoso", "Fecha": "2026-10-04T22:04:47Z"},
		"IntentoFallido": {"Id": "fallido", "Fecha": "2026-10-04T23:04:47Z", "CantidadDeIntentos": 3},
		"Sustento": {
			"Codigo": {"Pasos": ["test"]},
			"Instrucciones": {"Pasos": ["test"]}
		}
	}`, string(salida))
}

func TestPresentarDiagnostico_OmiteLoQueNoCambio(t *testing.T) {
	sustento := sustentoDeEjemplo()
	sustento.EjesCambiados = sustento.EjesCambiados[:1]
	sustento.Comparaciones[0].HayCantidadDeIntentos = false

	salida, err := json.Marshal(presentarDiagnostico(
		diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento}))

	require.NoError(t, err)
	var vista struct {
		IntentoFallido map[string]any
		Sustento       map[string]any
	}
	require.NoError(t, json.Unmarshal(salida, &vista))
	require.ElementsMatch(t, []string{"Id", "Fecha"}, claves(vista.IntentoFallido))
	require.Equal(t, []string{"Codigo"}, claves(vista.Sustento))
}

func TestPresentarDiagnostico_UnaAtribucionVaciaEsConAtribucionConUnSustentoVacio(t *testing.T) {
	// ES-5: nada cambió en lo que mira cada paso. Es un hecho, no un error: el Sustento existe y no tiene ejes.
	sustento := sustentoDeEjemplo()
	sustento.EjesCambiados = nil

	salida, err := json.Marshal(presentarDiagnostico(
		diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento}))

	require.NoError(t, err)
	var vista map[string]any
	require.NoError(t, json.Unmarshal(salida, &vista))
	require.NotContains(t, vista, "SinDiagnostico", "hay diagnóstico: su sustento no tiene ejes porque nada cambió")
	require.Equal(t, map[string]any{}, vista["Sustento"], "sin ejes, pero presente: distinto de no tener diagnóstico")
	require.Contains(t, vista, "IntentoFallido")
}

func TestPresentarDiagnostico_VariablesSoloConCambios(t *testing.T) {
	sustento := sustentoDeEjemplo()
	sustento.VariablesProducidasCambiadas = []diagnosticopublicado.CambioDeVariable{{Paso: "test", Nombre: "url"}}

	salida, err := json.Marshal(presentarDiagnostico(
		diagnosticopublicado.Respuesta{Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento}))

	require.NoError(t, err)
	var vista struct {
		Sustento struct{ Variables map[string]any }
	}
	require.NoError(t, json.Unmarshal(salida, &vista))
	require.Equal(t, []string{"ProducidasCambiadas"}, claves(vista.Sustento.Variables))
}

func TestPresentarDiagnostico_NingunaRespuestaLlevaTextoParaElUsuario(t *testing.T) {
	sustento := sustentoDeEjemplo()
	sustento.VariablesDeclaradasCambiadas = []diagnosticopublicado.CambioDeVariable{{Paso: "test", Nombre: "a"}}
	sustento.VariablesProducidasCambiadas = []diagnosticopublicado.CambioDeVariable{{Paso: "test", Nombre: "b"}}
	sustento.EjesCambiados = append(sustento.EjesCambiados, diagnosticopublicado.CambioDeEje{Eje: diagnosticopublicado.Variables, Pasos: []string{"test"}})
	for nombre, r := range map[string]diagnosticopublicado.Respuesta{
		"sin referencia": {Forma: diagnosticopublicado.SinReferencia, Mensaje: "texto del contexto"},
		"no se atribuye": {Forma: diagnosticopublicado.NoSeAtribuye},
		"con atribución": {Forma: diagnosticopublicado.ConAtribucion, Sustento: sustento},
	} {
		t.Run(nombre, func(t *testing.T) {
			salida, err := json.Marshal(presentarDiagnostico(r))

			require.NoError(t, err)
			require.NotContains(t, string(salida), "Mensaje", "el motor da datos; quien presenta pone las palabras")
			require.NotContains(t, string(salida), "Forma", "el discriminador, cuando hace falta, es SinDiagnostico")
			require.NotContains(t, string(salida), "texto del contexto")
		})
	}
}

// --- por el protocolo ---

func TestDiagnosticar_SinHistorialPrevioRespondeQueNoHayDiagnosticoPorFaltaDeReferencia(t *testing.T) {
	e := nuevoEntorno(t)
	intento := intentar(t, e, e.intento(), salidaBien)

	r := invocar(t, e.rutas, "diagnosticar", `{"Version":"1","Ambiente":"prod","Intento":`+quote(intento)+`}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.JSONEq(t, `{"SinDiagnostico":"sin_referencia"}`, string(r.respuesta(t).Result))
}

func TestDiagnosticar_AmbienteSinIntentosRespondeQueNoHayDiagnosticoSinFallar(t *testing.T) {
	e := nuevoEntorno(t)

	r := invocar(t, e.rutas, "diagnosticar", `{"Version":"1","Ambiente":"sand"}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	require.JSONEq(t, `{"SinDiagnostico":"sin_referencia"}`, string(r.respuesta(t).Result))
}

// confirmar commitea un cambio en un repositorio que ya existe.
func confirmar(t *testing.T, dir, ruta, contenido string) {
	t.Helper()
	escribir(t, dir, ruta, contenido)
	repo, err := git.PlainOpen(dir)
	require.NoError(t, err)
	w, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, w.AddWithOptions(&git.AddOptions{All: true}))
	firma := firmante
	firma.When = firma.When.Add(time.Hour)
	_, err = w.Commit("cambio de prueba", &git.CommitOptions{Author: &firma})
	require.NoError(t, err)
}

func TestDiagnosticar_ConAtribucionRespondeLaEstructuraCompactaDeUnVistazo(t *testing.T) {
	e := nuevoEntorno(t)
	exitoso := intentar(t, e, e.intento(), salidaBien)
	confirmar(t, e.repoPipeline, "steps/05-deploy/commands.yaml", "- name: comando-roto\n  description: sale mal\n  cmd: exit 1\n")
	fallido := intentar(t, e, e.intento(), salidaFallo)

	r := invocar(t, e.rutas, "diagnosticar", `{"Version":"1","Ambiente":"prod","Intento":`+quote(fallido)+`}`)

	require.Equal(t, salidaBien, r.codigo, r.errores)
	var vista map[string]any
	r.resultado(t, &vista)
	require.ElementsMatch(t, []string{"Ambiente", "IntentoExitoso", "IntentoFallido", "Sustento"}, claves(vista),
		"de un vistazo: contra qué se comparó, qué intento falla y qué cambió; sin discriminador, porque hay diagnóstico")
	require.NotContains(t, vista, "SinDiagnostico", "hay diagnóstico")
	require.NotContains(t, vista, "Forma")
	require.Equal(t, "prod", vista["Ambiente"])
	require.Equal(t, exitoso, vista["IntentoExitoso"].(map[string]any)["Id"])
	require.Equal(t, fallido, vista["IntentoFallido"].(map[string]any)["Id"])
	sustento := vista["Sustento"].(map[string]any)
	require.Equal(t, []string{"Instrucciones"}, claves(sustento), "solo el eje que cambió")
	instrucciones := sustento["Instrucciones"].(map[string]any)
	require.Equal(t, []string{"Pasos"}, claves(instrucciones), "solo los pasos: sin texto para el usuario")
	require.Equal(t, []any{"deploy"}, instrucciones["Pasos"])
	require.NotContains(t, r.salida, "Atribucion", "no es la Respuesta completa del borde")
	require.NotContains(t, r.salida, "Mensaje")
}

func TestPresentarDiagnostico_SinDiagnosticoSoloEstaCuandoNoHayDiagnostico(t *testing.T) {
	// El campo explica por qué no hay diagnóstico: con uno, no aparece; sin él, es lo único que hay. Así quien lo lee
	// decide con una regla: si trae SinDiagnostico, no hay diagnóstico, y el valor dice el motivo.
	con, err := json.Marshal(presentarDiagnostico(diagnosticopublicado.Respuesta{
		Forma: diagnosticopublicado.ConAtribucion, Sustento: sustentoDeEjemplo(),
	}))
	require.NoError(t, err)
	require.NotContains(t, string(con), "SinDiagnostico")

	for forma, motivo := range map[diagnosticopublicado.FormaDeRespuesta]string{
		diagnosticopublicado.SinReferencia: "sin_referencia",
		diagnosticopublicado.NoSeAtribuye:  "no_se_atribuye",
	} {
		sin, err := json.Marshal(presentarDiagnostico(diagnosticopublicado.Respuesta{Forma: forma}))
		require.NoError(t, err)
		var vista map[string]any
		require.NoError(t, json.Unmarshal(sin, &vista))
		require.Equal(t, []string{"SinDiagnostico"}, claves(vista), "es lo único que hay")
		require.Equal(t, motivo, vista["SinDiagnostico"])
	}
}
