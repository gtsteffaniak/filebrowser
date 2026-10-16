package settings

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestOfficeSecretRequired(t *testing.T) {
	cases := []struct {
		name    string
		office  OnlyOffice
		wantErr bool
	}{
		{"not configured", OnlyOffice{}, false},
		{"onlyoffice with secret", OnlyOffice{Url: "https://oo.example", Secret: "s"}, false},
		{"onlyoffice without secret", OnlyOffice{Url: "https://oo.example"}, true},
		{"explicit onlyoffice without secret", OnlyOffice{Url: "https://oo.example", Product: OfficeProductOnlyOffice}, true},
		{"collabora without secret", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora}, true},
		{"collabora with secret", OnlyOffice{Url: "https://cool.example", Secret: strongCollaboraSecret, Product: OfficeProductCollabora}, false},
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

// strongCollaboraSecret has the shape of `openssl rand -base64 32` output.
const strongCollaboraSecret = "q3F8vZ1kR7wN2xT5mB9cL4hJ6pD0sG8aYe1Uo5Ii3Wc="

func TestValidateOfficeSecretStrength(t *testing.T) {
	cases := []struct {
		name    string
		office  OnlyOffice
		wantErr bool
	}{
		{"collabora, short secret", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora, Secret: "s"}, true},
		{"collabora, 31 characters", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora, Secret: strongCollaboraSecret[:31]}, true},
		{"collabora, 32 characters", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora, Secret: strongCollaboraSecret[:32]}, false},
		{"collabora, generated secret", OnlyOffice{Url: "https://cool.example", Product: OfficeProductCollabora, Secret: strongCollaboraSecret}, false},
		// OnlyOffice's secret must match the Document Server's: no length rule.
		{"onlyoffice, short secret", OnlyOffice{Url: "https://oo.example", Secret: "s"}, false},
		{"not configured", OnlyOffice{}, false},
	}
	for _, c := range cases {
		err := ValidateOfficeSecret(c.office)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: ValidateOfficeSecret() error = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
