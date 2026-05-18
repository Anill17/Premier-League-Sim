package mocks

import (
	"context"

	"github.com/insider/league-api/internal/domain"
)

type TeamRepoMock struct {
	GetByIDFunc func(ctx context.Context, id int) (*domain.Team, error)
	ListFunc    func(ctx context.Context) ([]domain.Team, error)

	GetByIDCalls int
	ListCalls    int
}

var _ domain.TeamRepository = (*TeamRepoMock)(nil)

func (m *TeamRepoMock) GetByID(ctx context.Context, id int) (*domain.Team, error) {
	m.GetByIDCalls++
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *TeamRepoMock) List(ctx context.Context) ([]domain.Team, error) {
	m.ListCalls++
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}
