package validation

import "github.com/go-viper/mapstructure/v2"

// TagName is the struct tag name used for validation IDs.
const TagName = "validationID"

type (
	// Field represents a model field with its validation ID and associated validators.
	Field struct {
		ID         ID
		Validators []Validator
	}

	// Model defines an interface for models that provide their validation fields.
	Model interface {
		Validations() []Field
	}
)

// GetValues gets all values from the given model
// mapped by their validation IDs.
func GetValues(model Model) (map[ID]any, error) {
	decMap := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &decMap,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(model)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]any)
	addValuesByID(res, decMap, "")
	return res, nil
}

func addValuesByID(res map[ID]any, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		res[totalID] = v

		if nested, ok := v.(map[string]any); ok {
			addValuesByID(res, nested, totalID)
		}
	}
}

// getIDs gets all validation IDs from the given model
// structured as a map where keys are validation IDs.
func getIDs(model Model) (map[ID]struct{}, error) {
	decMap := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &decMap,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(model)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]struct{}, len(decMap))
	addIDs(res, decMap, "")
	return res, nil
}

func addIDs(res map[ID]struct{}, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		res[totalID] = struct{}{}

		if nested, ok := v.(map[string]any); ok {
			addIDs(res, nested, totalID)
		}
	}
}
