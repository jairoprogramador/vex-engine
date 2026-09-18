package dominio

import "time"

// DespliegueDeDiagnostico es un despliegue en lo que este contexto necesita de él.
type DespliegueDeDiagnostico struct {
	Id       IdDespliegue
	Ambiente Ambiente
	Intento  IdIntento
	Instante time.Time
}

// CandidatosParaElegirReferencias son los despliegues ya resueltos entre los que elegir referencias
// (DEC-06.6, DEC-06.8): el que el usuario eligió, si eligió alguno, o los dos por defecto, cada uno si
// existe. Resolverlos es I/O y le toca a la aplicación/infraestructura (DEC-06.14): este servicio solo
// decide con lo que ya tiene delante.
type CandidatosParaElegirReferencias struct {
	ElegidaPorElUsuario                     *DespliegueDeDiagnostico
	UltimoDelMismoAmbiente                  *DespliegueDeDiagnostico
	UltimoDelAmbienteAnteriorConMismoCodigo *DespliegueDeDiagnostico
}

// CandidataDeReferencia es un despliegue candidato a referencia, con la razón por la que se eligió.
type CandidataDeReferencia struct {
	Despliegue DespliegueDeDiagnostico
	Razon      RazonDeReferencia
}

// ElegirReferencias es el primer servicio de dominio (DEC-06.14): si el usuario eligió una, es la única
// que se usa. Si no, se devuelven las que existan de las dos por defecto — cero, una o las dos.
func ElegirReferencias(c CandidatosParaElegirReferencias) []CandidataDeReferencia {
	if c.ElegidaPorElUsuario != nil {
		return []CandidataDeReferencia{{Despliegue: *c.ElegidaPorElUsuario, Razon: ElegidaPorElUsuario}}
	}
	var candidatas []CandidataDeReferencia
	if c.UltimoDelMismoAmbiente != nil {
		candidatas = append(candidatas, CandidataDeReferencia{Despliegue: *c.UltimoDelMismoAmbiente, Razon: MismoAmbiente})
	}
	if c.UltimoDelAmbienteAnteriorConMismoCodigo != nil {
		candidatas = append(
			candidatas,
			CandidataDeReferencia{Despliegue: *c.UltimoDelAmbienteAnteriorConMismoCodigo, Razon: AmbienteAnterior},
		)
	}
	return candidatas
}

// EncontrarAmbienteAnterior es el que precede a actual en el orden declarado, si actual no es el primero
// ni está fuera de ese orden.
func EncontrarAmbienteAnterior(orden []Ambiente, actual Ambiente) (Ambiente, bool) {
	for i, a := range orden {
		if a == actual {
			if i == 0 {
				return Ambiente{}, false
			}
			return orden[i-1], true
		}
	}
	return Ambiente{}, false
}
