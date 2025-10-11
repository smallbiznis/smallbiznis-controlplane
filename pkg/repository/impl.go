package repository

import (
	"context"
	"errors"

	"smallbiznis-controlplane/pkg/db/option"

	"gorm.io/gorm"
)

type store[T any] struct {
	db *gorm.DB
}

func ProvideStore[T any](db *gorm.DB) Repository[T] {
	return &store[T]{db: db}
}

func (r *store[T]) WithTrx(tx *gorm.DB) Repository[T] {
	return &store[T]{db: tx}
}

func (r *store[T]) Find(ctx context.Context, query *T, opts ...option.QueryOption) ([]*T, error) {
	var result []*T
	stmt := r.buildQuery(ctx, query, opts...)
	err := stmt.Find(&result).Error
	return result, err
}

func (r *store[T]) FindOne(ctx context.Context, query *T, opts ...option.QueryOption) (*T, error) {
	var result T
	stmt := r.buildQuery(ctx, query, opts...)
	err := stmt.First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &result, err
}

func (r *store[T]) Create(ctx context.Context, resource *T) error {
	return r.db.WithContext(ctx).Create(resource).Error
}

func (r *store[T]) Update(ctx context.Context, resourceID string, resource any) error {
	return r.db.WithContext(ctx).Model(new(T)).Where("id = ?", resourceID).Updates(resource).Error
}

func (r *store[T]) Delete(ctx context.Context, resourceID string) error {
	var dummy T
	return r.db.WithContext(ctx).Where("id = ?", resourceID).Delete(&dummy).Error
}

func (r *store[T]) Count(ctx context.Context, query *T) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(query).Where(query).Count(&count).Error
	return count, err
}

func (r *store[T]) BatchCreate(ctx context.Context, resources []*T) error {
	if len(resources) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(resources).Error
}

func (r *store[T]) BatchUpdate(ctx context.Context, resources []*T) error {
	for _, resource := range resources {
		if err := r.db.WithContext(ctx).Save(resource).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *store[T]) buildQuery(ctx context.Context, filter *T, opts ...option.QueryOption) *gorm.DB {
	db := s.db.WithContext(ctx).Where(filter)

	for _, opt := range opts {
		db = opt.Apply(db)
	}

	return db
}
