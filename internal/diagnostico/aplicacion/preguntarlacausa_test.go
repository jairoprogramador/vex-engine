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
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c2"),
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

func TestPreguntarLaCausa_ES2_FuncionoEnStagingFallaEnProduccion(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
		OrdenDeAmbientes: []dominio.Ambiente{ambiente(t, "stag"), ambiente(t, "prod")},
	}
	h.ultimoConHash["stag|c1"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-stag"), Ambiente: ambiente(t, "stag"), Intento: idIntento(t, "i-stag"), Instante: t0,
	}
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h3"}, false)},
	}
	h.referencias["d-stag"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-stag"), ambiente(t, "stag"), t0, dominio.AmbienteAnterior,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h2"}, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.ConAtribucion, r.Forma)
	require.Equal(t, []publicado.Eje{publicado.Variables}, r.Atribucion)
}

func TestPreguntarLaCausa_ES1MasES2_CasoNormal(t *testing.T) {
	h := nuevoHistorialFalso()
	t0 := time.Now().Add(-2 * time.Hour)
	t1 := time.Now()

	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: t1,
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
		OrdenDeAmbientes: []dominio.Ambiente{ambiente(t, "stag"), ambiente(t, "prod")},
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-prod"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-prod"), Instante: t0,
	}
	h.ultimoConHash["stag|c1"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-stag"), Ambiente: ambiente(t, "stag"), Intento: idIntento(t, "i-stag"), Instante: t0,
	}
	h.recursosDelIntento["i-falla"] = recursosDePrueba{
		ejes: []dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h3"}, false)},
	}
	h.referencias["d-prod"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-prod"), ambiente(t, "prod"), t0, dominio.MismoAmbiente,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c0", "i1", map[string]string{"v": "h1"}, false)}, nil,
		),
	}
	h.referencias["d-stag"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-stag"), ambiente(t, "stag"), t0, dominio.AmbienteAnterior,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", map[string]string{"v": "h2"}, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.ConAtribucion, r.Forma)
	require.Equal(t, []publicado.Eje{publicado.Variables}, r.Atribucion, "el caso normal deja un solo candidato: las variables")
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
		Estado: dominio.Exitoso, HashDelCodigo: hashDelCodigo(t, "c2"),
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
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
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
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
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
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
	require.False(t, h.llamadoEjesDelIntento)
	require.False(t, h.llamadoReferencia)
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
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
		OrdenDeAmbientes: []dominio.Ambiente{ambiente(t, "stag"), ambiente(t, "prod")},
	}
	h.ultimoDelMismoAmbiente["prod"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-prod"), Ambiente: ambiente(t, "prod"), Intento: idIntento(t, "i-prod"), Instante: t0,
	}
	h.ultimoConHash["stag|c1"] = dominio.DespliegueDeDiagnostico{
		Id: idDespliegue(t, "d-stag"), Ambiente: ambiente(t, "stag"), Intento: idIntento(t, "i-stag"), Instante: t0,
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
	h.referencias["d-stag"] = referenciaDePrueba{
		referencia: dominio.NuevaReferencia(
			idDespliegue(t, "d-stag"), ambiente(t, "stag"), t0, dominio.AmbienteAnterior,
			[]dominio.EjesDeUnPaso{ejesDeUnPaso(t, "deploy", "c1", "i1", nil, false)}, nil,
		),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Intento: "i-falla"})
	require.NoError(t, err)
	require.Len(t, r.Sustento.Comparaciones, 2)
	for _, c := range r.Sustento.Comparaciones {
		if c.Razon == publicado.MismoAmbiente {
			require.True(t, c.HayCantidadDeIntentos)
			require.Equal(t, 3, c.CantidadDeIntentos)
		} else {
			require.False(t, c.HayCantidadDeIntentos, "la del ambiente anterior no cuenta intentos")
		}
	}
}

func TestPreguntarLaCausa_SinIntentoNiLanzamiento_TomaElUltimoDelAmbiente(t *testing.T) {
	h := nuevoHistorialFalso()
	h.ultimoIntentoDeAmbiente["prod"] = idIntento(t, "i-falla")
	h.intentos["i-falla"] = dominio.IntentoDeDiagnostico{
		Id: idIntento(t, "i-falla"), Ambiente: ambiente(t, "prod"), Instante: time.Now(),
		Estado: dominio.Fallido, HashDelCodigo: hashDelCodigo(t, "c1"),
	}

	r, err := nuevoServicio(h).PreguntarLaCausa(context.Background(), publicado.PeticionDeDiagnostico{Ambiente: "prod"})
	require.NoError(t, err)
	require.Equal(t, publicado.SinReferencia, r.Forma)
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
