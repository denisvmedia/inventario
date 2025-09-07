# Signed URLs for File Access

This document describes the signed URL system implemented in Inventario for secure file access without exposing JWT authentication tokens.

## Overview

The signed URL system replaces the previous JWT token-based file access mechanism to improve security by:

- **Preventing token exposure**: JWT tokens are no longer included in file URLs
- **Time-limited access**: File URLs automatically expire after a configurable duration
- **File-specific access**: Each signed URL is tied to a specific file and user
- **Tamper-proof**: URLs are cryptographically signed and cannot be modified

## Security Benefits

### Before (JWT in URLs)
```
/api/v1/files/123.pdf?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Problems:**
- JWT tokens exposed in URLs (browser history, logs, referrer headers)
- Long-lived tokens (24 hours) with full API access
- Tokens could be reused for any API endpoint
- Risk of token leakage when URLs are shared

### After (Signed URLs)
```
/api/v1/files/download/123.pdf?sig=abc123&exp=1234567890&uid=user-456
```

**Benefits:**
- No authentication tokens in URLs
- Short-lived signatures (default 15 minutes)
- File-specific access only
- Cannot be reused for other files or API endpoints

## Configuration

### Environment Variables

```bash
# File signing key (minimum 32 characters)
INVENTARIO_RUN_FILE_SIGNING_KEY=your-secure-32-byte-file-signing-key-here

# File URL expiration duration (default: 15m)
INVENTARIO_RUN_FILE_URL_EXPIRATION=15m
```

### Command Line Flags

```bash
inventario run \
  --file-signing-key="your-secure-32-byte-file-signing-key-here" \
  --file-url-expiration="15m"
```

### Configuration File

```yaml
server:
  file-signing-key: "your-secure-32-byte-file-signing-key-here"
  file-url-expiration: "15m"
```

### Key Generation

Generate a secure signing key:

```bash
# Generate a 32-byte hex key
openssl rand -hex 32

# Example output:
# a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456
```

## API Usage

### Generate Signed URL

**Request:**
```http
POST /api/v1/files/{fileID}/signed-url
Authorization: Bearer <jwt-token>
```

**Response:**
```json
{
  "signed_url": "/api/v1/files/download/123.pdf?sig=abc123&exp=1234567890&uid=user-456"
}
```

### Download File

**Request:**
```http
GET /api/v1/files/download/123.pdf?sig=abc123&exp=1234567890&uid=user-456
```

**Response:**
- File content with appropriate headers
- Or 401 Unauthorized if signature is invalid/expired

## Frontend Integration

### File Service Updates

The frontend file service has been updated to use signed URLs:

```typescript
// Generate signed URL for download
async getDownloadUrl(file: FileEntity): Promise<string> {
  const response = await api.post(`/api/v1/files/${file.id}/signed-url`)
  return response.data.signed_url
}

// Download file (now async)
async downloadFile(file: FileEntity) {
  const url = await this.getDownloadUrl(file)
  // ... trigger download
}
```

### Component Updates

Components that display file previews now:
1. Generate signed URLs when the component loads
2. Store URLs in reactive variables
3. Show loading states while URLs are being generated

## Security Features

### HMAC Signature Validation

- Uses HMAC-SHA256 with a separate signing key
- Signatures include: HTTP method, path, file ID, user ID, and expiration
- Constant-time comparison prevents timing attacks

### Expiration Handling

- URLs automatically expire after the configured duration
- Default expiration: 15 minutes (configurable)
- Expired URLs return 401 Unauthorized

### User Context Validation

- Each signed URL is tied to a specific user
- User must exist and be active
- User context is validated on each request

### Tamper Protection

- Any modification to the URL invalidates the signature
- File ID, user ID, and expiration are cryptographically protected
- Cross-user access attempts are logged and blocked

## Monitoring and Logging

### Security Events

The system logs the following security events:

```
WARN Invalid signed URL access attempt
  path=/api/v1/files/download/123.pdf
  query=sig=invalid&exp=1234567890&uid=user-456
  error="invalid signature"
  remote_addr=192.168.1.100
  user_agent="Mozilla/5.0..."

WARN Signed URL access attempt by inactive user
  user_id=user-456
  file_id=123
  user_email=user@example.com
  remote_addr=192.168.1.100
```

### Successful Access

```
DEBUG Signed URL file access granted
  user_id=user-456
  user_email=user@example.com
  file_id=123
  expires_at=2023-12-01T15:30:00Z
  path=/api/v1/files/download/123.pdf
  remote_addr=192.168.1.100
```

## Migration from JWT URLs

### Backward Compatibility

**No backward compatibility is provided.** The old JWT-based file URLs will no longer work.

### Migration Steps

1. **Update configuration**: Add file signing key and expiration settings
2. **Deploy backend**: The new signed URL system is automatically active
3. **Update frontend**: Frontend automatically uses the new API
4. **Clear browser cache**: Users may need to refresh to get new URLs

### Breaking Changes

- File URLs now use `/api/v1/files/download/` prefix instead of `/api/v1/files/`
- File downloads require generating signed URLs first
- Old JWT-based URLs will return 404 Not Found

## Troubleshooting

### Common Issues

**"Invalid or expired file URL" errors:**
- Check if the file signing key is consistent across restarts
- Verify the system clock is synchronized
- Ensure the expiration duration is appropriate for your use case

**"User not found" errors:**
- Verify the user still exists and is active
- Check if the user ID in the signed URL is correct

**"Method not allowed" errors:**
- Signed URLs only support GET requests
- Use the regular API endpoints for file management operations

### Configuration Validation

The system validates configuration on startup:
- File signing key must be at least 32 bytes
- File URL expiration must be at least 1 minute
- Missing configuration will generate random keys (not recommended for production)

## Best Practices

### Production Deployment

1. **Use consistent signing keys**: Set the same key across all instances
2. **Secure key storage**: Store keys in environment variables or secure config
3. **Monitor expiration**: Choose appropriate expiration times for your use case
4. **Log monitoring**: Monitor security logs for suspicious activity

### Security Considerations

1. **Key rotation**: Periodically rotate the file signing key
2. **HTTPS only**: Always use HTTPS in production
3. **Short expiration**: Use the shortest practical expiration time
4. **Access logging**: Monitor file access patterns for anomalies

### Performance

1. **Caching**: Frontend can cache signed URLs until they expire
2. **Batch generation**: Generate multiple signed URLs in parallel if needed
3. **CDN compatibility**: Signed URLs work with CDNs (with proper cache headers)
