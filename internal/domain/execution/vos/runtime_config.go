package vos

// RuntimeConfig contiene la configuración de runtime que llega desde el request HTTP.
// Es opcional: los executors locales no requieren imagen.
type RuntimeConfig struct {
	Image string
	Tag   string
}

// NewRuntimeConfig construye un RuntimeConfig con imagen y tag.
func NewRuntimeConfig(image, tag string) RuntimeConfig {
	return RuntimeConfig{
		Image: image,
		Tag:   tag,
	}
}

// IsEmpty reporta si la configuración está vacía (sin imagen ni tag).
// Un executor local no necesita imagen, por lo que este caso es válido.
func (rc RuntimeConfig) IsEmpty() bool {
	return rc.Image == "" && rc.Tag == ""
}
