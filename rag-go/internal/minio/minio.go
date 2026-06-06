package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	miniosdk "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"llmentor/rag-go/internal/config"
)

type Client struct {
	client     *miniosdk.Client
	bucket     string
	endpoint   string
	publicRead bool
}

func New(cfg config.MinIOConfig) (*Client, error) {
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	endpoint := parsed.Host
	secure := parsed.Scheme == "https"
	if endpoint == "" {
		endpoint = cfg.Endpoint
	}
	client, err := miniosdk.New(endpoint, &miniosdk.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &Client{client: client, bucket: cfg.BucketName, endpoint: strings.TrimRight(cfg.Endpoint, "/"), publicRead: cfg.PublicRead}, nil
}

func (c *Client) UploadFromURL(ctx context.Context, fileURL string) (string, error) {
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("download %s: %s", fileURL, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fileName := path.Base(resp.Request.URL.Path)
	if fileName == "." || !strings.Contains(fileName, ".") {
		fileName = fmt.Sprintf("file_%d", time.Now().UnixMilli())
	}
	objectName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), fileName)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(fileName))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := c.UploadBytes(ctx, objectName, data, contentType); err != nil {
		return "", err
	}
	return objectName, nil
}

func (c *Client) UploadBytes(ctx context.Context, objectName string, data []byte, contentType string) error {
	if err := c.ensureBucket(ctx); err != nil {
		return err
	}
	_, err := c.client.PutObject(ctx, c.bucket, objectName, bytes.NewReader(data), int64(len(data)), miniosdk.PutObjectOptions{ContentType: contentType})
	return err
}

func (c *Client) PresignedURL(ctx context.Context, objectName string) (string, error) {
	u, err := c.client.PresignedGetObject(ctx, c.bucket, objectName, 7*24*time.Hour, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.client.MakeBucket(ctx, c.bucket, miniosdk.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	if c.publicRead {
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, c.bucket)
		_ = c.client.SetBucketPolicy(ctx, c.bucket, policy)
	}
	return nil
}
