package parameter

// ---------------------------------------------------------------------------
// ParameterSet
// ---------------------------------------------------------------------------

// ListParameterSets returns all parameter sets for the given license.
func (s *Service) ListParameterSets(licenseId int) ([]ParameterSet, error) {
	return s.repo.FindParameterSets(licenseId)
}

// CreateParameterSet persists a new parameter set.
func (s *Service) CreateParameterSet(ps *ParameterSet) error {
	return s.repo.Create(ps)
}

// UpdateParameterSet persists changes to an existing parameter set.
func (s *Service) UpdateParameterSet(ps *ParameterSet) error {
	return s.repo.Save(ps)
}

// DeleteParameterSet removes a parameter set by ID.
func (s *Service) DeleteParameterSet(id string) error {
	return s.repo.DeleteByID(id)
}
