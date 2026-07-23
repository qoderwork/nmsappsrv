package license

import "time"

// TrueLicenseContent mirrors the Java TrueLicense LicenseContent bean exactly.
//
// Java side (de.fusseltron.license.LicenseContent / TrueLicense XML shape)
// exposes these fields via JAXB/XStream serialization wrapped in a CMS/PKCS7
// signed envelope. Every field here maps 1:1 so Go and Java can parse the same
// signed license file byte-for-byte once the verifier is swapped.
//
// Note: this struct is currently NOT used for verification — the active
// verifier still consumes go-infra/licensing.License. It IS used as the
// authoritative schema when implementing the TrueLicense-compatible verifier
// in a follow-up change.
type TrueLicenseContent struct {
	Subject        string            `xml:"subject"        json:"subject"`
	Holder         map[string]string `xml:"holder"         json:"holder"`          // X500Principal attributes, e.g. CN=..., O=...
	Issuer         map[string]string `xml:"issuer"         json:"issuer"`          // X500Principal of signing CA
	Issued         time.Time         `xml:"issued"         json:"issued"`          // issuedTime
	NotBefore      time.Time         `xml:"notBefore"      json:"notBefore"`       // validity start
	NotAfter       time.Time         `xml:"notAfter"       json:"notAfter"`        // validity end (= Expiry)
	ConsumerType   string            `xml:"consumerType"   json:"consumerType"`    // "USER" / "DEVICE" / "SERVER"
	ConsumerAmount int32             `xml:"consumerAmount" json:"consumerAmount"`  // seat count tied to ConsumerType
	Info           string            `xml:"info"           json:"info"`            // free-form description
	Extra          map[string]string `xml:"extra"          json:"extra,omitempty"` // NMS vendor extensions: enb_quantity, gnb_quantity, cpe_quantity, user_quantity, tenant_code, vendor_code, ...
}

// --- Well-known Extra keys shared with Java LicenseCustomizer -------------

const (
	ExtraEnbQuantity   = "enb_quantity"
	ExtraGnbQuantity   = "gnb_quantity"
	ExtraCpeQuantity   = "cpe_quantity"
	ExtraUserQuantity  = "user_quantity"
	ExtraTenantCode    = "tenant_code"
	ExtraLicenseName   = "license_name"
	ExtraLicenseType   = "license_type"
	ExtraAcsUrl        = "acs_url"
	ExtraOmcName       = "omc_name"
	ExtraProvinceAbbr  = "province_abbreviation"
	ExtraVendorCode    = "vendor_code"
	ExtraTimezone      = "timezone"
	ExtraRoleId        = "role_id"
	ExtraMachineFp     = "machine_fingerprint" // system-uuid binding
)
