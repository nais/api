package validate

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func init() {
	if err := validate.RegisterValidation("slackchannel", slackChannel); err != nil {
		panic(err)
	}

	if err := validate.RegisterValidation("optionalslackchannel", optionalSlackChannel, true); err != nil {
		panic(err)
	}

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		// skip if tag key says it should be ignored
		if name == "-" {
			return ""
		}
		return name
	})
}

func Validate(v interface{}) error {
	return validate.Struct(v)
}

// optionalSlackChannel validates if the field is an empty string or a valid Slack channel
func optionalSlackChannel(fl validator.FieldLevel) bool {
	return fl.Field().String() == "" || slackChannel(fl)
}

// slackChannel validates if the field is a valid Slack channel
func slackChannel(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	return strings.HasPrefix(s, "#") && len(s) >= 3 && len(s) <= 80
}
