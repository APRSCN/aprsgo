package model

// HistoryReturn provides a struct to return history of server
type HistoryReturn struct {
	Msg            string   `json:"msg"`
	Memory         [][2]any `json:"memory"`
	UplinkPacketRX [][2]any `json:"uplinkPacketRX"`
	UplinkPacketTX [][2]any `json:"uplinkPacketTX"`
	UplinkBytesRX  [][2]any `json:"uplinkBytesRX"`
	UplinkBytesTX  [][2]any `json:"uplinkBytesTX"`
}
