<div align="center">
  <h1>Vex Engine</h1>
  <p><strong>El motor de ejecución que convierte plantillas de despliegue en despliegues reales.</strong></p>
  <p>Un comando. Cualquier tecnología. Cualquier nube.</p>

  <p>
    <a href="https://github.com/jairoprogramador/vex-engine/releases">
      <img src="https://img.shields.io/github/v/release/jairoprogramador/vex-engine?style=for-the-badge" alt="Latest Release">
    </a>
    <a href="https://github.com/jairoprogramador/vex-engine/blob/main/LICENSE">
      <img src="https://img.shields.io/github/license/jairoprogramador/vex-engine?style=for-the-badge" alt="License">
    </a>
  </p>
</div>

---

Tu equipo de infraestructura ya definió **cómo** se despliega. Tú solo ejecutas:

```sh
vexe deploy sand
```

**Vex Core** (`vexe`) es el motor de ejecución del ecosistema [Vex](https://github.com/jairoprogramador/vex-client). Lee la configuración de tu proyecto (`vexconfig.yaml`), clona la plantilla de despliegue asociada y ejecuta cada paso en orden: `test, supply, package y deploy`. Todo esto sin que necesites saber qué hay detras.

Java, Node, Python, Go. AWS, Azure, GCP. Terraform, Docker, Kubernetes. A `vexe` le da igual: ejecuta lo que la plantilla diga.

## Cómo encaja en el ecosistema

`vexe` no trabaja solo. Forma parte de un ecosistema donde cada pieza tiene un rol claro:

| Componente | Rol | Repositorio |
| :--- | :--- | :--- |
| **Vex Client** (`vex`) | Inicializa proyectos, selecciona plantillas, prepara el entorno de ejecución. | [vex-client](https://github.com/jairoprogramador/vex-client) |
| **Vex Engine** (`vexe`) | **Motor de ejecución.** Lee la configuración, clona la plantilla y ejecuta los pasos de despliegue. | Este repositorio |
| **Template Store** | Catálogo de plantillas organizadas por nivel de arquitectura y costo. | [vex-template-store](https://github.com/jairoprogramador/vex-template-store) |

**Flujo típico:**

1. El desarrollador ejecuta `vex init` (vex-client) para vincular su proyecto con una plantilla.
2. Esto genera un archivo `vexconfig.yaml` en el proyecto.
3. Cuando el desarrollador ejecuta `vex deploy sand`, vex-client prepara el entorno y delega la ejecución a `vexe`, que se encarga del resto.

> Si tu proyecto ya tiene un `vexconfig.yaml`, puedes usar `vexe` directamente. Sin embargo, se recomienda usar `vex` (vex-client) como herramienta principal: acepta los mismos comandos y los delega internamente a `vexe`, pero además prepara el entorno de ejecución que la plantilla necesita.

## Instalación

### macOS (Homebrew)

```sh
brew install --cask jairoprogramador/vex-engine/vexe
```

Si macOS indica que no puede verificar el desarrollador:
**Ajustes del sistema → Privacidad y seguridad → "Abrir de todos modos"**, o en terminal: `xattr -cr $(which vexe)`.

### Linux

Descarga el paquete desde la [página de Releases](https://github.com/jairoprogramador/vex-engine/releases):

```sh
# Debian / Ubuntu
sudo dpkg -i vexe_*.deb

# Red Hat / Fedora
sudo rpm -i vexe_*.rpm
```

O descarga el binario directamente:

```sh
curl -sL https://github.com/jairoprogramador/vex-engine/releases/latest/download/vexe_linux_amd64.tar.gz | tar xz
sudo mv vexe /usr/local/bin/
```

### Windows

1. Descarga `vexe_windows_amd64.zip` desde [Releases](https://github.com/jairoprogramador/vex-engine/releases).
2. Descomprime y añade `vexe.exe` a tu `PATH`.

### Verificar instalación

```sh
vexe --version
```

## Uso

La sintaxis es siempre la misma:

```sh
vexe [step] [env]
```

Donde `step` es **hasta dónde** quieres ejecutar y `env` es **en qué entorno**.

### Steps disponibles

Cada step incluye la ejecución de todos los anteriores. Si ejecutas `deploy`, se ejecutan `test → acr → supply → package → deploy`.

| Step | Qué hace | Ámbito |
| :--- | :--- | :--- |
| `test` | Ejecuta pruebas: compilación, tests unitarios, análisis de seguridad, etc. | `project` |
| `acr` | Aprovisiona el registro de contenedores, común a todos los ambientes. | `project` |
| `supply` | Aprovisiona la infraestructura del ambiente (ej: Terraform apply). | `environment` |
| `package` | Empaqueta el proyecto (ej: build de imagen Docker). | `environment` |
| `deploy` | Despliega la aplicación en el entorno indicado. | `environment` |

Los nombres y el número de steps los define la plantilla: el motor no impone ninguno. Estos son los de las plantillas oficiales.

### Entornos

Los entornos están definidos en la plantilla. Los más comunes:

| Entorno | Uso |
| :--- | :--- |
| `sand` | Sandbox para desarrollo y pruebas. |
| `stag` | Staging, pre-producción. |
| `prod` | Producción. |

### Ejemplos

```sh
# Ejecutar solo los tests en sandbox
vexe test sand

# Aprovisionar infraestructura en staging
vexe supply stag

# Despliegue completo en producción
vexe deploy prod
```

## Control de estado inteligente

`vexe` no re-ejecuta pasos innecesariamente. Calcula una **huella** (SHA-256) de todo lo que un step declara —sus comandos, su configuración, sus archivos auxiliares y sus variables— y la compara con la del último resultado registrado de ese step. Si nada cambió, el step **revive** su resultado en vez de volver a ejecutarse.

Lo que se comprueba **lo decide la plantilla, no el motor**. Cada step lo declara en su propio `config.yaml`:

```yaml
# steps/02-acr/config.yaml
scope: project                 # dónde vive su estado
rules:
  - state_changed: [pipeline]  # re-ejecutar si cambió lo que este step declara
  - max_age: 24h               # y además, si pasaron 24 horas
```

### Ámbito: `project` o `environment`

Un step declara **exactamente uno**, y es lo que decide con quién comparte su resultado:

| Ámbito | Significado | Ejemplo |
| :--- | :--- | :--- |
| `project` | el trabajo es común a todos los ambientes | crear el registro de contenedores, correr los tests |
| `environment` | el trabajo es propio del ambiente en ejecución | provisionar el cluster de `prod`, desplegar en `sand` |

Un step de ámbito `project` se ejecuta **una vez para todo el proyecto** y todos los ambientes leen su resultado. Si un step necesita hacer las dos cosas, se parte en dos steps.

### Reglas

| Regla | Se re-ejecuta si... |
| :--- | :--- |
| `state_changed: [pipeline]` | cambió algo de lo que el step declara: sus comandos, su `config.yaml`, sus plantillas o sus variables |
| `state_changed: [pipeline, project]` | además, si cambió el código del proyecto. Es lo que hace la forma corta `state_changed` |
| `max_age: <duración>` | pasó ese tiempo desde el último resultado, haya cambiado algo o no |

Las reglas se combinan con **OR**: basta que una se cumpla. Un step **sin `config.yaml` se ejecuta siempre**, que es el default seguro — ejecutar de más nunca produce un despliegue que no ocurrió.

Esto significa menos tiempo esperando, menos errores por ejecuciones duplicadas y despliegues predecibles.

> El modelo declarado sustituye a una tabla de reglas fija por nombre de step. El diseño completo, con sus decisiones y alternativas descartadas, está en [`specs/`](specs/README.md).

## Inicio rápido

### 1. Inicializa tu proyecto con vex-client

```sh
# Instala vex-client si aún no lo tienes
# Ver: https://github.com/jairoprogramador/vex-client

vex init
```

Esto genera el archivo `vexconfig.yaml` que vincula tu proyecto con una plantilla de despliegue:

```yaml
# vexconfig.yaml (ejemplo)
project:
  id: 9238fa29be...
  name: "mi-api"
  version: "1.0.0"
  team: "backend"
  organization: "acme"

template:
  url: "https://github.com/jairoprogramador/mydeploy.git"
  ref: "main"
```

### 2. Despliega con vex core

> **Recomendado:** Usa los comandos a través de [vex-client](https://github.com/jairoprogramador/vex-client) (`vex deploy [env]`), ya que el cliente prepara automáticamente el entorno de ejecución que la plantilla necesita. Si usas `vexe` directamente, deberás configurar ese entorno por tu cuenta (dependencias, herramientas de la plantilla, etc.).

```sh
# Prueba primero
vexe test sand

# Si todo pasa, despliega
vexe deploy sand
```

`vexe` se encarga de:

1. Clonar la plantilla de despliegue.
2. Ejecutar los steps definidos.
3. Registrar el estado para futuras ejecuciones.

## Contribuciones

Las contribuciones son bienvenidas. Si encuentras un error o tienes una idea, abre un [issue](https://github.com/jairoprogramador/vex-engine/issues) o envía un [pull request](https://github.com/jairoprogramador/vex-engine/pulls).

Para entender la arquitectura interna, el proyecto sigue **Domain-Driven Design** con capas separadas en `cmd/`, `internal/application/`, `internal/domain/` e `internal/infrastructure/`.

## Licencia

Distribuido bajo la [Business Source License 1.1](https://github.com/jairoprogramador/vex-engine/blob/main/LICENSE).
