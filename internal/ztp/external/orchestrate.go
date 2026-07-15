package external

import "context"

// RunRegistration pushes a single device to every configured external E911
// system, in the Java-mandated order: MSAG → BMC (old) → NewBMC → LMF 1..4 →
// GMLC. As each system succeeds its cancel flag is set so a later failure can
// roll it back. The first failure short-circuits and returns the failing
// system's name plus the underlying error; the caller is expected to invoke
// Rollback.DeleteInfoFromE911Components with the (partially-populated) cancel
// record. A disabled system is a no-op (registrar.Add returns nil without
// calling the transport), so it neither blocks nor needs rollback.
//
// This is the canonical orchestration used by ztp.Thread.ProcessElement; it is
// factored out so the ordering + rollback-on-failure behaviour is unit-testable
// without a database.
func RunRegistration(ctx context.Context, reg *Registry, dev *DeviceContext, cancel *CancelDTO) (string, error) {
	if err := reg.MSAG.Add(ctx, dev); err != nil {
		return "MSAG", err
	}
	if err := reg.BMC.Add(ctx, dev); err != nil {
		return "BMC", err
	}
	cancel.BmcAdded = true
	if err := reg.NewBMC.Add(ctx, dev); err != nil {
		return "new BMC", err
	}
	cancel.BmcAdded = true

	lmfAdded := false
	for _, l := range reg.LMF {
		if err := l.Add(ctx, dev); err != nil {
			return l.Name(), err
		}
		lmfAdded = true
	}
	if lmfAdded {
		cancel.LmfAdded = true
	}

	if err := reg.GMLC.Add(ctx, dev); err != nil {
		return "GMLC", err
	}
	cancel.GmlcAdded = true
	return "", nil
}
