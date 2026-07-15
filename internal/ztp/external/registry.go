package external

import "fmt"

// Registry bundles every external-system client for a ZTPSetting. The
// orchestrator obtains registrars from here and calls them in order.
type Registry struct {
	Spectrum *SpectrumClient
	MSAG     *MSAGClient
	BMC      *BMCClient
	NewBMC   *BMCClient
	LMF      []*LMFClient
	GMLC     *GMLCClient
}

// NewRegistry builds all clients from an ExternalConfig. A nil transport is
// replaced with NotImplementedTransport (Phase 2a default).
func NewRegistry(cfg *ExternalConfig, t Transport) *Registry {
	if t == nil {
		t = NotImplementedTransport{}
	}
	lmfs := make([]*LMFClient, 0, len(cfg.LMF))
	for i, l := range cfg.LMF {
		name := "lmf"
		if i > 0 {
			name = fmt.Sprintf("lmf-%d", i+1)
		}
		lmfs = append(lmfs, NewLMFClient(name, l, t))
	}
	return &Registry{
		Spectrum: NewSpectrumClient(cfg.Spectrum, t),
		MSAG:     NewMSAGClient(cfg.MSAG, t),
		BMC:      NewBMCClientOld(cfg.BMC, t),
		NewBMC:   NewBMCClientNew(cfg.NewBMC, t),
		LMF:      lmfs,
		GMLC:     NewGMLCClient(cfg.GMLC, t),
	}
}

// Registrars returns the registerable systems (those that implement Add/Delete),
// excluding the Spectrum lookup. Used by the orchestrator to iterate pushes.
func (r *Registry) Registrars() []Registrar {
	out := []Registrar{r.MSAG, r.BMC, r.NewBMC, r.GMLC}
	for _, l := range r.LMF {
		out = append(out, l)
	}
	return out
}
