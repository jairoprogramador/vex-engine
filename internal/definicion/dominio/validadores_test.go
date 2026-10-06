package dominio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Helpers: construyen los valores mínimos que cada prueba necesita, para que las tablas de casos no repitan
// boilerplate.

func comprobacionConConfig(existe bool, datos ConfiguracionDeclarada) *comprobacion {
	return &comprobacion{
		pipelineDeclarado: PipelineDeclarado{
			Configuracion: Declarado[ConfiguracionDeclarada]{Existe: existe, Datos: datos},
		},
	}
}

func TestContextoDePasoFalla(t *testing.T) {
	comp := &comprobacion{}
	ctx := contextoDePaso{comprobacion: comp, paso: &PasoComprobado{Nombre: "registro"}, fichero: "steps/01-registro/commands.yaml"}

	ctx.falla(Formato, "%s no dice cmd", "el comando 1")

	require.Equal(t, []Fallo{{
		Invariante: Formato, Fichero: "steps/01-registro/commands.yaml", Paso: "registro", Detalle: "el comando 1 no dice cmd",
	}}, comp.fallos)
}

func TestValidadorVersion(t *testing.T) {
	t.Run("falla si no existe config.yaml", func(t *testing.T) {
		comp := comprobacionConConfig(false, ConfiguracionDeclarada{})
		validador := &ValidadorVersion{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
		require.Contains(t, comp.fallos[0].Detalle, "no está")
	})

	t.Run("falla si version es nil", func(t *testing.T) {
		comp := comprobacionConConfig(true, ConfiguracionDeclarada{})
		validador := &ValidadorVersion{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
	})

	t.Run("falla si version es distinta de la esperada", func(t *testing.T) {
		version := "2"
		comp := comprobacionConConfig(true, ConfiguracionDeclarada{Version: &version})
		validador := &ValidadorVersion{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
		require.Contains(t, comp.fallos[0].Detalle, "no se lee")
	})

	t.Run("pasa si version es correcta", func(t *testing.T) {
		version := VersionDelFormato
		comp := comprobacionConConfig(true, ConfiguracionDeclarada{Version: &version})
		validador := &ValidadorVersion{}
		err := validador.Validar(comp)
		require.NoError(t, err)
		require.Len(t, comp.fallos, 0)
	})
}

func TestValidadorArchivosIlegibles(t *testing.T) {
	t.Run("reporta archivos ilegibles", func(t *testing.T) {
		comp := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Ilegibles: []FicheroIlegibleDeclarado{
					{Fichero: "config.yaml", Motivo: "no se puede leer"},
				},
			},
		}
		validador := &ValidadorArchivosIlegibles{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
		require.Contains(t, comp.fallos[0].Detalle, "no se puede leer")
	})

	t.Run("reporta archivos desconocidos", func(t *testing.T) {
		comp := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Desconocidos: []string{"archivo_extraño.yaml"},
			},
		}
		validador := &ValidadorArchivosIlegibles{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
		require.Contains(t, comp.fallos[0].Detalle, "no es parte del formato")
	})

	t.Run("pasa si no hay archivos ilegibles ni desconocidos", func(t *testing.T) {
		comp := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{},
		}
		validador := &ValidadorArchivosIlegibles{}
		err := validador.Validar(comp)
		require.NoError(t, err)
		require.Len(t, comp.fallos, 0)
	})
}

func TestValidadorSalidas(t *testing.T) {
	t.Run("indexa variables de salida correctamente", func(t *testing.T) {
		comp := &comprobacion{
			resultadoComprobacion: resultadoComprobacion{pasosComprobados: []PasoComprobado{
				{
					Nombre: "paso1",
					Orden:  1,
					Comandos: []ComandoComprobado{
						{
							Nombre: "cmd1",
							VariablesDeSalida: []VariableDeComandoComprobada{
								{Nombre: "VAR1"},
							},
						},
					},
				},
			}},
		}
		validador := &ValidadorVariablesDeSalida{}
		err := validador.Validar(comp)
		require.NoError(t, err)
		require.Len(t, comp.variablesDeComandos, 1)
		require.NotNil(t, comp.variablesDeComandos["VAR1"])
	})

	t.Run("no falla si el mismo comando declara la misma salida dos veces", func(t *testing.T) {
		comp := &comprobacion{
			resultadoComprobacion: resultadoComprobacion{pasosComprobados: []PasoComprobado{
				{
					Nombre: "paso1",
					Orden:  1,
					Comandos: []ComandoComprobado{
						{
							Nombre: "cmd1",
							VariablesDeSalida: []VariableDeComandoComprobada{
								{Nombre: "VAR1"},
								{Nombre: "VAR1"},
							},
						},
					},
				},
			}},
		}
		validador := &ValidadorVariablesDeSalida{}
		err := validador.Validar(comp)
		require.NoError(t, err)
	})

	t.Run("falla si dos comandos distintos producen la misma salida", func(t *testing.T) {
		comp := &comprobacion{
			resultadoComprobacion: resultadoComprobacion{pasosComprobados: []PasoComprobado{
				{
					Nombre: "paso1",
					Orden:  1,
					Comandos: []ComandoComprobado{
						{
							Nombre:            "cmd1",
							VariablesDeSalida: []VariableDeComandoComprobada{{Nombre: "VAR1"}},
						},
						{
							Nombre:            "cmd2",
							VariablesDeSalida: []VariableDeComandoComprobada{{Nombre: "VAR1"}},
						},
					},
				},
			}},
		}
		validador := &ValidadorVariablesDeSalida{}
		err := validador.Validar(comp)
		require.Error(t, err)
		require.Len(t, comp.fallos, 1)
		require.Contains(t, comp.fallos[0].Detalle, "ya produce el")
	})
}

func TestVariableDePipelineEnComprobacionFichero(t *testing.T) {
	t.Run("almacena fichero de origen", func(t *testing.T) {
		variable := variableDePipelineEnComprobacion{
			VariableDePipelineComprobada: VariableDePipelineComprobada{
				Nombre: "VAR1",
			},
			fichero: "variables/shared/test.yaml",
		}
		require.Equal(t, "VAR1", variable.Nombre)
		require.Equal(t, "variables/shared/test.yaml", variable.fichero)
	})
}

func TestPosicion(t *testing.T) {
	t.Run("antesDe compara posiciones correctamente", func(t *testing.T) {
		base := posicion{paso: 1, comando: 1}
		pasoPosterior := posicion{paso: 2, comando: 1}
		comandoPosterior := posicion{paso: 1, comando: 2}

		require.True(t, base.antesDe(pasoPosterior))
		require.True(t, base.antesDe(comandoPosterior))
		require.False(t, pasoPosterior.antesDe(base))
		require.False(t, comandoPosterior.antesDe(base))
	})

	t.Run("antesDe es falso para la misma posición", func(t *testing.T) {
		p := posicion{paso: 1, comando: 1}
		otra := posicion{paso: 1, comando: 1}

		require.False(t, p.antesDe(otra))
	})
}

func TestVariableDeComandoEnComprobacion(t *testing.T) {
	t.Run("laVe retorna true si es compartida", func(t *testing.T) {
		variable := variableDeSalidaEnComprobacion{
			nombre:     "VAR1",
			compartida: true,
		}
		require.True(t, variable.laVe(Compartido))
		require.True(t, variable.laVe(Ambito("ambiente1")))
	})

	t.Run("laVe retorna false si no es compartida y el ámbito es compartido", func(t *testing.T) {
		variable := variableDeSalidaEnComprobacion{
			nombre:     "VAR1",
			compartida: false,
		}
		require.False(t, variable.laVe(Compartido))
		require.True(t, variable.laVe(Ambito("ambiente1")))
	})
}
