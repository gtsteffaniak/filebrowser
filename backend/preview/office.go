package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

type officePreviewResponse struct {
	FileURL    string `json:"fileUrl"`
	FileType   string `json:"fileType"`
	EndConvert bool   `json:"endConvert"`
	Error      string `json:"error"`
}

// GenerateOfficePreview generates a preview for an office document using OnlyOffice.
func (s *Service) GenerateOfficePreview(ctx context.Context, filetype, key, title, url string) ([]byte, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()
	data := []byte{}
	// Create the request payload
	requestPayload := map[string]interface{}{
		"Filetype":   filetype,
		"key":        key,
		"outputType": "jpg",
		"title":      title,
		"url":        url,
		"thumbnail": map[string]interface{}{
			"width":  200,
			"height": 200,
		},
	}
	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(requestPayload))
	ss, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
	if err != nil {
		return data, errors.New("could not generate a new jwt")
	}

	// Use JSON encoder with SetEscapeHTML(false)
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(requestPayload)
	if err != nil {
		return data, err
	}
	convertURL := settings.Config.Integrations.OnlyOffice.Url + "/converter"
	if settings.Config.Integrations.OnlyOffice.InternalUrl != "" {
		convertURL = settings.Config.Integrations.OnlyOffice.InternalUrl + "/converter"
	}
	// Send the request with buf.Bytes() — not jsonData
	req, err := http.NewRequestWithContext(ctx, "POST", convertURL, buf)
	if err != nil {
		return data, err
	}
	req.Header.Set("Authorization", "Bearer "+ss)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("failed to generate preview, status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}
	var response officePreviewResponse

	// Now decode the raw response into struct
	if err = json.Unmarshal(bodyBytes, &response); err != nil {
		return data, fmt.Errorf("could not decode JSON: %w", err)
	}
	if response.Error != "" {
		return data, fmt.Errorf("error from OnlyOffice: %s", response.Error)
	}

	// make get request to binary data response.FileURL and return the body as a byte array data
	resp, err = http.Get(response.FileURL)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("failed to get preview file, status code: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
