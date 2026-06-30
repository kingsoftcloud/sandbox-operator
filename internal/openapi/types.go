package openapi

type Credential struct {
	AccessKeyID     string
	SecretAccessKey string
	AccountID       string
	Region          string
}

type Template struct {
	TemplateID                   string `json:"id"`
	TemplateName                 string `json:"name"`
	Description                  string `json:"description"`
	TemplateCategory             string `json:"access"`
	TemplateType                 string `json:"type"`
	ImageConfig                  *ImageConfig
	Command                      string `json:"startCmd"`
	Ports                        []int
	CPU                          int `json:"cpuCount"`
	Memory                       int `json:"memoryMB"`
	Envs                         []Env
	NetworkConfig                *NetworkConfig
	PreheatConfig                *PreheatConfig
	InstanceQuota                int    `json:"quota"`
	Status                       string `json:"status"`
	CanDelete                    bool   `json:"canDelete"`
	CreatedAt                    string `json:"createdAt"`
	UpdatedAt                    string `json:"updatedAt"`
	KlogProjectName              string
	KlogPoolName                 string
	RemainingInstanceQuota       int
	RemainingSystemInstanceQuota int
	PreheatedInstanceNumber      int
	CredentialAccessKeyIDMasked  string
}

type Sandbox struct {
	SandboxID           string `json:"sandboxID"`
	TemplateID          string `json:"templateID"`
	TemplateType        string `json:"templateType"`
	TemplateCategory    string
	Status              string `json:"state"`
	Timeout             int    `json:"timeout"`
	CreateTime          string `json:"startedAt"`
	EndTime             string `json:"endAt"`
	Endpoint            string `json:"domain"`
	CustomConfiguration *CustomConfiguration
}

type ImageConfig struct {
	Source             string
	ImageURL           string
	ImageEndpoint      string
	ImageNamespace     string
	ImageName          string
	ImageTag           string
	RegistryInstanceID string
}

type NetworkConfig struct {
	PublicNetworkEnable        bool
	PrivateNetworkEnable       bool
	SharedInternetAccessEnable bool
	VPCID                      string
	SubnetID                   string
}

type PreheatConfig struct {
	Enabled bool
	Number  int
}

type Env struct {
	Key   string
	Value string
}

type MountConfig struct {
	Enabled         bool
	AccessKey       string
	SecretAccessKey string
	MountPoints     []MountPoint
}

type MountPoint struct {
	BucketName     string
	FileSystemName string
	RemotePath     string
	LocalMountPath string
	ReadOnly       bool
}

type CustomConfiguration struct {
	ImageURL string
	Port     int
	Command  string
}

type CreateTemplateRequest struct {
	TemplateName     string
	Description      string
	TemplateCategory string
	TemplateType     string
	ImageConfig      *ImageConfig
	Command          string
	Ports            []int
	CPU              int
	Memory           int
	Envs             []Env
	NetworkConfig    *NetworkConfig
	PreheatConfig    *PreheatConfig
	InstanceQuota    int
	KS3MountConfig   *MountConfig
	KPFSMountConfig  *MountConfig
	KlogEnabled      bool
}

type CreateTemplateResponse struct {
	TemplateID      string
	KlogProjectName string
	KlogPoolName    string
}

type UpdateTemplateRequest struct {
	TemplateID string
	CreateTemplateRequest
}

type StartSandboxRequest struct {
	TemplateID      string
	Timeout         int
	Envs            []Env
	KS3MountConfig  *MountConfig
	KPFSMountConfig *MountConfig
}

type StartSandboxResponse struct {
	SandboxID  string
	Endpoint   string
	TemplateID string
	Token      string
	Timeout    int
}

type ListTemplatesRequest struct {
	PageNum  int `json:"page"`
	PageSize int `json:"pageSize"`
}

type ListSandboxesRequest struct {
	PageNum  int `json:"page"`
	PageSize int `json:"pageSize"`
}

type TemplateList struct {
	Items     []Template `json:"templates"`
	NextToken string     `json:"nextToken"`
	Total     int        `json:"totalCount"`
}

type SandboxList struct {
	Items     []Sandbox `json:"sandboxes"`
	NextToken string    `json:"nextToken"`
	Total     int       `json:"totalCount"`
}
