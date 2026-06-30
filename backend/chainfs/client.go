package chainfs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

const chunkSize = 10 * 1024 * 1024 // 10 MB

// FileSubmission matches the ChainFS FileSubmission schema returned by FileEncode
// and accepted by FileCreate.
type FileSubmission struct {
	Name                 *string `json:"name"`
	Directory            *string `json:"directory"`
	Description          *string `json:"description"`
	Tags                 *string `json:"tags"`
	EncMethod            *string `json:"encMethod"`
	EncString            *string `json:"encString"`
	FileSha256Hash       *string `json:"fileSha256Hash"`
	FileSizeBytes        int32   `json:"fileSizeBytes"`
	IsSegment            bool    `json:"isSegment"`
	SegmentSha256Hash    *string `json:"segmentSha256Hash"`
	SegmentStartPosition int32   `json:"segmentStartPosition"`
	SegmentSizeBytes     int32   `json:"segmentSizeBytes"`
}

// fileCreateResponse matches the FileCreateResponse schema from ChainFS.
type fileCreateResponse struct {
	Success    bool    `json:"success"`
	Message    *string `json:"message"`
	GuidValue  string  `json:"guidValue"` // FileGuid of the created file
}

// UploadFile encodes and stores a file (<=10MB) on ChainFS. Returns the FileGuid.
func UploadFile(baseUrl, bearerToken, filename string, data io.Reader, aesPassword string) (string, error) {
	submission, err := encodeChunk(baseUrl, bearerToken, filename, data, aesPassword, -1, -1)
	if err != nil {
		return "", err
	}
	return createFile(baseUrl, bearerToken, submission)
}

// UploadFileSegmented encodes and stores a file in 10MB chunks. Returns the FileGuid.
func UploadFileSegmented(baseUrl, bearerToken, filename string, reader io.ReadSeeker, totalSize int64, aesPassword string) (string, error) {
	var lastSubmission *FileSubmission
	var startByte int64

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := bytes.NewReader(buf[:n])
			submission, encErr := encodeChunk(baseUrl, bearerToken, filename, chunk, aesPassword, startByte, int64(n))
			if encErr != nil {
				return "", encErr
			}
			lastSubmission = submission
			startByte += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error reading file for upload: %w", err)
		}
	}
	if lastSubmission == nil {
		return "", fmt.Errorf("no data was read from file")
	}
	return createFile(baseUrl, bearerToken, lastSubmission)
}

// encodeChunk calls POST /api/Debug/FileEncode and returns the FileSubmission.
// Pass startByte=-1 to omit segmentation params.
func encodeChunk(baseUrl, bearerToken, filename string, data io.Reader, aesPassword string, startByte, size int64) (*FileSubmission, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("BinaryFileSubmission", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, data); err != nil {
		return nil, fmt.Errorf("failed to write file to form: %w", err)
	}

	if aesPassword != "" {
		if err := writer.WriteField("Password", aesPassword); err != nil {
			return nil, fmt.Errorf("failed to write password field: %w", err)
		}
	}
	writer.Close()

	endpoint := fmt.Sprintf("%s/api/Debug/FileEncode", baseUrl)
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create FileEncode request: %w", err)
	}

	q := req.URL.Query()
	if aesPassword != "" {
		q.Set("Encrypt", "true")
	}
	if startByte >= 0 {
		q.Set("IsSegment", "true")
		q.Set("SegmentStartByte", fmt.Sprintf("%d", startByte))
		q.Set("SegmentSizeBytes", fmt.Sprintf("%d", size))
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ChainFS FileEncode request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read FileEncode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ChainFS FileEncode returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var submission FileSubmission
	if err := json.Unmarshal(respBody, &submission); err != nil {
		return nil, fmt.Errorf("failed to parse FileEncode response: %w", err)
	}
	return &submission, nil
}

// createFile calls POST /api/NansenFile/FileCreate with a FileSubmission and returns the FileGuid.
func createFile(baseUrl, bearerToken string, submission *FileSubmission) (string, error) {
	bodyBytes, err := json.Marshal(submission)
	if err != nil {
		return "", fmt.Errorf("failed to marshal FileSubmission: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/NansenFile/FileCreate", baseUrl)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create FileCreate request: %w", err)
	}

	// Default: store on Sepolia blockchain for 5 years
	q := req.URL.Query()
	q.Set("SelectedChains", `{"Sepolia (default)":true}`)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ChainFS FileCreate request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read FileCreate response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ChainFS FileCreate returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result fileCreateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse FileCreate response: %w", err)
	}
	if !result.Success || result.GuidValue == "" {
		msg := ""
		if result.Message != nil {
			msg = *result.Message
		}
		return "", fmt.Errorf("ChainFS FileCreate failed: %s", msg)
	}
	return result.GuidValue, nil
}

// UserInfo holds the relevant subscription fields from GET /api/NansenFile/UserInfo.
type UserInfo struct {
	Subscribed           bool   `json:"subscribed"`
	EnhancedSubscription bool   `json:"enhancedSubscription"`
	IsEnterprise         bool   `json:"isEnterprise"`
	SubscriptionExpires  string `json:"subscriptionExpires"`
}

// IsActive returns true if the user has an active enhanced subscription.
func (u *UserInfo) IsActive() bool {
	return u.EnhancedSubscription
}

// GetUserInfo fetches the ChainFS user's subscription status using their Bearer token.
func GetUserInfo(baseUrl, bearerToken string) (*UserInfo, error) {
	endpoint := fmt.Sprintf("%s/api/NansenFile/UserInfo", baseUrl)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create UserInfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ChainFS UserInfo request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read UserInfo response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ChainFS UserInfo returned status %d: %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse UserInfo response: %w", err)
	}

	return &info, nil
}

// AcornToolsAccess is the response from the acorn.tools internal access check.
type AcornToolsAccess struct {
	HasAccess  bool   `json:"hasAccess"`
	PlanTier   string `json:"planTier"`
	QuotaBytes int64  `json:"quotaBytes"`
}

// CheckAcornToolsAccess verifies whether a user (identified by their Azure sub claim)
// has acorn-drive access according to the acorn.tools billing system.
func CheckAcornToolsAccess(acornToolsBaseURL, apiSecret, azureSub string) (*AcornToolsAccess, error) {
	endpoint := fmt.Sprintf("%s/api/internal/acorn-drive/access?azure_sub=%s", acornToolsBaseURL, url.QueryEscape(azureSub))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create acorn.tools request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acorn.tools access check failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read acorn.tools response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("acorn.tools access check returned status %d: %s", resp.StatusCode, string(body))
	}

	var access AcornToolsAccess
	if err := json.Unmarshal(body, &access); err != nil {
		return nil, fmt.Errorf("failed to parse acorn.tools response: %w", err)
	}
	return &access, nil
}

// GetLoginUrl fetches the Azure AD B2C login URL from ChainFS API
func GetLoginUrl(baseUrl string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/NansenFile/LoginURL", baseUrl)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch login URL from ChainFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ChainFS API returned status %d when fetching login URL", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read login URL response: %w", err)
	}

	return string(body), nil
}

// GetLogoutUrl fetches the logout URL from ChainFS API
func GetLogoutUrl(baseUrl string) (string, error) {
	endpoint := fmt.Sprintf("%s/api/NansenFile/LogoutURL", baseUrl)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch logout URL from ChainFS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ChainFS API returned status %d when fetching logout URL", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read logout URL response: %w", err)
	}

	return string(body), nil
}
