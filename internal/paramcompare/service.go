package paramcompare

import (
	"fmt"

	"gorm.io/gorm"
)

// Service defines the business-logic contract for parameter comparison.
type Service interface {
	CompareDeviceWithTemplate(deviceID uint, templateID uint) (*CompareResult, error)
	BatchCompare(deviceIDs []uint, templateID uint) ([]CompareResult, error)
	ListTemplates() ([]TemplateInfo, error)
}

// service is the concrete implementation of Service.
type service struct {
	repo Repository
}

// NewService creates a Service backed by a fresh Repository.
func NewService(db *gorm.DB) Service {
	return &service{repo: NewRepository(db)}
}

// CompareDeviceWithTemplate fetches the device's actual parameters and the
// template's expected parameters, then compares them and returns deviations.
func (s *service) CompareDeviceWithTemplate(deviceID uint, templateID uint) (*CompareResult, error) {
	// 1. Fetch template name
	templateName, err := s.repo.GetTemplateName(templateID)
	if err != nil {
		return nil, fmt.Errorf("template %d not found: %w", templateID, err)
	}

	// 2. Fetch device parameters
	deviceParams, err := s.repo.GetDeviceParameters(deviceID)
	if err != nil {
		return nil, fmt.Errorf("fetch device params: %w", err)
	}

	// 3. Fetch template parameters
	templateParams, err := s.repo.GetTemplateParameters(templateID)
	if err != nil {
		return nil, fmt.Errorf("fetch template params: %w", err)
	}

	// 4. Build maps for O(1) lookup
	deviceMap := make(map[string]string, len(deviceParams))
	for _, dp := range deviceParams {
		deviceMap[dp.ParamName] = dp.ParamValue
	}

	templateMap := make(map[string]string, len(templateParams))
	for _, tp := range templateParams {
		templateMap[tp.ParamPath] = tp.ParamValue
	}

	// 5. Compare
	result := &CompareResult{
		DeviceID:     deviceID,
		TemplateName: templateName,
	}

	var deviations []Deviation
	matchCount := 0
	mismatchCount := 0
	missingInDevice := 0

	// Check each template parameter against the device
	for paramName, expectedValue := range templateMap {
		actualValue, exists := deviceMap[paramName]
		if !exists {
			missingInDevice++
			deviations = append(deviations, Deviation{
				ParameterName: paramName,
				ActualValue:   "",
				ExpectedValue: expectedValue,
				Status:        "missing_in_device",
			})
		} else if actualValue != expectedValue {
			mismatchCount++
			deviations = append(deviations, Deviation{
				ParameterName: paramName,
				ActualValue:   actualValue,
				ExpectedValue: expectedValue,
				Status:        "mismatch",
			})
		} else {
			matchCount++
			deviations = append(deviations, Deviation{
				ParameterName: paramName,
				ActualValue:   actualValue,
				ExpectedValue: expectedValue,
				Status:        "match",
			})
		}
	}

	// Check for parameters in device that are not in template
	missingInTemplate := 0
	for paramName, actualValue := range deviceMap {
		if _, exists := templateMap[paramName]; !exists {
			missingInTemplate++
			deviations = append(deviations, Deviation{
				ParameterName: paramName,
				ActualValue:   actualValue,
				ExpectedValue: "",
				Status:        "missing_in_template",
			})
		}
	}

	result.TotalParams = len(templateMap) + missingInTemplate
	result.MatchCount = matchCount
	result.MismatchCount = mismatchCount
	result.MissingInDevice = missingInDevice
	result.MissingInTemplate = missingInTemplate
	result.Deviations = deviations

	return result, nil
}

// BatchCompare compares multiple devices against the same template.
func (s *service) BatchCompare(deviceIDs []uint, templateID uint) ([]CompareResult, error) {
	var results []CompareResult
	for _, deviceID := range deviceIDs {
		res, err := s.CompareDeviceWithTemplate(deviceID, templateID)
		if err != nil {
			// Return a partial result with error info rather than failing entirely
			results = append(results, CompareResult{
				DeviceID:     deviceID,
				TemplateName: fmt.Sprintf("(error: %v)", err),
			})
			continue
		}
		results = append(results, *res)
	}
	return results, nil
}

// ListTemplates returns available templates for comparison.
func (s *service) ListTemplates() ([]TemplateInfo, error) {
	return s.repo.ListTemplates()
}

// newService creates a Service backed by the given mock Repository (test helper).
func newService(repo Repository) Service {
	return &service{repo: repo}
}
