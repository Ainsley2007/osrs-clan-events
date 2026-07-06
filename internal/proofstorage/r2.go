package proofstorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicBaseURL   string
}

func (c R2Config) Enabled() bool {
	return len(c.MissingEnvVars()) == 0
}

func (c R2Config) MissingEnvVars() []string {
	checks := []struct {
		name  string
		value string
	}{
		{"R2_ACCOUNT_ID", c.AccountID},
		{"R2_ACCESS_KEY_ID", c.AccessKeyID},
		{"R2_SECRET_ACCESS_KEY", c.SecretAccessKey},
		{"R2_BUCKET", c.Bucket},
		{"R2_PUBLIC_BASE_URL", c.PublicBaseURL},
	}
	var missing []string
	for _, check := range checks {
		if strings.TrimSpace(check.value) == "" {
			missing = append(missing, check.name)
		}
	}
	return missing
}

type R2Store struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
	httpClient    *http.Client
}

func NewR2Store(cfg R2Config) (*R2Store, error) {
	if !cfg.Enabled() {
		return nil, fmt.Errorf("incomplete R2 configuration")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		UsePathStyle: true,
	})

	return &R2Store{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: cfg.PublicBaseURL,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

const (
	healthCheckKey = "_healthcheck/r2-check.png"
	maxProofBytes  = 10 << 20 // 10 MiB
)

func (s *R2Store) PersistFromURL(ctx context.Context, submissionID int64, sourceURL string) (string, error) {
	data, contentType, err := s.download(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailableProof, err)
	}

	ext := extensionFromResponse(sourceURL, contentType)
	key := objectKey(submissionID, ext)

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("%w: upload failed: %v", ErrUnavailableProof, err)
	}

	return buildPublicURL(s.publicBaseURL, key), nil
}

// HealthCheck verifies credentials, bucket access, upload, and delete.
func (s *R2Store) HealthCheck(ctx context.Context) (sampleURL string, err error) {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	}); err != nil {
		return "", fmt.Errorf("head bucket: %w", err)
	}

	body := bytes.NewReader(minPNG)
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(healthCheckKey),
		Body:          body,
		ContentLength: aws.Int64(int64(len(minPNG))),
		ContentType:   aws.String("image/png"),
	}); err != nil {
		return "", fmt.Errorf("upload health check object: %w", err)
	}

	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(healthCheckKey),
	}); err != nil {
		return "", fmt.Errorf("delete health check object: %w", err)
	}

	return buildPublicURL(s.publicBaseURL, healthCheckKey), nil
}

var minPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func (s *R2Store) DeleteBySubmissionID(ctx context.Context, submissionID int64) error {
	prefix := fmt.Sprintf("%d.", submissionID)
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var deleteErr error
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list proof objects: %w", err)
		}
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}
			if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    object.Key,
			}); err != nil {
				deleteErr = err
			}
		}
	}
	return deleteErr
}

func (s *R2Store) download(ctx context.Context, sourceURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download proof: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode) {
		return nil, "", fmt.Errorf("download proof: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProofBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read proof body: %w", err)
	}
	if len(data) > maxProofBytes {
		return nil, "", fmt.Errorf("download proof: file too large")
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return data, contentType, nil
}
