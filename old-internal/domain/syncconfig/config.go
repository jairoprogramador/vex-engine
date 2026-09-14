// Package syncconfig es el DESTINO del estado, expresado como dato.
//
// Sustituye al enum `--mode local|remote` (spec 16). La diferencia no es de
// forma: `--mode` era una bandera con un default cableado —`remote`, que
// significaba «Supabase»— dentro de la pieza que tiene que ser portable. Aquí no
// hay default en ninguna capa: si falta la configuración, el motor no arranca.
// El default vive en quien invoca.
//
// Es la tercera vez que la misma forma aparece en el diseño de Vex
// —`provider: keyvault|vault|github|local` para secretos, `vexconfig.yaml` sin
// nada implícito, y esto—, y de ahí que se escriba como patrón: contrato con
// `type` + payload específico, nunca un default cableado en el componente
// portable.
//
// El nombre resuelve además una colisión: `domain/state` son las variables con
// efecto real (spec 11), y esto es a dónde se sincroniza lo que se registra.
// Dos cosas sin relación con nombres casi idénticos.
package syncconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Type es el vocabulario CERRADO de destinos. Que sea cerrado es lo que permite
// que un `type` mal escrito falle al arrancar en vez de comportarse como el
// default de alguien.
type Type string

const (
	// TypeLocal escribe en una ruta del sistema de archivos: el volumen que el
	// invocador monta. No es un no-op y no puede serlo — ver ErrCongelado.
	TypeLocal Type = "local"

	// TypeHTTP empuja hacia un endpoint de ingesta. Está en el vocabulario y NO
	// está implementado: ver ErrCongelado.
	TypeHTTP Type = "http"
)

// ErrCongelado distingue «este destino existe y todavía no está implementado» de
// «esta configuración está mal escrita». Son dos diagnósticos distintos y el
// segundo no debe tapar al primero: quien pasa `type: http` no se equivocó de
// vocabulario, llegó antes que la spec 26.
var ErrCongelado = errors.New("destino congelado")

// Config es un value object: `Type` + el payload de ese tipo, validado en la
// construcción. No hay forma de tener una Config inválida en las manos.
type Config struct {
	tipo     Type
	path     string
	endpoint string
}

// New construye la configuración desde el vocabulario crudo tal como llega del
// transporte, y es el ÚNICO sitio donde se decide qué es válido.
func New(tipo, localPath, httpEndpoint string) (Config, error) {
	switch Type(strings.TrimSpace(tipo)) {
	case "":
		return Config{}, fmt.Errorf(
			"la configuración de destino no declara 'type' (vocabulario: %s)", vocabulario())

	case TypeLocal:
		path := strings.TrimSpace(localPath)
		if path == "" {
			return Config{}, errors.New("el destino 'local' no declara 'local.path'")
		}
		if !filepath.IsAbs(path) {
			// El motor cambia de directorio de trabajo durante la ejecución —cada
			// comando corre en el workdir de su step—, así que una ruta relativa
			// apuntaría a un sitio distinto según quién la resolviera.
			return Config{}, fmt.Errorf(
				"el destino 'local' declara una ruta relativa (%q): tiene que ser absoluta", path)
		}
		return Config{tipo: TypeLocal, path: filepath.Clean(path)}, nil

	case TypeHTTP:
		return Config{}, fmt.Errorf(
			"%w: 'type: http' todavía no está implementado, el motor sólo escribe en 'type: local'"+
				" (requiere el endpoint de ingesta de vex-portal-backend; lo descongela la spec 26)",
			ErrCongelado)

	default:
		return Config{}, fmt.Errorf(
			"tipo de destino %q desconocido (vocabulario: %s)", tipo, vocabulario())
	}
}

// NewLocal es el constructor del único tipo que hoy se puede construir.
func NewLocal(path string) (Config, error) {
	return New(string(TypeLocal), path, "")
}

func (c Config) Type() Type { return c.tipo }

// Path es la raíz del destino con `type: local`. Vacía para cualquier otro tipo.
func (c Config) Path() string { return c.path }

// Endpoint es la URL de ingesta con `type: http`. Hoy siempre vacía: ninguna
// Config de ese tipo llega a construirse.
func (c Config) Endpoint() string { return c.endpoint }

func (c Config) IsZero() bool { return c.tipo == "" }

func (c Config) String() string {
	if c.tipo == TypeLocal {
		return fmt.Sprintf("local:%s", c.path)
	}
	return string(c.tipo)
}

func vocabulario() string {
	return strings.Join([]string{string(TypeLocal), string(TypeHTTP)}, ", ")
}
