# Problemas Identificados - Análisis Manual

## NOMBRES (Confusos / Poco Descriptivos)

### errores.go
- ❌ `Invariante` - ¿Qué es realmente? Debería ser `ValidacionRule` o `ValidationCategory`
- ❌ `Fallo` - Poco descriptivo. Debería ser `ValidationError` o `CheckError`
- ❌ Campo `Paso` en Fallo - ¿Step name? Sin documenta ción

### valores.go
- ❌ `Ambito` - ¿Scope? Sin documentación clara
- ❌ `lista` (línea 62) - Muy vago. Debería ser `targetList` o `outputList`
- ❌ `limpia` (línea 74) - ¿cleanedPath? Inconsistencia con naming
- ❌ `DelPaso` (línea 30) - ¿ofStep? Poco claro, campo sin context
- ❌ `patronDirectorioDePaso` - ¿StepDirPattern? Mezcla idiomas

### puertos.go
- ❌ `DeHoy` - Mezcla idiomas. ¿GetLatest? ¿GetCurrent? 
- ❌ `DeUnCommit` - Mezcla idiomas. ¿GetForCommit?
- ❌ `fuente` - ¿source? ¿origin? Poco claro

### comprobacion.go
- ❌ `c` - Abreviación de comprobacion, confusa sin contexto
- ❌ `v` - Variables sin nombres descriptivos (en validadores)
- ❌ `p` - En problemas, poco claro

## FUNCIONES (Demasiadas responsabilidades)

### comprobacion.go
- ⚠️ `Comprobar()` - Orquesta demasiados validadores (aunque refactorizado)
- ⚠️ `usos()` en valores.go - Mezcla búsqueda y clasificación

### validadores.go  
- ⚠️ `ValidadorUsos.Validar()` - Hace demasiadas cosas (circulos, valores, pasos)

## ORGANIZACIÓN (Código muerto / Desorganizado)

### comprobacion.go
- ❓ Verificar métodos sin uso después de refactorización
- ❓ Funciones `necesita()`, `declaradaVisible()` - ¿Podrían moverse?

### validadores.go
- ⚠️ Funciones helper sin agrupar
- ⚠️ `detectadorCirculos` - Debería estar en su propia sección

## ESTANDARIZACIÓN (Inconsistente)

- ❌ Mezcla de español/inglés en nombres
  - `DeHoy`, `DeUnCommit` vs `Comprobar`, `FallosDeComprobacion`
  - `Ambito`, `Regla` vs `Pipelines`, `Context`
  
- ❌ Métodos con prefijo `Es` inconsistentes
  - `EsCompartido()` vs `esEstandar()` (casos inconsistentes)

- ❌ Parámetros cortos inconsistentes
  - Líneas 62, 74 usan nombres vagos
  - Debería haber pauta clara

## LIMPIEZA (Código obsoleto / Complejo)

- ⚠️ Comentarios en español sin traducción
- ⚠️ Línea 62: Uso de `lista := &nombres` es poco idiomático
- ⚠️ Línea 87: `Ve()` es poco descriptivo - ¿`canSee()`? ¿`isVisibleFrom()`?

## PRIORIDAD POR IMPACTO

### CRÍTICO (Claridad del dominio)
1. Renombrar `Invariante` → `ValidationRule`
2. Renombrar `Fallo` → `ValidationFailure` o `CheckFailure`
3. Estandarizar idioma (español O inglés, no ambos)

### ALTO (Legibilidad)
1. Mejorar nombres de métodos: `Ve()` → `canSee()` o `isVisibleFrom()`
2. Renombrar `Ambito` → `Scope` (si es inglés) o `Ámbito` (si es español)
3. Expandir abreviaciones: `c` → `check`, `v` → `validator`, `p` → `problems`

### MEDIO (Organización)
1. Agrupar funciones por responsabilidad
2. Mover `detectadorCirculos` a su propio bloque
3. Documentar campos sin documentación

### BAJO (Estética)
1. Estandarizar comentarios
2. Refactorizar líneas complejas (línea 62, etc.)
3. Mejorar formatting

## Próximos pasos
- Esperar análisis completo del agente
- Combinar con estos hallazgos
- Crear plan de refactorización por prioridad
