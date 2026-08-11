package deployment_test

// El lado LECTOR de `objects/` (spec 22).
//
// Dos cosas se prueban aquí y las dos son la razón de que este archivo exista:
//
//  1. **Verify distingue dos diagnósticos.** «El `content_id` no recomputa» es
//     corrupción; «no puedo recomputarlo con la regla que tengo» no lo es. Mismo
//     comando, dos diagnósticos distintos (spec 22 §7), y confundirlos sería la
//     peor forma de fallo posible: señalar un registro sano.
//  2. **El enlace despliegue→objeto se DERIVA.** No está escrito en ninguna parte
//     —el hecho no lleva el `content_id` y el objeto no lleva su posición—, así que
//     reproducirlo recomputando `dep-v1` es la única vía y a la vez una
//     verificación de la regla.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	infraDeployment "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
)

// UNA TIENDA QUE NO EXISTE ES UNA TIENDA VACÍA, no un error. Es el estado normal del
// destino antes del primer empuje (spec 21).
func TestScanObjects_UnaRaizAusenteEsUnaTiendaVacia(t *testing.T) {
	index, err := infraDeployment.ScanObjects(filepath.Join(t.TempDir(), "no-existe"))

	require.NoError(t, err)
	assert.Empty(t, index.All())
}

// EL OBJETO QUE EL MOTOR ESCRIBE RECOMPUTA. Es la línea base: sin ella, un defecto
// que los demás casos encuentren podría ser de la propia comprobación.
func TestVerify_ElObjetoQueElMotorEscribeRecomputa(t *testing.T) {
	base := t.TempDir()
	escribirObjeto(t, base, contenidoDePrueba(t, "deploy", "sand"))

	index, err := infraDeployment.ScanObjects(base)
	require.NoError(t, err)
	require.Len(t, index.All(), 1)

	verdict := index.All()[0].Verify()

	assert.True(t, verdict.Comparable)
	assert.True(t, verdict.Matches)
	assert.Equal(t, verdict.Declared, verdict.Recomputed)
}

// EL PRIMERO DE LOS DOS DIAGNÓSTICOS: un `content_id` que no es el de su material.
//
// Es CORRUPCIÓN, y es exactamente lo que un almacén direccionado por contenido no
// puede tener: el archivo afirma ser algo que no es.
func TestVerify_UnContentIDQueNoRecomputaEsCorrupcion(t *testing.T) {
	base := t.TempDir()
	path := escribirObjeto(t, base, contenidoDePrueba(t, "deploy", "sand"))

	// Se toca el MATERIAL y no el identificador, que es la forma en la que esto
	// ocurre de verdad: un byte que cambia bajo un identificador que ya se emitió.
	mutarObjeto(t, path, func(dto *infraDeployment.FileObjectDTO) {
		dto.Canonical += "\ncontaminado"
	})

	index, err := infraDeployment.ScanObjects(base)
	require.NoError(t, err)
	verdict := index.All()[0].Verify()

	assert.True(t, verdict.Comparable, "la regla se conoce: lo que no cuadra es el contenido")
	assert.False(t, verdict.Matches)
	assert.NotEqual(t, verdict.Declared, verdict.Recomputed)
}

// EL SEGUNDO DIAGNÓSTICO, y es el cuarto caso de la §7: un registro SANO que este
// binario no puede verificar de forma comparable.
//
// El objeto se identificó con una regla de huella de árbol que este binario no
// calcula. Recomputar y fallar sería señalar corrupción donde no hay ninguna —el
// registro está perfecto, lo que falta es la regla— y por eso el prefijo de versión
// se lee ANTES de hashear nada. Para eso sirve llevarlo.
func TestVerify_UnaReglaQueEsteBinarioNoCalculaNoEsCorrupcion(t *testing.T) {
	base := t.TempDir()

	for _, caso := range []struct {
		nombre string
		mutar  func(*infraDeployment.FileObjectDTO)
		regla  string
	}{
		{
			nombre: "la huella del árbol del proyecto",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.Source.Project = "v2:" + strings.Repeat("11", 32)
			},
			regla: "v2",
		},
		{
			nombre: "la huella de las instrucciones de un step",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.Steps[0].Declaration = "inst-v2:" + strings.Repeat("33", 32)
			},
			regla: "inst-v2",
		},
		{
			nombre: "la composición del objeto",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.ContentID = "cnt-v2:" + strings.Repeat("44", 32)
			},
			regla: "cnt-v2",
		},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			dir := filepath.Join(base, caso.regla)
			path := escribirObjeto(t, dir, contenidoDePrueba(t, "deploy", "sand"))
			mutarObjeto(t, path, caso.mutar)

			index, err := infraDeployment.ScanObjects(dir)
			require.NoError(t, err)
			verdict := index.All()[0].Verify()

			assert.False(t, verdict.Comparable)
			assert.False(t, verdict.Matches)
			assert.Empty(t, verdict.Recomputed,
				"no se recomputa: hacerlo con otra regla daría un valor que no significa nada")
			assert.Contains(t, verdict.Detail, caso.regla,
				"el diagnóstico nombra la REGLA, que es lo que lo hace accionable")
		})
	}
}

// EL TERCER VEREDICTO, que es el que impide que el segundo sea un agujero: **un token
// que no se puede leer no es una regla más nueva, es un archivo roto**.
//
// Sin la distinción, borrar el prefijo de un objeto lo volvería «no comparable» —que
// no cuenta como corrupción— y `verify` diría que la tienda está bien y que el aviso
// se puede ignorar. Un motor más nuevo cambiaría la REGLA; no dejaría de escribir un
// sha256 en hexadecimal.
func TestVerify_UnTokenIlegibleEsUnArchivoRotoYNoUnaReglaNueva(t *testing.T) {
	base := t.TempDir()

	for _, caso := range []struct {
		nombre string
		mutar  func(*infraDeployment.FileObjectDTO)
	}{
		{
			nombre: "el prefijo borrado",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.Source.Project = ""
			},
		},
		{
			nombre: "el prefijo vacío pero con separador",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.Source.Pipeline = ":" + strings.Repeat("22", 32)
			},
		},
		{
			nombre: "sin separador",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.ContentID = strings.Repeat("44", 32)
			},
		},
		{
			nombre: "un hash que no tiene forma de hash",
			mutar: func(dto *infraDeployment.FileObjectDTO) {
				dto.Steps[0].Declaration = "inst-v1:no-es-un-sha256"
			},
		},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			dir := filepath.Join(base, strings.ReplaceAll(caso.nombre, " ", "-"))
			path := escribirObjeto(t, dir, contenidoDePrueba(t, "deploy", "sand"))
			mutarObjeto(t, path, caso.mutar)

			index, err := infraDeployment.ScanObjects(dir)
			require.NoError(t, err)
			verdict := index.All()[0].Verify()

			assert.True(t, verdict.Malformed,
				"un token ilegible es corrupción de una tienda permanente")
			assert.False(t, verdict.Comparable)
			assert.False(t, verdict.Matches)
		})
	}
}

// LAS TRES CAPAS DE VERSIÓN SON INDEPENDIENTES, y por eso se comprueban las tres.
//
// Mirar sólo el prefijo del `content_id` dejaría pasar un objeto compuesto sobre una
// huella que este binario no sabe reproducir (`SPEC-CONTENT-v1.md` §2).
func TestVerify_UnObjetoSinFormaCanonicaNoSePuedeVerificar(t *testing.T) {
	base := t.TempDir()
	path := escribirObjeto(t, base, contenidoDePrueba(t, "deploy", "sand"))
	mutarObjeto(t, path, func(dto *infraDeployment.FileObjectDTO) { dto.Canonical = "" })

	index, err := infraDeployment.ScanObjects(base)
	require.NoError(t, err)
	verdict := index.All()[0].Verify()

	assert.True(t, verdict.Comparable, "las reglas se conocen: lo que falta es el material")
	assert.True(t, verdict.Malformed,
		"el objeto se escribe con su forma canónica para que esta comprobación exista")
	assert.False(t, verdict.Matches)
	assert.Contains(t, verdict.Detail, "forma canónica")
}

// EL ENLACE SE DERIVA DE LA REGLA, y con la cadena entera de una sola pasada.
//
// `deployment_id = H(content_id, parent)`, así que con los objetos de la tienda y los
// despliegues conocidos el enlace se recompone. Para enlazar el tercero de una cadena
// basta que el segundo esté en la lista: no hace falta haberlo enlazado antes.
func TestLink_ElEnlaceDespliegueObjetoSeDerivaDeLaRegla(t *testing.T) {
	base := t.TempDir()

	primero := contenidoDePrueba(t, "deploy", "sand")
	segundo := contenidoDePrueba(t, "deploy", "prod")
	escribirObjeto(t, base, primero)
	escribirObjeto(t, base, segundo)

	raiz, err := domDeployment.DeploymentIDOf(primero.ID(), domDeployment.DeploymentID{})
	require.NoError(t, err)
	hijo, err := domDeployment.DeploymentIDOf(segundo.ID(), raiz)
	require.NoError(t, err)
	nieto, err := domDeployment.DeploymentIDOf(primero.ID(), hijo)
	require.NoError(t, err)

	index, err := infraDeployment.ScanObjects(base)
	require.NoError(t, err)

	enlace := index.Link([]domDeployment.DeploymentID{raiz, hijo, nieto})

	require.Len(t, enlace, 3)
	assert.Equal(t, primero.ID().String(), enlace[raiz.String()].ContentID())
	assert.Equal(t, segundo.ID().String(), enlace[hijo.String()].ContentID())
	assert.Equal(t, primero.ID().String(), enlace[nieto.String()].ContentID(),
		"el mismo contenido en otra posición: es la diferencia entre las dos identidades")
}

// EL LÍMITE DEL ENLACE, DECLARADO: si el padre no está entre los conocidos, el
// despliegue no enlaza.
//
// Ocurre de verdad —una cabeza de linaje cuyo intento murió entre situarse en la
// historia y abrir su tira— y la respuesta correcta es una ausencia honesta: quien
// pregunta recibe «no consta el objeto» y sigue viendo los hechos.
func TestLink_SinElPadreEntreLosConocidosNoHayEnlace(t *testing.T) {
	base := t.TempDir()
	contenido := contenidoDePrueba(t, "deploy", "sand")
	escribirObjeto(t, base, contenido)

	huerfano, err := domDeployment.ParseDeploymentID(
		domDeployment.DeploymentIDVersion + ":" + strings.Repeat("ef", 32))
	require.NoError(t, err)

	index, err := infraDeployment.ScanObjects(base)
	require.NoError(t, err)

	assert.Empty(t, index.Link([]domDeployment.DeploymentID{huerfano}))
}

// UN OBJETO ILEGIBLE SE CUENTA Y NO TUMBA EL RECORRIDO.
//
// `objects/` es permanente, así que un archivo roto es algo que hay que REPORTAR;
// negarse a leer los demás por él lo esconde.
func TestScanObjects_UnObjetoIlegibleSeCuentaSinTumbarElRecorrido(t *testing.T) {
	base := t.TempDir()
	escribirObjeto(t, base, contenidoDePrueba(t, "deploy", "sand"))

	roto := filepath.Join(base, domDeployment.ContentIDVersion, "zz", "roto.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(roto), 0o755))
	require.NoError(t, os.WriteFile(roto, []byte("{no es json"), 0o644))

	index, err := infraDeployment.ScanObjects(base)

	require.NoError(t, err)
	assert.Len(t, index.All(), 1)
	assert.Len(t, index.Ilegibles(), 1)
}

// ── Utilidades ──────────────────────────────────────────────────────────────

// escribirObjeto usa el almacén REAL: lo que se verifica tiene que ser lo que el
// motor escribe, no una forma parecida montada a mano.
func escribirObjeto(t *testing.T, base string, content domDeployment.Content) string {
	t.Helper()

	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{
		ProjectCommit: "abc123", PipelineCommit: "def456",
	}))

	hash := content.ID().Hash()
	return filepath.Join(base, content.ID().Version(), hash[:2], hash[2:]+".json")
}

// mutarObjeto reescribe el archivo con el cambio pedido. Se hace a mano y por fuera
// del almacén a propósito: el almacén no deja escribir un objeto contradictorio, y lo
// que estos casos montan es precisamente el archivo que no debería existir.
func mutarObjeto(t *testing.T, path string, mutar func(*infraDeployment.FileObjectDTO)) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var dto infraDeployment.FileObjectDTO
	require.NoError(t, json.Unmarshal(data, &dto))
	mutar(&dto)

	mutado, err := json.MarshalIndent(dto, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, mutado, 0o644))
}
