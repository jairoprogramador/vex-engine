package infraestructura_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
)

// El adaptador se prueba contra el Historial real, sobre su almacén en memoria (DEC-11.3): es la frontera de
// otro contexto, y aquí solo se comprueba la traducción — nunca el propio Historial.

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

func pasoDePrueba(t *testing.T, nombre string) dominio.PasoDelPipeline {
	t.Helper()
	p, err := dominio.NuevoPasoDelPipeline(nombre, false)
	require.NoError(t, err)
	return p
}

func aperturaDePrueba(t *testing.T, ambiente string) dominio.AperturaDeIntento {
	t.Helper()
	hash, err := dominio.NuevoHashDeCodigo("h1")
	require.NoError(t, err)
	return dominio.AperturaDeIntento{
		Ambiente: ambiente, Solicitante: "ana", Pasos: []dominio.PasoDelPipeline{pasoDePrueba(t, "01-pruebas")},
		HastaPaso: "01-pruebas", ConCommits: true, HashDelCodigo: hash,
		FuenteDelProyecto: "git@proyecto", CommitDelProyecto: "c-proyecto",
		FuenteDelPipeline: "git@pipeline", CommitDelPipeline: "c-pipeline",
	}
}

func recursosDePrueba(t *testing.T, codigo, instrucciones string) dominio.RecursosDeUnPaso {
	t.Helper()
	c, err := dominio.NuevoHashDeCodigo(codigo)
	require.NoError(t, err)
	i, err := dominio.NuevoHashDeInstrucciones(instrucciones)
	require.NoError(t, err)
	return dominio.NuevosRecursosDeUnPaso(c, i, dominio.NuevoHashDeVariables("v1"))
}

func ambitoProd(t *testing.T) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)
	return a
}

func TestAdaptadorDeHistorial_AbrirRegistrarYCerrarUnIntentoExitoso(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	id, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	require.NotEmpty(t, id)

	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, id, "01-pruebas", true, recursos))

	despliegue, hubo, err := adaptador.CerrarIntento(ctx, id, dominio.Exitoso, "", "")
	require.NoError(t, err)
	require.True(t, hubo)
	require.NotEmpty(t, despliegue)
}

func TestAdaptadorDeHistorial_AbandonarUnIntentoLiberaSuAmbiente(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	id, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	_, err = adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.Error(t, err, "con el ambiente ocupado no se abre otro")

	require.NoError(t, adaptador.AbandonarIntento(ctx, id))

	_, err = adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err, "abandonado, el ambiente queda libre")
}

func TestAdaptadorDeHistorial_AbandonarUnIntentoQueNoExisteEsUnError(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)

	err := infraestructura.NuevoHistorial(h).AbandonarIntento(ctx, "no-existe")

	require.Error(t, err)
}

func TestAdaptadorDeHistorial_UltimaVezDeUnPasoSinRegistroNoHay(t *testing.T) {
	h, _ := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	u, err := adaptador.UltimaVezDeUnPaso(context.Background(), "01-pruebas", ambitoProd(t))
	require.NoError(t, err)
	require.False(t, u.Hay)
}

func TestAdaptadorDeHistorial_UltimaVezDeUnComienzoSinFinalNoEsValida(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	id, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id, "01-pruebas", recursosDePrueba(t, "c1", "i1")))

	u, err := adaptador.UltimaVezDeUnPaso(ctx, "01-pruebas", ambitoProd(t))
	require.NoError(t, err)
	require.True(t, u.Hay)
	require.False(t, u.Valida)
}

func TestAdaptadorDeHistorial_UltimaVezDeUnFinalFallidoNoEsValida(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	id, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, id, "01-pruebas", false, recursos))

	u, err := adaptador.UltimaVezDeUnPaso(ctx, "01-pruebas", ambitoProd(t))
	require.NoError(t, err)
	require.True(t, u.Hay)
	require.False(t, u.Valida)
}

func TestAdaptadorDeHistorial_UltimaVezDeUnFinalExitosoEsValidaYApuntaASiMisma(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	id, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, id, "01-pruebas", true, recursos))

	u, err := adaptador.UltimaVezDeUnPaso(ctx, "01-pruebas", ambitoProd(t))
	require.NoError(t, err)
	require.True(t, u.Hay)
	require.True(t, u.Valida)
	require.Equal(t, recursos.HashDelCodigo(), u.Recursos.HashDelCodigo())
	require.Equal(t, recursos.HashDeInstrucciones(), u.Recursos.HashDeInstrucciones())
	require.Equal(t, recursos.HashDeVariables(), u.Recursos.HashDeVariables(), "el hash de las variables viaja con el paso")
	require.Equal(t, dominio.Evidencia{Intento: id, Paso: "01-pruebas"}, u.Evidencia)
}

func TestAdaptadorDeHistorial_UltimaVezDeUnaNoReejecucionSigueElSaltoDeEvidencia(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	id1, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id1, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, id1, "01-pruebas", true, recursos))
	_, _, err = adaptador.CerrarIntento(ctx, id1, dominio.Exitoso, "", "")
	require.NoError(t, err)

	id2, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	evidenciaOriginal := dominio.Evidencia{Intento: id1, Paso: "01-pruebas"}
	require.NoError(t, adaptador.RegistrarNoReejecucion(ctx, id2, "01-pruebas", evidenciaOriginal, recursos))

	u, err := adaptador.UltimaVezDeUnPaso(ctx, "01-pruebas", ambitoProd(t))
	require.NoError(t, err)
	require.True(t, u.Valida)
	require.Equal(t, evidenciaOriginal, u.Evidencia, "la evidencia de una no-reejecución apunta al final original, nunca a sí misma")
}

func TestAdaptadorDeHistorial_DespliegueParaRollbackDaLasDosFuentes(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	apertura := aperturaDePrueba(t, "prod")
	id, err := adaptador.AbrirIntento(ctx, apertura)
	require.NoError(t, err)
	recursos := recursosDePrueba(t, "c1", "i1")
	require.NoError(t, adaptador.RegistrarComienzo(ctx, id, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, id, "01-pruebas", true, recursos))
	despliegue, hubo, err := adaptador.CerrarIntento(ctx, id, dominio.Exitoso, "", "")
	require.NoError(t, err)
	require.True(t, hubo)

	destino, err := adaptador.DespliegueParaRollback(ctx, despliegue)
	require.NoError(t, err)
	require.Equal(t, despliegue, destino.Despliegue())
	require.Equal(t, "prod", destino.Ambiente())
	require.Equal(t, apertura.FuenteDelProyecto, destino.FuenteDelProyecto())
	require.Equal(t, apertura.CommitDelProyecto, destino.CommitDelProyecto())
	require.Equal(t, apertura.FuenteDelPipeline, destino.FuenteDelPipeline())
	require.Equal(t, apertura.CommitDelPipeline, destino.CommitDelPipeline())
}

func TestAdaptadorDeHistorial_ElDetalleDistingueElPasoPrecargadoDelEjecutado(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	recursos := recursosDePrueba(t, "c1", "i1")

	primero, err := adaptador.AbrirIntento(ctx, aperturaDePrueba(t, "prod"))
	require.NoError(t, err)
	require.NoError(t, adaptador.RegistrarComienzo(ctx, primero, "01-pruebas", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, primero, "01-pruebas", true, recursos))
	_, _, err = adaptador.CerrarIntento(ctx, primero, dominio.Exitoso, "", "")
	require.NoError(t, err)

	aperturaConDosPasos := aperturaDePrueba(t, "prod")
	aperturaConDosPasos.Pasos = append(aperturaConDosPasos.Pasos, pasoDePrueba(t, "02-acr"))
	aperturaConDosPasos.HastaPaso = "02-acr"
	segundo, err := adaptador.AbrirIntento(ctx, aperturaConDosPasos)
	require.NoError(t, err)
	evidencia := dominio.Evidencia{Intento: primero, Paso: "01-pruebas"}
	require.NoError(t, adaptador.RegistrarNoReejecucion(ctx, segundo, "01-pruebas", evidencia, recursos))
	require.NoError(t, adaptador.RegistrarComienzo(ctx, segundo, "02-acr", recursos))
	require.NoError(t, adaptador.RegistrarFinal(ctx, segundo, "02-acr", false, recursos))
	_, _, err = adaptador.CerrarIntento(ctx, segundo, dominio.Fallido, "", "")
	require.NoError(t, err)

	detalle, err := adaptador.DetalleDelIntento(ctx, segundo)

	require.NoError(t, err)
	require.Equal(t, []dominio.PasoDelDetalle{
		{Nombre: "01-pruebas", Estado: dominio.PasoPrecargado},
		{Nombre: "02-acr", Estado: dominio.PasoFallido},
	}, detalle.Pasos)
	require.Positive(t, detalle.Tiempo)
}
