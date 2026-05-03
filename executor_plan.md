# Plan de Refactoring: Dominio `execution` — VexEngine

**Fecha:** 2026-04-21
**Alcance:** `/vex-engine/internal/domain/execution/`
**Estrategia:** Refactoring profundo — no rewrite desde cero

---

## 1. Diagnóstico del Estado Actual

### 1.1 God Objects confirmados por análisis de código

**`services/command_executor.go`** (138 líneas, un único método `Execute`):
El método hace exactamente 8 cosas distintas:
1. Detectar si el `workdir` apunta al scope compartido (`filepath.Base` sobre el string)
2. Construir rutas absolutas para los archivos de template
3. Procesar/interpolar archivos de template en disco
4. Interpolar el string del comando con variables
5. Construir el directorio de ejecución (`execDir`)
6. Ejecutar el proceso hijo via `CommandRunner`
7. Validar exit code y verificar probes con regex compiladas en caliente
8. Extraer variables de output y construir `VariableSet`

**`services/step_executor.go`** (91 líneas):
Acumula dos responsabilidades que deberían estar separadas:
1. Orquestar la secuencia de comandos con acumulación de variables
2. Inyectar variables de workspace (`step_workdir`, `shared_workdir`) — lógica que pertenece a la construcción del contexto, no a la ejecución

### 1.2 Bug crítico: `defer ce.fileProcessor.Restore()` comentado

```go
// services/command_executor.go línea 57
//defer ce.fileProcessor.Restore()
```

`FileProcessor` guarda backups en memoria (`fp.backups map[string][]byte`), pero `Restore()` nunca se llama. Los archivos de template en disco quedan con las variables interpoladas entre ejecuciones. Esto rompe la idempotencia del pipeline si el workdir no se borra entre runs.

### 1.3 Detección de scope frágil

```go
// command_executor.go línea 44
isShared := filepath.Base(command.Workdir()) == vos.SharedScope
```

Si `workdir` es `"shared/terraform"`, `filepath.Base` devuelve `"terraform"` y detecta como NO shared. Solo funciona correctamente cuando el leaf exacto es `"shared"`.

### 1.4 Regex compiladas sin caché

`output_extractor.go` y `command_executor.go#checkProbes` compilan la misma regex en cada invocación del método. El error de regex inválida se detecta en runtime tardíamente en lugar de en construcción.

### 1.5 Singletons globales ilegítimos

```go
// services/interpolator.go
var defaultInterpolator ports.Interpolator = &Interpolator{}

// services/output_extractor.go
var defaultOutputExtractor ports.OutputExtractor = &OutputExtractor{}
```

Los constructores retornan siempre la misma instancia global en lugar de crear instancias nuevas. Esto impide testear en paralelo y viola el principio de instancias independientes.

### 1.6 API de retorno inconsistente

```go
// ports/command_executor.go
Execute(...) *vos.ExecutionResult           // sin error

// ports/step_executor.go
Execute(...) (*vos.ExecutionResult, error)  // con error
```

El `CommandExecutor` encapsula el error dentro del `ExecutionResult`, pero `StepExecutor` lo sube como segundo retorno. El orquestador revisa ambos, duplicando lógica de detección de fallo.

### 1.7 Magic strings en servicios de dominio

```go
vos.NewOutputVar("step_workdir", stepWorkdir, false)
vos.NewOutputVar("shared_workdir", sharedWorkdir, false)
filepath.Base(command.Workdir()) == vos.SharedScope
```

### 1.8 Lógica de dominio en la capa de aplicación

`execution_orchestrator.go` contiene construcción de variables de proyecto, copia del template al workdir, lectura/escritura de vars por step, y filtrado de vars compartidas vs. de step. Todo esto es lógica de dominio.

### 1.9 Tensión entre dominios de nombres

`pipeline.StepName` es un struct con orden+nombre (`"01-test"`) mientras que `storage.vos.StepName` es un enum fijo (`"test"`, `"supply"`, `"package"`, `"deploy"`). La conversión se hace sin abstracción y falla silenciosamente si se agrega un paso nuevo.

---

## 2. Principios de Diseño a Aplicar

### 2.1 SRP — Single Responsibility Principle

Cada servicio hace exactamente una cosa:
- `TemplateHandler`: procesar archivos en disco y restaurarlos (nunca ejecuta comandos)
- `InterpolationHandler`: solo interpola strings (no toca archivos)
- `ProbeHandler`: solo verifica que regex hagan match (no extrae ni guarda)
- `OutputHandler`: solo extrae variables de stdout via regex
- `ExecutionHandler`: solo ejecuta el proceso hijo (no interpola, no verifica)
- `StepVariableBuilder`: solo construye el VariableSet inicial de un step por capas

### 2.2 OCP — Open/Closed Principle

El pipeline de ejecución de un comando se define como una cadena de `CommandHandler`. Para agregar validaciones nuevas (e.g., timeout checks, audit logging) se agrega un handler sin tocar el código existente.

### 2.3 DIP — Dependency Inversion Principle

Todos los ports se definen en el paquete consumidor (`ports/`). Ningún servicio de dominio importa infraestructura. Los singletons globales se eliminan — cada constructor recibe sus dependencias.

### 2.4 ISP — Interface Segregation Principle

`FileProcessor` actual tiene `Process` y `Restore` juntos. Se separan en dos interfaces:
- `TemplateApplier`: `Apply(files []string, vars VariableSet) (TemplateSession, error)`
- `TemplateSession`: `Restore() error`

### 2.5 DDD aplicado

- `Execution` (aggregate root) gana métodos de dominio: `MarkStepCached()`, `MarkStepFailed()`, `MarkStepSucceeded()`
- `StepContext` (nuevo VO) encapsula `envWorkdir` + `sharedWorkdir` — reemplaza los strings sueltos que viajan entre servicios
- `WorkdirScope` (nuevo VO tipo enum): `ScopeEnv | ScopeShared` — elimina la detección via `filepath.Base`
- `CommandContext` (nuevo VO) encapsula `command + vars + stepContext` — el input del pipeline de ejecución

### 2.6 Patrones de diseño a aplicar

| Patrón | Dónde | Por qué |
|--------|-------|---------|
| **Chain of Responsibility** | Pipeline de ejecución de comando | Cada fase (template, interpolación, run, probe, extract) es un handler encadenable |
| **Strategy** | `ShellStrategy` para multiplataforma | Inyectar `LinuxShellStrategy` o `WindowsShellStrategy` |
| **Builder** | `StepVariableBuilder` | Construir el contexto de variables por capas de forma legible |
| **Transactional/RAII** | Restauración de templates | `TemplateSession` con `Restore()` garantizado via `defer` |
| **ACL (Anti-Corruption Layer)** | Conversión de StepName entre dominios | Aísla la tensión entre `pipeline.StepName` y `storage.vos.StepName` |

---

## 3. Nueva Estructura de Archivos

```
domain/execution/
├── aggregates/
│   └── execution.go                  ← SIN CAMBIOS ESTRUCTURALES, agregar MarkStep*
├── entities/
│   └── step.go                       ← Simplificar: usar StepContext VO en lugar de strings sueltos
├── vos/
│   ├── execution_id.go               ← sin cambios
│   ├── execution_status.go           ← sin cambios
│   ├── execution_result.go           ← CAMBIO: simplificar, mover StepResult a archivo propio
│   ├── step_result.go                ← NUEVO: StepStatus + StepResult
│   ├── command_result.go             ← sin cambios
│   ├── command.go                    ← sin cambios
│   ├── command_output.go             ← CAMBIO: agregar CompiledProbe() *regexp.Regexp en constructor
│   ├── output_var.go                 ← CAMBIO: eliminar SharedScope const, mover a workdir_scope.go
│   ├── variable_set.go               ← sin cambios
│   ├── runtime_config.go             ← sin cambios
│   ├── step_context.go               ← NUEVO: encapsula envWorkdir + sharedWorkdir
│   ├── workdir_scope.go              ← NUEVO: enum ScopeEnv/ScopeShared + ScopeFromWorkdir()
│   ├── command_context.go            ← NUEVO: command + vars + stepContext + rutas absolutas
│   └── variable_layer.go             ← NUEVO: constantes de nombres de variables de dominio
├── ports/
│   ├── executor.go                   ← sin cambios
│   ├── step_runner.go                ← RENOMBRAR step_executor.go + ajuste de firma
│   ├── command_handler.go            ← RENOMBRAR command_executor.go + nueva abstracción
│   ├── command_runner.go             ← sin cambios
│   ├── variable_resolver.go          ← sin cambios
│   ├── interpolator.go               ← sin cambios
│   ├── output_extractor.go           ← sin cambios
│   ├── template_applier.go           ← NUEVO: TemplateApplier + TemplateSession
│   ├── copy_workdir.go               ← sin cambios
│   ├── file_system.go                ← sin cambios
│   ├── log_emitter.go                ← sin cambios
│   ├── execution_repository.go       ← sin cambios
│   ├── vars_repository.go            ← sin cambios
│   ├── step_name_translator.go       ← NUEVO: port ACL pipeline.StepName → storage.StepName
│   └── file_processor.go             ← ELIMINAR (reemplazado por template_applier.go)
└── services/
    ├── step_runner.go                ← REFACTOR step_executor.go: solo orquesta la secuencia
    ├── command_pipeline.go           ← NUEVO: Chain of Responsibility de handlers
    ├── template_handler.go           ← NUEVO: handler que aplica/restaura templates
    ├── interpolation_handler.go      ← NUEVO: handler que interpola el cmd string
    ├── execution_handler.go          ← NUEVO: handler que corre el proceso hijo
    ├── probe_handler.go              ← NUEVO: handler que verifica regex probes
    ├── output_handler.go             ← NUEVO: handler que extrae variables de output
    ├── step_variable_builder.go      ← NUEVO: construye VariableSet por capas
    ├── pipeline_runner.go            ← NUEVO: orquestador de nivel de pipeline (desde orchestrator)
    ├── step_name_translator.go       ← NUEVO: implementación ACL
    ├── variable_resolver.go          ← sin cambios estructurales (limpiar singleton)
    ├── interpolator.go               ← CAMBIO: eliminar singleton global
    ├── output_extractor.go           ← CAMBIO: eliminar singleton + usar CompiledProbe del VO
    └── files_processor.go            ← ELIMINAR (reemplazado por template_handler.go)
```

---

## 4. Cambios en Tipos y VOs

### 4.1 Nuevo: `vos/workdir_scope.go`

Elimina la detección frágil via `filepath.Base`:

```go
package vos

import (
    "path/filepath"
    "strings"
)

type WorkdirScope int

const (
    ScopeEnv    WorkdirScope = iota
    ScopeShared WorkdirScope = iota
)

const sharedScopeName = "shared"

// ScopeFromWorkdir determina el scope dado el path de workdir declarado en el YAML.
// Un workdir es "shared" si su primer segmento es "shared".
// Ejemplo: "shared/terraform" → ScopeShared, "env/terraform" → ScopeEnv.
func ScopeFromWorkdir(workdir string) WorkdirScope {
    if workdir == "" {
        return ScopeEnv
    }
    normalized := filepath.ToSlash(workdir)
    firstSegment := strings.SplitN(normalized, "/", 2)[0]
    if firstSegment == sharedScopeName {
        return ScopeShared
    }
    return ScopeEnv
}

func (s WorkdirScope) IsShared() bool { return s == ScopeShared }
```

### 4.2 Nuevo: `vos/step_context.go`

Elimina los strings sueltos `workspaceStep` y `workspaceShared` que viajan entre servicios:

```go
package vos

import "errors"

type StepContext struct {
    envWorkdir    string
    sharedWorkdir string
}

func NewStepContext(envWorkdir, sharedWorkdir string) (StepContext, error) {
    if envWorkdir == "" {
        return StepContext{}, errors.New("envWorkdir no puede estar vacío")
    }
    if sharedWorkdir == "" {
        return StepContext{}, errors.New("sharedWorkdir no puede estar vacío")
    }
    return StepContext{envWorkdir: envWorkdir, sharedWorkdir: sharedWorkdir}, nil
}

func (sc StepContext) EnvWorkdir() string    { return sc.envWorkdir }
func (sc StepContext) SharedWorkdir() string { return sc.sharedWorkdir }

func (sc StepContext) WorkdirFor(scope WorkdirScope) string {
    if scope == ScopeShared {
        return sc.sharedWorkdir
    }
    return sc.envWorkdir
}
```

### 4.3 Nuevo: `vos/command_context.go`

Input unificado del pipeline de comando — resuelve rutas absolutas una sola vez:

```go
package vos

import "path/filepath"

type CommandContext struct {
    command     Command
    vars        VariableSet
    stepContext StepContext
    executionID ExecutionID
    scope       WorkdirScope
    absWorkdir  string
}

func NewCommandContext(
    cmd Command,
    vars VariableSet,
    stepCtx StepContext,
    execID ExecutionID,
) CommandContext {
    scope := ScopeFromWorkdir(cmd.Workdir())
    base := stepCtx.WorkdirFor(scope)
    absWorkdir := base
    if cmd.Workdir() != "" {
        absWorkdir = filepath.Join(base, cmd.Workdir())
    }
    return CommandContext{
        command:     cmd,
        vars:        vars.Clone(),
        stepContext: stepCtx,
        executionID: execID,
        scope:       scope,
        absWorkdir:  absWorkdir,
    }
}

func (cc CommandContext) Command() Command        { return cc.command }
func (cc CommandContext) Vars() VariableSet       { return cc.vars }
func (cc CommandContext) StepContext() StepContext { return cc.stepContext }
func (cc CommandContext) ExecutionID() ExecutionID { return cc.executionID }
func (cc CommandContext) Scope() WorkdirScope     { return cc.scope }
func (cc CommandContext) AbsWorkdir() string      { return cc.absWorkdir }

func (cc CommandContext) AbsTemplatePaths() []string {
    files := cc.command.TemplateFiles()
    abs := make([]string, len(files))
    base := cc.stepContext.WorkdirFor(cc.scope)
    for i, f := range files {
        abs[i] = filepath.Join(base, cc.command.Workdir(), f)
    }
    return abs
}
```

### 4.4 Nuevo: `vos/variable_layer.go`

Elimina magic strings en servicios de dominio:

```go
package vos

const (
    VarStepWorkdir   = "step_workdir"
    VarSharedWorkdir = "shared_workdir"
    VarProjectID     = "project_id"
    VarProjectName   = "project_name"
    VarProjectOrg    = "project_organization"
    VarProjectTeam   = "project_team"
    VarEnvironment   = "environment"
    VarProjectWorkdir = "project_workdir"
)
```

### 4.5 Nuevo: `vos/step_result.go`

Tipo específico para resultado de un step (separado de `ExecutionResult`):

```go
package vos

type StepStatus string

const (
    StepSuccess StepStatus = "SUCCESS"
    StepFailure StepStatus = "FAILURE"
    StepCached  StepStatus = "CACHED"
)

type StepResult struct {
    StepName   string
    Status     StepStatus
    Logs       string
    OutputVars VariableSet
    Error      error
}

func (r *StepResult) Failed() bool {
    return r.Status == StepFailure || r.Error != nil
}
```

### 4.6 Cambio en `vos/command_output.go`

Compilar y cachear regex al construir el VO — el error de regex inválida se detecta al construir, no en runtime:

```go
type CommandOutput struct {
    name          string
    probe         string
    compiledProbe *regexp.Regexp
}

func NewCommandOutput(name, probe string) (CommandOutput, error) {
    if probe == "" {
        return CommandOutput{}, errors.New("probe no puede estar vacío")
    }
    re, err := regexp.Compile(probe)
    if err != nil {
        return CommandOutput{}, fmt.Errorf("probe '%s' tiene regex inválida: %w", probe, err)
    }
    return CommandOutput{name: name, probe: probe, compiledProbe: re}, nil
}

func (op CommandOutput) CompiledProbe() *regexp.Regexp { return op.compiledProbe }
```

### 4.7 Eliminaciones

- `vos/output_var.go`: eliminar `const SharedScope = "shared"` — se mueve a `workdir_scope.go`
- `ports/file_processor.go`: eliminar — reemplazado por `ports/template_applier.go`
- `services/files_processor.go`: eliminar — reemplazado por `services/template_handler.go`

---

## 5. Descomposición de `CommandExecutor`

El `CommandExecutor` actual hace 8 cosas. Se descompone en 5 handlers especializados encadenados via Chain of Responsibility.

### 5.1 Abstracción base

```go
// ports/command_handler.go
package ports

import "context"

// CommandExecution es el estado mutable que viaja por la cadena de handlers.
type CommandExecution struct {
    Context         vos.CommandContext
    Emitter         LogEmitter
    InterpolatedCmd string            // llenado por InterpolationHandler
    RunResult       *vos.CommandResult // llenado por ExecutionHandler
    OutputVars      vos.VariableSet   // llenado por OutputHandler
    sessions        []TemplateSession // acumuladas por TemplateHandler
}

func (ce *CommandExecution) RegisterSession(s TemplateSession) {
    ce.sessions = append(ce.sessions, s)
}

func (ce *CommandExecution) RestoreAllSessions() error {
    var firstErr error
    for _, s := range ce.sessions {
        if err := s.Restore(); err != nil && firstErr == nil {
            firstErr = err
        }
    }
    return firstErr
}

// CommandHandler es un eslabón en la cadena de procesamiento de un comando.
type CommandHandler interface {
    Handle(ctx context.Context, exec *CommandExecution) error
}
```

### 5.2 Handler 1: `TemplateHandler`

```go
// services/template_handler.go
type TemplateHandler struct {
    applier ports.TemplateApplier
}

func (h *TemplateHandler) Handle(ctx context.Context, exec *ports.CommandExecution) error {
    files := exec.Context.AbsTemplatePaths()
    if len(files) == 0 {
        return nil
    }
    session, err := h.applier.Apply(files, exec.Context.Vars())
    if err != nil {
        return fmt.Errorf("aplicar templates: %w", err)
    }
    exec.RegisterSession(session)
    return nil
}
```

### 5.3 Handler 2: `InterpolationHandler`

```go
// services/interpolation_handler.go
type InterpolationHandler struct {
    interpolator ports.Interpolator
}

func (h *InterpolationHandler) Handle(ctx context.Context, exec *ports.CommandExecution) error {
    interpolated, err := h.interpolator.Interpolate(
        exec.Context.Command().Cmd(),
        exec.Context.Vars(),
    )
    if err != nil {
        return fmt.Errorf("interpolar comando '%s': %w", exec.Context.Command().Name(), err)
    }
    exec.InterpolatedCmd = interpolated
    return nil
}
```

### 5.4 Handler 3: `ExecutionHandler`

```go
// services/execution_handler.go
type ExecutionHandler struct {
    runner ports.CommandRunner
}

func (h *ExecutionHandler) Handle(ctx context.Context, exec *ports.CommandExecution) error {
    exec.Emitter.Emit(exec.Context.ExecutionID(),
        fmt.Sprintf("Ejecutando: %s", exec.Context.Command().Name()))

    result, err := h.runner.Run(ctx, exec.InterpolatedCmd, exec.Context.AbsWorkdir())
    if err != nil {
        return fmt.Errorf("iniciar proceso '%s': %w", exec.Context.Command().Name(), err)
    }
    if result.ExitCode != 0 {
        return fmt.Errorf("comando '%s' falló con exit code %d:\n%s",
            exec.Context.Command().Name(), result.ExitCode, result.NormalizedStderr)
    }
    exec.RunResult = result
    return nil
}
```

### 5.5 Handler 4: `ProbeHandler`

```go
// services/probe_handler.go
type ProbeHandler struct{}

func (h *ProbeHandler) Handle(ctx context.Context, exec *ports.CommandExecution) error {
    for _, output := range exec.Context.Command().Outputs() {
        matches := output.CompiledProbe().FindStringSubmatch(exec.RunResult.NormalizedStdout)
        if len(matches) < 1 {
            return fmt.Errorf("probe '%s' no encontró coincidencia en stdout del comando '%s'",
                output.Probe(), exec.Context.Command().Name())
        }
    }
    return nil
}
```

### 5.6 Handler 5: `OutputHandler`

```go
// services/output_handler.go
type OutputHandler struct {
    extractor ports.OutputExtractor
}

func (h *OutputHandler) Handle(ctx context.Context, exec *ports.CommandExecution) error {
    vars, err := h.extractor.ExtractVars(
        exec.RunResult.NormalizedStdout,
        exec.Context.Command().Outputs(),
    )
    if err != nil {
        return fmt.Errorf("extraer variables de output: %w", err)
    }
    if exec.Context.Scope().IsShared() {
        promoted := vos.NewVariableSet()
        for _, v := range vars {
            ov, _ := vos.NewOutputVar(v.Name(), v.Value(), true)
            promoted.Add(ov)
        }
        exec.OutputVars = promoted
    } else {
        exec.OutputVars = vars
    }
    return nil
}
```

---

## 6. Pipeline de Ejecución de un Comando

### 6.1 Orquestador de la cadena

```go
// services/command_pipeline.go
package services

type CommandPipeline struct {
    handlers []ports.CommandHandler
}

func NewCommandPipeline(handlers ...ports.CommandHandler) *CommandPipeline {
    return &CommandPipeline{handlers: handlers}
}

func (p *CommandPipeline) Run(
    ctx context.Context,
    exec *ports.CommandExecution,
) (*vos.StepResult, error) {
    for _, handler := range p.handlers {
        if err := handler.Handle(ctx, exec); err != nil {
            logs := ""
            if exec.RunResult != nil {
                logs = exec.RunResult.NormalizedStdout + "\n" + exec.RunResult.NormalizedStderr
            }
            return &vos.StepResult{
                Status: vos.StepFailure,
                Logs:   logs,
                Error:  err,
            }, err
        }
    }
    logs := ""
    if exec.RunResult != nil {
        logs = exec.RunResult.NormalizedStdout
    }
    return &vos.StepResult{
        Status:     vos.StepSuccess,
        Logs:       logs,
        OutputVars: exec.OutputVars,
    }, nil
}
```

### 6.2 Factory del pipeline (en wiring de dependencias)

```go
func NewDefaultCommandPipeline(
    applier   ports.TemplateApplier,
    interpolator ports.Interpolator,
    runner    ports.CommandRunner,
    extractor ports.OutputExtractor,
) *CommandPipeline {
    return NewCommandPipeline(
        &TemplateHandler{applier: applier},
        &InterpolationHandler{interpolator: interpolator},
        &ExecutionHandler{runner: runner},
        &ProbeHandler{},
        &OutputHandler{extractor: extractor},
    )
}
```

### 6.3 Flujo completo visual

```
CommandContext
     │
     ▼
TemplateHandler ──── Apply(files, vars) ──── registra TemplateSession
     │
     ▼
InterpolationHandler ── Interpolate(cmd, vars) ─► exec.InterpolatedCmd
     │
     ▼
ExecutionHandler ── runner.Run(interpolatedCmd, absWorkdir) ─► exec.RunResult
     │ (exit code != 0 → error inmediato)
     ▼
ProbeHandler ── CompiledProbe().FindStringSubmatch(stdout)
     │ (sin match → error)
     ▼
OutputHandler ── ExtractVars(stdout, outputs) ─► exec.OutputVars
     │
     ▼
StepResult{Status: Success, Logs, OutputVars}
```

---

## 7. Gestión de Variables — Nuevo Diseño

### 7.1 Capas de variables (documentadas explícitamente)

```
Capa 0 — Variables de proyecto:
  project_id, project_name, project_org, project_team,
  project_workdir, environment, etc.
  → Disponibles desde el inicio, nunca cambian entre steps.

Capa 1 — Variables de pipeline por step:
  Vienen de pipeline.Step.Variables() para el ambiente actual.
  → Ya cargadas por el dominio pipeline. Execution no lee YAML.

Capa 2a — Variables shared de ejecuciones previas:
  Leídas de VarsRepository.Get("vars/shared/<step>.var").
  → Persistidas sin componente de ambiente. Disponibles en TODOS
    los ambientes del mismo paso. Si primera ejecución = vacío.

Capa 2b — Variables de ambiente de ejecuciones previas:
  Leídas de VarsRepository.Get("vars/<env>/<step>.var").
  → Específicas del ambiente. Si primera ejecución = vacío.
  → Las vars shared (capa 2a) se cargan después y tienen precedencia.

Capa 3 — Variables dinámicas por step:
  step_workdir → ruta del workdir del ambiente actual del step.
  shared_workdir → ruta del workdir shared del step.
  → Cambian en cada step. Se inyectan después de resolver capas 0-2.

Capa 4 — Variables generadas por outputs durante la ejecución:
  Extraídas de stdout de comandos via regex.
  → Se acumulan entre comandos del mismo step.
  → Al finalizar el step: las marcadas isShared=true se guardan en
    "vars/shared/<step>.var"; las demás en "vars/<env>/<step>.var".
  → Se propagan al siguiente step vía cumulativeVars.
```

### 7.2 `StepVariableBuilder` — Constructor de capas

```go
// services/step_variable_builder.go
package services

type StepVariableBuilder struct {
    resolver ports.VariableResolver
}

func NewStepVariableBuilder(resolver ports.VariableResolver) *StepVariableBuilder {
    return &StepVariableBuilder{resolver: resolver}
}

// Build aplica capas 0-3 y retorna el VariableSet resuelto.
// La capa 4 (outputs) se acumula durante la ejecución de comandos en StepRunner.
//
// Orden de precedencia (mayor número = gana en colisión de nombres):
//   0: projectVars < 1: pipelineVars < 2a: persistedEnvVars < 2b: persistedSharedVars < 3: stepCtx
func (b *StepVariableBuilder) Build(
    projectVars        vos.VariableSet, // Capa 0: variables del proyecto
    pipelineVars       vos.VariableSet, // Capa 1: variables del pipeline/ambiente
    persistedEnvVars   vos.VariableSet, // Capa 2a: outputs previos del scope env
    persistedSharedVars vos.VariableSet, // Capa 2b: outputs previos del scope shared
    stepCtx            vos.StepContext, // Capa 3: rutas dinámicas del step
) (vos.VariableSet, error) {

    accumulated := projectVars.Clone()

    // Capa 2a primero, luego 2b encima: shared tiene mayor precedencia que env
    accumulated.AddAll(persistedEnvVars)
    accumulated.AddAll(persistedSharedVars)

    // Resolver referencias en las vars de pipeline contra lo ya acumulado
    resolvedPipeline, err := b.resolver.Resolve(accumulated, pipelineVars)
    if err != nil {
        return nil, fmt.Errorf("resolver variables de pipeline: %w", err)
    }
    accumulated.AddAll(resolvedPipeline)

    // Capa 3: vars de rutas del step — no requieren resolución
    workdirVar, _ := vos.NewOutputVar(vos.VarStepWorkdir, stepCtx.EnvWorkdir(), false)
    sharedVar, _ := vos.NewOutputVar(vos.VarSharedWorkdir, stepCtx.SharedWorkdir(), false)
    accumulated.Add(workdirVar)
    accumulated.Add(sharedVar)

    return accumulated, nil
}
```

### 7.3 Persistencia y propagación entre steps

Responsabilidades del `StepRunner` (no del application orchestrator):

```go
// ANTES de ejecutar el step — cargar capas 2a y 2b por separado:
persistedEnvVars, _    := varsRepo.Get(workspace.VarsFilePath(env, stepName))
persistedSharedVars, _ := varsRepo.Get(workspace.VarsFilePath("shared", stepName))

initialVars, err := varBuilder.Build(
    projectVars,
    pipelineVars,
    persistedEnvVars,    // Capa 2a: env-specific (menor precedencia)
    persistedSharedVars, // Capa 2b: shared cross-env (mayor precedencia)
    stepCtx,
)

// DESPUÉS del step exitoso — separar outputs por scope:
envOutputVars    := result.OutputVars.Filter(func(v vos.OutputVar) bool { return !v.IsShared() })
sharedOutputVars := result.OutputVars.Filter(func(v vos.OutputVar) bool { return v.IsShared() })

// Guardar solo si hay cambios respecto a lo cargado (evitar writes innecesarios)
if !envOutputVars.Equals(persistedEnvVars) {
    varsRepo.Save(workspace.VarsFilePath(env, stepName), envOutputVars)
}
if !sharedOutputVars.Equals(persistedSharedVars) {
    // La ruta shared NO contiene el ambiente — disponible para todos los ambientes
    varsRepo.Save(workspace.VarsFilePath("shared", stepName), sharedOutputVars)
}

// Propagar TODO (env + shared) al siguiente step vía cumulativeVars:
cumulativeVars.AddAll(result.OutputVars)
```

**Invariante clave:** la ruta `vars/shared/<step>.var` no contiene el nombre del ambiente. Cuando el mismo paso se ejecuta en `prod` después de haber corrido en `develop`, las `persistedSharedVars` ya contienen los outputs de la ejecución anterior — sin importar en qué ambiente se produjeron.

---

## 8. Gestión de Templates — Restauración Garantizada

### 8.1 Problema raíz

El estado de backup vive en el servicio (`FileProcessor.backups map[string][]byte`), no en una sesión delimitada. El `defer Restore()` está comentado. Si el proceso termina con error a mitad del procesamiento, el archivo queda contaminado.

### 8.2 Solución: `TemplateSession` con ciclo de vida explícito

```go
// ports/template_applier.go
package ports

// TemplateApplier aplica variables a archivos de template en disco.
// Retorna una TemplateSession que DEBE ser restaurada por el llamador.
type TemplateApplier interface {
    Apply(files []string, vars vos.VariableSet) (TemplateSession, error)
}

// TemplateSession representa los cambios aplicados a templates en disco.
// Llamar Restore() revierte todos los archivos a su estado original.
type TemplateSession interface {
    Restore() error
}
```

```go
// Implementación interna
type fileTemplateSession struct {
    backups map[string][]byte
    fs      ports.FileSystem
}

func (s *fileTemplateSession) Restore() error {
    var firstErr error
    for path, original := range s.backups {
        if err := s.fs.WriteFile(path, original); err != nil && firstErr == nil {
            firstErr = fmt.Errorf("restaurar template %s: %w", path, err)
        }
    }
    return firstErr
}
```

```go
// En StepRunner, la restauración se garantiza con defer al nivel del step:
var sessions []ports.TemplateSession
defer func() {
    for _, s := range sessions {
        if err := s.Restore(); err != nil {
            emitter.EmitWarning(execID, fmt.Sprintf("restaurar template: %v", err))
        }
    }
}()
```

La clave del diseño: la sesión se crea en `TemplateHandler.Handle()`, se registra en `CommandExecution.sessions`, y `StepRunner` llama `RestoreAllSessions()` con `defer` al nivel del step, no del comando. Esto garantiza restauración aunque haya múltiples comandos con templates.

---

## 9. Scope Compartido — Semántica Completa y Nuevo Diseño

### 9.1 Qué significa "shared" en VexEngine

Cada paso de un pipeline (`test`, `supply`, `package`, `deploy`) puede tener un subdirectorio `shared/` dentro de su directorio de template:

```
steps/
└── 01-supply/
    ├── commands.yaml        ← comandos del paso
    ├── terraform/           ← archivos de infraestructura (scope env)
    └── shared/              ← directorio compartido entre ambientes
        └── ecr/             ← p. ej. crear repositorio ECR (una sola vez)
```

Cuando un comando en `commands.yaml` declara `workdir: "shared"` o `workdir: "shared/ecr"`:

1. **Ejecuta en el workdir físico del scope shared** (`workspace/workdir/shared/<stepName>/`) — no en el del ambiente actual
2. **Sus output vars quedan marcadas `isShared=true`**
3. **Se persisten en `vars/shared/<stepName>.var`** — sin componente de ambiente en la ruta
4. **Son cargadas en TODAS las ejecuciones** de ese paso, independientemente del ambiente

**Caso de uso:** una imagen Docker se publica en ECR una sola vez. El paso `supply` tiene comandos normales (por ambiente, p.ej. configurar Terraform workspace) y comandos en `workdir: "shared"` (push a ECR). Si se ejecuta primero en `develop` y luego en `prod`, el segundo ambiente encuentra las shared vars ya persistidas — sin re-ejecutar la publicación de la imagen.

### 9.2 Contraste: variables de ambiente vs. variables compartidas

| Característica | Scope `env` | Scope `shared` |
|---|---|---|
| Ruta de persistencia | `vars/<env>/<step>.var` | `vars/shared/<step>.var` |
| Disponibilidad | Solo en el mismo ambiente | En todos los ambientes del mismo paso |
| Directorio de ejecución | `workdir/<env>/<stepName>/` | `workdir/shared/<stepName>/` |
| Variable automática | `${var.step_workdir}` | `${var.shared_workdir}` |
| Cuándo se recarga | Cada run del paso en ese ambiente | Cada run del paso en cualquier ambiente |

### 9.3 Eliminación de `filepath.Base` frágil

```
workdir = "shared"            → Base = "shared"    ✓ correcto
workdir = "shared/ecr"        → Base = "ecr"       ✗ falso negativo
workdir = "env/shared"        → Base = "shared"    ✗ falso positivo
workdir = ""                  → Base = "."         ✓ correcto accidentalmente
```

### 9.4 Nueva lógica en `WorkdirScope` (sección 4.1)

La regla semántica correcta es: **el scope es `shared` si y solo si el primer segmento del path es `"shared"`**.

```go
func ScopeFromWorkdir(workdir string) WorkdirScope {
    if workdir == "" {
        return ScopeEnv
    }
    normalized := filepath.ToSlash(workdir)
    firstSegment := strings.SplitN(normalized, "/", 2)[0]
    if firstSegment == sharedScopeName {
        return ScopeShared
    }
    return ScopeEnv
}
```

El scope se calcula exactamente una vez al construir `CommandContext`, no en cada ejecución de handler.

### 9.5 Copia del template al workspace — dos pasadas

Antes de ejecutar un paso, se realizan **dos copias del directorio del template** al workspace del proyecto. Esto aísla el pipeline original y mantiene los dos scopes separados físicamente:

```go
// Paso A: copia para el scope del ambiente (excluye cualquier directorio "shared/")
envStepPath := workspace.ScopeWorkdirPath(environment, stepName)
copyWorkdir.Copy(ctx, stepTemplatePath, envStepPath, isShared: false)

// Paso B: copia para el scope shared (solo archivos dentro de directorios "shared/")
sharedStepPath := workspace.ScopeWorkdirPath("shared", stepName)
copyWorkdir.Copy(ctx, stepTemplatePath, sharedStepPath, isShared: true)
```

Resultado en el filesystem del workspace:

```
workspace/
└── workdir/
    ├── prod/
    │   └── supply/          ← solo archivos fuera de shared/ (terraform/, etc.)
    └── shared/
        └── supply/          ← solo archivos dentro de shared/ (ecr/, etc.)
```

La interfaz `CopyWorkdir` en `ports/copy_workdir.go` ya tiene el flag `isShared bool`. La implementación filtra por nombre de directorio: cuando `isShared=false` omite cualquier directorio llamado `"shared"`; cuando `isShared=true` solo incluye archivos cuyo ancestro es un directorio `"shared"`.

---

## 10. Multiplataforma en Infraestructura

### 10.1 Estado actual y evaluación

`os/exec` de Go no hace shell wrapping automático. Hay que construir `exec.Command("sh", "-c", cmd)` explícitamente. La librería estándar es la correcta — no hay dependencias externas necesarias para esto.

Lo que debe cambiar es la estructura: la detección de OS debe ser una Strategy inyectable para que sea testeable sin invocar shells reales.

### 10.2 Patrón Strategy inyectable (en infraestructura)

```go
// infrastructure/shell/strategy.go
package shell

// ShellStrategy define cómo envolver un comando string según el OS.
type ShellStrategy interface {
    Wrap(cmd string) (executable string, args []string)
}

type LinuxShellStrategy struct{}
func (s *LinuxShellStrategy) Wrap(cmd string) (string, []string) {
    return "sh", []string{"-c", cmd}
}

type WindowsShellStrategy struct{}
func (s *WindowsShellStrategy) Wrap(cmd string) (string, []string) {
    return "cmd.exe", []string{"/C", cmd}
}

func NewShellStrategy() ShellStrategy {
    if runtime.GOOS == "windows" {
        return &WindowsShellStrategy{}
    }
    return &LinuxShellStrategy{}
}
```

```go
// infrastructure/executor/local/command_runner.go
type CommandRunner struct {
    strategy shell.ShellStrategy
}

func (r *CommandRunner) Run(ctx context.Context, command, workDir string) (*vos.CommandResult, error) {
    exe, args := r.strategy.Wrap(command)
    cmd := exec.CommandContext(ctx, exe, args...)
    cmd.Dir = workDir
    // capturar stdout/stderr, normalizar ANSI, retornar CommandResult
}
```

Para tests de dominio, se puede inyectar una `MockShellStrategy` que retorna el comando tal como llega sin invocar nada.

### 10.3 Normalización de output

La normalización de stdout/stderr (remover ANSI escape codes, trim de espacios) es responsabilidad del `CommandRunner` en infraestructura. El dominio recibe stdout limpio y no sabe de ANSI.

---

## 11. Interfaz Pública del Dominio

### 11.1 Nuevo servicio de dominio: `PipelineRunner`

La lógica de orquestación de steps se mueve desde `application/execution_orchestrator.go` a un servicio de dominio:

```go
// domain/execution/services/pipeline_runner.go
package services

type PipelineRunRequest struct {
    ExecutionID  vos.ExecutionID
    ProjectVars  vos.VariableSet
    Steps        []StepRunRequest
}

type StepRunRequest struct {
    Name           string
    StepContext    vos.StepContext
    Commands       []execVos.Command
    PipelineVars   vos.VariableSet
    VarsEnvPath    string
    VarsSharedPath string
}

type PipelineRunner struct {
    stepRunner      ports.StepRunner
    varsRepo        ports.VarsRepository
    variableBuilder *StepVariableBuilder
    nameTranslator  ports.StepNameTranslator
    emitter         ports.LogEmitter
}

func (r *PipelineRunner) Run(ctx context.Context, req PipelineRunRequest) error
```

### 11.2 Simplificación del application layer

```go
// application/execution_orchestrator.go — responsabilidades restantes:
// 1. Clonar repositorio del pipeline (llama RepositoryFetcher)
// 2. Clonar repositorio del proyecto (llama project.RepositoryFetcher)
// 3. Resolver versión del proyecto
// 4. Construir PipelineRunRequest con los datos de dominio
// 5. Llamar pipelineRunner.Run(ctx, req)
// 6. Actualizar status del aggregate Execution (via ExecutionRepository)
```

El application layer coordina fuentes de datos externas, pero la lógica de ejecución de pasos, construcción de variables y persistencia de outputs vive en el dominio.

---

## 12. Tensión `pipeline.StepName` vs `storage.vos.StepName`

### 12.1 Naturaleza del problema

| Tipo | Naturaleza | Ejemplo |
|------|------------|---------|
| `pipeline.StepName` | Open set — cualquier `NN-nombre` | `"01-test"`, `"03-deploy-prod"` |
| `storage.vos.StepName` | Enum cerrado — 4 valores fijos | `"test"`, `"supply"`, `"package"`, `"deploy"` |

La conversión falla silenciosamente si se agrega un paso cuyo nombre no esté en el enum de storage.

### 12.2 Anti-Corruption Layer explícito

```go
// ports/step_name_translator.go
package ports

// StepNameTranslator aísla la incompatibilidad de tipos entre pipeline.StepName
// y storage.vos.StepName. Si el nombre de paso no tiene equivalente en storage,
// retorna error explicativo.
type StepNameTranslator interface {
    Translate(name pipelineDom.StepName) (storageVos.StepName, error)
}
```

```go
// services/step_name_translator.go
type StepNameACL struct{}

func (t *StepNameACL) Translate(name pipelineDom.StepName) (storageVos.StepName, error) {
    sn, err := storageVos.NewStepName(name.Name())
    if err != nil {
        return storageVos.StepName(""), fmt.Errorf(
            "el paso '%s' no tiene equivalente en storage (valores válidos: test, supply, package, deploy): %w",
            name.Name(), err,
        )
    }
    return sn, nil
}
```

A largo plazo, si `storage.vos.StepName` debe abrirse para soportar steps arbitrarios, ese cambio ocurre en el dominio `storage` sin tocar `execution`.

---

## 13. Plan de Migración Paso a Paso

### Fase 1 — Nuevos VOs (sin cambios de comportamiento)

Empezar aquí porque no rompe compilación ni tests.

1. Crear `vos/workdir_scope.go` con `WorkdirScope`, constantes y `ScopeFromWorkdir()`
2. Crear `vos/step_context.go` con `StepContext` y `NewStepContext()`
3. Crear `vos/variable_layer.go` con constantes de nombres de variables
4. Crear `vos/step_result.go` con `StepStatus` y `StepResult`
5. Modificar `vos/command_output.go`: agregar `compiledProbe` y compilar en constructor
6. Crear `vos/command_context.go` con resolución de rutas absolutas
7. **Verificar:** `go build ./...` pasa sin errores

### Fase 2 — Separar `FileProcessor` en sesión transaccional

Resolver el bug del `Restore()` comentado.

1. Crear `ports/template_applier.go` con `TemplateApplier` + `TemplateSession`
2. Crear implementación `fileTemplateSession` dentro de `services/`
3. Adaptar lógica de `services/files_processor.go` a la nueva interfaz
4. Mantener temporalmente `ports/file_processor.go` como alias para compilación
5. **Verificar:** compilación y tests pasan

### Fase 3 — Descomponer `CommandExecutor` en handlers

Es el cambio más arriesgado — hacerlo con tests previos.

1. Agregar tests de integración para el comportamiento actual de `CommandExecutor.Execute()`
2. Crear `ports/command_handler.go` con `CommandExecution` y `CommandHandler`
3. Crear `services/template_handler.go`
4. Crear `services/interpolation_handler.go`
5. Crear `services/execution_handler.go`
6. Crear `services/probe_handler.go` (usa `CompiledProbe()` del VO)
7. Crear `services/output_handler.go`
8. Crear `services/command_pipeline.go` con `CommandPipeline`
9. **Verificar:** tests de integración del paso 1 pasan

### Fase 4 — Refactorizar `StepExecutor` → `StepRunner`

1. Crear `services/step_variable_builder.go`
2. Crear `services/step_runner.go` con `StepRunner` que usa `CommandPipeline`
3. Agregar `defer exec.RestoreAllSessions()` al nivel del step en `StepRunner`
4. Actualizar `ports/step_executor.go` → `ports/step_runner.go` con nueva firma
5. Actualizar referencias en `execution_orchestrator.go`

### Fase 5 — Eliminar singletons y limpiar

1. En `services/interpolator.go`: eliminar `var defaultInterpolator` — constructor crea instancia nueva
2. En `services/output_extractor.go`: eliminar `var defaultOutputExtractor` — usar `CompiledProbe()` del VO
3. Actualizar todos los constructores que usan estas funciones

### Fase 6 — Mover lógica de dominio fuera del application layer

1. Crear `services/pipeline_runner.go` con `PipelineRunner`
2. Crear `ports/step_name_translator.go` + `services/step_name_translator.go`
3. Mover construcción de variables (capas 0-3), copia de workdir y persistencia de vars desde `execution_orchestrator.go` al `PipelineRunner`
4. Simplificar `execution_orchestrator.go` para que solo coordine (Clone, Fetch, Version, → `pipelineRunner.Run`)

### Fase 7 — Limpieza y eliminación de archivos obsoletos

1. Eliminar `ports/file_processor.go`
2. Eliminar `services/files_processor.go`
3. Confirmar que `services/command_executor.go` puede eliminarse
4. Renombrar archivos de ports según la nueva nomenclatura
5. **Verificar:** `go build ./...` y `go test ./...` pasan completamente

---

## 14. Criterios de Éxito

### 14.1 Criterios de código

- [ ] `services/command_executor.go` no existe. Su lógica está en 5 handlers <= 50 líneas cada uno
- [ ] `services/step_executor.go` no existe. `services/step_runner.go` orquesta sin lógica de construcción de variables inline
- [ ] `defer session.Restore()` existe y NO está comentado en ningún lugar
- [ ] `filepath.Base(command.Workdir()) == ...` no aparece en ningún archivo de dominio
- [ ] `var defaultInterpolator` y `var defaultOutputExtractor` no existen
- [ ] Las regex de probes se compilan exactamente una vez (en el constructor de `CommandOutput`)
- [ ] No hay magic strings de nombres de variables en `services/` — todo usa constantes de `variable_layer.go`
- [ ] Las firmas de `StepRunner` y `CommandHandler` son simétricas en cuanto a manejo de errores

### 14.2 Criterios de arquitectura

- [ ] El application layer no contiene lógica de construcción de variables, filtrado de OutputVars, ni decisiones de rutas de workspace
- [ ] Cada servicio en `services/` implementa exactamente una interfaz de `ports/`
- [ ] No hay imports cruzados entre servicios de dominio (solo hacia `ports/` y `vos/`)
- [ ] `execution/` no importa directamente paquetes de `storage/` ni `pipeline/` — usa sus propios ports

### 14.3 Criterios de comportamiento (no-regresión)

- [ ] Los tests existentes pasan sin modificación
- [ ] Archivos de template interpolados en el paso N no afectan el estado del paso N+1 (idempotencia restaurada)
- [ ] Un step marcado como `Cached` no ejecuta comandos ni modifica el filesystem
- [ ] Variables marcadas `isShared=true` se persisten en `vars/shared/<step>.var` (sin componente de ambiente)
- [ ] Variables marcadas `isShared=false` se persisten en `vars/<env>/<step>.var`
- [ ] Si `supply` corre en `develop` produciendo shared vars, al correr en `prod` esas vars ya están disponibles sin re-ejecutar los comandos shared
- [ ] `ScopeFromWorkdir("shared/ecr")` devuelve `ScopeShared` (no falla como el `filepath.Base` actual)
- [ ] El workdir físico de un comando con `workdir: "shared/ecr"` resuelve a `workspace/workdir/shared/<stepName>/shared/ecr/`
- [ ] El mismo pipeline puede ejecutarse en Linux, macOS y Windows sin cambiar código de dominio

### 14.4 Criterios de testabilidad

- [ ] `CommandPipeline` puede testearse con mocks de handlers independientemente
- [ ] `StepVariableBuilder` puede testearse sin dependencias de filesystem
- [ ] `TemplateHandler` puede testearse con un `TemplateApplier` mock
- [ ] `ShellStrategy` puede mockearse para tests de `CommandRunner` sin invocar procesos reales
- [ ] `StepNameTranslator` puede mockearse para simular pasos no registrados en storage
