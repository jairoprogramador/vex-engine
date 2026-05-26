package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// newRunCommand define `vexd run`: ejecuta una pipeline a partir de un
// RequestInput JSON y termina con exit code 0 (succeeded), 1 (failed) o 2
// (input error). Es el modo one-shot que reemplaza al servicio HTTP.
func newRunCommand() *cobra.Command {
	args := cli.RunArgs{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Ejecuta una pipeline (one-shot) a partir de un RequestInput JSON",
		Long: `Lee un RequestInput JSON desde --input <archivo>, la env var indicada en
--input-env (default VEX_REQUEST_INPUT, acepta JSON crudo o base64+JSON), o stdin
(en ese orden de prioridad), ejecuta la pipeline y reporta logs/stages.

Exit codes:
  0  ejecución exitosa
  1  fallo de la pipeline
  2  input invalido (JSON malformado, schema_version no soportado, fuente vacía)`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runCmd, err := buildRunCommand(args)
			if err != nil {
				return err
			}
			code := runCmd.Execute(os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr(), args)
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
	cmd.Flags().StringVar(&args.StepCodeEndpoint, "step-code-endpoint", "", "URL del endpoint de estado de código de proyecto (modo remoto)")
	cmd.Flags().StringVar(&args.StepInstEndpoint, "step-inst-endpoint", "", "URL del endpoint de estado de instrucciones (modo remoto)")
	cmd.Flags().StringVar(&args.StepTimeEndpoint, "step-time-endpoint", "", "URL del endpoint de estado de tiempo (modo remoto)")
	cmd.Flags().StringVar(&args.StepVarsEndpoint, "step-vars-endpoint", "", "URL del endpoint de estado de variables (modo remoto)")
	cmd.Flags().StringVar(&args.StepDeleteEndpoint, "step-delete-endpoint", "", "URL del endpoint de borrado de estado de paso (modo remoto)")
	cmd.Flags().StringVar(&args.LogToken, "log-token", "", "bearer token para los endpoints supabase")
	cmd.Flags().StringVar(&args.ExecutionID, "execution-id", "", "UUID asignado externamente para la ejecución (lo usa el reporter)")
	cmd.Flags().BoolVar(&args.Quiet, "quiet", false, "suprime stdout local (no afecta a los endpoints supabase)")
	cmd.Flags().StringVar(&args.Mode, "mode", "remote",
		`modo de ejecución: "remote" usa recursos de vex; "local" usa recursos de la maquina local`)

	return cmd
}
