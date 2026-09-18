package dominio

// Pipeline es lo que Simulación necesita del pipeline comprobado que trae Definición: recortado respecto al
// de Ejecución porque Simulación no decide re-ejecuciones (sin Reglas ni EdadMaxima) y no corre comandos de
// verdad (sin Directorio de trabajo ni Aserciones, que validarían una salida real que aquí no existe).
type Pipeline struct {
	Ambientes []Ambiente // en su orden
	Pasos     []Paso     // en su orden
	// Variables son los literales del pipeline, cada uno con su ámbito.
	Variables []VariableDeclarada
}

// Ambiente es un ambiente en su lugar del orden. Valor es con el que se nombra en variables/, y el que
// identifica su ámbito.
type Ambiente struct {
	Nombre string
	Valor  string
}

// Paso se identifica por su nombre.
type Paso struct {
	Nombre   string
	Comandos []Comando
	// Material son los ficheros del paso; solo importan los marcados como plantilla.
	Material []Fichero
	// Compartido: ve solo lo compartido, no lo de un ambiente concreto (igual que en Definición y Ejecución).
	Compartido bool
}

type Comando struct {
	Nombre  string
	Linea   string
	Salidas []VariableDeSalida
}

// VariableDeSalida es un nombre y la expresión regular que dice qué forma tendrá su valor.
type VariableDeSalida struct {
	Nombre    string
	Expresion string
	// Compartida: lo que produce es del ámbito compartido. Si no, del ámbito del ambiente en simulación.
	Compartida bool
}

// Fichero es un fichero del material de un paso. Solo interesa su contenido si es una plantilla: los demás no
// se interpolan.
type Fichero struct {
	Ruta      string
	Contenido string
	Plantilla bool
}

// VariableDeclarada es un literal escrito en el pipeline, que puede usar otras variables. Pertenece a un
// ámbito, no a un paso.
type VariableDeclarada struct {
	Nombre string
	Ambito Ambito
	Valor  string
}
