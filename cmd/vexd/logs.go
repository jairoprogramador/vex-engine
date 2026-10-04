package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/borde"
)

// logsDeUnIntento es lo que la línea de comandos muestra de la salida de los comandos de un intento: cuál es y,
// por cada paso que corrió, lo que escribió cada comando. Es solo presentación: el Historial guarda, y el
// borde publica, la salida completa.
type logsDeUnIntento struct {
	IntentoId string
	Salidas   salidasPorPaso
}

type vistaDeLog struct {
	Comando   string `json:"comando"`
	Salida    string `json:"salida"`
	Resultado string `json:"resultado"`
}

// salidasPorPaso son los comandos de cada paso, en el orden en que corrieron los pasos. Un map los daría
// ordenados por nombre, y el orden en que se ejecutaron es lo que quien lee necesita.
type salidasPorPaso []salidasDeUnPaso

type salidasDeUnPaso struct {
	Paso     string
	Comandos []vistaDeLog
}

func (s salidasPorPaso) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for k, paso := range s {
		if k > 0 {
			b.WriteByte(',')
		}
		nombre, err := json.Marshal(paso.Paso)
		if err != nil {
			return nil, err
		}
		comandos, err := json.Marshal(paso.Comandos)
		if err != nil {
			return nil, err
		}
		b.Write(nombre)
		b.WriteByte(':')
		b.Write(comandos)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// presentarLogs agrupa por paso, que son los que corrieron: uno al que el intento no llegó, o que no re-ejecutó,
// no tiene salidas y no aparece. Al texto se le quita el salto de línea final con que el comando terminó su
// última línea.
func presentarLogs(r borde.RespuestaDeLogs) logsDeUnIntento {
	logs := logsDeUnIntento{IntentoId: r.Intento, Salidas: salidasPorPaso{}}
	for _, s := range r.Salidas {
		vista := vistaDeLog{
			Comando:   s.Comando,
			Salida:    strings.TrimRight(s.Texto, "\r\n"),
			Resultado: resultadoDeLaSalida(s.Exitoso),
		}
		if ultimo := len(logs.Salidas) - 1; ultimo >= 0 && logs.Salidas[ultimo].Paso == s.Paso {
			logs.Salidas[ultimo].Comandos = append(logs.Salidas[ultimo].Comandos, vista)
			continue
		}
		logs.Salidas = append(logs.Salidas, salidasDeUnPaso{Paso: s.Paso, Comandos: []vistaDeLog{vista}})
	}
	return logs
}

func resultadoDeLaSalida(exitoso bool) string {
	if exitoso {
		return estadoExitoso
	}
	return estadoFallido
}
