package dominio

import "fmt"

// LimiteDeSalidaDeUnComando es lo más que se conserva de lo que escribe un comando: 16 MiB. Lo que pase de ahí
// no se guarda ni se usa para comprobar aserciones o capturar variables.
const LimiteDeSalidaDeUnComando = 16 << 20

// SalidaDeComando acumula lo que escribe un comando, hasta un límite. Pasado el límite sigue aceptando todo lo
// que le escriban —un comando que escribe sin parar no se queda bloqueado en su tubería— pero lo descarta, y el
// texto termina con una marca que dice que se truncó: un comando no puede agotar la memoria del motor.
//
// No es segura para varias goroutines: tiene un solo escritor, el comando. El valor cero no vale: se crea con
// NuevaSalidaDeComando.
type SalidaDeComando struct {
	limite      int
	texto       []byte
	descartados int
}

// NuevaSalidaDeComando crea la salida de un comando, con el límite del motor.
func NuevaSalidaDeComando() *SalidaDeComando {
	return NuevaSalidaDeComandoConLimite(LimiteDeSalidaDeUnComando)
}

// NuevaSalidaDeComandoConLimite cambia cuántos bytes se conservan.
func NuevaSalidaDeComandoConLimite(limite int) *SalidaDeComando {
	return &SalidaDeComando{limite: max(limite, 0)}
}

// Write conserva lo que quepa y descarta el resto. Nunca falla ni escribe de menos: quien escribe no se entera
// del límite.
func (s *SalidaDeComando) Write(p []byte) (int, error) {
	cabe := min(s.limite-len(s.texto), len(p))
	s.texto = append(s.texto, p[:cabe]...)
	s.descartados += len(p) - cabe
	return len(p), nil
}

// Truncada dice si el comando escribió más de lo que se conserva.
func (s *SalidaDeComando) Truncada() bool { return s.descartados > 0 }

// Texto es lo que se conservó, con la marca de truncado al final si hubo más.
func (s *SalidaDeComando) Texto() string {
	if !s.Truncada() {
		return string(s.texto)
	}
	return string(s.texto) + fmt.Sprintf("\n[SALIDA TRUNCADA: el comando escribió %d bytes más de los %d que se conservan]\n", s.descartados, s.limite)
}
