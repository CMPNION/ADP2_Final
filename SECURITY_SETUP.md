# Security Setup Guide

## Secrets Management

This project uses environment variables for all sensitive credentials. **Never commit secrets to the repository.**

### Environment Variables Setup

1. **Create a `.env` file** in the project root (already in `.gitignore`):

```bash
# API Gateway & Auth
JWT_SECRET=your-secure-jwt-secret-here

# Mailgun Configuration
MAILGUN_API_KEY=your-mailgun-api-key-here
MAILGUN_DOMAIN=your-mailgun-sandbox-domain
MAILGUN_BASE_URL=https://api.mailgun.net
MAILGUN_FROM_EMAIL=noreply@yourdomain.com

# Database URLs
INVENTORY_DB_URL=postgres://user:password@postgres:5432/inventory_db
CATALOG_DB_URL=postgres://user:password@postgres:5432/catalog_db
ORDER_DB_URL=postgres://user:password@postgres:5432/order_db

# Infrastructure
REDIS_ADDR=redis:6379
NATS_URL=nats://nats:4222
```

2. **Use the provided `example.env`** as a template:

```bash
cp example.env .env
# Edit .env with your actual credentials
```

### Getting Mailgun Credentials

1. Sign up at [mailgun.com](https://mailgun.com)
2. Create a sandbox domain (for testing)
3. Get your API Key from the dashboard
4. Add to `.env`:

```
MAILGUN_API_KEY=your-key-here
MAILGUN_DOMAIN=sandboxXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX.mailgun.org
MAILGUN_FROM_EMAIL=your-verified-email@example.com
```

### JWT Secret Generation

Generate a secure JWT secret:

```bash
# Using openssl
openssl rand -base64 32

# Or use Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"
```

## Code-Level Security

### Best Practices

1. **Environment Variables Only**
   - All credentials read from environment variables
   - No hardcoded secrets in source code
   - Fallback to warnings if env vars missing (don't use defaults)

2. **Password Hashing**
   - bcrypt with cost factor 12 (Go: `bcrypt.DefaultCost`)
   - Passwords never logged or exposed

3. **JWT Tokens**
   - Algorithm: HS256 (HMAC-SHA256)
   - Expiration: 24 hours from issue
   - Signature validation required on all protected endpoints

4. **Transport Security**
   - gRPC services use standard TCP (can be TLS enabled)
   - API Gateway supports CORS for development
   - Authorization header required: `Bearer <token>`

### Audit & Verification

#### Check for Hardcoded Secrets

```bash
# Search for common secret patterns
grep -r "MAILGUN_API_KEY=\|password=" . --include="*.go" --include="*.env"

# Should return no results (except .env which is ignored)
```

#### Review .gitignore

```bash
cat .gitignore
# Should include: .env
```

#### Verify No Secrets in Git History

```bash
# After filtering, check for API key patterns
git log -p -S "mailgun.org" | head -20
git log -p -S "bcrypt" | head -20

# Both are OK - bcrypt is code, mailgun.org references are acceptable
```

## Production Deployment

### Environment Variable Management

**Option 1: Docker Secrets** (Docker Swarm)
```yaml
services:
  api-gateway:
    environment:
      - JWT_SECRET_FILE=/run/secrets/jwt_secret
      - MAILGUN_API_KEY_FILE=/run/secrets/mailgun_key
```

**Option 2: Kubernetes Secrets**
```bash
kubectl create secret generic app-secrets \
  --from-literal=JWT_SECRET=your-secret \
  --from-literal=MAILGUN_API_KEY=your-key
```

**Option 3: AWS Secrets Manager**
```go
// Use AWS SDK to fetch secrets at runtime
secrets := awssecretsmanager.GetSecrets(ctx)
```

**Option 4: HashiCorp Vault**
```go
// Use Vault client to fetch secrets
client := vault.NewClient(config)
secret, _ := client.Logical().Read("secret/data/app")
```

### Database Security

1. **Use Strong Credentials**
   - Generate secure passwords: `openssl rand -base64 32`
   - Store in secrets manager, not code

2. **TLS Connections**
   ```go
   // PostgreSQL connection with TLS
   connStr := "postgres://user:pass@host:5432/db?sslmode=require"
   ```

3. **Least Privilege**
   - Database users should only have required permissions
   - Separate read-only users for reporting

### API Security

1. **Rate Limiting** (Implemented)
   - Limited by API Gateway
   - Per-user or per-IP based

2. **CORS** (Implemented)
   - Configured in API Gateway
   - Restrict to specific origins in production

3. **Input Validation** (In handlers)
   - Validate all incoming requests
   - Sanitize before database operations

4. **HTTPS/TLS** (Ready for production)
   ```yaml
   # In docker-compose.yml or k8s
   - "443:443"  # TLS port
   environment:
     - TLS_CERT=/secrets/cert.pem
     - TLS_KEY=/secrets/key.pem
   ```

## Incident Response

### If Secrets are Exposed

1. **Immediately Rotate Credentials**
   ```bash
   # Generate new API key in Mailgun dashboard
   # Generate new JWT secret
   # Update database passwords
   ```

2. **Audit Logs**
   - Check API Gateway logs for suspicious activity
   - Review database query logs

3. **Notify Users**
   - If user passwords compromised, require password reset
   - Send security notification emails

4. **Git History Cleanup**
   ```bash
   # Use git filter-branch or BFG to remove secrets
   FILTER_BRANCH_SQUELCH_WARNING=1 git filter-branch -f --tree-filter \
     'find . -name "*.go" -exec sed -i "s/EXPOSED_SECRET/REDACTED/g" {} +'
   git push -f origin main
   ```

## Compliance Checklist

- [ ] All sensitive credentials in `.env` (not committed)
- [ ] `.gitignore` includes `.env`
- [ ] No hardcoded secrets in source code
- [ ] JWT secret changed from defaults
- [ ] Database passwords generated securely
- [ ] Mailgun API key secured and verified
- [ ] All API endpoints validate Authorization header
- [ ] Passwords hashed with bcrypt (cost >= 10)
- [ ] HTTPS/TLS ready for production
- [ ] Rate limiting configured
- [ ] Input validation implemented
- [ ] Secrets management solution chosen for production

## References

- [OWASP: Secrets Management](https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html)
- [Git Secret Scanning](https://docs.github.com/en/code-security/secret-scanning)
- [bcrypt Documentation](https://godoc.org/golang.org/x/crypto/bcrypt)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8949)
