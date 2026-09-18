# Plan de Ejecución - Clean Code + 5S

## Resumen del Análisis
- **38+ Problemas** encontrados en el dominio
- **3,045 líneas** de código en 7 archivos
- **Duplicación crítica**: Validadores definidos 2 veces
- **Nombres confusos**: Variables con 1-2 caracteres

## Fases de Refactorización

### FASE 1: CLEANUP (Limpieza Inmediata) - Semana 1
**Objetivo:** Remover duplicación, extraer constantes, estandarizar nombres

#### 1.1 - Eliminar Código Duplicado
- [ ] Decidir: Mantener validadores.go O comprobacion.go
  - **Decisión**: Mantener AMBOS pero unificar:
    - `validadores.go`: Interfaces, tipos públicos
    - `comprobacion.go`: Orquestación, métodos de validación
- [ ] Refactorizar `Comprobar()` para usar interfaz `Validador`
- [ ] Remover métodos duplicados

#### 1.2 - Extraer Constantes
```go
// valores.go → agregar:
const (
    DirVariables = "variables/"
    DirSteps = "steps/"
    FileConfig = "config.yaml"
    FileEnvironments = "environments.yaml"
    FileCommands = "commands.yaml"
)
```

#### 1.3 - Renombrar Variables Ambiguas (CRÍTICO)
| Antes | Después | Scope |
|-------|---------|-------|
| `c` | `comp` | Métodos de comprobacion |
| `v` | `validador` | Métodos de validadores |
| `p` | `paso/problema` | Contexto-dependiente |
| `m` | `matches` | Regex match |
| `s` | `salida` | Variables de salida |
| `d` | `variable` | Variable declarada |
| `lista` | `targetList` | valores.go línea 62 |
| `limpia` | `cleanPath` | valores.go línea 74 |

Herramienta: `grep -rn '\bc\s*\*comprobacion' --include="*.go"` para localizar

#### 1.4 - Documentación Mínima (Comenta Intención)
- [ ] Agregar comment a `Comprobar()` explicando orden de validación
- [ ] Documentar invariantes de `PipelineComprobado`
- [ ] Documentar `Ambito.Ve()` → renombrar si es confuso

### FASE 2: REORGANIZACIÓN (Sistematización) - Semana 2
**Objetivo:** Estructura clara, métodos bien organizados

#### 2.1 - Reorganizar comprobacion.go
```
1. Tipos privados (comprobacion, procesadorXXX)
2. Métodos de comprobacion (validación)
3. Métodos de procesadores
4. Funciones globales privadas
5. Tests
```

#### 2.2 - Crear Archivo detectador_circulos.go
- [ ] Mover `detectadorCirculos` struct (línea 883)
- [ ] Mover métodos asociados
- [ ] Mantener en paquete `dominio`

#### 2.3 - Consolidar constantes en valores.go
- [ ] Mover strings hardcodeados a `const`
- [ ] Agregar sección clara de constantes al inicio

#### 2.4 - Estandarizar Receiver Names
```go
// Antes confuso:
func (c *comprobacion) version()
func (v *validadorAmbientes) validar()
func (p *procesadorComandos) procesar()
func (p *problemas) anotar()  // ¡CONFLICTO!

// Después claro:
func (comp *comprobacion) version()
func (vam *validadorAmbientes) validar()
func (pc *procesadorComandos) procesar()
func (probs *problemas) anotar()
```

### FASE 3: SIMPLIFICACIÓN (SRP) - Semana 3
**Objetivo:** Funciones con una responsabilidad

#### 3.1 - Refactorizar Funciones Grandes
| Función | Acción | Líneas |
|---------|--------|--------|
| `Comprobar()` | ✅ Ya descompuesta | - |
| `pasos()` | Dividir en sub-métodos | 170-182 |
| `usos()` | Dividir: `validarValores()` + `validarVisibilidad()` | 728-748 |
| `usosEnLosPasos()` | Clarificar con métodos nombrados | 755-761 |

#### 3.2 - Eliminar Abstracciones Innecesarias
- [ ] `(c *comprobacion).material()` → ¿inline o mantener?
- [ ] `procesadorComandos.procesarPlantillas()` → ¿simplificar?
- [ ] Struct `problemas` → ¿inline en `usosEnLosPasos()`?

#### 3.3 - Precachear en lugar de O(n) búsquedas
```go
// Problema línea 72:
func (c *comprobacion) ilegible(fichero string) bool {
    return slices.ContainsFunc(c.pipelineDeclarado.Ilegibles, ...)  // O(n) cada vez!
}

// Solución:
type comprobacion struct {
    ...
    mapaIlegibles map[string]bool  // precacheado
}
```

### FASE 4: ESTANDARIZACIÓN (Seiketsu) - Semana 4
**Objetivo:** Consistencia en todo el dominio

#### 4.1 - Unificar Manejo de Errores
```go
// Todas las funciones retornan error (nil = OK):
func (comp *comprobacion) version() error
func (vam *validadorAmbientes) validar() error
```

#### 4.2 - Estandarizar Comentarios
- [ ] Públicos: `// Sentence case.`
- [ ] Privados: `// brief explanation.`
- [ ] Métodos: `// (v *Type) Method explanation.`

#### 4.3 - Unificar Idioma (Español)
- [ ] Mantener consistencia: tipos/métodos en español
- [ ] Renombrar: `DeHoy()` → `ObtenerDelDia()`
- [ ] Renombrar: `Ve()` → `PuedeVer()`

#### 4.4 - Validación en Bordes
- [ ] Agregar precondiciones documentadas
- [ ] Agregar postcondiciones
- [ ] Validar inputs desde `Comprobar()`

### FASE 5: DISCIPLINA (Shitsuke) - Semana 5+
**Objetivo:** Mantener mejoras, mejorar continuamente

#### 5.1 - Tests
- [ ] Agregar tests para interpolación
- [ ] Agregar tests para extracción de variables
- [ ] Consolidar tests en comprobacion_test.go

#### 5.2 - Documentación
- [ ] README del dominio
- [ ] Invariantes claras
- [ ] Guía de extensión

#### 5.3 - Linting
- [ ] golangci-lint checks
- [ ] Code review checklist

## Orden de Ejecución Recomendado

```
Semana 1 (CLEANUP):
1. Extraer constantes (impacto: 10 lines, bajo riesgo)
2. Renombrar c→comp, p→paso/problema (alto impacto, riesgo bajo si tests pasan)
3. Documentar Comprobar() orden
4. Tests: verificar todo sigue funcionando

Semana 2 (REORGANIZACIÓN):
5. Reorganizar comprobacion.go
6. Crear detectador_circulos.go
7. Estandarizar receivers
8. Tests nuevamente

Semana 3 (SIMPLIFICACIÓN):
9. Refactorizar pasos(), usos()
10. Precachear ilegibles
11. Tests

Semana 4 (ESTANDARIZACIÓN):
12. Unificar error handling
13. Unificar idioma
14. Estandarizar comentarios
15. Tests

Semana 5+ (DISCIPLINA):
16. Tests de interpolación
17. Documentación
18. Review y mejoras continuas
```

## Commits Esperados

```
Commit 1: refactor: extraer constantes del dominio
Commit 2: refactor: renombrar variables ambiguas (c→comp, p→paso/problema)
Commit 3: refactor: organizar comprobacion.go según responsabilidad
Commit 4: refactor: crear detectador_circulos.go
Commit 5: refactor: estandarizar receiver names
Commit 6: refactor: refactorizar funciones grandes
Commit 7: refactor: precachear ilegibles para O(1)
Commit 8: refactor: unificar error handling a tipos error
Commit 9: refactor: estandarizar idioma español en dominio
Commit 10: refactor: completar test coverage (interpolación)
Commit 11: docs: agregar documentación de invariantes
```

## Métricas de Éxito

| Métrica | Antes | Después | Target |
|---------|-------|---------|--------|
| Variables con 1-2 chars | 20+ | 0 | ✓ |
| Funciones > 50 líneas | 8 | <5 | ✓ |
| Código duplicado | ~200 líneas | 0 | ✓ |
| Cyclomatic complexity máx | ~15 | <10 | ✓ |
| Test coverage | ~95% | >98% | ✓ |
| Documentación de invariantes | 0% | 100% | ✓ |

## Riesgo Mitigation

- ✅ Tests existentes (52 pasos) = red carpet
- ✅ Refactorización por fases (no big bang)
- ✅ Cada commit es atómico y testeable
- ✅ Rename-refactoring seguro con IDE (GoLand/VS Code)

## Estado: LISTO PARA EJECUTAR

El análisis es completo. Plan es claro. Tests van a validar cada paso.

**¿Empezamos con Fase 1 (CLEANUP)?**
