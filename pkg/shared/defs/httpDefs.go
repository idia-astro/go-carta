package defs

type WorkerSpawnBody struct {
	Username string `json:"username"`
}

type WorkerListItem struct {
	WorkerId  string `json:"workerId"`
	ProcessId int    `json:"processId"`
	Username  string `json:"username"`
	UserId    uint32 `json:"userId"`
}

type WorkerInfo struct {
	Address   string `json:"address"`
	Port      int    `json:"port"`
	ProcessId int    `json:"processId"`
	UserId    uint32 `json:"userId"`
	WorkerId  string `json:"workerId"`
}

type WorkerStatus struct {
	WorkerInfo
	Alive         bool `json:"alive"`
	ExitedCleanly bool `json:"exitedCleanly"`
	IsReachable   bool `json:"isReachable"`
}
