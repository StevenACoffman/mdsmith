package config

import "fmt"

// ValidateKinds returns an error if any kind named in a kind-assignment
// entry is not declared in cfg.Kinds, or if any declared kind sets a
// schema both inline (KindBody.Schema) and via the legacy
// rules.required-structure.schema: path. Front-matter kinds are
// validated at lint time via ValidateFrontMatterKinds (see engine).
func ValidateKinds(cfg *Config) error {
	if len(cfg.Kinds) == 0 && len(cfg.KindAssignment) == 0 {
		return nil
	}
	for name, body := range cfg.Kinds {
		if err := validateKindSchemaSources(name, body); err != nil {
			return err
		}
	}
	for i, entry := range cfg.KindAssignment {
		for _, name := range entry.Kinds {
			if _, ok := cfg.Kinds[name]; !ok {
				return fmt.Errorf(
					"kind-assignment[%d]: references undeclared kind %q", i, name,
				)
			}
		}
	}
	return nil
}

// validateKindSchemaSources rejects a kind that declares a schema
// through both the inline `schema:` block and the legacy
// `rules.required-structure.schema:` path. The two sources are
// equivalent but mutually exclusive; setting both makes the
// effective schema ambiguous.
func validateKindSchemaSources(name string, body KindBody) error {
	if body.Schema == nil {
		return nil
	}
	rsCfg, ok := body.Rules["required-structure"]
	if !ok {
		return nil
	}
	pathSetting, ok := rsCfg.Settings["schema"]
	if !ok {
		return nil
	}
	path, ok := pathSetting.(string)
	if !ok || path == "" {
		return nil
	}
	return fmt.Errorf(
		"kind %q: schema is declared both inline (kinds.%s.schema:) "+
			"and as a file (kinds.%s.rules.required-structure.schema: %q); "+
			"pick one source",
		name, name, name, path)
}

// ValidateFrontMatterKinds returns an error if any of the supplied front-matter
// kind names is not declared in cfg.Kinds. filePath is used in the message.
func ValidateFrontMatterKinds(cfg *Config, filePath string, kinds []string) error {
	for _, name := range kinds {
		if _, ok := cfg.Kinds[name]; !ok {
			return fmt.Errorf(
				"%s: front matter references undeclared kind %q", filePath, name,
			)
		}
	}
	return nil
}
