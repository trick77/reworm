package account

// Identity is the paired device a request comes from and the profile it plays as.
//
// Table: devices
type Identity struct {
	DeviceID    string
	DeviceLabel string
	ProfileID   string
	Name        string
	Avatar      string
	Admin       bool
}

// profileListJSON is one profile GET /api/profiles lists.
//
// Table: profiles
type profileListJSON struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// deviceListJSON is one device GET /api/devices lists.
//
// Table: devices
type deviceListJSON struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	LastSeen int64  `json:"lastSeen"`
	Current  bool   `json:"current"`
}
