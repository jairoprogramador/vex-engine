package dominio

import "time"

// DespliegueDeDiagnostico es un despliegue en lo que este contexto necesita de él.
type DespliegueDeDiagnostico struct {
	Id       IdDespliegue
	Ambiente Ambiente
	Intento  IdIntento
	Instante time.Time
}

// CandidatosParaElegirReferencias son los despliegues ya resueltos entre los que elegir la referencia
// (DEC-06.6): el que el usuario eligió, si eligió alguno, o el último del mismo ambiente, si existe.
// Resolverlos es I/O y le toca a la aplicación/infraestructura (DEC-06.14): este servicio solo decide con
// lo que ya tiene delante.
type CandidatosParaElegirReferencias struct {
	ElegidaPorElUsuario    *DespliegueDeDiagnostico
	UltimoDelMismoAmbiente *DespliegueDeDiagnostico
}

// CandidataDeReferencia es un despliegue candidato a referencia, con la razón por la que se eligió.
type CandidataDeReferencia struct {
	Despliegue DespliegueDeDiagnostico
	Razon      RazonDeReferencia
}

// ElegirReferencias es el primer servicio de dominio (DEC-06.14): si el usuario eligió una, es la única
// que se usa; si no, la del mismo ambiente. Un despliegue del propio intento que se diagnostica no es
// historial previo: no se compara un intento consigo mismo, y se devuelve ninguna referencia.
func ElegirReferencias(c CandidatosParaElegirReferencias, intento IdIntento) []CandidataDeReferencia {
	switch {
	case c.ElegidaPorElUsuario != nil:
		return referenciaDistintaDe(intento, *c.ElegidaPorElUsuario, ElegidaPorElUsuario)
	case c.UltimoDelMismoAmbiente != nil:
		return referenciaDistintaDe(intento, *c.UltimoDelMismoAmbiente, MismoAmbiente)
	default:
		return nil
	}
}

func referenciaDistintaDe(intento IdIntento, despliegue DespliegueDeDiagnostico, razon RazonDeReferencia) []CandidataDeReferencia {
	if despliegue.Intento == intento {
		return nil
	}
	return []CandidataDeReferencia{{Despliegue: despliegue, Razon: razon}}
}
