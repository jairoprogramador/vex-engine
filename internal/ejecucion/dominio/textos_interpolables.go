package dominio

// TextosInterpolables son los textos de un paso donde ${var.<nombre>} se sustituye al ejecutar: la línea de
// cada comando y el contenido de cada plantilla. Dicen qué variables consume el paso, y por tanto cuáles
// importan para decidir si hay que re-ejecutarlo. Debe coincidir con lo que ejecutarPaso interpola.
func TextosInterpolables(comandos []ComandoDeclarado, material []FicheroDeclarado) []string {
	var textos []string
	for _, c := range comandos {
		textos = append(textos, c.Linea())
	}
	for _, f := range material {
		if f.Plantilla() {
			textos = append(textos, f.Contenido())
		}
	}
	return textos
}
