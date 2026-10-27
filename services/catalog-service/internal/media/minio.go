package media

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	s3Region         = "us-east-1"
	s3Service        = "s3"
	s3UnsignedBody   = "UNSIGNED-PAYLOAD"
	defaultHTTPTimeout = 10 * time.Second
)

// Config configures the S3-compatible MinIO media client.
type Config struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	Bucket        string
	PublicBaseURL string
	UseSSL        bool
	HTTPClient    *http.Client
}

// SignedUpload holds the presigned PUT URL and resulting public URL.
type SignedUpload struct {
	UploadURL        string
	PublicURL        string
	ExpiresInSeconds int32
}

// MinIOClient signs browser PUT URLs and manages bucket/object lifecycle via S3 SigV4.
type MinIOClient struct {
	endpoint      string
	scheme        string
	accessKey     string
	secretKey     string
	bucket        string
	publicBaseURL string
	httpClient    *http.Client
	now           func() time.Time
	bucketMu      sync.Mutex
	bucketReady   atomic.Bool
}

// NewMinIOClient validates configuration and returns a MinIOClient.
func NewMinIOClient(cfg Config) (*MinIOClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return nil, errors.New("minio endpoint is required")
	}
	accessKey := strings.TrimSpace(cfg.AccessKey)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("minio access key and secret key are required")
	}
	bucket := strings.Trim(strings.TrimSpace(cfg.Bucket), "/")
	if bucket == "" {
		return nil, errors.New("minio bucket is required")
	}
	publicBase := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if publicBase == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		publicBase = fmt.Sprintf("%s://%s/%s", scheme, endpoint, bucket)
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &MinIOClient{
		endpoint:      endpoint,
		scheme:        scheme,
		accessKey:     accessKey,
		secretKey:     secretKey,
		bucket:        bucket,
		publicBaseURL: publicBase,
		httpClient:    hc,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

// Bucket returns the configured bucket name.
func (c *MinIOClient) Bucket() string {
	return c.bucket
}

// EnsureBucket creates the bucket if missing and applies anonymous read-only GetObject policy.
// Once bootstrap succeeds, subsequent calls return nil without network I/O.
func (c *MinIOClient) EnsureBucket(ctx context.Context) error {
	if c.bucketReady.Load() {
		return nil
	}
	c.bucketMu.Lock()
	defer c.bucketMu.Unlock()
	if c.bucketReady.Load() {
		return nil
	}

	bucketURI := "/" + s3EscapeSegment(c.bucket)
	status, err := c.doSignedRequest(ctx, http.MethodHead, bucketURI, "", nil)
	if err != nil {
		return fmt.Errorf("head bucket %q: %w", c.bucket, err)
	}
	if status == http.StatusNotFound {
		createStatus, err := c.doSignedRequest(ctx, http.MethodPut, bucketURI, "", nil)
		if err != nil {
			return fmt.Errorf("create bucket %q: %w", c.bucket, err)
		}
		if createStatus != http.StatusOK && createStatus != http.StatusNoContent && createStatus != http.StatusConflict {
			return fmt.Errorf("create bucket %q returned status %d", c.bucket, createStatus)
		}
	} else if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("head bucket %q returned status %d", c.bucket, status)
	}

	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, c.bucket)
	policyStatus, err := c.doSignedRequest(ctx, http.MethodPut, bucketURI, "policy=", []byte(policy))
	if err != nil {
		return fmt.Errorf("apply bucket policy %q: %w", c.bucket, err)
	}
	if policyStatus != http.StatusOK && policyStatus != http.StatusNoContent {
		return fmt.Errorf("apply bucket policy %q returned status %d", c.bucket, policyStatus)
	}
	c.bucketReady.Store(true)
	return nil
}

// PresignPut generates an AWS SigV4 query-signed PUT URL locking Content-Length, Content-Type, and Host.
func (c *MinIOClient) PresignPut(objectKey, contentType string, sizeBytes int64, ttl time.Duration) (*SignedUpload, error) {
	cleanKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if cleanKey == "" {
		return nil, errors.New("object key is required")
	}
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		return nil, errors.New("content type is required")
	}
	if sizeBytes <= 0 {
		return nil, errors.New("size bytes must be positive")
	}
	expiresSec := int(ttl.Seconds())
	if expiresSec <= 0 {
		expiresSec = 300
	}

	now := c.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s3Region, s3Service)

	escapedKey := encodeS3Path(cleanKey)
	canonicalURI := "/" + s3EscapeSegment(c.bucket) + "/" + escapedKey
	signedHeaders := "content-length;content-type;host"

	queryPairs := map[string]string{
		"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
		"X-Amz-Credential":    c.accessKey + "/" + credentialScope,
		"X-Amz-Date":          amzDate,
		"X-Amz-Expires":       strconv.Itoa(expiresSec),
		"X-Amz-SignedHeaders": signedHeaders,
	}
	canonicalQuery := buildCanonicalQuery(queryPairs)
	canonicalHeaders := "content-length:" + strconv.FormatInt(sizeBytes, 10) + "\n" +
		"content-type:" + ct + "\n" +
		"host:" + c.endpoint + "\n"
	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		s3UnsignedBody,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(deriveSigningKey(c.secretKey, dateStamp, s3Region, s3Service), []byte(stringToSign)))
	publicURL := c.publicBaseURL + "/" + escapedKey
	uploadURL := publicURL + "?" + canonicalQuery + "&X-Amz-Signature=" + signature

	return &SignedUpload{
		UploadURL:        uploadURL,
		PublicURL:        publicURL,
		ExpiresInSeconds: int32(expiresSec),
	}, nil
}

// RemoveObject deletes an object from the bucket idempotently (404 is treated as success).
func (c *MinIOClient) RemoveObject(ctx context.Context, objectKey string) error {
	cleanKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if cleanKey == "" {
		return nil
	}
	uri := "/" + s3EscapeSegment(c.bucket) + "/" + encodeS3Path(cleanKey)
	status, err := c.doSignedRequest(ctx, http.MethodDelete, uri, "", nil)
	if err != nil {
		return fmt.Errorf("delete object %q: %w", cleanKey, err)
	}
	if status == http.StatusOK || status == http.StatusNoContent || status == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("delete object %q returned status %d", cleanKey, status)
}

func (c *MinIOClient) doSignedRequest(ctx context.Context, method, canonicalURI, canonicalQuery string, body []byte) (int, error) {
	now := c.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, s3Region, s3Service)

	payloadHash := sha256Hex(body)
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", c.endpoint, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(deriveSigningKey(c.secretKey, dateStamp, s3Region, s3Service), []byte(stringToSign)))

	targetURL := fmt.Sprintf("%s://%s%s", c.scheme, c.endpoint, canonicalURI)
	if canonicalQuery != "" {
		targetURL += "?" + canonicalQuery
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return 0, err
	}
	req.Host = c.endpoint
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, credentialScope, signedHeaders, signature,
	))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	return resp.StatusCode, nil
}

func buildCanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, s3EscapeSegment(k)+"="+s3EscapeSegment(params[k]))
	}
	return strings.Join(parts, "&")
}

func encodeS3Path(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = s3EscapeSegment(seg)
	}
	return strings.Join(segments, "/")
}

func s3EscapeSegment(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			b.WriteByte(ch)
		} else {
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
