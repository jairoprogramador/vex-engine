package dominio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidadorVersion(t *testing.T) {
	t.Run("falla si no existe config.yaml", func(t *testing.T) {
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Configuracion: Declarado[ConfiguracionDeclarada]{Existe: false},
			},
		}
		v := &ValidadorVersion{}
		err := v.Validar(c)
		require.Error(t, err)
		require.Len(t, c.fallos, 1)
		require.Contains(t, c.fallos[0].Detalle, "no está")
	})

	t.Run("falla si version es nil", func(t *testing.T) {
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Configuracion: Declarado[ConfiguracionDeclarada]{
					Existe: true,
					Datos:  ConfiguracionDeclarada{},
				},
			},
		}
		v := &ValidadorVersion{}
		err := v.Validar(c)
		require.Error(t, err)
		require.Len(t, c.fallos, 1)
	})

	t.Run("falla si version es distinta de la esperada", func(t *testing.T) {
		version := "2"
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Configuracion: Declarado[ConfiguracionDeclarada]{
					Existe: true,
					Datos: ConfiguracionDeclarada{
						Version: &version,
					},
				},
			},
		}
		v := &ValidadorVersion{}
		err := v.Validar(c)
		require.Error(t, err)
		require.Len(t, c.fallos, 1)
		require.Contains(t, c.fallos[0].Detalle, "no se lee")
	})

	t.Run("pasa si version es correcta", func(t *testing.T) {
		version := VersionDelFormato
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Configuracion: Declarado[ConfiguracionDeclarada]{
					Existe: true,
					Datos: ConfiguracionDeclarada{
						Version: &version,
					},
				},
			},
		}
		v := &ValidadorVersion{}
		err := v.Validar(c)
		require.NoError(t, err)
		require.Len(t, c.fallos, 0)
	})
}

func TestValidadorArchivosIlegibles(t *testing.T) {
	t.Run("reporta archivos ilegibles", func(t *testing.T) {
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Ilegibles: []FicheroIlegibleDeclarado{
					{Fichero: "config.yaml", Motivo: "no se puede leer"},
				},
			},
		}
		v := &ValidadorArchivosIlegibles{}
		err := v.Validar(c)
		require.Error(t, err)
		require.Len(t, c.fallos, 1)
		require.Contains(t, c.fallos[0].Detalle, "no se puede leer")
	})

	t.Run("reporta archivos desconocidos", func(t *testing.T) {
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{
				Desconocidos: []string{"archivo_extraño.yaml"},
			},
		}
		v := &ValidadorArchivosIlegibles{}
		err := v.Validar(c)
		require.Error(t, err)
		require.Len(t, c.fallos, 1)
		require.Contains(t, c.fallos[0].Detalle, "no es parte del formato")
	})

	t.Run("pasa si no hay archivos ilegibles ni desconocidos", func(t *testing.T) {
		c := &comprobacion{
			pipelineDeclarado: PipelineDeclarado{},
		}
		v := &ValidadorArchivosIlegibles{}
		err := v.Validar(c)
		require.NoError(t, err)
	})
}

func TestValidadorSalidas(t *testing.T) {
	t.Run("indexa variables de salida correctamente", func(t *testing.T) {
		c := &comprobacion{
			pasosComprobados: []PasoComprobado{
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
			},
		}
		v := &ValidadorSalidas{}
		err := v.Validar(c)
		require.NoError(t, err)
		require.Len(t, c.variablesDeComandos, 1)
		require.NotNil(t, c.variablesDeComandos["VAR1"])
	})

	t.Run("detecta duplicados en el mismo comando", func(t *testing.T) {
		c := &comprobacion{
			pasosComprobados: []PasoComprobado{
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
			},
		}
		v := &ValidadorSalidas{}
		err := v.Validar(c)
		require.NoError(t, err)
	})
}

func TestVariableDePipelineEnComprobacionFichero(t *testing.T) {
	t.Run("almacena fichero de origen", func(t *testing.T) {
		v := variableDePipelineEnComprobacion{
			VariableDePipelineComprobada: VariableDePipelineComprobada{
				Nombre: "VAR1",
			},
			fichero: "variables/shared/test.yaml",
		}
		require.Equal(t, "VAR1", v.Nombre)
		require.Equal(t, "variables/shared/test.yaml", v.fichero)
	})
}

func TestPosicion(t *testing.T) {
	t.Run("antesDe compara posiciones correctamente", func(t *testing.T) {
		p1 := posicion{paso: 1, comando: 1}
		p2 := posicion{paso: 2, comando: 1}
		p3 := posicion{paso: 1, comando: 2}

		require.True(t, p1.antesDe(p2))
		require.True(t, p1.antesDe(p3))
		require.False(t, p2.antesDe(p1))
		require.False(t, p3.antesDe(p1))
	})
}

func TestVariableDeComandoEnComprobacion(t *testing.T) {
	t.Run("laVe retorna true si es compartida", func(t *testing.T) {
		v := variableDeComandoEnComprobacion{
			nombre:     "VAR1",
			compartida: true,
		}
		require.True(t, v.laVe(Compartido))
		require.True(t, v.laVe(Ambito("ambiente1")))
	})

	t.Run("laVe retorna false si no es compartida y el ámbito es compartido", func(t *testing.T) {
		v := variableDeComandoEnComprobacion{
			nombre:     "VAR1",
			compartida: false,
		}
		require.False(t, v.laVe(Compartido))
		require.True(t, v.laVe(Ambito("ambiente1")))
	})
}
