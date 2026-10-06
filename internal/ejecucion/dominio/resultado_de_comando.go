package dominio

// VariableProducida es una variable de salida con el valor que capturó su expresión al ejecutar el comando.
type VariableProducida struct {
	Nombre     string
	Valor      string
	Compartida bool
}

// ResultadoDeUnComando es si el comando terminó bien —su código de salida es cero y su salida cumple todas
// sus aserciones— y lo que produjo. Solo tiene sentido si de verdad se ejecutó: no es la decisión de si había
// que ejecutarlo.
type ResultadoDeUnComando struct {
	Exitoso    bool
	Producidas []VariableProducida
}
