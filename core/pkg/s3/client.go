// Package s3 wraps aws-sdk-go-v2's S3 client behind a small,
// options-configured object-upload surface. It works with AWS S3 (leave
// Endpoint empty, set Region) and S3-compatible stores such as Cloudflare R2
// (set Endpoint to the account endpoint, Region "auto").
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultTimeout = 30 * time.Second
	defaultRegion  = "us-east-1"
)

// Config holds the S3 credentials, target bucket and (optional) custom
// endpoint. An empty bucket or credential leaves the client disabled.
type Config struct {
	// Endpoint is the S3 endpoint URL. Empty uses AWS's default resolution;
	// set it for S3-compatible stores (e.g. an R2 account endpoint).
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

// Client uploads objects to an S3 bucket.
type Client struct {
	s3      *awss3.Client
	bucket  string
	timeout time.Duration
}

// New builds a Client. When the bucket or either credential is empty the client
// is returned disabled (Enabled reports false, Upload errors).
func New(cfg Config, opts ...Option) *Client {
	c := &Client{bucket: cfg.Bucket, timeout: defaultTimeout}
	for _, opt := range opts {
		opt(c)
	}
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return c
	}

	region := cfg.Region
	if region == "" {
		region = defaultRegion
	}

	o := awss3.Options{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
	}
	// A custom endpoint (R2, MinIO, ...) needs path-style addressing; AWS uses
	// virtual-hosted style via its default endpoint resolution.
	if cfg.Endpoint != "" {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	}
	c.s3 = awss3.New(o)

	return c
}

// Enabled reports whether the client is configured to talk to S3.
func (c *Client) Enabled() bool { return c.s3 != nil }

// Upload writes body to the bucket under key with the given content type.
func (c *Client) Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	if c.s3 == nil {
		return errors.New("s3: storage not configured")
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("s3 upload read: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err = c.s3.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}

	return nil
}
