package model

type Environment struct {
	ID            int    `json:"Id"`
	Name          string `json:"Name"`
	URL           string `json:"URL"`
	PublicURL     string `json:"PublicURL"`
	GroupID       int    `json:"GroupId"`
	Status        int    `json:"Status"`
	Type          int    `json:"Type"`
	EdgeID        string `json:"EdgeID"`
	TagIDs        []int  `json:"TagIds"`
	TLS           bool   `json:"TLS"`
	RunningStatus string `json:"-"`
}

type Port struct {
	IP          string `json:"IP"`
	PrivatePort int    `json:"PrivatePort"`
	PublicPort  int    `json:"PublicPort"`
	Type        string `json:"Type"`
}

type Container struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Command string   `json:"Command"`
	Created int64    `json:"Created"`
	Ports   []Port   `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
}

type Stack struct {
	ID              int    `json:"Id"`
	Name            string `json:"Name"`
	Type            int    `json:"Type"`
	Status          int    `json:"Status"`
	EndpointID      int    `json:"EndpointId"`
	EntryPoint      string `json:"EntryPoint"`
	CreationDate    int64  `json:"CreationDate"`
	UpdateDate      int64  `json:"UpdateDate"`
	CreatedBy       string `json:"CreatedBy"`
	UpdatedBy       string `json:"UpdatedBy"`
	ResourceControl any    `json:"ResourceControl"`
	Origin          string `json:"-"`
	Limited         bool   `json:"-"`
}

type Image struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	Size        int64    `json:"Size"`
	Created     int64    `json:"Created"`
	Containers  int64    `json:"Containers"`
	RepoDigests []string `json:"RepoDigests"`
}
