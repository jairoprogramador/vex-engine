# Regla de dependencias del motor (IT-05 DEC-05.2). Se lee junto a modelo/arquitectura.md.
#
# Entrada: una línea por paquete, «paquete import1 import2 ...», como la da go list.
# Uso, desde ecosistema/vex-engine:
#
#   go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./... \
#     | awk -v mod="$(go list -m)" -f docs/modelo/reglas-dependencias.awk
#
# Imprime cada importación que rompe la regla, con la razón, y sale con 1 si hay alguna.

BEGIN {
  raiz = mod "/"
  pre = mod "/internal/"

  # Contextos de arriba de cada contexto, según modelo/context-map.md.
  arriba["diagnostico"] = "historial"
  arriba["lanzamiento"] = "historial"
  arriba["ejecucion"]   = "historial definicion resolucion suministro"
  arriba["resolucion"]  = "historial definicion"
  arriba["simulacion"]  = "definicion resolucion suministro"
  arriba["definicion"]  = "suministro"

  # Contextos que el borde expone al CLI y al portal.
  entrada = "ejecucion simulacion lanzamiento diagnostico historial"

  fallos = 0
}

function parte(p, n,   r, a) { r = substr(p, length(pre) + 1); split(r, a, "/"); return a[n] }
function en(x, lista,   v, i, k) { k = split(lista, v, " "); for (i = 1; i <= k; i++) if (v[i] == x) return 1; return 0 }
function falla(de, hacia, por) { print de " -> " hacia ": " por; fallos++ }

index($1, pre) == 1 {
  c = parte($1, 1); l = parte($1, 2)
  for (i = 2; i <= NF; i++) {
    # Del propio módulo, lo nuevo solo usa lo nuevo: nunca old-internal ni cmd (RD-01).
    if (index($i, raiz) == 1 && index($i, pre) != 1) {
      falla($1, $i, "el código nuevo no usa el código antiguo")
      continue
    }
    if (index($i, pre) != 1) {
      if ((l == "dominio" || l == "publicado" || l == "reservado") && $i ~ /^[^\/]+\./)
        falla($1, $i, "el dominio y lo publicado solo usan la biblioteca estándar")
      continue
    }
    u = parte($i, 1); m = parte($i, 2)
    if (u == "borde" && c != "borde")
      falla($1, $i, "solo cmd/vexd usa el borde")
    else if (c == u) {
      if (l == "publicado" || l == "reservado")
        falla($1, $i, "lo publicado no depende de nada interno")
      else if (l == "dominio" && m != "dominio")
        falla($1, $i, "el dominio solo depende de su propio dominio")
    }
    else if (c == "borde") {
      if (m != "publicado" || !en(u, entrada))
        falla($1, $i, "el borde solo usa lo publicado de un contexto de entrada")
    }
    else if (l == "dominio" || l == "publicado" || l == "reservado")
      falla($1, $i, "el dominio y lo publicado no ven otros contextos")
    else if (!en(u, arriba[c]))
      falla($1, $i, u " no está arriba de " c " en el context map")
    else if (!(m == "publicado" || (m == "reservado" && c == "resolucion" && u == "historial")))
      falla($1, $i, "de otro contexto solo se usa lo publicado")
  }
}

END { exit (fallos > 0) }
