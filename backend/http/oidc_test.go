package http

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestUserInfoUnmarshaller(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		groupsClaim string
		expected    userInfo
	}{
		{
			name:        "standard groups array",
			jsonData:    `{"name":"John","email":"john@example.com","groups":["admin","users"]}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name": "John", 
					"email": "john@example.com",
					"groups": []interface{}{"admin","users"},
				},
				Groups: []string{"admin", "users"},
			},
		},
		{
			name:        "custom groups claim name",
			jsonData:    `{"name":"Jane","email":"jane@example.com","roles":["admin","users"]}`,
			groupsClaim: "roles",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name": "Jane", 
					"email": "jane@example.com",
					"roles": []interface{}{"admin","users"},
				},
				Groups: []string{"admin", "users"},
			},
		},
		{
			name:        "groups as comma-separated string",
			jsonData:    `{"name":"Bob","email":"bob@example.com","groups":"admin, users, guests"}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name": "Bob", 
					"email": "bob@example.com",
					"groups": "admin, users, guests",
				},
				Groups: []string{"admin", "users", "guests"},
			},
		},
		{
			name:        "no groups field",
			jsonData:    `{"name":"Alice","email":"alice@example.com"}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name": "Alice", 
					"email": "alice@example.com",
				},
				Groups: nil,
			},
		},
		{
			name:        "empty groups array",
			jsonData:    `{"name":"Charlie","email":"charlie@example.com","groups":[]}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name": "Charlie", 
					"email": "charlie@example.com",
					"groups": []interface{}{},
				},
				Groups: []string{},
			},
		},
		{
			name:        "preferred_username and sub",
			jsonData:    `{"sub":"auth0|123","preferred_username":"jdoe","email":"jdoe@example.com"}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"sub":                 "auth0|123",
					"preferred_username": "jdoe",
					"email":              "jdoe@example.com",
				},
				Groups: nil,
			},
		},
		{
			name:        "empty groups string",
			jsonData:    `{"name":"Dana","groups":""}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"name":   "Dana",
					"groups": "",
				},
				Groups: nil,
			},
		},
		{
			name:        "single group in array",
			jsonData:    `{"preferred_username":"single","groups":["admins"]}`,
			groupsClaim: "groups",
			expected: userInfo{
				Claims: map[string]interface{}{
					"preferred_username": "single",
					"groups":             []interface{}{"admins"},
				},
				Groups: []string{"admins"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userdata userInfo
			unmarshaller := &userInfoUnmarshaller{
				userInfo:    &userdata,
				groupsClaim: tt.groupsClaim,
			}

			err := json.Unmarshal([]byte(tt.jsonData), unmarshaller)
			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if !reflect.DeepEqual(userdata, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, userdata)
			}
		})
	}
}

func TestUserInfoUnmarshaller_InvalidJSON(t *testing.T) {
	var userdata userInfo
	u := &userInfoUnmarshaller{userInfo: &userdata, groupsClaim: "groups"}
	err := json.Unmarshal([]byte(`{invalid json}`), u)
	if err == nil {
		t.Fatal("Unmarshal expected to fail on invalid JSON")
	}
}
