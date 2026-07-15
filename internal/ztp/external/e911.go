package external

import "context"

// CancelDTO mirrors Java's CancelDTO — the record of what was pushed to each
// external system for a device, persisted in cpe_element.e911_data so the
// failure path can roll the registrations back.
type CancelDTO struct {
	GmlcIdentifier string `json:"gmlcIdentifier"`
	BmcAdded       bool   `json:"bmcAdded"`
	LmfAdded       bool   `json:"lmfAdded"`
	GmlcAdded      bool   `json:"gmlcAdded"`
	GndId          int    `json:"gndId"`
	CellId         int    `json:"cellId"`
	Mcc            string `json:"mcc"`
	Mnc            string `json:"mnc"`
	Tac            int    `json:"tac"`
	ArfcnDl        int    `json:"arfcnDl"`
	ArfcnUl        int    `json:"arfcnUl"`
}

// Rollback drives the E911 rollback: when a device fails provisioning, every
// external system it was registered against is asked to delete the cell. The
// cancel flags gate which systems are contacted (matching Java's
// deleteInfoFromE911Components).
type Rollback struct {
	reg *Registry
}

// NewRollback builds a rollback driver from a registry.
func NewRollback(reg *Registry) *Rollback {
	return &Rollback{reg: reg}
}

// DeleteInfoFromE911Components rolls back the external registrations recorded
// in cancel. It is best-effort: individual delete errors are ignored so one
// stubborn system does not block the others (matching Java's fire-and-forget
// cleanup).
func (r *Rollback) DeleteInfoFromE911Components(ctx context.Context, cancel *CancelDTO, dev *DeviceContext) {
	if cancel == nil {
		return
	}
	if cancel.BmcAdded {
		_ = r.reg.BMC.Delete(ctx, dev)
		_ = r.reg.NewBMC.Delete(ctx, dev)
	}
	if cancel.LmfAdded {
		for _, l := range r.reg.LMF {
			_ = l.Delete(ctx, dev)
		}
	}
	if cancel.GmlcAdded {
		_ = r.reg.GMLC.Delete(ctx, dev)
	}
}
