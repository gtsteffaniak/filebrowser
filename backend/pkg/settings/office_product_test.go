package settings

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestOfficeSecretRequiredOnlyForOnlyOffice(t *testing.T) {
	cases := []struct {
		name    string
		office  OnlyOffice
		wantErr bool
	}{
		{"not configured", OnlyOffice{}, false},
		{"onlyoffice with secret", OnlyOffice{Url: "https://oo.example", Secret: "s"}, false},
		{"onlyoffice without secret", OnlyOffice{Url: "https://oo.example"}, true},
		{"explicit onlyoffice without secret", OnlyOffice{Url: "https://oo.example", Product: OfficeProductOnlyOffice}, true},
		{"collabora without secret", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora}, false},
		{"unknown product", OnlyOffice{Url: "https://x.example", Secret: "s", Product: "wps"}, true},
	}
	for _, c := range cases {
		// Same validator and struct tags ValidateConfig applies to the whole config.
		err := validator.New().Struct(Integrations{OnlyOffice: c.office})
		if (err != nil) != c.wantErr {
			t.Errorf("%s: validation error = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
