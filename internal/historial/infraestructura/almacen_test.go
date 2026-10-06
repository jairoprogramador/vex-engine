package infraestructura

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

func contexto() context.Context { return context.Background() }

// Las garantías del almacén valen igual para los dos.
func almacenes() map[string]func(t *testing.T) Almacen {
	return map[string]func(t *testing.T) Almacen{
		"local": func(t *testing.T) Almacen {
			a, err := NuevoAlmacenLocal(t.TempDir())
			require.NoError(t, err)
			return a
		},
		"memoria": func(*testing.T) Almacen { return NuevoAlmacenEnMemoria() },
	}
}

func TestAlmacen_UnaSecuenciaSoloCrece(t *testing.T) {
	for nombre, nuevo := range almacenes() {
		t.Run(nombre, func(t *testing.T) {
			a, ctx := nuevo(t), contexto()
			s := Secuencia{Familia: "intentos", Nombre: "i1"}

			vacia, err := a.Leer(ctx, s)
			require.NoError(t, err)
			require.Empty(t, vacia)

			require.NoError(t, a.Anadir(ctx, s, 1, []byte("uno")))
			require.NoError(t, a.Anadir(ctx, s, 2, []byte("dos")))

			require.ErrorIs(t, a.Anadir(ctx, s, 2, []byte("otro")), dominio.ErrConflicto, "no se sobrescribe")
			err = a.Anadir(ctx, s, 4, []byte("hueco"))
			require.Error(t, err)
			require.NotErrorIs(t, err, dominio.ErrConflicto)
			require.Error(t, a.Anadir(ctx, s, 0, []byte("cero")))

			leidos, err := a.Leer(ctx, s)
			require.NoError(t, err)
			require.Equal(t, [][]byte{[]byte("uno"), []byte("dos")}, leidos)
		})
	}
}

func TestAlmacen_DeVariosEscritoresEnLaMismaPosicionGanaUno(t *testing.T) {
	for nombre, nuevo := range almacenes() {
		t.Run(nombre, func(t *testing.T) {
			a, ctx := nuevo(t), contexto()
			s := Secuencia{Familia: "ocupaciones", Nombre: "staging"}

			const escritores = 16
			errores := make(chan error, escritores)
			var wg sync.WaitGroup
			for k := range escritores {
				wg.Add(1)
				go func(k int) {
					defer wg.Done()
					errores <- a.Anadir(ctx, s, 1, []byte{byte(k)})
				}(k)
			}
			wg.Wait()
			close(errores)

			ganadores := 0
			for err := range errores {
				if err == nil {
					ganadores++
					continue
				}
				require.ErrorIs(t, err, dominio.ErrConflicto)
			}
			require.Equal(t, 1, ganadores)
			leidos, err := a.Leer(ctx, s)
			require.NoError(t, err)
			require.Len(t, leidos, 1)
		})
	}
}

func TestAlmacen_NombresDeUnaFamilia(t *testing.T) {
	for nombre, nuevo := range almacenes() {
		t.Run(nombre, func(t *testing.T) {
			a, ctx := nuevo(t), contexto()
			require.NoError(t, a.Anadir(ctx, Secuencia{Familia: "intentos", Nombre: "b"}, 1, []byte("x")))
			require.NoError(t, a.Anadir(ctx, Secuencia{Familia: "intentos", Nombre: "a"}, 1, []byte("x")))
			require.NoError(t, a.Anadir(ctx, Secuencia{Familia: "despliegues", Nombre: "staging"}, 1, []byte("x")))
			require.NoError(t, a.Anadir(ctx, Secuencia{Familia: "lanzamientos"}, 1, []byte("x")))

			nombres, err := a.Nombres(ctx, "intentos")
			require.NoError(t, err)
			require.Equal(t, []string{"a", "b"}, nombres)

			nombres, err = a.Nombres(ctx, "ocupaciones")
			require.NoError(t, err)
			require.Empty(t, nombres)
		})
	}
}

func TestAlmacenLocal_LoEscritoSeLeeDesdeOtraInstanciaYNoDejaTemporales(t *testing.T) {
	raiz, ctx := t.TempDir(), contexto()
	s := Secuencia{Familia: "despliegues", Nombre: "staging"}

	escritor, err := NuevoAlmacenLocal(raiz)
	require.NoError(t, err)
	require.NoError(t, escritor.Anadir(ctx, s, 1, []byte("uno")))

	lector, err := NuevoAlmacenLocal(raiz)
	require.NoError(t, err)
	leidos, err := lector.Leer(ctx, s)
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("uno")}, leidos)

	entradas, err := os.ReadDir(filepath.Join(raiz, "despliegues", "staging"))
	require.NoError(t, err)
	require.Len(t, entradas, 1)
	require.Equal(t, "000000000001.json", entradas[0].Name())
}

func TestAlmacenLocal_UnHuecoEnLaSecuenciaEsUnError(t *testing.T) {
	raiz, ctx := t.TempDir(), contexto()
	dir := filepath.Join(raiz, "intentos", "i1")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "000000000001.json"), []byte("uno"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "000000000003.json"), []byte("tres"), 0o644))

	a, err := NuevoAlmacenLocal(raiz)
	require.NoError(t, err)
	_, err = a.Leer(ctx, Secuencia{Familia: "intentos", Nombre: "i1"})
	require.ErrorContains(t, err, "falta la posición 2")
}

func TestAlmacenLocal_UnNombreQueVieneDeFueraNoSaleDeSuDirectorio(t *testing.T) {
	fuera, ctx := t.TempDir(), contexto()
	raiz := filepath.Join(fuera, "raiz")
	require.NoError(t, os.Mkdir(raiz, 0o755))
	a, err := NuevoAlmacenLocal(raiz)
	require.NoError(t, err)

	s := Secuencia{Familia: "intentos", Nombre: "../../fuera"}
	require.NoError(t, a.Anadir(ctx, s, 1, []byte("x")))

	entradas, err := os.ReadDir(fuera)
	require.NoError(t, err)
	require.Len(t, entradas, 1, "nada se escribió fuera de la raíz")
	nombres, err := a.Nombres(ctx, "intentos")
	require.NoError(t, err)
	require.Equal(t, []string{"../../fuera"}, nombres)
	leidos, err := a.Leer(ctx, s)
	require.NoError(t, err)
	require.Len(t, leidos, 1)
}

func TestNuevoAlmacenLocal_LaRaizTieneQueExistir(t *testing.T) {
	_, err := NuevoAlmacenLocal(filepath.Join(t.TempDir(), "no-existe"))
	require.Error(t, err)
}

func TestOfuscar_ElValorVuelveYNoSeLeeASimpleVista(t *testing.T) {
	for _, valor := range []string{"", "3", "s3cr3t-de-produccion", "contraseña con eñe"} {
		ofuscado := ofuscar(valor)
		if len(valor) > 3 {
			require.False(t, strings.Contains(ofuscado, valor), "%q aparece en %q", valor, ofuscado)
		}
		vuelta, err := desofuscar(ofuscado)
		require.NoError(t, err)
		require.Equal(t, valor, vuelta)
	}
	_, err := desofuscar("en-claro")
	require.Error(t, err)
}
