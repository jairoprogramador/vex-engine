package fingerprint

import (
	"path"
	"strings"
)

// ignoreRule es una línea de un .gitignore ya interpretada.
//
// domain son los componentes del directorio que contiene el archivo de reglas,
// relativos a la raíz: una regla sólo se evalúa sobre rutas que caen dentro de
// su dominio.
type ignoreRule struct {
	pattern   string
	domain    []string
	dirOnly   bool
	inclusion bool
	anchored  bool
}

// ignoreDB son todas las reglas del árbol, en el orden en que el recorrido las
// encontró.
type ignoreDB struct {
	rules []ignoreRule
}

// parseIgnoreRules interpreta el contenido de un .gitignore.
//
// Escapes soportados (SPEC-v1.md §4.3): una barra invertida delante de un
// carácter lo vuelve literal. Eso afecta a tres decisiones que se toman aquí
// —comentario, negación y recorte de espacios finales— y al resto lo interpreta
// el motor de globs, que ya trata "\x" como el literal "x".
func parseIgnoreRules(content string, domain []string) []ignoreRule {
	var rules []ignoreRule

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		line = trimUnescapedTrailingSpace(line)

		// Un "#" escapado no abre un comentario; tampoco lo hace un "#" que no
		// esté al principio.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := ignoreRule{domain: domain}

		if strings.HasPrefix(line, "!") {
			rule.inclusion = true
			line = line[1:]
		}

		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = line[:len(line)-1]
		}

		if strings.HasPrefix(line, "/") {
			rule.anchored = true
			line = line[1:]
		} else if strings.Contains(line, "/") {
			rule.anchored = true
		}

		rule.pattern = line
		rules = append(rules, rule)
	}

	return rules
}

// trimUnescapedTrailingSpace recorta los espacios y tabuladores finales que no
// estén escapados. `foo\ ` es el patrón «foo » con su espacio; `foo   ` es
// «foo».
func trimUnescapedTrailingSpace(line string) string {
	end := len(line)
	for end > 0 {
		c := line[end-1]
		if c != ' ' && c != '\t' {
			break
		}
		if isEscaped(line, end-1) {
			break
		}
		end--
	}
	return line[:end]
}

// isEscaped indica si el carácter en la posición i viene precedido por un número
// impar de barras invertidas.
func isEscaped(s string, i int) bool {
	backslashes := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		backslashes++
	}
	return backslashes%2 == 1
}

// matches decide si una ruta queda fuera de la huella.
//
// Gana la última regla que casa en el orden de acumulación, que es el orden del
// recorrido. Git le daría prioridad al .gitignore más profundo; la v1 no
// (divergencia (a), congelada y documentada).
func (db *ignoreDB) matches(pathComponents []string, isDir bool) bool {
	result := false

	for _, rule := range db.rules {
		if !pathIsUnderDomain(pathComponents, rule.domain) {
			continue
		}

		if rule.dirOnly && !isDir {
			continue
		}

		relative := pathComponents[len(rule.domain):]

		if matchPattern(rule.pattern, relative, rule.anchored) {
			result = !rule.inclusion
		}
	}

	return result
}

func pathIsUnderDomain(pathComponents, domain []string) bool {
	if len(pathComponents) <= len(domain) {
		return false
	}
	for i, d := range domain {
		if pathComponents[i] != d {
			return false
		}
	}
	return true
}

// matchPattern evalúa un patrón contra los componentes de una ruta.
//
// Se usa path.Match y no filepath.Match a propósito: filepath.Match desactiva
// los escapes cuando el separador del sistema es "\", así que la misma regla
// daría huellas distintas según el sistema operativo. Una primitiva de identidad
// no puede depender de eso.
//
// Un patrón que no es un glob válido no casa con nada (SPEC-v1.md §4.5).
func matchPattern(pattern string, relative []string, anchored bool) bool {
	if len(relative) == 0 {
		return false
	}

	if strings.Contains(pattern, "**") {
		return matchDoubleGlobParts(strings.Split(pattern, "/"), relative)
	}

	if anchored {
		// Divergencia (b), congelada: un patrón anclado compara sólo el prefijo
		// de componentes, así que "/build" casa también con "build/x/y".
		return matchGlobSequence(strings.Split(pattern, "/"), relative)
	}

	for i := range relative {
		if ok, err := path.Match(pattern, relative[i]); err == nil && ok {
			return true
		}
	}
	return false
}

func matchDoubleGlobParts(patParts, pathParts []string) bool {
	if len(patParts) == 0 {
		return len(pathParts) == 0
	}

	if patParts[0] == "**" {
		for i := 0; i <= len(pathParts); i++ {
			if matchDoubleGlobParts(patParts[1:], pathParts[i:]) {
				return true
			}
		}
		return false
	}

	if len(pathParts) == 0 {
		return false
	}

	ok, err := path.Match(patParts[0], pathParts[0])
	if err != nil || !ok {
		return false
	}

	return matchDoubleGlobParts(patParts[1:], pathParts[1:])
}

func matchGlobSequence(patParts, pathParts []string) bool {
	if len(patParts) > len(pathParts) {
		return false
	}
	for i, p := range patParts {
		ok, err := path.Match(p, pathParts[i])
		if err != nil || !ok {
			return false
		}
	}
	return true
}
