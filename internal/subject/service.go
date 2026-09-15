package subject

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrSubjectInvalidColor error = errors.New("Error to verify valid color")
var ErrSubjectRequiredColor error = errors.New("Subject color is required")
var ErrSubjectRequiredName error = errors.New("Subject name is required")

type SubjectContract interface {
	Create(ctx context.Context, subject SubjectCreateInput) error
	GetAll(ctx context.Context) ([]Subject, error)
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

func ValidColor(color Color) error {
	switch color {
	case ColorBlack, ColorBlue, ColorGray, ColorGreen, ColorOrange, ColorPink, ColorPurple, ColorRed, ColorWhite, ColorYellow:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrSubjectInvalidColor, color)
	}
}
