# Terraform tiene su propio ${var.…}: este fichero no está en templates, así que el motor lo copia tal cual.
resource "azurerm_container_registry" "registro" {
  name = "${var.nombre}"
  sku  = var.sku
}
