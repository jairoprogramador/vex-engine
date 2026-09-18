package infraestructura

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

// La forma de los registros en el almacén. Cada uno lleva su formato: el almacén es la memoria del motor, y
// un lector tiene que poder decir que no sabe leer un registro en vez de leerlo mal.
const formatoDeRegistro = 1

type contenidoJSON struct {
	Contexto string `json:"contexto"`
	Datos    []byte `json:"datos,omitempty"`
}

type pasoDeclaradoJSON struct {
	Nombre     string `json:"nombre"`
	Compartido bool   `json:"compartido,omitempty"`
}

type aperturaJSON struct {
	Ambiente      string              `json:"ambiente"`
	Solicitante   string              `json:"solicitante"`
	Pasos         []pasoDeclaradoJSON `json:"pasos"`
	HastaPaso     string              `json:"hasta_paso"`
	ConCommits    bool                `json:"con_commits"`
	HashDelCodigo string              `json:"hash_del_codigo"`
}

type evidenciaJSON struct {
	Intento string `json:"intento"`
	Paso    string `json:"paso"`
}

type cierreJSON struct {
	Estado  string `json:"estado"`
	Destino string `json:"destino,omitempty"`
}

type registroDeIntentoJSON struct {
	Formato       int            `json:"formato"`
	Tipo          string         `json:"tipo"`
	Instante      time.Time      `json:"instante"`
	Apertura      *aperturaJSON  `json:"apertura,omitempty"`
	Paso          string         `json:"paso,omitempty"`
	Exitoso       bool           `json:"exitoso,omitempty"`
	Evidencia     *evidenciaJSON `json:"evidencia,omitempty"`
	Variable      string         `json:"variable,omitempty"`
	ValorOfuscado string         `json:"valor_ofuscado,omitempty"`
	Cierre        *cierreJSON    `json:"cierre,omitempty"`
	Contenido     *contenidoJSON `json:"contenido,omitempty"`
}

type despliegueJSON struct {
	Formato  int       `json:"formato"`
	Id       string    `json:"id"`
	Ambiente string    `json:"ambiente"`
	Intento  string    `json:"intento"`
	Padre    string    `json:"padre,omitempty"`
	Instante time.Time `json:"instante"`
}

type ocupacionJSON struct {
	Formato  int       `json:"formato"`
	Intento  string    `json:"intento"`
	Instante time.Time `json:"instante"`
}

type lanzamientoJSON struct {
	Formato    int            `json:"formato"`
	Id         string         `json:"id"`
	Ambiente   string         `json:"ambiente"`
	Despliegue string         `json:"despliegue"`
	Instante   time.Time      `json:"instante"`
	Contenido  *contenidoJSON `json:"contenido,omitempty"`
}

type reservaJSON struct {
	Formato   int       `json:"formato"`
	Reservado bool      `json:"reservado"`
	Instante  time.Time `json:"instante"`
}

var nombresDeTipo = map[dominio.TipoDeRegistro]string{
	dominio.TipoApertura:      "apertura",
	dominio.TipoComienzo:      "comienzo",
	dominio.TipoFinal:         "final",
	dominio.TipoNoReejecucion: "no_reejecucion",
	dominio.TipoVariable:      "variable",
	dominio.TipoValor:         "valor",
	dominio.TipoCierre:        "cierre",
	dominio.TipoAbandono:      "abandono",
}

var estados = map[string]dominio.Estado{
	dominio.Exitoso.String():   dominio.Exitoso,
	dominio.Fallido.String():   dominio.Fallido,
	dominio.Cancelado.String(): dominio.Cancelado,
}

func codificarRegistroDeIntento(r dominio.RegistroDeIntento) ([]byte, error) {
	tipo, ok := nombresDeTipo[r.Tipo]
	if !ok {
		return nil, fmt.Errorf("codificar: tipo de registro desconocido: %d", r.Tipo)
	}
	j := registroDeIntentoJSON{
		Formato:   formatoDeRegistro,
		Tipo:      tipo,
		Instante:  r.Instante,
		Paso:      string(r.Paso),
		Exitoso:   r.Exitoso,
		Variable:  string(r.Variable),
		Contenido: contenidoAJSON(r.Contenido),
	}
	switch r.Tipo {
	case dominio.TipoApertura:
		a := r.Apertura
		j.Apertura = &aperturaJSON{
			Ambiente: string(a.Ambiente), Solicitante: string(a.Solicitante), HastaPaso: string(a.HastaPaso),
			ConCommits: a.ConCommits, HashDelCodigo: string(a.HashDelCodigo),
		}
		for _, p := range a.Pasos {
			j.Apertura.Pasos = append(j.Apertura.Pasos, pasoDeclaradoJSON{Nombre: string(p.Nombre), Compartido: p.Compartido})
		}
	case dominio.TipoNoReejecucion:
		j.Evidencia = &evidenciaJSON{Intento: string(r.Evidencia.Intento), Paso: string(r.Evidencia.Paso)}
	case dominio.TipoValor:
		j.ValorOfuscado = ofuscar(r.Valor)
	case dominio.TipoCierre:
		j.Cierre = &cierreJSON{Estado: r.Cierre.Estado.String(), Destino: string(r.Cierre.Destino)}
	}
	return json.Marshal(j)
}

func decodificarRegistroDeIntento(datos []byte) (dominio.RegistroDeIntento, error) {
	var j registroDeIntentoJSON
	if err := decodificar(datos, &j, &j.Formato); err != nil {
		return dominio.RegistroDeIntento{}, err
	}
	r := dominio.RegistroDeIntento{
		Instante:  j.Instante,
		Paso:      dominio.NombrePaso(j.Paso),
		Exitoso:   j.Exitoso,
		Variable:  dominio.NombreVariable(j.Variable),
		Contenido: contenidoDeJSON(j.Contenido),
	}
	for tipo, nombre := range nombresDeTipo {
		if nombre == j.Tipo {
			r.Tipo = tipo
		}
	}
	if r.Tipo == 0 {
		return dominio.RegistroDeIntento{}, fmt.Errorf("decodificar: tipo de registro desconocido: %q", j.Tipo)
	}
	if a := j.Apertura; a != nil {
		r.Apertura = dominio.Apertura{
			Ambiente: dominio.Ambiente(a.Ambiente), Solicitante: dominio.Solicitante(a.Solicitante),
			HastaPaso: dominio.NombrePaso(a.HastaPaso), ConCommits: a.ConCommits,
			HashDelCodigo: dominio.HashDelCodigo(a.HashDelCodigo),
		}
		for _, p := range a.Pasos {
			r.Apertura.Pasos = append(r.Apertura.Pasos, dominio.PasoDeclarado{
				Nombre: dominio.NombrePaso(p.Nombre), Compartido: p.Compartido,
			})
		}
	}
	if e := j.Evidencia; e != nil {
		r.Evidencia = dominio.Evidencia{Intento: dominio.IdIntento(e.Intento), Paso: dominio.NombrePaso(e.Paso)}
	}
	if r.Tipo == dominio.TipoValor {
		valor, err := desofuscar(j.ValorOfuscado)
		if err != nil {
			return dominio.RegistroDeIntento{}, err
		}
		r.Valor = valor
	}
	if c := j.Cierre; c != nil {
		estado, ok := estados[c.Estado]
		if !ok {
			return dominio.RegistroDeIntento{}, fmt.Errorf("decodificar: estado desconocido: %q", c.Estado)
		}
		r.Cierre = dominio.Cierre{Estado: estado, Destino: dominio.IdDespliegue(c.Destino)}
	}
	return r, nil
}

func codificarDespliegue(d dominio.Despliegue) ([]byte, error) {
	return json.Marshal(despliegueJSON{
		Formato: formatoDeRegistro, Id: string(d.Id()), Ambiente: string(d.Ambiente()),
		Intento: string(d.Intento()), Padre: string(d.Padre()), Instante: d.Instante(),
	})
}

func decodificarDespliegue(datos []byte) (dominio.Despliegue, error) {
	var j despliegueJSON
	if err := decodificar(datos, &j, &j.Formato); err != nil {
		return dominio.Despliegue{}, err
	}
	return dominio.ReconstituirDespliegue(
		dominio.IdDespliegue(j.Id), dominio.Ambiente(j.Ambiente), dominio.IdIntento(j.Intento),
		dominio.IdDespliegue(j.Padre), j.Instante,
	), nil
}

func codificarOcupacion(r dominio.RegistroDeOcupacion) ([]byte, error) {
	return json.Marshal(ocupacionJSON{Formato: formatoDeRegistro, Intento: string(r.Intento), Instante: r.Instante})
}

func decodificarOcupacion(datos []byte) (dominio.RegistroDeOcupacion, error) {
	var j ocupacionJSON
	if err := decodificar(datos, &j, &j.Formato); err != nil {
		return dominio.RegistroDeOcupacion{}, err
	}
	return dominio.RegistroDeOcupacion{Intento: dominio.IdIntento(j.Intento), Instante: j.Instante}, nil
}

func codificarLanzamiento(l dominio.Lanzamiento) ([]byte, error) {
	return json.Marshal(lanzamientoJSON{
		Formato: formatoDeRegistro, Id: string(l.Id()), Ambiente: string(l.Ambiente()),
		Despliegue: string(l.Despliegue()), Instante: l.Instante(), Contenido: contenidoAJSON(l.Contenido()),
	})
}

func decodificarLanzamiento(datos []byte) (dominio.Lanzamiento, error) {
	var j lanzamientoJSON
	if err := decodificar(datos, &j, &j.Formato); err != nil {
		return dominio.Lanzamiento{}, err
	}
	return dominio.ReconstituirLanzamiento(
		dominio.IdLanzamiento(j.Id), dominio.Ambiente(j.Ambiente), dominio.IdDespliegue(j.Despliegue),
		j.Instante, contenidoDeJSON(j.Contenido),
	), nil
}

func codificarReserva(r dominio.Reserva) ([]byte, error) {
	return json.Marshal(reservaJSON{Formato: formatoDeRegistro, Reservado: r.Reservado, Instante: r.Instante})
}

func decodificarReserva(datos []byte) (dominio.Reserva, error) {
	var j reservaJSON
	if err := decodificar(datos, &j, &j.Formato); err != nil {
		return dominio.Reserva{}, err
	}
	return dominio.Reserva{Reservado: j.Reservado, Instante: j.Instante}, nil
}

func decodificar(datos []byte, destino any, formato *int) error {
	if err := json.Unmarshal(datos, destino); err != nil {
		return fmt.Errorf("decodificar: %w", err)
	}
	if *formato != formatoDeRegistro {
		return fmt.Errorf("decodificar: formato de registro %d desconocido", *formato)
	}
	return nil
}

func contenidoAJSON(c dominio.Contenido) *contenidoJSON {
	if c.Contexto == "" && len(c.Datos) == 0 {
		return nil
	}
	return &contenidoJSON{Contexto: string(c.Contexto), Datos: c.Datos}
}

func contenidoDeJSON(j *contenidoJSON) dominio.Contenido {
	if j == nil {
		return dominio.Contenido{}
	}
	return dominio.Contenido{Contexto: dominio.Contexto(j.Contexto), Datos: j.Datos}
}
