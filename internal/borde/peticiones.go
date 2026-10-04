package borde

// Las peticiones de las operaciones cuyo contexto no tiene una propia: el borde les pone la versión del
// lenguaje publicado (DEC-05.6) a lo que ese contexto recibe suelto.

// PeticionDeLanzamiento es lanzar un despliegue en un ambiente. Sin nombre, el nombre toma la versión.
type PeticionDeLanzamiento struct {
	Version    string
	Ambiente   string
	Despliegue string
	Nombre     string
}

// PeticionDeReserva es reservar un ambiente: desde ahora, el dueño del negocio decide sus lanzamientos.
type PeticionDeReserva struct {
	Version  string
	Ambiente string
}

// PeticionDeLiberacion es liberar un ambiente reservado.
type PeticionDeLiberacion struct {
	Version  string
	Ambiente string
}

// PeticionDeAbandono es dar por abandonado un intento sin desenlace.
type PeticionDeAbandono struct {
	Version string
	Intento string
}

// PeticionDeConsultaDeIntento es consultar un intento por su identidad.
type PeticionDeConsultaDeIntento struct {
	Version string
	Intento string
}

// PeticionDeIntentosDeUnAmbiente es consultar los intentos de un ambiente.
type PeticionDeIntentosDeUnAmbiente struct {
	Version  string
	Ambiente string
}

// PeticionDeDesplieguesDeUnAmbiente es consultar los despliegues de un ambiente.
type PeticionDeDesplieguesDeUnAmbiente struct {
	Version  string
	Ambiente string
}

// PeticionDeLogs es consultar la salida de los comandos de un intento. Sin Intento, es la del último que se
// abrió en cualquier ambiente. Resultado filtra por cómo terminó cada comando: "exitoso", "fallido", o vacío
// para todos.
type PeticionDeLogs struct {
	Version   string
	Intento   string
	Resultado string
}
