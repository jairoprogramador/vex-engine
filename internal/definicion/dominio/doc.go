// Package dominio modela el pipeline declarado (lo que hay escrito en el repositorio de pipelinecode: config.yaml,
// environments.yaml, variables/ y steps/) y su comprobación: la única forma de obtener un PipelineComprobado es
// llamando a Comprobar, que valida un PipelineDeclarado y, si no hay fallos, devuelve la forma ya comprobada —
// nunca al revés. Un PipelineComprobado no tiene campos exportados que se puedan escribir desde fuera del
// paquete: lo que este paquete no comprobó, no existe como PipelineComprobado.
//
// # Invariantes del modelo
//
// Una variable pertenece a un ámbito, no a un paso ni a un comando (Ambito). El ámbito compartido (Compartido)
// se ve desde cualquier ambiente; el ámbito de un ambiente concreto solo se ve desde ese ambiente. Un paso sin
// scope propio se ejecuta en el ámbito del ambiente en que corre; un paso con scope: shared se ejecuta siempre
// en el ámbito compartido, y su resultado no varía entre ambientes.
//
// Un nombre de variable pertenece a un solo ámbito a la vez: no puede estar declarada dos veces en ámbitos que
// se vean entre sí (variableDePipelineEnComprobacion, choque), ni producirla dos comandos del mismo ámbito
// (variableDeComandoEnComprobacion). Toda variable usada tiene que verse desde donde se usa y, si es una salida,
// estar producida antes de usarse — el orden de los pasos y de los comandos dentro de un paso importa
// (posicion.antesDe).
//
// # Orden de validación
//
// Comprobar ejecuta ValidadorVersion primero: si el formato no es el que se lee, nada más se comprueba, porque
// todo lo demás fallaría por la misma causa. El resto de validadoresDelPipeline corre siempre completo, en este
// orden, y ninguno corta la comprobación de los siguientes: ValidadorArchivosIlegibles, ValidadorAmbientes,
// ValidadorPasos, ValidadorVariablesDeSalida, ValidadorVariablesDeclaradas y ValidadorUsoDeVariables. Ese orden es significativo:
// ValidadorVariablesDeclaradas necesita los ambientes ya comprobados (para saber si un directorio de variables/
// es de alguno), y ValidadorUsoDeVariables necesita las salidas y las variables ya indexadas (para saber qué se ve y qué
// falta por producirse).
//
// # Cómo agregar un nuevo Validador
//
// Un Validador es un struct sin estado que implementa Validar(*comprobacion) error: lee del *comprobacion lo que
// necesite (pipelineDeclarado y lo que ya comprobaron los validadores anteriores) y anota fallos con
// comp.falla(...). Nunca hace panic ni devuelve antes de anotar todo lo que puede anotar en esa pasada — un
// fallo no debe ocultar otros fallos no relacionados. Si la lógica crece, se descompone en un tipo
// «validadorXxxImpl» con *comprobacion como campo (patrón ya usado por validadorPasosImpl, validadorUsosImpl,
// etc.), no repitiendo *comprobacion como parámetro en cada método. Lo que un validador produce para que otro lo
// use (como pasosComprobados o variablesDeComandos) es un campo de resultadoComprobacion, no uno suelto en
// comprobacion: así el contexto no sigue creciendo con cada validador nuevo. Por último, se agrega a
// validadoresDelPipeline en el orden que corresponda, y se documenta aquí por qué va ahí.
package dominio
