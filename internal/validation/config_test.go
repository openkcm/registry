package validation_test

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/openkcm/registry/internal/validation"
)

type Config struct {
	Validations []validation.ConfigField `yaml:"validations"`
}

func TestConfig(t *testing.T) {
	config := `
    validations:
    - id: Auth.Type
      constraints:
      - type: list
        spec:
          allowlist: ["oidc"]
    - id: Auth.Properties.Issuer
      omitIdCheck: true
      constraints:
      - type: non-empty`

	v := viper.New()
	v.SetConfigType("yaml")

	err := v.ReadConfig(strings.NewReader(config))
	assert.NoError(t, err)

	var cfg Config
	err = v.Unmarshal(&cfg)
	assert.NoError(t, err)

	assert.Len(t, cfg.Validations, 2)

	assert.Equal(t, validation.ID("Auth.Type"), cfg.Validations[0].ID)
	assert.Len(t, cfg.Validations[0].Constraints, 1)
	assert.Equal(t, "list", cfg.Validations[0].Constraints[0].Type)
	assert.NotNil(t, cfg.Validations[0].Constraints[0].Spec)
	assert.NotEmpty(t, cfg.Validations[0].Constraints[0].Spec.AllowList)
	assert.Equal(t, []string{"oidc"}, cfg.Validations[0].Constraints[0].Spec.AllowList)

	assert.Equal(t, validation.ID("Auth.Properties.Issuer"), cfg.Validations[1].ID)
	assert.True(t, cfg.Validations[1].OmitIDCheck)
	assert.Len(t, cfg.Validations[1].Constraints, 1)
	assert.Equal(t, "non-empty", cfg.Validations[1].Constraints[0].Type)
}
