package minio

import (
	"context"

	"smallbiznis-controlplane/pkg/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Client = fx.Module("minio.client", fx.Provide(registerClient))

func registerClient(c *config.Config) *minio.Client {
	client, err := minio.New(c.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.Minio.AccessKey, c.Minio.SecretKey, ""),
		Secure: c.Minio.Secure,
	})
	if err != nil {
		zap.L().Fatal("failed to create MinIO client", zap.Error(err))
	}
	exists, errBucketExists := client.BucketExists(context.Background(), c.Minio.BucketName)
	if errBucketExists != nil {
		zap.L().Fatal("failed to check if bucket exists", zap.String("access_key", c.Minio.AccessKey), zap.String("secret_key", c.Minio.SecretKey), zap.Error(errBucketExists))
	}
	zap.L().Info("MinIO client initialized", zap.String("endpoint", c.Minio.Endpoint), zap.Bool("bucketExists", exists))
	return client
}
