package sync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	domSync "github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
	deploymentInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/deployment"
	recordInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
)

const (
	// Los mismos permisos que el resto de las tiendas.
	sinkDirPerm  os.FileMode = 0o755
	sinkFilePerm os.FileMode = 0o644

	// maxEventLine acota la línea que el escáner acepta. Una tira es un archivo
	// que puede haber quedado a medias en una muerte dura, así que el lector no
	// puede confiar en que lo que encuentra sea una línea.
	maxEventLine = 1 << 20
)

var _ domSync.Sink = (*LocalSink)(nil)

// LocalSink escribe de `staging/` a `destino.path`.
//
// # NO es un no-op, y no puede serlo
//
// Es la mitad que la spec 16 anunció y ésta ejecuta de verdad: no se puede
// garantizar que el volumen esté montado, y empujar a ciegas dejaría al motor
// «sincronizando» contra el filesystem efímero del contenedor SIN NINGUNA SEÑAL.
// Por eso el destino se comprueba en cada empuje y un fallo es ruidoso —ruta
// inexistente, permisos, disco lleno—, que es también lo que lo hace sustituible
// por el sink `http` (LSP, §5.2'): si uno callara y el otro no, el cliente no
// podría tratarlos igual.
//
// # Las dos tiendas se empujan con reglas distintas porque son cosas distintas
//
//   - `objects/`: write-once direccionado por contenido. Se delega en
//     `FileObjectStore`, la implementación de referencia (spec 18): idempotente
//     si el contenido coincide —volver a declarar la misma intención es lo que
//     pasa en cada re-despliegue sin cambios— y ERROR si la misma dirección trae
//     otro contenido, comparando la forma CANÓNICA y no el archivo byte a byte.
//     El ingestor de la spec 26 tiene que hacer exactamente lo mismo.
//   - `events/`: anexo por posición. Dentro de una tira el `seq` es único y
//     monótono, así que «anexa lo mayor que lo último que hay en el destino» ES
//     el insert-if-absent que §5.2 pide por `event_id`, sin leer ninguno. De ahí
//     que re-empujar desde cero contra un destino ya poblado no duplique nada.
type LocalSink struct {
	stagingPath string
	destination string

	// objects es OTRO almacén de objetos, con la misma implementación y otra
	// raíz. No se comparte con el del área de trabajo: son dos tiendas, y el
	// empuje es precisamente el paso de una a la otra.
	objects domDeployment.ObjectStore
}

func NewLocalSink(stagingPath, destinationPath string) *LocalSink {
	return &LocalSink{
		stagingPath: stagingPath,
		destination: destinationPath,
		objects: deploymentInfra.NewFileObjectStore(
			filepath.Join(destinationPath, deploymentInfra.ObjectsDirName)),
	}
}

// Destination es cómo se nombra este destino en el `sync_failed` que explica un
// hueco. Lleva el tipo delante por lo mismo que `syncconfig.Config.String()`:
// «no llegó» sin decir a dónde no llegó no explica nada.
func (s *LocalSink) Destination() string { return "local:" + s.destination }

func (s *LocalSink) Push(ctx *context.Context, batch domSync.Batch) (domRecord.Seq, error) {
	if batch.IsZero() {
		return domRecord.Seq{}, fmt.Errorf("local sink: no hay lote que empujar")
	}
	if err := s.alcanzable(); err != nil {
		return domRecord.Seq{}, err
	}

	// El objeto primero, y el orden importa en una sola dirección: unos hechos en
	// el destino que apuntan a una intención que no está allí dejan el registro
	// remoto afirmando algo que no se puede leer. Al revés —objeto sin hechos— es
	// un intento que se empujó pronto, que es un estado normal.
	if err := s.objects.Put(ctx, batch.Content(), batch.Metadata()); err != nil {
		return domRecord.Seq{}, fmt.Errorf("local sink: empujar la intención del despliegue: %w", err)
	}

	return s.pushEvents(batch)
}

// alcanzable es la comprobación que convierte «el volumen no está montado» en un
// fallo inmediato con causa clara, en vez de en un directorio creado dentro del
// contenedor efímero.
//
// Se hace en cada empuje y no sólo al arrancar: un volumen se puede desmontar a
// mitad de un despliegue, y ése es justo el caso en el que el silencio cuesta
// caro.
func (s *LocalSink) alcanzable() error {
	info, err := os.Stat(s.destination)
	if err != nil {
		return fmt.Errorf(
			"local sink: destino inaccesible en %q (¿el volumen no está montado?): %w",
			s.destination, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local sink: el destino %q no es un directorio", s.destination)
	}
	return nil
}

// pushEvents anexa al destino los hechos que todavía no tiene.
//
// El suelo es el MAYOR de dos cosas: lo que el `ack` dice que ya se confirmó y
// lo que el destino tiene de verdad. Que se mire el destino y no sólo el `ack` es
// lo que hace que forzar `seq = 0` contra un destino poblado no duplique — y lo
// que hace que el `ack` sea una optimización y no un punto de control.
func (s *LocalSink) pushEvents(batch domSync.Batch) (domRecord.Seq, error) {
	rel := recordInfra.StreamRelPath(batch.Stream())
	origen := filepath.Join(s.stagingPath, recordInfra.EventsDirName, rel)
	destino := filepath.Join(s.destination, recordInfra.EventsDirName, rel)

	presente, terminada, err := ultimaPosicion(destino)
	if err != nil {
		return domRecord.Seq{}, err
	}

	suelo := presente
	if desde := batch.From().Position(); desde > suelo {
		suelo = desde
	}

	lineas, ultima, err := lineasDesde(origen, suelo)
	if err != nil {
		return domRecord.Seq{}, err
	}
	if len(lineas) == 0 {
		return posicion(suelo), nil
	}

	if err := anexar(destino, lineas, terminada); err != nil {
		return domRecord.Seq{}, err
	}
	return posicion(ultima), nil
}

// ultimaPosicion es el `seq` mayor que el destino ya tiene para esta tira, y si
// el archivo termina en una línea completa.
//
// Una línea ILEGIBLE se ignora en vez de hacer fallar el empuje: la única forma
// de que aparezca una es que una máquina muriera a mitad de un `write`, y
// negarse a seguir dejaría el destino congelado para siempre por culpa de un
// byte. Lo que sí se conserva es que la línea rota no se pisa: nunca se trunca
// nada del destino (ver `anexar`).
func ultimaPosicion(path string) (uint64, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, true, nil
		}
		return 0, false, fmt.Errorf("local sink: abrir la tira del destino %s: %w", path, err)
	}
	defer file.Close()

	var mayor uint64
	terminada := true
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	for scanner.Scan() {
		linea := scanner.Bytes()
		if len(bytes.TrimSpace(linea)) == 0 {
			continue
		}
		seq, ok := posicionDe(linea)
		terminada = ok
		if ok && seq > mayor {
			mayor = seq
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, false, fmt.Errorf("local sink: leer la tira del destino %s: %w", path, err)
	}
	return mayor, terminada, nil
}

// lineasDesde selecciona del área de trabajo las líneas con posición MAYOR que
// el suelo, en el orden en que están.
//
// La tira local es de solo-anexar y se escribe en orden de `seq`, así que
// conservar el orden del archivo es conservar el orden de observación. Una línea
// que no se puede leer se salta: es la cola de una muerte dura, y no hay forma de
// empujar media línea.
func lineasDesde(path string, suelo uint64) ([][]byte, uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Que no haya tira local es una anomalía y no una ausencia: para llegar
			// aquí el resolutor ya abrió el registro del intento. Decirlo es mejor
			// que devolver «no había nada pendiente», que se lee como un empuje que
			// funcionó.
			return nil, 0, fmt.Errorf(
				"local sink: la tira del intento no está en el área de trabajo (%s)", path)
		}
		return nil, 0, fmt.Errorf("local sink: abrir la tira %s: %w", path, err)
	}
	defer file.Close()

	lineas := make([][]byte, 0, 16)
	ultima := suelo
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	for scanner.Scan() {
		linea := scanner.Bytes()
		if len(bytes.TrimSpace(linea)) == 0 {
			continue
		}
		seq, ok := posicionDe(linea)
		if !ok || seq <= suelo {
			continue
		}
		lineas = append(lineas, append([]byte(nil), linea...))
		if seq > ultima {
			ultima = seq
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("local sink: leer la tira %s: %w", path, err)
	}
	return lineas, ultima, nil
}

// anexar escribe las líneas al final de la tira del destino.
//
// Usa `O_APPEND` con `Sync` y no el escritor atómico por lo mismo que el sink
// local de hechos (spec 19): esto ANEXA, y un rename por empuje reescribiría el
// archivo entero cada vez. El `Sync` tampoco es prudencia — el caso que este
// archivo cubre es el de la máquina que muere.
//
// Si el destino traía una línea a medias se le pone un salto delante en vez de
// truncarla: **del destino no se borra nada**. Lo que queda es una línea rota
// entre líneas buenas, que un lector sabe saltarse; lo que un truncado podría
// llevarse es un hecho que otra máquina acababa de escribir.
func anexar(path string, lineas [][]byte, terminada bool) error {
	if err := os.MkdirAll(filepath.Dir(path), sinkDirPerm); err != nil {
		return fmt.Errorf("local sink: crear el directorio de %s: %w", path, err)
	}

	var buffer bytes.Buffer
	if !terminada {
		buffer.WriteByte('\n')
	}
	for _, linea := range lineas {
		buffer.Write(linea)
		buffer.WriteByte('\n')
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, sinkFilePerm)
	if err != nil {
		return fmt.Errorf("local sink: abrir la tira del destino %s: %w", path, err)
	}
	if _, err := file.Write(buffer.Bytes()); err != nil {
		_ = file.Close()
		return fmt.Errorf("local sink: escribir en %s: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("local sink: sincronizar %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("local sink: cerrar %s: %w", path, err)
	}
	return nil
}

// posicionDe lee el `seq` del sobre sin decodificar la carga.
//
// Basta con el sobre: la selección es por posición, y el empuje transporta la
// línea TAL CUAL. Decodificar el hecho entero obligaría a este adaptador a
// conocer el vocabulario —y a rechazar un tipo que un motor más nuevo hubiera
// escrito, que es justo lo contrario de lo que un transporte debe hacer.
func posicionDe(linea []byte) (uint64, bool) {
	var sobre struct {
		Seq uint64 `json:"seq"`
	}
	if err := json.Unmarshal(linea, &sobre); err != nil {
		return 0, false
	}
	return sobre.Seq, sobre.Seq > 0
}

// posicion traduce el contador a la posición del dominio. El cero no es una
// posición: es «no consta», que es lo que significa un destino sin nada de esta
// tira.
func posicion(valor uint64) domRecord.Seq {
	if valor == 0 {
		return domRecord.Seq{}
	}
	seq, err := domRecord.NewSeq(valor)
	if err != nil {
		return domRecord.Seq{}
	}
	return seq
}
