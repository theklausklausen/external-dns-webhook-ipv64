# IPv64 API Documentation

This document describes the ipv64.net API integration used by the external-dns-webhook-ipv64.

## Authentication

The webhook uses **Bearer Token Authentication** as specified in the ipv64 API documentation:

```
Authorization: Bearer {your-api-key}
```

## API Endpoints

### Base URL
- Production: `https://ipv64.net/api.php`
- Alternative: `https://ipv64.net/api`

### Get Account Information

**Method:** `GET`  
**Endpoint:** `?get_account_info`

Retrieves information about your ipv64 account including:
- Email address
- Account class
- API limit
- Current API call count

**Response:**
```json
{
  "response": {
    "email": "user@example.com",
    "account_class": "premium",
    "api_limit": 1000,
    "api_calls": 42
  },
  "status": "success",
  "api-call": "get_account_info"
}
```

### Get Domains and Records

**Method:** `GET`  
**Endpoint:** `?get_domains`

Retrieves all domains and their DNS records.

**Response:**
```json
{
  "response": {
    "example.com": {
      "domain": "example.com",
      "records": [
        {
          "id": 12345,
          "domain": "example.com",
          "praefix": "@",
          "type": "A",
          "content": "1.2.3.4"
        },
        {
          "id": 12346,
          "domain": "example.com",
          "praefix": "www",
          "type": "A",
          "content": "1.2.3.4"
        }
      ]
    }
  },
  "status": "success",
  "api-call": "get_domains"
}
```

### Add Domain

**Method:** `POST`  
**Content-Type:** `application/x-www-form-urlencoded`  
**Parameters:**
- `add_domain`: Domain name (e.g., `example.com`)

Creates a new domain. Automatically creates A or AAAA record.

**Response:**
```json
{
  "response": "Domain created successfully",
  "status": "success"
}
```

### Delete Domain

**Method:** `DELETE`  
**Content-Type:** `application/x-www-form-urlencoded`  
**Parameters:**
- `del_domain`: Domain name (e.g., `example.com`)

Deletes a domain immediately with all known DNS records.

**Response:**
```json
{
  "response": "Domain deleted successfully",
  "status": "success"
}
```

### Add DNS Record

**Method:** `POST`  
**Content-Type:** `application/x-www-form-urlencoded`  
**Parameters:**
- `add_record`: Domain name (e.g., `example.com`)
- `praefix`: Domain prefix/subdomain (e.g., `www`, `@` for apex)
- `type`: Record type (A, AAAA, TXT, CNAME, MX, NS, SRV)
- `content`: Record content/value

Creates a new DNS record in the specified domain.

**Response:**
```json
{
  "response": "Record created successfully",
  "status": "success"
}
```

### Delete DNS Record

**Method:** `DELETE`  
**Content-Type:** `application/x-www-form-urlencoded`

**Option 1 - By Record Details:**
- `del_record`: Domain name (e.g., `example.com`)
- `praefix`: Domain prefix/subdomain
- `type`: Record type
- `content`: Record content

**Option 2 - By Record ID:**
- `del_record`: DNS Record ID (integer)

Deletes the DNS record immediately from the domain.

**Response:**
```json
{
  "response": "Record deleted successfully",
  "status": "success"
}
```

## Rate Limits

### Account Limits
- **Default:** 64 API calls per 24 hours
- **Varies:** Depends on account class

### Call Limits
- **Maximum:** 5 API requests within 10 seconds

## HTTP Status Codes

- `200 OK` - Successful request
- `201 Created` - Resource created successfully
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Invalid or missing authentication
- `403 Forbidden` - Insufficient permissions
- `429 Too Many Requests` - Rate limit exceeded

## Supported Record Types

The webhook supports the following DNS record types:

- **A** - IPv4 address
- **AAAA** - IPv6 address
- **CNAME** - Canonical name
- **TXT** - Text record
- **MX** - Mail exchange
- **NS** - Name server
- **SRV** - Service record

## Error Handling

The webhook handles errors gracefully:

1. **Authentication Errors**: Logged and webhook fails to start
2. **Rate Limiting**: Errors logged, operations may be retried
3. **Not Found**: Ignored for delete operations
4. **Already Exists**: Ignored for create operations

## Best Practices

1. **Minimize API Calls**: The webhook caches domain information when possible
2. **Batch Operations**: Changes are applied in batches when feasible
3. **Retry Logic**: Failed operations may be retried by external-dns
4. **Monitoring**: Check account API usage regularly

## Example: Creating a Record

```bash
curl -X POST https://ipv64.net/api.php \
  -H "Authorization: Bearer your-api-key" \
  -d "add_record=example.com" \
  -d "praefix=www" \
  -d "type=A" \
  -d "content=1.2.3.4"
```

## Example: Deleting a Record

```bash
curl -X DELETE https://ipv64.net/api.php \
  -H "Authorization: Bearer your-api-key" \
  -d "del_record=example.com" \
  -d "praefix=www" \
  -d "type=A" \
  -d "content=1.2.3.4"
```

## References

- [ipv64.net Official Documentation](https://ipv64.net)
- [external-dns Documentation](https://github.com/kubernetes-sigs/external-dns)
