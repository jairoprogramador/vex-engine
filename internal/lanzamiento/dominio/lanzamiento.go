package dominio

// Lanzamiento es la decisión de hacer visible un despliegue, con su versión y su nombre. No es un agregado:
// Lanzamiento no guarda nada propio, sus registros son del Historial (IT-10 DEC-10.5).
type Lanzamiento struct {
	despliegue IdDespliegue
	hash       HashDelCodigo
	version    Version
	nombre     Nombre
}

// NuevoLanzamiento construye un lanzamiento. Si no tiene nombre, toma el valor de la versión
// (IT-02 DEC-02.17).
func NuevoLanzamiento(despliegue IdDespliegue, hash HashDelCodigo, version Version, nombre Nombre) Lanzamiento {
	if nombre.Vacia() {
		nombre = Nombre(version.String())
	}
	return Lanzamiento{despliegue: despliegue, hash: hash, version: version, nombre: nombre}
}

func (l Lanzamiento) Despliegue() IdDespliegue     { return l.despliegue }
func (l Lanzamiento) HashDelCodigo() HashDelCodigo { return l.hash }
func (l Lanzamiento) Version() Version             { return l.version }
func (l Lanzamiento) Nombre() Nombre               { return l.nombre }
