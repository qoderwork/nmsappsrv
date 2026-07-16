// Package opmsg defines the canonical device-operation message payload that
// travels through Redis LIST `mq.OperationQueue` ("operation_queue").
//
// The struct mirrors Java `OperationDto` field-for-field so the dispatcher in
// `internal/operation` can faithfully reproduce
// `apiCommandProcessor.processCommand` semantics.
//
// This package intentionally has **no dependencies on tr069 / misc** so it can
// be imported by any publisher without creating an import cycle (the
// dispatcher itself depends on tr069, and tr069 depends on misc).
package opmsg

import (
	"encoding/json"
	"time"
)

// ProtocolType values (mirrors Java `OperationDto.PROTOCOL_*`).
const (
	ProtocolTR069 = 1 // Java OperationDto.PROTOCOL_TR069
	ProtocolDAS   = 2 // Java OperationDto.PROTOCOL_DAS (reserved; Go currently only ships TR-069)
)

// Message is the canonical payload pushed to `mq.OperationQueue` and consumed
// by the dispatcher worker. It mirrors Java's `OperationDto` field set.
type Message struct {
	// EventType is a coarse label kept for backwards-compat with Go's older
	// per-publisher `operationMessage.EventType`. In Java this is the same
	// string as `Operation`; the dispatcher switch reads `Operation`.
	EventType string `json:"eventType,omitempty"`

	// NeNeid is the `cpe_element.ne_neid` (device element id).
	NeNeid int64 `json:"neNeid"`

	// Operation is the device-operation type. Must match a Java EventType
	// constant string verbatim (PascalCase, e.g. "Reboot", "FactoryReset",
	// "SetParameterValues", "AddObject", "UpdateCertificate", "CollectLog",
	// "Shutdown", "GetParameterValues", "Download", "Upload").
	Operation string `json:"operation"`

	// OperationParam is a JSON-encoded payload whose schema depends on
	// `Operation`:
	//   - "Download" / "UpdateCertificate" / "ZTP" / "UpdateLicense" / "Restore" /
	//     "SendCBSDCertFile"  → TR069DownloadDTO JSON
	//   - "Upload" / "CollectLog" / "Backup" / "BackupDaily"  → TR069UploadDTO JSON
	//   - "SetParameterValues"  → JSON of `[]ParameterValueStruct`
	//   - "GetParameterValues"  → JSON of `[]string` (parameter paths)
	//   - "AddObject" / "DeleteObject"  → object path string
	//   - others may be empty
	OperationParam string `json:"operationParam,omitempty"`

	// OperationUser is the operator username; stored in event_log and used by
	// the REBOOT / SOFT_REBOOT post-processing (sets `rebootUser_{neId}`).
	OperationUser string `json:"operationUser,omitempty"`

	// IsNorthSend is true when the operation originates from the north-bound
	// REST API. Java carries this on the DTO; Go currently sets it on the
	// north-bound publish path and the dispatcher preserves it on event_log.
	IsNorthSend bool `json:"isNorthSend,omitempty"`

	// TaskId is the originating task id (e.g. reboot_task / batch_add_object_task
	// / cbsdcert_file_send_task). Java transient; not persisted.
	TaskId int `json:"taskId,omitempty"`

	// BatchId is the batch identifier when the operation is part of a batch.
	// Java transient; not persisted.
	BatchId int64 `json:"batchId,omitempty"`

	// ProtocolType selects the message-builder. Currently only
	// `ProtocolTR069` (=1) is wired; the dispatcher throws on `ProtocolDAS`.
	// Mirrors Java's `OperationDto.PROTOCOL_TR069 / PROTOCOL_DAS`.
	ProtocolType int `json:"protocolType,omitempty"`

	// Priority is reserved for future Qos routing; Java carries it on the DTO.
	Priority int `json:"priority,omitempty"`

	// ExpiredAt is the millisecond epoch after which the SOAP message is
	// considered stale. Mirrors Java's `EventDto.expiredAt`. The dispatcher
	// passes it to the per-`Send*` primitives that accept it.
	ExpiredAt int64 `json:"expiredAt,omitempty"`

	// CommandTrackId is the `event_log.id` row that owns this operation. It
	// becomes the `command_track` row's id (Java) or the `event_log`
	// `command_track_data` JSON entry (Go consolidated). The dispatcher reads
	// it to update the originating event_log status on dispatch failure.
	CommandTrackId int64 `json:"commandTrackId,omitempty"`

	// WebTrackId is the front-facing tracker id. Mirrors Java
	// `OperationDto.webTrackId`; the dispatcher passes it to the SOAP layer
	// when the device response handler correlates back to a UI request.
	WebTrackId string `json:"webTrackId,omitempty"`
}

// Marshal serializes the message to JSON for `LPush`. Equivalent to
// `json.Marshal(msg)`; this helper exists so call sites do not have to import
// `encoding/json` directly.
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal parses a payload read from `BRPop`. Inverse of `Marshal`.
func Unmarshal(data []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ExpiredAtTime returns ExpiredAt as a `time.Time` (UTC) for callers that
// want to pass it into the per-`Send*` primitives. Returns zero time if
// ExpiredAt <= 0.
func (m *Message) ExpiredAtTime() time.Time {
	if m.ExpiredAt <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(m.ExpiredAt).UTC()
}
