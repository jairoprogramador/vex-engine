package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jairoprogramador/vex-engine/old-internal/interfaces/cli"
)

// newRunCommand define `vexd run`: ejecuta una pipeline a partir de un
// RequestInput JSON y termina con exit code 0 (succeeded), 1 (failed) o 2
// (input error). Es el modo one-shot que reemplaza al servicio HTTP.
func newRunCommand() *cobra.Command {
	args := cli.RunArgs{}
	paths := enginePaths{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Ejecuta una pipeline (one-shot) a partir de un RequestInput JSON",
		Long: `Lee un RequestInput JSON desde --input <archivo>, la env var indicada en
--input-env (default VEX_REQUEST_INPUT, acepta JSON crudo o base64+JSON), o stdin
(en ese orden de prioridad), ejecuta la pipeline y reporta logs/stages.

El destino del estado es OBLIGATORIO y no tiene default: se pasa con
--state-config <archivo> o con la env var VEX_STATE_CONFIG (YAML/JSON, crudo o
base64):

  type: local
  local:
    path: /mnt/vex-state

'type: http' está en el vocabulario y todavía no está implementado.

Exit codes:
  0    ejecución exitosa
  1    fallo de la pipeline
  2    input invalido (JSON malformado, schema_version no soportado, fuente
       vacía, destino del estado ausente o inalcanzable)
  130  ejecución cancelada (SIGINT/SIGTERM)`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runCmd, err := buildRunCommand(args, paths)
			if err != nil {
				// El cableado falla por dos razones distintas y sólo una es culpa
				// de quien invocó; el exit code las separa igual que las separa
				// para el RequestInput.
				fmt.Fprintln(cmd.ErrOrStderr(), err)
				os.Exit(cli.ExitCodeFor(err))
			}
			code := runWithSignalHandling(func(ctx context.Context) int {
				return runCmd.Execute(ctx, os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr(), args)
			})
			if code != cli.ExitSucceeded {
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&args.InputFile, "input", "", "ruta a un archivo con el RequestInput JSON")
	cmd.Flags().StringVar(&args.InputEnv, "input-env", "VEX_REQUEST_INPUT", "nombre de la env var con el RequestInput (raw JSON o base64)")
	cmd.Flags().StringVar(&args.LogEndpoint, "log-endpoint", "", "URL de la edge function log-ingest")
	cmd.Flags().StringVar(&args.StatusEndpoint, "status-endpoint", "", "URL de la edge function execution-status")
	cmd.Flags().StringVar(&args.LogToken, "log-token", "", "bearer token para los endpoints supabase")
	cmd.Flags().StringVar(&args.ExecutionID, "execution-id", "", "UUID asignado externamente para la ejecución (lo usa el reporter)")
	cmd.Flags().BoolVar(&args.Quiet, "quiet", false, "suprime stdout local (no afecta a los endpoints supabase)")

	// El destino del estado: obligatorio y sin default (spec 16 §5.1).
	cmd.Flags().StringVar(&args.StateConfigFile, "state-config", "",
		"ruta a la configuración de destino del estado (YAML/JSON: type + payload); sin ella se usa la env var "+cli.StateConfigEnvVar)

	// Y las dos rutas que quedan del enum `--mode`: dónde vive el área de trabajo
	// del motor y dónde está ya el proyecto. Las dos las dice el caller.
	cmd.Flags().StringVar(&args.StagingDir, "staging-dir", "",
		"área de trabajo del motor; vacío resuelve $XDG_STATE_HOME/vex/staging → $HOME/.local/state/vex/staging → TempDir")
	cmd.Flags().StringVar(&paths.vexHome, "vex-home", "",
		"raíz bajo la que vive .vex/ (clones y workdirs); vacío usa el $HOME del proceso")
	cmd.Flags().StringVar(&paths.projectPath, "project-path", "",
		"punto de montaje del proyecto ya presente en disco; vacío clona el proyecto desde su url")

	return cmd
}
