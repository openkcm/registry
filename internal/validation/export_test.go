package validation

var GetValidators = getValidators

func GetValidator(c Constraint) (Validator, error) {
	return c.getValidator()
}
