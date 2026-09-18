package aplicacion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

// El doble de este fichero es el puerto que el propio dominio de Diagnóstico declara (dominio.Historial)
// — nunca el ACL real: probar contra la frontera que Diagnóstico ya decidió es lo que mantiene un
// contexto aislado del otro (DEC-11.3).

func nuevoServicio(h *historialFalso) *aplicacion.Servicio {
	return aplicacion.NuevoServicio(aplicacion.Dependencias{Historial: h})
}

func idIntento(t *testing.T, valor string) dominio.IdIntento {
	t.Helper()
	i, err := dominio.NuevoIdIntento(valor)
	require.NoError(t, err)
	return i
}

func idDespliegue(t *testing.T, valor string) dominio.IdDespliegue {
	t.Helper()
	d, err := dominio.NuevoIdDespliegue(valor)
	require.NoError(t, err)
	return d
}

func idLanzamiento(t *testing.T, valor string) dominio.IdLanzamiento {
	t.Helper()
	l, err := dominio.NuevoIdLanzamiento(valor)
	require.NoError(t, err)
	return l
}

func ambiente(t *testing.T, valor string) dominio.Ambiente {
	t.Helper()
	a, err := dominio.NuevaAmbiente(valor)
	require.NoError(t, err)
	return a
}

func hashDelCodigo(t *testing.T, valor string) dominio.HashDelCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDelCodigo(valor)
	require.NoError(t, err)
	return h
}

func hashDeInstrucciones(t *testing.T, valor string) dominio.HashDeInstrucciones {
	t.Helper()
	h, err := dominio.NuevoHashDeInstrucciones(valor)
	require.NoError(t, err)
	return h
}

func nombrePaso(t *testing.T, valor string) dominio.NombrePaso {
	t.Helper()
	p, err := dominio.NuevoNombrePaso(valor)
	require.NoError(t, err)
	return p
}

func nombreDeVariable(t *testing.T, valor string) dominio.NombreDeVariable {
	t.Helper()
	n, err := dominio.NuevoNombreDeVariable(valor)
	require.NoError(t, err)
	return n
}

func hashDeVariable(t *testing.T, valor string) dominio.HashDeVariable {
	t.Helper()
	h, err := dominio.NuevoHashDeVariable(valor)
	require.NoError(t, err)
	return h
}

// ejesDeUnPaso construye EjesDeUnPaso para las pruebas del servicio, con la variable declarada que hace
// falta en cada caso.
func ejesDeUnPaso(t *testing.T, paso, codigo, instrucciones string, declaradas map[string]string, porEvidencia bool) dominio.EjesDeUnPaso {
	t.Helper()
	mapa := map[dominio.NombreDeVariable]dominio.HashDeVariable{}
	for nombre, hash := range declaradas {
		mapa[nombreDeVariable(t, nombre)] = hashDeVariable(t, hash)
	}
	return dominio.NuevosEjesDeUnPaso(
		nombrePaso(t, paso), hashDelCodigo(t, codigo), hashDeInstrucciones(t, instrucciones), mapa, porEvidencia,
	)
}

func producidasDeUnPaso(t *testing.T, paso string, producidas map[string]string) dominio.ProducidasDeUnPaso {
	t.Helper()
	mapa := map[dominio.NombreDeVariable]dominio.HashDeVariable{}
	for nombre, hash := range producidas {
		mapa[nombreDeVariable(t, nombre)] = hashDeVariable(t, hash)
	}
	return dominio.NuevasProducidasDeUnPaso(nombrePaso(t, paso), mapa)
}

type recursosDePrueba struct {
	ejes       []dominio.EjesDeUnPaso
	producidas []dominio.ProducidasDeUnPaso
}

type referenciaDePrueba struct {
	referencia dominio.Referencia
	producidas []dominio.ProducidasDeUnPaso
}

// historialFalso guarda en memoria lo que Diagnóstico le pide al Historial, para que las pruebas
// comprueben el algoritmo de PreguntarLaCausa sin depender del ACL real.
type historialFalso struct {
	intentos                map[string]dominio.IntentoDeDiagnostico
	ultimoIntentoDeAmbiente map[string]dominio.IdIntento
	despliegueDeLanzamiento map[string]dominio.IdDespliegue
	despliegues             map[string]dominio.DespliegueDeDiagnostico
	ultimoDelMismoAmbiente  map[string]dominio.DespliegueDeDiagnostico
	ultimoConHash           map[string]dominio.DespliegueDeDiagnostico
	recursosDelIntento      map[string]recursosDePrueba
	referencias             map[string]referenciaDePrueba
	cantidadDeIntentos      int

	llamadoEjesDelIntento bool
	llamadoReferencia     bool
}

func nuevoHistorialFalso() *historialFalso {
	return &historialFalso{
		intentos:                map[string]dominio.IntentoDeDiagnostico{},
		ultimoIntentoDeAmbiente: map[string]dominio.IdIntento{},
		despliegueDeLanzamiento: map[string]dominio.IdDespliegue{},
		despliegues:             map[string]dominio.DespliegueDeDiagnostico{},
		ultimoDelMismoAmbiente:  map[string]dominio.DespliegueDeDiagnostico{},
		ultimoConHash:           map[string]dominio.DespliegueDeDiagnostico{},
		recursosDelIntento:      map[string]recursosDePrueba{},
		referencias:             map[string]referenciaDePrueba{},
	}
}

func (h *historialFalso) Intento(_ context.Context, id dominio.IdIntento) (dominio.IntentoDeDiagnostico, error) {
	i, ok := h.intentos[id.String()]
	if !ok {
		return dominio.IntentoDeDiagnostico{}, errors.New("intento desconocido en la prueba")
	}
	return i, nil
}

func (h *historialFalso) UltimoIntentoDeUnAmbiente(
	_ context.Context, amb dominio.Ambiente,
) (dominio.IdIntento, bool, error) {
	id, ok := h.ultimoIntentoDeAmbiente[amb.String()]
	return id, ok, nil
}

func (h *historialFalso) DespliegueDeUnLanzamiento(
	_ context.Context, lanzamiento dominio.IdLanzamiento,
) (dominio.IdDespliegue, error) {
	id, ok := h.despliegueDeLanzamiento[lanzamiento.String()]
	if !ok {
		return dominio.IdDespliegue{}, errors.New("lanzamiento desconocido en la prueba")
	}
	return id, nil
}

func (h *historialFalso) Despliegue(_ context.Context, id dominio.IdDespliegue) (dominio.DespliegueDeDiagnostico, error) {
	d, ok := h.despliegues[id.String()]
	if !ok {
		return dominio.DespliegueDeDiagnostico{}, errors.New("despliegue desconocido en la prueba")
	}
	return d, nil
}

func (h *historialFalso) UltimoDespliegueAnteriorA(
	_ context.Context, amb dominio.Ambiente, _ time.Time,
) (dominio.DespliegueDeDiagnostico, bool, error) {
	d, ok := h.ultimoDelMismoAmbiente[amb.String()]
	return d, ok, nil
}

func (h *historialFalso) UltimoDespliegueConHashDelCodigo(
	_ context.Context, amb dominio.Ambiente, hash dominio.HashDelCodigo,
) (dominio.DespliegueDeDiagnostico, bool, error) {
	d, ok := h.ultimoConHash[amb.String()+"|"+hash.String()]
	return d, ok, nil
}

func (h *historialFalso) EjesDelIntento(
	_ context.Context, intento dominio.IdIntento,
) ([]dominio.EjesDeUnPaso, []dominio.ProducidasDeUnPaso, error) {
	h.llamadoEjesDelIntento = true
	r, ok := h.recursosDelIntento[intento.String()]
	if !ok {
		return nil, nil, errors.New("recursos desconocidos en la prueba")
	}
	return r.ejes, r.producidas, nil
}

func (h *historialFalso) Referencia(
	_ context.Context, despliegue dominio.DespliegueDeDiagnostico, _ dominio.RazonDeReferencia,
) (dominio.Referencia, []dominio.ProducidasDeUnPaso, error) {
	h.llamadoReferencia = true
	r, ok := h.referencias[despliegue.Id.String()]
	if !ok {
		return dominio.Referencia{}, nil, errors.New("referencia desconocida en la prueba")
	}
	return r.referencia, r.producidas, nil
}

func (h *historialFalso) CantidadDeIntentos(
	_ context.Context, _ dominio.IdDespliegue, _ dominio.IdIntento,
) (int, error) {
	return h.cantidadDeIntentos, nil
}

var _ dominio.Historial = (*historialFalso)(nil)
