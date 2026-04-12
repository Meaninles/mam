package integration

type GatewayRecord struct {
	ID               string `json:"id"`
	GatewayType      string `json:"gatewayType"`
	DisplayName      string `json:"displayName"`
	BaseURL          string `json:"baseUrl"`
	Enabled          bool   `json:"enabled"`
	RuntimeStatus    string `json:"runtimeStatus"`
	ClientDeviceID   string `json:"clientDeviceId,omitempty"`
	LastTestAt       string `json:"lastTestAt,omitempty"`
	LastErrorCode    string `json:"lastErrorCode,omitempty"`
	LastErrorMessage string `json:"lastErrorMessage,omitempty"`
	HasPassword      bool   `json:"hasPassword"`
	Username         string `json:"username,omitempty"`
}

type GatewayListResponse struct {
	Items []GatewayRecord `json:"items"`
}

type SaveCD2GatewayRequest struct {
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

type SaveCD2GatewayResponse struct {
	Message string        `json:"message"`
	Record  GatewayRecord `json:"record"`
}

type TestCD2GatewayRequest struct {
	BaseURL  string `json:"baseUrl,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

type TestCD2GatewayResponse struct {
	Message string        `json:"message"`
	Record  GatewayRecord `json:"record"`
}

type RuntimeComponentRecord struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	LastCheckedAt    string `json:"lastCheckedAt,omitempty"`
	LastErrorCode    string `json:"lastErrorCode,omitempty"`
	LastErrorMessage string `json:"lastErrorMessage,omitempty"`
}

type RuntimeStatusResponse struct {
	Components []RuntimeComponentRecord `json:"components"`
}
