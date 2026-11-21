package dominio

// longitudMaximaDeUnMetadato es lo más largo que puede ser un metadato del proyecto o el valor de un ambiente.
const longitudMaximaDeUnMetadato = 128

// ComprobarMetadato comprueba un dato del proyecto que da quien invoca y que acaba, tal cual, dentro de la línea
// de un comando (las variables estándar project_name, project_team, environment…): hasta 128 bytes de letras
// ASCII, números y . _ -, empezando por letra o número. Así no puede traer espacios, comillas ni nada que el
// shell interprete, ni parecer una opción (-x) o una ruta (..). Vacío vale: el metadato es opcional.
//
// El error dice el campo, nunca el valor: puede ser largo, y es justo lo que no se quiere repetir.
func ComprobarMetadato(campo, valor string) error {
	if valor == "" {
		return nil
	}
	if len(valor) > longitudMaximaDeUnMetadato {
		return invalidoElCampo(campo, "", "%s no puede pasar de %d bytes", campo, longitudMaximaDeUnMetadato)
	}
	for i := 0; i < len(valor); i++ {
		c := valor[i]
		letraONumero := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		if !letraONumero && (i == 0 || (c != '.' && c != '_' && c != '-')) {
			return invalidoElCampo(campo, "", "%s solo admite letras sin tilde, números y . _ -, y empieza por letra o número", campo)
		}
	}
	return nil
}
