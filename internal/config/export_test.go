package config

func (o *Orbital) Validate() error {
	return o.validate()
}

func (t *TypeValidators) Validate() error {
	return t.validate()
}
