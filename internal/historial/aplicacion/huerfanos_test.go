package aplicacion_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Recuperar un ambiente cuyo dueño murió (EJ-6 / DEC-07.8): quien lo encuentra ocupado observa si el dueño sigue
// latiendo y, si no, lo cierra como fallido por interrumpido y ocupa el ambiente.

// ventanaDePrueba es lo bastante corta para no hacer lentas las pruebas, y lo bastante larga para que un latido
// cada 5 ms la vea crecer.
const ventanaDePrueba = 120 * time.Millisecond

// relojManual da siempre el mismo instante hasta que una prueba lo avanza: así decide si un latido «es reciente».
type relojManual struct {
	mu    sync.Mutex
	ahora time.Time
}

func (r *relojManual) Ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ahora
}

func (r *relojManual) Avanzar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ahora = r.ahora.Add(d)
}

func historialQueRecupera(almacen infraestructura.Almacen, reloj dominio.Reloj, ventana time.Duration) *aplicacion.Servicio {
	return aplicacion.NuevoServicio(aplicacion.Dependencias{
		Intentos:      infraestructura.NuevosIntentos(almacen),
		Despliegues:   infraestructura.NuevosDespliegues(almacen),
		Ocupaciones:   infraestructura.NuevasOcupaciones(almacen),
		Lanzamientos:  infraestructura.NuevosLanzamientos(almacen),
		Reservas:      infraestructura.NuevasReservas(almacen),
		Salidas:       infraestructura.NuevasSalidas(almacen),
		Latidos:       infraestructura.NuevosLatidos(almacen),
		Reloj:         reloj,
		Identidades:   infraestructura.IdentidadesUUID{},
		VentanaDeVida: ventana,
	})
}

// abrirCaido abre un intento, deja un paso con comienzo y sin final, y «muere»: no vuelve a escribir nada.
func abrirCaido(t *testing.T, h *aplicacion.Servicio, ambiente string) string {
	t.Helper()
	ctx := context.Background()
	id, err := h.AbrirIntento(ctx, apertura(ambiente))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id, "supply", nada))
	return id
}

// latirHasta escribe un latido cada 5 ms, como Ejecución, hasta que termina la prueba.
func latirHasta(t *testing.T, h *aplicacion.Servicio, intento string) {
	t.Helper()
	parar := make(chan struct{})
	terminada := make(chan struct{})
	go func() {
		defer close(terminada)
		for {
			_ = h.RegistrarLatido(context.Background(), intento)
			select {
			case <-parar:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	t.Cleanup(func() { close(parar); <-terminada })
}

func requerirOcupado(t *testing.T, err error, intento string) {
	t.Helper()
	var ocupado *publicado.AmbienteOcupadoError
	require.ErrorAs(t, err, &ocupado)
	require.Equal(t, intento, ocupado.Intento)
}

func TestHuerfanos_UnDuenoMuertoSeCierraComoInterrumpidoYElAmbienteSeOcupa(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	muerto := abrirCaido(t, historialSobre(almacen), "prod")
	nuevo := historialQueRecupera(almacen, &relojManual{ahora: t0}, ventanaDePrueba)
	ctx := context.Background()

	id, err := nuevo.AbrirIntento(ctx, apertura("prod"))

	require.NoError(t, err, "un proceso caído no puede bloquear el ambiente para siempre")
	require.NotEqual(t, muerto, id)
	viejo, err := nuevo.Intento(ctx, muerto)
	require.NoError(t, err)
	require.Equal(t, publicado.Fallido, viejo.Estado, "cuenta como un error")
	require.Equal(t, publicado.CausaInterrumpido, viejo.Causa, "y dice por qué")
	require.False(t, viejo.Abandonado)
	require.Len(t, viejo.Registros, 1, "el paso que empezó y nunca terminó queda tal cual: es lo que ocurrió")
	actual, err := nuevo.Intento(ctx, id)
	require.NoError(t, err)
	require.Empty(t, actual.Estado, "el intento nuevo está en curso")
}

func TestHuerfanos_ElIntentoNuevoPuedeCerrarseConNormalidadTrasLaRecuperacion(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	abrirCaido(t, historialSobre(almacen), "prod")
	nuevo := historialQueRecupera(almacen, &relojManual{ahora: t0}, ventanaDePrueba)
	ctx := context.Background()
	id, err := nuevo.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	hacerPasos(t, nuevo, ctx, id, "supply", "deploy")

	despliegue, hay, err := nuevo.CerrarIntento(ctx, id, publicado.Exitoso, "", "")

	require.NoError(t, err)
	require.True(t, hay, "el intento que recuperó el ambiente llega a despliegue como cualquier otro")
	require.NotEmpty(t, despliegue.Id)
}

func TestHuerfanos_UnDuenoConLatidoRecienteNoHaceEsperar(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	reloj := &relojManual{ahora: t0}
	vivo := abrirCaido(t, historialQueRecupera(almacen, reloj, 10*time.Second), "prod")
	require.NoError(t, historialQueRecupera(almacen, reloj, 10*time.Second).RegistrarLatido(context.Background(), vivo))
	otro := historialQueRecupera(almacen, reloj, 10*time.Second)

	empezo := time.Now()
	_, err := otro.AbrirIntento(context.Background(), apertura("prod"))

	requerirOcupado(t, err, vivo)
	require.Less(t, time.Since(empezo), time.Second, "con un latido reciente no se espera la ventana de 10 s")
}

func TestHuerfanos_UnDuenoQueSigueLatiendoNoSeLibera(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	reloj := &relojManual{ahora: t0}
	h := historialQueRecupera(almacen, reloj, ventanaDePrueba)
	vivo := abrirCaido(t, h, "prod")
	latirHasta(t, h, vivo)
	// El último latido parece viejo por el reloj (un reloj desfasado, o un latido atrasado): solo mirar el reloj
	// diría que murió. Observar la ventana demuestra que sigue vivo.
	reloj.Avanzar(time.Hour)

	_, err := h.AbrirIntento(context.Background(), apertura("prod"))

	requerirOcupado(t, err, vivo)
	intento, errIntento := h.Intento(context.Background(), vivo)
	require.NoError(t, errIntento)
	require.Empty(t, intento.Estado, "no se cerró: un dueño vivo no se toca aunque el reloj diga otra cosa")
}

func TestHuerfanos_UnDuenoSinLatidosQueEscribeEnLaVentanaNoSeLibera(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	h := historialQueRecupera(almacen, &relojManual{ahora: t0}, ventanaDePrueba)
	vivo := abrirCaido(t, h, "prod")
	go func() {
		time.Sleep(ventanaDePrueba / 4)
		_ = h.RegistrarFinal(context.Background(), vivo, "supply", true, nada)
	}()

	_, err := h.AbrirIntento(context.Background(), apertura("prod"))

	requerirOcupado(t, err, vivo)
}

func TestHuerfanos_SinVentanaNoHayRecuperacion(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	muerto := abrirCaido(t, historialSobre(almacen), "prod")
	sinRecuperacion := historialQueRecupera(almacen, &relojManual{ahora: t0}, 0)

	_, err := sinRecuperacion.AbrirIntento(context.Background(), apertura("prod"))

	requerirOcupado(t, err, muerto)
}

func TestHuerfanos_UnIntentoQueOcupoElAmbienteSinLlegarAAbrirseSeAbandona(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	ctx := context.Background()
	// Lo que queda si la apertura no se pudo escribir: la ocupación existe, el intento no tiene ningún registro.
	ocupacion := dominio.NuevaOcupacion("prod")
	require.NoError(t, ocupacion.Ocupar("sin-apertura", t0, nil))
	require.NoError(t, infraestructura.NuevasOcupaciones(almacen).Anadir(ctx, ocupacion))
	h := historialQueRecupera(almacen, &relojManual{ahora: t0}, ventanaDePrueba)

	id, err := h.AbrirIntento(ctx, apertura("prod"))

	require.NoError(t, err)
	require.NotEqual(t, "sin-apertura", id)
}

func TestHuerfanos_UnIntentoQueTerminoMientrasTantoNoSeToca(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	h := historialQueRecupera(almacen, &relojManual{ahora: t0}, ventanaDePrueba)
	ctx := context.Background()
	lento := abrirCaido(t, h, "prod")
	go func() {
		time.Sleep(ventanaDePrueba / 4)
		_ = h.RegistrarFinal(ctx, lento, "supply", true, nada)
		_, _, _ = h.CerrarIntento(ctx, lento, publicado.Fallido, "", "")
	}()

	id, err := h.AbrirIntento(ctx, apertura("prod"))

	require.NoError(t, err, "si el dueño terminó solo, el ambiente queda libre y se ocupa")
	require.NotEqual(t, lento, id)
	cerrado, err := h.Intento(ctx, lento)
	require.NoError(t, err)
	require.Empty(t, cerrado.Causa, "lo cerró su propio proceso: no es una interrupción")
}

func TestHuerfanos_VariosRecuperadoresALaVezOcupanUnoSolo(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	muerto := abrirCaido(t, historialSobre(almacen), "prod")
	reloj := &relojManual{ahora: t0}
	const recuperadores = 4

	var (
		mu        sync.Mutex
		ganadores []string
		rechazos  []error
		espera    sync.WaitGroup
	)
	for range recuperadores {
		espera.Add(1)
		go func() {
			defer espera.Done()
			h := historialQueRecupera(almacen, reloj, ventanaDePrueba)
			id, err := h.AbrirIntento(context.Background(), apertura("prod"))
			if err == nil {
				// El que gana empieza a latir, como hace Ejecución nada más abrir.
				latirHasta(t, h, id)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				rechazos = append(rechazos, err)
				return
			}
			ganadores = append(ganadores, id)
		}()
	}
	espera.Wait()

	require.Len(t, ganadores, 1, "un ambiente nunca admite dos intentos a la vez")
	require.Len(t, rechazos, recuperadores-1)
	for _, err := range rechazos {
		requerirOcupado(t, err, ganadores[0])
	}
	viejo, err := historialSobre(almacen).Intento(context.Background(), muerto)
	require.NoError(t, err)
	require.Equal(t, publicado.CausaInterrumpido, viejo.Causa)
}

func TestHuerfanos_EsperarRespetaLaCancelacion(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	abrirCaido(t, historialSobre(almacen), "prod")
	h := historialQueRecupera(almacen, &relojManual{ahora: t0}, 10*time.Second)
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelar()

	empezo := time.Now()
	_, err := h.AbrirIntento(ctx, apertura("prod"))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(empezo), time.Second, "cancelar interrumpe la espera de la ventana")
}

func TestLatidos_CadaLatidoSeSumaYSeLeeElUltimo(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	reloj := &relojManual{ahora: t0}
	h := historialQueRecupera(almacen, reloj, ventanaDePrueba)
	ctx := context.Background()
	id := abrirCaido(t, h, "prod")
	latidos := infraestructura.NuevosLatidos(almacen)

	require.NoError(t, h.RegistrarLatido(ctx, id))
	reloj.Avanzar(5 * time.Second)
	require.NoError(t, h.RegistrarLatido(ctx, id))

	cantidad, err := latidos.Cantidad(ctx, dominio.IdIntento(id))
	require.NoError(t, err)
	require.Equal(t, 2, cantidad)
	ultimo, hay, err := latidos.Ultimo(ctx, dominio.IdIntento(id))
	require.NoError(t, err)
	require.True(t, hay)
	require.Equal(t, t0.Add(5*time.Second), ultimo)
}

func TestCierre_LaCausaDeUnErrorQuedaEnElIntento(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	_, _, err = h.CerrarIntento(ctx, id, publicado.Fallido, publicado.CierrePorError, "")

	require.NoError(t, err)
	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.Equal(t, publicado.Fallido, intento.Estado)
	require.Equal(t, publicado.CausaError, intento.Causa)
}

func TestCierre_SoloElHistorialPuedeEscribirInterrumpido(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	_, _, err = h.CerrarIntento(ctx, id, publicado.Fallido, publicado.CausaDeCierre(publicado.CausaInterrumpido), "")

	require.Error(t, err)
	require.True(t, errors.Is(err, publicado.ErrRechazado), "quien cierra no puede fingir una interrupción")
	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.Empty(t, intento.Estado, "el cierre rechazado no escribió nada")
}

func TestLatido_UnIntentoTerminadoNoSigueLatiendo(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	require.NoError(t, h.AbandonarIntento(ctx, id))

	err = h.RegistrarLatido(ctx, id)

	require.ErrorIs(t, err, publicado.ErrRechazado)
}

func TestLatido_UnIntentoQueNoExisteNoLate(t *testing.T) {
	h, ctx := nuevoHistorial()

	err := h.RegistrarLatido(ctx, "no-existe")

	require.ErrorIs(t, err, publicado.ErrNoExiste)
}
