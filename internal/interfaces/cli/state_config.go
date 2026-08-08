package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jairoprogramador/vex-engine/internal/domain/syncconfig"
)

// StateConfigEnvVar es la env var que transporta la configuración de destino
// cuando no se pasa `--state-config`. Mismo mecanismo que `VEX_REQUEST_INPUT` y
// por la misma razón: la Fly Machine ya pasa el input por entorno, y tener dos
// convenciones de transporte en el mismo binario sería gratuito de evitar.
//
// Sin stdin: lo ocupa el RequestInput (spec 16 §5.2).
const StateConfigEnvVar = "VEX_STATE_CONFIG"

// ErrInputInvalido marca los errores que el proceso traduce a exit code 2. Un
// destino ausente, mal escrito o inalcanzable no es un fallo de la pipeline: es
// una invocación que no se puede atender, y el exit code tiene que decirlo
// (ver ExitCodeFor).
var ErrInputInvalido = errors.New("input invalido")

// inputError lleva el centinela en `Unwrap` y no en el texto: quien lee el
// stderr quiere la causa, no la etiqueta con la que el proceso elige su exit
// code. La causa original sigue en el árbol, así que `errors.Is` la encuentra
// —`syncconfig.ErrCongelado`, por ejemplo— igual que encuentra el centinela.
type inputError struct{ causa error }

func (e *inputError) Error() string   { return e.causa.Error() }
func (e *inputError) Unwrap() []error { return []error{e.causa, ErrInputInvalido} }

func inputErrorf(format string, a ...any) error {
	return &inputError{causa: fmt.Errorf(format, a...)}
}

// ExitCodeFor traduce un error de CONSTRUCCIÓN del motor al exit code del
// proceso. Existe porque el cableado puede fallar por dos razones distintas y
// sólo una de ellas es culpa de quien invocó.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSucceeded
	}
	if errors.Is(err, ErrInputInvalido) {
		return ExitInputError
	}
	return ExitFailed
}

// stateConfigDTO es la forma externa de la configuración de destino. Se decodifica
// con YAML, que acepta JSON tal cual: el contrato de §5.1 está escrito en YAML y
// quien lo genere desde código lo hará en JSON.
type stateConfigDTO struct {
	Type  string `yaml:"type"`
	Local struct {
		Path string `yaml:"path"`
	} `yaml:"local"`
	HTTP struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"http"`
}

// readStateConfig resuelve la configuración de destino: `--state-config <archivo>`
// → env var VEX_STATE_CONFIG (crudo o base64). **Sin default en ninguna capa**:
// si no hay ninguna de las dos, el motor no arranca.
//
// Todos los errores envuelven ErrInputInvalido, incluido el de `type: http`: es
// una invocación que el motor no puede atender, aunque la causa sea suya.
func readStateConfig(args RunArgs) (syncconfig.Config, error) {
	raw, origen, err := readStateConfigSource(args)
	if err != nil {
		return syncconfig.Config{}, err
	}

	dto, err := decodeStateConfig(raw)
	if err != nil {
		return syncconfig.Config{}, inputErrorf(
			"vexd run: configuración de destino (%s): %w", origen, err)
	}

	cfg, err := syncconfig.New(dto.Type, dto.Local.Path, dto.HTTP.Endpoint)
	if err != nil {
		return syncconfig.Config{}, inputErrorf(
			"vexd run: configuración de destino (%s): %w", origen, err)
	}
	return cfg, nil
}

func readStateConfigSource(args RunArgs) ([]byte, string, error) {
	if args.StateConfigFile != "" {
		data, err := os.ReadFile(args.StateConfigFile)
		if err != nil {
			return nil, "", inputErrorf(
				"vexd run: leer --state-config %s: %w", args.StateConfigFile, err)
		}
		return data, "--state-config " + args.StateConfigFile, nil
	}

	if raw := strings.TrimSpace(os.Getenv(StateConfigEnvVar)); raw != "" {
		return []byte(raw), "env var " + StateConfigEnvVar, nil
	}

	// El mensaje nombra las dos vías Y la versión de esquema: quien llegue aquí
	// es, casi siempre, un cliente escrito contra el contrato v1 —donde el
	// destino era `--mode` y seis endpoints—, y ese diagnóstico sin la pista de
	// la versión cuesta una tarde (spec 16 §7).
	return nil, "", inputErrorf(
		"vexd run: falta la configuración de destino del estado: pásala con --state-config <archivo>"+
			" o con la env var %s (YAML/JSON, crudo o base64). Desde schema_version %d el motor no tiene"+
			" un destino por defecto: --mode y los seis --step-*-endpoint se retiraron",
		StateConfigEnvVar, supportedSchemaVersion)
}

// decodeStateConfig acepta el contenido crudo o en base64. Se intenta primero el
// crudo y sólo se prueba base64 si aquel no produjo un documento con `type`:
// un base64 es un escalar YAML válido, así que el orden inverso decodificaría
// silencio en vez de configuración.
func decodeStateConfig(raw []byte) (stateConfigDTO, error) {
	dto, err := unmarshalStateConfig(raw)
	if err == nil && strings.TrimSpace(dto.Type) != "" {
		return dto, nil
	}

	decoded, b64Err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if b64Err == nil {
		if fromB64, decErr := unmarshalStateConfig(decoded); decErr == nil {
			return fromB64, nil
		}
	}

	if err != nil {
		return stateConfigDTO{}, err
	}
	return dto, nil
}

func unmarshalStateConfig(raw []byte) (stateConfigDTO, error) {
	var dto stateConfigDTO
	if err := yaml.Unmarshal(raw, &dto); err != nil {
		return stateConfigDTO{}, fmt.Errorf("no es YAML ni JSON válido: %w", err)
	}
	return dto, nil
}
