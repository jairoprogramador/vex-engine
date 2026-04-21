# Plan de Refactor: `domain/state` → `domain/storage`

## 1. Objetivo y alcance

**Qué cambia:**
- Se renombra el bounded context `internal/domain/state/` a `internal/domain/storage/` y la capa de infraestructura `internal/infrastructure/state/` a `internal/infrastructure/storage/gob/` (conviviendo con el ya existente `storage/filesystem/`).
- Se reemplaza el acoplamiento por `string` (nombres de paso, rutas de archivo) por tipos de dominio (`StepName`, `StorageKey`).
- Se sustituye el combo `BaseMatcher + matchers concretos + factory` por una composición de `Rule`s por paso (Specification Pattern), configurada declarativamente en un catálogo de `StepPolicy`.
- Se mueve la decisión "¿debe ejecutarse?" del orchestrator al dominio (`ExecutionDecider` + aggregate `StepHistory`).
- El DTO de infraestructura deja de silenciar errores al reconstruir VOs.

**Qué NO cambia:**
- El algoritmo SHA-256 de `Sha256FingerprintService` (sólo se mueve y se renombra el paquete).
- El formato `gob` en disco (sólo se renombra el DTO y se endurece la reconstrucción). Se puede evolucionar después sin romper compatibilidad si se conserva la estructura del DTO.
- La semántica de TTL por defecto (30 días).
- La firma pública del `ExecutionOrchestrator` desde el punto de vista HTTP (sólo cambia por dentro qué puerto invoca).

**Criterios de éxito:**
- `go build ./...` y `go test ./...` pasan en cada paso del plan de migración.
- Agregar una regla nueva (ej. `AntivirusScanRule`) requiere 1 archivo nuevo + 1 línea en el catálogo — cero cambios en código existente (Open/Closed).
- `internal/domain/storage/` no importa `os`, `filepath`, ni `encoding/gob`.
- El orchestrator ya no contiene `if !hasChanged { continue }`: delega en `decider.Decide(...)` que retorna una decisión tipada.

## 2. Diagnóstico del estado actual

- Existen 4 matchers concretos (`test/supply/package/deploy`) con una `BaseMatcher.matchCommon` que siempre compara `Instruction+Vars` — invariante implícito, no documentado en el puerto (pain F).
- `NewStateMatcherFactory` hace `switch` sobre `tableName string` (derivado de `filepath.Base(stateTablePath)` sin extensión). Si mañana `test` pasa a `unittest`, fallan silenciosamente todos los callers (pain C).
- `StateManager.HasStateChanged` retorna `(true, err)` cuando la factory falla por nombre inválido — no es un cambio real, es un error de configuración, pero el orchestrator lo trata igual (pain C).
- `StateManager` persiste y decide (pain D) — dos responsabilidades en un servicio de dominio.
- `CachePolicy` se pasa en cada llamada aunque sólo `test` la consume (pain E). La política es conceptualmente *por paso*, no global.
- `SupplyStateMatcher` / `DeployStateMatcher` comparan `Environment` con `==` de strings crudos, no con `Environment.Equals(...)` (pain G).
- `fromDTO` en `state_table_dto.go:53-68` crea `vos.Fingerprint{}` o `vos.Environment{}` vacíos si el valor persistido es inválido, silenciando errores. Un archivo corrupto produce matches espurios (pain A).
- El orchestrator pide fingerprints completos *aunque el paso no los necesite* (por ejemplo `supply` no depende de `code`, pero se calcula igual). Además contiene la lógica `if !hasChanged { continue }` — lógica de negocio filtrada a `application` (pain H).
- La identidad del "estado por paso" viaja como `stateTablePath string` atravesando puertos del dominio (pain I), acoplando el dominio a que la persistencia sea un archivo.
- Los matchers no tienen tests unitarios (pain J); sólo `StateManager` los ejercita indirectamente.
- `StateEntry.SetCreatedAt` es un setter público usado exclusivamente por el DTO de infraestructura — rompe encapsulación.

## 3. Principios del rediseño

- **DDD — Ubiquitous language**: `StorageKey`, `StepHistory`, `ExecutionDecision`, `StepPolicy`, `Rule`. Nada de `table`, `tb`, `matcher` heredados de la terminología técnica.
- **SOLID — Open/Closed via Specification/Composite**: una `Rule` nueva se añade sin modificar otras; las reglas se componen con `AllRules(r1, r2, ...)`.
- **Tell, Don't Ask**: el aggregate `StepHistory` responde `Decide(policy, current, clock)` — el `ExecutionDecider` no itera entries crudas fuera del aggregate.
- **Hexagonal purity**: `internal/domain/storage/` no importa paquetes de `os`, `encoding/gob` ni `filepath`. Los puertos hablan de `StorageKey`, no de paths.
- **Fail loud on corruption**: el DTO de infra propaga errores al reconstruir VOs. Un archivo corrupto aborta la lectura, no devuelve un match vacío.
- **Policy como dato, no como código**: el mapa `StepName → StepPolicy` vive en un catálogo (`services/step_policies.go`) legible de un vistazo y modificable en un sólo sitio.

## 4. Arquitectura propuesta

### 4.1 Estructura de directorios final

```
internal/domain/storage/
├── vos/
│   ├── fingerprint.go           — hash SHA-256 validado, Equals
│   ├── fingerprint_kind.go      — enum: KindCode / KindInstruction / KindVars
│   ├── fingerprint_set.go       — mapa FingerprintKind → Fingerprint (antes CurrentStateFingerprints)
│   ├── environment.go           — VO Environment (Equals coherente)
│   ├── step_name.go             — VO StepName (enum tipado: Test/Supply/Package/Deploy)
│   ├── storage_key.go           — VO StorageKey (ProjectID + TemplateName + StepName)
│   ├── ttl.go                   — VO TTL (antes CachePolicy; nombre positivo del dominio)
│   └── decision.go              — VO ExecutionDecision (Run/Skip + Reason)
├── aggregates/
│   ├── history_entry.go         — antes StateEntry; constructor único, sin setter CreatedAt
│   └── step_history.go          — antes StateTable; aggregate root con Decide(...)
├── ports/
│   ├── history_repository.go    — Get/Save/Append por StorageKey
│   ├── fingerprint_service.go   — FromFile/FromDirectory
│   └── clock.go                 — interfaz Now() time.Time (inyectable, testeable)
├── services/
│   ├── rules/
│   │   ├── rule.go              — interface Rule + composite AllRules
│   │   ├── fingerprint_rule.go  — Rule parametrizada por FingerprintKind
│   │   ├── environment_rule.go  — Rule sobre Environment
│   │   └── ttl_rule.go          — Rule sobre CreatedAt + TTL + Clock
│   ├── step_policies.go         — catálogo: StepName → StepPolicy (reglas + TTL opcional)
│   └── execution_decider.go     — servicio de dominio que orquesta History + Policy
└── errors.go                    — ErrHistoryCorrupted, ErrUnknownStep, etc.

internal/infrastructure/storage/
├── filesystem/                  — (ya existe) execution_repository.go se mantiene
└── gob/
    ├── gob_history_repository.go — antes gob_state_repository.go
    ├── history_dto.go            — antes state_table_dto.go (con errores propagados)
    ├── sha256_fingerprint_service.go — sin cambios de lógica
    └── path_resolver.go          — traduce StorageKey → ruta en .vex/
```

### 4.2 Value Objects

- **`Fingerprint`** (`vos/fingerprint.go`): idéntico al actual. Invariante: valor no vacío; `Equals` por igualdad de string.
- **`FingerprintKind`** (`vos/fingerprint_kind.go`): enum cerrado `KindCode | KindInstruction | KindVars`. Permite que las reglas sean paramétricas en el "qué comparar" sin switch.
- **`FingerprintSet`** (`vos/fingerprint_set.go`): reemplazo de `CurrentStateFingerprints`. API: `NewFingerprintSet(map[FingerprintKind]Fingerprint, Environment)`, `Get(kind) (Fingerprint, bool)`, `Environment() Environment`. Invariante: cualquier `Fingerprint` presente en el mapa es válido; ausencias son legítimas (p. ej. `supply` puede omitir `KindCode`).
- **`Environment`** (`vos/environment.go`): idéntico al actual. Uso consistente de `Equals` en todas las reglas.
- **`StepName`** (`vos/step_name.go`): tipo tipado (`type StepName string`) con constructor `NewStepName(string) (StepName, error)` que valida contra el conjunto conocido. Elimina el switch sobre strings en la factory actual.
- **`StorageKey`** (`vos/storage_key.go`): identidad abstracta del historial de un paso. Campos: `projectName`, `templateName`, `step StepName`. Inmutable; tiene `Equals`. Justificación: el dominio pregunta "¿dónde está el historial del paso X del proyecto Y con pipeline Z?" sin saber que es un archivo — la infra resuelve la ruta.
- **`TTL`** (`vos/ttl.go`): renombre de `CachePolicy`. API: `NewTTL(d time.Duration) TTL` (default 30d si ≤0), `Duration() time.Duration`. Motivo del renombre: "CachePolicy" sugiere algo global; la semántica real es "duración de validez de una ejecución histórica".
- **`ExecutionDecision`** (`vos/decision.go`): `type Action int` con `ActionRun | ActionSkip`, más `reason string` y `matchedEntryAt time.Time` opcional. API: `DecisionRun(reason string)`, `DecisionSkip(matchedAt time.Time)`, `ShouldRun() bool`, `Reason() string`. El orchestrator logguea la razón (hoy no lo hace).

### 4.3 Aggregate(s)

**Aggregate root: `StepHistory`** (reemplaza `StateTable`).

Invariantes:
- Máximo 5 entries.
- Entries ordenadas cronológicamente ascendente por `CreatedAt`.
- Ninguna entry tiene VOs "vacíos" — la construcción siempre se hace por `NewHistoryEntry(set FingerprintSet, at time.Time)`; no hay setter `SetCreatedAt` expuesto.
- `StepHistory` conoce su `StorageKey` (para saber de qué paso es), no un `name string` suelto.

Métodos públicos:
- `NewStepHistory(key StorageKey) *StepHistory`
- `LoadStepHistory(key StorageKey, entries []HistoryEntry) (*StepHistory, error)` — valida, ordena, trunca.
- `Append(set FingerprintSet, now time.Time)` — añade respetando invariante de orden y max.
- `Decide(policy StepPolicy, current FingerprintSet, now time.Time) ExecutionDecision` — recorre sus entries aplicando la `Rule` compuesta de la policy y retorna `Run` o `Skip`. Aquí vive la lógica "¿cambió?" expresada como negación de "alguna entry hace match".
- `Entries() []HistoryEntry` — sólo lectura, para el DTO de infra.
- `Key() StorageKey`.

Justificación como raíz: es la unidad transaccional (se lee y se escribe como un todo al archivo `.tb`), protege la invariante de orden y max, y es la única vía para añadir entries desde el servicio.

### 4.4 Ports

```go
// ports/history_repository.go
type HistoryRepository interface {
    // Get retorna nil, nil si no existe historia para la key.
    // Retorna ErrHistoryCorrupted si el artefacto no puede reconstruirse.
    Get(ctx context.Context, key vos.StorageKey) (*aggregates.StepHistory, error)
    Save(ctx context.Context, history *aggregates.StepHistory) error
}

// ports/fingerprint_service.go
type FingerprintService interface {
    FromFile(filePath string) (vos.Fingerprint, error)
    FromDirectory(dirPath string) (vos.Fingerprint, error)
}

// ports/clock.go
type Clock interface { Now() time.Time }
```

Notas de contrato:
- `Get` recibe `StorageKey`, no `string`. La implementación gob traduce a `{rootVex}/{project}/{template}/storage/{step}.tb`.
- `Save` persiste el aggregate completo (lee la key del propio aggregate).
- Se añade `Clock` como puerto para hacer testeables las reglas TTL sin `time.Now()` real.
- `FingerprintService` se mantiene casi idéntica; sólo cambia el import path.

Implementadores:
- `HistoryRepository` → `infrastructure/storage/gob.GobHistoryRepository` (default `.vex/`).
- `FingerprintService` → `infrastructure/storage/gob.Sha256FingerprintService`.
- `Clock` → `infrastructure/storage/gob.SystemClock` o un `FakeClock` para tests.

### 4.5 Domain Services

- **`ExecutionDecider`** (`services/execution_decider.go`):
  - Responsabilidad única: decidir si ejecutar o saltar, y si se ejecuta con éxito, persistir una nueva entry.
  - Depende de: `HistoryRepository`, `StepPolicyCatalog`, `Clock`.
  - API pública:
    - `Decide(ctx, key StorageKey, current FingerprintSet) (ExecutionDecision, error)`
    - `RecordSuccess(ctx, key StorageKey, current FingerprintSet) error`
  - Separación clara vs hoy: `Decide` NO persiste (evita efecto lateral sorpresa); `RecordSuccess` se invoca sólo tras ejecución exitosa del paso (hoy el orchestrator hace esto manualmente; ahora se formaliza).

- **`StepPolicyCatalog`** (`services/step_policies.go`): mapa inmutable `StepName → StepPolicy`. Función pública `DefaultCatalog() StepPolicyCatalog` con los 4 pasos actuales. Expone `Lookup(StepName) (StepPolicy, error)` retornando `ErrUnknownStep` si no existe.

- **`FingerprintService`**: sigue siendo puerto; no hay servicio de dominio alrededor porque es puro cálculo y la orquestación vive en la capa de aplicación.

Decisión: el caso de uso "chequea + ejecuta + persiste" NO entra al dominio como un único método. El dominio provee `Decide` y `RecordSuccess` por separado; el orchestrator los enlaza porque entre ambos debe correr el step real (efecto lateral de `application`). Meterlo todo en un método `DecideAndExecute` obligaría al dominio a conocer `StepExecutor`, rompiendo el layering.

### 4.6 Patrón elegido para "debe ejecutarse?"

**Elección: Specification Pattern + Composite.**

Razones:
- Cada "dependencia" (fingerprint, TTL, environment) es naturalmente una *predicación* sobre `(HistoryEntry, FingerprintSet, now)` → `bool`. Eso es literalmente una Specification.
- La composición de predicados con `AllRules(r1, r2, ...)` es trivial y se amplía añadiendo un fichero nuevo (Open/Closed).
- Cada regla es testeable en aislamiento (pain J resuelto).
- Alternativas consideradas:
  - *Strategy plano por paso* (lo actual): rechazado porque obliga a editar un switch central cada vez que cambia la dependencia de un paso.
  - *Chain of Responsibility*: rechazado porque no hay semántica de "la siguiente maneja si yo no puedo"; todas las reglas deben pasar.
  - *Rule engine genérico (interpretado)*: rechazado — overkill; un puñado de reglas Go tipadas es más claro y más rápido.

Interfaz y composite (pseudocódigo):

```go
// services/rules/rule.go
type Rule interface {
    // Satisfies devuelve true si la entry histórica cumple la regla respecto del estado actual.
    Satisfies(entry aggregates.HistoryEntry, current vos.FingerprintSet, now time.Time) bool
}

type allRules struct{ rs []Rule }
func AllRules(rs ...Rule) Rule { return &allRules{rs} }
func (a *allRules) Satisfies(e aggregates.HistoryEntry, c vos.FingerprintSet, now time.Time) bool {
    for _, r := range a.rs { if !r.Satisfies(e, c, now) { return false } }
    return true
}
```

Reglas concretas:

```go
// fingerprint_rule.go
func NewFingerprintRule(kind vos.FingerprintKind) Rule { ... }
// Satisfies: entry.Fingerprint(kind).Equals(current.Fingerprint(kind)); si alguna ausencia → false.

// environment_rule.go
func NewEnvironmentRule() Rule { ... }
// Satisfies: entry.Environment().Equals(current.Environment()).

// ttl_rule.go
func NewTTLRule(ttl vos.TTL) Rule { ... }
// Satisfies: now.Before(entry.CreatedAt().Add(ttl.Duration())).
```

Uso desde `StepHistory.Decide`:

```go
for _, entry := range h.entries {
    if policy.Rule().Satisfies(entry, current, now) {
        return DecisionSkip(entry.CreatedAt())
    }
}
return DecisionRun("no matching history entry")
```

### 4.7 Configuración por paso (StepPolicy)

`StepPolicy` agrupa la regla compuesta y metadata del paso:

```go
type StepPolicy struct {
    step vos.StepName
    rule rules.Rule
    ttl  vos.TTL // zero si el paso no usa TTL
}
```

Catálogo (servicio de dominio, `services/step_policies.go`, un solo archivo — donde mañana se añade una dependencia nueva):

```go
func DefaultCatalog() StepPolicyCatalog {
    return newCatalog(map[vos.StepName]StepPolicy{
        vos.StepTest: newPolicy(vos.StepTest, vos.NewTTL(0), rules.AllRules(
            rules.NewFingerprintRule(vos.KindInstruction),
            rules.NewFingerprintRule(vos.KindVars),
            rules.NewFingerprintRule(vos.KindCode),
            rules.NewTTLRule(vos.NewTTL(0)),
        )),
        vos.StepSupply: newPolicy(vos.StepSupply, vos.TTL{}, rules.AllRules(
            rules.NewFingerprintRule(vos.KindInstruction),
            rules.NewFingerprintRule(vos.KindVars),
            rules.NewEnvironmentRule(),
        )),
        vos.StepPackage: newPolicy(vos.StepPackage, vos.TTL{}, rules.AllRules(
            rules.NewFingerprintRule(vos.KindInstruction),
            rules.NewFingerprintRule(vos.KindVars),
            rules.NewFingerprintRule(vos.KindCode),
        )),
        vos.StepDeploy: newPolicy(vos.StepDeploy, vos.TTL{}, rules.AllRules(
            rules.NewFingerprintRule(vos.KindInstruction),
            rules.NewFingerprintRule(vos.KindVars),
            rules.NewFingerprintRule(vos.KindCode),
            rules.NewEnvironmentRule(),
        )),
    })
}
```

Agregar un paso nuevo o una regla nueva:
1. Crear la regla en `services/rules/` (ej. `antivirus_rule.go`) implementando `Rule`.
2. Añadir una entrada más en `DefaultCatalog` referenciándola.
3. No se toca nada más — ni aggregate, ni decider, ni orchestrator.

## 5. Capa de infraestructura

Mover `internal/infrastructure/state/` a `internal/infrastructure/storage/gob/`. Cambios concretos:

- **`gob_history_repository.go`** (antes `gob_state_repository.go`):
  - Recibe un `PathResolver` que traduce `StorageKey` → `string` de ruta. El resolver default implementa "git-like en `.vex/`": `{rootVexPath}/{projectName}/{templateName}/storage/{step}.tb`.
  - `Get(ctx, key)`: abre archivo, decodifica DTO, invoca `fromDTO(dto, key)` que ahora retorna `(*aggregates.StepHistory, error)`. Si el archivo no existe → `(nil, nil)`. Si decode falla o VOs inválidos → `ErrHistoryCorrupted` envolviendo el error original (pain A resuelto).
  - `Save(ctx, history)`: lee `history.Key()`, resuelve la ruta, asegura directorio, encodea DTO.

- **`history_dto.go`** (antes `state_table_dto.go`):
  - `fromDTO` propaga errores en lugar de crear VOs vacíos:
    ```go
    func fromDTO(dto *HistoryDTO, key vos.StorageKey) (*aggregates.StepHistory, error) {
        entries := make([]aggregates.HistoryEntry, 0, len(dto.Entries))
        for i, e := range dto.Entries {
            entry, err := buildEntry(e)
            if err != nil { return nil, fmt.Errorf("entry %d: %w", i, ErrHistoryCorrupted) }
            entries = append(entries, entry)
        }
        return aggregates.LoadStepHistory(key, entries)
    }
    ```
  - El DTO persiste `map[FingerprintKind]string` en lugar de campos sueltos `Code/Instruction/Vars`, lo que permite omitir fingerprints que un paso no usa sin "vacíos semánticos" en disco.

- **`sha256_fingerprint_service.go`**: sólo cambia el paquete (`package gob`). Lógica intacta.

- **`path_resolver.go`** (nuevo): contiene el acoplamiento a filesystem `.vex/`. Justificación: mantiene el repositorio agnóstico del layout — mañana una implementación Redis/SQLite no necesita `PathResolver`, sólo `StorageKey`.

- **`cmd/vexd/factory.go`** actualiza imports y wiring:
  - `stateSvc.NewStateManager(stateRepo)` → `storageSvc.NewExecutionDecider(historyRepo, storageSvc.DefaultCatalog(), storageSvc.NewSystemClock())`.
  - `stateInfra.NewGobStateRepository()` → `gobInfra.NewGobHistoryRepository(gobInfra.NewDefaultPathResolver(cfg.rootVexPath))`.
  - `stateInfra.NewSha256FingerprintService()` → `gobInfra.NewSha256FingerprintService()`.

## 6. Capa de aplicación

Cambios en `internal/application/execution_orchestrator.go`:

- El campo `stateManager staPrt.StateManager` se reemplaza por `decider storagePorts.ExecutionDecider` (o se puede mantener como interfaz en el puerto con métodos `Decide` + `RecordSuccess`).
- Se reemplaza `workspace.StateTablePath(stepName)` (que devolvía una ruta) por construir un `StorageKey` a partir de `(project.Name(), templateName, stepName)`.
- El bloque actual del orchestrator (líneas 222-311) queda así (conceptualmente):

```go
key := storageVos.NewStorageKey(project.Name(), templateName, stepName)
current := buildFingerprintSet(stepName, ...) // sólo calcula los kinds requeridos por la policy

decision, err := o.decider.Decide(ctx, key, current)
if err != nil { return ... }
if !decision.ShouldRun() {
    emit(fmt.Sprintf("  - Paso '%s' sin cambios (match de %s). Omitiendo.", stepName, decision.MatchedAt()))
    continue
}
// ... ejecutar step ...
if err := o.decider.RecordSuccess(ctx, key, current); err != nil { emit("ADVERTENCIA: ...") }
```

Ventajas:
- El `if !hasChanged { continue }` se reemplaza por una decisión tipada con razón (observabilidad mejor).
- Se elimina `staVos.NewCachePolicy(0)` del orchestrator — la policy vive en el dominio.
- El método `generateStepFingerprints` puede pasar a calcular *sólo* los kinds que la policy pide (optimización futura; no requerida para el refactor inicial — basta con pasar el set completo).

## 7. Plan de migración (pasos ejecutables)

Cada paso debe compilar y pasar tests existentes antes de avanzar.

1. **Crear `internal/domain/storage/` vacío con los VOs renombrados**: copiar `Fingerprint`, `Environment`, y añadir nuevos (`StepName`, `StorageKey`, `TTL` como alias/renombre de `CachePolicy`, `FingerprintKind`, `FingerprintSet`, `ExecutionDecision`). Mantener `domain/state/` intacto. Tests: unit tests de los VOs nuevos. `go build ./...` debe pasar.

2. **Crear `services/rules/` y reglas concretas** (`fingerprint_rule.go`, `environment_rule.go`, `ttl_rule.go`, `rule.go` con `AllRules`). Tests unitarios independientes por regla — aquí se cubre el pain J.

3. **Crear aggregate `StepHistory` y `HistoryEntry`** en `internal/domain/storage/aggregates/`, con la firma nueva (sin setter CreatedAt, constructor que recibe `FingerprintSet`). Portar y adaptar `state_table_test.go` a `step_history_test.go`.

4. **Crear `services/step_policies.go`** con `DefaultCatalog` + tests que verifiquen los 4 pasos actuales.

5. **Crear `services/execution_decider.go`** con `Decide` y `RecordSuccess`. Mock de `HistoryRepository` y `Clock` en tests. Casos: historial vacío → Run, match → Skip, match expirado por TTL → Run, repo error → error.

6. **Crear puertos** `ports/history_repository.go`, `ports/fingerprint_service.go`, `ports/clock.go`.

7. **Crear `internal/infrastructure/storage/gob/`**: copiar y renombrar `gob_state_repository.go` → `gob_history_repository.go` adaptado al nuevo puerto y con `PathResolver`. Reescribir `history_dto.go` propagando errores. Copiar `sha256_fingerprint_service.go` bajo el nuevo paquete.

8. **Añadir `PathResolver` default** apuntando a `.vex/{project}/{template}/storage/{step}.tb`. Test unitario sobre la traducción.

9. **Wiring paralelo en `cmd/vexd/factory.go`**: construir también el nuevo `ExecutionDecider` y pasarlo al orchestrator *junto con* el viejo `StateManager` (temporalmente), detrás de un flag o feature toggle interno. Esto permite mergear sin romper.

10. **Migrar `ExecutionOrchestrator`** para usar `Decide` / `RecordSuccess` y `StorageKey`. Remover la llamada a `workspace.StateTablePath` en el orchestrator. Ajustar tests de orchestrator (si existen).

11. **Retirar el viejo puerto** `StateManager` del orchestrator y del wiring en `factory.go`.

12. **Borrar `internal/domain/state/`** completo.

13. **Borrar `internal/infrastructure/state/`** completo. Mover tests que sobrevivan a la nueva ubicación.

14. **Limpiar `workspace.StateTablePath`** y `StateDirPath` de `domain/workspace/aggregates/workspace.go` si ya no lo usa nadie más (sí lo hace sólo el orchestrator; tras el paso 10, queda muerto). Dejar `StateDirPath` sólo si el path_resolver de infra lo reutiliza como helper; mejor moverlo al `PathResolver` de infra.

15. **Renombrar referencias en logs / mensajes de error** de "estado" a "historial" (`ADVERTENCIA: no se pudo guardar el estado del paso` → `...el historial del paso`) para alinear el lenguaje con el nuevo modelo.

## 8. Pruebas

Tests nuevos:
- `vos/storage_key_test.go` — igualdad, formato, validación.
- `vos/fingerprint_set_test.go` — ausencias, lookup por kind.
- `vos/step_name_test.go` — rechazo de nombres desconocidos.
- `services/rules/fingerprint_rule_test.go`, `environment_rule_test.go`, `ttl_rule_test.go` — uno por regla (cubre pain J).
- `services/rules/rule_test.go` — `AllRules` corta al primer false, vacío devuelve true.
- `aggregates/step_history_test.go` — invariantes de orden/max (portado); método `Decide` con policies de los 4 pasos.
- `services/execution_decider_test.go` — con mock `HistoryRepository` + `FakeClock`.
- `services/step_policies_test.go` — cada StepName retorna una policy; unknown → error.
- `infrastructure/storage/gob/history_dto_test.go` — un archivo con un fingerprint inválido devuelve `ErrHistoryCorrupted` (pain A).
- `infrastructure/storage/gob/path_resolver_test.go` — StorageKey → path esperado.

Tests existentes adaptados:
- `aggregates/state_table_test.go` → `aggregates/step_history_test.go`: ajuste de nombres y de constructor, pero la cobertura de orden/max se preserva.
- `services/state_manager_test.go` se divide entre `execution_decider_test.go` y tests de reglas; se elimina.
- `infrastructure/state/sha256_fingerprint_service_test.go` → `infrastructure/storage/gob/sha256_fingerprint_service_test.go` sin cambios de lógica.

## 9. Riesgos y mitigaciones

1. **Compatibilidad binaria de archivos `.tb` existentes**: el DTO pasa de campos fijos (`Code/Instruction/Vars/Environment`) a `map[FingerprintKind]string`. Los archivos viejos no se decodificarán. *Mitigación*: versionar el DTO (`HistoryDTO{Version int; Entries ...}`) y, en la primera lectura, si `Version==0` leer el formato legacy y convertir. Alternativa pragmática: invalidar los `.tb` existentes (borrarlos — son caché) y documentarlo.

2. **Ruptura silenciosa en llamadores del workspace**: `StateTablePath` deja de usarse tras el refactor pero sigue público. *Mitigación*: grep final de referencias; eliminar el método o marcarlo como deprecated con comentario claro si se prevé que infraestructura siga usándolo.

3. **Regresión en semántica TTL**: hoy `CachePolicy{ttl:0}` se convierte en 30d por default. La nueva `TTL` debe conservar ese comportamiento exacto para no reejecutar tests cuando no toca. *Mitigación*: test explícito `TestTTL_DefaultOnZero`.

4. **Errores de `HistoryRepository` enmascaran cambios legítimos**: hoy `HasStateChanged` devuelve `(true, err)` para errores de lectura. Con la nueva API `Decide` devuelve `(ExecutionDecision, error)`. El orchestrator debe distinguir entre "error real" (aborta) y "archivo corrupto → tratar como no-historia y re-ejecutar". *Mitigación*: `ExecutionDecider.Decide` atrapa `ErrHistoryCorrupted` internamente y retorna `DecisionRun("historial corrupto")` + log; otros errores se propagan.

5. **Explosión de archivos pequeños**: el refactor parte matchers+factory en ~6 archivos nuevos. *Mitigación*: es aceptable porque cada regla es una unidad testeable y el valor (Open/Closed) compensa. No hacer "un archivo por regla y además un paquete por regla" — todas en `services/rules/`.

6. **Migración big-bang vs incremental**: hacerlo todo de una trae riesgo de PR gigante. *Mitigación*: los pasos 1–8 del plan son puramente aditivos (el código viejo sigue vivo) — se pueden mergear de a poco. El corte real es el paso 10.

## 10. Archivos a eliminar, renombrar, crear

**Eliminar** (tras migración completa):
- `internal/domain/state/vos/fingerprint.go`
- `internal/domain/state/vos/current_state_fingerprints.go`
- `internal/domain/state/vos/step.go`
- `internal/domain/state/vos/environment.go`
- `internal/domain/state/vos/cache_policy.go`
- `internal/domain/state/vos/fingerprint_test.go`
- `internal/domain/state/aggregates/state_entry.go`
- `internal/domain/state/aggregates/state_table.go`
- `internal/domain/state/aggregates/state_table_test.go`
- `internal/domain/state/ports/state_manager.go`
- `internal/domain/state/ports/fingerprint_service.go`
- `internal/domain/state/ports/state_repository.go`
- `internal/domain/state/ports/state_matcher.go`
- `internal/domain/state/services/state_manager.go`
- `internal/domain/state/services/state_manager_test.go`
- `internal/domain/state/services/matcher_factory.go`
- `internal/domain/state/services/matchers/base_matcher.go`
- `internal/domain/state/services/matchers/test_state_matcher.go`
- `internal/domain/state/services/matchers/supply_state_matcher.go`
- `internal/domain/state/services/matchers/package_state_matcher.go`
- `internal/domain/state/services/matchers/deploy_state_matcher.go`
- `internal/infrastructure/state/gob_state_repository.go`
- `internal/infrastructure/state/state_table_dto.go`
- `internal/infrastructure/state/sha256_fingerprint_service.go`
- `internal/infrastructure/state/sha256_fingerprint_service_test.go`

**Renombrar / reubicar** (con adaptaciones menores de firma o paquete):
- `state/vos/fingerprint.go` → `storage/vos/fingerprint.go`
- `state/vos/environment.go` → `storage/vos/environment.go`
- `state/vos/cache_policy.go` → `storage/vos/ttl.go` (renombrado + API `NewTTL`)
- `state/vos/step.go` → `storage/vos/step_name.go` (constantes → tipo tipado con constructor)
- `state/ports/fingerprint_service.go` → `storage/ports/fingerprint_service.go`
- `infrastructure/state/sha256_fingerprint_service.go` → `infrastructure/storage/gob/sha256_fingerprint_service.go`
- `infrastructure/state/sha256_fingerprint_service_test.go` → idem

**Crear**:
- `internal/domain/storage/vos/fingerprint_kind.go`
- `internal/domain/storage/vos/fingerprint_set.go`
- `internal/domain/storage/vos/storage_key.go`
- `internal/domain/storage/vos/decision.go`
- `internal/domain/storage/aggregates/history_entry.go`
- `internal/domain/storage/aggregates/step_history.go`
- `internal/domain/storage/aggregates/step_history_test.go`
- `internal/domain/storage/ports/history_repository.go`
- `internal/domain/storage/ports/clock.go`
- `internal/domain/storage/services/rules/rule.go`
- `internal/domain/storage/services/rules/fingerprint_rule.go`
- `internal/domain/storage/services/rules/environment_rule.go`
- `internal/domain/storage/services/rules/ttl_rule.go`
- `internal/domain/storage/services/rules/*_test.go` (uno por regla + composite)
- `internal/domain/storage/services/step_policies.go`
- `internal/domain/storage/services/step_policies_test.go`
- `internal/domain/storage/services/execution_decider.go`
- `internal/domain/storage/services/execution_decider_test.go`
- `internal/domain/storage/errors.go`
- `internal/infrastructure/storage/gob/gob_history_repository.go`
- `internal/infrastructure/storage/gob/history_dto.go`
- `internal/infrastructure/storage/gob/history_dto_test.go`
- `internal/infrastructure/storage/gob/path_resolver.go`
- `internal/infrastructure/storage/gob/path_resolver_test.go`
- `internal/infrastructure/storage/gob/system_clock.go`

**Modificar** (sin renombrar el archivo):
- `cmd/vexd/factory.go` — swap de wiring (imports, constructor del decider).
- `internal/application/execution_orchestrator.go` — usar `Decide` / `RecordSuccess` + `StorageKey`; eliminar `CachePolicy` e `if !hasChanged`.
- `internal/domain/workspace/aggregates/workspace.go` — eliminar `StateDirPath` y `StateTablePath` si tras el paso 10 no tienen callers.
