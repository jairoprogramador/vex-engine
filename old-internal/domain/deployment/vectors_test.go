package deployment_test

// Los vectores congelados de `SPEC-CONTENT-v1.md` §7.
//
// Están escritos como LITERALES y no calculados: reproducirlos es lo que le
// permite al backend (spec 26) validar su reimplementación de la regla sin leer
// este código. Si un cambio los mueve, el cambio es una `cnt-v2` —o un bug—, y
// las dos cosas tienen que doler aquí antes que en un despliegue.
//
// La forma canónica se escribe línea a línea, con sus separadores explícitos,
// porque una regla que sólo se puede comprobar por su hash no se puede depurar
// cuando dos implementaciones discrepan.
const contenidoCanonicoEsperado = "vex-content/cnt-v1" +
	"\n" + "\"https://vex.test/acme/demo-app\"" +
	"\n" + "\"deploy\"" +
	"\n" + "\"sand\"" +
	"\n" + "\"v1:3333333333333333333333333333333333333333333333333333333333333333\"" +
	"\n" + "\"v1:4444444444444444444444444444444444444444444444444444444444444444\"" +
	"\n" + "\"2\"" +
	"\n" + "\"true\"" +
	"\n" + "2" +
	"\n" + "\"01-test\"\x1e\"\"\x1e\"\"\x1e\"pipe-v1:1111111111111111111111111111111111111111111111111111111111111111\"\x1e0" +
	"\n" + "\"02-supply\"\x1e\"project\"\x1e\"state_changed\\x1e\\\"pipeline\\\"\"\x1e\"pipe-v1:2222222222222222222222222222222222222222222222222222222222222222\"\x1e2" +
	"\n" + "\"acr_name\"\x1e\"\\\"vexacr\\\"\"" +
	"\n" + "\"image\"\x1e\"step-output\\x1e\\\"01-test\\\"\\x1e\\\"image\\\"\""

const (
	// El material base, y el mismo material con `destination = "prod"`.
	vectorContentIDBase = "cnt-v1:c1d0dd8880053c2c7829dd84bd9a24351c7f476aa718c77850180b7aede249e1"
	vectorContentIDProd = "cnt-v1:e11db527e332d2b191d95d1610d03463a95209382b31548e247406f7f9d7303d"

	// El primer despliegue del linaje —sin padre— y el segundo, que es el MISMO
	// contenido colgando del primero.
	vectorDeploymentIDRaiz    = "dep-v1:8cfd173977dceb8ac0708882cc7069eacd412f6cea266cc3e67dfd59b9892c00"
	vectorDeploymentIDSegundo = "dep-v1:15305711e5c548f5d875362e6fca527d9ca39396813e94d592db5b4393d8bc28"
)
