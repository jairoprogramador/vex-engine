package record

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// secretBytes es el tamaño del secreto local. 32 bytes es el tamaño del
	// bloque de salida de SHA-256: más no añade nada y menos lo debilita.
	secretBytes = 32

	// secretPerm son los permisos del archivo. Es la única pieza del motor que
	// nace 0600: todo lo demás que el motor escribe está pensado para leerse.
	secretPerm fs.FileMode = 0o600

	// secretDirPerm son los permisos del directorio que lo contiene.
	secretDirPerm fs.FileMode = 0o700
)

// ReadOrCreateDigestSecret devuelve el secreto local con el que se derivan las
// claves de resumen, creándolo en el primer uso.
//
// # Por qué vive en el DESTINO y no en el área de trabajo del motor
//
// Es la decisión incómoda de la spec 20 §5.2 y conviene tenerla escrita, porque
// la palabra «local» del texto invita a la respuesta equivocada. El resumen
// existe para comparar un proyecto contra SU PROPIA HISTORIA —«`DB_POOL_SIZE`
// cambió de 10 a 50»—, así que dos ejecuciones del mismo proyecto tienen que
// producir el mismo digest para el mismo valor. En la topología real esas dos
// ejecuciones ocurren en dos máquinas efímeras distintas, cuyo `$HOME` nace
// vacío y muere con el contenedor: un secreto guardado ahí se regeneraría en
// cada intento y todos los digests dejarían de compararse, **en silencio**.
//
// Lo único que esas dos máquinas comparten por configuración es el destino del
// estado. Por eso el secreto cuelga de ahí, y por eso cuelga en un directorio
// propio: `state/`, `cache/`, `lineage/` —y con la spec 21, `objects/` y
// `events/`— son el registro; `keys/` NO lo es, y no debe sincronizarse a ningún
// sitio. Es la frase que el §6 de la spec pide («la clave, fuera del registro»)
// dicha en una ruta.
//
// El destino ya guarda los valores en claro (`state/…`, spec 11 §5.3), así que
// no se está bajando el listón de confianza de nada: se está poniendo la clave
// donde ya vive lo que protegería.
//
// # Crear no es sobrescribir
//
// Se abre con O_EXCL y no con el escritor atómico del repositorio: un rename
// PUBLICA, y publicar encima de un secreto que ya existía invalidaría toda la
// historia de resúmenes de ese destino sin decir nada. Si otro proceso lo creó
// entre el `Stat` y el `OpenFile`, se relee el suyo.
func ReadOrCreateDigestSecret(path string) ([]byte, error) {
	secreto, err := readDigestSecret(path)
	if err == nil {
		return secreto, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), secretDirPerm); err != nil {
		return nil, fmt.Errorf("record: crear el directorio del secreto de resumen: %w", err)
	}

	nuevo := make([]byte, secretBytes)
	if _, err := rand.Read(nuevo); err != nil {
		return nil, fmt.Errorf("record: generar el secreto de resumen: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, secretPerm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// Alguien se adelantó. El suyo es tan bueno como el nuestro y además
			// es el que ya se usó: se relee.
			return readDigestSecret(path)
		}
		return nil, fmt.Errorf("record: crear el secreto de resumen en %q: %w", path, err)
	}

	if _, err := file.WriteString(hex.EncodeToString(nuevo) + "\n"); err != nil {
		file.Close()
		return nil, fmt.Errorf("record: escribir el secreto de resumen en %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("record: sincronizar el secreto de resumen en %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("record: cerrar el secreto de resumen en %q: %w", path, err)
	}
	return nuevo, nil
}

// readDigestSecret lee el secreto ya existente. Va en hexadecimal y no en crudo
// para que el archivo se pueda copiar, comparar y diagnosticar con las
// herramientas de siempre sin que nadie lo abra en un editor y le añada un BOM.
func readDigestSecret(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("record: leer el secreto de resumen de %q: %w", path, err)
	}

	secreto, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		// Un secreto ilegible NO se regenera: regenerarlo dejaría de comparar con
		// toda la historia anterior sin que nadie lo pidiera, que es exactamente el
		// fallo silencioso que esta pieza existe para evitar.
		return nil, fmt.Errorf(
			"record: el secreto de resumen de %q no es hexadecimal válido: %w", path, err)
	}
	if len(secreto) < secretBytes {
		return nil, fmt.Errorf(
			"record: el secreto de resumen de %q tiene %d bytes y necesita %d",
			path, len(secreto), secretBytes)
	}
	return secreto, nil
}
