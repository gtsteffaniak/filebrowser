# FileBrowser Initialization Scripts

## Overview

This guide provides automated initialization scripts for FileBrowser using the existing admin user and API. This approach requires no special initialization mode and works across Docker, Docker Compose, Kubernetes, and bare metal deployments.

## How It Works

FileBrowser creates a default admin user on first startup using credentials from your configuration:

- Default username: `admin` (configurable via `auth.adminUsername` in config.yaml)
- Default password: `admin` (configurable via `auth.adminPassword` in config.yaml)

The initialization scripts:
1. Wait for FileBrowser to be ready (health check)
2. Authenticate using the admin credentials
3. Receive a JWT token
4. Use the token to make authenticated API calls
5. Perform setup tasks (create users, configure settings, etc.)

## Authentication

FileBrowser's login API accepts credentials via:

**Query Parameter + Header:**
```bash
curl -H "X-Password: ${PASSWORD}" \
  "http://localhost:8080/api/auth/login?username=${USERNAME}"
```

**Response:**
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...  # JWT token (200 OK)
```

**Using the Token:**
```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8080/api/users
```

## Init Script (Linux / Docker Compose)

Create `init-filebrowser.sh`:

```bash
#!/bin/bash
set -e

# Configuration from environment
FILEBROWSER_URL="${FILEBROWSER_URL:-http://localhost:8080}"
ADMIN_USERNAME="${FILEBROWSER_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${FILEBROWSER_ADMIN_PASSWORD:-admin}"
MAX_RETRIES="${MAX_RETRIES:-30}"
RETRY_DELAY="${RETRY_DELAY:-2}"

echo "FileBrowser initialization script started"
echo "Target URL: ${FILEBROWSER_URL}"

# Function to wait for FileBrowser to be ready
wait_for_filebrowser() {
    local retries=0
    echo "Waiting for FileBrowser to be ready..."
    
    while [ $retries -lt $MAX_RETRIES ]; do
        if curl -f -s "${FILEBROWSER_URL}/health" > /dev/null 2>&1; then
            echo "FileBrowser is ready!"
            return 0
        fi
        retries=$((retries + 1))
        echo "Attempt ${retries}/${MAX_RETRIES} - waiting ${RETRY_DELAY}s..."
        sleep $RETRY_DELAY
    done
    
    echo "ERROR: FileBrowser failed to start after ${MAX_RETRIES} attempts"
    return 1
}

# Function to get auth token
get_auth_token() {
    local username=$1
    local password=$2
    
    echo "Logging in as ${username}..."
    
    local response
    response=$(curl -s -w "\n%{http_code}" \
        -H "X-Password: ${password}" \
        "${FILEBROWSER_URL}/api/auth/login?username=${username}")
    
    local http_code=$(echo "$response" | tail -n1)
    local token=$(echo "$response" | head -n-1)
    
    if [ "$http_code" -eq 200 ] && [ -n "$token" ]; then
        echo "Successfully authenticated!"
        echo "$token"
        return 0
    else
        echo "ERROR: Authentication failed (HTTP ${http_code})"
        return 1
    fi
}

# Function to create user
create_user() {
    local token=$1
    local username=$2
    local password=$3
    local is_admin=${4:-false}
    
    echo "Creating user: ${username}..."
    
    local response
    response=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Authorization: Bearer ${token}" \
        -H "Content-Type: application/json" \
        -d "{
            \"username\": \"${username}\",
            \"password\": \"${password}\",
            \"permissions\": {
                \"admin\": ${is_admin},
                \"modify\": true,
                \"share\": false
            }
        }" \
        "${FILEBROWSER_URL}/api/users")
    
    local http_code=$(echo "$response" | tail -n1)
    
    if [ "$http_code" -eq 201 ]; then
        echo "User ${username} created successfully"
        return 0
    elif [ "$http_code" -eq 409 ]; then
        echo "User ${username} already exists"
        return 0
    else
        echo "WARNING: Failed to create user ${username} (HTTP ${http_code})"
        return 1
    fi
}

# Main execution
main() {
    # Wait for FileBrowser to be ready
    if ! wait_for_filebrowser; then
        exit 1
    fi
    
    # Get authentication token
    TOKEN=$(get_auth_token "$ADMIN_USERNAME" "$ADMIN_PASSWORD")
    if [ -z "$TOKEN" ]; then
        echo "ERROR: Failed to get authentication token"
        exit 1
    fi
    
    echo ""
    echo "Running initialization tasks..."
    echo "Token: ${TOKEN:0:20}..."
    echo ""
    
    # Example: Create additional users
    create_user "$TOKEN" "demo" "demo123" false
    create_user "$TOKEN" "viewer" "viewer123" false
    
    # Example: Additional API calls can be added here
    # - Update settings: POST /api/settings
    # - Create shares: POST /api/share
    # - Configure sources: (requires restart)
    
    echo ""
    echo "Initialization complete!"
}

# Run main function
main "$@"
```

Make the script executable:
```bash
chmod +x init-filebrowser.sh
```

## Docker Compose Setup

### Basic Setup

`docker-compose.yml`:
```yaml
version: '3.8'

services:
  filebrowser:
    image: your-filebrowser:latest
    ports:
      - "8080:8080"
    environment:
      - FILEBROWSER_ADMIN_USERNAME=admin
      - FILEBROWSER_ADMIN_PASSWORD=${ADMIN_PASSWORD:-changeme}
    volumes:
      - ./data:/data
      - ./database:/database
      - ./config.yaml:/config.yaml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s

  filebrowser-init:
    image: curlimages/curl:latest
    depends_on:
      filebrowser:
        condition: service_healthy
    environment:
      - FILEBROWSER_URL=http://filebrowser:8080
      - FILEBROWSER_ADMIN_USERNAME=admin
      - FILEBROWSER_ADMIN_PASSWORD=${ADMIN_PASSWORD:-changeme}
    volumes:
      - ./init-filebrowser.sh:/scripts/init-filebrowser.sh:ro
    command: sh /scripts/init-filebrowser.sh
    restart: "no"

volumes:
  data:
  database:
```

### Usage

```bash
# Set admin password via environment
export ADMIN_PASSWORD="my-secure-password"

# Start services (init will run automatically)
docker-compose up -d

# Check init logs
docker-compose logs filebrowser-init

# Follow logs
docker-compose logs -f filebrowser-init
```

### Production Setup with Secrets

Create `.env` file (DO NOT commit to git):
```bash
ADMIN_PASSWORD=your-secure-password-here
```

Update `docker-compose.yml`:
```yaml
version: '3.8'

services:
  filebrowser:
    image: your-filebrowser:latest
    ports:
      - "8080:8080"
    env_file:
      - .env
    environment:
      - FILEBROWSER_ADMIN_USERNAME=admin
      - FILEBROWSER_ADMIN_PASSWORD=${ADMIN_PASSWORD}
    volumes:
      - data:/data
      - database:/database
      - ./config.yaml:/config.yaml
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s
    restart: unless-stopped

  filebrowser-init:
    image: curlimages/curl:latest
    depends_on:
      filebrowser:
        condition: service_healthy
    env_file:
      - .env
    environment:
      - FILEBROWSER_URL=http://filebrowser:8080
      - FILEBROWSER_ADMIN_USERNAME=admin
      - FILEBROWSER_ADMIN_PASSWORD=${ADMIN_PASSWORD}
    volumes:
      - ./init-filebrowser.sh:/scripts/init-filebrowser.sh:ro
    command: sh /scripts/init-filebrowser.sh
    restart: "no"

volumes:
  data:
    driver: local
  database:
    driver: local
```

## Kubernetes Setup

### Option 1: Init Job (Recommended)

Create `filebrowser-init-job.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: filebrowser-secrets
  namespace: default
type: Opaque
stringData:
  admin-username: admin
  admin-password: changeme-in-production

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: filebrowser-init-script
  namespace: default
data:
  init.sh: |
    #!/bin/sh
    set -e
    
    echo "Waiting for FileBrowser to be ready..."
    RETRIES=0
    MAX_RETRIES=30
    
    until curl -f -s "${FILEBROWSER_URL}/health" > /dev/null 2>&1; do
      RETRIES=$((RETRIES + 1))
      if [ $RETRIES -ge $MAX_RETRIES ]; then
        echo "ERROR: FileBrowser not ready after ${MAX_RETRIES} attempts"
        exit 1
      fi
      echo "Waiting... (${RETRIES}/${MAX_RETRIES})"
      sleep 2
    done
    
    echo "FileBrowser is ready!"
    echo "Getting auth token..."
    
    TOKEN=$(curl -s -H "X-Password: ${ADMIN_PASSWORD}" \
      "${FILEBROWSER_URL}/api/auth/login?username=${ADMIN_USERNAME}")
    
    if [ -z "$TOKEN" ]; then
      echo "ERROR: Failed to authenticate"
      exit 1
    fi
    
    echo "Authenticated successfully!"
    echo "Token: ${TOKEN:0:20}..."
    
    # Create demo user
    echo "Creating demo user..."
    HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
      -X POST \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Content-Type: application/json" \
      -d '{"username":"demo","password":"demo123","permissions":{"admin":false,"modify":true,"share":false}}' \
      "${FILEBROWSER_URL}/api/users")
    
    if [ "$HTTP_CODE" = "201" ]; then
      echo "Demo user created successfully"
    elif [ "$HTTP_CODE" = "409" ]; then
      echo "Demo user already exists"
    else
      echo "WARNING: Failed to create demo user (HTTP ${HTTP_CODE})"
    fi
    
    echo "Initialization complete!"

---
apiVersion: batch/v1
kind: Job
metadata:
  name: filebrowser-init
  namespace: default
spec:
  ttlSecondsAfterFinished: 300
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      containers:
      - name: init
        image: curlimages/curl:latest
        command: ["/bin/sh", "/scripts/init.sh"]
        env:
        - name: FILEBROWSER_URL
          value: "http://filebrowser.default.svc.cluster.local"
        - name: ADMIN_USERNAME
          valueFrom:
            secretKeyRef:
              name: filebrowser-secrets
              key: admin-username
        - name: ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: filebrowser-secrets
              key: admin-password
        volumeMounts:
        - name: init-script
          mountPath: /scripts
      volumes:
      - name: init-script
        configMap:
          name: filebrowser-init-script
          defaultMode: 0755
```

### Option 2: Deployment with Init Container

Create `filebrowser-deployment.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: filebrowser-secrets
  namespace: default
type: Opaque
stringData:
  admin-username: admin
  admin-password: changeme-in-production

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: filebrowser-config
  namespace: default
data:
  config.yaml: |
    server:
      port: 8080
      database: /database/database.db
      sources:
        - name: files
          path: /data
    auth:
      adminUsername: admin
      adminPassword: changeme
    userDefaults:
      permissions:
        modify: false
        share: false

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: filebrowser
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: filebrowser
  template:
    metadata:
      labels:
        app: filebrowser
    spec:
      containers:
      - name: filebrowser
        image: your-filebrowser:latest
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: FILEBROWSER_ADMIN_USERNAME
          valueFrom:
            secretKeyRef:
              name: filebrowser-secrets
              key: admin-username
        - name: FILEBROWSER_ADMIN_PASSWORD
          valueFrom:
            secretKeyRef:
              name: filebrowser-secrets
              key: admin-password
        volumeMounts:
        - name: data
          mountPath: /data
        - name: database
          mountPath: /database
        - name: config
          mountPath: /config
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: filebrowser-data
      - name: database
        persistentVolumeClaim:
          claimName: filebrowser-database
      - name: config
        configMap:
          name: filebrowser-config

---
apiVersion: v1
kind: Service
metadata:
  name: filebrowser
  namespace: default
spec:
  selector:
    app: filebrowser
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: filebrowser-data
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: filebrowser-database
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

### Kubernetes Usage

```bash
# Create namespace (optional)
kubectl create namespace filebrowser

# Update the secret with your password
kubectl create secret generic filebrowser-secrets \
  --from-literal=admin-username=admin \
  --from-literal=admin-password=your-secure-password \
  -n filebrowser

# Deploy FileBrowser
kubectl apply -f filebrowser-deployment.yaml -n filebrowser

# Wait for deployment
kubectl wait --for=condition=ready pod -l app=filebrowser -n filebrowser --timeout=60s

# Run init job
kubectl apply -f filebrowser-init-job.yaml -n filebrowser

# Check init job status
kubectl get jobs -n filebrowser
kubectl logs job/filebrowser-init -n filebrowser

# Access FileBrowser (port-forward for testing)
kubectl port-forward svc/filebrowser 8080:80 -n filebrowser
```

## Bare Metal / Systemd Setup

### SystemD Service File

Create `/etc/systemd/system/filebrowser.service`:

```ini
[Unit]
Description=FileBrowser Service
After=network.target

[Service]
Type=simple
User=filebrowser
Group=filebrowser
Environment="FILEBROWSER_ADMIN_USERNAME=admin"
Environment="FILEBROWSER_ADMIN_PASSWORD=changeme"
EnvironmentFile=-/etc/filebrowser/env
WorkingDirectory=/opt/filebrowser
ExecStart=/opt/filebrowser/filebrowser
ExecStartPost=/bin/sleep 5
ExecStartPost=/opt/filebrowser/init-filebrowser.sh
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Create `/etc/filebrowser/env`:
```bash
FILEBROWSER_ADMIN_USERNAME=admin
FILEBROWSER_ADMIN_PASSWORD=your-secure-password
```

### Setup Commands

```bash
# Create user
sudo useradd -r -s /bin/false filebrowser

# Create directories
sudo mkdir -p /opt/filebrowser
sudo mkdir -p /etc/filebrowser
sudo mkdir -p /var/lib/filebrowser

# Copy files
sudo cp filebrowser /opt/filebrowser/
sudo cp init-filebrowser.sh /opt/filebrowser/
sudo cp config.yaml /opt/filebrowser/
sudo chmod +x /opt/filebrowser/filebrowser
sudo chmod +x /opt/filebrowser/init-filebrowser.sh

# Set permissions
sudo chown -R filebrowser:filebrowser /opt/filebrowser
sudo chown -R filebrowser:filebrowser /var/lib/filebrowser
sudo chmod 600 /etc/filebrowser/env

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable filebrowser
sudo systemctl start filebrowser

# Check status
sudo systemctl status filebrowser
sudo journalctl -u filebrowser -f
```

## Common API Operations

### Create User

```bash
curl -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "password123",
    "permissions": {
      "admin": false,
      "modify": true,
      "share": false,
      "api": false
    },
    "scopes": [
      {
        "name": "/data",
        "scope": "/"
      }
    ]
  }' \
  http://localhost:8080/api/users
```

### List Users

```bash
curl -H "Authorization: Bearer ${TOKEN}" \
  http://localhost:8080/api/users
```

### Update Settings

```bash
curl -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "branding": {
      "name": "My FileBrowser"
    }
  }' \
  http://localhost:8080/api/settings
```

### Create Share

```bash
curl -X POST \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/shared-folder",
    "expires": "2025-12-31T23:59:59Z",
    "password": "sharepass123"
  }' \
  http://localhost:8080/api/share
```

## Best Practices

### Security

- **Never hardcode credentials** - Use environment variables or secrets management
- **Rotate admin password** - Change default password immediately after first login
- **Use HTTPS in production** - Configure TLS certificates
- **Limit network exposure** - Use firewalls and network policies
- **Audit API access** - Monitor logs for unauthorized access attempts

### Reliability

- **Wait for health checks** - Always verify service is ready before init
- **Make scripts idempotent** - Handle "already exists" errors gracefully
- **Set timeouts** - Don't wait indefinitely for service startup
- **Use retries** - Network issues can cause transient failures
- **Log all actions** - Helps with debugging and compliance

### Maintainability

- **Version control init scripts** - Track changes over time
- **Document custom operations** - Add comments for complex logic
- **Use functions** - Make scripts modular and reusable
- **Test in staging** - Verify scripts before production deployment
- **Keep scripts simple** - Complex logic belongs in application code

## Advantages of This Approach

### No Code Changes Required
- Uses existing authentication API
- No special init mode needed
- Works with current codebase as-is

### Security
- Credentials passed via environment variables
- No secrets in process list or command history
- Compatible with secrets management systems

### Cloud-Native
- Works with Kubernetes Secrets
- Supports Docker Compose secrets
- Compatible with HashiCorp Vault, AWS Secrets Manager, etc.

### Flexibility
- Works across all deployment types
- Easy to extend with additional setup tasks
- Scripts can be customized per environment

### Familiar Patterns
- Similar to database migration scripts
- Standard init container pattern
- DevOps teams will recognize the approach

## Troubleshooting

### Init Script Fails to Connect

**Problem:** Script cannot reach FileBrowser service

**Solutions:**
```bash
# Check service is running
docker-compose ps
kubectl get pods

# Check health endpoint
curl http://localhost:8080/health

# Verify network connectivity
ping filebrowser
nslookup filebrowser.default.svc.cluster.local

# Check firewall rules
iptables -L
kubectl get networkpolicies
```

### Authentication Fails

**Problem:** Cannot get token from login API

**Solutions:**
```bash
# Verify credentials are correct
echo "Username: ${FILEBROWSER_ADMIN_USERNAME}"
echo "Password: ${FILEBROWSER_ADMIN_PASSWORD}"

# Test login manually
curl -v -H "X-Password: admin" \
  "http://localhost:8080/api/auth/login?username=admin"

# Check logs for authentication errors
docker-compose logs filebrowser
kubectl logs deployment/filebrowser
```

### Init Job Runs Multiple Times

**Problem:** Kubernetes Job retries unnecessarily

**Solutions:**
- Ensure script exits with code 0 on success
- Set `backoffLimit` in Job spec
- Make script idempotent (handle existing resources)
- Add `ttlSecondsAfterFinished` to clean up completed jobs

### Permission Denied Errors

**Problem:** Script cannot execute or access files

**Solutions:**
```bash
# Make script executable
chmod +x init-filebrowser.sh

# Check file ownership
ls -la init-filebrowser.sh

# Verify volume mounts in Docker
docker-compose exec filebrowser-init ls -la /scripts

# Check SecurityContext in Kubernetes
kubectl describe pod <pod-name>
```

## Additional Resources

- [FileBrowser API Documentation](http://localhost:8080/swagger/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Kubernetes Jobs Documentation](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Kubernetes Init Containers](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)

## Contributing

If you improve these init scripts or add support for additional platforms, please consider contributing back to the project.

