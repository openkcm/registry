package validation

var (
	GetValidators = getValidators
	GetIDs        = getIDs
)

func (c Constraint) GetValidator() (Validator, error) {
	return c.getValidator()
}

func (v *Validation) Register(fields ...Field) error {
	return v.register(fields...)
}

func (v *Validation) RegisterConfig(fields ...ConfigField) error {
	return v.registerConfig(fields...)
}

func (v *Validation) CheckIDs(sources ...map[ID]struct{}) error {
	return v.checkIDs(sources...)
}
