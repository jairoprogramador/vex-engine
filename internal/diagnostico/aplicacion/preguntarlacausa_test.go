package aplicacion_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

func TestPreguntarLaCausa_ES1_AyerFuncionoHoyFalla(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido,
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-ref"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-ref"), Instante: t0,
	}
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c2", "i1", map[string]string{"v": "h2"}, false)},
	}
	h.referencias["d-ref"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-ref"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h1"}, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.ConAtribucion, r.Forma)
	require.ElementsMatch(t, []publicado.Eje{publicado.Codigo, publicado.Variables}, r.Atribucion)
}

func TestPreguntarLaCausa_ES3_FallaAnteElCliente(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-time.Hour)
	t1 := time.Now()

	h.despliegueDeLanzamiento["l1"] = idDespliegue(t, "d-launched")
	h.despliegues["d-launched"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-launched"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-launched"), Instante: t1,
	}
	// La atribución no comprueba el estado real del intento (DEC-06.7): el dueño del negocio dice que
	// falla, y el core lo toma como premisa aunque el intento haya quedado exitoso.
	h.intentos["i-launched"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-launched"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Exitoso,
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-ref"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-ref"), Instante: t0,
	}
	h.recursosDelIntento["i-launched"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c2", "i1", nil, false)},
	}
	h.referencias["d-ref"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-ref"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", nil, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Lanzamiento: "l1"})
	require.NoError(t, err)
	require.Equal(t, publicado.ConAtribucion, r.Forma)
}

func TestPreguntarLaCausa_ES4_UnPasoNoSeReejecuto(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido,
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-ref"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-ref"), Instante: t0,
	}
	// El paso "supply" no se re-ejecutó: sus recursos vienen del registro al que apunta su evidencia,
	// posiblemente de otro ambiente (DEC-06.5).
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "supply", "c1", "i1", nil, true)},
	}
	h.referencias["d-ref"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-ref"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "supply", "c1", "i1", nil, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, []string{"supply"}, r.Sustento.PasosComparadosPorEvidencia)
	require.Len(t, r.Sustento.Comparaciones, 1)
	require.Equal(t, []string{"supply"}, r.Sustento.Comparaciones[0].PasosComparadosPorEvidencia)
}

func TestPreguntarLaCausa_ES5_NadaCambio(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido,
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-ref"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-ref"), Instante: t0,
	}
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes:       []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h1"}, false)},
		producidas: []dominio.ProducidasDeUnPaso{producidasDeUnPaso(t, "deploy", map[string]string{"url": "h2"})},
	}
	h.referencias["d-ref"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-ref"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h1"}, false)}, nil,
		),
		producidas: []dominio.ProducidasDeUnPaso{producidasDeUnPaso(t, "deploy", map[string]string{"url": "h1"})},
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.ConAtribucion, r.Forma)
	require.Empty(t, r.Atribucion, "ES-5: ninguno de los tres ejes cambió")
	require.Equal(
		t, []publicado.CambioDeVariable{{Paso: "deploy", Nombre: "url"}}, r.Sustento.VariablesProducidasCambiadas,
		"una producida cambiada aparece en el sustento aunque no sea candidata",
	)
}

func TestPreguntarLaCausa_ES6_SinReferencia(t *testing.T) {
	h := nuevoHistorialFalso()
	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: time.Now(),
		Estado: dominio.Fallido,
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
	require.Equal(t, "no hay historial previo para poder diagnosticar", r.Mensaje)
	require.False(t, h.llamadoEjesDelIntento)
	require.False(t, h.llamadoReferencia)
}

func TestPreguntarLaCausa_ES6_LaReferenciaEsElMismoIntento(t *testing.T) {
	h := nuevoHistorialFalso()
	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: time.Now(), Estado: dominio.Fallido,
	}
	h.despliegues["d-propio"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-propio"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-falla"),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(
		context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla", Referencia: "d-propio"},
	)
	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
	require.Equal(t, "no hay historial previo para poder diagnosticar", r.Mensaje)
	require.False(t, h.llamadoReferencia, "no se compara un intento consigo mismo")
}

func TestPreguntarLaCausa_ES7_CanceladoOSinDesenlace(t *testing.T) {
	casos := map[string]dominio.EstadoDeIntento{
		"cancelado":     dominio.Cancelado,
		"sin desenlace": "",
	}
	for nombre, estado := range casos {
		t.Run(nombre, func(t *testing.T) {
			h := nuevoHistorialFalso()
			h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
				Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: time.Now(), Estado: estado,
			}

			r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
			require.NoError(t, err)
			require.Equal(t, publicado.NoSeAtribuye, r.Forma)
			require.False(t, h.llamadoEjesDelIntento, "no se atribuye: no hace falta pedir los ejes")
			require.False(t, h.llamadoReferencia, "no se atribuye: no hace falta pedir la referencia")
		})
	}
}

func TestPreguntarLaCausa_ES8_CantidadDeIntentos(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-2 * time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido,
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-prod"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-prod"), Instante: t0,
	}
	h.cantidadDeIntentos = 3
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", nil, false)},
	}
	h.referencias["d-prod"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-prod"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", nil, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Len(t, r.Sustento.Comparaciones, 1)
	require.Equal(t, publicado.MismoAmbiente, r.Sustento.Comparaciones[0].Razon)
	require.True(t, r.Sustento.Comparaciones[0].HayCantidadDeIntentos)
	require.Equal(t, 3, r.Sustento.Comparaciones[0].CantidadDeIntentos)
}

func TestPreguntarLaCausa_SinIntentoNiLanzamiento_TomaElUltimoDelAmbiente(t *testing.T) {
	h := nuevoHistorialFalso()
	h.ultimoIntentoDeAmbiente["prod"] = idIntento(t, "i-falla")
	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: time.Now(),
		Estado: dominio.Fallido,
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Ambiente: "prod"})
	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
}

func TestPreguntarLaCausa_AmbienteSinIntentos_RespondeSinReferenciaSinError(t *testing.T) {
	h := nuevoHistorialFalso()

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Ambiente: "prod"})

	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
	require.Equal(t, dominio.MensajeSinHistorialPrevio, r.Mensaje)
}

func TestPreguntarLaCausa_UnAmbienteVacioEsInvalido(t *testing.T) {
	h := nuevoHistorialFalso()
	_, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{})
	require.ErrorIs(t, err, publicado.ErrInvalido)
}

func TestPreguntarLaCausa_PropagaElErrorDelHistorial(t *testing.T) {
	h := nuevoHistorialFalso()
	_, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-desconocido"})
	require.Error(t, err)
	require.NotErrorIs(t, err, publicado.ErrInvalido, "no es un error de validación, es del Historial")
}
