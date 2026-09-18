#!/usr/bin/env bash
# Demo de vexd: compila el ejecutable, prepara un entorno de prueba y hace un primer intento.
# Guía completa: docs/guia-de-pruebas.md
#
# Uso:   scripts/demo.sh [--limpio] [--sin-compilar]
# El script pregunta el ambiente, el paso hasta el que se ejecuta, si limpia y si compila.
# Sin respuesta (Enter, o sin terminal) se asume el valor por defecto: primer ambiente, primer
# paso, no limpiar y sí compilar. Las opciones evitan la pregunta correspondiente:
#   --limpio        borra el directorio de la demo antes de empezar (empieza sin historial)
#   --sin-compilar  usa el ./vexd que ya existe
#
# Variables opcionales:
#   DEMO_DIR    dónde se arma la demo (por defecto /tmp/demo-vex)
#   AMBIENTE    ambiente que se intenta; si viene dada no se pregunta (lista: environments.yaml del ejemplo)
#   HASTA_PASO  paso hasta el que se ejecuta; si viene dada no se pregunta (lista: steps/ del ejemplo)
#
# Al terminar, deja escrito $DEMO_DIR/entorno.sh: haz `source $DEMO_DIR/entorno.sh` para tener las
# variables y la función `vexd_demo` con las que seguir probando otras operaciones.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EJEMPLO="$RAIZ/internal/ejecucion/testdata/ejemplo"
DEMO_DIR="${DEMO_DIR:-/tmp/demo-vex}"
AMBIENTE="${AMBIENTE:-}"
HASTA_PASO="${HASTA_PASO:-}"
LIMPIO=""
COMPILAR=""
for arg in "$@"; do
  case "$arg" in
    --limpio) LIMPIO=1 ;;
    --sin-compilar) COMPILAR=0 ;;
    -h|--help) sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

# Lee una respuesta solo si hay terminal; si no, o si viene vacía, queda vacía (= el defecto).
leer_respuesta() {
  RESPUESTA=""
  if [ -t 0 ]; then read -r -p "$1" RESPUESTA || true; fi
}

# pregunta_si_no <texto> <defecto: 1|0>  → deja 1 (sí) o 0 (no) en RESPUESTA_SI_NO
pregunta_si_no() {
  local sufijo="[s/N]"
  [ "$2" = 1 ] && sufijo="[S/n]"
  leer_respuesta "$1 $sufijo: "
  case "$RESPUESTA" in
    [sS]|[sS][iíIÍ]|[yY]|[yY][eE][sS]) RESPUESTA_SI_NO=1 ;;
    [nN]|[nN][oO]) RESPUESTA_SI_NO=0 ;;
    *) RESPUESTA_SI_NO="$2" ;;
  esac
}

# elegir <título> <valores...> con las etiquetas en ETIQUETAS (mismo orden) → deja el valor en ELEGIDO.
# Acepta el número o el valor; Enter o una respuesta que no existe elige el primero.
elegir() {
  local titulo="$1"; shift
  local valores=("$@") i
  ELEGIDO="${valores[0]}"
  echo "$titulo"
  for i in "${!valores[@]}"; do
    printf '  %d) %s\n' "$((i + 1))" "${ETIQUETAS[$i]}"
  done
  leer_respuesta "elige [1]: "
  [ -n "$RESPUESTA" ] || return 0
  if [[ "$RESPUESTA" =~ ^[0-9]+$ ]] && [ "$RESPUESTA" -ge 1 ] && [ "$RESPUESTA" -le "${#valores[@]}" ]; then
    ELEGIDO="${valores[$((RESPUESTA - 1))]}"
    return 0
  fi
  for i in "${!valores[@]}"; do
    [ "${valores[$i]}" = "$RESPUESTA" ] && { ELEGIDO="$RESPUESTA"; return 0; }
  done
  echo "  '$RESPUESTA' no es una opción: se usa $ELEGIDO"
}

# Ambientes disponibles: cada entrada de environments.yaml (name / description / value).
ambientes=(); ETIQUETAS=()
while IFS=$'\t' read -r valor etiqueta; do
  ambientes+=("$valor"); ETIQUETAS+=("$valor — $etiqueta")
done < <(awk '
  function cierra() { if (v != "") print v "\t" n (d != "" ? " · " d : "") }
  /^- *name:/        { cierra(); n = $0; sub(/^- *name: */, "", n); d = ""; v = "" }
  /^ +description:/  { d = $0; sub(/^ +description: */, "", d) }
  /^ +value:/        { v = $0; sub(/^ +value: */, "", v) }
  END                { cierra() }
' "$EJEMPLO/environments.yaml")
[ "${#ambientes[@]}" -gt 0 ] || { echo "no hay ambientes en $EJEMPLO/environments.yaml" >&2; exit 1; }

# Pasos disponibles: los directorios de steps/, sin el prefijo de orden (01-preparar → preparar).
pasos=()
for dir in "$EJEMPLO"/steps/*/; do
  nombre="$(basename "$dir")"
  pasos+=("${nombre#[0-9][0-9]-}")
done
[ "${#pasos[@]}" -gt 0 ] || { echo "no hay pasos en $EJEMPLO/steps" >&2; exit 1; }

if [ -z "$AMBIENTE" ]; then
  elegir "Ambientes disponibles:" "${ambientes[@]}"
  AMBIENTE="$ELEGIDO"
fi
if [ -z "$HASTA_PASO" ]; then
  ETIQUETAS=("${pasos[@]}")
  elegir "Pasos disponibles (se ejecuta hasta el elegido):" "${pasos[@]}"
  HASTA_PASO="$ELEGIDO"
fi
if [ -z "$LIMPIO" ]; then
  pregunta_si_no "¿Borrar la demo anterior y empezar sin historial?" 0
  LIMPIO="$RESPUESTA_SI_NO"
fi
if [ -z "$COMPILAR" ]; then
  pregunta_si_no "¿Compilar vexd?" 1
  COMPILAR="$RESPUESTA_SI_NO"
fi

VEXD="$RAIZ/vexd"

# 1. Compilar
if [ "$COMPILAR" = 1 ]; then
  echo "==> compilando $VEXD"
  (cd "$RAIZ" && go build -o vexd ./cmd/vexd)
fi
[ -x "$VEXD" ] || { echo "no existe $VEXD: responde que sí a compilar" >&2; exit 1; }
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
  cp -R "$EJEMPLO/." "$DEMO_DIR/pipeline/"
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
echo "==> intentar en '$AMBIENTE' hasta '$HASTA_PASO'  (stderr: lo que imprimen los comandos · stdout: la respuesta)"
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
  "HastaPaso": "$HASTA_PASO",
  "Metadatos": { "ProjectName": "vex-demo", "ProjectId": "p1" }
}
PETICION
codigo=$?
set -e

echo "==> código de salida: $codigo   (0 bien · 1 falló · 2 petición inválida · 130 cancelado)"
echo "==> para seguir: source $DEMO_DIR/entorno.sh   y mira docs/guia-de-pruebas.md"
exit "$codigo"
