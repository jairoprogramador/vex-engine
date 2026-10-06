package dominio

import "strconv"

// Ambiente es la separación en la que ocurre un lanzamiento: dev, staging, producción.
type Ambiente struct{ valor string }

func NuevaAmbiente(valor string) (Ambiente, error) {
	if valor == "" {
		return Ambiente{}, invalido("un ambiente no puede tener el nombre vacío")
	}
	return Ambiente{valor: valor}, nil
}

func (a Ambiente) String() string { return a.valor }

// IdDespliegue identifica, ante Lanzamiento, un despliegue del Historial. Es opaco: Lanzamiento nunca lo
// interpreta, solo lo lleva.
type IdDespliegue struct{ valor string }

func NuevoIdDespliegue(valor string) (IdDespliegue, error) {
	if valor == "" {
		return IdDespliegue{}, invalido("un despliegue no puede tener la identidad vacía")
	}
	return IdDespliegue{valor: valor}, nil
}

func (d IdDespliegue) String() string { return d.valor }

// HashDelCodigo es el hash del código con el que se hizo un despliegue. Dos despliegues con el mismo hash
// son el mismo código, aunque estén en ambientes distintos (IT-10 DEC-10.8).
type HashDelCodigo struct{ valor string }

func NuevoHashDelCodigo(valor string) (HashDelCodigo, error) {
	if valor == "" {
		return HashDelCodigo{}, invalido("un hash del código no puede estar vacío")
	}
	return HashDelCodigo{valor: valor}, nil
}

func (h HashDelCodigo) String() string { return h.valor }

// Version es la etiqueta técnica del lanzamiento: un número por proyecto que crece de uno en uno con cada
// código que se lanza por primera vez (IT-10 DEC-10.8). Solo la calcula DecidirVersion.
type Version struct{ numero int }

func NuevaVersion(numero int) (Version, error) {
	if numero < 1 {
		return Version{}, invalido("una versión no puede ser menor que uno: %d", numero)
	}
	return Version{numero: numero}, nil
}

func (v Version) Numero() int { return v.numero }

func (v Version) String() string { return strconv.Itoa(v.numero) }

// Nombre es la etiqueta de negocio del lanzamiento. Vacío significa que quien lanza no lo puso, y la
// factoría toma el valor de la versión (IT-02 DEC-02.17).
type Nombre string

func (n Nombre) Vacia() bool { return n == "" }

func (n Nombre) String() string { return string(n) }
