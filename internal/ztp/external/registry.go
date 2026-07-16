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

// NewRegistry builds all clients from an ExternalConfig using a single shared
// Transport (used by tests with a FuncTransport). Production code should use
// NewRegistryWithTransports, which assigns per-system mTLS transports.
func NewRegistry(cfg *ExternalConfig, t Transport) *Registry {
	if t == nil {
		t = NotImplementedTransport{}
	}
	return NewRegistryWithTransports(cfg, &Transports{Shared: t, LMF: []Transport{t, t, t, t}})
}

// NewRegistryWithTransports builds all clients, assigning the shared transport
// to MSAG / BMC / NewBMC / Spectrum / GMLC and a per-instance transport to
// each LMF (mirroring Java's nbiClient for the first group and lmf1Client..4
// for the LMF instances).
func NewRegistryWithTransports(cfg *ExternalConfig, tr *Transports) *Registry {
	if tr == nil {
		return NewRegistry(cfg, nil)
	}
	lmfs := make([]*LMFClient, 0, len(cfg.LMF))
	for i, l := range cfg.LMF {
		name := "lmf"
		if i > 0 {
			name = fmt.Sprintf("lmf-%d", i+1)
		}
		lt := tr.Shared
		if i < len(tr.LMF) {
			lt = tr.LMF[i]
		}
		lmfs = append(lmfs, NewLMFClient(name, l, lt))
	}
	return &Registry{
		Spectrum: NewSpectrumClient(cfg.Spectrum, tr.Shared),
		MSAG:     NewMSAGClient(cfg.MSAG, tr.Shared),
		BMC:      NewBMCClientOld(cfg.BMC, tr.Shared),
		NewBMC:   NewBMCClientNew(cfg.NewBMC, tr.Shared),
		LMF:      lmfs,
		GMLC:     NewGMLCClient(cfg.GMLC, tr.Shared),
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
