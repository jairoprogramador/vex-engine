package borde_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	lanzamientopublicado "github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
	simulacionpublicado "github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// contextosFalsos son los dobles de los cinco contextos de entrada. Cada uno solo anota qué operación le
// llegó y con qué: el borde no decide nada, así que eso es todo lo que hay que mirar.
type contextosFalsos struct {
	llamadas []string
	recibido map[string]any
	// ultimoIntento es el que dice el historial que se abrió último; vacío si no hay ninguno.
	ultimoIntento string
}

func (c *contextosFalsos) anotar(operacion string, datos any) {
	if c.recibido == nil {
		c.recibido = map[string]any{}
	}
	c.llamadas = append(c.llamadas, operacion)
	c.recibido[operacion] = datos
}

func (c *contextosFalsos) Intentar(
	_ context.Context, p ejecucionpublicado.PeticionDeIntento, _ ejecucionpublicado.Entorno,
) (ejecucionpublicado.Resultado, error) {
	c.anotar("intentar", p)
	return ejecucionpublicado.Resultado{}, nil
}

func (c *contextosFalsos) HacerRollback(
	_ context.Context, p ejecucionpublicado.PeticionDeRollback, _ ejecucionpublicado.Entorno,
) (ejecucionpublicado.Resultado, error) {
	c.anotar("rollback", p)
	return ejecucionpublicado.Resultado{}, nil
}

func (c *contextosFalsos) Simular(
	_ context.Context, p simulacionpublicado.PeticionDeSimulacion,
) (simulacionpublicado.Resultado, error) {
	c.anotar("simular", p)
	return simulacionpublicado.Resultado{}, nil
}

func (c *contextosFalsos) Lanzar(
	_ context.Context, ambiente, despliegue, nombre string,
) (lanzamientopublicado.Lanzamiento, error) {
	c.anotar("lanzar", [3]string{ambiente, despliegue, nombre})
	return lanzamientopublicado.Lanzamiento{}, nil
}

func (c *contextosFalsos) Reservar(_ context.Context, ambiente string) error {
	c.anotar("reservar", ambiente)
	return nil
}

func (c *contextosFalsos) Liberar(_ context.Context, ambiente string) error {
	c.anotar("liberar", ambiente)
	return nil
}

func (c *contextosFalsos) PreguntarLaCausa(
	_ context.Context, p diagnosticopublicado.PeticionDeDiagnostico,
) (diagnosticopublicado.Respuesta, error) {
	c.anotar("causa", p)
	return diagnosticopublicado.Respuesta{}, nil
}

func (c *contextosFalsos) AbandonarIntento(_ context.Context, intento string) error {
	c.anotar("abandonar", intento)
	return nil
}

func (c *contextosFalsos) Intento(_ context.Context, id string) (historialpublicado.Intento, error) {
	c.anotar("intento", id)
	return historialpublicado.Intento{Id: id, Apertura: historialpublicado.Apertura{Ambiente: "prod"}}, nil
}

func (c *contextosFalsos) IntentosDeUnAmbiente(_ context.Context, ambiente string) ([]historialpublicado.Intento, error) {
	c.anotar("intentos", ambiente)
	return nil, nil
}

func (c *contextosFalsos) DesplieguesDeUnAmbiente(
	_ context.Context, ambiente string,
) ([]historialpublicado.Despliegue, error) {
	c.anotar("despliegues", ambiente)
	return nil, nil
}

// consultaDeLogs es lo que le llega al historial al consultar las salidas de un intento.
type consultaDeLogs struct {
	Intento string
	Filtro  historialpublicado.FiltroDeSalidas
}

func (c *contextosFalsos) SalidasDeUnIntento(
	_ context.Context, intento string, filtro historialpublicado.FiltroDeSalidas,
) ([]historialpublicado.Salida, error) {
	c.anotar("logs", consultaDeLogs{Intento: intento, Filtro: filtro})
	return nil, nil
}

func (c *contextosFalsos) UltimoIntento(context.Context) (historialpublicado.Intento, bool, error) {
	if c.ultimoIntento == "" {
		return historialpublicado.Intento{}, false, nil
	}
	return historialpublicado.Intento{Id: c.ultimoIntento, Apertura: historialpublicado.Apertura{Ambiente: "prod"}}, true, nil
}

func servicioConContextosFalsos() (*borde.Servicio, *contextosFalsos) {
	falsos := &contextosFalsos{}
	return borde.NuevoServicio(borde.Dependencias{
		Ejecucion: falsos, Simulacion: falsos, Lanzamiento: falsos, Diagnostico: falsos, Historial: falsos,
	}), falsos
}

// operacion es una operación del lenguaje publicado invocada con una versión dada.
type operacion struct {
	nombre   string
	llamado  string // la operación con la que el doble anota la llegada
	esperado any    // lo que debe llegar al contexto
	invocar  func(s *borde.Servicio, version string) error
}

func todasLasOperaciones() []operacion {
	ctx := context.Background()
	return []operacion{
		{"intentar", "intentar", ejecucionpublicado.PeticionDeIntento{Version: "1", Ambiente: "prod"},
			func(s *borde.Servicio, v string) error {
				_, err := s.Intentar(ctx, ejecucionpublicado.PeticionDeIntento{Version: v, Ambiente: "prod"}, nil)
				return err
			}},
		{"hacer rollback", "rollback", ejecucionpublicado.PeticionDeRollback{Version: "1", Despliegue: "dep-1"},
			func(s *borde.Servicio, v string) error {
				_, err := s.HacerRollback(ctx, ejecucionpublicado.PeticionDeRollback{Version: v, Despliegue: "dep-1"}, nil)
				return err
			}},
		{"simular", "simular", simulacionpublicado.PeticionDeSimulacion{Version: "1", Fuente: "p", Commit: "c"},
			func(s *borde.Servicio, v string) error {
				_, err := s.Simular(ctx, simulacionpublicado.PeticionDeSimulacion{Version: v, Fuente: "p", Commit: "c"})
				return err
			}},
		{"lanzar", "lanzar", [3]string{"prod", "dep-1", "estreno"},
			func(s *borde.Servicio, v string) error {
				_, err := s.Lanzar(ctx, borde.PeticionDeLanzamiento{
					Version: v, Ambiente: "prod", Despliegue: "dep-1", Nombre: "estreno",
				})
				return err
			}},
		{"reservar", "reservar", "prod",
			func(s *borde.Servicio, v string) error {
				return s.Reservar(ctx, borde.PeticionDeReserva{Version: v, Ambiente: "prod"})
			}},
		{"liberar", "liberar", "prod",
			func(s *borde.Servicio, v string) error {
				return s.Liberar(ctx, borde.PeticionDeLiberacion{Version: v, Ambiente: "prod"})
			}},
		{"preguntar la causa", "causa", diagnosticopublicado.PeticionDeDiagnostico{Version: "1", Intento: "int-1"},
			func(s *borde.Servicio, v string) error {
				_, err := s.PreguntarLaCausa(ctx, diagnosticopublicado.PeticionDeDiagnostico{Version: v, Intento: "int-1"})
				return err
			}},
		{"abandonar un intento", "abandonar", "int-1",
			func(s *borde.Servicio, v string) error {
				return s.AbandonarIntento(ctx, borde.PeticionDeAbandono{Version: v, Intento: "int-1"})
			}},
		{"consultar un intento", "intento", "int-1",
			func(s *borde.Servicio, v string) error {
				_, err := s.Intento(ctx, borde.PeticionDeConsultaDeIntento{Version: v, Intento: "int-1"})
				return err
			}},
		{"consultar los intentos de un ambiente", "intentos", "prod",
			func(s *borde.Servicio, v string) error {
				_, err := s.IntentosDeUnAmbiente(ctx, borde.PeticionDeIntentosDeUnAmbiente{Version: v, Ambiente: "prod"})
				return err
			}},
		{"consultar los logs de un intento", "logs", consultaDeLogs{Intento: "int-1", Filtro: historialpublicado.SoloLasFallidas},
			func(s *borde.Servicio, v string) error {
				_, err := s.Logs(ctx, borde.PeticionDeLogs{Version: v, Intento: "int-1", Resultado: "fallido"})
				return err
			}},
		{"consultar los despliegues de un ambiente", "despliegues", "prod",
			func(s *borde.Servicio, v string) error {
				_, err := s.DesplieguesDeUnAmbiente(ctx, borde.PeticionDeDesplieguesDeUnAmbiente{Version: v, Ambiente: "prod"})
				return err
			}},
	}
}

func TestServicio_CadaOperacionRechazaUnaVersionNoSoportadaSinLlamarAlContexto(t *testing.T) {
	for _, op := range todasLasOperaciones() {
		for _, version := range []string{"99", ""} {
			t.Run(op.nombre+" con versión "+version, func(t *testing.T) {
				servicio, falsos := servicioConContextosFalsos()

				err := op.invocar(servicio, version)

				require.ErrorIs(t, err, borde.ErrVersionNoSoportada)
				require.Empty(t, falsos.llamadas)
			})
		}
	}
}

// consultasPrevias son las consultas al historial que una operación hace antes de la suya: los logs piden el
// intento para decir de qué ambiente son.
var consultasPrevias = map[string][]string{"logs": {"intento"}}

func TestServicio_CadaOperacionLlegaASuContextoDeEntradaConSusDatos(t *testing.T) {
	for _, op := range todasLasOperaciones() {
		t.Run(op.nombre, func(t *testing.T) {
			servicio, falsos := servicioConContextosFalsos()

			err := op.invocar(servicio, "1")

			require.NoError(t, err)
			require.Equal(t, append(consultasPrevias[op.llamado], op.llamado), falsos.llamadas)
			require.Equal(t, op.esperado, falsos.recibido[op.llamado])
		})
	}
}

// Logs sin intento consulta el último que se abrió, y con él consulta el que se pide.
func TestServicio_LogsSinIntentoConsultaElUltimoQueSeAbrio(t *testing.T) {
	servicio, falsos := servicioConContextosFalsos()
	falsos.ultimoIntento = "int-9"

	respuesta, err := servicio.Logs(context.Background(), borde.PeticionDeLogs{Version: "1"})

	require.NoError(t, err)
	require.Equal(t, "int-9", respuesta.Intento, "la respuesta dice cuál intento es")
	require.Equal(t, "prod", respuesta.Ambiente, "y de qué ambiente")
	require.Equal(t, consultaDeLogs{Intento: "int-9", Filtro: historialpublicado.TodasLasSalidas}, falsos.recibido["logs"])
}

func TestServicio_LogsSinNingunIntentoDiceQueNoExiste(t *testing.T) {
	servicio, _ := servicioConContextosFalsos()

	_, err := servicio.Logs(context.Background(), borde.PeticionDeLogs{Version: "1"})

	require.ErrorIs(t, err, historialpublicado.ErrNoExiste)
}

func TestServicio_LogsTraduceElResultadoAlFiltro(t *testing.T) {
	filtros := map[string]historialpublicado.FiltroDeSalidas{
		"":        historialpublicado.TodasLasSalidas,
		"exitoso": historialpublicado.SoloLasExitosas,
		"fallido": historialpublicado.SoloLasFallidas,
	}
	for resultado, filtro := range filtros {
		t.Run("resultado "+resultado, func(t *testing.T) {
			servicio, falsos := servicioConContextosFalsos()

			_, err := servicio.Logs(context.Background(), borde.PeticionDeLogs{Version: "1", Intento: "int-1", Resultado: resultado})

			require.NoError(t, err)
			require.Equal(t, consultaDeLogs{Intento: "int-1", Filtro: filtro}, falsos.recibido["logs"])
		})
	}
}

func TestServicio_LogsRechazaUnResultadoDesconocidoSinConsultar(t *testing.T) {
	servicio, falsos := servicioConContextosFalsos()

	_, err := servicio.Logs(context.Background(), borde.PeticionDeLogs{Version: "1", Intento: "int-1", Resultado: "roto"})

	require.ErrorIs(t, err, borde.ErrPeticionInvalida)
	require.Empty(t, falsos.llamadas)
}

// Consultar el historial no devuelve ningún valor (DEC-04.7): de un intento que produjo una variable, lo que
// se puede consultar por el borde no contiene lo que la variable valía.
func TestE2E_ConsultarElHistorialNoDevuelveNingunValor(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	resultado, err := s.borde.Intentar(ctx, s.peticion(t), nil)
	require.NoError(t, err)
	require.NotEmpty(t, resultado.Despliegue)

	// Los valores son los que produjo cada paso del ejemplo que declara salidas, no uno escrito a mano.
	const largoMinimo = 6 // más corto, encontrarlo en una consulta serializada podría ser casualidad
	var valoresProducidos []string
	for _, paso := range pasosDelEjemplo(t) {
		if len(paso.productos()) == 0 {
			continue
		}
		guardados, err := s.historial.ValoresDeUnPaso(ctx, resultado.Intento, paso.Nombre)
		require.NoError(t, err)
		for _, nombre := range paso.productos() {
			require.Contains(t, guardados, nombre, "la prueba solo vale si el valor sí existe, por la relación reservada")
			if len(guardados[nombre]) >= largoMinimo {
				valoresProducidos = append(valoresProducidos, guardados[nombre])
			}
		}
	}
	require.NotEmpty(t, valoresProducidos, "el ejemplo necesita producir algún valor de %d o más caracteres", largoMinimo)

	intento, err := s.borde.Intento(ctx, borde.PeticionDeConsultaDeIntento{Version: "1", Intento: resultado.Intento})
	require.NoError(t, err)
	intentos, err := s.borde.IntentosDeUnAmbiente(ctx, borde.PeticionDeIntentosDeUnAmbiente{Version: "1", Ambiente: "prod"})
	require.NoError(t, err)
	despliegues, err := s.borde.DesplieguesDeUnAmbiente(ctx, borde.PeticionDeDesplieguesDeUnAmbiente{Version: "1", Ambiente: "prod"})
	require.NoError(t, err)
	require.NotEmpty(t, intentos)
	require.NotEmpty(t, despliegues)

	for nombre, consulta := range map[string]any{"intento": intento, "intentos": intentos, "despliegues": despliegues} {
		serializada, err := json.Marshal(consulta)
		require.NoError(t, err)
		for _, valor := range valoresProducidos {
			require.NotContains(t, string(serializada), valor, nombre)
		}
	}
}

func TestE2E_AbandonarUnIntentoPorElBordeLiberaElAmbiente(t *testing.T) {
	s := montarSistema(t)
	ctx := context.Background()
	id, err := s.historial.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: "prod", Solicitante: "otro", Pasos: []historialpublicado.PasoDeclarado{{Nombre: primerPaso(t)}},
		HastaPaso: primerPaso(t), HashDelCodigo: "h", ConCommits: true,
	})
	require.NoError(t, err)

	require.NoError(t, s.borde.AbandonarIntento(ctx, borde.PeticionDeAbandono{Version: "1", Intento: id}))

	resultado, err := s.borde.Intentar(ctx, s.peticion(t), nil)
	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
}
