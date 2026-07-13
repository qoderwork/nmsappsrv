package tr069

import "nmsappsrv/internal/tr069/soap"

// DefaultSender is the process-wide OperationSender, set during wiring (cmd/main.go)
// so that other modules (e.g. monitor ingestion) can issue device operations without
// reconstructing the sender.
var DefaultSender *OperationSender

// MonitorGPVCallback is invoked by the GetParameterValues response handler when a GPV
// response arrives whose operationId starts with "monitor:". The monitor package
// registers this at startup to persist device samples into monitor_data.
//
// It is declared here (not in the monitor package) to avoid an import cycle:
// monitor imports tr069 (to send GPV + set this hook), but tr069 must not import monitor.
var MonitorGPVCallback func(sn, operationId string, values []soap.ParameterValueStruct)

// MMLResponseCallback is invoked by the SetParameterValues response handler when
// an SPV response arrives whose operationId starts with "mml:". The mml package
// registers this at startup to mark the execution result delivered (status=3)
// and record success/failure via has_fault (对齐 Java MmlMessageProcessor).
//
// It is declared here (not in the mml package) to avoid an import cycle:
// mml imports tr069 (to send the SPV + set this hook), but tr069 must not import mml.
var MMLResponseCallback func(resultId int, success bool, faultString string)
