# How validation works

This guide covers how to declare and use validations in your models.

## Declaring Models

Models are defined using normal structs with validation tags. These structs can contain fields of any type,
provided the associated constraint supports that type.

```go
type System struct {
    ID     string  `validationID: "System.ID"`
    Region string  `validationID: "System.Region"`
    Type   string  `validationID: "System.Type"`
    Labels Map     `validationID: "System.Labels"`
}
```

Map types must implement the `validation.Map` interface to allow validation of its key-value pairs.

## Configuring Validations

Validations are configured as a list of validation IDs with associated constraints.

```yaml
validations:
    - id: System.Type
        constraints:
        - type: non-empty
    - id: System.Region
        constraints:
        - type: list
          spec:
            allowlist: ["region-application", "region-system"]
    - id: System.Labels.Team
        skipIfNotExists: true
        constraints:
        - type: non-empty
```

If a validation ID does not exist in any model, an error will be returned at initialization.
Since validations can be defined for dynamic fields in map types, the `skipIfNotExists` flag can be set to true to skip
the check at startup.

There are a couple of built-in constraints available:

| Constraint | Field Type | Description | Spec |
|------------|------------| ----------- | -----|
| `list` | string | Field must only contain allowlisted values | `allowlist`: list of allowed values |
| `non-empty` | string | Field must not be empty | (none) |
| `non-empty-keys` | validation.Map implementer | Field must not have empty keys | (none) |

## Declaring Validations

Default validations can also be declared programmatically if a model returns a list of `validation.Field`.

```go
func (s *System) ValidationFields() []validation.Field {
    return []validation.Field{
        {
            ID: "System.ID",
            Validators: []validation.Validator{
                validation.NonEmptyConstraint{},
            },
        },
    }
}
```

## Initializing Validations

Validations must be initialized using `validation.New` which expects a `validation.Config`.
`validation.Config` requires a list of config fields (see [Configuring Validations](#configuring-validations)) and models to validate.

```go
config := validation.Config{
    Fields: fields,
    Models: []validation.Model{
        &System{},
    },
}
v, err := validation.New(config)
```

## Using Validations

Once initialized, validations can be used to validate models but also individual fields.

To validate a model, first its values must be extracted by their validation IDs.
```go
values, err := validation.GetValues(system)
if err != nil {
    // handle error
}
err = v.ValidateAll(values)
```

To validate an individual field, use its validation ID.
```go
err = v.ValidateField("System.Type", system.Type)
```





