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
