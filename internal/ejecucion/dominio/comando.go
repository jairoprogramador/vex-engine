package dominio

// ComandoDeclarado es lo que un comando declara: su línea, dónde corre, qué plantillas interpola y qué mira en
// su salida. Nunca lleva nada resuelto — ni el texto interpolado ni el valor de una variable de salida —, así
// que es también la materia prima del hash de instrucciones (DEC-08.7).
type ComandoDeclarado struct {
	nombre     string
	linea      string
	directorio string
	plantillas []string
	salidas    []VariableDeSalidaDeclarada
	aserciones []AsercionDeclarada
}

func NuevoComandoDeclarado(
	nombre, linea, directorio string, plantillas []string, salidas []VariableDeSalidaDeclarada,
	aserciones []AsercionDeclarada,
) (ComandoDeclarado, error) {
	if linea == "" {
		return ComandoDeclarado{}, invalido("%q: un comando declarado no puede tener la línea vacía", nombre)
	}
	return ComandoDeclarado{
		nombre: nombre, linea: linea, directorio: directorio,
		plantillas: plantillas, salidas: salidas, aserciones: aserciones,
	}, nil
}

func (c ComandoDeclarado) Nombre() string { return c.nombre }

func (c ComandoDeclarado) Linea() string { return c.linea }

// Directorio es el workdir del comando, relativo al directorio del paso.
func (c ComandoDeclarado) Directorio() string { return c.directorio }

// Plantillas son las rutas del material del paso que este comando interpola, relativas al directorio del paso.
func (c ComandoDeclarado) Plantillas() []string { return c.plantillas }

func (c ComandoDeclarado) Salidas() []VariableDeSalidaDeclarada { return c.salidas }

func (c ComandoDeclarado) Aserciones() []AsercionDeclarada { return c.aserciones }

// VariableDeSalidaDeclarada es un nombre y la expresión regular que dice qué forma tendrá su valor, tal como el
// paso la declara — sin resolver: el valor real solo existe después de ejecutar el comando.
type VariableDeSalidaDeclarada struct {
	nombre     string
	expresion  string
	compartida bool
}

func NuevaVariableDeSalidaDeclarada(nombre, expresion string, compartida bool) (VariableDeSalidaDeclarada, error) {
	if nombre == "" {
		return VariableDeSalidaDeclarada{}, invalido("una variable de salida declarada no puede tener el nombre vacío")
	}
	if expresion == "" {
		return VariableDeSalidaDeclarada{}, invalido("%q: una variable de salida declarada necesita su expresión", nombre)
	}
	return VariableDeSalidaDeclarada{nombre: nombre, expresion: expresion, compartida: compartida}, nil
}

func (v VariableDeSalidaDeclarada) Nombre() string { return v.nombre }

func (v VariableDeSalidaDeclarada) Expresion() string { return v.expresion }

func (v VariableDeSalidaDeclarada) Compartida() bool { return v.compartida }

// AsercionDeclarada es una expresión regular que la salida de un comando tiene que cumplir para que el comando
// se dé por bueno. No produce ninguna variable.
type AsercionDeclarada struct {
	expresion string
}

func NuevaAsercionDeclarada(expresion string) (AsercionDeclarada, error) {
	if expresion == "" {
		return AsercionDeclarada{}, invalido("una aserción declarada no puede tener la expresión vacía")
	}
	return AsercionDeclarada{expresion: expresion}, nil
}

func (a AsercionDeclarada) Expresion() string { return a.expresion }

// FicheroDeclarado es un fichero o un enlace del material de un paso, tal como Definición lo declara.
type FicheroDeclarado struct {
	ruta       string
	contenido  string
	ejecutable bool
	enlace     string
	plantilla  bool
}

func NuevoFicheroDeclarado(ruta, contenido string, ejecutable bool, enlace string, plantilla bool) (FicheroDeclarado, error) {
	if ruta == "" {
		return FicheroDeclarado{}, invalido("un fichero declarado no puede tener la ruta vacía")
	}
	return FicheroDeclarado{
		ruta: ruta, contenido: contenido, ejecutable: ejecutable, enlace: enlace, plantilla: plantilla,
	}, nil
}

func (f FicheroDeclarado) Ruta() string { return f.ruta }

func (f FicheroDeclarado) Contenido() string { return f.contenido }

func (f FicheroDeclarado) Ejecutable() bool { return f.ejecutable }

// Enlace es el destino si es un enlace, y vacío si no.
func (f FicheroDeclarado) Enlace() string { return f.enlace }

// Plantilla dice si algún comando lo interpola. Si no, se copia tal cual.
func (f FicheroDeclarado) Plantilla() bool { return f.plantilla }
