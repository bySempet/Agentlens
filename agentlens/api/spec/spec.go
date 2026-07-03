// Package spec embebe la especificación OpenAPI de la API para servirla en
// tiempo de ejecución (GET /openapi.yaml) y mantenerla junto al código.
package spec

import _ "embed"

//go:embed openapi.yaml
var OpenAPIYAML []byte
