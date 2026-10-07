# Arquitectura objetivo — Motor de Vex

> **Espacio de la solución.** Dónde vive el modelo: cómo se comporta el proceso, cómo se ordenan los
> paquetes y qué regla de dependencias lo protege. Los contextos están en `bounded-contexts.md` y sus
> relaciones, en `context-map.md`.
>
> **Vigente** desde el cierre de IT-05 (2026-09-14). Si contradice a un documento de `iteraciones/`,
> manda éste.

---

## El proceso

- **Una invocación, una operación.** Cada vez que el CLI o el portal usan el motor, arranca un
  proceso que atiende **una sola** operación del lenguaje publicado y termina. Las operaciones son:
  intentar, hacer rollback, simular, lanzar, reservar o liberar un ambiente, preguntar la causa,
  consultar el historial y dar por abandonado un intento (`DEC-05.6`, IT-07 `DEC-07.8`).
- **Nada vive entre invocaciones.** La continuidad está en el Historial y en el espacio de trabajo de cada ambiente, y la concurrencia no es un
  concepto del dominio (restricción declarada de `dominio.md`): un ambiente tiene como mucho un intento
  en curso (IT-07 `DEC-07.8`). Lo que la infraestructura guarde para
  ir más rápido, como una copia de un repositorio o un índice, no es memoria: se puede borrar sin que
  cambie ninguna decisión (`DEC-05.10`). El espacio de trabajo **no** es una copia:
  es memoria, en un lugar fijo por ambiente que el motor conoce. Si el motor no lo alcanza, el intento no
  empieza (IT-06 `DEC-06.18`).
- **Los contextos se hablan con llamadas directas y síncronas** a lo que publica el contexto de
  arriba, siempre en el sentido del context map. No hay bus de mensajes ni consistencia eventual:
  todo ocurre dentro de un proceso que empieza y acaba (`DEC-05.3`).
- **Los eventos de dominio se entregan dentro del mismo proceso**, justo después de escribir **y
  llevar al almacén** el registro que anuncian. El registro es la verdad; el evento solo avisa (`DEC-05.4`).

---

## Los paquetes

```
ecosistema/vex-engine/
├── cmd/vexd/                  raíz de composición: el único paquete que conoce todos los contextos
└── internal/
    ├── borde/                 Open Host hacia el CLI y el portal: el lenguaje publicado del motor
    ├── catalogo/
    ├── diagnostico/
    ├── definicion/
    ├── historial/             incluye la sincronización, dentro de infraestructura/
    ├── lanzamiento/
    ├── ejecucion/
    ├── resolucion/
    ├── simulacion/
    └── suministro/
```

**Un paquete de primer nivel por contexto, y las capas dentro** (`DEC-05.1`). Cada contexto repite
la misma forma:

```
internal/<contexto>/
├── dominio/            el modelo
├── aplicacion/         los casos de uso
├── infraestructura/    los adaptadores
└── publicado/          lo que ofrece a otros
```

| Capa | Qué contiene | Qué puede usar |
|---|---|---|
| `dominio/` | el modelo del contexto: sus conceptos, sus reglas y los puertos que necesita | solo su propio dominio y la biblioteca estándar |
| `aplicacion/` | los casos de uso: lo que hace el contexto cuando se le pide algo | su dominio, su `publicado/` y lo publicado por los contextos que tiene arriba |
| `infraestructura/` | los adaptadores: persistencia, traducciones (ACL) hacia los contextos de arriba, y lo que se compra | todo lo de su contexto, lo publicado por los contextos de arriba y librerías de terceros |
| `publicado/` | lo que el contexto ofrece a otros: operaciones y datos, en su lenguaje publicado | solo la biblioteca estándar |
| `reservado/` *(solo en `historial`)* | la relación del valor ofuscado (`DEC-04.7`) | solo la biblioteca estándar. Solo lo usa `resolucion` |

**Un solo módulo de Go.** Hay un binario y un desarrollador. Un módulo por contexto obligaría a
versionar cada frontera por separado, y eso no aporta ninguna traducción que las capas no den ya.

**Nombres.** Cada paquete se llama como su contexto, sin tildes ni eñes: `diagnostico`,
`definicion`, `resolucion` (`DEC-05.7`).

---

## Cómo se ve el context map en los paquetes

| Patrón de `context-map.md` | Dónde vive |
|---|---|
| **Customer–Supplier** | el contexto de arriba añade a su `publicado/` lo que necesita el de abajo |
| **Conformist** | la `aplicacion/` del de abajo usa tal cual los tipos del `publicado/` del de arriba |
| **ACL** | un adaptador en la `infraestructura/` del de abajo traduce lo publicado por el de arriba a un puerto de su propio `dominio/` |
| **Published Language** | los tipos del `publicado/` del de arriba, con su versión |
| **Open Host Service** | `internal/borde/`, hacia fuera |
| **Evento de dominio** | el de arriba declara en su `publicado/` la interfaz de quien escucha, y la raíz de composición registra a los que escuchan. El Historial no importa Lanzamiento |
| **Separate Ways** | ninguna importación entre los dos |

---

## El Historial

- **Los registros son su modelo**: hechos que se añaden y no se cambian. Lo que se deriva de ellos
  (el último despliegue, la cantidad de intentos) se calcula recorriéndolos (`DEC-05.5`).
- **Guardar y sincronizar es infraestructura**, detrás de puertos de su dominio.
- **No hay un modelo de lectura aparte.** Consultar es recorrer. Si recorrer se queda corto, se añade
  un índice derivable que no participa en ninguna decisión, sin tocar el modelo.
- **Cada registro se escribe en cuanto ocurre el hecho**, no al final del intento. Así, lo que un paso
  ya hizo en el mundo no se pierde si el proceso muere. Un intento que tiene registros y ninguno que
  diga cómo terminó está **sin desenlace**: es un hecho, y el core no le atribuye causa (`DEC-05.8`).
- **Cada registro se lleva al almacén en cuanto se escribe**: para los demás, un hecho no existe
  hasta que está llevado. Si no se puede llevar, el intento se detiene antes del siguiente paso y se cierra
  como fallido; si ni el cierre se puede llevar, queda en el historial compartido sin desenlace. Por eso el aviso de un despliegue se entrega
  **después** de llevar su registro, y toda invocación trae el historial antes de leerlo (`DEC-05.9`). Si por debajo es un fichero que se lleva o una base de
  datos queda escondido detrás de la interfaz del Historial: un registro no está escrito hasta que se
  puede leer desde cualquier máquina (IT-06 `DEC-06.18`).

---

## La regla de dependencias

Es la condición de cierre de E5: **se comprueba mecánicamente** (`DEC-05.2`).

1. `dominio/` solo usa su propio dominio y la biblioteca estándar.
2. `publicado/` y `reservado/` solo usan la biblioteca estándar.
3. De otro contexto **solo se usa `publicado/`**, y solo si ese contexto está **arriba** en el context
   map:

   | Contexto | Contextos de arriba |
   |---|---|
   | `diagnostico` | `historial` |
   | `lanzamiento` | `historial` |
   | `catalogo` | `historial` · `definicion` |
   | `ejecucion` | `historial` · `definicion` · `resolucion` · `suministro` |
   | `resolucion` | `historial` · `definicion` |
   | `simulacion` | `definicion` · `resolucion` · `suministro` |
   | `definicion` | `suministro` |
   | `historial` · `suministro` | ninguno |

4. `historial/reservado` solo lo usa `resolucion`.
5. `borde` solo usa el `publicado/` de los contextos de entrada: `ejecucion`, `simulacion`,
   `lanzamiento`, `catalogo`, `diagnostico` e `historial`. Y a `borde` solo lo usa `cmd/vexd`.
6. `cmd/vexd` puede usarlo todo: es donde se conectan los contextos.

**El comando**, desde `ecosistema/vex-engine`:

```bash
go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./... \
  | awk -v mod="$(go list -m)" -f docs/modelo/reglas-dependencias.awk
```

Imprime cada importación que rompe la regla, con la razón, y sale con 1 si hay alguna. La tabla de
contextos de arriba está copiada en el script: si cambia el context map, cambian los dos.

---

## Vocabulario de la arquitectura

Estas palabras **no** son lenguaje de ningún contexto: nombran piezas de la solución.

| Término | Qué nombra |
|---|---|
| **dominio** | la capa donde vive el modelo de un contexto |
| **aplicación** | la capa de los casos de uso de un contexto |
| **infraestructura** | la capa de los adaptadores de un contexto |
| **publicado** | lo que un contexto ofrece a otros |
| **reservado** | la única relación que no forma parte de lo publicado: el valor ofuscado, hacia Resolución |
| **borde** | el Open Host del motor hacia el CLI y el portal |
| **raíz de composición** | el único sitio que conoce todos los contextos y los conecta |

