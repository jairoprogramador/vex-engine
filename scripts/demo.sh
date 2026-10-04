#!/usr/bin/env bash
# Demo de vexd: compila el ejecutable, prepara un entorno de prueba y ejecuta una operación (por defecto, intentar).
# Guía completa: docs/guia-de-pruebas.md
#
# Uso:   scripts/demo.sh [--limpio] [--sin-compilar]
# El script pregunta la operación, sus datos (ambiente, paso, ids…), si limpia y si compila.
# Sin respuesta (Enter, o sin terminal) se asume el valor por defecto: primer ambiente, primer
# paso, no limpiar y sí compilar. Las opciones evitan la pregunta correspondiente:
#   --limpio        borra el directorio de la demo antes de empezar (empieza sin historial)
#   --sin-compilar  usa el ./vexd que ya existe
#
# Variables opcionales:
#   DEMO_DIR    dónde se arma la demo (por defecto /tmp/demo-vex)
#   OPERACION   operación de vexd (intentar, rollback, simular, lanzar…); si viene dada no se pregunta
#   DESPLIEGUE, INTENTO, NOMBRE, REFERENCIA, RESULTADO   datos de las operaciones que los piden; si vienen dados no se preguntan
#   AMBIENTE    ambiente que se intenta o simula; si viene dada no se pregunta (lista: environments.yaml del ejemplo)
#   HASTA_PASO  paso hasta el que se ejecuta o simula; si viene dada no se pregunta (lista: steps/ del ejemplo)
#
# Al terminar, deja escrito $DEMO_DIR/entorno.sh: haz `source $DEMO_DIR/entorno.sh` para tener las
# variables y la función `vexd_demo` con las que seguir probando otras operaciones.
set -euo pipefail

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EJEMPLO="$RAIZ/internal/ejecucion/testdata/ejemplo"
DEMO_DIR="${DEMO_DIR:-/tmp/demo-vex}"
AMBIENTE="${AMBIENTE:-}"
HASTA_PASO="${HASTA_PASO:-}"
OPERACION="${OPERACION:-}"
DESPLIEGUE="${DESPLIEGUE:-}"
INTENTO="${INTENTO:-}"
NOMBRE="${NOMBRE:-}"
REFERENCIA="${REFERENCIA:-}"
RESULTADO="${RESULTADO:-}"
LIMPIO=""
COMPILAR=""
for arg in "$@"; do
  case "$arg" in
    --limpio) LIMPIO=1 ;;
    --sin-compilar) COMPILAR=0 ;;
    -h|--help) sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

# Color solo si la salida es una terminal y no se pidió lo contrario (NO_COLOR).
# Cada zona tiene su color: opciones (amarillo/verde) · petición (cian) · respuesta del motor (magenta).
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  NEGRITA=$'\033[1m'; TENUE=$'\033[2m'; RESET=$'\033[0m'
  ROJO=$'\033[31m'; VERDE=$'\033[32m'; AMARILLO=$'\033[33m'; CIAN=$'\033[36m'; MAGENTA=$'\033[35m'
else
  NEGRITA=""; TENUE=""; RESET=""; ROJO=""; VERDE=""; AMARILLO=""; CIAN=""; MAGENTA=""
fi

# seccion <color> <título>: banda que separa las tres zonas de la demo.
seccion() {
  printf '\n%s%s━━ %s %s%s\n' "$NEGRITA" "$1" "$2" "$(printf '━%.0s' $(seq 1 $((60 - ${#2}))))" "$RESET"
}

# Lee una respuesta solo si hay terminal; si no, o si viene vacía, queda vacía (= el defecto).
leer_respuesta() {
  RESPUESTA=""
  if [ -t 0 ]; then read -r -p "${AMARILLO}${1}${RESET}" RESPUESTA || true; fi
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
  printf '\n%s%s%s\n' "$NEGRITA" "$titulo" "$RESET"
  for i in "${!valores[@]}"; do
    printf '  %s%d)%s %s\n' "$AMARILLO" "$((i + 1))" "$RESET" "${ETIQUETAS[$i]}"
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
  printf '  %s%s no es una opción: se usa %s%s\n' "$ROJO" "'$RESPUESTA'" "$ELEGIDO" "$RESET"
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
ETIQUETAS_AMBIENTES=("${ETIQUETAS[@]}")

# Pasos disponibles: los directorios de steps/, sin el prefijo de orden (01-preparar → preparar).
pasos=()
for dir in "$EJEMPLO"/steps/*/; do
  nombre="$(basename "$dir")"
  pasos+=("${nombre#[0-9][0-9]-}")
done
[ "${#pasos[@]}" -gt 0 ] || { echo "no hay pasos en $EJEMPLO/steps" >&2; exit 1; }

# Operaciones de vexd (vexd sin argumentos las lista), con lo que hace cada una.
operaciones=(intentar rollback simular lanzar reservar liberar diagnosticar abandonar intento intentos despliegues logs)
DESCRIPCIONES=(
  "ejecutar el pipeline hasta un paso en un ambiente"
  "volver a un despliegue anterior"
  "simular un intento hasta un paso, sin efectos"
  "hacer visible un despliegue"
  "reservar un ambiente (no se lanza solo)"
  "liberar un ambiente reservado"
  "por qué falló un intento"
  "dar por perdido un intento sin desenlace"
  "consultar un intento"
  "consultar los intentos de un ambiente"
  "consultar los despliegues de un ambiente"
  "ver la salida de los comandos de un intento"
)

en_lista() { local x="$1" e; shift; for e in "$@"; do [ "$e" = "$x" ] && return 0; done; return 1; }

# pedir_texto <variable> <pregunta> <obligatorio: 1|0>: si la variable viene vacía, la pregunta. Los
# identificadores (intento, despliegue) los da el motor en sus respuestas: se copian de ahí.
pedir_texto() {
  [ -z "${!1}" ] || return 0
  leer_respuesta "$2: "
  if [ -z "$RESPUESTA" ] && [ "$3" = 1 ]; then
    echo "${ROJO}$2 es obligatorio para '$OPERACION'${RESET}" >&2; exit 2
  fi
  printf -v "$1" '%s' "$RESPUESTA"
}

seccion "$AMARILLO" "OPCIONES"
if [ -z "$OPERACION" ]; then
  ETIQUETAS=()
  for i in "${!operaciones[@]}"; do ETIQUETAS+=("${operaciones[$i]} — ${DESCRIPCIONES[$i]}"); done
  elegir "Operaciones disponibles:" "${operaciones[@]}"
  OPERACION="$ELEGIDO"
fi
en_lista "$OPERACION" "${operaciones[@]}" || { echo "${ROJO}operación desconocida: $OPERACION (${operaciones[*]})${RESET}" >&2; exit 2; }

if en_lista "$OPERACION" intentar simular lanzar reservar liberar diagnosticar intentos despliegues && [ -z "$AMBIENTE" ]; then
  ETIQUETAS=("${ETIQUETAS_AMBIENTES[@]}")
  elegir "Ambientes disponibles:" "${ambientes[@]}"
  AMBIENTE="$ELEGIDO"
fi
if en_lista "$OPERACION" intentar simular && [ -z "$HASTA_PASO" ]; then
  ETIQUETAS=("${pasos[@]}")
  elegir "Pasos disponibles (se ejecuta o simula hasta el elegido):" "${pasos[@]}"
  HASTA_PASO="$ELEGIDO"
fi
case "$OPERACION" in
  rollback)     pedir_texto DESPLIEGUE "Despliegue al que volver (id)" 1 ;;
  lanzar)       pedir_texto DESPLIEGUE "Despliegue a lanzar (id)" 1
                pedir_texto NOMBRE "Nombre del lanzamiento (Enter: la versión)" 0 ;;
  diagnosticar) pedir_texto INTENTO "Intento que falló (Enter: el último del ambiente)" 0
                pedir_texto REFERENCIA "Despliegue de referencia (Enter: lo elige el motor)" 0 ;;
  abandonar|intento) pedir_texto INTENTO "Intento (id)" 1 ;;
  logs)         pedir_texto INTENTO "Intento (Enter: el último)" 0
                pedir_texto RESULTADO "Resultado (exitoso o fallido; Enter: todos)" 0 ;;
esac
if [ -z "$LIMPIO" ]; then
  pregunta_si_no "¿Empezar sin historial?" 0
  LIMPIO="$RESPUESTA_SI_NO"
fi
if [ -z "$COMPILAR" ]; then
  pregunta_si_no "¿Compilar vexd?" 1
  COMPILAR="$RESPUESTA_SI_NO"
fi

si_no() { [ "$1" = 1 ] && echo sí || echo no; }
FILAS_ETIQUETA=(); FILAS_VALOR=()
elegida() { [ -z "$2" ] || { FILAS_ETIQUETA+=("$1"); FILAS_VALOR+=("$2"); }; }

# relleno <texto> <ancho>: el texto con espacios hasta el ancho (cuenta caracteres, no bytes).
relleno() { printf '%s%*s' "$1" "$(($2 - ${#1}))" ""; }
# linea <n>: n veces el trazo horizontal del cuadro.
linea() { printf '─%.0s' $(seq 1 "$1"); }

# cuadro <título>: las filas acumuladas por elegida, en un cuadro alineado.
cuadro() {
  local titulo="$1" i ancho_e=0 ancho_v=0 ancho
  for i in "${!FILAS_ETIQUETA[@]}"; do
    [ "${#FILAS_ETIQUETA[$i]}" -le "$ancho_e" ] || ancho_e=${#FILAS_ETIQUETA[$i]}
    [ "${#FILAS_VALOR[$i]}" -le "$ancho_v" ] || ancho_v=${#FILAS_VALOR[$i]}
  done
  ancho=$((ancho_e + ancho_v + 5))                      # ' etiqueta │ valor '
  [ "$ancho" -ge $((${#titulo} + 2)) ] || ancho=$((${#titulo} + 2))
  ancho_v=$((ancho - ancho_e - 5))                      # el valor absorbe lo que sobra del título
  printf '\n%s┌%s┐%s\n' "$VERDE" "$(linea "$ancho")" "$RESET"
  printf '%s│%s %s%s%s %s│%s\n' "$VERDE" "$RESET" "$NEGRITA$VERDE" "$(relleno "$titulo" $((ancho - 2)))" "$RESET" "$VERDE" "$RESET"
  printf '%s├%s┤%s\n' "$VERDE" "$(linea "$ancho")" "$RESET"
  for i in "${!FILAS_ETIQUETA[@]}"; do
    printf '%s│%s %s%s%s %s│%s %s %s│%s\n' "$VERDE" "$RESET" \
      "$VERDE" "$(relleno "${FILAS_ETIQUETA[$i]}" "$ancho_e")" "$RESET" \
      "$VERDE" "$RESET" "$(relleno "${FILAS_VALOR[$i]}" "$ancho_v")" "$VERDE" "$RESET"
  done
  printf '%s└%s┘%s\n' "$VERDE" "$(linea "$ancho")" "$RESET"
}

elegida operación "$OPERACION"
elegida ambiente "$AMBIENTE"
en_lista "$OPERACION" intentar simular && elegida "hasta paso" "$HASTA_PASO"
elegida despliegue "$DESPLIEGUE"
elegida intento "$INTENTO"
elegida nombre "$NOMBRE"
elegida referencia "$REFERENCIA"
elegida limpiar "$(si_no "$LIMPIO")"
elegida compilar "$(si_no "$COMPILAR")"
cuadro "Opciones elegidas"

VEXD="$RAIZ/vexd"

seccion "$CIAN" "PETICIÓN"

# 1. Compilar
if [ "$COMPILAR" = 1 ]; then
  echo "${TENUE}compilando $VEXD${RESET}"
  (cd "$RAIZ" && go build -o vexd ./cmd/vexd)
fi
[ -x "$VEXD" ] || { echo "${ROJO}no existe $VEXD: responde que sí a compilar${RESET}" >&2; exit 1; }
echo "${TENUE}versión de vexd: $("$VEXD" version)${RESET}"

# 2. Preparar directorios y repos (el del pipeline solo se crea si no existe: así se puede editar entre pruebas)
[ "$LIMPIO" = 1 ] && rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"/{almacen,espacio,material}

git_demo() { git -C "$1" -c user.name=demo -c user.email=demo@vex.test "${@:2}"; }

if [ ! -d "$DEMO_DIR/proyecto/.git" ]; then
  echo "${TENUE}creando el repo del proyecto${RESET}"
  mkdir -p "$DEMO_DIR/proyecto"
  echo "mi app" > "$DEMO_DIR/proyecto/README.md"
  git -C "$DEMO_DIR/proyecto" init -q
  git_demo "$DEMO_DIR/proyecto" add -A
  git_demo "$DEMO_DIR/proyecto" commit -qm inicial
fi
if [ ! -d "$DEMO_DIR/pipeline/.git" ]; then
  echo "${TENUE}creando el repo del pipeline (copia de internal/ejecucion/testdata/ejemplo)${RESET}"
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

# 3. Ejecutar la operación: la petición se arma con las opciones elegidas, se muestra y se envía tal cual.
# Cada operación tiene sus campos (docs/guia-de-pruebas.md); los opcionales vacíos se omiten.
SOLICITANTE="${USER:-demo}"
METADATOS='{ "ProjectName": "vex-demo", "ProjectId": "p1" }'
case "$OPERACION" in
  intentar)
    CAMPOS="\"Ambiente\": \"$AMBIENTE\", \"Solicitante\": \"$SOLICITANTE\",
  \"FuenteDelProyecto\": \"$DEMO_DIR/proyecto\", \"FuenteDelPipeline\": \"$DEMO_DIR/pipeline\",
  \"HastaPaso\": \"$HASTA_PASO\", \"Metadatos\": $METADATOS" ;;
  rollback)
    CAMPOS="\"Despliegue\": \"$DESPLIEGUE\", \"Solicitante\": \"$SOLICITANTE\", \"Metadatos\": $METADATOS" ;;
  simular)
    CAMPOS="\"Ambiente\": \"$AMBIENTE\", \"Solicitante\": \"$SOLICITANTE\",
  \"HastaPaso\": \"$HASTA_PASO\", \"CopiaDeTrabajo\": \"$DEMO_DIR/pipeline\",
  \"Metadatos\": $METADATOS" ;;
  lanzar)
    CAMPOS="\"Ambiente\": \"$AMBIENTE\", \"Despliegue\": \"$DESPLIEGUE\""
    if [ -n "$NOMBRE" ]; then CAMPOS="$CAMPOS, \"Nombre\": \"$NOMBRE\""; fi ;;
  reservar|liberar|intentos|despliegues)
    CAMPOS="\"Ambiente\": \"$AMBIENTE\"" ;;
  diagnosticar)
    CAMPOS="\"Ambiente\": \"$AMBIENTE\""
    if [ -n "$INTENTO" ]; then CAMPOS="$CAMPOS, \"Intento\": \"$INTENTO\""; fi
    if [ -n "$REFERENCIA" ]; then CAMPOS="$CAMPOS, \"Referencia\": \"$REFERENCIA\""; fi ;;
  abandonar|intento)
    CAMPOS="\"Intento\": \"$INTENTO\"" ;;
  logs)
    CAMPOS="\"Intento\": \"$INTENTO\", \"Resultado\": \"$RESULTADO\"" ;;
esac
PETICION="{
  \"Version\": \"1\",
  $CAMPOS
}"
printf '\n%svexd %s%s  %s(la petición JSON entra por la entrada estándar)%s\n' "$CIAN$NEGRITA" "$OPERACION" "$RESET" "$TENUE" "$RESET"
printf '%s%s%s\n' "$CIAN" "$PETICION" "$RESET"

seccion "$MAGENTA" "RESPUESTA DEL MOTOR"
printf '%sstdout: la respuesta · stderr: los errores%s\n\n' "$TENUE" "$RESET"
set +e
"$VEXD" "$OPERACION" \
  --almacen "$DEMO_DIR/almacen" \
  --espacio "$DEMO_DIR/espacio" \
  --material "$DEMO_DIR/material" <<<"$PETICION"
codigo=$?
set -e

color_codigo="$ROJO"
[ "$codigo" = 0 ] && color_codigo="$VERDE"
seccion "$color_codigo" "RESULTADO"
printf 'código de salida: %s%s%s   %s(0 bien · 1 falló · 2 petición inválida · 130 cancelado)%s\n' \
  "$color_codigo$NEGRITA" "$codigo" "$RESET" "$TENUE" "$RESET"
printf '%spara seguir: source %s/entorno.sh   y mira docs/guia-de-pruebas.md%s\n' "$TENUE" "$DEMO_DIR" "$RESET"
exit "$codigo"
