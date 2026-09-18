package aplicacion

import (
	"context"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Consultar es recorrer (IT-05 DEC-05.5). Ninguna consulta devuelve un valor, salvo ValoresDeUnPaso, que es
// de la relación reservada.

// UltimaVezDeUnPaso: en el ámbito de un ambiente, recorre sus intentos en su orden; en el compartido, todos.
func (s *Servicio) UltimaVezDeUnPaso(
	ctx context.Context, paso string, ambito publicado.Ambito,
) (publicado.RegistroDePaso, bool, error) {
	p := dominio.PasoEnSuAmbito{
		Paso:   dominio.NombrePaso(paso),
		Ambito: dominio.Ambito{Compartido: ambito.Compartido, Ambiente: dominio.Ambiente(ambito.Ambiente)},
	}
	var (
		intentos []*dominio.Intento
		err      error
	)
	if p.Ambito.Compartido {
		intentos, err = s.d.Intentos.Recorrer(ctx)
	} else {
		intentos, err = s.intentosDeUnAmbiente(ctx, p.Ambito.Ambiente)
	}
	if err != nil {
		return publicado.RegistroDePaso{}, false, traducir(err)
	}
	r, de, ok := dominio.UltimaVezDeUnPaso(p, intentos)
	if !ok {
		return publicado.RegistroDePaso{}, false, nil
	}
	return registroDePasoAPublicado(de, r), true, nil
}

func (s *Servicio) DespliegueYSuIntento(
	ctx context.Context, id string,
) (publicado.Despliegue, publicado.Intento, error) {
	despliegue, err := s.buscarDespliegue(ctx, dominio.IdDespliegue(id))
	if err != nil {
		return publicado.Despliegue{}, publicado.Intento{}, traducir(err)
	}
	intento, err := s.Intento(ctx, string(despliegue.Intento()))
	if err != nil {
		return publicado.Despliegue{}, publicado.Intento{}, err
	}
	return despliegueAPublicado(despliegue), intento, nil
}

func (s *Servicio) VariablesDeUnPaso(ctx context.Context, intento, paso string) ([]publicado.Variable, error) {
	i, err := s.leerIntento(ctx, dominio.IdIntento(intento))
	if err != nil {
		return nil, traducir(err)
	}
	var variables []publicado.Variable
	for _, r := range i.Variables(dominio.NombrePaso(paso)) {
		variables = append(variables, publicado.Variable{
			Intento: intento, Paso: paso, Nombre: string(r.Variable), Instante: r.Instante,
			Contenido: contenidoAPublicado(r.Contenido),
		})
	}
	return variables, nil
}

// ValoresDeUnPaso es de la relación reservada.
func (s *Servicio) ValoresDeUnPaso(ctx context.Context, intento, paso string) (map[string]string, error) {
	i, err := s.leerIntento(ctx, dominio.IdIntento(intento))
	if err != nil {
		return nil, traducir(err)
	}
	valores := map[string]string{}
	for nombre, valor := range i.Valores(dominio.NombrePaso(paso)) {
		valores[string(nombre)] = valor
	}
	return valores, nil
}

func (s *Servicio) UltimoDespliegue(ctx context.Context, ambiente string) (publicado.Despliegue, bool, error) {
	despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
	if err != nil {
		return publicado.Despliegue{}, false, traducir(err)
	}
	ultimo, ok := despliegues.Ultimo()
	if !ok {
		return publicado.Despliegue{}, false, nil
	}
	return despliegueAPublicado(ultimo), true, nil
}

func (s *Servicio) UltimaReserva(ctx context.Context, ambiente string) (publicado.Reserva, bool, error) {
	reservas, err := s.d.Reservas.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
	if err != nil {
		return publicado.Reserva{}, false, traducir(err)
	}
	ultima, ok := reservas.Ultima()
	if !ok {
		return publicado.Reserva{}, false, nil
	}
	return publicado.Reserva{Ambiente: ambiente, Reservado: ultima.Reservado, Instante: ultima.Instante}, true, nil
}

func (s *Servicio) UltimoLanzamiento(ctx context.Context, ambiente string) (publicado.Lanzamiento, bool, error) {
	lanzamientos, err := s.d.Lanzamientos.Todos(ctx)
	if err != nil {
		return publicado.Lanzamiento{}, false, traducir(err)
	}
	ultimo, ok := lanzamientos.UltimoDeUnAmbiente(dominio.Ambiente(ambiente))
	if !ok {
		return publicado.Lanzamiento{}, false, nil
	}
	return lanzamientoAPublicado(ultimo), true, nil
}

// Intento es uno que se abrió. Un intento sin apertura no se consulta: nunca empezó.
func (s *Servicio) Intento(ctx context.Context, id string) (publicado.Intento, error) {
	intento, err := s.d.Intentos.Intento(ctx, dominio.IdIntento(id))
	if err != nil {
		return publicado.Intento{}, traducir(err)
	}
	resultado, ok := intentoAPublicado(intento)
	if !ok {
		return publicado.Intento{}, traducir(noExiste(nil, "el intento %s", id))
	}
	return resultado, nil
}

// IntentosDeUnAmbiente, en el orden del ambiente.
func (s *Servicio) IntentosDeUnAmbiente(ctx context.Context, ambiente string) ([]publicado.Intento, error) {
	intentos, err := s.intentosDeUnAmbiente(ctx, dominio.Ambiente(ambiente))
	if err != nil {
		return nil, traducir(err)
	}
	var resultado []publicado.Intento
	for _, intento := range intentos {
		if uno, ok := intentoAPublicado(intento); ok {
			resultado = append(resultado, uno)
		}
	}
	return resultado, nil
}

func (s *Servicio) DesplieguesDeUnAmbiente(ctx context.Context, ambiente string) ([]publicado.Despliegue, error) {
	despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
	if err != nil {
		return nil, traducir(err)
	}
	var resultado []publicado.Despliegue
	for _, d := range despliegues.Todos() {
		resultado = append(resultado, despliegueAPublicado(d))
	}
	return resultado, nil
}

func (s *Servicio) UltimoDespliegueConHashDelCodigo(
	ctx context.Context, ambiente, hash string,
) (publicado.Despliegue, bool, error) {
	despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
	if err != nil {
		return publicado.Despliegue{}, false, traducir(err)
	}
	todos := despliegues.Todos()
	for k := len(todos) - 1; k >= 0; k-- {
		intento, err := s.d.Intentos.Intento(ctx, todos[k].Intento())
		if err != nil {
			return publicado.Despliegue{}, false, traducir(err)
		}
		if a, ok := intento.Apertura(); ok && a.HashDelCodigo == dominio.HashDelCodigo(hash) {
			return despliegueAPublicado(todos[k]), true, nil
		}
	}
	return publicado.Despliegue{}, false, nil
}

func (s *Servicio) CantidadDeIntentos(ctx context.Context, despliegue, intento string) (int, error) {
	d, err := s.buscarDespliegue(ctx, dominio.IdDespliegue(despliegue))
	if err != nil {
		return 0, traducir(err)
	}
	ocupacion, err := s.d.Ocupaciones.DeUnAmbiente(ctx, d.Ambiente())
	if err != nil {
		return 0, traducir(err)
	}
	ids := ocupacion.Intentos()
	desde := slices.Index(ids, d.Intento())
	hasta := slices.Index(ids, dominio.IdIntento(intento))
	if desde < 0 || hasta <= desde {
		return 0, traducir(noExiste(nil, "el intento %s posterior al despliegue %s en %q", intento, despliegue, d.Ambiente()))
	}
	cantidad := 0
	for _, id := range ids[desde+1 : hasta+1] {
		i, err := s.d.Intentos.Intento(ctx, id)
		if err != nil {
			return 0, traducir(err)
		}
		if _, abierto := i.Apertura(); abierto {
			cantidad++
		}
	}
	return cantidad, nil
}

// intentosDeUnAmbiente, en el orden de sus ocupaciones.
func (s *Servicio) intentosDeUnAmbiente(ctx context.Context, ambiente dominio.Ambiente) ([]*dominio.Intento, error) {
	ocupacion, err := s.d.Ocupaciones.DeUnAmbiente(ctx, ambiente)
	if err != nil {
		return nil, err
	}
	var intentos []*dominio.Intento
	for _, id := range ocupacion.Intentos() {
		intento, err := s.d.Intentos.Intento(ctx, id)
		if err != nil {
			return nil, err
		}
		intentos = append(intentos, intento)
	}
	return intentos, nil
}

func (s *Servicio) buscarDespliegue(ctx context.Context, id dominio.IdDespliegue) (dominio.Despliegue, error) {
	todos, err := s.d.Despliegues.Recorrer(ctx)
	if err != nil {
		return dominio.Despliegue{}, err
	}
	for _, despliegues := range todos {
		if d, ok := despliegues.Buscar(id); ok {
			return d, nil
		}
	}
	return dominio.Despliegue{}, noExiste(nil, "el despliegue %s", id)
}
