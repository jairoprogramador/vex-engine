package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Traducción entre el modelo y el lenguaje publicado. Es el único sitio donde un registro sale del dominio, y
// por eso donde se deja fuera el valor: ningún tipo publicado tiene dónde ponerlo.

func aperturaDeDominio(a publicado.Apertura) dominio.Apertura {
	apertura := dominio.Apertura{
		Ambiente:      dominio.Ambiente(a.Ambiente),
		Solicitante:   dominio.Solicitante(a.Solicitante),
		HastaPaso:     dominio.NombrePaso(a.HastaPaso),
		ConCommits:    a.ConCommits,
		HashDelCodigo: dominio.HashDelCodigo(a.HashDelCodigo),
	}
	for _, p := range a.Pasos {
		apertura.Pasos = append(apertura.Pasos, dominio.PasoDeclarado{
			Nombre: dominio.NombrePaso(p.Nombre), Compartido: p.Compartido,
		})
	}
	return apertura
}

func contenidoDeDominio(c publicado.Contenido) dominio.Contenido {
	return dominio.Contenido{Contexto: dominio.Contexto(c.Contexto), Datos: c.Datos}
}

func contenidoAPublicado(c dominio.Contenido) publicado.Contenido {
	return publicado.Contenido{Contexto: string(c.Contexto), Datos: c.Datos}
}

var estadosPublicados = map[dominio.Estado]publicado.Estado{
	dominio.Exitoso:   publicado.Exitoso,
	dominio.Fallido:   publicado.Fallido,
	dominio.Cancelado: publicado.Cancelado,
}

// estadoDeDominio da el estado cero, que el dominio rechaza, si el publicado no es ninguno de los tres.
func estadoDeDominio(e publicado.Estado) dominio.Estado {
	for de, pub := range estadosPublicados {
		if pub == e {
			return de
		}
	}
	return 0
}

func intentoAPublicado(i *dominio.Intento) (publicado.Intento, bool) {
	a, ok := i.Apertura()
	if !ok {
		return publicado.Intento{}, false
	}
	registros := i.Registros()
	resultado := publicado.Intento{
		Id:         string(i.Id()),
		Instante:   registros[0].Instante,
		Abandonado: i.Abandonado(),
		Apertura: publicado.Apertura{
			Ambiente:      string(a.Ambiente),
			Solicitante:   string(a.Solicitante),
			HastaPaso:     string(a.HastaPaso),
			ConCommits:    a.ConCommits,
			HashDelCodigo: string(a.HashDelCodigo),
			Contenido:     contenidoAPublicado(registros[0].Contenido),
			Pasos:         make([]publicado.PasoDeclarado, 0, len(a.Pasos)),
		},
		// Un intento que aún no dio ningún paso tiene una lista vacía de registros, no nil.
		Registros: []publicado.RegistroDePaso{},
	}
	for _, p := range a.Pasos {
		resultado.Apertura.Pasos = append(resultado.Apertura.Pasos, publicado.PasoDeclarado{
			Nombre: string(p.Nombre), Compartido: p.Compartido,
		})
	}
	if cierre, ok := i.Cierre(); ok {
		resultado.Estado = estadosPublicados[cierre.Estado]
		resultado.Destino = string(cierre.Destino)
	}
	for _, r := range registros {
		if r.Tipo == dominio.TipoComienzo || r.Tipo == dominio.TipoFinal || r.Tipo == dominio.TipoNoReejecucion {
			resultado.Registros = append(resultado.Registros, registroDePasoAPublicado(i.Id(), r))
		}
	}
	return resultado, true
}

var tiposDePasoPublicados = map[dominio.TipoDeRegistro]publicado.TipoDeRegistroDePaso{
	dominio.TipoComienzo:      publicado.Comienzo,
	dominio.TipoFinal:         publicado.Final,
	dominio.TipoNoReejecucion: publicado.NoReejecucion,
}

func registroDePasoAPublicado(intento dominio.IdIntento, r dominio.RegistroDeIntento) publicado.RegistroDePaso {
	return publicado.RegistroDePaso{
		Intento:   string(intento),
		Paso:      string(r.Paso),
		Tipo:      tiposDePasoPublicados[r.Tipo],
		Exitoso:   r.Exitoso,
		Evidencia: publicado.Evidencia{Intento: string(r.Evidencia.Intento), Paso: string(r.Evidencia.Paso)},
		Instante:  r.Instante,
		Contenido: contenidoAPublicado(r.Contenido),
	}
}

func despliegueAPublicado(d dominio.Despliegue) publicado.Despliegue {
	return publicado.Despliegue{
		Id: string(d.Id()), Ambiente: string(d.Ambiente()), Intento: string(d.Intento()),
		Padre: string(d.Padre()), Instante: d.Instante(),
	}
}

func lanzamientoAPublicado(l dominio.Lanzamiento) publicado.Lanzamiento {
	return publicado.Lanzamiento{
		Id: string(l.Id()), Ambiente: string(l.Ambiente()), Despliegue: string(l.Despliegue()),
		Instante: l.Instante(), Contenido: contenidoAPublicado(l.Contenido()),
	}
}

func salidaAPublicado(s dominio.Salida) publicado.Salida {
	return publicado.Salida{
		Paso: string(s.Paso), Comando: s.Comando, Exitoso: s.Exitoso, Texto: s.Texto, Instante: s.Instante,
	}
}
