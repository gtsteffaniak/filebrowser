# OnlyOffice Integration Troubleshooting Guide

## Overview

The OnlyOffice integration allows users to edit documents directly in the browser using OnlyOffice Document Server. This guide explains how the system works and how to troubleshoot common issues.

## System Architecture

### Components
- **Frontend (Vue)**: Initiates OnlyOffice editor with configuration
- **FileBrowser API (Go)**: Manages authentication, file access, and configuration
- **OnlyOffice Server**: Document editing service (external)
- **File System**: Storage backend

### Integration Flow

1. **Config Request**: Frontend requests OnlyOffice configuration with clean parameters (`source`, `path`, optional `hash`)
2. **URL Building**: Backend builds download and callback URLs internally
3. **Document Loading**: OnlyOffice server downloads file using provided URL
4. **Editing**: User edits document in browser via OnlyOffice JavaScript API
5. **Saving**: OnlyOffice server notifies backend via callback when document changes

## API Endpoints

### GET `/api/onlyoffice/config`

**Purpose**: Get OnlyOffice client configuration for a file

**Parameters**:
- `source` (required): File source identifier
- `path` (required): File path within the source
- `hash` (optional): Share hash for public shares

**Example**:
```
GET /api/onlyoffice/config?source=downloads&path=/document.docx
GET /api/onlyoffice/config?source=shared&path=/doc.docx&hash=abc123
```

**Response**: OnlyOffice client configuration JSON

### POST `/api/onlyoffice/callback`

**Purpose**: Receive notifications from OnlyOffice server about document changes

**Parameters** (query string):
- `source` (required): File source identifier
- `path` (required): File path within the source
- `hash` (optional): Share hash for public shares
- `auth` (required): Authentication token

**Body**: OnlyOffice callback payload with status and document URL

## Configuration

### Required Settings

```yaml
integrations:
  onlyoffice:
    url: "http://onlyoffice-server:8000"  # OnlyOffice Document Server URL
    secret: "your-secret-key"             # Optional JWT secret for security
```

### Server Settings

```yaml
server:
  baseURL: "/filebrowser"                 # Base URL for external access
  internalUrl: "http://filebrowser:8080"  # Internal URL for OnlyOffice server communication
```

**Important**: `internalUrl` should be accessible from the OnlyOffice server. This is typically a Docker network internal address.

## Troubleshooting

### Common Issues

#### 1. OnlyOffice Integration Not Configured
**Error**: `only-office integration must be configured in settings`

**Solution**:
- Ensure `integrations.onlyoffice.url` is set in configuration
- Verify OnlyOffice server is running and accessible

#### 2. Missing Parameters
**Error**: `missing required parameters: source and path are required`

**Cause**: Frontend not sending required parameters

**Debug Steps**:
1. Check browser dev tools network tab
2. Verify URL parameters: `source` and `path` must be present
3. For shares, `hash` parameter should also be included

#### 3. Source Not Available
**Error**: `source X is not available for user Y`

**Cause**: User doesn't have access to the specified source

**Debug Steps**:
1. Check user permissions and scopes
2. Verify source name is correct
3. Check if source is properly configured

#### 4. File Not Found
**Error**: File info retrieval fails

**Debug Steps**:
1. Verify file exists at the specified path
2. Check file permissions
3. Ensure source index is available

#### 5. Document ID Generation Failed
**Error**: `failed to generate document ID`

**Cause**: OnlyOffice document key cache miss or index issues

**Debug Steps**:
1. Check if source index is healthy
2. Verify file path resolution
3. Look for cache-related issues in logs

#### 6. OnlyOffice Server Can't Download File
**Symptoms**: 
- OnlyOffice editor shows loading indefinitely
- OnlyOffice server logs show download failures

**Debug Steps**:
1. Check if `internalUrl` is configured correctly
2. Verify OnlyOffice server can reach FileBrowser on the internal URL
3. Check authentication token validity
4. Test download URL manually

#### 7. Document Save Failures
**Error**: Callback handler errors during save

**Debug Steps**:
1. Check user modify permissions
2. Verify file system write permissions
3. Check callback URL accessibility from OnlyOffice server
4. Look for network connectivity issues

### Debugging Tips

#### Enable Debug Logging

Add debug logging to see detailed flow:
```bash
export LOG_LEVEL=debug
```

#### Key Log Messages to Look For

**Successful Config Generation**:
```
OnlyOffice config request: source=downloads, path=/doc.docx, isShare=false
OnlyOffice: built download URL=http://localhost:8080/api/raw?files=downloads%3A%3A%2Fdoc.docx&auth=token123
OnlyOffice: successfully generated config for file=doc.docx
```

**Successful Callback Processing**:
```
OnlyOffice callback: source=downloads, path=/doc.docx, isShare=false, status=2
OnlyOffice: successfully saved updated document for source=downloads, path=/path/to/doc.docx
```

#### Network Testing

Test download URL manually:
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" "http://your-server/api/raw?files=source%3A%3Apath"
```

Test from OnlyOffice server perspective:
```bash
# From OnlyOffice server container/machine
curl "http://filebrowser:8080/api/raw?files=source%3A%3Apath&auth=token"
```

### Common Configuration Issues

#### 1. Docker Network Setup

If using Docker, ensure services can communicate:

```yaml
# docker-compose.yml
version: '3.8'
services:
  filebrowser:
    image: filebrowser/filebrowser
    container_name: filebrowser
    networks:
      - onlyoffice-network
    environment:
      - INTERNAL_URL=http://filebrowser:8080

  onlyoffice:
    image: onlyoffice/documentserver
    container_name: onlyoffice
    networks:
      - onlyoffice-network

networks:
  onlyoffice-network:
    driver: bridge
```

#### 2. Reverse Proxy Configuration

When using a reverse proxy, ensure proper headers:

```nginx
# nginx.conf
location /onlyoffice/ {
    proxy_pass http://onlyoffice:8000/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## Status Codes Reference

### OnlyOffice Callback Status Codes
- `1`: Document being edited
- `2`: Document ready for saving (closed with changes)
- `3`: Document saving error
- `4`: Document closed without changes
- `6`: Document being edited, but force save requested
- `7`: Document error

### HTTP Response Codes
- `200`: Success
- `400`: Bad request (missing parameters, invalid format)
- `403`: Forbidden (no access to source, no modify permissions)
- `404`: Not found (file not found, document ID generation failed)
- `500`: Internal server error (configuration issues, system errors)

## Testing Checklist

- [ ] OnlyOffice server is running and accessible
- [ ] FileBrowser configuration includes OnlyOffice URL
- [ ] Internal URL is configured for Docker/network setups
- [ ] User has access to the file source
- [ ] File exists and is readable
- [ ] OnlyOffice server can reach FileBrowser's internal URL
- [ ] Authentication tokens are valid
- [ ] File format is supported by OnlyOffice
- [ ] User has modify permissions for editing

## Performance Considerations

### File Size Limits
- Large files may timeout during download
- Consider setting appropriate timeout values
- Monitor bandwidth usage for concurrent users

### Caching
- Document IDs are cached to prevent conflicts
- Cache cleanup happens when documents are closed
- Monitor cache memory usage

### Rate Limiting
- Bandwidth throttling is supported for share links
- Configure `MaxBandwidth` in share settings for large files
