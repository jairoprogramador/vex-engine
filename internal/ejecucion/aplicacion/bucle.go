package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// contextoDelIntento agrupa lo que no cambia entre pasos, para no arrastrar media docena de parámetros sueltos
// por cada función del bucle.
type contextoDelIntento struct {
	id                string
	ambiente          string
	ubicacion         dominio.Ubicacion
	fuenteDelPipeline string
	commitDelPipeline string
	hashDelCodigo     dominio.HashDeCodigo

	pasosPorNombre      map[string]dominio.PasoDeEjecucion
	estandarCompartidas map[string]string

	salida publicado.Salida
}

// recorrer es el bucle explícito de EJ-1/EJ-2 (docs/modelo/contextos/ejecucion.md, «Servicio de aplicación:
// intentar»): por cada paso, decide, hace y registra. Nil significa que el intento llegó a un desenlace —
// exitoso, fallido o cancelado (EJ-3) — y quien llama debe cerrarlo. Un error significa que algo impidió
// seguir sin que el intento llegara a un desenlace: el Historial no aceptó un registro (EJ-4) u otro puerto
// falló, y quien llama NO debe cerrarlo — queda sin desenlace, tal como pide EJ-4.
func (s *Servicio) recorrer(ctx context.Context, c contextoDelIntento, intento *dominio.IntentoEnCurso) error {
	for {
		paso, ok := intento.SiguientePaso()
		if !ok {
			return nil
		}
		if ctx.Err() != nil {
			intento.Cancelar()
			return nil
		}
		if err := s.darUnPaso(ctx, c, intento, paso); err != nil {
			return err
		}
	}
}

func (s *Servicio) darUnPaso(ctx context.Context, c contextoDelIntento, intento *dominio.IntentoEnCurso, paso dominio.PasoDelPipeline) error {
	pasoDeEjecucion := c.pasosPorNombre[paso.Nombre()]
	ambito, err := dominio.AmbitoDelPaso(paso, c.ambiente)
	if err != nil {
		return err
	}

	directorioDelPaso := s.d.EspacioDeTrabajo.DirectorioDelPaso(c.ubicacion, paso.Nombre())
	estandar := conElPaso(c.estandarCompartidas, paso.Nombre(), directorioDelPaso)
	if err := s.d.Variables.DeclararVariablesDeUnPaso(
		ctx, c.id, paso.Nombre(), ambito, c.fuenteDelPipeline, c.commitDelPipeline, estandar,
	); err != nil {
		return err
	}

	decision, ahora, err := s.decidirPaso(ctx, c.id, pasoDeEjecucion, ambito, c.hashDelCodigo)
	if err != nil {
		return err
	}

	if decision.SeReejecuta() {
		return s.reejecutarPaso(ctx, c, intento, paso, pasoDeEjecucion, ambito, ahora)
	}
	return s.dejarSinReejecutar(ctx, c, intento, paso, ambito, decision, ahora)
}

// reejecutarPaso es la mitad de EJ-1 que de verdad hace algo: deja un registro al empezar, antes del primer
// comando, y otro al terminar (DEC-09.7) — nada empieza sin que el anterior esté escrito porque IntentoEnCurso
// no da el siguiente paso hasta que Completar confirma este.
//
// EJ-3: si ejecutarPaso falla porque ctx se canceló, el comando falló por la cancelación, no por sí mismo —
// se registra como no exitoso igual, pero con un contexto que ya no depende del cancelado (context.WithoutCancel,
// el mismo patrón que retira el material al terminar), para que cerrar como cancelado no falle por la propia
// cancelación; y el intento se marca cancelado, que es lo que hace que gane sobre el fallo que ella misma
// provocó (DEC-09.2).
func (s *Servicio) reejecutarPaso(
	ctx context.Context, c contextoDelIntento, intento *dominio.IntentoEnCurso, paso dominio.PasoDelPipeline,
	pasoDeEjecucion dominio.PasoDeEjecucion, ambito dominio.Ambito, ahora dominio.RecursosDeUnPaso,
) error {
	if err := s.d.Historial.RegistrarComienzo(ctx, c.id, paso.Nombre(), ahora); err != nil {
		return err
	}

	exitoso, err := s.ejecutarPaso(ctx, c.id, c.ubicacion, pasoDeEjecucion, ambito, c.salida)
	ctxDelRegistro, cancelado := ctx, false
	if err != nil {
		if ctx.Err() == nil {
			return err
		}
		exitoso, cancelado = false, true
		ctxDelRegistro = context.WithoutCancel(ctx)
	}

	if err := s.d.Historial.RegistrarFinal(ctxDelRegistro, c.id, paso.Nombre(), exitoso, ahora); err != nil {
		return err
	}
	// Completar antes de Cancelar: el paso se completa como fallido primero, y solo entonces la cancelación
	// gana sobre ese fallo — Completar rechaza cualquier llamada una vez el intento ya está cancelado.
	if err := intento.Completar(paso.Nombre(), exitoso); err != nil {
		return err
	}
	if cancelado {
		intento.Cancelar()
	}
	return nil
}

func (s *Servicio) dejarSinReejecutar(
	ctx context.Context, c contextoDelIntento, intento *dominio.IntentoEnCurso, paso dominio.PasoDelPipeline,
	ambito dominio.Ambito, decision dominio.Decision, ahora dominio.RecursosDeUnPaso,
) error {
	if err := s.d.Variables.NoReejecutado(ctx, c.id, paso.Nombre(), ambito); err != nil {
		return err
	}
	if err := s.d.Historial.RegistrarNoReejecucion(ctx, c.id, paso.Nombre(), decision.Evidencia(), ahora); err != nil {
		return err
	}
	return intento.Completar(paso.Nombre(), true)
}
