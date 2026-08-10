package record

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
	deploymentInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
)

var _ domRecord.ResultProjection = (*ScanProjection)(nil)

// ScanProjection es la implementación v1 del modelo de lectura: **escaneo**.
//
// # Toda consulta recorre, y es la decisión de la spec 22 §4
//
// Lee los JSONL bajo `events/` y aplica `Fold`. Cero lógica de plegado nueva —
// `Fold` ya existe y es puro—, cero dependencias nuevas, y suficiente durante
// meses. Lo que compra a cambio de escanear es ser **la implementación de
// referencia** contra la que se valida el pliegue del backend (spec 26): dos
// implementaciones de la misma lógica sin una que sirva de patrón divergen sin que
// nadie se entere.
//
// El límite está declarado: un `vex stats` a 90 días es malo aquí. La respuesta no
// es optimizar esto sino meter un índice DETRÁS DEL PUERTO, que es una decisión de
// rendimiento y no de arquitectura — `index.db` es una vista materializada,
// reconstruible por definición, así que añadirla no migra nada.
//
// # Dos raíces, y hay que elegir de cuál se lee
//
// `objects/` y `events/` existen en el área de trabajo y en el destino, y **no son
// equivalentes**: el área de trabajo es un SUPERCONJUNTO, porque un intento
// interrumpido no llega a empujarse nunca por la vía del cierre (spec 21 §9.11).
// Leer el destino contesta «¿qué se publicó?» y leer el área de trabajo contesta
// «¿qué ocurrió aquí?».
//
// Este tipo no elige: recibe las dos rutas ya resueltas y responde por ellas. Quien
// elige es `vexd record`, y su default es el área de trabajo — porque su función es
// validar lo que `vexd` escribe, y un intento que murió antes de empujar es
// justamente el caso que más falta hace poder mirar.
type ScanProjection struct {
	eventsRoot  string
	objectsRoot string
}

// NewScanProjection construye la proyección sobre una raíz.
//
// `objectsRoot` sólo lo necesita `History`, que filtra por ambiente y el ambiente
// vive en el objeto: los hechos no lo llevan, porque es INTENCIÓN y no
// circunstancia. Vacío degrada `History` a «no hay historia», no a un error — es lo
// que ocurre contra un destino al que nunca se empujó nada.
func NewScanProjection(eventsRoot, objectsRoot string) *ScanProjection {
	return &ScanProjection{eventsRoot: eventsRoot, objectsRoot: objectsRoot}
}

// Attempt pliega la tira del intento pedido.
//
// La ruta no se busca: se COMPONE con la misma función que la escribió
// (`StreamDir`), así que el escaneo se limita al directorio de ese despliegue. Es
// la única consulta de las dos que no recorre la tienda entera, y lo es porque el
// layout de la spec 17 puso el `deployment_id` en la ruta precisamente para eso.
//
// Dentro del directorio puede haber más de un archivo —uno por ejecución— y se
// selecciona por el `attempt` que sus hechos declaran, no por el nombre: el archivo
// lo nombra el `execution_id`, que es único y NO es ordenable (decisión I-2).
//
// Si de varias tiras del mismo intento sólo una tiene hechos, es ésa. Si hay
// varias con el mismo número —que este motor no produce, porque el linaje avanza
// en cada ejecución— gana la que más lejos llegó: la que tiene el `seq` mayor.
func (p *ScanProjection) Attempt(
	ctx *context.Context,
	id domDeployment.DeploymentID,
	attempt domDeployment.Attempt) (domRecord.AttemptResult, error) {

	if id.IsZero() {
		return domRecord.AttemptResult{}, errors.New(
			"scan projection: una consulta sin despliegue no tiene sujeto")
	}
	if attempt.IsZero() {
		return domRecord.AttemptResult{}, errors.New(
			"scan projection: el intento 0 no es un intento")
	}

	strips, err := p.Strips(ctx, id)
	if err != nil {
		return domRecord.AttemptResult{}, err
	}

	// El desempate es por POSICIÓN alcanzada y no por número de hechos: una tira con
	// líneas ilegibles o de un tipo desconocido —el estado normal de una tira leída
	// del destino— llegó tan lejos como dice su `seq` mayor, aunque produzca menos
	// hechos que otra más corta y limpia. Contar hechos elegiría la que llegó MENOS
	// lejos, que es lo contrario de lo que se busca.
	var elegida Strip
	for _, strip := range strips {
		if !strip.Attempt().Equals(attempt) {
			continue
		}
		if strip.MaxSeq() > elegida.MaxSeq() {
			elegida = strip
		}
	}
	if len(elegida.Events) == 0 {
		return domRecord.AttemptResult{}, fmt.Errorf(
			"%w: despliegue %s, intento %s", domRecord.ErrAttemptNoConsta, id, attempt)
	}
	return domRecord.Fold(elegida.Events), nil
}

// History son los intentos registrados contra un ambiente, del más reciente al más
// antiguo.
//
// # El ambiente no está en los hechos, así que hay que pasar por el objeto
//
// El `Destination` es INTENCIÓN: vive en el objeto de despliegue y no en la tira.
// Filtrar por ambiente exige entonces enlazar cada despliegue con su objeto, y ese
// enlace tampoco está escrito — se DERIVA recomputando `dep-v1` (ver
// `ObjectIndex.Link`). Un despliegue que no enlaza no entra en la historia de
// ningún ambiente, y decirlo es mejor que colarlo en la de todos.
//
// # El orden es por instante de inicio, y es el único caso donde el reloj decide
//
// `Fold` ordena por `seq` y nunca por tiempo, porque `seq` es la posición DENTRO de
// un intento. Entre intentos distintos no hay ninguna posición común: dos tiras no
// comparten numeración. Lo único comparable es el instante, con su límite conocido
// —relojes de máquinas distintas— y es preferible a ordenar por nombre de archivo,
// que es un `execution_id` deliberadamente no ordenable.
func (p *ScanProjection) History(
	ctx *context.Context,
	destination domDeployment.Destination,
	limit int) ([]domRecord.AttemptResult, error) {

	if destination.IsZero() {
		return nil, errors.New("scan projection: una historia sin ambiente no tiene sujeto")
	}

	deployments, err := p.Deployments()
	if err != nil {
		return nil, err
	}
	if len(deployments) == 0 {
		return nil, nil
	}

	index, err := deploymentInfra.ScanObjects(p.objectsRoot)
	if err != nil {
		return nil, err
	}
	enlace := index.Link(deployments)

	results := make([]domRecord.AttemptResult, 0, len(deployments))
	for _, id := range deployments {
		object, enlazado := enlace[id.String()]
		if !enlazado || object.Object.Destination != destination.String() {
			continue
		}
		strips, err := p.Strips(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, strip := range strips {
			if len(strip.Events) == 0 {
				continue
			}
			results = append(results, domRecord.Fold(strip.Events))
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].StartedAt.After(results[j].StartedAt)
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Strips son las tiras de un despliegue, leídas y ordenadas por nombre de archivo.
//
// Está exportada porque `record log` y `record verify` necesitan la tira CRUDA y no
// su pliegue: la primera imprime los sobres —incluidos los de un tipo que este
// binario no conoce— y la segunda tiene que recorrer las posiciones, porque el
// pliegue ignora los huecos a propósito. Devolver sólo `AttemptResult` obligaría a
// esas dos a abrir los archivos por su cuenta, que es cómo dos lectores del mismo
// formato se separan.
func (p *ScanProjection) Strips(
	_ *context.Context, id domDeployment.DeploymentID) ([]Strip, error) {

	if id.IsZero() {
		return nil, errors.New("scan projection: una tira sin despliegue no tiene dónde vivir")
	}

	dir := filepath.Join(p.eventsRoot, StreamDir(id))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan projection: listar %s: %w", dir, err)
	}

	strips := make([]Strip, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != eventsFileExt {
			continue
		}
		strip, err := ReadStrip(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		strips = append(strips, strip)
	}
	return strips, nil
}

// Deployments son los despliegues de los que hay al menos una tira.
//
// Sale de la RUTA y no de los hechos, y es la única vez que eso es correcto: el
// layout parte el `deployment_id` en `<versión>/<hash>` porque los dos puntos son
// ilegales en rutas de Windows, así que el directorio ES la identidad y recomponerla
// no es adivinar. Leer `attempt_started` de cada tira daría lo mismo y costaría
// abrir todos los archivos para saber cuáles hay.
//
// Un directorio cuyo nombre no compone una identidad válida se ignora: es basura en
// la tienda, no un despliegue.
func (p *ScanProjection) Deployments() ([]domDeployment.DeploymentID, error) {
	deployments := make([]domDeployment.DeploymentID, 0, 8)
	vistos := make(map[string]struct{}, 8)

	err := filepath.WalkDir(p.eventsRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != eventsFileExt {
			return nil
		}

		hashDir := filepath.Dir(path)
		id, parseErr := domDeployment.ParseDeploymentID(
			filepath.Base(filepath.Dir(hashDir)) + ":" + filepath.Base(hashDir))
		if parseErr != nil {
			return nil
		}
		if _, visto := vistos[id.String()]; visto {
			return nil
		}
		vistos[id.String()] = struct{}{}
		deployments = append(deployments, id)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("scan projection: recorrer %s: %w", p.eventsRoot, err)
	}

	sort.SliceStable(deployments, func(i, j int) bool {
		return deployments[i].String() < deployments[j].String()
	})
	return deployments, nil
}
