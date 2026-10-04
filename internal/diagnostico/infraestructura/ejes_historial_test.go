package infraestructura_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	diagnosticodominio "github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	diagnosticoinfraestructura "github.com/jairoprogramador/vex-engine/internal/diagnostico/infraestructura"
	ejecuciondominio "github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	ejecucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	resoluciondominio "github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	resolucioninfraestructura "github.com/jairoprogramador/vex-engine/internal/resolucion/infraestructura"
)

// El ACL de Diagnóstico se prueba contra el Historial real, alimentado por los ACL reales de Ejecución y
// Resolución (DEC-11.3): es la única frontera donde un cambio en la forma que escriben esos dos contextos
// —"ejecucion/registro-v1", "ejecucion/apertura-v1", "resolucion/variable-v1"— se detectaría, porque
// Diagnóstico guarda su propia copia, más angosta, de esas formas (contenido.go).

type relojQueAvanza struct {
	mu    sync.Mutex
	ahora time.Time
}

func (r *relojQueAvanza) Ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ahora = r.ahora.Add(time.Second)
	return r.ahora
}

func nuevoHistorialReal(t *testing.T) (*historialaplicacion.Servicio, context.Context) {
	t.Helper()
	almacen := historialinfraestructura.NuevoAlmacenEnMemoria()
	servicio := historialaplicacion.NuevoServicio(historialaplicacion.Dependencias{
		Intentos:     historialinfraestructura.NuevosIntentos(almacen),
		Despliegues:  historialinfraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  historialinfraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: historialinfraestructura.NuevosLanzamientos(almacen),
		Reservas:     historialinfraestructura.NuevasReservas(almacen),
		Salidas:      historialinfraestructura.NuevasSalidas(almacen),
		Reloj:        &relojQueAvanza{ahora: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)},
		Identidades:  historialinfraestructura.IdentidadesUUID{},
	})
	return servicio, context.Background()
}

func pasoDePrueba(t *testing.T, nombre string) ejecuciondominio.PasoDelPipeline {
	t.Helper()
	p, err := ejecuciondominio.NuevoPasoDelPipeline(nombre, false)
	require.NoError(t, err)
	return p
}

func aperturaDePrueba(t *testing.T, ambiente string, orden []string) ejecuciondominio.AperturaDeIntento {
	t.Helper()
	hash, err := ejecuciondominio.NuevoHashDeCodigo("h1")
	require.NoError(t, err)
	return ejecuciondominio.AperturaDeIntento{
		Ambiente: ambiente, Solicitante: "ana", Pasos: []ejecuciondominio.PasoDelPipeline{pasoDePrueba(t, "deploy")},
		HastaPaso: "deploy", ConCommits: true, HashDelCodigo: hash,
		FuenteDelProyecto: "git@proyecto", CommitDelProyecto: "c-proyecto",
		FuenteDelPipeline: "git@pipeline", CommitDelPipeline: "c-pipeline",
		OrdenDeAmbientes: orden,
	}
}

func recursosDePrueba(t *testing.T, codigo, instrucciones string) ejecuciondominio.RecursosDeUnPaso {
	t.Helper()
	c, err := ejecuciondominio.NuevoHashDeCodigo(codigo)
	require.NoError(t, err)
	i, err := ejecuciondominio.NuevoHashDeInstrucciones(instrucciones)
	require.NoError(t, err)
	return ejecuciondominio.NuevosRecursosDeUnPaso(c, i, ejecuciondominio.NuevoHashDeVariables("v1"))
}

func ambitoDePrueba(t *testing.T, ambiente string) resoluciondominio.Ambito {
	t.Helper()
	a, err := resoluciondominio.AmbitoDeAmbiente(ambiente)
	require.NoError(t, err)
	return a
}

func idIntentoDiag(t *testing.T, valor string) diagnosticodominio.IdIntento {
	t.Helper()
	i, err := diagnosticodominio.NuevoIdIntento(valor)
	require.NoError(t, err)
	return i
}

// TestEjesDelIntento_HashesDeVerdad monta un intento completo con el ACL real de
// Ejecución (hashes de código e instrucciones) y de Resolución (una variable declarada), y comprueba que
// el ACL de Diagnóstico los decodifica correctamente — la única prueba capaz de detectar que su copia
// privada de esas formas se desincronizó de la de quien las escribe.
func TestEjesDelIntento_HashesDeVerdad(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	ejecucion := ejecucioninfraestructura.NuevoHistorial(h)
	resolucion := resolucioninfraestructura.NuevoHistorial(h, h)
	diagnostico := diagnosticoinfraestructura.NuevoHistorial(h)

	id, err := ejecucion.AbrirIntento(ctx, aperturaDePrueba(t, "prod", []string{"stag", "prod"}))
	require.NoError(t, err)

	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, ejecucion.RegistrarComienzo(ctx, id, "deploy", recursos))
	hashVariable := resoluciondominio.CalcularHashDeVariable("un-valor")
	require.NoError(t, resolucion.RegistrarVariable(
		ctx, id, "deploy", "v", hashVariable, resoluciondominio.OrigenDeclarada, ambitoDePrueba(t, "prod"),
	))
	require.NoError(t, ejecucion.RegistrarFinal(ctx, id, "deploy", true, recursos))

	intento, err := diagnostico.Intento(ctx, idIntentoDiag(t, id))
	require.NoError(t, err)
	require.Equal(t, ambienteDiag(t, "prod"), intento.Ambiente)

	ejes, producidas, err := diagnostico.EjesDelIntento(ctx, idIntentoDiag(t, id))
	require.NoError(t, err)
	require.Len(t, ejes, 1)
	require.Equal(t, "deploy", ejes[0].Paso().String())
	require.Equal(t, "c1", ejes[0].HashDelCodigo().String())
	require.Equal(t, "i1", ejes[0].HashDeInstrucciones().String())
	require.False(t, ejes[0].ComparadoPorEvidencia())
	hash, ok := ejes[0].VariableDeclarada(nombreDeVariableDiag(t, "v"))
	require.True(t, ok)
	require.Equal(t, hashVariable.String(), hash.String())
	require.Empty(t, producidas[0].Nombres(), "no se registró ninguna variable producida")
}

// TestEjesDelIntento_VariableProducidaNoEsEje comprueba la invariante «una variable producida no es eje»
// (DEC-06.10) contra la separación real que hace el ACL de Resolución.
func TestEjesDelIntento_VariableProducidaNoEsEje(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	ejecucion := ejecucioninfraestructura.NuevoHistorial(h)
	resolucion := resolucioninfraestructura.NuevoHistorial(h, h)
	diagnostico := diagnosticoinfraestructura.NuevoHistorial(h)

	id, err := ejecucion.AbrirIntento(ctx, aperturaDePrueba(t, "prod", nil))
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, ejecucion.RegistrarComienzo(ctx, id, "deploy", recursos))
	hashProducida := resoluciondominio.CalcularHashDeVariable("url-real")
	require.NoError(t, resolucion.RegistrarVariable(
		ctx, id, "deploy", "url", hashProducida, resoluciondominio.OrigenProducida, ambitoDePrueba(t, "prod"),
	))
	require.NoError(t, ejecucion.RegistrarFinal(ctx, id, "deploy", true, recursos))

	ejes, producidas, err := diagnostico.EjesDelIntento(ctx, idIntentoDiag(t, id))
	require.NoError(t, err)
	_, esDeclarada := ejes[0].VariableDeclarada(nombreDeVariableDiag(t, "url"))
	require.False(t, esDeclarada, "una producida nunca es eje")
	hash, esProducida := producidas[0].Variable(nombreDeVariableDiag(t, "url"))
	require.True(t, esProducida)
	require.Equal(t, hashProducida.String(), hash.String())
}

// TestEjesDelIntento_SigueLaEvidenciaEntreAmbientes es ES-4: un paso que no se re-ejecutó al pasar a otro
// ambiente compara con los recursos del registro al que apunta su evidencia (DEC-06.5).
func TestEjesDelIntento_SigueLaEvidenciaEntreAmbientes(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	ejecucion := ejecucioninfraestructura.NuevoHistorial(h)
	diagnostico := diagnosticoinfraestructura.NuevoHistorial(h)

	idStag, err := ejecucion.AbrirIntento(ctx, aperturaDePrueba(t, "stag", nil))
	require.NoError(t, err)
	recursosOriginales := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, ejecucion.RegistrarComienzo(ctx, idStag, "deploy", recursosOriginales))
	require.NoError(t, ejecucion.RegistrarFinal(ctx, idStag, "deploy", true, recursosOriginales))
	_, _, err = ejecucion.CerrarIntento(ctx, idStag, ejecuciondominio.Exitoso, "")
	require.NoError(t, err)

	idProd, err := ejecucion.AbrirIntento(ctx, aperturaDePrueba(t, "prod", nil))
	require.NoError(t, err)
	evidencia := ejecuciondominio.Evidencia{Intento: idStag, Paso: "deploy"}
	require.NoError(t, ejecucion.RegistrarNoReejecucion(ctx, idProd, "deploy", evidencia, recursosOriginales))

	ejes, _, err := diagnostico.EjesDelIntento(ctx, idIntentoDiag(t, idProd))
	require.NoError(t, err)
	require.Len(t, ejes, 1)
	require.True(t, ejes[0].ComparadoPorEvidencia())
	require.Equal(t, "c1", ejes[0].HashDelCodigo().String())
	require.Equal(t, "i1", ejes[0].HashDeInstrucciones().String())
}

// TestReferencia_ArmaLaReferenciaDesdeUnDespliegue comprueba la factoría para el lado de la referencia.
func TestReferencia_ArmaLaReferenciaDesdeUnDespliegue(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	ejecucion := ejecucioninfraestructura.NuevoHistorial(h)
	diagnostico := diagnosticoinfraestructura.NuevoHistorial(h)

	id, err := ejecucion.AbrirIntento(ctx, aperturaDePrueba(t, "prod", nil))
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, ejecucion.RegistrarComienzo(ctx, id, "deploy", recursos))
	require.NoError(t, ejecucion.RegistrarFinal(ctx, id, "deploy", true, recursos))
	despliegueId, hubo, err := ejecucion.CerrarIntento(ctx, id, ejecuciondominio.Exitoso, "")
	require.NoError(t, err)
	require.True(t, hubo)

	idDespliegueDiag, err := diagnosticodominio.NuevoIdDespliegue(despliegueId)
	require.NoError(t, err)
	despliegueDiag := diagnosticodominio.DespliegueDeDiagnostico{
		Id: idDespliegueDiag, Ambiente: ambienteDiag(t, "prod"), Intento: idIntentoDiag(t, id),
	}

	referencia, _, err := diagnostico.Referencia(ctx, despliegueDiag, diagnosticodominio.MismoAmbiente)
	require.NoError(t, err)
	require.Equal(t, diagnosticodominio.MismoAmbiente, referencia.Razon())
	require.Len(t, referencia.Ejes(), 1)
	require.Equal(t, "c1", referencia.Ejes()[0].HashDelCodigo().String())
}

func ambienteDiag(t *testing.T, valor string) diagnosticodominio.Ambiente {
	t.Helper()
	a, err := diagnosticodominio.NuevaAmbiente(valor)
	require.NoError(t, err)
	return a
}

func nombreDeVariableDiag(t *testing.T, valor string) diagnosticodominio.NombreDeVariable {
	t.Helper()
	n, err := diagnosticodominio.NuevoNombreDeVariable(valor)
	require.NoError(t, err)
	return n
}
