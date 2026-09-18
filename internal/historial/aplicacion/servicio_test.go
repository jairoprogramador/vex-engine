package aplicacion_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Los escenarios del Historial, sobre el almacén en memoria (IT-11 DEC-11.3).

var t0 = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

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

func historialSobre(almacen infraestructura.Almacen) *aplicacion.Servicio {
	return aplicacion.NuevoServicio(aplicacion.Dependencias{
		Intentos:     infraestructura.NuevosIntentos(almacen),
		Despliegues:  infraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  infraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: infraestructura.NuevosLanzamientos(almacen),
		Reservas:     infraestructura.NuevasReservas(almacen),
		Reloj:        &relojQueAvanza{ahora: t0},
		Identidades:  infraestructura.IdentidadesUUID{},
	})
}

func nuevoHistorial() (*aplicacion.Servicio, context.Context) {
	ctx := context.Background()
	return historialSobre(infraestructura.NuevoAlmacenEnMemoria()), ctx
}

var nada = publicado.Contenido{}

func apertura(ambiente string) publicado.Apertura {
	return publicado.Apertura{
		Ambiente:      ambiente,
		Solicitante:   "ana",
		Pasos:         []publicado.PasoDeclarado{{Nombre: "supply", Compartido: true}, {Nombre: "deploy"}},
		HastaPaso:     "deploy",
		ConCommits:    true,
		HashDelCodigo: "h1",
	}
}

func hacerPasos(t *testing.T, h *aplicacion.Servicio, ctx context.Context, intento string, pasos ...string) {
	t.Helper()
	for _, paso := range pasos {
		require.NoError(t, h.RegistrarComienzo(ctx, intento, paso, nada))
		require.NoError(t, h.RegistrarFinal(ctx, intento, paso, true, nada))
	}
}

// intentar abre, hace bien los pasos pedidos y cierra.
func intentar(
	t *testing.T, h *aplicacion.Servicio, ctx context.Context, a publicado.Apertura, estado publicado.Estado,
	destino string,
) (string, publicado.Despliegue, bool) {
	t.Helper()
	id, err := h.AbrirIntento(ctx, a)
	require.NoError(t, err)
	for _, p := range a.Pasos {
		hacerPasos(t, h, ctx, id, p.Nombre)
		if p.Nombre == a.HastaPaso {
			break
		}
	}
	d, hay, err := h.CerrarIntento(ctx, id, estado, destino)
	require.NoError(t, err)
	return id, d, hay
}

// escuchaQueLee comprueba, al recibir el aviso, que el despliegue ya se puede leer.
type escuchaQueLee struct {
	t            *testing.T
	h            *aplicacion.Servicio
	eventos      []publicado.DespliegueRegistrado
	leidoAlAviso []publicado.Despliegue
}

func (e *escuchaQueLee) DespliegueRegistrado(ctx context.Context, evento publicado.DespliegueRegistrado) {
	e.eventos = append(e.eventos, evento)
	ultimo, ok, err := e.h.UltimoDespliegue(ctx, evento.Despliegue.Ambiente)
	require.NoError(e.t, err)
	require.True(e.t, ok)
	e.leidoAlAviso = append(e.leidoAlAviso, ultimo)
}

func escuchar(t *testing.T, h *aplicacion.Servicio) *escuchaQueLee {
	e := &escuchaQueLee{t: t, h: h}
	h.Escuchar(e)
	return e
}

func TestIntentar_UnIntentoCompletoCreaSuDespliegueYLoAnunciaDespuesDeEscribirlo(t *testing.T) {
	h, ctx := nuevoHistorial()
	escucha := escuchar(t, h)

	id, d, hay := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")

	require.True(t, hay)
	require.Equal(t, id, d.Intento)
	require.Empty(t, d.Padre)
	require.Len(t, escucha.eventos, 1)
	require.Equal(t, d, escucha.eventos[0].Despliegue)
	require.Equal(t, d, escucha.leidoAlAviso[0], "al avisar, el despliegue ya está escrito")

	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.Equal(t, publicado.Exitoso, intento.Estado)
	require.Len(t, intento.Registros, 4)
}

func TestAbrirIntento_EnUnAmbienteOcupadoSeRechazaDiciendoCual(t *testing.T) {
	h, ctx := nuevoHistorial()
	primero, err := h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)

	_, err = h.AbrirIntento(ctx, apertura("staging"))
	var ocupado *publicado.AmbienteOcupadoError
	require.ErrorAs(t, err, &ocupado)
	require.Equal(t, primero, ocupado.Intento)
	require.Equal(t, "staging", ocupado.Ambiente)
	require.ErrorIs(t, err, publicado.ErrRechazado)

	_, err = h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err, "otro ambiente está libre")

	_, _, err = h.CerrarIntento(ctx, primero, publicado.Fallido, "")
	require.NoError(t, err)
	_, err = h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err, "el cierre libera el ambiente")
}

func TestAbrirIntento_UnaAperturaIncompletaNoOcupaElAmbiente(t *testing.T) {
	h, ctx := nuevoHistorial()
	a := apertura("staging")
	a.HastaPaso = "notify"
	_, err := h.AbrirIntento(ctx, a)
	require.ErrorIs(t, err, publicado.ErrRechazado)

	_, err = h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)
}

func TestAbandonarIntento_LiberaElAmbienteYElIntentoNoAceptaMasRegistros(t *testing.T) {
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)

	require.NoError(t, h.AbandonarIntento(ctx, id))
	require.ErrorIs(t, h.RegistrarComienzo(ctx, id, "supply", nada), publicado.ErrRechazado)
	require.ErrorIs(t, h.AbandonarIntento(ctx, id), publicado.ErrRechazado, "ya está abandonado")

	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.True(t, intento.Abandonado)
	require.True(t, intento.SinDesenlace(), "abandonado no es un estado")

	otro, _, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	require.ErrorIs(t, h.AbandonarIntento(ctx, otro), publicado.ErrRechazado, "con desenlace no se abandona")
	require.ErrorIs(t, h.AbandonarIntento(ctx, "no-existe"), publicado.ErrNoExiste)
}

func TestAbandonarIntento_LaMaquinaDelIntentoNoSigueEscribiendo(t *testing.T) {
	almacen := infraestructura.NuevoAlmacenEnMemoria()
	enLaMaquina, desdeElPortal := historialSobre(almacen), historialSobre(almacen)
	ctx := context.Background()

	id, err := enLaMaquina.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)
	hacerPasos(t, enLaMaquina, ctx, id, "supply")

	require.NoError(t, desdeElPortal.AbandonarIntento(ctx, id))

	require.ErrorIs(t, enLaMaquina.RegistrarComienzo(ctx, id, "deploy", nada), publicado.ErrRechazado)
	_, _, err = enLaMaquina.CerrarIntento(ctx, id, publicado.Exitoso, "")
	require.ErrorIs(t, err, publicado.ErrRechazado)
}

func TestCerrarIntento_SoloLlegaADespliegueExitosoConCommitsYTodosLosPasos(t *testing.T) {
	casos := map[string]struct {
		preparar func(*publicado.Apertura)
		estado   publicado.Estado
	}{
		"con una copia de trabajo":  {func(a *publicado.Apertura) { a.ConCommits = false }, publicado.Exitoso},
		"sin pedir todos los pasos": {func(a *publicado.Apertura) { a.HastaPaso = "supply" }, publicado.Exitoso},
		"fallido":                   {func(*publicado.Apertura) {}, publicado.Fallido},
		"cancelado":                 {func(*publicado.Apertura) {}, publicado.Cancelado},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			h, ctx := nuevoHistorial()
			escucha := escuchar(t, h)
			a := apertura("staging")
			c.preparar(&a)

			id, _, hay := intentar(t, h, ctx, a, c.estado, "")

			require.False(t, hay)
			require.Empty(t, escucha.eventos)
			despliegues, err := h.DesplieguesDeUnAmbiente(ctx, "staging")
			require.NoError(t, err)
			require.Empty(t, despliegues)
			intento, err := h.Intento(ctx, id)
			require.NoError(t, err)
			require.Equal(t, c.estado, intento.Estado)
		})
	}
}

func TestCerrarIntento_ElDestinoDeUnRollbackEsElPadre(t *testing.T) {
	h, ctx := nuevoHistorial()
	_, d1, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	_, d2, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	require.Equal(t, d1.Id, d2.Padre)

	_, d3, hay := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, d1.Id)
	require.True(t, hay)
	require.Equal(t, d1.Id, d3.Padre, "dos despliegues comparten padre")

	_, deProd, _ := intentar(t, h, ctx, apertura("prod"), publicado.Exitoso, "")
	id, err := h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)
	hacerPasos(t, h, ctx, id, "supply", "deploy")
	_, _, err = h.CerrarIntento(ctx, id, publicado.Exitoso, deProd.Id)
	require.ErrorIs(t, err, publicado.ErrRechazado, "el destino es de otro ambiente")
	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	require.True(t, intento.SinDesenlace(), "un cierre rechazado no se escribe")
}

func TestCerrarIntento_RepetirElMismoCierreNoEscribeOtro(t *testing.T) {
	h, ctx := nuevoHistorial()
	escucha := escuchar(t, h)
	id, d, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")

	otra, hay, err := h.CerrarIntento(ctx, id, publicado.Exitoso, "")
	require.NoError(t, err)
	require.True(t, hay)
	require.Equal(t, d, otra)
	require.Len(t, escucha.eventos, 1)

	_, _, err = h.CerrarIntento(ctx, id, publicado.Fallido, "")
	require.ErrorIs(t, err, publicado.ErrRechazado, "como mucho un cierre")
	despliegues, err := h.DesplieguesDeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	require.Len(t, despliegues, 1)
}

func TestRegistrarNoReejecucion_LaEvidenciaApuntaAUnFinalExitoso(t *testing.T) {
	h, ctx := nuevoHistorial()
	enStaging, _, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	id, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)

	require.ErrorIs(t,
		h.RegistrarNoReejecucion(ctx, id, "supply", publicado.Evidencia{Intento: enStaging, Paso: "notify"}, nada),
		publicado.ErrRechazado)
	require.ErrorIs(t,
		h.RegistrarNoReejecucion(ctx, id, "supply", publicado.Evidencia{Intento: "no-existe", Paso: "supply"}, nada),
		publicado.ErrRechazado)

	razon := publicado.Contenido{Contexto: "ejecucion", Datos: []byte(`{"razon":"nada cambió"}`)}
	require.NoError(t,
		h.RegistrarNoReejecucion(ctx, id, "supply", publicado.Evidencia{Intento: enStaging, Paso: "supply"}, razon))
	hacerPasos(t, h, ctx, id, "deploy")
	_, hay, err := h.CerrarIntento(ctx, id, publicado.Exitoso, "")
	require.NoError(t, err)
	require.True(t, hay, "un paso que no se re-ejecutó cuenta como hecho")
}

func TestUltimaVezDeUnPaso_EsElUltimoRegistroEnSuAmbito(t *testing.T) {
	h, ctx := nuevoHistorial()
	enStaging, _, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	enProd, err := h.AbrirIntento(ctx, apertura("prod"))
	require.NoError(t, err)
	hacerPasos(t, h, ctx, enProd, "supply")
	require.NoError(t, h.RegistrarComienzo(ctx, enProd, "deploy", nada))

	r, ok, err := h.UltimaVezDeUnPaso(ctx, "deploy", publicado.Ambito{Ambiente: "staging"})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, enStaging, r.Intento)
	require.Equal(t, publicado.Final, r.Tipo)
	require.True(t, r.Exitoso)

	r, ok, err = h.UltimaVezDeUnPaso(ctx, "deploy", publicado.Ambito{Ambiente: "prod"})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, publicado.Comienzo, r.Tipo, "un comienzo sin final obliga a re-ejecutar")

	r, ok, err = h.UltimaVezDeUnPaso(ctx, "supply", publicado.Ambito{Compartido: true})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, enProd, r.Intento, "el compartido no depende del ambiente")

	_, ok, err = h.UltimaVezDeUnPaso(ctx, "deploy", publicado.Ambito{Ambiente: "dev"})
	require.NoError(t, err)
	require.False(t, ok)
}

func TestConsultas_LoQuePideDiagnostico(t *testing.T) {
	h, ctx := nuevoHistorial()
	i1, d1, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")
	otroCodigo := apertura("staging")
	otroCodigo.HashDelCodigo = "h2"
	i2, _, _ := intentar(t, h, ctx, otroCodigo, publicado.Fallido, "")
	i3, _, _ := intentar(t, h, ctx, otroCodigo, publicado.Fallido, "")

	cantidad, err := h.CantidadDeIntentos(ctx, d1.Id, i3)
	require.NoError(t, err)
	require.Equal(t, 2, cantidad)
	cantidad, err = h.CantidadDeIntentos(ctx, d1.Id, i2)
	require.NoError(t, err)
	require.Equal(t, 1, cantidad)
	_, err = h.CantidadDeIntentos(ctx, d1.Id, i1)
	require.ErrorIs(t, err, publicado.ErrNoExiste, "el intento del propio despliegue no es posterior")

	d, ok, err := h.UltimoDespliegueConHashDelCodigo(ctx, "staging", "h1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, d1, d)
	_, ok, err = h.UltimoDespliegueConHashDelCodigo(ctx, "staging", "h2")
	require.NoError(t, err)
	require.False(t, ok)

	despliegue, intento, err := h.DespliegueYSuIntento(ctx, d1.Id)
	require.NoError(t, err)
	require.Equal(t, d1, despliegue)
	require.Equal(t, i1, intento.Id)

	intentos, err := h.IntentosDeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	var ids []string
	for _, i := range intentos {
		ids = append(ids, i.Id)
	}
	require.Equal(t, []string{i1, i2, i3}, ids)
}

func TestLanzamientoYReserva(t *testing.T) {
	h, ctx := nuevoHistorial()
	_, d, _ := intentar(t, h, ctx, apertura("staging"), publicado.Exitoso, "")

	_, err := h.RegistrarLanzamiento(ctx, "prod", d.Id, nada)
	require.ErrorIs(t, err, publicado.ErrRechazado, "el despliegue es de otro ambiente")

	l, err := h.RegistrarLanzamiento(ctx, "staging", d.Id,
		publicado.Contenido{Contexto: "lanzamiento", Datos: []byte(`{"version":1}`)})
	require.NoError(t, err)
	ultimo, ok, err := h.UltimoLanzamiento(ctx, "staging")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, l, ultimo)

	require.NoError(t, h.RegistrarReserva(ctx, "prod", true))
	require.NoError(t, h.RegistrarReserva(ctx, "prod", false))
	reserva, ok, err := h.UltimaReserva(ctx, "prod")
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, reserva.Reservado)
	_, ok, err = h.UltimaReserva(ctx, "staging")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestNadaDeLoPublicadoContieneUnValor(t *testing.T) {
	const secreto = "s3cr3t-de-produccion"
	h, ctx := nuevoHistorial()
	id, err := h.AbrirIntento(ctx, apertura("staging"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id, "supply", nada))
	require.NoError(t, h.RegistrarVariable(ctx, id, "supply", "DB_PASSWORD",
		publicado.Contenido{Contexto: "resolucion", Datos: []byte(`{"hash":"abc","origen":"declarada"}`)}))
	require.NoError(t, h.GuardarValor(ctx, id, "supply", "DB_PASSWORD", secreto))
	require.NoError(t, h.RegistrarFinal(ctx, id, "supply", true, nada))
	hacerPasos(t, h, ctx, id, "deploy")
	d, _, err := h.CerrarIntento(ctx, id, publicado.Exitoso, "")
	require.NoError(t, err)

	intento, err := h.Intento(ctx, id)
	require.NoError(t, err)
	intentos, err := h.IntentosDeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	variables, err := h.VariablesDeUnPaso(ctx, id, "supply")
	require.NoError(t, err)
	ultimaVez, _, err := h.UltimaVezDeUnPaso(ctx, "supply", publicado.Ambito{Compartido: true})
	require.NoError(t, err)
	despliegue, suIntento, err := h.DespliegueYSuIntento(ctx, d.Id)
	require.NoError(t, err)

	for _, publicado := range []any{intento, intentos, variables, ultimaVez, despliegue, suIntento} {
		require.NotContains(t, fmt.Sprintf("%+v", publicado), secreto)
	}
	require.Len(t, variables, 1)
	require.Equal(t, "DB_PASSWORD", variables[0].Nombre)

	valores, err := h.ValoresDeUnPaso(ctx, id, "supply")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"DB_PASSWORD": secreto}, valores, "solo vuelve por la relación reservada")

	t.Run("ningún tipo publicado tiene dónde poner un valor", func(t *testing.T) {
		tipos := []any{
			publicado.Intento{}, publicado.Apertura{}, publicado.RegistroDePaso{}, publicado.Variable{},
			publicado.Despliegue{}, publicado.Lanzamiento{}, publicado.Reserva{}, publicado.DespliegueRegistrado{},
			publicado.Contenido{}, publicado.Evidencia{}, publicado.Ambito{}, publicado.PasoDeclarado{},
		}
		for _, tipo := range tipos {
			rt := reflect.TypeOf(tipo)
			for k := 0; k < rt.NumField(); k++ {
				require.NotContains(t, strings.ToLower(rt.Field(k).Name), "valor", "%s.%s", rt.Name(), rt.Field(k).Name)
			}
		}
	})
}
