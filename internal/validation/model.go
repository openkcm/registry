package validation

import "github.com/go-viper/mapstructure/v2"

const TagName = "validationID"

type (
	StructField struct {
		ID         ID
		Validators []Validator
	}

	Struct interface {
		Fields() []StructField
	}

	Field interface {
		Field() StructField
	}

	Map interface {
		Map() map[string]any
	}
)

func GetFields(fields ...Field) []StructField {
	res := make([]StructField, 0, len(fields))
	for _, f := range fields {
		res = append(res, f.Field())
	}
	return res
}

func GetIDs(s Struct) (map[ID]struct{}, error) {
	m := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &m,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(s)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]struct{}, len(m))
	addIDs(res, m, "")
	return res, nil
}

func GetValuesByID(s Struct) (map[ID]any, error) {
	m := make(map[string]any)
	config := mapstructure.DecoderConfig{
		TagName: TagName,
		Result:  &m,
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(s)
	if err != nil {
		return nil, err
	}
	res := make(map[ID]any)
	addValuesByID(res, m, "")
	return res, nil
}

func addIDs(result map[ID]struct{}, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		result[totalID] = struct{}{}

		if m, ok := v.(map[string]any); ok {
			addIDs(result, m, totalID)
		}
	}
}

func addValuesByID(result map[ID]any, m map[string]any, id ID) {
	for k, v := range m {
		totalID := ID(k)
		if id != "" {
			totalID = id + "." + ID(k)
		}
		result[totalID] = v

		if m, ok := v.(map[string]any); ok {
			addValuesByID(result, m, totalID)
		}

		if m, ok := v.(Map); ok {
			addValuesByID(result, m.Map(), totalID)
		}
	}
}
