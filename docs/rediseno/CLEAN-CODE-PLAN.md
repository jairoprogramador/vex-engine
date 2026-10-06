# Plan de Código Limpio - Robert C. Martin + 5S

## Objetivo
Aplicar principios de Clean Code y metodología 5S al dominio `internal/definicion/dominio/`

## Fechas
- Inicio: 2026-01-16
- Estado: En análisis

## Ámbito
Archivos del dominio:
- `comprobacion.go` (refactorizado)
- `declaracion.go` 
- `errores.go`
- `pipeline.go`
- `puertos.go`
- `valores.go`
- `validadores.go` (nuevo)
- Tests asociados

## Principios a Aplicar

### 1. NOMBRES (Robert C. Martin - Clean Code Cap. 2)
- [ ] Usar nombres descriptivos e intencionados
- [ ] Evitar desinformación
- ] Usar nombres pronunciables
- [ ] Usar nombres buscables
- [ ] Evitar code smell: abreviaciones confusas
- [ ] Un concepto, un nombre (consistencia)

### 2. FUNCIONES (Robert C. Martin - Clean Code Cap. 3)
- [ ] Función = Una cosa (Single Responsibility)
- [ ] Nombres que describan la intención
- [ ] Mínimo de parámetros (< 3)
- [ ] Sin parámetros de bandera (flags)
- [ ] Sin efectos secundarios
- [ ] Evitar duplicación

### 3. ORGANIZACIÓN - 5S SEIRI
- [ ] Identificar y remover código muerto
- [ ] Remover funciones sin uso
- [ ] Remover campos de structs sin uso
- [ ] Remover imports innecesarios
- [ ] Remover comentarios obsoletos

### 4. SISTEMATIZACIÓN - 5S SEITON
- [ ] Organizar funciones por lógica
- [ ] Agrupar tipos relacionados
- [ ] Estructura consistente en archivos
- [ ] Orden claro: tipos → constantes → variables → funciones

### 5. LIMPIEZA - 5S SEISO
- [ ] Remover comentarios redundantes
- [ ] Remover código de debugging
- [ ] Extraer valores mágicos a constantes
- [ ] Simplificar lógica innecesariamente compleja

### 6. ESTANDARIZACIÓN - 5S SEIKETSU
- [ ] Convención de nombres consistente
- [ ] Patrones de error uniformes
- [ ] Estructura de structs uniforme
- [ ] Comentarios formato estándar

### 7. DISCIPLINA - 5S SHITSUKE
- [ ] Resolver TODOs y FIXMEs
- [ ] Identificar y manejar errores potenciales
- [ ] Validación de inputs
- [ ] Documentación consistente

## Fases

### Fase 1: Análisis (En curso)
- [ ] Revisar todos los archivos
- [ ] Identificar problemas por categoría
- [ ] Priorizar cambios

### Fase 2: Refactorización por Prioridad
- [ ] CRÍTICO: Renombraciones que afectan claridad
- [ ] ALTO: Remover código muerto, funciones duplicadas
- [ ] MEDIO: Estandarización, organización
- [ ] BAJO: Documentación, comentarios

### Fase 3: Validación
- [ ] Tests existentes siguen pasando
- [ ] Compilación sin errores
- [ ] Code review de cambios

## Métricas

### Antes
- [ ] Complejidad cognitiva del dominio
- [ ] Número de funciones > 30 líneas
- [ ] Código duplicado
- [ ] Nombres poco claros

### Después
- [ ] Reducción de complejidad
- [ ] Funciones más pequeñas
- [ ] Código DRY
- [ ] Nombres cristalinos

## Resultado Esperado

Dominio que cumple:
- ✅ Clean Code: Nombres, Funciones, Comentarios
- ✅ 5S: Organizado, limpio, estandarizado
- ✅ SOLID: Principios aplicados
- ✅ Testeable: Fácil de verificar
- ✅ Mantenible: Fácil de cambiar

## Estado de Commits

```
Commit Base: 572ee10 (fix: corregir test AST)
En construcción: Plan de Clean Code
```
