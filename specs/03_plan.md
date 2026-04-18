# Plan de Refactorización: Desacoplamiento Git, SRP y Eliminación de Rutas del Dominio

**Fecha:** 2026-04-18  
**Repositorio:** `github.com/jairoprogramador/vex-engine`  
**Estado:** Pendiente de implementación

---

## 1. Diagnóstico

### Problema 1 — Violación de SRP en `GitClonerTemplate`

**Archivo:** `internal/infrastructure/project/git_cloner_template.go`

`GitClonerTemplate` implementa dos responsabilidades distintas:
- `EnsureCloned`: detectar si un repo ya está clonado y ejecutar `git clone` si no lo está.
- `Run`: ejecutar comandos de shell arbitrarios.

`Run` ya tiene una implementación canónica en `internal/infrastructure/execution/shell_command_runner.go` (`ShellCommandRunner`). La copia en `GitClonerTemplate` es código duplicado con una diferencia de comportamiento material: `ShellCommandRunner` captura stdout y stderr por separado con streams dedicados; `GitClonerTemplate.Run` usa `cmd.CombinedOutput()` y devuelve un `*ports.CommandResultDTO` en lugar del `*vos.CommandResult` que usa el resto del sistema.

El puerto que define este contrato, `domain/project/ports.ClonerTemplate`, hereda la violación al incluir `Run` en su interfaz. Ningún dominio debería exigir que su colaborador sepa ejecutar comandos shell — eso es una implementación técnica, no un contrato de negocio.

**Por qué importa:** Si mañana se cambia el runner de shell (por ejemplo, para capturar logs por línea en lugar de en bloque), habría que modificar dos implementaciones distintas. El bug ya existe: los tests de `GitClonerTemplate_Run` prueban un método que no debería existir.

---

### Problema 2 — Acoplamiento cruzado entre dominios en `GitFetcher`

**Archivo:** `internal/infrastructure/pipeline/git_fetcher.go`

`GitFetcher` (infraestructura del dominio `pipeline`) importa `domain/project/ports.ClonerTemplate`, que es un puerto del dominio `project`. Esto crea dependencia entre dos dominios que deben ser autónomos.

```go
import proPrt "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
```

**Por qué importa:** El dominio `pipeline` no tiene razón para conocer la existencia del dominio `project`. Si el dominio `project` evoluciona o su puerto cambia, `GitFetcher` se rompe. La separación de dominios deja de ser una propiedad del código y se convierte en una convención frágil.

---

### Problema 3 — Duplicación de `resolveLocalPath` con lógica divergente

- `internal/infrastructure/pipeline/local_path.go`: usa SHA-256 sobre la URL completa para generar un sufijo de 8 caracteres. Esta es la implementación correcta.
- `internal/domain/pipeline/services/plan_builder.go`: usa solo `url.Name()` sin hash.

El resultado es que `GitFetcher` clona el repo en `<baseDir>/<name>-<hash8>` y `PlanBuilder` lee desde `<baseDir>/<name>`. El directorio nunca coincide. La ejecución solo funciona si hay un solo repo con ese nombre base o si la ruta ya existe por otra razón.

El comentario en el código dice "duplicada aquí para que el domain service sea autocontenido en tests", lo que confirma que la duplicación fue intencional pero la divergencia no.

**Por qué importa:** Es un bug latente que se activa con repos de organizaciones diferentes que tienen el mismo nombre (`org-a/myapp` vs `org-b/myapp`). Además viola la regla de que el dominio no debe conocer detalles del filesystem.

---

### Problema 4 — El dominio conoce rutas del sistema de archivos

`PlanBuilder` (domain service) no solo recibe una URL y calcula `localPath` — también construye todos los subpaths internamente con `filepath.Join`:

```go
// plan_builder.go — todo esto ocurre en el dominio
filepath.Join(localPath, "environments.yaml")
filepath.Join(localPath, "steps")
filepath.Join(localPath, "steps", stepName.FullName(), "commands.yaml")
filepath.Join(localPath, "variables", env.Value(), stepName.Name()+".yaml")
```

El dominio sabe que los pipelines se almacenan en un filesystem, que los entornos están en `environments.yaml`, que los pasos están en `steps/`, y que las variables siguen la convención `variables/<env>/<step>.yaml`. Toda esa estructura es un detalle de la implementación concreta `yamlFileReader` (infrastructure), no un concepto del dominio.

Si el storage cambia (por ejemplo, a una base de datos o a una caché en memoria), el domain service `PlanBuilder` tendría que modificarse aunque no sea responsable del almacenamiento.

---

## 2. Decisiones de Arquitectura

### Decisión A — Crear `infrastructure/git` como paquete compartido de infrastructure

La funcionalidad de clonación git (`EnsureCloned`) es una capacidad de infrastructure que tanto `infrastructure/pipeline` como `infrastructure/project` necesitan. La solución es extraerla a un paquete de infrastructure compartido:

```
internal/infrastructure/git/
    cloner.go          — GitCloner struct con EnsureCloned
    cloner_test.go     — tests migrados desde git_cloner_template_test.go
```

Este paquete es infrastructure pura: no importa ningún dominio, no define ports. Simplemente encapsula la mecánica de `git clone` usando `ShellCommandRunner` inyectado.

---

### Decisión B — Interfaz local `gitCloner` en `infrastructure/pipeline` (sin puerto de dominio)

Para romper el acoplamiento cruzado entre dominios en `GitFetcher`, la solución correcta en Go es definir una interfaz mínima **en el lado del consumidor**, dentro del propio paquete `infrastructure/pipeline`:

```go
// infrastructure/pipeline/git_fetcher.go — interfaz local, privada al paquete
type gitCloner interface {
    EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
}
```

`NewGitFetcher` recibe esta interfaz. `*infrastructure/git.GitCloner` la satisface implícitamente (duck typing). El dominio `project` queda desacoplado.

**Por qué no crear `domain/pipeline/ports.GitCloner`:** Un puerto de dominio debería representar una capacidad del negocio, no un mecanismo técnico. `GitCloner` nombra una tecnología (`git`) y describe *cómo* se obtiene un repositorio, no *qué* significa para el dominio pipeline. Elevar ese detalle al dominio es incorrecto. El dominio pipeline ya tiene `ports.RepositoryFetcher` — ese es el contrato correcto: "dame acceso al repositorio", sin mencionar git.

Definir la interfaz localmente en el paquete consumidor es el patrón Go idiomático: interfaces pequeñas, en el lado del consumidor, sin visibilidad innecesaria.

---

### Decisión C — Simplificar `domain/project/ports.ClonerTemplate`

El puerto `ClonerTemplate` actual tiene dos métodos: `EnsureCloned` y `Run`. Una vez que `GitCloner` (infrastructure) inyecta `ShellCommandRunner` en lugar de implementar `Run` directamente, el puerto de dominio puede reducirse:

```go
// domain/project/ports/cloner_template.go  — DESPUÉS
type ClonerTemplate interface {
    EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
}
```

Eliminar `CommandResultDTO` y el método `Run` de la interfaz.

`ExecutionOrchestrator` en application solo llama `EnsureCloned` — no llama `Run`. El cambio es no-breaking para application.

---

### Decisión D — Mover el conocimiento de filesystem al Reader (infrastructure)

El problema no es solo que `PlanBuilder` calcule `localPath` — es que construye todos los subpaths internamente. La solución es que la interfaz `Reader` reciba un `pathBase` opaco y que cada método de la implementación concreta construya sus rutas internamente.

**`pathBase` es opaco para el dominio**: es el identificador que retorna `RepositoryFetcher.Fetch()` — podría ser un directorio, una clave de caché, o cualquier cosa que la implementación infrastructure decida.

#### Cambio en la interfaz `Reader` (definida en el dominio)

```go
// ANTES — el dominio construye paths completos y los pasa al Reader
type Reader interface {
    ReadEnvironments(ctx context.Context, sourcePath string) ([]pipeline.Environment, error)
    ReadStepNames(ctx context.Context, stepsDir string) ([]pipeline.StepName, error)
    ReadCommands(ctx context.Context, commandsFilePath string) ([]pipeline.Command, error)
    ReadVariables(ctx context.Context, variablesFilePath string) ([]pipeline.Variable, error)
}

// DESPUÉS — el dominio pasa pathBase opaco; cada método sabe cómo encontrar su recurso
type Reader interface {
    ReadEnvironments(ctx context.Context, pathBase string) ([]pipeline.Environment, error)
    ReadStepNames(ctx context.Context, pathBase string) ([]pipeline.StepName, error)
    ReadCommands(ctx context.Context, pathBase string, stepName pipeline.StepName) ([]pipeline.Command, error)
    ReadVariables(ctx context.Context, pathBase string, env pipeline.Environment, stepName pipeline.StepName) ([]pipeline.Variable, error)
}
```

`PlanBuilder` nunca más construye paths. Pasa `pathBase` al Reader y recibe los datos:

```go
func (b *PlanBuilder) Load(
    ctx context.Context,
    pathBase string,      // opaco — viene de RepositoryFetcher.Fetch()
    envName string,
    limit pipeline.StepLimit,
) (*pipeline.PipelinePlan, error) {
    environment, err := b.resolveEnvironment(ctx, pathBase, envName)
    // ...
}

func (b *PlanBuilder) resolveEnvironment(ctx context.Context, pathBase, envName string) (pipeline.Environment, error) {
    environments, err := b.reader.ReadEnvironments(ctx, pathBase)
    // ...
}
```

La implementación concreta `yamlFileReader` (infrastructure) absorbe toda la lógica de construcción de paths:

```go
// infrastructure/pipeline/yaml_file_reader.go
func (r *yamlFileReader) ReadEnvironments(ctx context.Context, pathBase string) ([]pipeline.Environment, error) {
    return r.readEnvironmentsFromFile(ctx, filepath.Join(pathBase, "environments.yaml"))
}

func (r *yamlFileReader) ReadStepNames(ctx context.Context, pathBase string) ([]pipeline.StepName, error) {
    return r.readStepNamesFromDir(ctx, filepath.Join(pathBase, "steps"))
}

func (r *yamlFileReader) ReadCommands(ctx context.Context, pathBase string, stepName pipeline.StepName) ([]pipeline.Command, error) {
    return r.readCommandsFromFile(ctx, filepath.Join(pathBase, "steps", stepName.FullName(), "commands.yaml"))
}

func (r *yamlFileReader) ReadVariables(ctx context.Context, pathBase string, env pipeline.Environment, stepName pipeline.StepName) ([]pipeline.Variable, error) {
    return r.readVariablesFromFile(ctx, filepath.Join(pathBase, "variables", env.Value(), stepName.Name()+".yaml"))
}
```

El dominio `pipeline` queda libre de `filepath`, `os`, y de cualquier convención de estructura de directorios.

#### Cambio en `PipelineLoader.Load`

```go
// domain/pipeline/ports/pipeline_loader.go

// ANTES
Load(ctx context.Context, url pipeline.RepositoryURL, env string, limit pipeline.StepLimit) (*pipeline.PipelinePlan, error)

// DESPUÉS
Load(ctx context.Context, pathBase string, env string, limit pipeline.StepLimit) (*pipeline.PipelinePlan, error)
```

`pathBase` es el valor que retorna `RepositoryFetcher.Fetch()`. El caller obtiene el `pathBase` del fetcher y lo pasa directamente al loader.

**Impacto en `ValidatePipelineUseCase`:** Ya llama `fetcher.Fetch()` pero descarta el resultado con `_`. Con el cambio, lo usa:

```go
// ANTES — localPath se descartaba
localPath, err := uc.fetcher.Fetch(ctx, repoURL, ref)
// ...
plan, err := uc.loader.Load(ctx, repoURL, "", ...)  // pasaba url, no localPath

// DESPUÉS — localPath se pasa al loader
localPath, err := uc.fetcher.Fetch(ctx, repoURL, ref)
// ...
plan, err := uc.loader.Load(ctx, localPath, "", ...)
```

**Impacto en `ExecutionOrchestrator`:** `buildPlan` actualmente llama `pipelineLoader.Load(ctx, repoURL, env, limit)` sin pasar por `GitFetcher`. Con el refactor, `buildPlan` debe recibir `localPath` obtenido de `fetcher.Fetch()`. Se inyecta `ports.RepositoryFetcher` en `ExecutionOrchestrator` y se coordina fetch→load explícitamente:

```go
func (o *ExecutionOrchestrator) fetchAndBuildPlan(ctx context.Context, cmd dto.RequestInput) (*pipDom.PipelinePlan, error) {
    repoURL, err := pipDom.NewRepositoryURL(cmd.Pipeline.URL)
    // ...
    ref, err := pipDom.NewRepositoryRef(cmd.Pipeline.Ref)
    // ...
    localPath, err := o.fetcher.Fetch(ctx, repoURL, ref)
    if err != nil {
        return nil, fmt.Errorf("obtener repositorio pipeline: %w", err)
    }

    limit := pipDom.NewStepLimit(cmd.Execution.Step)
    plan, err := o.pipelineLoader.Load(ctx, localPath, cmd.Execution.Environment, limit)
    if err != nil {
        return nil, fmt.Errorf("cargar plan: %w", err)
    }
    return plan, nil
}
```

---

### Decisión E — Canonizar `resolveLocalPath` en `infrastructure/pipeline`

Con `PlanBuilder` eliminado de la ecuación, `resolveLocalPath` con hash SHA-256 vive únicamente en `internal/infrastructure/pipeline/local_path.go`. No hay duplicación.

---

## 3. Diagrama de Dependencias

### Antes

```
infrastructure/pipeline/git_fetcher.go
    └── imports domain/project/ports  (VIOLACION: cross-domain)
              └── ClonerTemplate { EnsureCloned, Run }  (VIOLACION: SRP)

infrastructure/project/git_cloner_template.go
    └── implements domain/project/ports.ClonerTemplate
    └── Run() — duplica ShellCommandRunner con tipo distinto (CommandResultDTO)

domain/pipeline/services/plan_builder.go
    └── recibe baseDir string  (VIOLACION: filesystem en dominio)
    └── resolveLocalPath() sin hash  (BUG: ruta distinta a la de GitFetcher)
    └── filepath.Join() para construir subpaths  (VIOLACION: estructura de dirs en dominio)

infrastructure/pipeline/local_path.go
    └── resolveLocalPath() con hash  (implementación correcta, pero duplicada)
```

```
cmd/vexd/factory.go
    ├── projInfra.NewGitClonerTemplate()  →  gitCloner (proPrt.ClonerTemplate)
    ├── pipInfra.NewGitFetcher(gitCloner, baseDir)  — gitCloner es project port
    └── pipInfra.NewYamlPipelineLoader(baseDir)     — PlanBuilder con baseDir
```

### Después

```
infrastructure/git/cloner.go (GitCloner)
    └── inyecta execution/ports.CommandRunner  (ShellCommandRunner)
    └── no implementa ningún puerto de dominio directamente
    └── satisface gitCloner (interfaz local de infrastructure/pipeline) por duck typing
    └── satisface domain/project/ports.ClonerTemplate por duck typing

infrastructure/pipeline/git_fetcher.go
    └── define interfaz local privada: gitCloner { EnsureCloned }
    └── inyecta gitCloner (interfaz local — sin cross-domain)
    └── resuelve localPath con hash SHA-256 (local_path.go)
    └── Fetch() retorna localPath

infrastructure/pipeline/yaml_pipeline_loader.go
    └── recibe localPath de Fetch()
    └── pasa localPath a PlanBuilder.Load()
    └── yamlFileReader construye todos los filepath.Join internamente

domain/pipeline/services/plan_builder.go
    └── Load(ctx, pathBase string, env, limit)  — sin baseDir, sin URL, sin filepath
    └── Reader recibe pathBase opaco — cada método sabe cómo encontrar su recurso
    └── dominio libre de os, filepath, convenciones de directorio

domain/project/ports/cloner_template.go
    └── ClonerTemplate { EnsureCloned }  — Run eliminado
```

```
cmd/vexd/factory.go
    ├── gitInfra.NewGitCloner(runner)         →  gitCloner
    ├── pipInfra.NewGitFetcher(gitCloner, baseDir)
    └── pipInfra.NewYamlPipelineLoader()      — sin baseDir
```

**Grafo de dependencias resultante (estricto):**

```
domain/pipeline/ports.RepositoryFetcher  ◀──  infrastructure/pipeline.GitFetcher
                                                    │ (localPath/pathBase)
                                                    ▼
                                         infrastructure/pipeline.YamlPipelineLoader
                                                    │ (pathBase opaco)
                                                    ▼
                                         domain/pipeline/services.PlanBuilder
                                                    │ (pathBase opaco)
                                                    ▼
                                         domain/pipeline.Reader (interfaz)
                                                    ▲
                                         infrastructure/pipeline.yamlFileReader
                                              (construye filepath.Join aquí)
```

---

## 4. Plan de Implementación Paso a Paso

Cada paso debe compilar antes de pasar al siguiente. Se indica qué archivos se crean, modifican o eliminan.

---

### Paso 1 — Crear `infrastructure/git` con `GitCloner`

**Nuevo archivo:** `internal/infrastructure/git/cloner.go`

```go
package git

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    execPorts "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
)

type GitCloner struct {
    runner execPorts.CommandRunner
}

func NewGitCloner(runner execPorts.CommandRunner) *GitCloner {
    return &GitCloner{runner: runner}
}

// EnsureCloned garantiza que repoURL@ref esté disponible en localPath.
// Si el directorio ya existe y es un repo git, no hace nada.
func (c *GitCloner) EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error {
    if _, err := os.Stat(localPath); err == nil {
        isGit, err := isGitRepository(localPath)
        if err != nil {
            return fmt.Errorf("git cloner: verificar repositorio: %w", err)
        }
        if isGit {
            return nil
        }
    }

    command := fmt.Sprintf("git clone --branch %s %s %s", ref, repoURL, localPath)
    res, err := c.runner.Run(ctx, command, "")
    if err != nil {
        return fmt.Errorf("git cloner: iniciar clonación: %w", err)
    }
    if res.ExitCode != 0 {
        return fmt.Errorf("git cloner: clonar '%s' (código %d): %s", repoURL, res.ExitCode, res.CombinedOutput())
    }
    return nil
}

func isGitRepository(path string) (bool, error) {
    _, err := os.Stat(filepath.Join(path, ".git"))
    if err == nil {
        return true, nil
    }
    if os.IsNotExist(err) {
        return false, nil
    }
    return false, err
}

// Verificaciones de contrato — domain/project ve GitCloner como ClonerTemplate.
var _ proPorts.ClonerTemplate = (*GitCloner)(nil)
```

> Nota: el `var _` de `proPorts.ClonerTemplate` se agrega después del Paso 2. No se crea ningún puerto de dominio `pipeline.GitCloner` — la interfaz local del Paso 2 cumple ese rol.

**Nuevo archivo:** `internal/infrastructure/git/cloner_test.go`

Migrar los tests de `internal/infrastructure/project/git_cloner_template_test.go`. Los tests de `TestGitClonerTemplate_EnsureCloned` se mueven intactos (ajustar imports y nombre de constructor). Los tests de `TestGitClonerTemplate_Run` se eliminan — ese método ya no existe.

---

### Paso 2 — Agregar interfaz local `gitCloner` en `infrastructure/pipeline`

**Modificar:** `internal/infrastructure/pipeline/git_fetcher.go`

Reemplazar el import de `domain/project/ports` con una interfaz local mínima definida dentro del mismo archivo:

```go
package pipeline

import (
    "context"
    "fmt"
    "os"

    "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
    "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
)

// gitCloner es una interfaz local que abstrae la operación de clonación.
// Se define aquí (lado del consumidor) para romper el acoplamiento con domain/project.
// infrastructure/git.GitCloner la satisface implícitamente.
type gitCloner interface {
    EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
}

type GitFetcher struct {
    cloner  gitCloner
    baseDir string
}

func NewGitFetcher(cloner gitCloner, baseDir string) ports.RepositoryFetcher {
    return &GitFetcher{cloner: cloner, baseDir: baseDir}
}

var _ ports.RepositoryFetcher = (*GitFetcher)(nil)

func (f *GitFetcher) Fetch(ctx context.Context, url pipeline.RepositoryURL, ref pipeline.RepositoryRef) (string, error) {
    localPath := resolveLocalPath(f.baseDir, url)

    if err := os.MkdirAll(f.baseDir, 0o750); err != nil {
        return "", fmt.Errorf("git fetcher: crear directorio base: %w", err)
    }

    if err := f.cloner.EnsureCloned(ctx, url.String(), ref.String(), localPath); err != nil {
        return "", fmt.Errorf("git fetcher: %w", err)
    }

    return localPath, nil
}
```

El import `proPrt "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"` desaparece. El acoplamiento cruzado queda eliminado.

**Actualizar `cmd/vexd/factory.go`:**

```go
// ANTES
gitCloner := projInfra.NewGitClonerTemplate()
gitFetcher := pipInfra.NewGitFetcher(gitCloner, pipelinesBaseDir)

// DESPUÉS
gitCloner  := gitInfra.NewGitCloner(runner)
gitFetcher := pipInfra.NewGitFetcher(gitCloner, pipelinesBaseDir)
```

`gitCloner` también se pasa a `ExecutionOrchestrator` como `proPrt.ClonerTemplate` — `*git.GitCloner` satisface esa interfaz (misma firma `EnsureCloned`). Sin cambios en `ExecutionOrchestrator` para este paso.

---

### Paso 3 — Simplificar `domain/project/ports.ClonerTemplate`

**Modificar:** `internal/domain/project/ports/cloner_template.go`

```go
// ANTES
type CommandResultDTO struct {
    Output   string
    ExitCode int
}

type ClonerTemplate interface {
    EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
    Run(ctx context.Context, command string, workDir string) (*CommandResultDTO, error)
}

// DESPUÉS
type ClonerTemplate interface {
    EnsureCloned(ctx context.Context, repoURL, ref, localPath string) error
}
```

Eliminar `CommandResultDTO` y el método `Run` de la interfaz.

**No rompe la compilación en este paso:** `infrastructure/git.GitCloner` solo implementa `EnsureCloned` — satisface el puerto simplificado. `infrastructure/project/git_cloner_template.go` tiene `Run` extra pero Go no se queja de métodos adicionales en una implementación.

---

### Paso 4 — Cambiar la interfaz `Reader` para recibir `pathBase` opaco

Este es el paso más importante para eliminar el conocimiento de filesystem del dominio.

**Modificar:** `internal/domain/pipeline/services/plan_builder.go`

```go
// Reader — ANTES: recibe paths completos ya construidos
type Reader interface {
    ReadEnvironments(ctx context.Context, sourcePath string) ([]pipeline.Environment, error)
    ReadStepNames(ctx context.Context, stepsDir string) ([]pipeline.StepName, error)
    ReadCommands(ctx context.Context, commandsFilePath string) ([]pipeline.Command, error)
    ReadVariables(ctx context.Context, variablesFilePath string) ([]pipeline.Variable, error)
}

// Reader — DESPUÉS: recibe pathBase opaco; la implementación construye sus paths
type Reader interface {
    ReadEnvironments(ctx context.Context, pathBase string) ([]pipeline.Environment, error)
    ReadStepNames(ctx context.Context, pathBase string) ([]pipeline.StepName, error)
    ReadCommands(ctx context.Context, pathBase string, stepName pipeline.StepName) ([]pipeline.Command, error)
    ReadVariables(ctx context.Context, pathBase string, env pipeline.Environment, stepName pipeline.StepName) ([]pipeline.Variable, error)
}
```

Cambios en `PlanBuilder`:
- Eliminar campo `baseDir string` del struct.
- `NewPlanBuilder(reader Reader)` — sin `baseDir`.
- `Load(ctx, pathBase string, envName string, limit)` — `pathBase` reemplaza a `url RepositoryURL`.
- Eliminar la función `resolveLocalPath` local.
- Eliminar todos los `filepath.Join` del dominio — ya no son responsabilidad de `PlanBuilder`.
- Los métodos privados `resolveEnvironment`, `resolveSteps`, `assembleStep` pasan `pathBase` al reader sin construir sub-paths.
- Eliminar imports: `path/filepath`, `os`, y `pipeline.RepositoryURL` si deja de usarse.

```go
type PlanBuilder struct {
    reader Reader
}

func NewPlanBuilder(reader Reader) *PlanBuilder {
    return &PlanBuilder{reader: reader}
}

var _ ports.PipelineLoader = (*PlanBuilder)(nil)

func (b *PlanBuilder) Load(
    ctx context.Context,
    pathBase string,
    envName string,
    limit pipeline.StepLimit,
) (*pipeline.PipelinePlan, error) {
    environment, err := b.resolveEnvironment(ctx, pathBase, envName)
    if err != nil {
        return nil, err
    }

    stepsToExecute, err := b.resolveSteps(ctx, pathBase, limit)
    if err != nil {
        return nil, err
    }

    assembledSteps := make([]*pipeline.Step, 0, len(stepsToExecute))
    for _, stepName := range stepsToExecute {
        step, err := b.assembleStep(ctx, pathBase, stepName, environment)
        if err != nil {
            return nil, fmt.Errorf("ensamblar paso '%s': %w", stepName.Name(), err)
        }
        assembledSteps = append(assembledSteps, step)
    }

    return pipeline.NewPipelinePlan(environment, assembledSteps)
}

func (b *PlanBuilder) resolveEnvironment(ctx context.Context, pathBase, envName string) (pipeline.Environment, error) {
    environments, err := b.reader.ReadEnvironments(ctx, pathBase)
    // ... resto igual
}

func (b *PlanBuilder) resolveSteps(ctx context.Context, pathBase string, limit pipeline.StepLimit) ([]pipeline.StepName, error) {
    allStepNames, err := b.reader.ReadStepNames(ctx, pathBase)
    // ... resto igual
}

func (b *PlanBuilder) assembleStep(
    ctx context.Context,
    pathBase string,
    stepName pipeline.StepName,
    env pipeline.Environment,
) (*pipeline.Step, error) {
    commands, err := b.reader.ReadCommands(ctx, pathBase, stepName)
    // ...
    variables, err := b.reader.ReadVariables(ctx, pathBase, env, stepName)
    // ...
}
```

---

### Paso 5 — Cambiar `PipelineLoader.Load` y actualizar `YamlPipelineLoader`

**Modificar:** `internal/domain/pipeline/ports/pipeline_loader.go`

```go
type PipelineLoader interface {
    // Load construye el plan leyendo desde pathBase.
    // pathBase es opaco — viene del resultado de RepositoryFetcher.Fetch().
    Load(ctx context.Context, pathBase string, env string, limit pipeline.StepLimit) (*pipeline.PipelinePlan, error)
}
```

**Modificar:** `internal/infrastructure/pipeline/yaml_pipeline_loader.go`

`YamlPipelineLoader` ya no necesita `baseDir`. `yamlFileReader` absorbe toda la lógica de construcción de paths:

```go
type YamlPipelineLoader struct {
    planBuilder *services.PlanBuilder
}

func NewYamlPipelineLoader() ports.PipelineLoader {
    reader := &yamlFileReader{}
    builder := services.NewPlanBuilder(reader)
    return &YamlPipelineLoader{planBuilder: builder}
}

func (l *YamlPipelineLoader) Load(ctx context.Context, pathBase string, env string, limit dom.StepLimit) (*dom.PipelinePlan, error) {
    return l.planBuilder.Load(ctx, pathBase, env, limit)
}
```

**Modificar:** `internal/infrastructure/pipeline/yaml_file_reader.go` (o donde viva `yamlFileReader`)

Cada método construye sus paths internamente:

```go
func (r *yamlFileReader) ReadEnvironments(ctx context.Context, pathBase string) ([]pipeline.Environment, error) {
    // filepath.Join vive aquí, en infrastructure
    return r.readEnvironmentsFromFile(ctx, filepath.Join(pathBase, "environments.yaml"))
}

func (r *yamlFileReader) ReadStepNames(ctx context.Context, pathBase string) ([]pipeline.StepName, error) {
    return r.readStepNamesFromDir(ctx, filepath.Join(pathBase, "steps"))
}

func (r *yamlFileReader) ReadCommands(ctx context.Context, pathBase string, stepName pipeline.StepName) ([]pipeline.Command, error) {
    return r.readCommandsFromFile(ctx, filepath.Join(pathBase, "steps", stepName.FullName(), "commands.yaml"))
}

func (r *yamlFileReader) ReadVariables(ctx context.Context, pathBase string, env pipeline.Environment, stepName pipeline.StepName) ([]pipeline.Variable, error) {
    path := filepath.Join(pathBase, "variables", env.Value(), stepName.Name()+".yaml")
    return r.readVariablesFromFile(ctx, path)
}
```

**Actualizar `cmd/vexd/factory.go`:**

```go
// ANTES
pipelineLoader := pipInfra.NewYamlPipelineLoader(pipelinesBaseDir)

// DESPUÉS
pipelineLoader := pipInfra.NewYamlPipelineLoader()
```

---

### Paso 6 — Actualizar los callers de `PipelineLoader.Load`

**`internal/application/usecase/validate_pipeline.go`:**

```go
// ANTES — Load recibía url; el localPath obtenido de Fetch se descartaba
localPath, err := uc.fetcher.Fetch(ctx, repoURL, ref)
// ...
plan, err := uc.loader.Load(ctx, repoURL, envName, limit)  // pasaba url

// DESPUÉS — localPath se usa como pathBase
localPath, err := uc.fetcher.Fetch(ctx, repoURL, ref)
if err != nil { ... }
// ...
plan, err := uc.loader.Load(ctx, localPath, envName, limit)
```

**`internal/application/execution_orchestrator.go`:**

Inyectar `ports.RepositoryFetcher` en `ExecutionOrchestrator` y coordinar fetch→load explícitamente en `buildPlan` (renombrar a `fetchAndBuildPlan`):

```go
// En ExecutionOrchestrator, agregar campo:
fetcher pipPrt.RepositoryFetcher

func (o *ExecutionOrchestrator) fetchAndBuildPlan(ctx context.Context, cmd dto.RequestInput) (*pipDom.PipelinePlan, error) {
    repoURL, err := pipDom.NewRepositoryURL(cmd.Pipeline.URL)
    // ...
    ref, err := pipDom.NewRepositoryRef(cmd.Pipeline.Ref)
    // ...
    localPath, err := o.fetcher.Fetch(ctx, repoURL, ref)
    if err != nil {
        return nil, fmt.Errorf("obtener repositorio pipeline: %w", err)
    }

    limit := pipDom.NewStepLimit(cmd.Execution.Step)
    plan, err := o.pipelineLoader.Load(ctx, localPath, cmd.Execution.Environment, limit)
    if err != nil {
        return nil, fmt.Errorf("cargar plan: %w", err)
    }
    return plan, nil
}
```

Actualizar `factory.go` para inyectar `gitFetcher` en `ExecutionOrchestrator`.

---

### Paso 7 — Eliminar `infrastructure/project/git_cloner_template.go`

Una vez que `infrastructure/git.GitCloner` reemplaza a `GitClonerTemplate`:

1. Verificar que ningún paquete importe `internal/infrastructure/project` solo por `GitClonerTemplate`.
2. Eliminar `internal/infrastructure/project/git_cloner_template.go`.
3. Eliminar `internal/infrastructure/project/git_cloner_template_test.go` (los tests ya fueron migrados en el Paso 1).
4. Verificar que `factory.go` ya no importe `projInfra.NewGitClonerTemplate()`.

---

### Paso 8 — Agregar verificación de contrato en `infrastructure/git`

Con `domain/project/ports.ClonerTemplate` simplificado (solo `EnsureCloned`), agregar al final de `internal/infrastructure/git/cloner.go`:

```go
import proPorts "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"

var _ proPorts.ClonerTemplate = (*GitCloner)(nil)
```

No se agrega verificación para `domain/pipeline/ports.GitCloner` porque ese puerto no existe — la interfaz local `gitCloner` en `infrastructure/pipeline` es privada y no requiere verificación explícita.

---

## 5. Resumen de Archivos Afectados

| Acción | Archivo |
|---|---|
| CREAR | `internal/infrastructure/git/cloner.go` |
| CREAR | `internal/infrastructure/git/cloner_test.go` |
| MODIFICAR | `internal/domain/project/ports/cloner_template.go` — eliminar `Run` y `CommandResultDTO` |
| MODIFICAR | `internal/domain/pipeline/ports/pipeline_loader.go` — cambiar `url` por `pathBase string` |
| MODIFICAR | `internal/domain/pipeline/services/plan_builder.go` — eliminar `baseDir`, `url`, `resolveLocalPath`, todos los `filepath.Join`; cambiar `Reader` |
| MODIFICAR | `internal/infrastructure/pipeline/git_fetcher.go` — agregar interfaz local `gitCloner`, eliminar import `domain/project/ports` |
| MODIFICAR | `internal/infrastructure/pipeline/yaml_pipeline_loader.go` — eliminar `baseDir` |
| MODIFICAR | `internal/infrastructure/pipeline/yaml_file_reader.go` — absorber `filepath.Join` en cada método |
| MODIFICAR | `internal/application/usecase/validate_pipeline.go` — pasar `localPath` a `Load` |
| MODIFICAR | `internal/application/execution_orchestrator.go` — agregar `fetcher`, unificar fetch+load |
| MODIFICAR | `cmd/vexd/factory.go` — nuevo constructor, nuevas inyecciones |
| ELIMINAR | `internal/infrastructure/project/git_cloner_template.go` |
| ELIMINAR | `internal/infrastructure/project/git_cloner_template_test.go` |

**No se crea:** `internal/domain/pipeline/ports/git_cloner.go` — ese puerto no existe.

---

## 6. Riesgos y Consideraciones

### Riesgo 1 — Bug latente de rutas activado durante el refactor

El bug de `resolveLocalPath` (con vs sin hash) podría no haber causado fallos visibles si todos los repos tenían nombres únicos en el `baseDir`. Al unificar la función con hash, cualquier pipeline que fue clonado sin hash (con la ruta `<baseDir>/<name>`) no será encontrado con la nueva lógica (`<baseDir>/<name>-<hash8>`).

**Mitigación:** En entornos donde `pipelinesBaseDir` ya tiene contenido, borrar el directorio y dejar que se clone de nuevo. Documentar en las release notes del refactor.

### Riesgo 2 — `ExecutionOrchestrator` ya recibe `pipelineLoader` pero no `gitFetcher`

El orchestrator actualmente llama `pipelineLoader.Load` con una URL — si ese `Load` interno usaba la ruta sin hash, los pipelines funcionaban solo si coincidían. Al agregar `fetcher` al orchestrator, nos aseguramos de que siempre se use la ruta con hash. El comportamiento cambia para mejor, pero requiere que el pipeline esté accesible por git desde donde corre `vexd`.

### Riesgo 3 — `CommandResultDTO` eliminado de `domain/project/ports`

Si hay código externo (ej. tests de integración en otros paquetes) que use `proPrt.CommandResultDTO`, ese código rompe. Verificar con:

```bash
grep -r "CommandResultDTO" internal/
```

Con el código actual, el único uso es dentro de `git_cloner_template.go` (que se elimina) y su test (que se migra). No hay otros usos.

### Riesgo 4 — Tests de `PlanBuilder` que pasan `baseDir` y paths completos al Reader

Los tests actuales de `PlanBuilder` crean un `Reader` mock que recibe paths completos (ruta + `environments.yaml`, ruta + `steps/`, etc.). Con la nueva firma de `Reader`, los mocks cambian: reciben `pathBase` y los parámetros de contexto (`StepName`, `Environment`).

Los fixtures de disco siguen siendo los mismos — solo cambia la interfaz del mock y cómo se pasa el identificador. Los tests se simplifican porque el mock no necesita conocer la estructura de directorios.

Buscar tests afectados:
```bash
grep -r "NewPlanBuilder\|ReadEnvironments\|ReadStepNames\|ReadCommands\|ReadVariables" internal/
```

### Riesgo 5 — `RepositoryRef` en `ExecutionOrchestrator`

`buildPlan` actualmente no construye un `RepositoryRef` — el ref del pipeline viene del DTO como string. Al hacer explícito el `Fetch` en el orchestrator, es necesario construir `RepositoryRef` desde `cmd.Pipeline.Ref`. Verificar que `pipeline.NewRepositoryRef` existe y valida correctamente.

### Riesgo 6 — Interfaz `gitCloner` local vs tipo concreto

`NewGitFetcher` recibe la interfaz local `gitCloner`. `*infrastructure/git.GitCloner` la satisface implícitamente. En `factory.go`, el tipo estático que se pasa es `*git.GitCloner` — Go inferirá la satisfacción en tiempo de compilación. No hace falta una aserción `var _` porque la interfaz es privada y el compilador valida en el punto de uso.

---

## 7. Orden de Ejecución Recomendado

```
Paso 1  →  Paso 2  →  Paso 3  →  Paso 4  →  Paso 5  →  Paso 6  →  Paso 7  →  Paso 8
```

Los pasos 1 y 2 son los más importantes de hacer primero: crean la nueva estructura sin romper nada. Los pasos 3 y 4 modifican puertos e interfaces de dominio — hacerlos juntos en el mismo commit reduce el tiempo de compilación rota. Los pasos 5+6 deben ir juntos porque cambian el puerto `PipelineLoader` y todos sus callers. Los pasos 7 y 8 son limpieza final.

**Punto de compilación seguro:** Después de cada paso, ejecutar `go build ./...` desde `vex-engine/`. Después de los pasos 4+5+6 juntos, ejecutar `go test ./...`.
