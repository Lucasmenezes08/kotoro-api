package sprint

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrSprintNameEmpty error = errors.New("Sprint name is empty")
	ErrSprintNotFound error = errors.New("Sprint not found")
	ErrSprintAlreadyExists error = errors.New("Only one sprint per day is allowed")
)


type SprintServiceContract interface{
	Create(ctx context.Context, sprint CreateSprintModel)error
}


type SprintService struct {
	repo SprintRepositoryContract
}

func NewSprintService(repo SprintRepositoryContract) *SprintService{
	return &SprintService{
		repo: repo,
	}
}


func (s *SprintService)Create(ctx context.Context, payload CreateSprintModel)error{

	exist, err := s.repo.ExistsByDate(ctx, payload.SprintDate)
	if err != nil{
		return err 
	}

	if exist {
		return ErrSprintAlreadyExists
	}

	if payload.Name != nil {
		if strings.TrimSpace(*payload.Name) == ""{
			return ErrSprintNameEmpty
		}
	}
	
	input := CreateSprintModel{
		Name: payload.Name,
		SprintDate: payload.SprintDate,
		Status: Creating,
	}

	if sprint := s.repo.Create(ctx, input); sprint != nil{
		return sprint
	}

	return nil
}