// Package settings provides configuration loading, validation, and YAML generation
// for Filebrowser. External applications may import this package to reuse the
// config schema and initialization logic.
//
// Primary entry points:
//   - Initialize: load and validate config from a YAML file
//   - Config: global Settings instance after Initialize
//   - ValidateConfig, SetDefaults: config lifecycle helpers
package settings
