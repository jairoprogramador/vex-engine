package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// `vexd record` — la superficie de consulta y mantenimiento del registro
// (spec 22).
//
// Vive en `vexd` y no en la CLI `vex` porque su función es VALIDAR LO QUE `vexd`
// ESCRIBE, mientras todavía hay pocos datos permanentes emitidos: los objetos y los
// eventos no se corrigen borrando. Un `vex log` para el usuario final es otro
// proyecto y llega después.
//
// Los subcomandos comparten la resolución del destino y del área de trabajo con
// `run`, y eso es el punto: una consulta que mirara otro sitio que el motor no sería
// una consulta, sería una segunda opinión.

// newRecordCommand define `vexd record` y sus subcomandos.
func newRecordCommand() *cobra.Command {
	args := cli.RecordArgs{}
	paths := enginePaths{}

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Inspecciona y mantiene el registro de despliegue que escribe vexd",
		Long: `Consulta y mantenimiento del registro (spec 22).

El destino se resuelve por la MISMA vía que en 'vexd run': --state-config <archivo>
o la env var ` + cli.StateConfigEnvVar + `.

Las dos raíces no son equivalentes y --from elige:

  --from work         el área de trabajo del motor. «¿qué ocurrió aquí?» (default)
  --from destination  el destino. «¿qué se publicó?»

El área de trabajo es un SUPERCONJUNTO: un intento interrumpido no llega a
empujarse nunca.

Exit codes:
  0    la consulta respondió; 'verify' no encontró corrupción
  1    'verify' encontró corrupción, o la consulta no pudo responder
  2    invocación inválida (destino ausente, identificador malformado)`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&args.StateConfigFile, "state-config", "",
		"ruta a la configuración de destino del estado; sin ella se usa la env var "+cli.StateConfigEnvVar)
	cmd.PersistentFlags().StringVar(&args.StagingDir, "staging-dir", "",
		"área de trabajo del motor; misma resolución que en 'run'")
	cmd.PersistentFlags().StringVar(&paths.vexHome, "vex-home", "",
		"raíz bajo la que vive .vex/; vacío usa el $HOME del proceso")
	cmd.PersistentFlags().StringVar(&args.From, "from", cli.FromWork,
		"raíz de lectura: work | destination")

	cmd.AddCommand(newRecordShowCommand(&args, &paths))
	cmd.AddCommand(newRecordLogCommand(&args, &paths))
	cmd.AddCommand(newRecordHistoryCommand(&args, &paths))
	cmd.AddCommand(newRecordVerifyCommand(&args, &paths))
	cmd.AddCommand(newRecordGCCommand(&args, &paths))
	cmd.AddCommand(newRecordRebuildCommand(&args, &paths))

	return cmd
}

func newRecordShowCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	return &cobra.Command{
		Use:   "show <deployment_id>",
		Short: "La intención congelada de un despliegue y lo que su intento fue",
		Long: `Imprime el objeto —con su content_id— y el pliegue de sus hechos.

Un intento INTERRUMPIDO no es un error: es la respuesta correcta, con el último
step alcanzado. Es la propiedad que justificó el event sourcing.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, positional []string) error {
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			return salir(cmd, recordCmd.Show(
				cmd.Context(), cmd.OutOrStdout(), *args, positional[0]))
		},
	}
}

func newRecordLogCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <deployment_id> [attempt]",
		Short: "Los hechos de un despliegue, en orden de seq",
		Long: `Imprime el sobre CRUDO de cada hecho, incluidos los de un tipo que este
binario no conoce: un motor más nuevo puede haber escrito uno, y un 'log' que sólo
enseñara lo que entiende esconderia justo la línea que hay que mirar.

Es también el único sitio donde un hueco del destino se puede explicar: 'sync_failed'
no cambia el resultado del intento, así que Fold lo ignora y no aparece en 'show'.`,
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, positional []string) error {
			if len(positional) == 2 {
				attempt, err := numeroDeIntento(positional[1])
				if err != nil {
					return salir(cmd, fmt.Errorf(
						"vexd record log: %q no es un número de intento: %w", positional[1], err))
				}
				args.Attempt = attempt
			}
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			return salir(cmd, recordCmd.Log(
				cmd.Context(), cmd.OutOrStdout(), *args, positional[0]))
		},
	}
	return cmd
}

func newRecordHistoryCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <ambiente>",
		Short: "Los intentos de un ambiente, marcando cuáles son destinos válidos",
		Long: `Lista los intentos del más reciente al más antiguo y MARCA los que pueden
nombrarse como destino: terminaron bien y todos sus steps cerraron correctos.

La marca es el motivo de que el listado exista. Sin ella, quien elige una ejecución
pasada para volver a ella escoge una que falla después.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, positional []string) error {
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			return salir(cmd, recordCmd.History(
				cmd.Context(), cmd.OutOrStdout(), *args, positional[0]))
		},
	}
	cmd.Flags().IntVar(&args.Limit, "limit", 0, "cuántos intentos como máximo (0 = todos)")
	return cmd
}

func newRecordVerifyCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Comprueba los invariantes del registro",
		Long: `Recomputa el content_id de cada objeto, exige seq sin huecos y que todo
*_started tenga su *_finished —o el intento marcado interrupted—.

Distingue DOS diagnósticos que no son el mismo hecho: «el content_id no recomputa»
es corrupción, y «no puedo recomputarlo con la regla que tengo» es un límite de este
binario. Sólo el primero cambia el exit code.

No recomputa las huellas del índice: es desechable y no tiene invariantes que
preservar.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			report, err := recordCmd.Verify(cmd.Context(), cmd.OutOrStdout(), *args)
			if err != nil {
				return salir(cmd, err)
			}
			if report.Corruptos > 0 {
				os.Exit(cli.ExitFailed)
			}
			return nil
		},
	}
}

func newRecordGCCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Informa de lo que se puede podar y, con --apply, lo poda",
		Long: `Las reglas de vida de las tiendas, hechas ejecutables:

  objects/ events/ lineage/ keys/ state/   NUNCA se borran
  cache/                                   sí, por edad o entero (--all)
  staging/                                 sí, tiras con ack completo

De state/ se LISTA el inventario y no se borra nada: una tienda cuya regla es «no se
borra nunca» no puede tener un comando que la borre. Qué está huérfano depende del
pipelinecode —renumerar un step, retirar 'rules'— y este comando no lo tiene.

Sin --apply no se toca nada.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			_, err = recordCmd.GC(cmd.Context(), cmd.OutOrStdout(), *args)
			return salir(cmd, err)
		},
	}
	cmd.Flags().BoolVar(&args.Cache, "cache", false, "selecciona el índice de contenido")
	cmd.Flags().BoolVar(&args.Staging, "staging", false,
		"selecciona el búfer del área de trabajo (tiras confirmadas y ack huérfanos)")
	cmd.Flags().BoolVar(&args.All, "all", false,
		"con --cache, poda el índice ENTERO en vez de por edad")
	cmd.Flags().DurationVar(&args.MaxAge, "max-age", cli.DefaultCacheMaxAge,
		"retención propia del gc sobre el índice; se mide por el mtime del archivo")
	cmd.Flags().BoolVar(&args.Apply, "apply", false, "borra de verdad; sin él sólo informa")
	return cmd
}

func newRecordRebuildCommand(args *cli.RecordArgs, paths *enginePaths) *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Reconstruye el índice de contenido desde state/",
		Long: `Una entrada es {step_fingerprint, state_key, record_id} y los tres salen del
almacén: el record_id nombra el archivo, la state_key es su ruta y la clave es el
step_fingerprint que el registro guarda dentro.

La url del proyecto no está en la ruta —el directorio la abrevia con un hash— y se
recupera de lineage/ y objects/, que la llevan dentro.

No decide nada: el índice no participa en ninguna decisión del motor.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			recordCmd, err := buildRecordCommand(*args, *paths)
			if err != nil {
				return salir(cmd, err)
			}
			_, err = recordCmd.Rebuild(cmd.Context(), cmd.OutOrStdout())
			return salir(cmd, err)
		},
	}
}

// salir traduce el error al exit code del proceso con la MISMA regla que `run`:
// una invocación que no se puede atender es un 2, y todo lo demás un 1.
//
// Se escribe a stderr y se sale en vez de devolver el error a Cobra porque la raíz
// sale con 2 para cualquier error, y aquí la diferencia entre «no supiste invocarme»
// y «no pude responder» es información.
func salir(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	fmt.Fprintln(cmd.ErrOrStderr(), err)
	os.Exit(cli.ExitCodeFor(err))
	return nil
}

// buildRecordCommand resuelve las rutas del entorno real y delega, igual que
// `buildRunCommand`: el binario resuelve el `$HOME` y el cableado no lo mira.
func buildRecordCommand(args cli.RecordArgs, paths enginePaths) (*cli.RecordCommand, error) {
	root := paths.vexHome
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home: %w", err)
		}
		root = home
	}
	return cli.BuildRecordCommand(cli.EngineConfig{RootVexPath: root}, args)
}

// numeroDeIntento lee el `attempt` posicional.
//
// El cero se rechaza AQUÍ, en el borde, y no se deja llegar a la proyección: un
// `attempt: 0` es «no consta», así que una consulta con él no tiene sujeto y tiene que
// fallar donde se escribió mal (spec 17 §9).
func numeroDeIntento(text string) (int, error) {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, err
	}
	if value < 1 {
		return 0, fmt.Errorf("el intento 0 no es un intento")
	}
	return value, nil
}
