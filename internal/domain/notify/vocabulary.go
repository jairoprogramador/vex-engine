package notify

// Vocabulary es lo que el proceso SABE que son valores de variable.
//
// # Por qué el puerto está aquí y no en `command`
//
// Porque quien lo consume es la FRONTERA DE SALIDA (spec 20 §5.1'): el dominio
// trabaja con valores reales —interpolar `${var.x}` con un asterisco no
// desplegaría nada— y lo que tiene que estar redactado es lo que sale del
// proceso. El mapa acumulado lo implementa porque es quien tiene el dato, no
// porque le importe para qué se usa.
//
// Se consulta EN CADA LÍNEA y no una vez al empezar, y no es un detalle: el mapa
// crece mientras la ejecución corre —cada `outputs` extraído entra en él— así
// que una copia tomada al arrancar redactaría exactamente los valores que ya
// estaban y ninguno de los que la ejecución produjo.
type Vocabulary interface {
	// Values son los pares (nombre, valor) conocidos en este instante.
	Values() map[string]string
}

// VocabularyAware lo implementa el observador que necesita conocer los valores
// para NO imprimirlos.
//
// Es opcional a propósito: `LogObserver` sigue siendo lo que era —una línea y a
// dónde va— y ni `StdoutLogObserver` ni `SupabaseLogObserver` tienen por qué
// saber de esto. Quien redacta es un decorador que envuelve a los dos, y esta
// interfaz es cómo el dominio le entrega lo único que no puede deducir.
type VocabularyAware interface {
	UseVocabulary(vocabulary Vocabulary)
}
