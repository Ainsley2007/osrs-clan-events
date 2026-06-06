package proofstorage

import (
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
	return strings.TrimSpace(c.AccountID) != "" &&
		strings.TrimSpace(c.AccessKeyID) != "" &&
		strings.TrimSpace(c.SecretAccessKey) != "" &&
		strings.TrimSpace(c.Bucket) != "" &&
		strings.TrimSpace(c.PublicBaseURL) != ""
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

func (s *R2Store) PersistFromURL(ctx context.Context, submissionID int64, sourceURL string) (string, error) {
	body, contentType, err := s.download(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailableProof, err)
	}
	defer body.Close()

	ext := extensionFromResponse(sourceURL, contentType)
	key := ObjectKey(submissionID, ext)

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("%w: upload failed: %v", ErrUnavailableProof, err)
	}

	return PublicURL(s.publicBaseURL, key), nil
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

func (s *R2Store) download(ctx context.Context, sourceURL string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download proof: %w", err)
	}
	if !isSuccessStatus(resp.StatusCode) {
		resp.Body.Close()
		return nil, "", fmt.Errorf("download proof: HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.Body, contentType, nil
}
