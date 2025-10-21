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

	// Map defines an interface for types that can be represented as a map.
	Map interface {
		Map() map[string]any
	}
)

// GetIDs gets all validation IDs from the given input model
// structured as a map where keys are validation IDs.
func GetIDs(input any) (map[ID]struct{}, error) {
	decMap := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &decMap,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(input)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]struct{}, len(decMap))
	addIDs(res, decMap, "")
	return res, nil
}

// GetValuesByID gets all values from the given input model
// mapped by their validation IDs.
//
// If one of the values implements the Map interface,
// the keys and values of the resulting map will be flattened
// into the resulting map.
func GetValuesByID(input any) (map[ID]any, error) {
	decMap := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &decMap,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(input)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]any)
	addValuesByID(res, decMap, "")
	return res, nil
}

func addIDs(res map[ID]struct{}, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		res[totalID] = struct{}{}

		if m, ok := v.(map[string]any); ok {
			addIDs(res, m, totalID)
		}
	}
}

func addValuesByID(res map[ID]any, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		res[totalID] = v

		if m, ok := v.(map[string]any); ok {
			addValuesByID(res, m, totalID)
		}

		if m, ok := v.(Map); ok {
			addValuesByID(res, m.Map(), totalID)
		}
	}
}
