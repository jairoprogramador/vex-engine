package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

// El lado LECTOR de `objects/`, que la spec 22 publica.
//
// `FileObjectStore` sólo escribe: su `read` es privado y existe para sostener el
// write-once. Lo que la consulta necesita es otra cosa —recorrer la tienda,
// recomputar identidades y enlazar un despliegue con su intención— y ninguna de
// las tres es una operación del puerto de escritura. Por eso vive en su propio
// archivo y no cuelga de `ObjectStore`: un lector no debe poder escribir
// (spec 22 §5.2').

// IndexedObject es un objeto encontrado en la tienda, con el archivo del que
// salió.
//
// Lleva el DTO y no un `Content` rehidratado, y eso no es pereza: reconstruir un
// `Content` desde el archivo sería LOSSY —las reglas del step se serializan por su
// forma canónica, no por su gramática— y uno reconstruido a medias produce un
// `content_id` distinto del que el archivo declara, que es exactamente el defecto
// que `record verify` existe para detectar. Lo que la verificación necesita
// —recomputar el resumen de `canonical`— no requiere rehidratar nada.
type IndexedObject struct {
	Path   string
	Object FileObjectDTO
}

// ContentID es la identidad que el archivo AFIRMA tener. No se recomputa aquí:
// compararla con la recomputada es trabajo de `Verify`.
func (o IndexedObject) ContentID() string { return o.Object.ContentID }

// ObjectIndex es la tienda de objetos recorrida.
//
// Es una lista y no un mapa por `content_id` porque hace las dos cosas: buscar por
// identidad —que un mapa haría— y ENLAZAR un despliegue con su objeto, que exige
// recorrer todos los candidatos.
type ObjectIndex struct {
	objects []IndexedObject

	// ilegibles son los archivos que están y no se pudieron decodificar. Se
	// cuentan en vez de tumbar el recorrido: `objects/` es permanente, así que un
	// archivo roto es algo que hay que REPORTAR, y negarse a leer los demás lo
	// esconde.
	ilegibles []string
}

// ScanObjects recorre `<base>/<versión>/<2 hex>/<62 hex>.json`.
//
// Una raíz que no existe NO es un error: es una tienda vacía, y es el estado
// normal del destino antes del primer empuje (spec 21). Devolver error obligaría a
// cada llamador a distinguir «no hay nada» de «no se pudo mirar», que es
// exactamente lo que este tipo existe para no repetir.
func ScanObjects(basePath string) (*ObjectIndex, error) {
	index := &ObjectIndex{}

	err := filepath.WalkDir(basePath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != objectFileExt {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("object index: leer %s: %w", path, err)
		}
		var dto FileObjectDTO
		if err := json.Unmarshal(data, &dto); err != nil {
			index.ilegibles = append(index.ilegibles, path)
			return nil
		}
		index.objects = append(index.objects, IndexedObject{Path: path, Object: dto})
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	sort.SliceStable(index.objects, func(i, j int) bool {
		return index.objects[i].Object.ContentID < index.objects[j].Object.ContentID
	})
	return index, nil
}

// All son los objetos encontrados, ordenados por su `content_id`.
func (i *ObjectIndex) All() []IndexedObject { return i.objects }

// Ilegibles son los archivos que no se pudieron decodificar.
func (i *ObjectIndex) Ilegibles() []string { return i.ilegibles }

// ByContentID busca por la identidad que el archivo declara.
func (i *ObjectIndex) ByContentID(id string) (IndexedObject, bool) {
	for _, object := range i.objects {
		if object.Object.ContentID == id {
			return object, true
		}
	}
	return IndexedObject{}, false
}

// Link enlaza cada despliegue con el objeto del que salió, RECOMPUTANDO la regla
// `dep-v1`.
//
// # Por qué hay que derivarlo y no basta con leerlo
//
// Porque el enlace no está escrito en ninguna parte, y es una consecuencia del
// diseño y no un olvido: `attempt_started` es el único hecho que lleva el
// `deployment_id` y NO lleva el `content_id` —el objeto no repite la
// circunstancia y el hecho no repite la intención—, y el objeto no lleva su
// posición porque el mismo contenido cae en posiciones distintas cada vez que se
// vuelve a desplegar. Añadir el campo sería cambiar el vocabulario de eventos, que
// esta spec no toca.
//
// Lo que sí hay es la REGLA: `deployment_id = H(content_id, parent)`
// (`SPEC-CONTENT-v1.md` §6). Con los objetos de la tienda y los despliegues
// conocidos, recomponerla es una vuelta sobre el producto de los dos conjuntos, y
// tiene una propiedad que un campo persistido no daría — reproducir el enlace ES
// verificar `dep-v1`, así que un objeto que no enlaza con ningún despliegue es una
// señal y no un silencio.
//
// # Los candidatos a padre son los despliegues que se conocen, más la raíz
//
// La raíz —el `DeploymentID` cero— es «este es el primer despliegue de este
// ambiente», así que entra siempre. Los demás se pasan enteros: una cadena
// a → b → c se resuelve de una pasada, porque para enlazar `c` sólo hace falta
// que `b` esté en la lista, no que se haya enlazado antes.
//
// El límite, declarado: si el padre de un despliegue no está entre los conocidos
// —una cabeza de linaje cuyo intento murió entre situarse en la historia y abrir
// su tira— ese despliegue no enlaza. Es una ausencia honesta, no un error: quien
// pregunta recibe «no consta el objeto» y sigue viendo los hechos.
func (i *ObjectIndex) Link(
	deployments []domDeployment.DeploymentID) map[string]IndexedObject {

	if len(deployments) == 0 || len(i.objects) == 0 {
		return map[string]IndexedObject{}
	}

	buscados := make(map[string]struct{}, len(deployments))
	padres := make([]domDeployment.DeploymentID, 0, len(deployments)+1)
	padres = append(padres, domDeployment.DeploymentID{})
	for _, id := range deployments {
		if id.IsZero() {
			continue
		}
		if _, visto := buscados[id.String()]; visto {
			continue
		}
		buscados[id.String()] = struct{}{}
		padres = append(padres, id)
	}

	enlace := make(map[string]IndexedObject, len(buscados))
	for _, object := range i.objects {
		contentID, err := domDeployment.ParseContentID(object.Object.ContentID)
		if err != nil {
			// Un objeto cuya identidad declarada no tiene forma no enlaza con nada, y
			// eso lo reporta `Verify`: aquí sólo significa que no es candidato.
			continue
		}
		for _, parent := range padres {
			derivado, err := domDeployment.DeploymentIDOf(contentID, parent)
			if err != nil {
				continue
			}
			if _, buscado := buscados[derivado.String()]; !buscado {
				continue
			}
			// El primero gana y no se sobrescribe: dos objetos que derivaran el mismo
			// `deployment_id` serían una colisión de sha256, no un empate que haya que
			// desempatar.
			if _, ya := enlace[derivado.String()]; !ya {
				enlace[derivado.String()] = object
			}
		}
	}
	return enlace
}

// ObjectVerdict es el resultado de recomputar la identidad de un objeto.
type ObjectVerdict struct {
	// Recomputed es el `content_id` que sale de `canonical`, vacío cuando no se
	// pudo recomputar de forma comparable.
	Recomputed string

	// Declared es el que el archivo AFIRMA. Está aquí para que el diagnóstico
	// pueda enseñar los dos: una diferencia sin los dos valores no se depura.
	Declared string

	// Comparable dice si este binario implementa las reglas con las que el objeto
	// se identificó. **Falso NO es corrupción por sí solo** —puede ser una regla más
	// nueva— y no confundirlo es el punto.
	Comparable bool

	// Malformed dice que el archivo no declara una regla en absoluto: un prefijo
	// ausente, vacío o con un hash que no tiene forma de hash.
	//
	// **Es la distinción que separa «una regla que no tengo» de «un archivo roto»**,
	// y sin ella las dos se leen igual: un objeto con el token borrado saldría como
	// «este binario no calcula la regla ''» y `verify` diría que la tienda está bien.
	// Un token que no se puede ni leer no es un motor más nuevo — es corrupción de una
	// tienda permanente, y tiene que cambiar el exit code.
	Malformed bool

	// Matches dice si el recomputado coincide con el declarado. Sólo significa algo
	// cuando `Comparable` es cierto.
	Matches bool

	// Detail explica el veredicto para un humano, nombrando la REGLA cuando el
	// problema es de regla: «la huella del árbol no es comparable aquí» es
	// accionable; «el content_id no recomputa» no lo es.
	Detail string
}

// Verify recomputa la identidad de un objeto y dice si el archivo dice la verdad.
//
// # La comprobación entera es un sha256 sobre `canonical`
//
// El objeto lleva la forma normativa completa sobre la que se calculó su
// `content_id`, así que verificar no exige rehidratar el agregado ni volver a
// recorrer ningún árbol: `sha256(canonical)` contra el `content_id` declarado
// (spec 18 §9). De ahí sale una propiedad que conviene no perder de vista — esta
// comprobación **sí** es reproducible entre máquinas, porque las huellas de árbol
// entran en `canonical` como DATO y no se recalculan. El defecto heredado de la
// spec 08 §9.10 no la alcanza.
//
// # Y aun así hay TRES veredictos, no uno
//
// «No recomputa» es corrupción. «No puedo recomputarlo con la regla que tengo» no
// lo es, y confundirlas sería la peor forma de fallo posible para un comando que
// existe para detectar corrupción: señalar un registro perfectamente sano.
//
// El tercero es el que hace falta para que el segundo no se convierta en un agujero:
// **un token que no se puede ni leer no es una regla más nueva, es un archivo roto**.
// Sin separarlos, borrar el prefijo de un objeto lo volvería «no comparable» y
// `verify` diría que la tienda está bien.
//
// Los prefijos de versión son lo que separa los tres —para eso sirve llevarlos— y se
// leen ANTES de hashear nada: un objeto emitido con `cnt-v2`, o con una huella de
// árbol `v2:`, no se recomputa con la regla v1 para luego declararlo roto. El
// diagnóstico es POR REGLA, que es lo que lo hace accionable.
func (o IndexedObject) Verify() ObjectVerdict {
	verdict := ObjectVerdict{Declared: o.Object.ContentID}

	switch regla, estado := o.reglaAjena(); estado {
	case tokenMalformado:
		verdict.Malformed = true
		verdict.Detail = fmt.Sprintf(
			"el objeto no declara con qué regla se identificó (%s): un prefijo de versión"+
				" ausente o ilegible no es una regla más nueva, es un archivo roto", regla)
		return verdict
	case tokenDeOtraVersion:
		verdict.Detail = fmt.Sprintf(
			"este binario no calcula la regla '%s' con la que se identificó el objeto:"+
				" no se puede verificar aquí de forma comparable", regla)
		return verdict
	}
	verdict.Comparable = true

	if o.Object.Canonical == "" {
		// Sin material no hay nada que recomputar, y eso SÍ es un defecto del
		// archivo: el objeto se escribe con su forma canónica precisamente para que
		// esta comprobación exista (spec 17 §9).
		verdict.Malformed = true
		verdict.Detail = "el objeto no lleva su forma canónica: no hay material que recomputar"
		return verdict
	}

	sum := sha256.Sum256([]byte(o.Object.Canonical))
	verdict.Recomputed = domDeployment.ContentIDVersion + ":" + hex.EncodeToString(sum[:])
	verdict.Matches = verdict.Recomputed == verdict.Declared
	if verdict.Matches {
		verdict.Detail = "el content_id recomputa"
	} else {
		verdict.Detail = fmt.Sprintf(
			"el content_id declarado no es el de su material (recomputado %s)", verdict.Recomputed)
	}
	return verdict
}

// tokensLegiblesDeDeclaracion son los tokens que este binario acepta en el campo
// de la huella de declaración de un step del objeto.
//
// Son DOS, y la lista es la primera del motor en la que «lo que sé leer» deja de
// coincidir con «lo que sé calcular» (spec 27, spec 22 §9):
//
//	pipe-v1   la regla de hoy (SPEC-PIPELINE-v1.md)
//	inst-v1   la que `pipe-v1` absorbió, y que este binario ya NO calcula
//
// `inst-v1` se queda porque `verify` **no recomputa la regla: hashea el
// material**. Las huellas entran en la forma canónica como DATO, así que para
// verificar un objeto `inst-v1` no hace falta saber calcular `inst-v1`, hace
// falta reconocer el token como legítimo. Quitarla costaría que todos los objetos
// emitidos antes de la spec 27 pasaran a `not_comparable` —no a corrupción, que
// es lo correcto, pero dejarían de verificarse PARA SIEMPRE, siendo `objects/`
// write-once y permanente— a cambio de ahorrar una constante.
var tokensLegiblesDeDeclaracion = []string{
	fingerprint.DeclarationVersion,
	"inst-v1",
}

// tokenEstado es lo que se puede decir de un prefijo de versión mirándolo.
type tokenEstado int

const (
	// tokenPropio es la regla que este binario calcula.
	tokenPropio tokenEstado = iota

	// tokenDeOtraVersion es una regla bien formada que este binario no implementa: un
	// motor más nuevo. NO es corrupción.
	tokenDeOtraVersion

	// tokenMalformado es un valor que no declara ninguna regla. SÍ es corrupción.
	tokenMalformado
)

// reglaAjena devuelve la primera regla del objeto que este binario no puede usar, y
// en qué sentido no puede usarla.
//
// Son las tres capas de versión que el material transporta, y se comprueban las
// tres porque son INDEPENDIENTES (`SPEC-CONTENT-v1.md` §2): la composición del
// objeto, la huella del árbol —dos veces, proyecto y pipelinecode— y la huella de
// la declaración de cada step. Cualquiera puede saltar a v2 sin que las otras se
// muevan, así que mirar sólo el prefijo del `content_id` dejaría pasar un objeto
// compuesto con una huella que este binario no sabe reproducir.
//
// La regla de composición se comprueba sobre el prefijo del `content_id` y no
// sobre el nombre del directorio: el archivo tiene que poder responder por sí solo.
func (o IndexedObject) reglaAjena() (string, tokenEstado) {
	// El orden es el de la composición: primero la identidad, luego el material del
	// que sale. Da el diagnóstico más útil cuando falla más de una.
	campos := []tokenEsperado{
		{o.Object.ContentID, []string{domDeployment.ContentIDVersion}},
		{o.Object.Source.Project, []string{fingerprint.Version}},
		{o.Object.Source.Pipeline, []string{fingerprint.Version}},
	}
	for _, step := range o.Object.Steps {
		campos = append(campos, tokenEsperado{step.Declaration, tokensLegiblesDeDeclaracion})
	}

	for _, campo := range campos {
		if regla, estado := clasificarToken(campo.valor, campo.esperadas); estado != tokenPropio {
			return regla, estado
		}
	}
	return "", tokenPropio
}

// tokenEsperado empareja un valor con las reglas con las que este binario sabe
// LEERLO — que no tienen por qué ser las que sabe calcular.
type tokenEsperado struct {
	valor     string
	esperadas []string
}

// clasificarToken separa las tres formas de un `<versión>:<hash>`.
//
// Un hash que no tiene forma de hash cuenta como malformado y no como «otra
// versión», y es deliberado: un motor más nuevo cambiaría la REGLA, no dejaría de
// escribir un sha256 en hexadecimal. Sin esa comprobación, corromper la mitad
// derecha de un identificador pasaría por una regla del futuro.
func clasificarToken(valor string, esperadas []string) (string, tokenEstado) {
	version, hash, found := strings.Cut(valor, ":")
	if !found || version == "" {
		return valor, tokenMalformado
	}
	if !esHashDeSha256(hash) {
		return version, tokenMalformado
	}
	for _, esperada := range esperadas {
		if version == esperada {
			return version, tokenPropio
		}
	}
	return version, tokenDeOtraVersion
}

func esHashDeSha256(hash string) bool {
	if len(hash) != sha256HexLen {
		return false
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// sha256HexLen es la longitud en hexadecimal de un sha256, la misma que valida
// `versionedHash` en el dominio. Se repite aquí porque aquélla no está exportada y
// exportarla para esto sería abrir el value object para que un lector lo mire por
// dentro.
const sha256HexLen = 64
