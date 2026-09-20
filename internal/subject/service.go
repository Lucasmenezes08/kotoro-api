package subject

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var ErrSubjectInvalidColor error = errors.New("Error to verify valid color")
var ErrSubjectRequiredColor error = errors.New("Subject color is required")
var ErrSubjectRequiredName error = errors.New("Subject name is required")
var ErrSubjectNotFound error = errors.New("Subject not found")
var ErrSubjectUpdateEmpty error = errors.New("Update fields must have one value at least")
var ErrSubjectCreateBatchEmpty error = errors.New("Subjects fields must have one value at least")
var ErrSubjectDuplicated error = errors.New("This subject already exists")

const maxWorkerPools int = 4

type SubjectBatchJob struct {
	Index   int
	Subject SubjectCreateInput
}

type SubjectBatchResult struct {
	Index   int
	Name    string
	Created bool
	Err     error
}

type SubjectContract interface {
	Create(ctx context.Context, subject SubjectCreateInput) error
	GetAll(ctx context.Context) ([]Subject, error)
	Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	CreateBatch(ctx context.Context, subjects []SubjectCreateInput) ([]SubjectBatchResult, error)
}

type SubjectService struct {
	repository SubjectRepositoryContract
}

func NewSubjectService(repository SubjectRepositoryContract) *SubjectService {
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

	duplicate, _ := s.repository.GetByName(ctx, subject.Name)

	if duplicate != nil {
		return ErrSubjectDuplicated
	}

	if err := ValidColor(subject.Color); err != nil {
		return err
	}

	if err := s.repository.Create(ctx, subject); err != nil {
		return fmt.Errorf("error to create subject, error, %w\n", err)
	}
	return nil
}

func (s *SubjectService) Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error {
		
	if input.Name == nil && input.Color == nil {
		return ErrSubjectUpdateEmpty
	}

	if input.Name != nil {
		if strings.TrimSpace(*input.Name) == "" {
			return ErrSubjectRequiredName
		}

		duplicate, _ := s.repository.GetByName(ctx, *input.Name)
		if duplicate != nil && duplicate.Id != id{
			return ErrSubjectDuplicated
	}
	}

	if input.Color != nil {
		if strings.TrimSpace(string(*input.Color)) == "" {
			return ErrSubjectRequiredColor
		}
		if ValidColor(*input.Color) != nil {
			return ErrSubjectInvalidColor
		}
	}

	if err := s.repository.Update(ctx, id, input); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (s *SubjectService) DeleteById(ctx context.Context, id uuid.UUID) error {
	if err := s.repository.DeleteById(ctx, id); err != nil {
		return err
	}

	return nil
}

func (s *SubjectService) CreateBatch(ctx context.Context, subjects []SubjectCreateInput) ([]SubjectBatchResult, error) {
	if len(subjects) == 0 {
		return []SubjectBatchResult{}, nil
	}

	var wg sync.WaitGroup
	jobs := make(chan SubjectBatchJob)
	results := make(chan SubjectBatchResult)

	workerCount := maxWorkerPools

	if len(subjects) < workerCount {
		workerCount = len(subjects)
	}

	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go s.workerCreateSubjects(ctx, jobs, results, &wg)
	}

	go func() {
		defer close(jobs)

		for index, input := range subjects {
			job := SubjectBatchJob{
				Index:   index,
				Subject: input,
			}

			select {
			case jobs <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	batchResults := make(
		[]SubjectBatchResult,
		len(subjects),
	)

	for result := range results {
		batchResults[result.Index] = result
	}

	if err := ctx.Err(); err != nil {
		return batchResults, err
	}

	return batchResults, nil
}

func (s *SubjectService) workerCreateSubjects(ctx context.Context, subjectjobs <-chan SubjectBatchJob, subjectJobResult chan<- SubjectBatchResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-subjectjobs:
			if !ok {
				return
			}

			err := s.Create(ctx, job.Subject)

			result := SubjectBatchResult{
				Index:   job.Index,
				Name:    job.Subject.Name,
				Created: err == nil,
				Err:     err,
			}

			select {
			case subjectJobResult <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func ValidColor(color Color) error {
	switch color {
	case ColorBlack, ColorBlue, ColorGray, ColorGreen, ColorOrange, ColorPink, ColorPurple, ColorRed, ColorWhite, ColorYellow:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrSubjectInvalidColor, color)
	}
}
