//go:build !windows

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
)

// Con el motor entero: lo que un paso guarda en el directorio efímero, o deja en el directorio de otro paso, no
// llega al paso siguiente.
func TestAislamiento_UnPasoNoLeDejaNadaAlSiguiente(t *testing.T) {
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/04-package/commands.yaml", "- name: deja-cosas\n  description: guarda una sesión y planta un fichero en el paso siguiente\n"+
			"  cmd: echo sesion > \"$VEX_STEP_TMP/sesion\" && echo plantado > ${var.step_workdir}/../deploy/plantado && echo \"efimero=$VEX_STEP_TMP\"\n")
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: mira\n  description: dice lo que encuentra\n"+
			"  cmd: echo \"efimero=$VEX_STEP_TMP\"; echo \"dentro=$(ls -A \"$VEX_STEP_TMP\" | wc -l | tr -d ' ')\"; test -e plantado && echo plantado=si || echo plantado=no\n")
	})
	e.rutas.efimero = t.TempDir()
	efimero := filepath.Join(e.rutas.efimero, "vex-step-tmp")

	r := invocar(t, e.rutas, "intentar", e.intento())

	require.Equal(t, salidaBien, r.codigo, r.salida+r.errores)
	var l borde.RespuestaDeLogs
	invocar(t, e.rutas, "logs", `{"Version":"1"}`).resultado(t, &l)
	salidas := map[string]string{}
	for _, salida := range l.Salidas {
		salidas[salida.Comando] = strings.TrimSpace(salida.Texto)
	}
	require.Equal(t, "efimero="+efimero, salidas["deja-cosas"], "la ruta es la que se configuró")
	require.Equal(t, "efimero="+efimero+"\ndentro=0\nplantado=no", salidas["mira"],
		"la misma ruta, vacía otra vez; y el directorio del paso, solo con lo declarado")
	require.NoDirExists(t, efimero, "al terminar el último paso tampoco queda")
}

func TestRutasDelEntorno_LeeLaBaseDelDirectorioEfimero(t *testing.T) {
	entorno := map[string]string{nombreAlmacen: "/a", nombreEspacio: "/e", nombreMaterial: "/m", nombreEfimero: "/tmp"}

	r := rutasDelEntorno(func(nombre string) string { return entorno[nombre] })

	require.Equal(t, rutas{almacen: "/a", espacio: "/e", material: "/m", efimero: "/tmp"}, r)
}
