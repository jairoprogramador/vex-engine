package dominio_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// Valores con la forma de los de verdad, pero inventados.
const (
	claveDeCuentaDePrueba = "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleXRlc3RrZXl0ZXN0a2V5MTIzNA=="
	firmaDeSASDePrueba    = "Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4%3D"
	tokenOpacoDePrueba    = "ya29.a0AfH6SMBx7Kq_ejemplo-de-token-opaco-0123456789"
	clavePrivadaDePrueba  = "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAejemplo\nlineaDos+/==\n-----END RSA PRIVATE KEY-----"
)

func TestRedactar_TapaLasFormasConocidas(t *testing.T) {
	casos := []struct {
		nombre, texto, esperado string
	}{
		{"un JWT, como siempre", "token " + jwtDePrueba + "\n", "token [REDACTADO]\n"},

		{"una clave privada en PEM", "clave:\n" + clavePrivadaDePrueba + "\nfin\n", "clave:\n[REDACTADO]\nfin\n"},
		{"una clave privada PKCS#8", "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----", "[REDACTADO]"},
		{"una clave privada de OpenSSH", "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----\n", "[REDACTADO]\n"},
		{"dos claves privadas, con texto en medio",
			clavePrivadaDePrueba + "\nentre las dos\n" + clavePrivadaDePrueba, "[REDACTADO]\nentre las dos\n[REDACTADO]"},
		{"una clave privada cortada antes del END se tapa hasta el final",
			"antes\n-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIejemplo\n", "antes\n[REDACTADO]"},

		{"la clave de una cadena de conexión de Storage",
			"DefaultEndpointsProtocol=https;AccountName=cuenta;AccountKey=" + claveDeCuentaDePrueba + ";EndpointSuffix=core.windows.net",
			"DefaultEndpointsProtocol=https;AccountName=cuenta;AccountKey=[REDACTADO];EndpointSuffix=core.windows.net"},
		{"AccountKey al final de la línea", "AccountKey=" + claveDeCuentaDePrueba + "\n", "AccountKey=[REDACTADO]\n"},
		{"accountkey en minúsculas", "accountkey=" + claveDeCuentaDePrueba, "accountkey=[REDACTADO]"},
		{"la clave de una cadena de Service Bus",
			"Endpoint=sb://x.servicebus.windows.net/;SharedAccessKeyName=Root;SharedAccessKey=" + claveDeCuentaDePrueba,
			"Endpoint=sb://x.servicebus.windows.net/;SharedAccessKeyName=Root;SharedAccessKey=[REDACTADO]"},

		{"una SAS en una cadena de conexión",
			"BlobEndpoint=https://c.blob.core.windows.net/;SharedAccessSignature=sv=2022-11-02&ss=b&sig=" + firmaDeSASDePrueba + ";otro=1",
			"BlobEndpoint=https://c.blob.core.windows.net/;SharedAccessSignature=[REDACTADO];otro=1"},
		{"la firma de una SAS en una URL",
			"https://c.blob.core.windows.net/x/y.jar?sv=2022-11-02&se=2026-01-01&sig=" + firmaDeSASDePrueba + "&sp=r",
			"https://c.blob.core.windows.net/x/y.jar?sv=2022-11-02&se=2026-01-01&sig=[REDACTADO]&sp=r"},
		{"la firma, primera de la consulta", "GET /x?sig=" + firmaDeSASDePrueba + " HTTP/1.1", "GET /x?sig=[REDACTADO] HTTP/1.1"},

		{"un token opaco tras Bearer", "Authorization: Bearer " + tokenOpacoDePrueba + "\r\n", "Authorization: Bearer [REDACTADO]\r\n"},
		{"bearer en minúsculas", `-H "authorization: bearer ` + tokenOpacoDePrueba + `"`, `-H "authorization: bearer [REDACTADO]"`},
		{"un JWT tras Bearer se tapa una sola vez", "Bearer " + jwtDePrueba, "Bearer [REDACTADO]"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			redactado := dominio.Redactar(c.texto)

			require.Equal(t, c.esperado, redactado)
			require.Equal(t, redactado, dominio.Redactar(redactado), "redactar dos veces no cambia nada")
		})
	}
}

func TestRedactar_NoTocaElTextoNormal(t *testing.T) {
	casos := map[string]string{
		"vacío":                               "",
		"la salida de terraform":              "Apply complete! Resources: 3 added, 0 changed, 0 destroyed.\n",
		"la salida de maven":                  "[INFO] BUILD SUCCESS\n[INFO] Total time:  12.345 s\n",
		"un certificado, que es público":      "-----BEGIN CERTIFICATE-----\nMIIDdzCCAl+gAwIBAgIE\n-----END CERTIFICATE-----\n",
		"una clave pública":                   "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----\n",
		"hablar de claves privadas":           "no se encontró la PRIVATE KEY; genere una con openssl\n",
		"AccountKey sin valor":                "falta AccountKey= en la cadena de conexión\n",
		"AccountKey con un marcador":          "AccountKey=<tu-clave>;EndpointSuffix=core.windows.net\n",
		"AccountKey con un valor corto":       "AccountKey=abc\n",
		"el nombre de la cuenta":              "AccountName=cuentadealmacenamientolarga01;\n",
		"el nombre de la clave compartida":    "SharedAccessKeyName=RootManageSharedAccessKey;\n",
		"SharedAccessSignature sin valor":     "use SharedAccessSignature= o AccountKey=\n",
		"sig= fuera de una consulta":          "config sig=0123456789abcdef0123456789abcdef\n",
		"un parámetro que termina en sig":     "https://x.invalid/a?design=0123456789abcdef0123456789abcdef\n",
		"una firma corta":                     "https://x.invalid/a?sig=abc&v=1\n",
		"Bearer en una frase":                 "Bearer authentication-is-required-for-this-endpoint\n",
		"Bearer y una palabra corta":          "401: se esperaba Authorization: Bearer token\n",
		"bearer token, la expresión":          "the bearer token was rejected by the server\n",
		"Bearer con marcador":                 "Authorization: Bearer <token>\n",
		"una imagen con su digest":            "registro.azurecr.io/pagos-api@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08\n",
		"un identificador de recurso":         "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-pagos\n",
		"una URL con consulta normal":         "https://portal.invalid/buscar?q=terraform&signature_help=1&pagina=2\n",
		"base64 suelto, sin prefijo conocido": "contenido: " + claveDeCuentaDePrueba + "\n",
	}
	for nombre, texto := range casos {
		t.Run(nombre, func(t *testing.T) {
			require.Equal(t, texto, dominio.Redactar(texto))
		})
	}
}

func TestRedactar_NoDejaNingunaParteDelSecreto(t *testing.T) {
	texto := strings.Join([]string{
		"AccountKey=" + claveDeCuentaDePrueba, "https://x.invalid/?sig=" + firmaDeSASDePrueba,
		"Bearer " + tokenOpacoDePrueba, clavePrivadaDePrueba,
	}, "\n")

	redactado := dominio.Redactar(texto)

	for _, secreto := range []string{claveDeCuentaDePrueba, firmaDeSASDePrueba, tokenOpacoDePrueba, "MIIEpAIBAAKCAQEAejemplo", "lineaDos"} {
		require.NotContains(t, redactado, secreto)
	}
}
