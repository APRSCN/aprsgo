package meta

import (
	"fmt"
	"time"
)

const ENName = "APRSGo"
const Nickname = "Ampere"
const Version = "0.0.1"

var StartAt = time.Now()
var ServerText = fmt.Sprintf("%s/%s %s", ENName, Version, Nickname)
