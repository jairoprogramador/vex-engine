# Pipeline de ejemplo

Un pipeline en el formato 1 que **pasa la comprobación** y usa todo lo que el formato permite
(`docs/modelo/contextos/definicion.md`, «Los ficheros del pipeline»). No depende de ningún otro proyecto.

Las pruebas de `infraestructura/` lo leen tal cual y lo rompen fichero a fichero, en una copia, para ver cada
fallo de la comprobación. Este fichero no es parte del formato, y la lectura no lo mira.

| Qué usa | Dónde |
|---|---|
| una **aserción**: un `outputs` sin `name`, que no produce ninguna variable | `steps/01-pruebas` |
| un paso de **ámbito propio compartido** (`config.yaml`, `steps.registro.scope: shared`): sus variables de salida sin scope propio lo heredan, y variables compartidas declaradas una sola vez, dos de ellas otro nombre para una variable de salida | `steps/02-registro`, `variables/compartidas.yaml` |
| material con `${var.…}` propio de terraform, que no está en `templates` y se copia tal cual | `steps/02-registro/terraform/main.tf`, `steps/03-infraestructura/terraform/terraform.tfvars.sample` |
| una plantilla, variables de salida del ámbito del ambiente en ejecución, y una de ellas usada desde otro paso sin declararla | `steps/03-infraestructura`, `steps/05-despliegue`, `variables/<ambiente>/infraestructura.yaml` |
| la salida de un paso anterior usada sin declararla, `rules` por defecto y un script ejecutable | `steps/04-imagen` |
| un paso sin entrada en `config.yaml`, con `workdir`, dos plantillas y un literal que usa otro del mismo fichero | `steps/05-despliegue`, `variables/<ambiente>/despliegue.yaml` |
| un paso que usa variables declaradas en el fichero de otro paso: son del ámbito, no del paso | `steps/06-aviso` |
| dos ficheros de variables en un mismo ámbito, con el nombre que se quiera | `variables/<ambiente>/` |
| variables estándar: metadatos (`project_*`, `environment`) y generadas (`project_hash`, `step_workdir`…) | comandos y plantillas |
