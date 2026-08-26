# Bounded Contexts

| Bounded Context               | Subdominio                | Rol |
|--------------------------------|----------------------------|-----|
| **Diagnóstico**                | Core                       | Explota Registro + Definición: promedios de éxito, cambios detectados, información objetiva |
| **Registro de Despliegue**     | Registro de despliegue     | Fuente de verdad: intento → despliegue → lanzamiento, versión de producto, rollback |
| **Orquestación de Pipeline**   | Ejecución de pipeline      | Coordina el intento real, decide qué re-ejecutar, deja constancia en Registro |
| **Ejecución de Step**          | Ejecución de pipeline      | Corre los comandos reales de un step puntual |
| **Simulación de Pipeline**     | Simulación de pipeline     | Orquesta+"ejecuta" en memoria sin comandos reales; genera salidas simuladas vía regex declarada; nunca persiste |
| **Definición de Pipeline**     | Definición de pipeline     | Esquema declarado, reglas de re-ejecución, forma de salida esperada (regex) |
| **Resolución de Variables**    | Resolución de variables    | Extrapolación real, ámbito lógico/físico de variables |
| **Espacio de Trabajo**         | Espacio de trabajo         | Copia física mutable del pipeline por proyecto; ciclo de vida propio |
| **Suministro del Proyecto**    | Gestión del proyecto       | Clona/symlink del repo del proyecto + fingerprint |
| **Suministro del Pipeline**    | Gestión del repositorio de pipeline | Clona (intervalo de refresco) del repo del pipeline + fingerprint |
| **Sincronización de Estado**   | Sincronización de estado   | Backend local/remoto |