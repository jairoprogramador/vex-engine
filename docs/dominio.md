# Core domain (la ventaja competitiva, donde vale invertir el mejor diseño):

- Diagnóstico

# Supporting domain (necesario, con lógica propia, pero no es tu diferenciador):

- Registro de despliegue / historial: incluye content_id, "despliegue", y "lanzamiento" como agrupación (por ahora sin reglas propias)
- Ejecución de pipeline: incluye el concepto de "intento"; decide qué re-ejecutar apoyándose en Registro
- Definición de pipeline
- Resolución de variables
- Espacio de trabajo
- Simulación de pipeline

# Generic domain (plomería que cualquiera resolvería igual, candidato a no reinventar):

- Sincronización de estado
- Gestión del proyecto a desplegar: disponibiliza el código, dueño de su propio fingerprint
- Gestión del repositorio de pipeline: disponibiliza el pipeline base, dueño de su propio fingerprint