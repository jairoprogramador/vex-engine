package dominio

import (
	"slices"
	"time"
)

// TipoDeRegistro dice qué hecho cuenta un registro de un intento.
type TipoDeRegistro int

const (
	TipoApertura TipoDeRegistro = iota + 1
	TipoComienzo
	TipoFinal
	TipoNoReejecucion
	TipoVariable
	TipoValor
	TipoCierre
	TipoAbandono
)

// Apertura es lo que declara un intento al abrirse.
type Apertura struct {
	Ambiente    Ambiente
	Solicitante Solicitante
	// Pasos son todos los del pipeline, en orden: un despliegue los exige todos (IT-07 DEC-07.9).
	Pasos     []PasoDeclarado
	HastaPaso NombrePaso
	// ConCommits es falso si el material es una copia de trabajo: ese intento nunca llega a despliegue
	// (IT-10 DEC-10.7).
	ConCommits    bool
	HashDelCodigo HashDelCodigo
}

// Causa es por qué un intento terminó sin que un comando lo decidiera. Está vacía cuando el desenlace salió de
// los propios comandos (un paso exitoso, un comando que falló, una cancelación).
type Causa string

const (
	// CausaError: algo impidió seguir —interpolar, un puerto que falló— sin que ningún comando fallara.
	CausaError Causa = "error"
	// CausaInterrumpido: el proceso del intento murió sin cerrarlo, y otro intento lo cerró al encontrar el
	// ambiente ocupado y sin latidos.
	CausaInterrumpido Causa = "interrumpido"
)

// Cierre es cómo terminó un intento y, si es un rollback, a qué despliegue volvió.
type Cierre struct {
	Estado  Estado
	Causa   Causa
	Destino IdDespliegue
}

// validar: una causa solo explica un fallo —un intento exitoso o cancelado no la tiene— y solo hay dos.
func (c Cierre) validar() error {
	switch c.Causa {
	case "":
		return nil
	case CausaError, CausaInterrumpido:
		if c.Estado != Fallido {
			return rechazo("la causa %q solo la tiene un intento fallido, no uno %s", c.Causa, c.Estado)
		}
		return nil
	}
	return rechazo("la causa %q no existe", c.Causa)
}

// RegistroDeIntento es un hecho de un intento. Cada tipo usa sus campos, y el resto queda vacío.
type RegistroDeIntento struct {
	Tipo     TipoDeRegistro
	Instante time.Time

	Apertura  Apertura       // apertura
	Paso      NombrePaso     // comienzo, final, no re-ejecución, variable y valor
	Exitoso   bool           // final
	Evidencia Evidencia      // no re-ejecución
	Variable  NombreVariable // variable y valor
	Valor     string         // valor: solo sale por la relación reservada (IT-04 DEC-04.7)
	Cierre    Cierre         // cierre
	Contenido Contenido
}

// Intento es el agregado de un intento: sus registros, que se añaden y no se cambian. Cada registro se
// comprueba contra los que el intento ya tiene antes de añadirse.
type Intento struct {
	id        IdIntento
	registros []RegistroDeIntento
	leidos    int
}

// NuevoIntento es un intento que todavía no tiene ningún registro.
func NuevoIntento(id IdIntento) *Intento {
	return &Intento{id: id}
}

// ReconstituirIntento rehace un intento con sus registros leídos del almacén, comprobándolos uno a uno como
// cuando se escribieron.
func ReconstituirIntento(id IdIntento, registros []RegistroDeIntento) (*Intento, error) {
	i := NuevoIntento(id)
	for n, r := range registros {
		if err := i.anadir(r); err != nil {
			return nil, rechazo("intento %s, registro %d: %v", id, n+1, err)
		}
	}
	i.leidos = len(i.registros)
	return i, nil
}

func (i *Intento) Id() IdIntento { return i.id }

// Registros son todos, en el orden en que se añadieron.
func (i *Intento) Registros() []RegistroDeIntento { return slices.Clone(i.registros) }

// Leidos es cuántos registros venían del almacén. Los nuevos se añaden a partir de ahí.
func (i *Intento) Leidos() int { return i.leidos }

// Nuevos son los registros añadidos desde que se leyó.
func (i *Intento) Nuevos() []RegistroDeIntento { return slices.Clone(i.registros[i.leidos:]) }

// Abrir declara el intento. Solo hay una apertura, y es el primer registro.
func (i *Intento) Abrir(a Apertura, instante time.Time, contenido Contenido) error {
	return i.anadir(RegistroDeIntento{Tipo: TipoApertura, Instante: instante, Apertura: a, Contenido: contenido})
}

// Comenzar registra que un paso empieza, antes de su primer comando (IT-09 DEC-09.7).
func (i *Intento) Comenzar(paso NombrePaso, instante time.Time, contenido Contenido) error {
	return i.anadir(RegistroDeIntento{Tipo: TipoComienzo, Instante: instante, Paso: paso, Contenido: contenido})
}

// Terminar registra el final, exitoso o fallido, de un paso que comenzó.
func (i *Intento) Terminar(paso NombrePaso, exitoso bool, instante time.Time, contenido Contenido) error {
	return i.anadir(RegistroDeIntento{
		Tipo: TipoFinal, Instante: instante, Paso: paso, Exitoso: exitoso, Contenido: contenido,
	})
}

// NoReejecutar registra que un paso no se re-ejecuta. Su evidencia tiene que apuntar a un final exitoso, y
// para comprobarlo hace falta el intento apuntado (IT-09 DEC-09.7).
func (i *Intento) NoReejecutar(
	paso NombrePaso, evidencia Evidencia, apuntado *Intento, instante time.Time, contenido Contenido,
) error {
	if apuntado == nil || apuntado.id != evidencia.Intento || !apuntado.terminoBien(evidencia.Paso) {
		return rechazo("la evidencia de %q no apunta a un final exitoso: intento %s, paso %q",
			paso, evidencia.Intento, evidencia.Paso)
	}
	return i.anadir(RegistroDeIntento{
		Tipo: TipoNoReejecucion, Instante: instante, Paso: paso, Evidencia: evidencia, Contenido: contenido,
	})
}

// RegistrarVariable registra el hash de una variable de un paso en curso. El hash va en el contenido, que es
// de Resolución de Variables.
func (i *Intento) RegistrarVariable(
	paso NombrePaso, nombre NombreVariable, instante time.Time, contenido Contenido,
) error {
	return i.anadir(RegistroDeIntento{
		Tipo: TipoVariable, Instante: instante, Paso: paso, Variable: nombre, Contenido: contenido,
	})
}

// GuardarValor registra el valor de una variable de un paso en curso. Solo lo pide de vuelta Resolución de
// Variables, por la relación reservada.
func (i *Intento) GuardarValor(paso NombrePaso, nombre NombreVariable, valor string, instante time.Time) error {
	return i.anadir(RegistroDeIntento{
		Tipo: TipoValor, Instante: instante, Paso: paso, Variable: nombre, Valor: valor,
	})
}

// Cerrar registra el desenlace. El destino de un rollback tiene que ser un despliegue de su ambiente. Cerrar
// no crea el despliegue: eso es Desplegar, una vez escrito el cierre.
func (i *Intento) Cerrar(cierre Cierre, instante time.Time, despliegues *DesplieguesDeUnAmbiente) error {
	a, ok := i.Apertura()
	if !ok {
		return rechazo("el intento %s no tiene apertura, y no se cierra", i.id)
	}
	if despliegues == nil || despliegues.ambiente != a.Ambiente {
		return rechazo("el intento %s se cierra contra los despliegues de su ambiente, %q", i.id, a.Ambiente)
	}
	if err := cierre.validar(); err != nil {
		return err
	}
	if cierre.Destino != "" {
		if _, ok := despliegues.Buscar(cierre.Destino); !ok {
			return rechazo("el destino %s no es un despliegue de %q", cierre.Destino, a.Ambiente)
		}
	}
	return i.anadir(RegistroDeIntento{Tipo: TipoCierre, Instante: instante, Cierre: cierre})
}

// DarPorInterrumpido cierra el intento cuyo proceso murió: como fallido con CausaInterrumpido, o lo abandona si ni
// llegó a abrirse y no hay nada que cerrar. registrosVistos es lo que se observó antes de decidir que murió: si ya
// tiene más, su proceso escribió mientras tanto, estaba vivo, y devuelve ErrDuenoVivo sin tocarlo. Solo el Historial
// llama a esto, tras observar que no hubo señal de vida durante la ventana (SenalDeVida).
func (i *Intento) DarPorInterrumpido(
	registrosVistos int, instante time.Time, despliegues *DesplieguesDeUnAmbiente,
) error {
	if len(i.registros) > registrosVistos {
		return ErrDuenoVivo
	}
	if _, abierto := i.Apertura(); !abierto {
		return i.Abandonar(instante)
	}
	return i.Cerrar(Cierre{Estado: Fallido, Causa: CausaInterrumpido}, instante, despliegues)
}

// Desplegar es la factoría del despliegue (IT-07 DEC-07.3). Si el intento llega a despliegue, lo añade a los
// despliegues de su ambiente, con el destino de un rollback como padre o, si no lo hay, con el último. Si no
// llega, no hace nada. Si ya tiene su despliegue, lo devuelve sin añadir otro.
func (i *Intento) Desplegar(
	despliegues *DesplieguesDeUnAmbiente, nuevo IdDespliegue, instante time.Time,
) (Despliegue, bool, error) {
	cierre, ok := i.Cierre()
	if !ok || !i.llegaADespliegue() {
		return Despliegue{}, false, nil
	}
	a, _ := i.Apertura()
	if despliegues == nil || despliegues.ambiente != a.Ambiente {
		return Despliegue{}, false, rechazo("el despliegue de %s va a los despliegues de %q", i.id, a.Ambiente)
	}
	if d, ok := despliegues.DeUnIntento(i.id); ok {
		return d, true, nil
	}
	padre := cierre.Destino
	if padre == "" {
		if ultimo, ok := despliegues.Ultimo(); ok {
			padre = ultimo.id
		}
	}
	d := Despliegue{id: nuevo, ambiente: a.Ambiente, intento: i.id, padre: padre, instante: instante}
	if err := despliegues.anadir(d); err != nil {
		return Despliegue{}, false, err
	}
	return d, true, nil
}

// Abandonar da por abandonado un intento sin desenlace: libera su ambiente y ya no acepta registros
// (IT-07 DEC-07.8). También vale para un intento sin apertura, que ocupa su ambiente sin haber empezado.
func (i *Intento) Abandonar(instante time.Time) error {
	return i.anadir(RegistroDeIntento{Tipo: TipoAbandono, Instante: instante})
}

// Apertura es lo que declaró el intento, si ya se abrió.
func (i *Intento) Apertura() (Apertura, bool) {
	if len(i.registros) == 0 || i.registros[0].Tipo != TipoApertura {
		return Apertura{}, false
	}
	return i.registros[0].Apertura, true
}

// Cierre es el desenlace, si lo tiene.
func (i *Intento) Cierre() (Cierre, bool) {
	if r, ok := i.buscar(func(r RegistroDeIntento) bool { return r.Tipo == TipoCierre }); ok {
		return r.Cierre, true
	}
	return Cierre{}, false
}

// SinDesenlace: tiene registros y ninguno dice cómo terminó (IT-05 DEC-05.8).
func (i *Intento) SinDesenlace() bool {
	_, cerrado := i.Cierre()
	return len(i.registros) > 0 && !cerrado
}

// Abandonado: alguien lo dio por abandonado.
func (i *Intento) Abandonado() bool {
	_, ok := i.buscar(func(r RegistroDeIntento) bool { return r.Tipo == TipoAbandono })
	return ok
}

// Terminado: está cerrado o abandonado. Ya no acepta registros, y no ocupa su ambiente.
func (i *Intento) Terminado() bool {
	_, cerrado := i.Cierre()
	return cerrado || i.Abandonado()
}

// UltimoRegistroDePaso es el último comienzo, final o no re-ejecución de un paso en este intento.
func (i *Intento) UltimoRegistroDePaso(paso NombrePaso) (RegistroDeIntento, bool) {
	for k := len(i.registros) - 1; k >= 0; k-- {
		r := i.registros[k]
		if r.Paso == paso && (r.Tipo == TipoComienzo || r.Tipo == TipoFinal || r.Tipo == TipoNoReejecucion) {
			return r, true
		}
	}
	return RegistroDeIntento{}, false
}

// Variables son los registros de variable de un paso: el último de cada nombre, en el orden en que apareció
// cada nombre. Ninguno lleva valor.
func (i *Intento) Variables(paso NombrePaso) []RegistroDeIntento {
	return i.ultimosPorNombre(paso, TipoVariable)
}

// Valores son el último valor de cada variable de un paso.
func (i *Intento) Valores(paso NombrePaso) map[NombreVariable]string {
	valores := map[NombreVariable]string{}
	for _, r := range i.ultimosPorNombre(paso, TipoValor) {
		valores[r.Variable] = r.Valor
	}
	return valores
}

// DeclaraEn dice si este intento declaró el paso en esa posición.
func (i *Intento) DeclaraEn(p PasoEnSuAmbito) bool {
	a, ok := i.Apertura()
	if !ok {
		return false
	}
	for _, declarado := range a.Pasos {
		if declarado.Nombre != p.Paso || declarado.Compartido != p.Ambito.Compartido {
			continue
		}
		return p.Ambito.Compartido || a.Ambiente == p.Ambito.Ambiente
	}
	return false
}

func (i *Intento) anadir(r RegistroDeIntento) error {
	if r.Instante.IsZero() {
		return rechazo("un registro necesita su instante")
	}
	if i.Terminado() {
		return rechazo("el intento %s ya está cerrado o abandonado, y no acepta registros", i.id)
	}
	switch r.Tipo {
	case TipoApertura:
		if len(i.registros) > 0 {
			return rechazo("el intento %s ya tiene registros, y la apertura es el primero", i.id)
		}
		if err := comprobarApertura(r.Apertura); err != nil {
			return err
		}
	case TipoAbandono:
		// Terminado ya exige que no tenga desenlace.
	default:
		if err := i.comprobarRegistroTrasApertura(r); err != nil {
			return err
		}
	}
	i.registros = append(i.registros, r)
	return nil
}

func (i *Intento) comprobarRegistroTrasApertura(r RegistroDeIntento) error {
	if _, ok := i.Apertura(); !ok {
		return rechazo("el intento %s no tiene apertura", i.id)
	}
	switch r.Tipo {
	case TipoComienzo:
		if err := i.comprobarPasoPedido(r.Paso); err != nil {
			return err
		}
		if i.tiene(r.Paso, TipoComienzo) || i.tiene(r.Paso, TipoNoReejecucion) {
			return rechazo("el paso %q ya tiene un comienzo o una no re-ejecución", r.Paso)
		}
	case TipoFinal:
		if !i.tiene(r.Paso, TipoComienzo) || i.tiene(r.Paso, TipoFinal) {
			return rechazo("el paso %q no tiene un comienzo sin final", r.Paso)
		}
	case TipoNoReejecucion:
		if err := i.comprobarPasoPedido(r.Paso); err != nil {
			return err
		}
		if i.tiene(r.Paso, TipoComienzo) || i.tiene(r.Paso, TipoNoReejecucion) {
			return rechazo("el paso %q ya tiene un comienzo o una no re-ejecución", r.Paso)
		}
		if r.Evidencia.Intento == "" || r.Evidencia.Paso == "" || r.Evidencia.Intento == i.id {
			return rechazo("la no re-ejecución de %q necesita una evidencia en otro intento", r.Paso)
		}
	case TipoVariable, TipoValor:
		if r.Variable == "" {
			return rechazo("una variable necesita su nombre")
		}
		if !i.tiene(r.Paso, TipoComienzo) || i.tiene(r.Paso, TipoFinal) {
			return rechazo("la variable %q va bajo un paso en curso, y %q no lo está", r.Variable, r.Paso)
		}
	case TipoCierre:
		if !r.Cierre.Estado.valido() {
			return rechazo("un cierre necesita un estado")
		}
		if r.Cierre.Estado == Exitoso {
			for _, paso := range i.pasosPedidos() {
				if !i.hecho(paso) {
					return rechazo("el intento %s no se cierra como exitoso: el paso %q no terminó bien", i.id, paso)
				}
			}
		}
	default:
		return rechazo("tipo de registro desconocido: %d", r.Tipo)
	}
	return nil
}

func comprobarApertura(a Apertura) error {
	if a.Ambiente == "" || a.Solicitante == "" || a.HashDelCodigo == "" {
		return rechazo("una apertura necesita ambiente, solicitante y hash del código")
	}
	if len(a.Pasos) == 0 {
		return rechazo("una apertura declara los pasos del pipeline")
	}
	hasta := false
	vistos := map[NombrePaso]bool{}
	for _, p := range a.Pasos {
		if p.Nombre == "" || vistos[p.Nombre] {
			return rechazo("los pasos de una apertura tienen nombre y no se repiten: %q", p.Nombre)
		}
		vistos[p.Nombre] = true
		hasta = hasta || p.Nombre == a.HastaPaso
	}
	if !hasta {
		return rechazo("el paso pedido %q no es un paso del pipeline", a.HastaPaso)
	}
	return nil
}

func (i *Intento) comprobarPasoPedido(paso NombrePaso) error {
	if !slices.Contains(i.pasosPedidos(), paso) {
		return rechazo("el paso %q no está entre los pedidos al intento %s", paso, i.id)
	}
	return nil
}

// pasosPedidos son los del pipeline hasta el pedido, incluido.
func (i *Intento) pasosPedidos() []NombrePaso {
	a, _ := i.Apertura()
	var pedidos []NombrePaso
	for _, p := range a.Pasos {
		pedidos = append(pedidos, p.Nombre)
		if p.Nombre == a.HastaPaso {
			break
		}
	}
	return pedidos
}

// llegaADespliegue: cerrado como exitoso, con commits, y con todos los pasos del pipeline hechos
// (IT-07 DEC-07.9, IT-10 DEC-10.7).
func (i *Intento) llegaADespliegue() bool {
	cierre, cerrado := i.Cierre()
	a, _ := i.Apertura()
	if !cerrado || cierre.Estado != Exitoso || !a.ConCommits {
		return false
	}
	for _, p := range a.Pasos {
		if !i.hecho(p.Nombre) {
			return false
		}
	}
	return true
}

// hecho: un final exitoso o una no re-ejecución (IT-09 DEC-09.7).
func (i *Intento) hecho(paso NombrePaso) bool {
	return i.terminoBien(paso) || i.tiene(paso, TipoNoReejecucion)
}

func (i *Intento) terminoBien(paso NombrePaso) bool {
	_, ok := i.buscar(func(r RegistroDeIntento) bool { return r.Tipo == TipoFinal && r.Paso == paso && r.Exitoso })
	return ok
}

func (i *Intento) tiene(paso NombrePaso, tipo TipoDeRegistro) bool {
	_, ok := i.buscar(func(r RegistroDeIntento) bool { return r.Tipo == tipo && r.Paso == paso })
	return ok
}

func (i *Intento) buscar(cumple func(RegistroDeIntento) bool) (RegistroDeIntento, bool) {
	for _, r := range i.registros {
		if cumple(r) {
			return r, true
		}
	}
	return RegistroDeIntento{}, false
}

func (i *Intento) ultimosPorNombre(paso NombrePaso, tipo TipoDeRegistro) []RegistroDeIntento {
	var orden []NombreVariable
	ultimos := map[NombreVariable]RegistroDeIntento{}
	for _, r := range i.registros {
		if r.Tipo != tipo || r.Paso != paso {
			continue
		}
		if _, visto := ultimos[r.Variable]; !visto {
			orden = append(orden, r.Variable)
		}
		ultimos[r.Variable] = r
	}
	resultado := make([]RegistroDeIntento, 0, len(orden))
	for _, nombre := range orden {
		resultado = append(resultado, ultimos[nombre])
	}
	return resultado
}

// UltimaVezDeUnPaso es el último registro de un paso en su ámbito, y el intento que lo tiene (IT-06
// DEC-06.12). En el ámbito de un ambiente, los intentos llegan en el orden del ambiente y gana el último que
// tenga registro del paso. En el compartido, entre ambientes no hay un orden total, y gana el registro con el
// instante mayor.
func UltimaVezDeUnPaso(p PasoEnSuAmbito, intentos []*Intento) (RegistroDeIntento, IdIntento, bool) {
	var (
		ultimo RegistroDeIntento
		de     IdIntento
		hay    bool
	)
	for _, intento := range intentos {
		if !intento.DeclaraEn(p) {
			continue
		}
		r, ok := intento.UltimoRegistroDePaso(p.Paso)
		if !ok {
			continue
		}
		if !hay || !p.Ambito.Compartido || r.Instante.After(ultimo.Instante) {
			ultimo, de, hay = r, intento.id, true
		}
	}
	return ultimo, de, hay
}
