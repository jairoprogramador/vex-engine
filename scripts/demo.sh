#!/usr/bin/env bash
# Demo de vexd: compila el ejecutable, prepara un entorno de prueba y hace un primer intento.
# Guía completa: docs/guia-de-pruebas.md
#
# Uso:   scripts/demo.sh [--limpio] [--sin-compilar]
#   --limpio        borra el directorio de la demo antes de empezar (empieza sin historial)
#   --sin-compilar  usa el ./vexd que ya existe
#
# Variables opcionales:
#   DEMO_DIR   dónde se arma la demo (por defecto /tmp/demo-vex)
#   AMBIENTE   ambiente que se intenta (por defecto prod, el de environments.yaml del ejemplo)
#
# Al terminar, deja escrito $DEMO_DIR/entorno.sh: haz `source $DEMO_DIR/entorno.sh` para tener las
# variables y la función `vexd_demo` con las que seguir probando otras operaciones.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEMO_DIR="${DEMO_DIR:-/tmp/demo-vex}"
AMBIENTE="${AMBIENTE:-prod}"
LIMPIO=0
COMPILAR=1
for arg in "$@"; do
  case "$arg" in
    --limpio) LIMPIO=1 ;;
    --sin-compilar) COMPILAR=0 ;;
    -h|--help) sed -n '2,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

VEXD="$RAIZ/vexd"

# 1. Compilar
if [ "$COMPILAR" = 1 ]; then
  echo "==> compilando $VEXD"
  (cd "$RAIZ" && go build -o vexd ./cmd/vexd)
fi
[ -x "$VEXD" ] || { echo "no existe $VEXD: quita --sin-compilar" >&2; exit 1; }
echo "==> vexd $("$VEXD" version)"

# 2. Preparar directorios y repos (el del pipeline solo se crea si no existe: así se puede editar entre pruebas)
[ "$LIMPIO" = 1 ] && rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"/{almacen,espacio,material}

git_demo() { git -C "$1" -c user.name=demo -c user.email=demo@vex.test "${@:2}"; }

if [ ! -d "$DEMO_DIR/proyecto/.git" ]; then
  echo "==> creando el repo del proyecto"
  mkdir -p "$DEMO_DIR/proyecto"
  echo "mi app" > "$DEMO_DIR/proyecto/README.md"
  git -C "$DEMO_DIR/proyecto" init -q
  git_demo "$DEMO_DIR/proyecto" add -A
  git_demo "$DEMO_DIR/proyecto" commit -qm inicial
fi
if [ ! -d "$DEMO_DIR/pipeline/.git" ]; then
  echo "==> creando el repo del pipeline (copia de internal/ejecucion/testdata/ejemplo)"
  mkdir -p "$DEMO_DIR/pipeline"
  cp -R "$RAIZ/internal/ejecucion/testdata/ejemplo/." "$DEMO_DIR/pipeline/"
  git -C "$DEMO_DIR/pipeline" init -q
  git_demo "$DEMO_DIR/pipeline" add -A
  git_demo "$DEMO_DIR/pipeline" commit -qm inicial
fi

# Deja un entorno listo para seguir probando a mano.
cat > "$DEMO_DIR/entorno.sh" <<ENTORNO
export VEXD="$VEXD"
export DEMO_DIR="$DEMO_DIR"
export VEX_ALMACEN="$DEMO_DIR/almacen"
export VEX_ESPACIO="$DEMO_DIR/espacio"
export VEX_MATERIAL="$DEMO_DIR/material"
# vexd_demo <operación>   (la petición JSON se lee de la entrada estándar)
vexd_demo() { "$VEXD" "\$@"; }
ENTORNO

# 3. Intentar
echo "==> intentar en '$AMBIENTE'  (stderr: lo que imprimen los comandos · stdout: la respuesta)"
set +e
"$VEXD" intentar \
  --almacen "$DEMO_DIR/almacen" \
  --espacio "$DEMO_DIR/espacio" \
  --material "$DEMO_DIR/material" <<PETICION
{
  "Version": "1",
  "Ambiente": "$AMBIENTE",
  "Solicitante": "${USER:-demo}",
  "FuenteDelProyecto": "$DEMO_DIR/proyecto",
  "FuenteDelPipeline": "$DEMO_DIR/pipeline",
  "HastaPaso": "preparar"
  "Metadatos": { "ProjectName": "vex-demo", "ProjectId": "p1" }
}
PETICION
codigo=$?
set -e

echo "==> código de salida: $codigo   (0 bien · 1 falló · 2 petición inválida · 130 cancelado)"
echo "==> para seguir: source $DEMO_DIR/entorno.sh   y mira docs/guia-de-pruebas.md"
exit "$codigo"
