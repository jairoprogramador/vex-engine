package state

import (
	"fmt"
	"time"
)

// RecordIDLen son los 26 caracteres de un ULID en su forma canónica.
const RecordIDLen = 26

// recordIDAlphabet es el alfabeto base32 de Crockford: 32 símbolos en orden
// ASCII ascendente, sin `I`, `L`, `O` ni `U`.
//
// Que el orden del alfabeto sea el del ASCII es lo que hace que el orden
// LEXICOGRÁFICO de dos identificadores sea su orden TEMPORAL — y de ahí sale
// «el último registro se obtiene sin leer ninguno»: basta con ordenar los
// nombres de archivo.
const recordIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// recordIDEntropyLen son los 10 bytes (80 bits) de aleatoriedad del ULID. Los
// otros 48 bits son el instante en milisegundos.
const recordIDEntropyLen = 10

// RecordID identifica un registro. Es un ULID, y no un hash, a propósito
// (spec 11 §5.3):
//
// Es CIRCUNSTANCIAL —depende de *cuándo* ocurrió, no de *qué* contiene—, igual
// que `event_id` en el registro de despliegue. No necesita ser reproducible
// entre organizaciones; necesita ser único y **ordenable en el tiempo**.
//
// Un hash de contenido sería justo lo contrario de lo que hace falta: dos
// ejecuciones que producen exactamente lo mismo son DOS hechos distintos, y con
// un hash colapsarían en uno — que es la sobrescritura que esta spec elimina.
type RecordID struct {
	text string
}

// NewRecordID compone el identificador a partir del instante y de 10 bytes de
// entropía.
//
// El instante lo pone el llamador y viene del puerto `shared.Clock` (spec 07):
// no hay `time.Now()` en el dominio. La entropía la pone la infraestructura
// —`crypto/rand`— por la misma razón, y es lo que permite fijar un ULID entero
// en un test.
func NewRecordID(at time.Time, entropy []byte) (RecordID, error) {
	if at.IsZero() {
		return RecordID{}, fmt.Errorf("state: un record_id sin instante no es ordenable")
	}
	if len(entropy) != recordIDEntropyLen {
		return RecordID{}, fmt.Errorf(
			"state: la entropía del record_id son %d bytes, no %d", recordIDEntropyLen, len(entropy))
	}

	milis := at.UTC().UnixMilli()
	if milis < 0 || milis >= 1<<48 {
		return RecordID{}, fmt.Errorf(
			"state: el instante %s no cabe en los 48 bits de un ULID", at.UTC().Format(time.RFC3339))
	}

	var bytes [16]byte
	for i := 0; i < 6; i++ {
		bytes[i] = byte(milis >> (40 - 8*i))
	}
	copy(bytes[6:], entropy)

	return RecordID{text: encodeCrockford(bytes)}, nil
}

// ParseRecordID lee la forma canónica. La necesitan el almacén —que reconstruye
// el identificador desde el nombre del archivo— y el índice.
func ParseRecordID(text string) (RecordID, error) {
	if len(text) != RecordIDLen {
		return RecordID{}, fmt.Errorf(
			"state: el record_id %q no tiene %d caracteres", text, RecordIDLen)
	}
	for i := 0; i < len(text); i++ {
		if !isCrockfordDigit(text[i]) {
			return RecordID{}, fmt.Errorf(
				"state: el record_id %q tiene un carácter fuera del alfabeto", text)
		}
	}
	return RecordID{text: text}, nil
}

func (r RecordID) String() string { return r.text }

// IsZero indica que no hay identificador.
func (r RecordID) IsZero() bool { return r.text == "" }

// RecordIDFactory produce identificadores nuevos. Es un puerto y no una función
// porque la aleatoriedad es infraestructura, exactamente igual que el reloj:
// `crypto/rand` dentro del dominio sería un `time.Now()` con otro nombre.
type RecordIDFactory interface {
	New(at time.Time) (RecordID, error)
}

// encodeCrockford escribe los 128 bits como 26 símbolos de 5 bits. Los 130 bits
// que ocupan 26 símbolos se completan con DOS bits de relleno a la izquierda,
// que es la definición canónica del ULID.
func encodeCrockford(bytes [16]byte) string {
	const paddingBits = 2

	out := make([]byte, RecordIDLen)
	for i := 0; i < RecordIDLen; i++ {
		var symbol byte
		for bit := 0; bit < 5; bit++ {
			position := i*5 + bit - paddingBits
			symbol <<= 1
			if position >= 0 && bytes[position/8]&(1<<(7-position%8)) != 0 {
				symbol |= 1
			}
		}
		out[i] = recordIDAlphabet[symbol]
	}
	return string(out)
}

func isCrockfordDigit(c byte) bool {
	for i := 0; i < len(recordIDAlphabet); i++ {
		if recordIDAlphabet[i] == c {
			return true
		}
	}
	return false
}
