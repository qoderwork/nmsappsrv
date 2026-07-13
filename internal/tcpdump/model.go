package tcpdump

// NetworkCard mirrors Java's NetworkCartVO { ip, name } returned by
// GET /api/v2/listNetworkCards. In Java this is populated from /home/ip.info
// (lines of "<ip> <name>"); here we fall back to enumerating host interfaces.
type NetworkCard struct {
	IP   string `json:"ip"`
	Name string `json:"name"`
}

// TcpdumpFile mirrors Java's TcpdumpFileVo { fileName, modifyTime } returned by
// GET /api/v2/listTcpdumpFiles. modifyTime is epoch milliseconds (Java serialises
// java.util.Date as a number), which the frontend's date column expects.
type TcpdumpFile struct {
	FileName   string `json:"fileName"`
	ModifyTime int64  `json:"modifyTime"`
}

// DoCaptureRequest mirrors Java's DoTcpdumpDTO { duration, container }.
// The capture command is identical regardless of container (tcpdump -i eth0),
// so `container` is used only as the file-name prefix and as metadata, exactly
// like Java where it just selected which (api/comm/core) container's eth0 ran.
type DoCaptureRequest struct {
	Duration  int    `json:"duration" binding:"required"`
	Container string `json:"container" binding:"required"`
}

// DeleteFileRequest mirrors Java's DeleteTcpdumpFileDTO { fileName }.
type DeleteFileRequest struct {
	FileName string `json:"fileName" binding:"required"`
}

// BatchDeleteFileRequest mirrors Java's BatchDeleteTcpdumpFileDTO { fileNames }.
type BatchDeleteFileRequest struct {
	FileNames []string `json:"fileNames" binding:"required"`
}
