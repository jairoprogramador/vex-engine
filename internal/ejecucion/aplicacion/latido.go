package aplicacion

import (
	"context"
	"time"
)

// latirMientras hace que el intento deje constancia de que su proceso sigue vivo, cada intervalo que publica el Historial, hasta
// que la función devuelta se llame. Es lo que permite a otro intento que encuentre el ambiente ocupado distinguir
// un proceso muerto de uno que corre un comando largo: el latido lo escribe esta rutina, no el comando.
//
// La función devuelta espera a que la rutina termine, para que ningún latido se escriba después de cerrar el
// intento. Sin intervalo no hace nada.
func (s *Servicio) latirMientras(ctx context.Context, intento string) (parar func()) {
	intervalo := s.d.Historial.IntervaloDeLatido()
	if intervalo <= 0 {
		return func() {}
	}
	// context.WithoutCancel: una cancelación no mata el proceso, que sigue vivo hasta cerrar el intento (EJ-3).
	ctxDelLatido, cancelar := context.WithCancel(context.WithoutCancel(ctx))
	terminada := make(chan struct{})

	go func() {
		defer close(terminada)
		s.latir(ctxDelLatido, intento)
		reloj := time.NewTicker(intervalo)
		defer reloj.Stop()
		for {
			select {
			case <-ctxDelLatido.Done():
				return
			case <-reloj.C:
				s.latir(ctxDelLatido, intento)
			}
		}
	}()

	return func() {
		cancelar()
		<-terminada
	}
}

// latir no devuelve el error: un latido que no se pudo escribir no detiene el intento. Si el almacén falla de
// verdad, los registros del intento fallarán también y EJ-4 se encarga; si el intento fue abandonado, lo mismo.
func (s *Servicio) latir(ctx context.Context, intento string) {
	_ = s.d.Historial.Latir(ctx, intento)
}
