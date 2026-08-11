package instances

import (
	"context"
	"errors"
	"testing"
)

type queryRepository struct {
	items []Instance
	err   error
}

func (repository queryRepository) ListInstances(context.Context) ([]Instance, error) {
	return repository.items, repository.err
}

func (repository queryRepository) GetInstance(_ context.Context, id string) (Instance, error) {
	if repository.err != nil {
		return Instance{}, repository.err
	}
	for _, instance := range repository.items {
		if instance.ID == id {
			return instance, nil
		}
	}
	return Instance{}, errors.New("not found")
}

func (queryRepository) SaveInstance(context.Context, Instance) error { return nil }
func (queryRepository) DeleteInstance(context.Context, string) error { return nil }
func (queryRepository) IsDirectoryUsed(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestQueryServiceDelegatesToRepository(t *testing.T) {
	repository := queryRepository{items: []Instance{{ID: "instance", Name: "Test"}}}
	service := NewQueryService(repository)

	items, err := service.List(context.Background())
	if err != nil || len(items) != 1 || items[0].ID != "instance" {
		t.Fatalf("List() = %+v, %v", items, err)
	}
	instance, err := service.Get(context.Background(), "instance")
	if err != nil || instance.Name != "Test" {
		t.Fatalf("Get() = %+v, %v", instance, err)
	}
}

func TestQueryServicePropagatesRepositoryErrors(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewQueryService(queryRepository{err: want})

	if _, err := service.List(context.Background()); !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
	if _, err := service.Get(context.Background(), "instance"); !errors.Is(err, want) {
		t.Fatalf("Get() error = %v, want %v", err, want)
	}
}
