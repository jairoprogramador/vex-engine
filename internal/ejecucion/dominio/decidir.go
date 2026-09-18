package dominio

import "time"

// Evidencia es el enlace al registro con el que se hizo de verdad un paso que no se re-ejecuta — el mismo
// concepto que historial/publicado.Evidencia, en el lenguaje de este dominio.
type Evidencia struct {
	Intento string
	Paso    string
}

// UltimaVezDeUnPaso es lo que la aplicación trae del Historial sobre la última vez que un paso se registró bajo
// su ámbito, ya traducido a este dominio. Válida es true cuando ese registro es un final exitoso, o una
// no-reejecución — que nunca se escribe si no apunta a uno (DEC-09.7): las dos encadenan la misma validez, así
// que aquí no hace falta distinguirlas. Recursos, Edad y Evidencia solo tienen sentido cuando Válida es true.
type UltimaVezDeUnPaso struct {
	Hay       bool
	Valida    bool
	Recursos  RecursosDeUnPaso
	Edad      time.Duration
	Evidencia Evidencia
}

// TipoDeDecision es si un paso se re-ejecuta o no.
type TipoDeDecision int

const (
	SeReejecuta TipoDeDecision = iota + 1
	NoSeReejecuta
)

// Decision es el resultado de decidir un paso: si se re-ejecuta, con la razón de por qué; si no, con la
// evidencia del registro con el que de verdad se hizo.
type Decision struct {
	tipo      TipoDeDecision
	razon     string
	evidencia Evidencia
}

func decisionDeReejecutar(razon string) Decision {
	return Decision{tipo: SeReejecuta, razon: razon}
}

func decisionDeNoReejecutar(evidencia Evidencia) Decision {
	return Decision{tipo: NoSeReejecuta, evidencia: evidencia}
}

func (d Decision) SeReejecuta() bool { return d.tipo == SeReejecuta }

func (d Decision) Razon() string { return d.razon }

// Evidencia solo tiene sentido cuando SeReejecuta() es false.
func (d Decision) Evidencia() Evidencia { return d.evidencia }

// DecidirPaso recibe la regla del paso, sus recursos de ahora y su última vez, y devuelve una decisión. No lee
// nada, no calcula ningún hash y no escribe nada (DEC-09.3): cada dato ya viene resuelto por quien la llama.
//
// Un conjunto de reglas vacío (regla no mira nada, sin edad máxima) siempre re-ejecuta: un OR sobre cero
// condiciones no puede convertirse en "nunca re-ejecutar" por accidente — sería un paso que deja de vigilarse
// solo porque nadie marcó ninguna regla.
func DecidirPaso(regla Regla, ahora RecursosDeUnPaso, ultimaVez UltimaVezDeUnPaso) Decision {
	if !ultimaVez.Hay || !ultimaVez.Valida {
		return decisionDeReejecutar("no hay una última vez válida de la que partir")
	}

	if regla.MiraCodigo() && ahora.HashDelCodigo() != ultimaVez.Recursos.HashDelCodigo() {
		return decisionDeReejecutar("el código cambió")
	}
	if regla.MiraInstrucciones() && ahora.HashDeInstrucciones() != ultimaVez.Recursos.HashDeInstrucciones() {
		return decisionDeReejecutar("las instrucciones cambiaron")
	}
	if regla.MiraVariables() && ahora.CambiaronVariables() {
		return decisionDeReejecutar("las variables que ve cambiaron")
	}
	if edadMaxima := regla.EdadMaxima(); edadMaxima != 0 && ultimaVez.Edad > edadMaxima {
		return decisionDeReejecutar("la última vez ya caducó")
	}
	if !regla.MiraCodigo() && !regla.MiraInstrucciones() && !regla.MiraVariables() && regla.EdadMaxima() == 0 {
		return decisionDeReejecutar("la regla no mira nada")
	}

	return decisionDeNoReejecutar(ultimaVez.Evidencia)
}
