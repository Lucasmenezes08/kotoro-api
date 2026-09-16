package subject

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var ErrSubjectInvalidColor error = errors.New("Error to verify valid color")
var ErrSubjectRequiredColor error = errors.New("Subject color is required")
var ErrSubjectRequiredName error = errors.New("Subject name is required")
var ErrSubjectNotFound error = errors.New("Subject not found")
var ErrSubjectUpdateEmpty error = errors.New("Update fields must have one value at least")

type SubjectContract interface {
	Create(ctx context.Context, subject SubjectCreateInput) error
	GetAll(ctx context.Context) ([]Subject, error)
	Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput)error
	DeleteById(ctx context.Context, id uuid.UUID)error
}

type SubjectService struct {
	repository SubjectContract
}

func NewSubjectService(repository SubjectContract) *SubjectService {
	return &SubjectService{
		repository: repository,
	}
}

func (s *SubjectService) GetAll(ctx context.Context) ([]Subject, error) {
	sub, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("error to get all subjects, error, %w", err)
	}
	return sub, nil
}

func (s *SubjectService) Create(ctx context.Context, subject SubjectCreateInput) error {
	if strings.TrimSpace(subject.Name) == "" {
		return ErrSubjectRequiredName
	}

	if strings.TrimSpace(string(subject.Color)) == "" {
		return ErrSubjectRequiredColor
	}

	if err := ValidColor(subject.Color); err != nil {
		return err
	}

	if err := s.repository.Create(ctx, subject); err != nil {
		return fmt.Errorf("error to create subject, error, %w\n", err)
	}
	return nil
}

func (s *SubjectService)Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput)error{
	if input.Name == nil && input.Color == nil {
		return ErrSubjectUpdateEmpty
	}

	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == ""{
			return ErrSubjectRequiredName
		}
	}

	if input.Color != nil {
		if strings.TrimSpace(string(*input.Color)) == ""{
			return ErrSubjectRequiredColor
		}
		if ValidColor(*input.Color) != nil {
			return ErrSubjectInvalidColor
		}
	}

	if err := s.repository.Update(ctx, id, input); err != nil {
		return fmt.Errorf("%w",err)
	}
	return nil
}


func (s *SubjectService)DeleteById(ctx context.Context, id uuid.UUID)error{
	if err := s.repository.DeleteById(ctx, id); err != nil {
		return err
	} 

	return nil
}



func ValidColor(color Color) error {
	switch color {
	case ColorBlack, ColorBlue, ColorGray, ColorGreen, ColorOrange, ColorPink, ColorPurple, ColorRed, ColorWhite, ColorYellow:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrSubjectInvalidColor, color)
	}
}
