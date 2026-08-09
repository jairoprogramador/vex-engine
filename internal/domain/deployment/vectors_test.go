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
	"\n" + "\"01-test\"\x1e\"\"\x1e\"\"\x1e\"inst-v1:1111111111111111111111111111111111111111111111111111111111111111\"\x1e0" +
	"\n" + "\"02-supply\"\x1e\"project\"\x1e\"state_changed\\x1e\\\"pipeline\\\"\"\x1e\"inst-v1:2222222222222222222222222222222222222222222222222222222222222222\"\x1e2" +
	"\n" + "\"acr_name\"\x1e\"\\\"vexacr\\\"\"" +
	"\n" + "\"image\"\x1e\"step-output\\x1e\\\"01-test\\\"\\x1e\\\"image\\\"\""

const (
	// El material base, y el mismo material con `destination = "prod"`.
	vectorContentIDBase = "cnt-v1:5864ee248315bd98beb77736d34e06d67774159e680cdcd16e890f342cd30a5f"
	vectorContentIDProd = "cnt-v1:82a2a8b183e4a35d00f8497cd0e521b6bcd5a49b326701b48a151c368c44e0dc"

	// El primer despliegue del linaje —sin padre— y el segundo, que es el MISMO
	// contenido colgando del primero.
	vectorDeploymentIDRaiz    = "dep-v1:558d0c2d4548b3d22062804b671487f5bdc09c181e3cd91b4543d152d525e6fe"
	vectorDeploymentIDSegundo = "dep-v1:dabfd7ad662a1f2ca4a79f23ae0a4da5d7e24edf944ad2877af296a935d83d48"
)
