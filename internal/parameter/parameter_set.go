package parameter

// ---------------------------------------------------------------------------
// ParameterSet
// ---------------------------------------------------------------------------

// ListParameterSets returns all parameter sets for the given license.
func (s *service) ListParameterSets(licenseId int) ([]ParameterSet, error) {
	return s.repo.FindParameterSets(licenseId)
}

// CreateParameterSet persists a new parameter set.
func (s *service) CreateParameterSet(ps *ParameterSet) error {
	return s.repo.Create(ps)
}

// UpdateParameterSet persists changes to an existing parameter set.
func (s *service) UpdateParameterSet(ps *ParameterSet) error {
	return s.repo.Save(ps)
}

// DeleteParameterSet removes a parameter set by ID.
func (s *service) DeleteParameterSet(id string) error {
	return s.repo.DeleteByID(id)
}
