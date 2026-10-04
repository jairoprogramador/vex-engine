package dominio

import "time"

// RazonDeReferencia dice por qué se eligió una referencia — lista cerrada (DEC-06.6).
type RazonDeReferencia string

const (
	MismoAmbiente       RazonDeReferencia = "mismo_ambiente"
	ElegidaPorElUsuario RazonDeReferencia = "elegida_por_el_usuario"
)

func (r RazonDeReferencia) String() string { return string(r) }

// Referencia es un despliegue —nunca otra cosa, invariante «la referencia es siempre un despliegue»—, los
// ejes y las producidas de cada uno de sus pasos, y por qué se eligió. Solo se construye desde un
// despliegue: no hay otro constructor.
type Referencia struct {
	despliegue IdDespliegue
	ambiente   Ambiente
	instante   time.Time
	razon      RazonDeReferencia
	ejes       []EjesDeUnPaso
	producidas []ProducidasDeUnPaso
}

func NuevaReferencia(
	despliegue IdDespliegue, ambiente Ambiente, instante time.Time, razon RazonDeReferencia,
	ejes []EjesDeUnPaso, producidas []ProducidasDeUnPaso,
) Referencia {
	return Referencia{
		despliegue: despliegue, ambiente: ambiente, instante: instante, razon: razon,
		ejes: ejes, producidas: producidas,
	}
}

func (r Referencia) Despliegue() IdDespliegue         { return r.despliegue }
func (r Referencia) Ambiente() Ambiente               { return r.ambiente }
func (r Referencia) Instante() time.Time              { return r.instante }
func (r Referencia) Razon() RazonDeReferencia         { return r.razon }
func (r Referencia) Ejes() []EjesDeUnPaso             { return r.ejes }
func (r Referencia) Producidas() []ProducidasDeUnPaso { return r.producidas }
