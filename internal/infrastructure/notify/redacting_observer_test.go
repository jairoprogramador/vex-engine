package notify_test

// La redacción del stream de logs (spec 20 §5.3, D-A9 cerrada).
//
// Los tres casos que dan valor a esta pieza son las dos REGLAS que le faltaban
// al mecanismo propuesto —umbral de longitud y orden por longitud— y la
// LIMITACIÓN que se documenta en vez de disimularse. El tercero es un test que
// afirma un límite conocido, no un defecto pendiente.

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
)

// vocabulario es el mapa de valores conocidos, sin montar medio dominio: lo que
// el puerto pide es exactamente esto.
type vocabulario map[string]string

func (v vocabulario) Values() map[string]string { return v }

var _ domNotify.Vocabulary = vocabulario(nil)

// espia recoge lo que le llega ya redactado.
type espia struct {
	lineas  []string
	cerrado bool
}

func (e *espia) Notify(_ string, line string) { e.lineas = append(e.lineas, line) }
func (e *espia) Close()                       { e.cerrado = true }

func redactor(valores map[string]string) *notify.RedactingObserver {
	r := notify.NewRedactingObserver(&espia{})
	r.UseVocabulary(vocabulario(valores))
	return r
}

// UMBRAL DE LONGITUD (§5.3, regla 1). Sin él, una variable con valor `"prod"`
// reemplazaría cada aparición de `prod` en toda la salida —dentro de una ruta,
// de un nombre de recurso, de una palabra— y el log dejaría de servir.
func TestRedactingObserver_ElUmbralDeLongitud(t *testing.T) {
	r := redactor(map[string]string{
		"environment": "prod",
		"connection":  "s3cr3t-token",
	})

	assert.Equal(t, "desplegando en prod",
		r.Redact("desplegando en prod"),
		"cuatro caracteres: por debajo del umbral no se toca")

	assert.Equal(t, "usando ${var.connection} para conectar",
		r.Redact("usando s3cr3t-token para conectar"),
		"doce caracteres: por encima del umbral sí")

	// Y el umbral es una longitud, no una lista: ocho justos entran.
	ocho := redactor(map[string]string{"artifact_name": "demo-app"})
	assert.Equal(t, "construido ${var.artifact_name}", ocho.Redact("construido demo-app"))
}

// ORDEN POR LONGITUD (§5.3, regla 2). Si un valor contiene a otro gana el largo;
// al revés quedaría un reemplazo parcial DENTRO de otro valor —ilegible y
// potencialmente revelador del resto—.
func TestRedactingObserver_ElValorMasLargoGana(t *testing.T) {
	r := redactor(map[string]string{
		"corto": "abcdefghij",
		"largo": "abcdefghijklmn",
	})

	assert.Equal(t, "token=${var.largo} fin", r.Redact("token=abcdefghijklmn fin"),
		"el largo se redacta entero, no como el corto seguido de basura")

	// Y el corto sigue redactándose cuando aparece solo.
	assert.Equal(t, "token=${var.corto} fin", r.Redact("token=abcdefghij fin"))
}

// LIMITACIÓN DOCUMENTADA, VERIFICADA COMO TAL (§5.3).
//
// La redacción sólo cubre apariciones LITERALES. Un valor codificado en base64,
// url-encoded o escapado dentro de un JSON no se detecta, y no se disfraza con
// heurísticas que darían falsa confianza.
//
// **Es la razón por la que el registro no guarda extractos** (§5.1): un
// mecanismo best-effort es aceptable para un canal rotable, no para uno
// permanente. Este test afirma un límite conocido, no un defecto.
func TestRedactingObserver_UnValorCodificadoNoSeDetecta(t *testing.T) {
	const secreto = "s3cr3t-token-largo"
	r := redactor(map[string]string{"connection": secreto})

	codificado := base64.StdEncoding.EncodeToString([]byte(secreto))
	linea := "payload=" + codificado

	assert.Equal(t, linea, r.Redact(linea),
		"la redacción es exacta, no heurística: ver spec 20 §5.3")
}

// Sin vocabulario el decorador es un paso a través. Es el estado en el que vive
// hasta que la ejecución le entrega su mapa acumulado, y ahí todavía no hay
// ningún valor que ocultar.
func TestRedactingObserver_SinVocabularioNoRedactaNada(t *testing.T) {
	destino := &espia{}
	r := notify.NewRedactingObserver(destino)

	r.Notify("exec-1", "una línea con demo-app dentro")

	require.Len(t, destino.lineas, 1)
	assert.Equal(t, "una línea con demo-app dentro", destino.lineas[0])
}

// El decorador se interpone SIN romper el contrato: lo que llega al destino es
// la línea redactada, y el cierre se propaga —sin eso, envolver al fan-out
// silenciaría el flush del observador de Supabase—.
func TestRedactingObserver_DecoraSinRomperElContrato(t *testing.T) {
	destino := &espia{}
	r := notify.NewRedactingObserver(destino)
	r.UseVocabulary(vocabulario(map[string]string{"connection": "s3cr3t-token"}))

	r.Notify("exec-1", "conectando con s3cr3t-token")
	r.Close()

	require.Len(t, destino.lineas, 1)
	assert.Equal(t, "conectando con ${var.connection}", destino.lineas[0])
	assert.True(t, destino.cerrado)
}

// Dos variables con el MISMO valor no pueden dar dos redacciones distintas según
// cómo el runtime recorra el mapa. El orden es total a propósito.
func TestRedactingObserver_ElResultadoNoDependeDelRecorridoDelMapa(t *testing.T) {
	valores := map[string]string{
		"artifact_name": "demo-app-largo",
		"project_name":  "demo-app-largo",
	}

	primera := redactor(valores).Redact("construido demo-app-largo")
	for i := 0; i < 20; i++ {
		assert.Equal(t, primera, redactor(valores).Redact("construido demo-app-largo"))
	}
	assert.Equal(t, "construido ${var.artifact_name}", primera)
}
