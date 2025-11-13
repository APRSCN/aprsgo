package model

// StatsReturn provides a struct to return stats of server
type StatsReturn struct {
	Msg            string   `json:"msg"`
	Memory         [][2]any `json:"memory"`
	UplinkPacketRX [][2]any `json:"uplinkPacketRX"`
	UplinkPacketTX [][2]any `json:"uplinkPacketTX"`
	UplinkBytesRX  [][2]any `json:"uplinkBytesRX"`
	UplinkBytesTX  [][2]any `json:"uplinkBytesTX"`
}
